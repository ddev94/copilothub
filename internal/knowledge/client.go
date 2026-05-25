package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	chromem "github.com/philippgille/chromem-go"
)

const (
	dbFile   = "knowledge.db"
	metaFile = "knowledge-meta.json"
)

// Client is the native Go knowledge store backed by chromem-go (vector DB)
// with Ollama embeddings (all-MiniLM-L6-v2). Data is persisted to a single file.
type Client struct {
	db     *chromem.DB
	dbPath string
	embed  chromem.EmbeddingFunc
	meta   *metaStore
	mu     sync.Mutex
	queue  chan ingestJob
}

type ingestJob struct {
	projectID string
	docID     string
	fileName  string
	chunks    []string
}

type metaStore struct {
	mu   sync.RWMutex
	path string
	docs []metaEntry
}

type metaEntry struct {
	ProjectID  string  `json:"projectId"`
	DocID      string  `json:"docId"`
	Name       string  `json:"name"`
	SourceFile string  `json:"sourceFile"`
	CreatedAt  string  `json:"createdAt"`
	Status     string  `json:"status"`     // pending, approved, rejected
	Verified   bool    `json:"verified"`   // true if approved
	Confidence float64 `json:"confidence"` // 0.0-1.0
	SourceType string  `json:"sourceType"` // upload, chat, wiki
	ApprovedBy string  `json:"approvedBy,omitempty"`
	ApprovedAt string  `json:"approvedAt,omitempty"`
	RejectedAt string  `json:"rejectedAt,omitempty"`
}

// Document is stored document metadata (without chunks).
type Document struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	SourceFile string  `json:"sourceFile"`
	CreatedAt  string  `json:"createdAt"`
	Status     string  `json:"status"`
	Verified   bool    `json:"verified"`
	Confidence float64 `json:"confidence"`
	SourceType string  `json:"sourceType"`
	ApprovedBy string  `json:"approvedBy,omitempty"`
	ApprovedAt string  `json:"approvedAt,omitempty"`
}

// Chunk is a retrieved text segment with similarity score.
type Chunk struct {
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// NewClient opens (or creates) a knowledge store at storeDir using the given embedding config.
func NewClient(storeDir string, embedCfg EmbeddingConfig) (*Client, error) {
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, fmt.Errorf("knowledge store: %w", err)
	}
	dbPath := filepath.Join(storeDir, dbFile)

	db := chromem.NewDB()
	if _, err := os.Stat(dbPath); err == nil {
		if err := db.ImportFromFile(dbPath, ""); err != nil {
			db = chromem.NewDB() // start fresh if corrupted
		}
	}

	ms := &metaStore{path: filepath.Join(storeDir, metaFile)}
	if data, err := os.ReadFile(ms.path); err == nil {
		_ = json.Unmarshal(data, &ms.docs)
	}

	embed, err := NewEmbeddingFunc(embedCfg, defaultModelsDir())
	if err != nil {
		return nil, fmt.Errorf("embedding model: %w", err)
	}

	client := &Client{
		db:     db,
		dbPath: dbPath,
		embed:  embed,
		meta:   ms,
		queue:  make(chan ingestJob, 64),
	}
	go client.ingestWorker()
	return client, nil
}

func (c *Client) collection(ctx context.Context, projectID string) (*chromem.Collection, error) {
	return c.db.GetOrCreateCollection("proj_"+projectID, nil, c.embed)
}

func (c *Client) persist() error {
	return c.db.ExportToFile(c.dbPath, false, "")
}

// Ingest extracts text from filePath, chunks it, embeds and stores under projectID.
func (c *Client) Ingest(ctx context.Context, projectID, filePath, fileName, _ string) error {
	text, err := extractText(filePath, fileName)
	if err != nil {
		return fmt.Errorf("text extraction: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("no text extracted from %s", fileName)
	}

	var chunks []string
	if ext := strings.ToLower(filepath.Ext(fileName)); ext == ".md" || ext == ".markdown" {
		chunks = splitMarkdown(text, defaultChunkSize, defaultChunkOverlap)
	} else {
		chunks = splitText(text, defaultChunkSize, defaultChunkOverlap)
	}
	if len(chunks) == 0 {
		return fmt.Errorf("document produced no chunks: %s", fileName)
	}

	col, err := c.collection(ctx, projectID)
	if err != nil {
		return err
	}

	docID := uuid.New().String()
	docs := make([]chromem.Document, len(chunks))
	for i, chunk := range chunks {
		docs[i] = chromem.Document{
			ID:      fmt.Sprintf("%s_%d", docID, i),
			Content: chunk,
			Metadata: map[string]string{
				"doc_id": docID,
				"source": fileName,
			},
		}
	}
	if err := col.AddDocuments(ctx, docs, runtime.NumCPU()); err != nil {
		return fmt.Errorf("embed and store: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.persist(); err != nil {
		return fmt.Errorf("persist DB: %w", err)
	}

	c.meta.mu.Lock()
	c.meta.docs = append(c.meta.docs, metaEntry{
		ProjectID:  projectID,
		DocID:      docID,
		Name:       fileName,
		SourceFile: fileName,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		Status:     "pending",
		Verified:   false,
		Confidence: 0.8,
		SourceType: "upload",
	})
	c.meta.mu.Unlock()

	return c.meta.save()
}

// IngestAsync runs ingestion in the background with progress tracking.
// Returns docID immediately; embedding happens asynchronously.
func (c *Client) IngestAsync(ctx context.Context, projectID, filePath, fileName, sourceType string) (string, error) {
	text, err := extractText(filePath, fileName)
	if err != nil {
		return "", fmt.Errorf("text extraction: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("no text extracted from %s", fileName)
	}

	var chunks []string
	if ext := strings.ToLower(filepath.Ext(fileName)); ext == ".md" || ext == ".markdown" {
		chunks = splitMarkdown(text, defaultChunkSize, defaultChunkOverlap)
	} else {
		chunks = splitText(text, defaultChunkSize, defaultChunkOverlap)
	}
	if len(chunks) == 0 {
		return "", fmt.Errorf("document produced no chunks: %s", fileName)
	}

	docID := uuid.New().String()

	// Save metadata immediately so the document appears in the list
	c.meta.mu.Lock()
	c.meta.docs = append(c.meta.docs, metaEntry{
		ProjectID:  projectID,
		DocID:      docID,
		Name:       fileName,
		SourceFile: fileName,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		Status:     "pending",
		Verified:   false,
		Confidence: 0.8,
		SourceType: sourceType,
	})
	c.meta.mu.Unlock()
	_ = c.meta.save()

	// Enqueue for sequential processing
	c.queue <- ingestJob{projectID: projectID, docID: docID, fileName: fileName, chunks: chunks}

	return docID, nil
}

// ingestWorker processes enqueued embedding jobs one at a time to avoid
// concurrent map access in chromem-go.
func (c *Client) ingestWorker() {
	for job := range c.queue {
		c.ingestBackground(job.projectID, job.docID, job.fileName, job.chunks)
	}
}

// ingestBackground embeds chunks in batches and broadcasts progress.
func (c *Client) ingestBackground(projectID, docID, fileName string, chunks []string) {
	ctx := context.Background()
	total := len(chunks)

	IngestProgressTracker.Set(IngestProgress{
		State:       IngestRunning,
		DocID:       docID,
		FileName:    fileName,
		Message:     fmt.Sprintf("Embedding %d chunks...", total),
		ChunksTotal: total,
	})

	col, err := c.collection(ctx, projectID)
	if err != nil {
		IngestProgressTracker.Set(IngestProgress{
			State:   IngestFailed,
			DocID:   docID,
			Message: err.Error(),
		})
		return
	}

	// Process in batches of 20 chunks for better progress feedback
	batchSize := 20
	done := 0

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := chunks[i:end]

		docs := make([]chromem.Document, len(batch))
		for j, chunk := range batch {
			docs[j] = chromem.Document{
				ID:      fmt.Sprintf("%s_%d", docID, i+j),
				Content: chunk,
				Metadata: map[string]string{
					"doc_id": docID,
					"source": fileName,
				},
			}
		}

		if err := col.AddDocuments(ctx, docs, runtime.NumCPU()); err != nil {
			IngestProgressTracker.Set(IngestProgress{
				State:   IngestFailed,
				DocID:   docID,
				Message: fmt.Sprintf("Embedding failed at chunk %d: %v", done, err),
			})
			return
		}

		done += len(batch)
		pct := done * 100 / total
		IngestProgressTracker.Set(IngestProgress{
			State:       IngestRunning,
			DocID:       docID,
			FileName:    fileName,
			Message:     fmt.Sprintf("Embedding... %d/%d chunks", done, total),
			ChunksDone:  done,
			ChunksTotal: total,
			Percent:     pct,
		})
	}

	// Persist DB
	c.mu.Lock()
	_ = c.persist()
	c.mu.Unlock()

	IngestProgressTracker.Set(IngestProgress{
		State:       IngestCompleted,
		DocID:       docID,
		FileName:    fileName,
		Message:     fmt.Sprintf("Done! %d chunks embedded", total),
		ChunksDone:  total,
		ChunksTotal: total,
		Percent:     100,
	})
}

// ListDocuments returns metadata for all documents stored under projectID.
func (c *Client) ListDocuments(_ context.Context, projectID string) ([]Document, error) {
	c.meta.mu.RLock()
	defer c.meta.mu.RUnlock()

	docs := []Document{}
	for _, e := range c.meta.docs {
		if e.ProjectID == projectID {
			docs = append(docs, Document{
				ID:         e.DocID,
				Name:       e.Name,
				SourceFile: e.SourceFile,
				CreatedAt:  e.CreatedAt,
				Status:     e.Status,
				Verified:   e.Verified,
				Confidence: e.Confidence,
				SourceType: e.SourceType,
				ApprovedBy: e.ApprovedBy,
				ApprovedAt: e.ApprovedAt,
			})
		}
	}
	return docs, nil
}

// DeleteDocument removes all vectors and metadata for docID from projectID.
func (c *Client) DeleteDocument(ctx context.Context, projectID, docID string) error {
	col, err := c.collection(ctx, projectID)
	if err != nil {
		return err
	}
	if err := col.Delete(ctx, map[string]string{"doc_id": docID}, nil); err != nil {
		return fmt.Errorf("delete vectors: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.persist(); err != nil {
		return fmt.Errorf("persist DB: %w", err)
	}

	c.meta.mu.Lock()
	kept := c.meta.docs[:0]
	for _, e := range c.meta.docs {
		if !(e.ProjectID == projectID && e.DocID == docID) {
			kept = append(kept, e)
		}
	}
	c.meta.docs = kept
	c.meta.mu.Unlock()

	return c.meta.save()
}

// Retrieve returns the topK most semantically similar chunks for query.
func (c *Client) Retrieve(ctx context.Context, projectID, query string, topK int) ([]Chunk, error) {
	col, err := c.collection(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if col.Count() == 0 {
		return []Chunk{}, nil
	}
	results, err := col.Query(ctx, query, topK, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("vector query: %w", err)
	}

	approved := c.approvedDocIDSet(projectID)
	chunks := make([]Chunk, 0, len(results))
	for _, r := range results {
		docID := ""
		if r.Metadata != nil {
			docID = r.Metadata["doc_id"]
		}
		if !approved[docID] {
			continue
		}
		chunks = append(chunks, Chunk{Content: r.Content, Score: float64(r.Similarity)})
	}
	return chunks, nil
}

func (c *Client) PendingDocuments(_ context.Context, projectID string) ([]Document, error) {
	c.meta.mu.RLock()
	defer c.meta.mu.RUnlock()

	docs := []Document{}
	for _, e := range c.meta.docs {
		if e.ProjectID == projectID && e.Status == "pending" {
			docs = append(docs, Document{ID: e.DocID, Name: e.Name, SourceFile: e.SourceFile, CreatedAt: e.CreatedAt, Status: e.Status, Verified: e.Verified, Confidence: e.Confidence, SourceType: e.SourceType})
		}
	}
	return docs, nil
}

func (c *Client) ApproveDocument(_ context.Context, projectID, docID, approvedBy string) error {
	c.meta.mu.Lock()
	found := false
	for i := range c.meta.docs {
		e := &c.meta.docs[i]
		if e.ProjectID == projectID && e.DocID == docID {
			e.Status = "approved"
			e.Verified = true
			e.ApprovedBy = approvedBy
			e.ApprovedAt = time.Now().UTC().Format(time.RFC3339)
			e.RejectedAt = ""
			found = true
			break
		}
	}
	c.meta.mu.Unlock()
	if !found {
		return fmt.Errorf("document not found: %s", docID)
	}
	return c.meta.save()
}

func (c *Client) RejectDocument(_ context.Context, projectID, docID string) error {
	c.meta.mu.Lock()
	found := false
	for i := range c.meta.docs {
		e := &c.meta.docs[i]
		if e.ProjectID == projectID && e.DocID == docID {
			e.Status = "rejected"
			e.Verified = false
			e.ApprovedBy = ""
			e.ApprovedAt = ""
			e.RejectedAt = time.Now().UTC().Format(time.RFC3339)
			found = true
			break
		}
	}
	c.meta.mu.Unlock()
	if !found {
		return fmt.Errorf("document not found: %s", docID)
	}
	return c.meta.save()
}

func (c *Client) ApproveAllPending(ctx context.Context, projectID, approvedBy string) (int, error) {
	pending, err := c.PendingDocuments(ctx, projectID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, doc := range pending {
		if err := c.ApproveDocument(ctx, projectID, doc.ID, approvedBy); err == nil {
			count++
		}
	}
	return count, nil
}

func (c *Client) approvedDocIDSet(projectID string) map[string]bool {
	c.meta.mu.RLock()
	defer c.meta.mu.RUnlock()
	m := map[string]bool{}
	for _, e := range c.meta.docs {
		if e.ProjectID == projectID && e.Status == "approved" && e.Verified {
			m[e.DocID] = true
		}
	}
	return m
}

// SyncOrphanFiles scans the given directory for files that exist on disk but
// have no metadata entry. It re-ingests them asynchronously.
func (c *Client) SyncOrphanFiles(projectID, filesDir string) int {
	entries, err := os.ReadDir(filesDir)
	if err != nil {
		return 0
	}

	existing := c.knownFileNames(projectID)
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".pdf" && ext != ".md" && ext != ".docx" {
			continue
		}
		if existing[name] {
			continue
		}
		contentType := "text/plain"
		switch ext {
		case ".pdf":
			contentType = "application/pdf"
		case ".md":
			contentType = "text/markdown"
		case ".docx":
			contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		}
		filePath := filepath.Join(filesDir, name)
		if _, err := c.IngestAsync(context.Background(), projectID, filePath, name, contentType); err == nil {
			count++
		}
	}
	return count
}

func (c *Client) knownFileNames(projectID string) map[string]bool {
	c.meta.mu.RLock()
	defer c.meta.mu.RUnlock()
	m := map[string]bool{}
	for _, e := range c.meta.docs {
		if e.ProjectID == projectID {
			m[e.SourceFile] = true
		}
	}
	return m
}

// ── Repo code indexing ──────────────────────────────────────────────

// RepoIndexStatus tracks the indexing state of a repository.
type RepoIndexStatus struct {
	State      string `json:"state"` // "none" | "indexing" | "indexed" | "error"
	TotalFiles int    `json:"totalFiles"`
	DoneFiles  int    `json:"doneFiles"`
	Percent    int    `json:"percent"`
	Message    string `json:"message,omitempty"`
	IndexedAt  string `json:"indexedAt,omitempty"`
}

// repoCollectionName returns the chromem collection name for a repo's code index.
func repoCollectionName(projectID, repoID string) string {
	return "code_" + projectID + "_" + repoID
}

// repoMetaKey returns a synthetic doc ID prefix for repo code entries.
func repoMetaKey(projectID, repoID string) string {
	return "repo:" + projectID + ":" + repoID
}

// IndexRepoAsync indexes all source code in a repository directory asynchronously.
// Returns immediately; progress is broadcast via RepoIndexTracker.
func (c *Client) IndexRepoAsync(projectID, repoID, repoDir string) {
	go c.indexRepoBackground(projectID, repoID, repoDir)
}

func (c *Client) indexRepoBackground(projectID, repoID, repoDir string) {
	ctx := context.Background()
	colName := repoCollectionName(projectID, repoID)

	RepoIndexTracker.Set(RepoIndexProgress{
		ProjectID: projectID,
		RepoID:    repoID,
		State:     "indexing",
		Message:   "Scanning files...",
	})

	// Walk and chunk the repo
	var totalFiles int
	chunks, err := WalkAndChunkRepo(repoDir, func(filePath string) {
		totalFiles++
		RepoIndexTracker.Set(RepoIndexProgress{
			ProjectID:  projectID,
			RepoID:     repoID,
			State:      "indexing",
			Message:    fmt.Sprintf("Reading %s", filePath),
			TotalFiles: totalFiles,
		})
	})
	if err != nil {
		RepoIndexTracker.Set(RepoIndexProgress{
			ProjectID: projectID,
			RepoID:    repoID,
			State:     "error",
			Message:   fmt.Sprintf("Walk failed: %v", err),
		})
		return
	}

	if len(chunks) == 0 {
		RepoIndexTracker.Set(RepoIndexProgress{
			ProjectID: projectID,
			RepoID:    repoID,
			State:     "error",
			Message:   "No indexable source files found",
		})
		return
	}

	RepoIndexTracker.Set(RepoIndexProgress{
		ProjectID:   projectID,
		RepoID:      repoID,
		State:       "indexing",
		Message:     fmt.Sprintf("Embedding %d chunks from %d files...", len(chunks), totalFiles),
		TotalFiles:  totalFiles,
		TotalChunks: len(chunks),
	})

	// Delete old collection if it exists (re-index)
	_ = c.db.DeleteCollection(colName)

	col, err := c.db.GetOrCreateCollection(colName, nil, c.embed)
	if err != nil {
		RepoIndexTracker.Set(RepoIndexProgress{
			ProjectID: projectID,
			RepoID:    repoID,
			State:     "error",
			Message:   fmt.Sprintf("Collection create failed: %v", err),
		})
		return
	}

	// Embed in batches
	batchSize := 20
	done := 0
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]

		docs := make([]chromem.Document, len(batch))
		for j, chunk := range batch {
			docs[j] = chromem.Document{
				ID:      fmt.Sprintf("%s_%d", repoID, i+j),
				Content: chunk.Content,
				Metadata: map[string]string{
					"repo_id":   repoID,
					"file_path": chunk.FilePath,
				},
			}
		}

		if err := col.AddDocuments(ctx, docs, runtime.NumCPU()); err != nil {
			RepoIndexTracker.Set(RepoIndexProgress{
				ProjectID: projectID,
				RepoID:    repoID,
				State:     "error",
				Message:   fmt.Sprintf("Embedding failed at chunk %d: %v", done, err),
			})
			return
		}

		done += len(batch)
		pct := done * 100 / len(chunks)
		RepoIndexTracker.Set(RepoIndexProgress{
			ProjectID:   projectID,
			RepoID:      repoID,
			State:       "indexing",
			Message:     fmt.Sprintf("Embedding... %d/%d chunks", done, len(chunks)),
			TotalFiles:  totalFiles,
			TotalChunks: len(chunks),
			DoneChunks:  done,
			Percent:     pct,
		})
	}

	// Persist
	c.mu.Lock()
	_ = c.persist()
	c.mu.Unlock()

	// Save metadata entry
	metaKey := repoMetaKey(projectID, repoID)
	c.meta.mu.Lock()
	// Remove old repo meta entries
	kept := c.meta.docs[:0]
	for _, e := range c.meta.docs {
		if e.DocID != metaKey {
			kept = append(kept, e)
		}
	}
	c.meta.docs = append(kept, metaEntry{
		ProjectID:  projectID,
		DocID:      metaKey,
		Name:       fmt.Sprintf("repo-index:%s", repoID),
		SourceFile: repoDir,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		Status:     "approved",
		Verified:   true,
		SourceType: "code",
	})
	c.meta.mu.Unlock()
	_ = c.meta.save()

	RepoIndexTracker.Set(RepoIndexProgress{
		ProjectID:   projectID,
		RepoID:      repoID,
		State:       "indexed",
		Message:     fmt.Sprintf("Done! %d chunks from %d files", len(chunks), totalFiles),
		TotalFiles:  totalFiles,
		TotalChunks: len(chunks),
		DoneChunks:  len(chunks),
		Percent:     100,
	})
}

// DeleteRepoIndex removes all indexed code for a repo.
func (c *Client) DeleteRepoIndex(ctx context.Context, projectID, repoID string) error {
	colName := repoCollectionName(projectID, repoID)
	_ = c.db.DeleteCollection(colName)

	c.mu.Lock()
	_ = c.persist()
	c.mu.Unlock()

	metaKey := repoMetaKey(projectID, repoID)
	c.meta.mu.Lock()
	kept := c.meta.docs[:0]
	for _, e := range c.meta.docs {
		if e.DocID != metaKey {
			kept = append(kept, e)
		}
	}
	c.meta.docs = kept
	c.meta.mu.Unlock()

	return c.meta.save()
}

// RetrieveCode retrieves the top-K most relevant code chunks across the given repos.
// If repoIDs is empty, all indexed repos for the project are searched.
func (c *Client) RetrieveCode(ctx context.Context, projectID string, repoIDs []string, query string, topK int) ([]CodeChunk, error) {
	fmt.Println("RetrieveCode for repos:", repoIDs)
	if topK <= 0 {
		topK = 20
	}

	// Determine which repos to search
	searchRepoIDs := repoIDs
	if len(searchRepoIDs) == 0 {
		searchRepoIDs = c.indexedRepoIDs(projectID)
	}

	var allResults []CodeChunk
	perRepo := topK
	if len(searchRepoIDs) > 1 {
		perRepo = topK / len(searchRepoIDs)
		if perRepo < 5 {
			perRepo = 5
		}
	}

	for _, rid := range searchRepoIDs {
		colName := repoCollectionName(projectID, rid)
		col := c.db.GetCollection(colName, c.embed)
		if col == nil || col.Count() == 0 {
			continue
		}

		results, err := col.Query(ctx, query, perRepo, nil, nil)
		if err != nil {
			continue
		}

		for _, r := range results {
			cc := CodeChunk{
				Content: r.Content,
				Score:   float64(r.Similarity),
			}
			if r.Metadata != nil {
				cc.FilePath = r.Metadata["file_path"]
			}
			allResults = append(allResults, cc)
		}
	}

	fmt.Println("All results:", allResults)

	// Sort by score descending and limit to topK
	sortCodeChunks(allResults)
	if len(allResults) > topK {
		allResults = allResults[:topK]
	}

	return allResults, nil
}

// GetRepoIndexStatus returns the indexing status for a specific repo.
func (c *Client) GetRepoIndexStatus(projectID, repoID string) RepoIndexStatus {
	// Check if currently indexing
	prog := RepoIndexTracker.Get()
	if prog.ProjectID == projectID && prog.RepoID == repoID && prog.State == "indexing" {
		return RepoIndexStatus{
			State:      "indexing",
			TotalFiles: prog.TotalFiles,
			Percent:    prog.Percent,
			Message:    prog.Message,
		}
	}

	// Check if indexed (has meta entry)
	metaKey := repoMetaKey(projectID, repoID)
	c.meta.mu.RLock()
	defer c.meta.mu.RUnlock()
	for _, e := range c.meta.docs {
		if e.DocID == metaKey {
			colName := repoCollectionName(projectID, repoID)
			col := c.db.GetCollection(colName, c.embed)
			chunks := 0
			if col != nil {
				chunks = col.Count()
			}
			return RepoIndexStatus{
				State:     "indexed",
				DoneFiles: chunks,
				Percent:   100,
				IndexedAt: e.CreatedAt,
				Message:   fmt.Sprintf("%d chunks indexed", chunks),
			}
		}
	}

	return RepoIndexStatus{State: "none"}
}

// indexedRepoIDs returns the repo IDs that have been indexed for a project.
func (c *Client) indexedRepoIDs(projectID string) []string {
	prefix := "repo:" + projectID + ":"
	c.meta.mu.RLock()
	defer c.meta.mu.RUnlock()
	var ids []string
	for _, e := range c.meta.docs {
		if strings.HasPrefix(e.DocID, prefix) && e.SourceType == "code" {
			rid := strings.TrimPrefix(e.DocID, prefix)
			ids = append(ids, rid)
		}
	}
	return ids
}

// HasRepoIndex returns true if the repo has been indexed.
func (c *Client) HasRepoIndex(projectID, repoID string) bool {
	metaKey := repoMetaKey(projectID, repoID)
	c.meta.mu.RLock()
	defer c.meta.mu.RUnlock()
	for _, e := range c.meta.docs {
		if e.DocID == metaKey {
			return true
		}
	}
	return false
}

// sortCodeChunks sorts by score descending.
func sortCodeChunks(chunks []CodeChunk) {
	for i := 1; i < len(chunks); i++ {
		for j := i; j > 0 && chunks[j].Score > chunks[j-1].Score; j-- {
			chunks[j], chunks[j-1] = chunks[j-1], chunks[j]
		}
	}
}

func (ms *metaStore) save() error {
	ms.mu.RLock()
	data, err := json.MarshalIndent(ms.docs, "", "  ")
	ms.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(ms.path, data, 0644)
}

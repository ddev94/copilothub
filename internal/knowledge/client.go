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

	return &Client{
		db:     db,
		dbPath: dbPath,
		embed:  embed,
		meta:   ms,
	}, nil
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

	chunks := splitText(text, 800, 100)
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

func (ms *metaStore) save() error {
	ms.mu.RLock()
	data, err := json.MarshalIndent(ms.docs, "", "  ")
	ms.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(ms.path, data, 0644)
}

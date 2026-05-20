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
	ProjectID  string `json:"projectId"`
	DocID      string `json:"docId"`
	Name       string `json:"name"`
	SourceFile string `json:"sourceFile"`
	CreatedAt  string `json:"createdAt"`
}

// Document is stored document metadata (without chunks).
type Document struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourceFile string `json:"sourceFile"`
	CreatedAt  string `json:"createdAt"`
}

// Chunk is a retrieved text segment with similarity score.
type Chunk struct {
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// NewClient opens (or creates) a knowledge store at storeDir.
// Requires Ollama running at localhost:11434 with the all-minilm model pulled.
func NewClient(storeDir string) (*Client, error) {
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

	embed, err := newEmbeddingFunc(defaultModelsDir())
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
	chunks := make([]Chunk, len(results))
	for i, r := range results {
		chunks[i] = Chunk{Content: r.Content, Score: float64(r.Similarity)}
	}
	return chunks, nil
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

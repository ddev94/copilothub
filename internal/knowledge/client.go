package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

type Document struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourceFile string `json:"sourceFile"`
	CreatedAt  string `json:"createdAt"`
}

type Chunk struct {
	Content string `json:"content"`
	Score   float64 `json:"score"`
}

type retrieveResponse struct {
	Chunks []Chunk `json:"chunks"`
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Ingest(ctx context.Context, projectID, filePath, originalName, contentType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("projectId", projectID); err != nil {
		return err
	}
	if err := w.WriteField("fileName", originalName); err != nil {
		return err
	}
	part, err := w.CreateFormFile("file", filepath.Base(originalName))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ingest", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if contentType != "" {
		req.Header.Set("X-Source-Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("knowledge ingest failed: %s", strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *Client) ListDocuments(ctx context.Context, projectID string) ([]Document, error) {
	u, _ := url.Parse(c.baseURL + "/documents")
	q := u.Query()
	q.Set("projectId", projectID)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("knowledge list failed")
	}
	var out struct{ Documents []Document `json:"documents"` }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Documents, nil
}

func (c *Client) DeleteDocument(ctx context.Context, projectID, docID string) error {
	u, _ := url.Parse(c.baseURL + "/documents/" + url.PathEscape(docID))
	q := u.Query()
	q.Set("projectId", projectID)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("knowledge delete failed")
	}
	return nil
}

func (c *Client) Retrieve(ctx context.Context, projectID, query string, topK int) ([]Chunk, error) {
	payload := map[string]any{"projectId": projectID, "query": query, "topK": topK}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/retrieve", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("knowledge retrieve failed")
	}
	var out retrieveResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Chunks, nil
}

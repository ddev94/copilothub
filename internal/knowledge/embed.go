package knowledge

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/nlpodyssey/cybertron/pkg/models/bert"
	"github.com/nlpodyssey/cybertron/pkg/tasks"
	"github.com/nlpodyssey/cybertron/pkg/tasks/textencoding"
	chromem "github.com/philippgille/chromem-go"
	"google.golang.org/genai"
)

const (
	embeddingModelName  = "sentence-transformers/all-MiniLM-L6-v2"
	estimatedModelBytes = int64(100 * 1024 * 1024) // ~100MB
)

// EmbeddingConfig selects the embedding backend and its credentials.
type EmbeddingConfig struct {
	Provider string // "cybertron" (default) | "openai" | "ollama" | "google"
	Model    string
	Key      string // API key for openai or google
	URL      string // base URL for ollama
}

// NewEmbeddingFunc creates an embedding function for the given config.
// For cybertron, download progress is broadcast via EmbedProgress.
func NewEmbeddingFunc(cfg EmbeddingConfig, modelsDir string) (chromem.EmbeddingFunc, error) {
	switch cfg.Provider {
	case "openai":
		return newOpenAIEmbeddingFunc(cfg.Key, cfg.Model)
	case "ollama":
		return newOllamaEmbeddingFunc(cfg.Model, cfg.URL)
	case "google":
		return newGoogleEmbeddingFunc(cfg.Key, cfg.Model)
	default: // "cybertron" or ""
		return newCybertronEmbeddingFunc(modelsDir)
	}
}

func newOpenAIEmbeddingFunc(apiKey, model string) (chromem.EmbeddingFunc, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("embeddingKey required for OpenAI embeddings")
	}
	m := chromem.EmbeddingModelOpenAI3Small
	if model != "" {
		m = chromem.EmbeddingModelOpenAI(model)
	}
	return chromem.NewEmbeddingFuncOpenAI(apiKey, m), nil
}

func newOllamaEmbeddingFunc(model, baseURL string) (chromem.EmbeddingFunc, error) {
	if model == "" {
		model = "nomic-embed-text"
	}
	return chromem.NewEmbeddingFuncOllama(model, baseURL), nil
}

func newGoogleEmbeddingFunc(apiKey, model string) (chromem.EmbeddingFunc, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("embeddingKey required for Google embeddings")
	}
	if model == "" {
		model = "gemini-embedding-2"
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("google genai client: %w", err)
	}
	return func(ctx context.Context, text string) ([]float32, error) {
		contents := []*genai.Content{
			genai.NewContentFromText(text, genai.RoleUser),
		}
		result, err := client.Models.EmbedContent(ctx, model, contents, nil)
		if err != nil {
			return nil, fmt.Errorf("google embed: %w", err)
		}
		if len(result.Embeddings) == 0 {
			return nil, fmt.Errorf("google embed: no embeddings returned")
		}
		return result.Embeddings[0].Values, nil
	}, nil
}

// newCybertronEmbeddingFunc loads all-MiniLM-L6-v2 and broadcasts download progress.
func newCybertronEmbeddingFunc(modelsDir string) (chromem.EmbeddingFunc, error) {
	modelDir := filepath.Join(modelsDir, filepath.FromSlash("sentence-transformers/all-MiniLM-L6-v2"))
	if !isModelReady(modelDir) {
		EmbedProgress.Set(ModelProgress{
			State:   ModelStateDownloading,
			Message: "Đang tải model all-MiniLM-L6-v2 (~100MB)...",
			Total:   estimatedModelBytes,
		})
		ctx, cancel := context.WithCancel(context.Background())
		go monitorModelDir(ctx, modelDir)
		defer cancel()
	} else {
		EmbedProgress.Set(ModelProgress{State: ModelStateReady, Percent: 100, Message: "Model sẵn sàng"})
	}

	fn, err := loadCybertronModel(modelsDir)
	if err != nil {
		EmbedProgress.Set(ModelProgress{State: ModelStateError, Message: err.Error()})
		return nil, err
	}

	EmbedProgress.Set(ModelProgress{State: ModelStateReady, Percent: 100, Message: "Model sẵn sàng"})
	return fn, nil
}

func isModelReady(modelDir string) bool {
	found := false
	_ = filepath.WalkDir(modelDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || found {
			return nil
		}
		if filepath.Ext(path) == ".onnx" {
			info, _ := d.Info()
			if info != nil && info.Size() > 10*1024*1024 {
				found = true
			}
		}
		return nil
	})
	return found
}

func monitorModelDir(ctx context.Context, modelDir string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			size := dirSize(modelDir)
			pct := 0
			if estimatedModelBytes > 0 {
				pct = int(float64(size) / float64(estimatedModelBytes) * 100)
				if pct > 99 {
					pct = 99
				}
			}
			EmbedProgress.Set(ModelProgress{
				State:   ModelStateDownloading,
				Message: fmt.Sprintf("Đang tải... %s / ~%s", formatBytes(size), formatBytes(estimatedModelBytes)),
				Bytes:   size,
				Total:   estimatedModelBytes,
				Percent: pct,
			})
		}
	}
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func formatBytes(b int64) string {
	const mb = 1024 * 1024
	if b < mb {
		return fmt.Sprintf("%dKB", b/1024)
	}
	return fmt.Sprintf("%dMB", b/mb)
}

// loadCybertronModel downloads (if missing) and loads all-MiniLM-L6-v2 via cybertron.
func loadCybertronModel(modelsDir string) (chromem.EmbeddingFunc, error) {
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return nil, fmt.Errorf("models dir: %w", err)
	}

	m, err := tasks.Load[textencoding.Interface](&tasks.Config{
		ModelsDir:           modelsDir,
		ModelName:           embeddingModelName,
		DownloadPolicy:      tasks.DownloadMissing,
		ConversionPolicy:    tasks.ConvertMissing,
		ConversionPrecision: tasks.F32,
	})
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", embeddingModelName, err)
	}

	return func(ctx context.Context, text string) ([]float32, error) {
		result, err := m.Encode(ctx, text, int(bert.MeanPooling))
		if err != nil {
			return nil, err
		}
		f64 := result.Vector.Data().F64()
		emb := make([]float32, len(f64))
		for i, v := range f64 {
			emb[i] = float32(v)
		}
		return emb, nil
	}, nil
}

// defaultModelsDir returns the user-level cache directory for cybertron models.
func defaultModelsDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "copilothub", "models")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".copilothub", "models")
}

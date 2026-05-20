package knowledge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nlpodyssey/cybertron/pkg/models/bert"
	"github.com/nlpodyssey/cybertron/pkg/tasks"
	"github.com/nlpodyssey/cybertron/pkg/tasks/textencoding"
	chromem "github.com/philippgille/chromem-go"
)

const embeddingModelName = "sentence-transformers/all-MiniLM-L6-v2"

// newEmbeddingFunc loads all-MiniLM-L6-v2 via cybertron (pure Go, no Ollama).
// On first call the model is downloaded from HuggingFace Hub (~90 MB) and
// converted; subsequent calls load it from modelsDir in milliseconds.
func newEmbeddingFunc(modelsDir string) (chromem.EmbeddingFunc, error) {
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

// modelsDir returns a user-level cache directory shared across all repos.
func defaultModelsDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "copilothub", "models")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".copilothub", "models")
}

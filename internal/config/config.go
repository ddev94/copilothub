package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	dirName  = ".spec-clarify"
	fileName = "config.json"
)

type Config struct {
	AI        AIConfig        `json:"ai"`
	Knowledge KnowledgeConfig `json:"knowledge"`
}

type AIConfig struct {
	Provider string `json:"provider"` // "copilot" | "openai" | "anthropic"
	Token    string `json:"token"`    // GitHub token (copilot) or API key (openai/anthropic)
	Model    string `json:"model"`
	BaseURL  string `json:"baseURL"` // custom OpenAI-compatible base URL
}

type KnowledgeConfig struct {
	Enabled           bool   `json:"enabled"`
	TopK              int    `json:"topK"`
	EmbeddingProvider string `json:"embeddingProvider"` // "cybertron" | "openai" | "ollama"
	EmbeddingModel    string `json:"embeddingModel"`
	EmbeddingKey      string `json:"embeddingKey"`
	EmbeddingURL      string `json:"embeddingURL"`
}

type Store struct {
	path string
}

func NewStore(repoPath string) *Store {
	return &Store{path: filepath.Join(repoPath, dirName, fileName)}
}

func (s *Store) Load() (*Config, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s.defaults(), nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		cfg.AI.Token = t
	}
	if cfg.Knowledge.TopK <= 0 {
		cfg.Knowledge.TopK = 6
	}
	return &cfg, nil
}

func (s *Store) Save(cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

func (s *Store) defaults() *Config {
	return &Config{
		AI: AIConfig{
			Provider: "copilot",
			Token:    os.Getenv("GITHUB_TOKEN"),
		},
		Knowledge: KnowledgeConfig{
			Enabled:           true,
			TopK:              6,
			EmbeddingProvider: "cybertron",
		},
	}
}

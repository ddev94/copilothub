package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	dirName  = ".spec-designer"
	fileName = "config.json"
)

type Config struct {
	AI AIConfig `json:"ai"`
}

type AIConfig struct {
	Token string `json:"token"` // optional override; uses gh CLI auth if empty
	Model string `json:"model"`
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
	return &Config{AI: AIConfig{Token: os.Getenv("GITHUB_TOKEN")}}
}

package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Project represents a registered project.
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

// Store manages the list of projects stored in ~/.copilothub/projects.json.
type Store struct {
	baseDir string
}

// NewStore creates a project store rooted at the given base directory.
func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

// BaseDir returns the root data directory (~/.copilothub).
func (s *Store) BaseDir() string { return s.baseDir }

// ProjectDir returns the data directory for a specific project.
func (s *Store) ProjectDir(projectID string) string {
	return filepath.Join(s.baseDir, "projects", projectID)
}

func (s *Store) filePath() string {
	return filepath.Join(s.baseDir, "projects.json")
}

// List returns all registered projects.
func (s *Store) List() ([]Project, error) {
	data, err := os.ReadFile(s.filePath())
	if os.IsNotExist(err) {
		return []Project{}, nil
	}
	if err != nil {
		return nil, err
	}
	var projects []Project
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// Get returns a single project by ID.
func (s *Store) Get(id string) (*Project, error) {
	projects, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("project not found: %s", id)
}

// Create registers a new project.
func (s *Store) Create(name string) (*Project, error) {
	projects, err := s.List()
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()[:8]
	p := Project{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Create project data directory
	if err := os.MkdirAll(s.ProjectDir(id), 0755); err != nil {
		return nil, err
	}

	projects = append(projects, p)
	return &p, s.save(projects)
}

// Delete removes a project from the list. Does not delete data on disk for safety.
func (s *Store) Delete(id string) error {
	projects, err := s.List()
	if err != nil {
		return err
	}

	var filtered []Project
	for _, p := range projects {
		if p.ID != id {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) == len(projects) {
		return fmt.Errorf("project not found: %s", id)
	}

	return s.save(filtered)
}

func (s *Store) save(projects []Project) error {
	if err := os.MkdirAll(filepath.Dir(s.filePath()), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath(), data, 0644)
}

// DefaultBaseDir returns the default data directory (~/.copilothub).
func DefaultBaseDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".copilothub")
}

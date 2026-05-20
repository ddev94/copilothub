package project

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Project represents a registered project.
type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CreatedAt  string `json:"createdAt"`
	RepoURL    string `json:"repoURL,omitempty"`
	RepoBranch string `json:"repoBranch,omitempty"`
	RepoCloned bool   `json:"repoCloned,omitempty"`
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

// SourceDir returns the cloned source directory for a project.
func (s *Store) SourceDir(projectID string) string {
	return filepath.Join(s.ProjectDir(projectID), "source")
}

// Update replaces the project with the given ID.
func (s *Store) Update(p *Project) error {
	projects, err := s.List()
	if err != nil {
		return err
	}
	found := false
	for i, existing := range projects {
		if existing.ID == p.ID {
			projects[i] = *p
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("project not found: %s", p.ID)
	}
	return s.save(projects)
}

// ConnectRepo clones a GitHub repository into the project's source directory.
func (s *Store) ConnectRepo(projectID, repoURL, branch string) error {
	p, err := s.Get(projectID)
	if err != nil {
		return err
	}

	srcDir := s.SourceDir(projectID)

	// Remove existing source if any
	_ = os.RemoveAll(srcDir)

	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repoURL, srcDir)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	p.RepoURL = repoURL
	p.RepoBranch = branch
	p.RepoCloned = true
	return s.Update(p)
}

// DisconnectRepo removes the cloned source and clears repo info.
func (s *Store) DisconnectRepo(projectID string) error {
	p, err := s.Get(projectID)
	if err != nil {
		return err
	}

	srcDir := s.SourceDir(projectID)
	_ = os.RemoveAll(srcDir)

	p.RepoURL = ""
	p.RepoBranch = ""
	p.RepoCloned = false
	return s.Update(p)
}

// DefaultBaseDir returns the default data directory (~/.copilothub).
func DefaultBaseDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".copilothub")
}

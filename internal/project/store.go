package project

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Repository represents a single connected git repository within a project.
type Repository struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	RepoURL    string `json:"repoURL"`
	RepoBranch string `json:"repoBranch,omitempty"`
	RepoCloned bool   `json:"repoCloned"`
}

// Project represents a registered project.
type Project struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	CreatedAt    string       `json:"createdAt"`
	Repositories []Repository `json:"repositories,omitempty"`

	// Deprecated legacy single-repo fields — kept for JSON migration only.
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

// RepoSourceDir returns the clone directory for a specific repository.
// The special ID "legacy" maps to the old single-repo "source/" path.
func (s *Store) RepoSourceDir(projectID, repoID string) string {
	if repoID == "legacy" {
		return filepath.Join(s.ProjectDir(projectID), "source")
	}
	return filepath.Join(s.ProjectDir(projectID), "repos", repoID)
}

// SourceDir returns the source directory of the first connected repository.
// Kept for backward compatibility with features that predate multi-repo support.
func (s *Store) SourceDir(projectID string) string {
	p, err := s.Get(projectID)
	if err != nil || len(p.Repositories) == 0 {
		return filepath.Join(s.ProjectDir(projectID), "source")
	}
	return s.RepoSourceDir(projectID, p.Repositories[0].ID)
}

// SourceDirsForRepos returns clone directories for the given repo IDs.
// If repoIDs is empty, all cloned repos are returned.
func (s *Store) SourceDirsForRepos(projectID string, repoIDs []string) []string {
	p, err := s.Get(projectID)
	if err != nil {
		return nil
	}
	idSet := make(map[string]bool, len(repoIDs))
	for _, id := range repoIDs {
		idSet[id] = true
	}
	var dirs []string
	for _, r := range p.Repositories {
		if !r.RepoCloned {
			continue
		}
		if len(repoIDs) == 0 || idSet[r.ID] {
			dirs = append(dirs, s.RepoSourceDir(projectID, r.ID))
		}
	}
	return dirs
}

// migrateProject converts the legacy single-repo fields into the Repositories slice.
func migrateProject(p *Project) {
	if p.RepoURL != "" && len(p.Repositories) == 0 {
		name := filepath.Base(strings.TrimSuffix(p.RepoURL, ".git"))
		p.Repositories = []Repository{
			{
				ID:         "legacy",
				Name:       name,
				RepoURL:    p.RepoURL,
				RepoBranch: p.RepoBranch,
				RepoCloned: p.RepoCloned,
			},
		}
		p.RepoURL = ""
		p.RepoBranch = ""
		p.RepoCloned = false
	}
}

// List returns all registered projects, migrating legacy single-repo data if present.
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
	for i := range projects {
		migrateProject(&projects[i])
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

// AddRepo clones a repository and adds it to the project's repository list.
func (s *Store) AddRepo(projectID, repoURL, branch, name string) (*Repository, error) {
	p, err := s.Get(projectID)
	if err != nil {
		return nil, err
	}

	repoID := uuid.New().String()[:8]
	repoDir := s.RepoSourceDir(projectID, repoID)
	_ = os.RemoveAll(repoDir)

	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repoURL, repoDir)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git clone failed: %w", err)
	}

	repoName := name
	if repoName == "" {
		repoName = filepath.Base(strings.TrimSuffix(repoURL, ".git"))
	}

	r := Repository{
		ID:         repoID,
		Name:       repoName,
		RepoURL:    repoURL,
		RepoBranch: branch,
		RepoCloned: true,
	}
	p.Repositories = append(p.Repositories, r)
	return &r, s.Update(p)
}

// RemoveRepo disconnects and removes a repository from the project.
func (s *Store) RemoveRepo(projectID, repoID string) error {
	p, err := s.Get(projectID)
	if err != nil {
		return err
	}

	var filtered []Repository
	found := false
	for _, r := range p.Repositories {
		if r.ID == repoID {
			found = true
		} else {
			filtered = append(filtered, r)
		}
	}
	if !found {
		return fmt.Errorf("repository not found: %s", repoID)
	}

	_ = os.RemoveAll(s.RepoSourceDir(projectID, repoID))

	p.Repositories = filtered
	return s.Update(p)
}

// ChangeRepoBranch re-clones a repository on the specified branch.
func (s *Store) ChangeRepoBranch(projectID, repoID, branch string) error {
	p, err := s.Get(projectID)
	if err != nil {
		return err
	}

	var target *Repository
	for i := range p.Repositories {
		if p.Repositories[i].ID == repoID {
			target = &p.Repositories[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("repository not found: %s", repoID)
	}

	repoDir := s.RepoSourceDir(projectID, repoID)
	_ = os.RemoveAll(repoDir)

	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, target.RepoURL, repoDir)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	target.RepoBranch = branch
	return s.Update(p)
}

// ConnectRepo clones a repository into the project's legacy source directory.
// Deprecated: use AddRepo for new projects.
func (s *Store) ConnectRepo(projectID, repoURL, branch string) error {
	_, err := s.AddRepo(projectID, repoURL, branch, "")
	return err
}

// DisconnectRepo removes all repositories from a project.
// Deprecated: use RemoveRepo for targeted removal.
func (s *Store) DisconnectRepo(projectID string) error {
	p, err := s.Get(projectID)
	if err != nil {
		return err
	}
	for _, r := range p.Repositories {
		_ = os.RemoveAll(s.RepoSourceDir(projectID, r.ID))
	}
	// Also clean up old-style source dir
	_ = os.RemoveAll(filepath.Join(s.ProjectDir(projectID), "source"))
	p.Repositories = nil
	return s.Update(p)
}

// DefaultBaseDir returns the default data directory (~/.copilothub).
func DefaultBaseDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".copilothub")
}

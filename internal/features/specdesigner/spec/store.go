package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
)

const (
	dirName  = ".spec-designer"
	specsDir = "specs"
)

type Store struct {
	repoPath string
}

func NewStore(repoPath string) *Store {
	s := &Store{repoPath: repoPath}
	s.migrate()
	return s
}

func (s *Store) dir() string {
	return filepath.Join(s.repoPath, dirName, specsDir)
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir(), id+".json")
}

// migrate moves the legacy single spec.json into the multi-spec directory.
func (s *Store) migrate() {
	oldPath := filepath.Join(s.repoPath, dirName, "spec.json")
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return
	}
	var sp Spec
	if err := json.Unmarshal(data, &sp); err != nil {
		return
	}
	if sp.ID == "" {
		sp.ID = uuid.NewString()
	}
	if err := os.MkdirAll(s.dir(), 0755); err != nil {
		return
	}
	out, err := json.MarshalIndent(sp, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(s.path(sp.ID), out, 0644); err != nil {
		return
	}
	os.Remove(oldPath) //nolint:errcheck
}

func (s *Store) List() ([]SpecMeta, error) {
	entries, err := os.ReadDir(s.dir())
	if os.IsNotExist(err) {
		return []SpecMeta{}, nil
	}
	if err != nil {
		return nil, err
	}
	var metas []SpecMeta
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir(), e.Name()))
		if err != nil {
			continue
		}
		var sp Spec
		if err := json.Unmarshal(data, &sp); err != nil {
			continue
		}
		metas = append(metas, SpecMeta{
			ID:        sp.ID,
			Title:     sp.Title,
			Version:   sp.Version,
			UpdatedAt: sp.UpdatedAt,
		})
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})
	return metas, nil
}

func (s *Store) Load(id string) (*Spec, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, err
	}
	var sp Spec
	if err := json.Unmarshal(data, &sp); err != nil {
		return nil, err
	}
	return &sp, nil
}

func (s *Store) Save(sp *Spec) error {
	sp.UpdatedAt = time.Now()
	if err := os.MkdirAll(s.dir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(sp.ID), data, 0644)
}

func (s *Store) Delete(id string) error {
	return os.Remove(s.path(id))
}

func (s *Store) NewDefault() *Spec {
	now := time.Now()
	return &Spec{
		ID:          uuid.NewString(),
		Title:       "User Stories",
		Version:     "1.0.0",
		CreatedAt:   now,
		UpdatedAt:   now,
		Requirement: "",
		UserStories: []UserStory{},
	}
}

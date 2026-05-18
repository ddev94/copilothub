package repo

import (
	"os"
	"path/filepath"
	"strings"
)

type Info struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	TechStack []string `json:"techStack"`
	FileTree  []Node   `json:"fileTree"`
}

type Node struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"isDir"`
	Children []Node `json:"children,omitempty"`
}

type Scanner struct {
	root string
}

func NewScanner(root string) *Scanner {
	return &Scanner{root: root}
}

func (s *Scanner) Scan() (*Info, error) {
	tree, err := s.buildTree(s.root, 0)
	if err != nil {
		return nil, err
	}
	return &Info{
		Name:      filepath.Base(s.root),
		Path:      s.root,
		TechStack: s.detectStack(),
		FileTree:  tree,
	}, nil
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	".spec-designer": true, "dist": true, ".next": true,
	"__pycache__": true, ".venv": true, "build": true, "target": true,
}

func (s *Scanner) buildTree(dir string, depth int) ([]Node, error) {
	if depth > 4 {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var nodes []Node
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if skipDirs[e.Name()] {
			continue
		}
		node := Node{
			Name:  e.Name(),
			Path:  filepath.Join(dir, e.Name()),
			IsDir: e.IsDir(),
		}
		if e.IsDir() {
			node.Children, _ = s.buildTree(node.Path, depth+1)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

var stackMarkers = map[string][]string{
	"Go":         {"go.mod"},
	"Node.js":    {"package.json"},
	"Python":     {"requirements.txt", "pyproject.toml", "setup.py"},
	"Ruby":       {"Gemfile"},
	"Java":       {"pom.xml", "build.gradle"},
	"Rust":       {"Cargo.toml"},
	"PHP":        {"composer.json"},
	"Vue.js":     {"vue.config.js"},
	"React":      {"next.config.js", "next.config.ts"},
	"Docker":     {"Dockerfile"},
	"Kubernetes": {"k8s", "kubernetes"},
}

func (s *Scanner) detectStack() []string {
	var stack []string
	for tech, markers := range stackMarkers {
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(s.root, m)); err == nil {
				stack = append(stack, tech)
				break
			}
		}
	}
	return stack
}

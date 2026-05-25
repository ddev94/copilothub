package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"copilothub/internal/ai"
)

// SourceScanTools returns tools for reading and searching source code in baseDirs.
// All tools are path-constrained to the given directories.
func SourceScanTools(baseDirs []string) []ai.Tool {
	return []ai.Tool{
		readFileTool(baseDirs),
		listDirTool(baseDirs),
		searchCodeTool(baseDirs),
		findFilesTool(baseDirs),
	}
}

// resolveSafe returns the cleaned absolute path only if it falls within one of baseDirs.
func resolveSafe(path string, baseDirs []string) (string, error) {
	if !filepath.IsAbs(path) {
		// Try resolving relative path against each base dir
		for _, base := range baseDirs {
			candidate := filepath.Clean(filepath.Join(base, path))
			if strings.HasPrefix(candidate, filepath.Clean(base)) {
				return candidate, nil
			}
		}
		return "", fmt.Errorf("cannot resolve relative path %q against any allowed directory", path)
	}
	clean := filepath.Clean(path)
	for _, base := range baseDirs {
		cleanBase := filepath.Clean(base)
		if clean == cleanBase || strings.HasPrefix(clean, cleanBase+string(filepath.Separator)) {
			return clean, nil
		}
	}
	return "", fmt.Errorf("path %q is outside allowed directories", path)
}

// skipDir returns true for directories that should be excluded from source scanning.
func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".cache", "dist", "build",
		"__pycache__", ".idea", ".vscode", "target", "out", ".next", ".nuxt":
		return true
	}
	return false
}

func readFileTool(baseDirs []string) ai.Tool {
	return ai.Tool{
		Name:        "read_file",
		Description: "Read the full contents of a source file with line numbers.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute path or path relative to the repository root",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "", fmt.Errorf("path is required")
			}
			resolved, err := resolveSafe(path, baseDirs)
			if err != nil {
				return "", err
			}
			data, err := os.ReadFile(resolved)
			if err != nil {
				return "", fmt.Errorf("read %s: %w", path, err)
			}
			lines := strings.Split(string(data), "\n")
			var sb strings.Builder
			for i, line := range lines {
				fmt.Fprintf(&sb, "%4d | %s\n", i+1, line)
			}
			return sb.String(), nil
		},
	}
}

func listDirTool(baseDirs []string) ai.Tool {
	return ai.Tool{
		Name:        "list_directory",
		Description: "List files and subdirectories in a directory (one level deep).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path (absolute or relative to repository root)",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "", fmt.Errorf("path is required")
			}
			resolved, err := resolveSafe(path, baseDirs)
			if err != nil {
				return "", err
			}
			entries, err := os.ReadDir(resolved)
			if err != nil {
				return "", fmt.Errorf("list %s: %w", path, err)
			}
			var sb strings.Builder
			for _, e := range entries {
				if e.IsDir() {
					fmt.Fprintf(&sb, "[dir]  %s/\n", e.Name())
				} else {
					info, _ := e.Info()
					size := int64(0)
					if info != nil {
						size = info.Size()
					}
					fmt.Fprintf(&sb, "[file] %s (%d bytes)\n", e.Name(), size)
				}
			}
			if sb.Len() == 0 {
				return "(empty directory)", nil
			}
			return sb.String(), nil
		},
	}
}

func searchCodeTool(baseDirs []string) ai.Tool {
	return ai.Tool{
		Name:        "search_code",
		Description: "Search for a text pattern across source files. Returns matching lines with file:line references (max 50 matches).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Text to search for (case-insensitive substring)",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory to search in (optional, defaults to all repository roots)",
				},
				"glob": map[string]any{
					"type":        "string",
					"description": "File name glob to restrict search, e.g. '*.go', '*.ts' (optional)",
				},
			},
			"required": []string{"pattern"},
		},
		Handler: func(args map[string]any) (string, error) {
			pattern, _ := args["pattern"].(string)
			if pattern == "" {
				return "", fmt.Errorf("pattern is required")
			}
			searchPath, _ := args["path"].(string)
			glob, _ := args["glob"].(string)
			needle := strings.ToLower(pattern)

			roots := baseDirs
			if searchPath != "" {
				resolved, err := resolveSafe(searchPath, baseDirs)
				if err != nil {
					return "", err
				}
				roots = []string{resolved}
			}

			const maxMatches = 50
			var sb strings.Builder
			matchCount := 0

			for _, root := range roots {
				if matchCount >= maxMatches {
					break
				}
				filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error { //nolint:errcheck
					if err != nil {
						return nil
					}
					if d.IsDir() {
						if skipDir(d.Name()) {
							return filepath.SkipDir
						}
						return nil
					}
					if glob != "" {
						if matched, _ := filepath.Match(glob, d.Name()); !matched {
							return nil
						}
					}
					data, err := os.ReadFile(p)
					if err != nil {
						return nil
					}
					lines := strings.Split(string(data), "\n")
					relPath, _ := filepath.Rel(root, p)
					for i, line := range lines {
						if strings.Contains(strings.ToLower(line), needle) {
							fmt.Fprintf(&sb, "%s:%d: %s\n", relPath, i+1, strings.TrimSpace(line))
							matchCount++
							if matchCount >= maxMatches {
								sb.WriteString("... (truncated at 50 matches)\n")
								return filepath.SkipAll
							}
						}
					}
					return nil
				})
			}

			if sb.Len() == 0 {
				return fmt.Sprintf("No matches found for %q", pattern), nil
			}
			return sb.String(), nil
		},
	}
}

func findFilesTool(baseDirs []string) ai.Tool {
	return ai.Tool{
		Name:        "find_files",
		Description: "Find files by name glob pattern within the repository.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "File name glob, e.g. '*.go', 'handler.go', '*_test.go'",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory to search in (optional, defaults to all repository roots)",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(args map[string]any) (string, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return "", fmt.Errorf("name is required")
			}
			searchPath, _ := args["path"].(string)

			roots := baseDirs
			if searchPath != "" {
				resolved, err := resolveSafe(searchPath, baseDirs)
				if err != nil {
					return "", err
				}
				roots = []string{resolved}
			}

			var matches []string
			for _, root := range roots {
				filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error { //nolint:errcheck
					if err != nil {
						return nil
					}
					if d.IsDir() {
						if skipDir(d.Name()) {
							return filepath.SkipDir
						}
						return nil
					}
					if matched, _ := filepath.Match(name, d.Name()); matched {
						rel, _ := filepath.Rel(root, p)
						matches = append(matches, rel)
					}
					return nil
				})
			}

			if len(matches) == 0 {
				return fmt.Sprintf("No files found matching %q", name), nil
			}
			return strings.Join(matches, "\n"), nil
		},
	}
}

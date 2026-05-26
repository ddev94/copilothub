package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/langchaingo/textsplitter"
)

// CodeChunk represents a chunk of source code with file metadata.
type CodeChunk struct {
	Content     string   `json:"content"`
	FilePath    string   `json:"filePath"`
	Score       float64  `json:"score,omitempty"`
	Language    string   `json:"language,omitempty"`
	SymbolNames []string `json:"symbolNames,omitempty"`
	ChunkType   string   `json:"chunkType,omitempty"` // "content" | "file_summary"
	StartLine   int      `json:"startLine,omitempty"`
	EndLine     int      `json:"endLine,omitempty"`
}

// codeChunkSize and overlap tuned for source code.
const (
	codeChunkSize    = 1000
	codeChunkOverlap = 150
)

// supportedCodeExts lists file extensions to index.
var supportedCodeExts = map[string]bool{
	".go": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
	".py": true, ".java": true, ".kt": true, ".kts": true,
	".rb": true, ".rs": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
	".cs": true, ".swift": true, ".scala": true, ".php": true,
	".vue": true, ".svelte": true,
	".sql": true, ".graphql": true, ".gql": true,
	".yaml": true, ".yml": true, ".toml": true,
	".sh": true, ".bash": true, ".zsh": true,
	".md": true, ".markdown": true,
	".json": true, ".proto": true,
	".dart": true, ".ex": true, ".exs": true,
	".lua": true, ".r": true, ".R": true,
	".tf": true, ".hcl": true,
	".dockerfile": true,
}

// skipCodeDirs lists directories to exclude from code indexing.
var skipCodeDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, ".cache": true,
	"dist": true, "build": true, "__pycache__": true, ".idea": true,
	".vscode": true, "target": true, "out": true, ".next": true, ".nuxt": true,
	"coverage": true, ".tox": true, ".mypy_cache": true, ".pytest_cache": true,
	"venv": true, ".venv": true, "env": true, ".env": true,
	".terraform": true, ".gradle": true, "bin": true, "obj": true,
	"pods": true, "Pods": true, ".svn": true, ".hg": true,
}

// maxFileSize limits individual files to 512KB to avoid embedding huge generated files.
const maxFileSize = 512 * 1024

// extToLang maps file extension to a human-readable language name stored as chunk metadata.
var extToLang = map[string]string{
	".go": "Go", ".js": "JavaScript", ".ts": "TypeScript",
	".jsx": "JSX", ".tsx": "TSX", ".py": "Python",
	".java": "Java", ".kt": "Kotlin", ".kts": "Kotlin",
	".rb": "Ruby", ".rs": "Rust", ".c": "C", ".cpp": "C++",
	".h": "C", ".hpp": "C++", ".cs": "C#", ".swift": "Swift",
	".scala": "Scala", ".php": "PHP", ".vue": "Vue", ".svelte": "Svelte",
	".sql": "SQL", ".graphql": "GraphQL", ".gql": "GraphQL",
	".yaml": "YAML", ".yml": "YAML", ".toml": "TOML",
	".sh": "Shell", ".bash": "Shell", ".zsh": "Shell",
	".md": "Markdown", ".markdown": "Markdown",
	".json": "JSON", ".proto": "Protobuf",
	".dart": "Dart", ".ex": "Elixir", ".exs": "Elixir",
	".lua": "Lua", ".r": "R", ".R": "R",
	".tf": "Terraform", ".hcl": "HCL", ".dockerfile": "Dockerfile",
}

// WalkAndChunkRepo walks a repo directory and returns code chunks with metadata.
func WalkAndChunkRepo(repoDir string, onProgress func(filePath string)) ([]CodeChunk, error) {
	repoDir = filepath.Clean(repoDir)
	var allChunks []CodeChunk

	err := filepath.WalkDir(repoDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			if skipCodeDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		// Handle Dockerfile (no extension)
		if ext == "" && strings.EqualFold(d.Name(), "Dockerfile") {
			ext = ".dockerfile"
		}
		if !supportedCodeExts[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil || info.Size() > maxFileSize || info.Size() == 0 {
			return nil
		}

		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(repoDir, p)
		if onProgress != nil {
			onProgress(relPath)
		}

		lang := extToLang[ext]
		chunks := chunkCodeFile(string(data), relPath, ext, lang)

		// Generate a file-level summary chunk for hierarchical (two-tier) retrieval.
		allSyms := extractSymbols(string(data), ext)
		var sb strings.Builder
		fmt.Fprintf(&sb, "File: %s [%s]\n", relPath, lang)
		if len(allSyms) > 0 {
			fmt.Fprintf(&sb, "Defines: %s\n", strings.Join(allSyms, ", "))
		}
		allChunks = append(allChunks, CodeChunk{
			Content:     sb.String(),
			FilePath:    relPath,
			Language:    lang,
			SymbolNames: allSyms,
			ChunkType:   "file_summary",
		})
		allChunks = append(allChunks, chunks...)
		return nil
	})

	return allChunks, err
}

// chunkCodeFile splits a single source file into chunks using langchaingo's
// RecursiveCharacter splitter with language-aware separators.
func chunkCodeFile(content, filePath, ext, language string) []CodeChunk {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	totalLines := strings.Count(content, "\n") + 1
	header := fmt.Sprintf("// File: %s\n", filePath)

	// For small files, use a single chunk
	if len(content) <= codeChunkSize {
		return []CodeChunk{{
			Content:   header + content,
			FilePath:  filePath,
			Language:  language,
			ChunkType: "content",
			StartLine: 1,
			EndLine:   totalLines,
		}}
	}

	seps := separatorsForExt(ext)
	splitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithSeparators(seps),
		textsplitter.WithChunkSize(codeChunkSize),
		textsplitter.WithChunkOverlap(codeChunkOverlap),
	)

	parts, err := splitter.SplitText(content)
	if err != nil || len(parts) == 0 {
		return []CodeChunk{{
			Content:   header + content,
			FilePath:  filePath,
			Language:  language,
			ChunkType: "content",
			StartLine: 1,
			EndLine:   totalLines,
		}}
	}

	chunks := make([]CodeChunk, 0, len(parts))
	searchFrom := 0
	zeroCount := 0
	for _, part := range parts {
		start, end, next := chunkLineRange(content, part, searchFrom)
		searchFrom = next
		if start == 0 {
			zeroCount++
		}
		chunks = append(chunks, CodeChunk{
			Content:   header + part,
			FilePath:  filePath,
			Language:  language,
			ChunkType: "content",
			StartLine: start,
			EndLine:   end,
		})
	}
	if zeroCount > 0 {
		fmt.Printf("[index-debug] %s: %d/%d chunks failed line lookup\n", filePath, zeroCount, len(parts))
	}
	return chunks
}

// chunkLineRange finds the 1-based start/end line numbers of part within content,
// searching forward from searchFrom byte offset. Returns the next search offset.
// It tries multiple fingerprint lengths (raw, then trimmed) to handle splitter
// whitespace normalization.
func chunkLineRange(content, part string, searchFrom int) (startLine, endLine, nextOffset int) {
	if len(part) == 0 || len(content) == 0 {
		return 0, 0, searchFrom
	}

	tryFind := func(fp string, from int) int {
		if len(fp) == 0 {
			return -1
		}
		if idx := strings.Index(content[from:], fp); idx >= 0 {
			return from + idx
		}
		// Fallback: search entire content (handles mis-estimated searchFrom)
		if from > 0 {
			if idx := strings.Index(content, fp); idx >= 0 {
				return idx
			}
		}
		return -1
	}

	found := -1
	for _, fpLen := range []int{80, 40, 20} {
		if fpLen > len(part) {
			fpLen = len(part)
		}
		raw := part[:fpLen]
		if idx := tryFind(raw, searchFrom); idx >= 0 {
			found = idx
			break
		}
		// Also try trimmed version (splitter may strip leading separators)
		if trimmed := strings.TrimSpace(raw); trimmed != raw && len(trimmed) > 0 {
			if idx := tryFind(trimmed, searchFrom); idx >= 0 {
				found = idx
				break
			}
		}
	}

	if found < 0 {
		return 0, 0, searchFrom
	}

	startLine = strings.Count(content[:found], "\n") + 1
	endLine = startLine + strings.Count(part, "\n")

	advance := found + len(part) - codeChunkOverlap
	if advance <= searchFrom {
		advance = searchFrom + 1
	}
	return startLine, endLine, advance
}

// separatorsForExt returns language-aware separators for the RecursiveCharacter splitter.
// Mirrors the approach from LangChain's Language-based code splitter.
func separatorsForExt(ext string) []string {
	switch ext {
	case ".go":
		return []string{
			"\nfunc ", "\ntype ", "\nvar ", "\nconst ",
			"\n// ", "\n\n", "\n", " ", "",
		}
	case ".py":
		return []string{
			"\nclass ", "\ndef ", "\n\tdef ", "\n    def ",
			"\n# ", "\nif ", "\n\n", "\n", " ", "",
		}
	case ".js", ".jsx", ".ts", ".tsx", ".vue", ".svelte":
		return []string{
			"\nfunction ", "\nconst ", "\nlet ", "\nvar ",
			"\nclass ", "\nexport ", "\nimport ",
			"\nif ", "\nfor ", "\nwhile ",
			"\n// ", "\n\n", "\n", " ", "",
		}
	case ".java", ".kt", ".kts", ".scala":
		return []string{
			"\npublic ", "\nprivate ", "\nprotected ",
			"\nclass ", "\ninterface ", "\nenum ",
			"\n\t@", "\n    @",
			"\n// ", "\n\n", "\n", " ", "",
		}
	case ".rs":
		return []string{
			"\nfn ", "\npub fn ", "\nimpl ", "\nstruct ", "\nenum ",
			"\nmod ", "\ntrait ", "\nuse ",
			"\n// ", "\n\n", "\n", " ", "",
		}
	case ".rb":
		return []string{
			"\ndef ", "\nclass ", "\nmodule ",
			"\n# ", "\nif ", "\nunless ",
			"\n\n", "\n", " ", "",
		}
	case ".c", ".cpp", ".h", ".hpp":
		return []string{
			"\nvoid ", "\nint ", "\nchar ", "\nfloat ", "\ndouble ",
			"\nclass ", "\nstruct ", "\nenum ", "\nnamespace ",
			"\n#include ", "\n#define ",
			"\n// ", "\n\n", "\n", " ", "",
		}
	case ".cs":
		return []string{
			"\npublic ", "\nprivate ", "\nprotected ", "\ninternal ",
			"\nclass ", "\ninterface ", "\nenum ", "\nstruct ",
			"\nnamespace ", "\nusing ",
			"\n// ", "\n\n", "\n", " ", "",
		}
	case ".swift":
		return []string{
			"\nfunc ", "\nclass ", "\nstruct ", "\nenum ", "\nprotocol ",
			"\nimport ", "\nextension ",
			"\n// ", "\n\n", "\n", " ", "",
		}
	case ".php":
		return []string{
			"\nfunction ", "\nclass ", "\ninterface ", "\ntrait ",
			"\npublic ", "\nprivate ", "\nprotected ",
			"\nnamespace ", "\nuse ",
			"\n// ", "\n\n", "\n", " ", "",
		}
	case ".sql":
		return []string{
			"\nCREATE ", "\nALTER ", "\nDROP ",
			"\nSELECT ", "\nINSERT ", "\nUPDATE ", "\nDELETE ",
			"\nWHERE ", "\nJOIN ", "\nFROM ",
			"\n-- ", "\n\n", "\n", " ", "",
		}
	case ".md", ".markdown":
		return []string{
			"\n## ", "\n### ", "\n#### ",
			"\n- ", "\n* ", "\n1. ",
			"\n```", "\n\n", "\n", " ", "",
		}
	case ".sh", ".bash", ".zsh":
		return []string{
			"\nfunction ", "\n# ",
			"\nif ", "\nfor ", "\nwhile ", "\ncase ",
			"\n\n", "\n", " ", "",
		}
	case ".proto":
		return []string{
			"\nmessage ", "\nservice ", "\nenum ", "\nrpc ",
			"\n// ", "\n\n", "\n", " ", "",
		}
	case ".ex", ".exs":
		return []string{
			"\ndef ", "\ndefp ", "\ndefmodule ", "\ndefmacro ",
			"\n# ", "\n\n", "\n", " ", "",
		}
	case ".dart":
		return []string{
			"\nclass ", "\nvoid ", "\nFuture ",
			"\n// ", "\n\n", "\n", " ", "",
		}
	case ".lua":
		return []string{
			"\nfunction ", "\nlocal function ",
			"\n-- ", "\n\n", "\n", " ", "",
		}
	default:
		// Generic fallback
		return []string{"\n\n", "\n", " ", ""}
	}
}

// extractSymbols extracts top-level symbol names (functions, types, classes) from source
// code using simple line-by-line scanning — no external parser required.
func extractSymbols(content, ext string) []string {
	var symbols []string
	seen := map[string]bool{}

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		var name string

		switch ext {
		case ".go":
			if strings.HasPrefix(line, "func ") {
				rest := line[5:]
				if strings.HasPrefix(rest, "(") {
					// method: skip receiver, take name after closing paren
					if end := strings.Index(rest, ")"); end >= 0 {
						name = symbolFirstWord(strings.TrimSpace(rest[end+1:]))
					}
				} else {
					name = symbolFirstWord(rest)
				}
			} else if strings.HasPrefix(line, "type ") {
				name = symbolFirstWord(line[5:])
			}
		case ".py":
			for _, pref := range []string{"async def ", "def "} {
				if strings.HasPrefix(line, pref) {
					name = symbolFirstWord(line[len(pref):])
					break
				}
			}
			if name == "" && strings.HasPrefix(line, "class ") {
				name = symbolFirstWord(line[6:])
			}
		case ".js", ".ts", ".jsx", ".tsx":
			for _, pref := range []string{"export async function ", "export function ", "async function ", "function "} {
				if strings.HasPrefix(line, pref) {
					name = symbolFirstWord(line[len(pref):])
					break
				}
			}
			if name == "" {
				for _, pref := range []string{"export default class ", "export abstract class ", "export class ", "class "} {
					if strings.HasPrefix(line, pref) {
						name = symbolFirstWord(line[len(pref):])
						break
					}
				}
			}
		case ".rs":
			for _, pref := range []string{"pub async fn ", "pub fn ", "async fn ", "fn "} {
				if strings.HasPrefix(line, pref) {
					name = symbolFirstWord(line[len(pref):])
					break
				}
			}
			if name == "" {
				for _, pref := range []string{"pub struct ", "pub enum ", "pub trait ", "struct ", "enum ", "trait "} {
					if strings.HasPrefix(line, pref) {
						name = symbolFirstWord(line[len(pref):])
						break
					}
				}
			}
		case ".rb":
			if strings.HasPrefix(line, "def ") {
				name = symbolFirstWord(line[4:])
			} else if strings.HasPrefix(line, "class ") || strings.HasPrefix(line, "module ") {
				name = symbolFirstWord(line[strings.Index(line, " ")+1:])
			}
		case ".java", ".kt", ".kts":
			for _, kw := range []string{" class ", " interface ", " enum ", " object "} {
				if idx := strings.Index(line, kw); idx >= 0 {
					name = symbolFirstWord(line[idx+len(kw):])
					break
				}
			}
		}

		// Strip any trailing punctuation that may have been included.
		name = strings.TrimRight(name, "(<{:")
		name = strings.TrimSpace(name)
		if isSymbolValid(name) && !seen[name] {
			seen[name] = true
			symbols = append(symbols, name)
		}
	}
	return symbols
}

// symbolFirstWord returns the first identifier-like token from s.
func symbolFirstWord(s string) string {
	s = strings.TrimSpace(s)
	for i, c := range s {
		if c == ' ' || c == '\t' || c == '(' || c == '<' || c == '{' || c == ':' || c == ',' {
			return s[:i]
		}
	}
	return s
}

// isSymbolValid returns true if name looks like a plausible identifier.
func isSymbolValid(name string) bool {
	if len(name) == 0 || len(name) > 60 {
		return false
	}
	for i, c := range name {
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
				return false
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
				return false
			}
		}
	}
	return true
}

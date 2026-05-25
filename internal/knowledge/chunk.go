package knowledge

import (
	"strings"

	"github.com/tmc/langchaingo/textsplitter"
)

const (
	defaultChunkSize    = 800
	defaultChunkOverlap = 100
)

// splitMarkdown splits Markdown text respecting header boundaries and heading hierarchy.
func splitMarkdown(text string, chunkSize, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	splitter := textsplitter.NewMarkdownTextSplitter(
		textsplitter.WithChunkSize(chunkSize),
		textsplitter.WithChunkOverlap(overlap),
		textsplitter.WithHeadingHierarchy(true),
		textsplitter.WithCodeBlocks(true),
	)
	chunks, err := splitter.SplitText(text)
	if err != nil || len(chunks) == 0 {
		return splitText(text, chunkSize, overlap)
	}
	return chunks
}

// splitText splits plain text (PDF, DOCX, etc.) using recursive character splitting.
func splitText(text string, chunkSize, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	splitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithChunkSize(chunkSize),
		textsplitter.WithChunkOverlap(overlap),
	)
	chunks, err := splitter.SplitText(text)
	if err != nil {
		return nil
	}
	return chunks
}

package knowledge

import (
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/textsplitter"
)

const (
	defaultChunkSize    = 1200
	defaultChunkOverlap = 150
	// contextHeaderMaxLen limits how much context header we prepend to each chunk.
	contextHeaderMaxLen = 300
)

// ContextualChunkOpts provides metadata for enriching chunks with contextual headers.
// This follows the "Contextual Chunking" pattern where each chunk carries
// file-level and positional context for stronger embeddings.
type ContextualChunkOpts struct {
	FileName string // source file name
	Summary  string // AI-generated summary of the entire document (optional)
}

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
	return truncateChunks(chunks, chunkSize)
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
	return truncateChunks(chunks, chunkSize)
}

// enrichChunksWithContext prepends contextual headers to each chunk.
// Format: [File: {name}] [Topic: {summary}] [Context: {previous_chunk_summary}]
// This dramatically improves vector search quality by embedding file-level context.
func enrichChunksWithContext(chunks []string, opts ContextualChunkOpts) []string {
	if len(chunks) == 0 {
		return chunks
	}

	enriched := make([]string, len(chunks))
	for i, chunk := range chunks {
		var header strings.Builder

		// File context
		if opts.FileName != "" {
			fmt.Fprintf(&header, "[File: %s] ", opts.FileName)
		}

		// Topic/summary context (from AI enrichment)
		if opts.Summary != "" {
			summary := opts.Summary
			if len(summary) > 150 {
				summary = summary[:150] + "..."
			}
			fmt.Fprintf(&header, "[Topic: %s] ", summary)
		}

		// Previous chunk context (positional awareness)
		if i > 0 {
			prevSummary := summarizeChunk(chunks[i-1])
			fmt.Fprintf(&header, "[Context: %s] ", prevSummary)
		}

		headerStr := header.String()
		if len(headerStr) > contextHeaderMaxLen {
			headerStr = headerStr[:contextHeaderMaxLen]
		}

		if headerStr != "" {
			enriched[i] = headerStr + "\n" + chunk
		} else {
			enriched[i] = chunk
		}
	}
	return enriched
}

// summarizeChunk creates a brief summary of a chunk for use as positional context.
// Uses the first ~100 chars or the first line, whichever is shorter.
func summarizeChunk(chunk string) string {
	chunk = strings.TrimSpace(chunk)
	if chunk == "" {
		return ""
	}
	// Use first line if short enough
	if idx := strings.IndexByte(chunk, '\n'); idx > 0 && idx <= 100 {
		return strings.TrimSpace(chunk[:idx])
	}
	if len(chunk) > 100 {
		// Cut at word boundary
		cut := chunk[:100]
		if idx := strings.LastIndexByte(cut, ' '); idx > 50 {
			cut = cut[:idx]
		}
		return cut + "..."
	}
	return chunk
}

// truncateChunks ensures no chunk exceeds maxChars (safety for embedding model token limits).
func truncateChunks(chunks []string, maxChars int) []string {
	for i, c := range chunks {
		if len(c) > maxChars {
			chunks[i] = c[:maxChars]
		}
	}
	return chunks
}

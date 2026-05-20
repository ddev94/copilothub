package knowledge

import "strings"

// splitText splits text into overlapping chunks, preferring paragraph/sentence boundaries.
func splitText(text string, chunkSize, overlap int) []string {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return nil
	}
	if len(text) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0
	for start < len(text) {
		end := start + chunkSize
		if end >= len(text) {
			chunk := strings.TrimSpace(text[start:])
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
			break
		}
		seg := text[start:end]
		if i := strings.LastIndex(seg, "\n\n"); i > chunkSize/4 {
			end = start + i
		} else if i := strings.LastIndex(seg, ". "); i > chunkSize/4 {
			end = start + i + 1
		}
		chunk := strings.TrimSpace(text[start:end])
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		next := end - overlap
		if next <= start {
			next = start + 1
		}
		start = next
	}
	return chunks
}

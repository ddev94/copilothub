package wiki

import (
	"context"
	"copilothub/internal/ai"
	"copilothub/internal/knowledge"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// ---- PDF Refinery ----
// Uses AI to convert raw extracted PDF text into well-structured markdown.

func (h *Handler) refinePDF(ctx context.Context, filePath, fileName string) string {
	aiProv := h.newAI()
	if aiProv == nil {
		return ""
	}

	text, err := knowledge.ReadFileContent(filePath, fileName)
	if err != nil || strings.TrimSpace(text) == "" {
		return ""
	}

	// Truncate very long PDFs to avoid token limits
	if len(text) > 30000 {
		text = text[:30000]
	}

	prompt := fmt.Sprintf(`You are a document refinery. Convert this raw PDF-extracted text into clean, well-structured Markdown.

Rules:
- Fix broken sentences from PDF column/page splits
- Reconstruct tables as markdown tables
- Add proper headings (## ###) based on content structure
- Remove headers/footers/page numbers
- Keep ALL information — do not summarize or omit
- Output in the SAME language as the input

Raw text:
---
%s
---

Clean Markdown:`, text)

	messages := []ai.Message{{Role: "user", Content: prompt}}
	resp, err := aiProv.Complete(ctx, messages)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(resp)
}

// ---- Multi-Query Expansion ----
// Generates 3 variant queries from the user's question to improve retrieval coverage.
// This ensures we "sweep" the vector space from multiple angles.

func (h *Handler) expandQueries(ctx context.Context, question string) []string {
	base := strings.TrimSpace(question)
	if base == "" {
		return []string{}
	}
	queries := []string{base}

	aiProv := h.newAI()
	if aiProv != nil {
		prompt := fmt.Sprintf(`Given this user question, generate 5 alternative search queries that cover:
1) Direct phrasing in the SAME language as the question
2) English translation/equivalent (if question is not in English)
3) Japanese translation/equivalent (if relevant domain terms exist in Japanese)
4) Process/steps or constraint/rule phrasing
5) Synonyms and domain terminology across languages

IMPORTANT: Documents may mix Vietnamese, English, and Japanese. Generate queries in ALL relevant languages to ensure cross-language retrieval.

Return ONLY a JSON array of strings, no explanation.

Question: %s

Example: if question is "Application Form có bao nhiêu step?", output:
["Application Form steps", "đơn đăng ký các bước", "申請書 ステップ", "application form process flow", "form đăng ký quy trình thực hiện"]`, base)

		messages := []ai.Message{{Role: "user", Content: prompt}}
		resp, err := aiProv.Complete(ctx, messages)
		if err == nil {
			resp = strings.TrimSpace(resp)
			if idx := strings.Index(resp, "["); idx >= 0 {
				if end := strings.LastIndex(resp, "]"); end > idx {
					resp = resp[idx : end+1]
				}
			}
			var expanded []string
			if json.Unmarshal([]byte(resp), &expanded) == nil {
				seen := map[string]bool{normalizeQuery(base): true}
				for _, q := range expanded {
					q = strings.TrimSpace(q)
					if q == "" {
						continue
					}
					key := normalizeQuery(q)
					if !seen[key] {
						seen[key] = true
						queries = append(queries, q)
					}
				}
			}
		}
	}

	fallbacks := []string{
		base + " quy trình các bước",
		"điều kiện quy tắc " + base,
		base + " ví dụ trường hợp",
	}
	seen := map[string]bool{}
	for _, q := range queries {
		seen[normalizeQuery(q)] = true
	}
	for _, q := range fallbacks {
		key := normalizeQuery(q)
		if !seen[key] {
			seen[key] = true
			queries = append(queries, q)
		}
	}
	if len(queries) > 8 {
		queries = queries[:8]
	}
	return queries
}

// ---- AI Reranking ----
// After retrieving top-K chunks, send them + original question to AI for relevance scoring.
// Only keep chunks scoring > threshold (7/10).

type rankedChunk struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
}

func (h *Handler) rerankChunks(ctx context.Context, question string, chunks []knowledge.Chunk) []knowledge.Chunk {
	if len(chunks) == 0 {
		return chunks
	}

	for i := range chunks {
		chunks[i].Score = blendLexicalScore(question, chunks[i])
	}

	if len(chunks) <= 3 {
		sort.Slice(chunks, func(i, j int) bool { return chunks[i].Score > chunks[j].Score })
		return chunks
	}

	aiProv := h.newAI()
	if aiProv == nil {
		return chunks
	}

	// Build the chunks list for the AI
	var sb strings.Builder
	for i, chunk := range chunks {
		content := strings.TrimSpace(chunk.Content)
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		fmt.Fprintf(&sb, "--- Chunk %d ---\n%s\n\n", i+1, content)
	}

	prompt := fmt.Sprintf(`You are a relevance judge. Given a question and %d text chunks, score each chunk's relevance to the question from 1-10.

Question: %s

Chunks:
%s

Return ONLY a JSON array of objects with "index" (1-based) and "score" (1-10). No explanation.
Example: [{"index": 1, "score": 9}, {"index": 2, "score": 3}]`, len(chunks), question, sb.String())

	messages := []ai.Message{{Role: "user", Content: prompt}}
	resp, err := aiProv.Complete(ctx, messages)
	if err != nil {
		return chunks
	}

	// Parse response
	resp = strings.TrimSpace(resp)
	if idx := strings.Index(resp, "["); idx >= 0 {
		if end := strings.LastIndex(resp, "]"); end > idx {
			resp = resp[idx : end+1]
		}
	}

	var ranked []rankedChunk
	if err := json.Unmarshal([]byte(resp), &ranked); err != nil {
		return chunks
	}

	// Validate indices and sort all ranked chunks by AI score descending
	var valid []rankedChunk
	for _, r := range ranked {
		if r.Index >= 1 && r.Index <= len(chunks) {
			valid = append(valid, r)
		}
	}
	if len(valid) == 0 {
		if len(chunks) > 5 {
			return chunks[:5]
		}
		return chunks
	}

	sort.Slice(valid, func(i, j int) bool {
		return valid[i].Score > valid[j].Score
	})

	// Primary filter: chunks scoring >= 7.5 (hard threshold)
	const rerankThreshold = 7.5
	var filtered []rankedChunk
	for _, r := range valid {
		if r.Score >= rerankThreshold {
			filtered = append(filtered, r)
		}
	}

	// Smart fallback: if nothing passes threshold, take top-5 by AI score
	// (these are still the "best" chunks according to the AI, just lower confidence)
	if len(filtered) == 0 {
		topN := 5
		if len(valid) < topN {
			topN = len(valid)
		}
		filtered = valid[:topN]
	}

	result := make([]knowledge.Chunk, 0, len(filtered))
	for _, r := range filtered {
		c := chunks[r.Index-1]
		aiScore := r.Score / 10.0
		c.Score = 0.7*aiScore + 0.3*c.Score
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	return result
}

func normalizeQuery(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func tokenizeNormalized(s string) []string {
	norm := normalizeQuery(s)
	if norm == "" {
		return nil
	}
	parts := strings.Fields(norm)
	if len(parts) == 0 {
		return nil
	}
	return parts
}

func blendLexicalScore(question string, chunk knowledge.Chunk) float64 {
	base := chunk.Score
	qTokens := tokenizeNormalized(question)
	if len(qTokens) == 0 {
		return base
	}
	cTokens := tokenizeNormalized(chunk.Content)
	if len(cTokens) == 0 {
		return base
	}
	set := make(map[string]struct{}, len(cTokens))
	for _, t := range cTokens {
		set[t] = struct{}{}
	}
	matches := 0
	for _, qt := range qTokens {
		if _, ok := set[qt]; ok {
			matches++
		}
	}
	lexical := float64(matches) / float64(len(qTokens))
	if lexical > 1 {
		lexical = 1
	}
	blended := 0.75*base + 0.25*lexical
	if blended < 0 {
		return 0
	}
	if blended > 1 {
		return 1
	}
	return blended
}

// ---- AI Intent Detection ----
// Classifies user question into one of the predefined intents using AI.
// Falls back to heuristic detectIntent if AI is unavailable.

func (h *Handler) detectIntentAI(ctx context.Context, question string) string {
	aiProv := h.newAI()
	if aiProv == nil {
		return detectIntent(question)
	}

	prompt := fmt.Sprintf(`Classify this user question into exactly ONE intent category. Return ONLY the category name, nothing else.

Categories:
- relationship_query: ONLY when explicitly asking about relationships/dependencies between 2+ named entities
- as_is: ONLY when explicitly asking "hiện tại thế nào" or "đang hoạt động ra sao"
- to_be: ONLY when explicitly asking about future changes with words like "nếu thay đổi", "to-be", "sau khi cải tiến"
- summary: ONLY when explicitly asking "tóm tắt" or "overview"
- fact_lookup: DEFAULT for all other questions including "cần làm gì", "là gì", "có những gì", "quy trình", "các bước"

IMPORTANT: When in doubt, ALWAYS return fact_lookup. Do NOT over-classify into to_be or as_is.

Question: %s

Category:`, question)

	messages := []ai.Message{{Role: "user", Content: prompt}}
	resp, err := aiProv.Complete(ctx, messages)
	if err != nil {
		return detectIntent(question)
	}

	resp = strings.TrimSpace(strings.ToLower(resp))
	// Validate it's a known intent
	validIntents := map[string]bool{
		"relationship_query": true,
		"as_is":              true,
		"to_be":              true,
		"summary":            true,
		"fact_lookup":        true,
	}
	if validIntents[resp] {
		return resp
	}
	// AI returned something unexpected, fall back
	return detectIntent(question)
}

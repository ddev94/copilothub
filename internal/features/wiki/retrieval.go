package wiki

import (
	"context"
	"copilothub/internal/knowledge"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const maxEmbeddingChars = 1800

func splitQueryChunks(q string) []string {
	if len(q) <= maxEmbeddingChars {
		return []string{q}
	}
	var chunks []string
	for len(q) > 0 {
		if len(q) <= maxEmbeddingChars {
			chunks = append(chunks, q)
			break
		}
		cut := q[:maxEmbeddingChars]
		if idx := strings.LastIndex(cut, "\n\n"); idx > maxEmbeddingChars/2 {
			cut = cut[:idx]
		} else if idx := strings.LastIndexAny(cut, "\n "); idx > maxEmbeddingChars/2 {
			cut = cut[:idx]
		}
		chunks = append(chunks, strings.TrimSpace(cut))
		q = strings.TrimSpace(q[len(cut):])
	}
	return chunks
}

func (h *Handler) retrieveForQueries(ctx context.Context, c *knowledge.Client, projectID string, queries []string, topK int) ([]knowledge.Chunk, error) {
	var allChunks []knowledge.Chunk
	for _, q := range queries {
		queryChunks := splitQueryChunks(q)
		for _, qc := range queryChunks {
			got, err := c.Retrieve(ctx, projectID, qc, topK)
			if err != nil {
				return nil, err
			}
			allChunks = append(allChunks, got...)
		}
	}
	merged := h.mergeChunks(nil, allChunks, topK*4)
	return h.rerankChunks(ctx, strings.Join(queries, " | "), merged), nil
}

// mergeChunks deduplicates chunks using a content fingerprint (source + content hash).
// This preserves source diversity while removing true duplicates.
func (h *Handler) mergeChunks(base, incoming []knowledge.Chunk, limit int) []knowledge.Chunk {
	all := append(append([]knowledge.Chunk{}, base...), incoming...)

	// Deduplicate by source+content fingerprint
	seen := make(map[string]bool)
	merged := make([]knowledge.Chunk, 0, len(all))
	for _, chunk := range all {
		fp := chunkFingerprint(chunk)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		merged = append(merged, chunk)
	}

	// Sort by score descending before truncating
	sort.Slice(merged, func(i, j int) bool { return merged[i].Score > merged[j].Score })

	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

// chunkFingerprint generates a unique key from source file + content hash.
func chunkFingerprint(chunk knowledge.Chunk) string {
	content := strings.TrimSpace(chunk.Content)
	if len(content) > 300 {
		content = content[:300]
	}
	h := sha256.Sum256([]byte(chunk.SourceFile + "|" + content))
	return hex.EncodeToString(h[:8])
}

// expandByGraph uses the knowledge graph to find related entities, then retrieves
// actual content chunks using those entities as additional queries.
// This enables cross-document discovery that pure vector search misses.
func (h *Handler) expandByGraph(ctx context.Context, projectID, question string, chunks []knowledge.Chunk) []knowledge.Chunk {
	c := h.getKC()
	if c == nil {
		return chunks
	}

	// Strategy: extract entity names from existing chunks + question keywords,
	// then search graph for related entities → use their names as new queries
	// to retrieve REAL content from vector DB.

	// 1. Collect seed entities from existing chunks' source context
	seedTerms := extractSeedTerms(question, chunks)
	if len(seedTerms) == 0 {
		fmt.Printf("[wiki/graph-expand] no seed terms extracted for project=%s\n", projectID)
		return chunks
	}
	fmt.Printf("[wiki/graph-expand] seed terms: %v\n", seedTerms)

	// 2. Search graph for matching nodes using seed terms
	var matchedNodes []knowledge.GraphNode
	seen := make(map[string]bool)
	for _, term := range seedTerms {
		nodes, err := c.SearchGraphNodes(ctx, projectID, term)
		if err != nil {
			continue
		}
		for _, n := range nodes {
			if !seen[n.ID] {
				seen[n.ID] = true
				matchedNodes = append(matchedNodes, n)
			}
		}
	}

	if len(matchedNodes) == 0 {
		fmt.Printf("[wiki/graph-expand] no graph nodes matched for project=%s seeds=%v\n", projectID, seedTerms)
		return chunks
	}
	fmt.Printf("[wiki/graph-expand] matched %d nodes for project=%s\n", len(matchedNodes), projectID)

	// 3. Get neighbors (1-hop) to discover related entities
	relatedNames := make(map[string]bool)
	for _, node := range matchedNodes {
		relatedNames[node.CanonicalName] = true
		neighbors, _, err := c.GetGraphNeighbors(ctx, projectID, node.ID, 1)
		if err != nil {
			continue
		}
		for _, n := range neighbors {
			relatedNames[n.CanonicalName] = true
		}
	}

	// 4. Build graph-derived queries from related entity names
	var graphQueries []string
	for name := range relatedNames {
		if len(name) > 3 { // skip very short names
			graphQueries = append(graphQueries, name)
		}
	}

	// Limit to avoid excessive retrieval
	const maxGraphQueries = 5
	if len(graphQueries) > maxGraphQueries {
		graphQueries = graphQueries[:maxGraphQueries]
	}

	if len(graphQueries) == 0 {
		return chunks
	}
	fmt.Printf("[wiki/graph-expand] graph queries: %v\n", graphQueries)

	// 5. Retrieve REAL chunks using graph-derived queries
	const graphRetrieveTopK = 3
	for _, gq := range graphQueries {
		got, err := c.Retrieve(ctx, projectID, gq, graphRetrieveTopK)
		if err != nil {
			continue
		}
		chunks = append(chunks, got...)
	}

	// Deduplicate
	deduped := make([]knowledge.Chunk, 0, len(chunks))
	fpSeen := make(map[string]bool)
	for _, chunk := range chunks {
		fp := chunkFingerprint(chunk)
		if fpSeen[fp] {
			continue
		}
		fpSeen[fp] = true
		deduped = append(deduped, chunk)
	}

	sort.Slice(deduped, func(i, j int) bool { return deduped[i].Score > deduped[j].Score })

	// Budget limit
	const maxTotal = 20
	if len(deduped) > maxTotal {
		deduped = deduped[:maxTotal]
	}

	fmt.Printf("[wiki/graph-expand] result: %d chunks (was %d, added %d from graph)\n", len(deduped), len(chunks)-len(deduped)+len(deduped), len(deduped)-len(chunks)+len(chunks)-len(deduped))
	return deduped
}

// extractSeedTerms pulls key terms from the question and top chunks for graph search.
func extractSeedTerms(question string, chunks []knowledge.Chunk) []string {
	terms := make(map[string]bool)

	// From question: extract meaningful noun phrases (simple heuristic)
	qWords := strings.Fields(strings.ToLower(question))
	stopwords := map[string]bool{
		"có": true, "là": true, "của": true, "và": true, "các": true, "trong": true,
		"cho": true, "với": true, "được": true, "từ": true, "đến": true, "này": true,
		"đó": true, "một": true, "những": true, "tôi": true, "bạn": true, "nó": true,
		"gì": true, "gồm": true, "bao": true, "nhiêu": true, "nào": true, "sao": true,
		"thì": true, "mà": true, "để": true, "khi": true, "vậy": true, "ra": true,
		"lên": true, "xuống": true, "vào": true, "the": true, "is": true, "are": true,
		"what": true, "how": true, "which": true, "step": true, "liệt": true, "kê": true,
	}
	for _, w := range qWords {
		w = strings.Trim(w, "?!.,;:")
		if len(w) > 2 && !stopwords[w] {
			terms[w] = true
		}
	}

	// From top chunks: extract source file names (without extension) as terms
	for i, chunk := range chunks {
		if i >= 3 {
			break
		}
		if chunk.SourceFile != "" {
			name := strings.TrimSuffix(filepath.Base(chunk.SourceFile), filepath.Ext(chunk.SourceFile))
			// Remove numeric prefix like "13_"
			parts := strings.SplitN(name, "_", 2)
			if len(parts) == 2 && len(parts[0]) <= 3 {
				name = parts[1]
			}
			name = strings.ReplaceAll(name, "_", " ")
			terms[name] = true
		}
	}

	result := make([]string, 0, len(terms))
	for t := range terms {
		result = append(result, t)
	}
	return result
}

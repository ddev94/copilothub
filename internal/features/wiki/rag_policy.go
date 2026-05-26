package wiki

import (
	"context"
	"copilothub/internal/ai"
	"copilothub/internal/knowledge"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type retrievalPlan struct {
	Queries    []string
	UseGraph   bool
	TargetTopK int
	QueryType  string
}

type verificationResult struct {
	Sufficient           bool
	CoverageScore        float64
	Reason               string
	HighConfidenceChunks int
	SourceDiversity      int
	HasGaps              bool // true when sequence steps are missing
	ExpectedSteps        int  // total steps mentioned in evidence
	FoundSteps           int  // steps actually found in evidence
}

type refinementPlan struct {
	NextQueries []string
	Reason      string
}

// detectQueryType classifies the question into a query type (primary signal for the pipeline).
func detectQueryType(question string) string {
	q := strings.ToLower(strings.TrimSpace(question))

	// Process/flow detection takes priority when asking about steps
	isStepRelated := strings.Contains(q, "step") || strings.Contains(q, "các bước") ||
		strings.Contains(q, "quy trình") || strings.Contains(q, "flow") ||
		strings.Contains(q, "thực hiện") || strings.Contains(q, "trình tự")

	// "bao nhiêu step" or "có mấy bước" → process (not count)
	if isStepRelated {
		return "process"
	}

	switch {
	case strings.Contains(q, "bao nhiêu") || strings.Contains(q, "số lượng") || strings.Contains(q, "count") || strings.Contains(q, "có mấy"):
		return "count"
	case strings.Contains(q, "liệt kê") || strings.Contains(q, "gồm những") || strings.Contains(q, "status nào") || strings.Contains(q, "danh sách") || strings.Contains(q, "có những gì"):
		return "list"
	case strings.Contains(q, "ý nghĩa") || strings.Contains(q, "nghĩa là") || strings.Contains(q, "mapping") || strings.Contains(q, "tương ứng"):
		return "mapping"
	case strings.Contains(q, "so sánh") || strings.Contains(q, "khác nhau") || strings.Contains(q, "giống nhau"):
		return "compare"
	case strings.Contains(q, "tóm tắt") || strings.Contains(q, "tổng quan") || strings.Contains(q, "overview"):
		return "summary"
	default:
		return "fact"
	}
}

// buildInitialPlan creates a retrieval plan based on query-type (primary) and intent (secondary).
func (h *Handler) buildInitialPlan(queryType, intent, question string) retrievalPlan {
	targetTopK := h.topK
	switch queryType {
	case "summary":
		targetTopK = h.topK + 5
	case "count", "list", "mapping":
		targetTopK = h.topK + 3 // need more coverage for enumerations
	case "process":
		targetTopK = h.topK + 4 // processes often span multiple chunks
	}

	// Graph usage: start with relationship-based queries, but evidence-based
	// expansion will also trigger it later (Phase E)
	useGraph := intent == "relationship_query" || queryType == "mapping" || queryType == "compare"

	return retrievalPlan{
		Queries:    h.expandQueries(contextBackground(), question),
		UseGraph:   useGraph,
		TargetTopK: targetTopK,
		QueryType:  queryType,
	}
}

// shouldUseGraphByEvidence determines if graph expansion should activate
// based on evidence quality, not just initial intent classification.
func shouldUseGraphByEvidence(plan retrievalPlan, verified verificationResult, iter int) bool {
	if verified.HasGaps {
		return true // Always use graph when sequence gaps detected
	}
	if verified.Sufficient {
		return false
	}
	if plan.UseGraph {
		return true
	}
	if iter >= 2 && (verified.Reason == "low_confidence_density" || verified.Reason == "avg_score_low") {
		return true
	}
	return false
}

// verifyEvidence checks if the retrieved chunks provide sufficient coverage for the query type.
func (h *Handler) verifyEvidence(queryType string, chunks []knowledge.Chunk) verificationResult {
	if len(chunks) == 0 {
		return verificationResult{Sufficient: false, CoverageScore: 0, Reason: "no_chunks"}
	}

	sorted := append([]knowledge.Chunk{}, chunks...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })

	topN := 5
	if len(sorted) < topN {
		topN = len(sorted)
	}
	var avg float64
	highConfidence := 0
	for i := 0; i < topN; i++ {
		avg += sorted[i].Score
		if sorted[i].Score >= 0.65 {
			highConfidence++
		}
	}
	avg /= float64(topN)

	// Count source diversity
	sources := make(map[string]bool)
	for _, c := range chunks {
		if c.SourceFile != "" {
			sources[c.SourceFile] = true
		}
	}
	srcDiversity := len(sources)

	// Query-type-specific thresholds
	threshold := 0.55
	minHighConfidence := 2
	minChunks := 4
	switch queryType {
	case "summary":
		threshold = 0.45
		minHighConfidence = 1
		minChunks = 3
	case "compare", "mapping":
		threshold = 0.52
		minHighConfidence = 2
		minChunks = 5
	case "count", "list":
		threshold = 0.55
		minHighConfidence = 3
		minChunks = 5
	case "process":
		threshold = 0.50
		minHighConfidence = 2
		minChunks = 4
	}

	sufficient := len(chunks) >= minChunks && avg >= threshold && highConfidence >= minHighConfidence
	reason := "score_threshold"
	if len(chunks) < minChunks {
		reason = "insufficient_chunks"
	} else if highConfidence < minHighConfidence {
		reason = "low_confidence_density"
	} else if !sufficient {
		reason = "avg_score_low"
	}

	// Sequence gap detection for process queries
	var hasGaps bool
	var expectedSteps, foundSteps int
	if queryType == "process" && sufficient {
		expectedSteps, foundSteps = detectSequenceGaps(chunks)
		if expectedSteps > 0 && foundSteps < expectedSteps {
			sufficient = false
			hasGaps = true
			reason = "missing_sequence_steps"
			fmt.Printf("[wiki/verify] sequence gap: expected %d steps, found %d\n", expectedSteps, foundSteps)
		}
	}

	return verificationResult{
		Sufficient:           sufficient,
		CoverageScore:        avg,
		Reason:               reason,
		HighConfidenceChunks: highConfidence,
		SourceDiversity:      srcDiversity,
		HasGaps:              hasGaps,
		ExpectedSteps:        expectedSteps,
		FoundSteps:           foundSteps,
	}
}

// refinePlanAI uses AI to generate smarter follow-up queries based on what's missing.
func (h *Handler) refinePlanAI(ctx context.Context, question, queryType string, history []chatTurn, priorQueries []string, currentChunks []knowledge.Chunk, verified verificationResult) refinementPlan {
	if verified.Sufficient {
		return refinementPlan{}
	}

	aiProv := h.newAI()
	if aiProv == nil {
		return h.refinePlanHeuristic(question, queryType, priorQueries, verified)
	}

	// Build context of what we already have
	var chunkSummary strings.Builder
	limit := 5
	if len(currentChunks) < limit {
		limit = len(currentChunks)
	}
	for i := 0; i < limit; i++ {
		content := strings.TrimSpace(currentChunks[i].Content)
		if len(content) > 200 {
			content = content[:200]
		}
		fmt.Fprintf(&chunkSummary, "- [score=%.2f] %s\n", currentChunks[i].Score, content)
	}

	// Build gap-aware prompt
	var gapHint string
	if verified.HasGaps {
		var missingSteps []string
		for i := 1; i <= verified.ExpectedSteps; i++ {
			found := false
			for j := 0; j < limit; j++ {
				if strings.Contains(currentChunks[j].Content, fmt.Sprintf("Bước %d", i)) ||
					strings.Contains(currentChunks[j].Content, fmt.Sprintf("Step %d", i)) {
					found = true
					break
				}
			}
			if !found {
				missingSteps = append(missingSteps, fmt.Sprintf("Bước %d/Step %d", i, i))
			}
		}
		gapHint = fmt.Sprintf("\n\nCRITICAL: Evidence mentions %d steps but ONLY has details for %d. MISSING: %s. Generate queries that SPECIFICALLY target these missing steps. Do NOT search for steps already found.",
			verified.ExpectedSteps, verified.FoundSteps, strings.Join(missingSteps, ", "))
	}

	prompt := fmt.Sprintf(`Given a user question and what was already retrieved, generate 2-3 NEW search queries to find MISSING information.

Question: %s
Query type: %s
Coverage issue: %s (highConf=%d, avgScore=%.2f)
Prior queries: %s

Already retrieved (top chunks):
%s

Rules:
- Generate queries that would find DIFFERENT/MISSING information
- For "%s" type: focus on completeness and enumeration
- Do NOT repeat prior queries in different wording
- Return ONLY a JSON array of strings%s

Example: ["query 1", "query 2"]`, question, queryType, verified.Reason, verified.HighConfidenceChunks, verified.CoverageScore,
		strings.Join(priorQueries, " | "), chunkSummary.String(), queryType, gapHint)

	messages := []ai.Message{{Role: "user", Content: prompt}}
	resp, err := aiProv.Complete(ctx, messages)
	if err != nil {
		return h.refinePlanHeuristic(question, queryType, priorQueries, verified)
	}

	resp = strings.TrimSpace(resp)
	if idx := strings.Index(resp, "["); idx >= 0 {
		if end := strings.LastIndex(resp, "]"); end > idx {
			resp = resp[idx : end+1]
		}
	}
	var nextQueries []string
	if json.Unmarshal([]byte(resp), &nextQueries) != nil {
		return h.refinePlanHeuristic(question, queryType, priorQueries, verified)
	}

	// Dedup against prior queries
	seen := make(map[string]bool)
	for _, q := range priorQueries {
		seen[normalizeQuery(q)] = true
	}
	var filtered []string
	for _, q := range nextQueries {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		key := normalizeQuery(q)
		if !seen[key] {
			seen[key] = true
			filtered = append(filtered, q)
		}
	}
	if len(filtered) > 3 {
		filtered = filtered[:3]
	}
	if len(filtered) == 0 {
		return h.refinePlanHeuristic(question, queryType, priorQueries, verified)
	}
	return refinementPlan{NextQueries: filtered, Reason: "ai_refinement: " + verified.Reason}
}

// refinePlanHeuristic is the fallback when AI refinement is unavailable.
func (h *Handler) refinePlanHeuristic(question, queryType string, priorQueries []string, verified verificationResult) refinementPlan {
	base := strings.TrimSpace(question)
	if base == "" {
		return refinementPlan{}
	}

	var next []string
	switch queryType {
	case "count", "list":
		next = []string{
			"tất cả " + base,
			"danh sách đầy đủ " + base,
			"các loại " + base + " gồm những gì",
		}
	case "process":
		next = []string{
			"chi tiết từng bước " + base,
			"điều kiện và ngoại lệ " + base,
			"thứ tự thực hiện " + base,
		}
	case "mapping":
		next = []string{
			"bảng mapping " + base,
			"tương ứng giữa các mục " + base,
		}
	default:
		next = []string{
			"quy trình chi tiết " + base,
			"điều kiện và ngoại lệ " + base,
			"liên quan tài liệu nào " + base,
		}
	}

	seen := make(map[string]bool)
	for _, q := range priorQueries {
		seen[normalizeQuery(q)] = true
	}
	var filtered []string
	for _, q := range next {
		key := normalizeQuery(q)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, q)
	}
	if len(filtered) > 3 {
		filtered = filtered[:3]
	}
	return refinementPlan{NextQueries: filtered, Reason: "heuristic: " + verified.Reason}
}

func contextBackground() context.Context { return context.Background() }

// detectSequenceGaps checks if evidence mentions N steps but only contains details for fewer.
// Returns (expectedSteps, foundSteps). If expectedSteps==0, detection was inconclusive.
func detectSequenceGaps(chunks []knowledge.Chunk) (int, int) {
	var fullText strings.Builder
	for _, c := range chunks {
		fullText.WriteString(c.Content)
		fullText.WriteString("\n")
	}
	text := fullText.String()

	// Detect expected step count from mentions like "6 bước", "(6 BƯỚC)", "6 steps"
	expected := 0
	for n := 3; n <= 20; n++ {
		patterns := []string{
			fmt.Sprintf("%d bước", n),
			fmt.Sprintf("%d BƯỚC", n),
			fmt.Sprintf("%d steps", n),
			fmt.Sprintf("%d step", n),
		}
		for _, p := range patterns {
			if strings.Contains(text, p) {
				if n > expected {
					expected = n
				}
			}
		}
	}
	if expected == 0 {
		return 0, 0
	}

	// Count which steps are actually present in evidence
	found := 0
	for i := 1; i <= expected; i++ {
		stepPatterns := []string{
			fmt.Sprintf("Bước %d", i),
			fmt.Sprintf("bước %d", i),
			fmt.Sprintf("Step %d", i),
			fmt.Sprintf("step %d", i),
			fmt.Sprintf("BƯỚC %d", i),
		}
		for _, p := range stepPatterns {
			if strings.Contains(text, p) {
				found++
				break
			}
		}
	}
	return expected, found
}

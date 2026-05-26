package wiki

import (
	"context"
	"copilothub/internal/knowledge"
	"fmt"
	"strings"
)

const maxLoopIterations = 4

type chatInput struct {
	ProjectID  string
	SectionKey string
	Question   string
	History    []chatTurn
	Intent     string
}

type chatOutput struct {
	Answer         string
	Chunks         []knowledge.Chunk
	DetectedIntent string
	UsedGraph      bool
	QueryType      string
	Iterations     int
	StopReason     string
}

type chatStepReporter func(step string, status string, summary string, data map[string]any)

func (h *Handler) runChat(ctx context.Context, c *knowledge.Client, in chatInput) (chatOutput, error) {
	return h.runChatWithReporter(ctx, c, in, nil)
}

func (h *Handler) runChatWithReporter(ctx context.Context, c *knowledge.Client, in chatInput, report chatStepReporter) (chatOutput, error) {
	// Phase A: query-type is the PRIMARY signal; intent is secondary for style only
	queryType := detectQueryType(in.Question)
	intent := strings.TrimSpace(in.Intent)
	if intent == "" {
		intent = h.detectIntentAI(ctx, in.Question)
	}

	plan := h.buildInitialPlan(queryType, intent, in.Question)
	if report != nil {
		report(stepDetectIntent, "completed", "Đã phân loại dạng câu hỏi", map[string]any{
			"detectedIntent": intent,
			"queryType":      plan.QueryType,
			"targetTopK":     plan.TargetTopK,
			"useGraph":       plan.UseGraph,
		})
	}

	queries := append([]string{}, plan.Queries...)
	allChunks := make([]knowledge.Chunk, 0, h.topK*3)
	usedGraph := false
	var stopReason string

	if report != nil {
		report(stepRetrieveChunks, "started", "Đang truy xuất chunks", nil)
	}

	for iter := 1; iter <= maxLoopIterations; iter++ {
		// Retrieve
		got, err := h.retrieveForQueries(ctx, c, in.ProjectID, queries, plan.TargetTopK)
		if err != nil {
			return chatOutput{}, err
		}
		allChunks = h.mergeChunks(allChunks, got, h.topK*4)

		// Verify coverage BEFORE deciding on graph
		verified := h.verifyEvidence(plan.QueryType, allChunks)

		// Phase E: graph usage driven by evidence, not just intent
		if shouldUseGraphByEvidence(plan, verified, iter) && !usedGraph {
			if report != nil {
				report(stepExpandGraph, "started", "Đang mở rộng quan hệ qua graph", nil)
			}
			graphChunks := h.expandByGraph(ctx, in.ProjectID, in.Question, allChunks)
			graphAdded := len(graphChunks) - len(allChunks)
			if graphAdded > 0 {
				allChunks = graphChunks
				usedGraph = true
			}
			if report != nil {
				report(stepExpandGraph, "completed", fmt.Sprintf("Graph: +%d chunks", graphAdded), map[string]any{
					"chunkCount": len(allChunks),
					"graphAdded": graphAdded,
					"usedGraph":  usedGraph,
				})
			}
		}

		if report != nil {
			report(stepRetrieveChunks, "completed", fmt.Sprintf("Iteration %d: %d chunks, score=%.2f", iter, len(allChunks), verified.CoverageScore), map[string]any{
				"retrievedCount":      len(allChunks),
				"iteration":           iter,
				"coverageScore":       verified.CoverageScore,
				"highConfidenceCount": verified.HighConfidenceChunks,
				"sufficient":          verified.Sufficient,
				"sourceDiversity":     countUniqueSources(allChunks),
			})
		}

		fmt.Printf("[wiki/chat] queryType=%s intent=%s iter=%d chunks=%d score=%.2f highConf=%d sufficient=%t\n",
			plan.QueryType, intent, iter, len(allChunks), verified.CoverageScore, verified.HighConfidenceChunks, verified.Sufficient)

		if verified.Sufficient {
			stopReason = "sufficient_evidence"
			break
		}
		if iter == maxLoopIterations {
			stopReason = "max_iterations"
			break
		}

		// Refine queries for next iteration
		refined := h.refinePlanAI(ctx, in.Question, plan.QueryType, in.History, queries, allChunks, verified)
		if len(refined.NextQueries) == 0 {
			stopReason = "no_more_queries"
			break
		}
		queries = refined.NextQueries

		if report != nil {
			report(stepRetrieveChunks, "started", fmt.Sprintf("Refining: %s", refined.Reason), map[string]any{
				"nextQueries": refined.NextQueries,
				"reason":      refined.Reason,
			})
		}
	}

	// Report graph status if never triggered
	if !usedGraph && report != nil {
		report(stepExpandGraph, "skipped", "Graph không cần thiết hoặc không có dữ liệu phù hợp", nil)
	}

	// Synthesize
	if report != nil {
		report(stepSynthesizeAnswer, "started", "Đang tổng hợp câu trả lời", nil)
	}
	answer := h.synthesizeAnswer(ctx, in.ProjectID, plan.QueryType, in.SectionKey, in.Question, in.History, allChunks)
	if report != nil {
		report(stepSynthesizeAnswer, "completed", "Đã tổng hợp câu trả lời", map[string]any{
			"answerLength":    len(answer),
			"chunksUsed":      len(allChunks),
			"stopReason":      stopReason,
			"sourceDiversity": countUniqueSources(allChunks),
		})
	}

	return chatOutput{
		Answer:         answer,
		Chunks:         allChunks,
		DetectedIntent: intent,
		UsedGraph:      usedGraph,
		QueryType:      plan.QueryType,
		Iterations:     countIterations(stopReason),
		StopReason:     stopReason,
	}, nil
}

func countUniqueSources(chunks []knowledge.Chunk) int {
	seen := make(map[string]bool)
	for _, c := range chunks {
		if c.SourceFile != "" {
			seen[c.SourceFile] = true
		}
	}
	return len(seen)
}

func countIterations(stopReason string) int {
	// Not exact but informational
	switch stopReason {
	case "sufficient_evidence":
		return 1
	case "no_more_queries":
		return 2
	default:
		return maxLoopIterations
	}
}

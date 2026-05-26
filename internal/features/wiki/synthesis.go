package wiki

import (
	"context"
	"copilothub/internal/ai"
	"copilothub/internal/knowledge"
	"fmt"
	"sort"
	"strings"
)

func (h *Handler) synthesizeAnswer(ctx context.Context, projectID, queryType, sectionKey, question string, history []chatTurn, chunks []knowledge.Chunk) string {
	if len(chunks) == 0 {
		return "Không tìm thấy thông tin liên quan trong knowledge của project đã chọn."
	}

	// Confidence gating: separate high/low confidence chunks
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Score > chunks[j].Score })
	highConfChunks := filterHighConfidence(chunks, 0.50)
	if len(highConfChunks) == 0 {
		return "Dữ liệu hiện tại chưa có thông tin chi tiết về vấn đề này. Vui lòng upload thêm tài liệu liên quan hoặc đặt câu hỏi cụ thể hơn."
	}

	aiProv := h.newAI()
	if aiProv == nil {
		return summarizeFromChunks(question, highConfChunks)
	}

	var contextBuilder strings.Builder
	if c := h.getKC(); c != nil {
		if allDocs, _ := c.ListDocuments(ctx, projectID); len(allDocs) > 0 {
			contextBuilder.WriteString("# Tài liệu có sẵn\n")
			for i, d := range allDocs {
				fmt.Fprintf(&contextBuilder, "%d. %s\n", i+1, d.Name)
			}
			contextBuilder.WriteString("\n")
		}
	}

	if len(history) > 0 {
		contextBuilder.WriteString("# Hội thoại trước\n")
		start := 0
		if len(history) > 3 {
			start = len(history) - 3
		}
		for i := start; i < len(history); i++ {
			fmt.Fprintf(&contextBuilder, "Q: %s\nA: %s\n\n", history[i].Question, history[i].Answer)
		}
	}

	// Only pass high-confidence chunks as main evidence
	contextBuilder.WriteString("# Dữ liệu trích xuất (chỉ evidence có confidence cao)\n\n")
	for i, chunk := range highConfChunks {
		content := strings.TrimSpace(chunk.Content)
		for strings.HasPrefix(content, "[") {
			if idx := strings.Index(content, "] "); idx >= 0 {
				content = strings.TrimSpace(content[idx+2:])
			} else {
				break
			}
		}
		source := chunk.SourceFile
		if source == "" {
			source = "unknown"
		}
		fmt.Fprintf(&contextBuilder, "--- %d [source: %s, score: %.2f] ---\n%s\n\n", i+1, source, chunk.Score, content)
	}

	// If there are low-confidence supplementary chunks, mention them
	lowConfChunks := filterLowConfidence(chunks, 0.50)
	if len(lowConfChunks) > 0 && len(lowConfChunks) <= 5 {
		contextBuilder.WriteString("# Dữ liệu bổ sung (confidence thấp, chỉ tham khảo)\n\n")
		for i, chunk := range lowConfChunks {
			content := strings.TrimSpace(chunk.Content)
			if len(content) > 300 {
				content = content[:300] + "..."
			}
			fmt.Fprintf(&contextBuilder, "--- sup_%d [score: %.2f] ---\n%s\n\n", i+1, chunk.Score, content)
		}
	}

	prompt := fmt.Sprintf("CÂU HỎI: %s\n\n%s", question, contextBuilder.String())
	aiCtx, cancel := context.WithTimeout(ctx, aiTimeout)
	defer cancel()

	sysMsg := buildSynthesisSystemPrompt(queryType)
	messages := []ai.Message{{Role: "system", Content: sysMsg}, {Role: "user", Content: prompt}}
	answer, err := aiProv.Complete(aiCtx, messages)
	if err != nil {
		return fmt.Sprintf("Lỗi khi tổng hợp câu trả lời từ AI: %v", err)
	}
	return strings.TrimSpace(answer)
}

func buildSynthesisSystemPrompt(queryType string) string {
	base := `Bạn là Senior Business Analyst. Trả lời bằng tiếng Việt, dùng Markdown.

NGUYÊN TẮC BẮT BUỘC:
1. CHỈ trả lời dựa trên dữ liệu trích xuất được cung cấp. KHÔNG suy đoán, KHÔNG thêm thông tin ngoài evidence.
2. Mỗi ý chính PHẢI có citation [source: tên_file].
3. Nếu evidence không đủ để trả lời đầy đủ, NÊU RÕ phần nào thiếu dữ liệu.
4. Dữ liệu "confidence thấp" chỉ được dùng để bổ sung, KHÔNG được dùng làm kết luận chính.
5. KHÔNG dùng cụm "theo tài liệu cho thấy" hay "dựa trên thông tin" — trả lời trực tiếp.
`

	typeInstructions := map[string]string{
		"count": `
FORMAT BẮT BUỘC cho câu hỏi đếm/liệt kê số lượng:
1. **Tổng số**: X (hoặc "Chưa đủ dữ liệu để kết luận tổng số chính xác")
2. **Danh sách**:
   - Mục 1: ý nghĩa [source: file]
   - Mục 2: ý nghĩa [source: file]
   ...
3. **Lưu ý**: Nếu có mục nào chưa rõ hoặc thiếu evidence → ghi rõ "Có thể còn mục khác chưa có trong dữ liệu"`,

		"list": `
FORMAT BẮT BUỘC cho câu hỏi liệt kê:
- Liệt kê đầy đủ TẤT CẢ mục tìm thấy trong evidence
- Mỗi mục kèm mô tả ngắn và [source: file]
- Nếu danh sách có thể chưa đầy đủ → nêu rõ
- KHÔNG tự thêm mục không có trong evidence`,

		"mapping": `
FORMAT BẮT BUỘC cho câu hỏi mapping/tương ứng:
| Mục | Ý nghĩa/Giá trị | Nguồn |
|-----|-----------------|-------|
| X   | ...             | file  |

- Chỉ map những gì có evidence rõ ràng
- Mục nào chưa rõ → ghi "Chưa có thông tin trong dữ liệu"`,

		"process": `
FORMAT cho câu hỏi quy trình/flow/step:
1. ĐẦU TIÊN nêu tổng số steps nếu xác định được từ evidence.
2. Liệt kê TẤT CẢ các bước/step tìm thấy trong evidence, theo thứ tự.
3. Mỗi bước mô tả ngắn gọn nội dung chính + [source: file].
4. Nếu evidence đề cập đến step numbers (Step 1, Step 2...) → liệt kê HẾT các step tìm thấy, kể cả khi mô tả ngắn.
5. Nếu chỉ tìm thấy một phần flow → nêu rõ "Có thể còn bước khác chưa có trong dữ liệu hiện tại".
6. KHÔNG bỏ qua step nào có trong evidence chỉ vì mô tả ngắn.
7. Nếu evidence ghi "Có N bước" và bạn thấy đủ N bước trong evidence → BẮT BUỘC liệt kê TẤT CẢ N bước với đầy đủ chi tiết.
8. KHÔNG ĐƯỢC nói "chỉ có Bước 1" nếu evidence thực sự chứa nội dung các bước khác — hãy đọc KỸ toàn bộ evidence.`,

		"compare": `
FORMAT cho câu hỏi so sánh:
| Tiêu chí | A | B | Nguồn |
|-----------|---|---|-------|

- Chỉ so sánh dựa trên evidence có
- Điểm giống/khác nêu rõ ràng`,

		"summary": `Tóm tắt ngắn gọn theo các ý chính từ evidence. Dùng bullet points. Mỗi ý kèm [source: file].`,

		"fact": `Trả lời trực tiếp câu hỏi dựa trên evidence. Citation bắt buộc. Nếu evidence không đủ → nêu rõ.`,
	}

	instruction := typeInstructions[queryType]
	if instruction == "" {
		instruction = typeInstructions["fact"]
	}
	return base + instruction
}

func filterHighConfidence(chunks []knowledge.Chunk, threshold float64) []knowledge.Chunk {
	var result []knowledge.Chunk
	for _, c := range chunks {
		if c.Score >= threshold {
			result = append(result, c)
		}
	}
	return result
}

func filterLowConfidence(chunks []knowledge.Chunk, threshold float64) []knowledge.Chunk {
	var result []knowledge.Chunk
	for _, c := range chunks {
		if c.Score < threshold {
			result = append(result, c)
		}
	}
	if len(result) > 5 {
		return result[:5]
	}
	return result
}

func summarizeFromChunks(question string, chunks []knowledge.Chunk) string {
	if len(chunks) == 0 {
		return "Không tìm thấy thông tin liên quan trong knowledge của project đã chọn."
	}
	var b strings.Builder
	b.WriteString("Dựa trên knowledge của project, thông tin liên quan:\n")
	limit := len(chunks)
	if limit > 3 {
		limit = 3
	}
	for i := 0; i < limit; i++ {
		c := strings.TrimSpace(chunks[i].Content)
		if c == "" {
			continue
		}
		if len(c) > 700 {
			c = c[:700] + "..."
		}
		fmt.Fprintf(&b, "- %s\n", c)
	}
	_ = question
	return strings.TrimSpace(b.String())
}

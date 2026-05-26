package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const graphFile = "knowledge-graph.json"

// GraphNode represents an entity extracted from knowledge documents.
type GraphNode struct {
	ID            string   `json:"id"`
	CanonicalName string   `json:"canonicalName"`
	Type          string   `json:"type"` // concept, actor, process, rule, data
	Aliases       []string `json:"aliases"`
	SourceDocID   string   `json:"sourceDocId"`
	CreatedAt     string   `json:"createdAt"`
}

// GraphEdge represents a relationship between two entities.
type GraphEdge struct {
	ID              string  `json:"id"`
	FromNodeID      string  `json:"fromNodeId"`
	ToNodeID        string  `json:"toNodeId"`
	RelationType    string  `json:"relationType"` // depends_on, triggers, validates, same_as, contradicts
	EvidenceChunkID string  `json:"evidenceChunkId"`
	Confidence      float64 `json:"confidence"`
	CreatedAt       string  `json:"createdAt"`
}

type graphStore struct {
	mu       sync.RWMutex
	path     string
	nodes    []GraphNode
	edges    []GraphEdge
	revision int
}

func (c *Client) loadGraph() error {
	c.meta.mu.Lock()
	defer c.meta.mu.Unlock()
	graphPath := filepath.Join(filepath.Dir(c.meta.path), graphFile)
	data, err := os.ReadFile(graphPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var g struct {
		Nodes    []GraphNode `json:"nodes"`
		Edges    []GraphEdge `json:"edges"`
		Revision int         `json:"revision"`
	}
	if err := json.Unmarshal(data, &g); err != nil {
		return err
	}
	// Store in meta for now (could be separate field)
	return nil
}

func (c *Client) saveGraph(nodes []GraphNode, edges []GraphEdge, revision int) error {
	c.meta.mu.Lock()
	defer c.meta.mu.Unlock()
	graphPath := filepath.Join(filepath.Dir(c.meta.path), graphFile)
	g := struct {
		Nodes    []GraphNode `json:"nodes"`
		Edges    []GraphEdge `json:"edges"`
		Revision int         `json:"revision"`
	}{
		Nodes:    nodes,
		Edges:    edges,
		Revision: revision,
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(graphPath, data, 0644)
}

func (c *Client) nextGraphRevision() int {
	graphPath := filepath.Join(filepath.Dir(c.meta.path), graphFile)
	data, err := os.ReadFile(graphPath)
	if err != nil {
		return 1
	}
	var g struct {
		Revision int `json:"revision"`
	}
	if err := json.Unmarshal(data, &g); err != nil || g.Revision <= 0 {
		return 1
	}
	return g.Revision + 1
}

func (c *Client) RebuildGraphForProject(ctx context.Context, projectID string) error {
	return c.ExtractGraphFromApproved(ctx, projectID, ExtractEntitiesAndRelations)
}

// GraphHealthInfo holds observable graph state.
type GraphHealthInfo struct {
	NodeCount    int    `json:"nodeCount"`
	EdgeCount    int    `json:"edgeCount"`
	Revision     int    `json:"revision"`
	LastModified string `json:"lastModified,omitempty"`
}

func (c *Client) GraphHealth() (GraphHealthInfo, error) {
	graphPath := filepath.Join(filepath.Dir(c.meta.path), graphFile)
	info, err := os.Stat(graphPath)
	if err != nil {
		if os.IsNotExist(err) {
			return GraphHealthInfo{}, nil
		}
		return GraphHealthInfo{}, err
	}
	data, err := os.ReadFile(graphPath)
	if err != nil {
		return GraphHealthInfo{}, err
	}
	var g struct {
		Nodes    []GraphNode `json:"nodes"`
		Edges    []GraphEdge `json:"edges"`
		Revision int         `json:"revision"`
	}
	if err := json.Unmarshal(data, &g); err != nil {
		return GraphHealthInfo{}, err
	}
	return GraphHealthInfo{
		NodeCount:    len(g.Nodes),
		EdgeCount:    len(g.Edges),
		Revision:     g.Revision,
		LastModified: info.ModTime().UTC().Format(time.RFC3339),
	}, nil
}

func (c *Client) GraphStats() (int, int, int, error) {
	graphPath := filepath.Join(filepath.Dir(c.meta.path), graphFile)
	data, err := os.ReadFile(graphPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, 0, nil
		}
		return 0, 0, 0, err
	}
	var g struct {
		Nodes    []GraphNode `json:"nodes"`
		Edges    []GraphEdge `json:"edges"`
		Revision int         `json:"revision"`
	}
	if err := json.Unmarshal(data, &g); err != nil {
		return 0, 0, 0, err
	}
	return len(g.Nodes), len(g.Edges), g.Revision, nil
}

// ExtractGraphFromApproved builds a knowledge graph from all documents in a project.
// Uses AI to extract entities and relationships from markdown content.
func (c *Client) ExtractGraphFromApproved(ctx context.Context, projectID string, aiExtractor func(context.Context, string) ([]GraphNode, []GraphEdge, error)) error {
	c.meta.mu.RLock()
	projectDocs := []metaEntry{}
	for _, e := range c.meta.docs {
		if e.ProjectID == projectID {
			projectDocs = append(projectDocs, e)
		}
	}
	c.meta.mu.RUnlock()

	if len(projectDocs) == 0 {
		return nil
	}

	var allNodes []GraphNode
	var allEdges []GraphEdge
	nodeMap := make(map[string]GraphNode)

	for _, doc := range projectDocs {
		// Get chunks for this doc
		col, err := c.collection(ctx, projectID)
		if err != nil {
			continue
		}
		results, err := col.Query(ctx, doc.Name, 100, map[string]string{"doc_id": doc.DocID}, nil)
		if err != nil {
			continue
		}

		var docText strings.Builder
		for _, r := range results {
			docText.WriteString(r.Content)
			docText.WriteString("\n\n")
		}

		if docText.Len() == 0 {
			continue
		}

		nodes, edges, err := aiExtractor(ctx, docText.String())
		if err != nil {
			continue
		}

		for _, node := range nodes {
			node.SourceDocID = doc.DocID
			node.CreatedAt = time.Now().UTC().Format(time.RFC3339)
			if node.ID == "" {
				node.ID = uuid.New().String()
			}
			// Dedupe by canonical name
			key := strings.ToLower(strings.TrimSpace(node.CanonicalName))
			if existing, ok := nodeMap[key]; ok {
				// Merge aliases
				existing.Aliases = append(existing.Aliases, node.Aliases...)
				nodeMap[key] = existing
			} else {
				nodeMap[key] = node
			}
		}

		for _, edge := range edges {
			edge.CreatedAt = time.Now().UTC().Format(time.RFC3339)
			if edge.ID == "" {
				edge.ID = uuid.New().String()
			}
			allEdges = append(allEdges, edge)
		}
	}

	for _, node := range nodeMap {
		allNodes = append(allNodes, node)
	}
	sort.Slice(allNodes, func(i, j int) bool {
		return strings.ToLower(allNodes[i].CanonicalName) < strings.ToLower(allNodes[j].CanonicalName)
	})
	revision := c.nextGraphRevision()
	return c.saveGraph(allNodes, allEdges, revision)
}

// GetGraphNeighbors returns nodes connected to the given node ID.
func (c *Client) GetGraphNeighbors(_ context.Context, _ string, nodeID string, maxHops int) ([]GraphNode, []GraphEdge, error) {
	c.meta.mu.RLock()
	defer c.meta.mu.RUnlock()
	graphPath := filepath.Join(filepath.Dir(c.meta.path), graphFile)
	data, err := os.ReadFile(graphPath)
	if err != nil {
		return nil, nil, err
	}
	var g struct {
		Nodes []GraphNode `json:"nodes"`
		Edges []GraphEdge `json:"edges"`
	}
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, nil, err
	}

	visited := make(map[string]bool)
	var resultNodes []GraphNode
	var resultEdges []GraphEdge

	var traverse func(string, int)
	traverse = func(nid string, depth int) {
		if depth > maxHops || visited[nid] {
			return
		}
		visited[nid] = true
		for _, node := range g.Nodes {
			if node.ID == nid {
				resultNodes = append(resultNodes, node)
				break
			}
		}
		for _, edge := range g.Edges {
			if edge.FromNodeID == nid || edge.ToNodeID == nid {
				resultEdges = append(resultEdges, edge)
				next := edge.ToNodeID
				if edge.FromNodeID != nid {
					next = edge.FromNodeID
				}
				traverse(next, depth+1)
			}
		}
	}

	traverse(nodeID, 0)
	return resultNodes, resultEdges, nil
}

// SearchGraphNodes finds nodes by name/alias match.
// Supports multi-word queries by matching ANY word (length > 2) against node names.
func (c *Client) SearchGraphNodes(_ context.Context, _ string, query string) ([]GraphNode, error) {
	c.meta.mu.RLock()
	defer c.meta.mu.RUnlock()
	graphPath := filepath.Join(filepath.Dir(c.meta.path), graphFile)
	data, err := os.ReadFile(graphPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []GraphNode{}, nil
		}
		return nil, err
	}
	var g struct {
		Nodes []GraphNode `json:"nodes"`
	}
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)

	// Extract individual search terms from query
	terms := strings.Fields(queryLower)
	var significantTerms []string
	for _, t := range terms {
		if len(t) > 2 {
			significantTerms = append(significantTerms, t)
		}
	}

	nodeMatches := func(name string) bool {
		nameLower := strings.ToLower(name)
		// Exact substring match (original behavior)
		if strings.Contains(nameLower, queryLower) {
			return true
		}
		// Reverse: node name contained in query
		if len(nameLower) > 3 && strings.Contains(queryLower, nameLower) {
			return true
		}
		// Word-level match: any significant term matches node name
		for _, term := range significantTerms {
			if strings.Contains(nameLower, term) || strings.Contains(term, nameLower) {
				return true
			}
		}
		return false
	}

	var matches []GraphNode
	for _, node := range g.Nodes {
		if nodeMatches(node.CanonicalName) {
			matches = append(matches, node)
			continue
		}
		for _, alias := range node.Aliases {
			if nodeMatches(alias) {
				matches = append(matches, node)
				break
			}
		}
	}
	return matches, nil
}

// ExtractEntitiesAndRelations uses AI-like heuristics to extract entities
// and relationships from document text. Uses heading/structure-based extraction
// plus keyword pattern matching for domain entities.
func ExtractEntitiesAndRelations(ctx context.Context, text string) ([]GraphNode, []GraphEdge, error) {
	lines := strings.Split(text, "\n")
	var nodes []GraphNode
	var edges []GraphEdge
	nodeMap := make(map[string]int) // canonicalName -> index in nodes

	// Extract entities from headings and bold/keyword patterns
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Headings are primary concepts
		if strings.HasPrefix(line, "##") {
			name := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if name == "" {
				continue
			}
			lowerName := strings.ToLower(name)
			if _, exists := nodeMap[lowerName]; !exists {
				node := GraphNode{
					ID:            fmt.Sprintf("node_%d", i),
					CanonicalName: name,
					Type:          classifyNodeType(name, lines, i),
					Aliases:       []string{},
				}
				nodeMap[lowerName] = len(nodes)
				nodes = append(nodes, node)
			}
		}

		// Detect status/enum patterns: "- **StatusName**: description"
		if (strings.HasPrefix(line, "- **") || strings.HasPrefix(line, "* **")) && strings.Contains(line, "**") {
			start := strings.Index(line, "**") + 2
			end := strings.Index(line[start:], "**")
			if end > 0 {
				name := strings.TrimSpace(line[start : start+end])
				lowerName := strings.ToLower(name)
				if _, exists := nodeMap[lowerName]; !exists && len(name) > 1 && len(name) < 80 {
					node := GraphNode{
						ID:            fmt.Sprintf("node_%d", i),
						CanonicalName: name,
						Type:          "data",
						Aliases:       []string{},
					}
					nodeMap[lowerName] = len(nodes)
					nodes = append(nodes, node)
				}
			}
		}
	}

	// Build edges: sequential concepts relate to each other; items under same heading are siblings
	var lastHeadingIdx int = -1
	for i := 0; i < len(nodes); i++ {
		if nodes[i].Type == "concept" || nodes[i].Type == "process" {
			if lastHeadingIdx >= 0 {
				edges = append(edges, GraphEdge{
					ID:           fmt.Sprintf("edge_%d_%d", lastHeadingIdx, i),
					FromNodeID:   nodes[lastHeadingIdx].ID,
					ToNodeID:     nodes[i].ID,
					RelationType: "relates_to",
					Confidence:   0.6,
				})
			}
			lastHeadingIdx = i
		} else if lastHeadingIdx >= 0 {
			// Data nodes belong to the last heading
			edges = append(edges, GraphEdge{
				ID:           fmt.Sprintf("edge_%d_%d", lastHeadingIdx, i),
				FromNodeID:   nodes[lastHeadingIdx].ID,
				ToNodeID:     nodes[i].ID,
				RelationType: "contains",
				Confidence:   0.7,
			})
		}
	}

	return nodes, edges, nil
}

func classifyNodeType(name string, lines []string, lineIdx int) string {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "quy trình") || strings.Contains(lower, "flow") || strings.Contains(lower, "process") || strings.Contains(lower, "step") {
		return "process"
	}
	if strings.Contains(lower, "rule") || strings.Contains(lower, "quy tắc") || strings.Contains(lower, "điều kiện") {
		return "rule"
	}
	if strings.Contains(lower, "actor") || strings.Contains(lower, "user") || strings.Contains(lower, "khách") || strings.Contains(lower, "admin") {
		return "actor"
	}
	return "concept"
}

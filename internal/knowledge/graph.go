package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// ExtractGraphFromApproved builds a knowledge graph from all approved documents.
// Uses AI to extract entities and relationships from markdown content.
func (c *Client) ExtractGraphFromApproved(ctx context.Context, projectID string, aiExtractor func(context.Context, string) ([]GraphNode, []GraphEdge, error)) error {
	c.meta.mu.RLock()
	approvedDocs := []metaEntry{}
	for _, e := range c.meta.docs {
		if e.ProjectID == projectID && e.Status == "approved" && e.Verified {
			approvedDocs = append(approvedDocs, e)
		}
	}
	c.meta.mu.RUnlock()

	if len(approvedDocs) == 0 {
		return nil
	}

	var allNodes []GraphNode
	var allEdges []GraphEdge
	nodeMap := make(map[string]GraphNode)

	for _, doc := range approvedDocs {
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

	return c.saveGraph(allNodes, allEdges, 1)
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
	var matches []GraphNode
	for _, node := range g.Nodes {
		if strings.Contains(strings.ToLower(node.CanonicalName), queryLower) {
			matches = append(matches, node)
			continue
		}
		for _, alias := range node.Aliases {
			if strings.Contains(strings.ToLower(alias), queryLower) {
				matches = append(matches, node)
				break
			}
		}
	}
	return matches, nil
}

// ExtractEntitiesAndRelations is a placeholder for AI-based extraction.
// In real implementation, this would call ai.Provider with a specialized prompt.
func ExtractEntitiesAndRelations(ctx context.Context, text string) ([]GraphNode, []GraphEdge, error) {
	// Placeholder: simple keyword extraction
	// Real implementation would use LLM with structured output
	lines := strings.Split(text, "\n")
	var nodes []GraphNode
	var edges []GraphEdge

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Simple heuristic: lines with "##" are concepts
		if strings.HasPrefix(line, "##") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "##"))
			nodes = append(nodes, GraphNode{
				ID:            fmt.Sprintf("node_%d", i),
				CanonicalName: name,
				Type:          "concept",
				Aliases:       []string{},
			})
		}
	}

	// Simple edge: sequential concepts depend on each other
	for i := 0; i < len(nodes)-1; i++ {
		edges = append(edges, GraphEdge{
			ID:           fmt.Sprintf("edge_%d", i),
			FromNodeID:   nodes[i].ID,
			ToNodeID:     nodes[i+1].ID,
			RelationType: "relates_to",
			Confidence:   0.6,
		})
	}

	return nodes, edges, nil
}

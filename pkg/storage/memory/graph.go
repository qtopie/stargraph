package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/qtopie/stargraph/pkg/graph"
)

// Graph 纯原生内存拓扑图存储 (支持多跳子图遍历、邻接表与社区报告存取)
type Graph struct {
	mu          sync.RWMutex
	nodes       map[string]*graph.Node
	edges       map[string]*graph.Edge
	adjOut      map[string]map[string]*graph.Edge // source -> target -> edge
	adjIn       map[string]map[string]*graph.Edge // target -> source -> edge
	communities map[string]*graph.Community
}

// NewGraph 创建内存图存储
func NewGraph() *Graph {
	return &Graph{
		nodes:       make(map[string]*graph.Node),
		edges:       make(map[string]*graph.Edge),
		adjOut:      make(map[string]map[string]*graph.Edge),
		adjIn:       make(map[string]map[string]*graph.Edge),
		communities: make(map[string]*graph.Community),
	}
}

func (g *Graph) UpsertNode(ctx context.Context, node *graph.Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[node.ID] = node
	if _, ok := g.adjOut[node.ID]; !ok {
		g.adjOut[node.ID] = make(map[string]*graph.Edge)
	}
	if _, ok := g.adjIn[node.ID]; !ok {
		g.adjIn[node.ID] = make(map[string]*graph.Edge)
	}
	return nil
}

func (g *Graph) UpsertEdge(ctx context.Context, edge *graph.Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.edges[edge.ID] = edge

	if _, ok := g.adjOut[edge.SourceID]; !ok {
		g.adjOut[edge.SourceID] = make(map[string]*graph.Edge)
	}
	g.adjOut[edge.SourceID][edge.TargetID] = edge

	if _, ok := g.adjIn[edge.TargetID]; !ok {
		g.adjIn[edge.TargetID] = make(map[string]*graph.Edge)
	}
	g.adjIn[edge.TargetID][edge.SourceID] = edge

	return nil
}

func (g *Graph) BatchUpsertNodes(ctx context.Context, nodes []*graph.Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, n := range nodes {
		g.nodes[n.ID] = n
		if _, ok := g.adjOut[n.ID]; !ok {
			g.adjOut[n.ID] = make(map[string]*graph.Edge)
		}
		if _, ok := g.adjIn[n.ID]; !ok {
			g.adjIn[n.ID] = make(map[string]*graph.Edge)
		}
	}
	return nil
}

func (g *Graph) BatchUpsertEdges(ctx context.Context, edges []*graph.Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, e := range edges {
		g.edges[e.ID] = e

		if _, ok := g.adjOut[e.SourceID]; !ok {
			g.adjOut[e.SourceID] = make(map[string]*graph.Edge)
		}
		g.adjOut[e.SourceID][e.TargetID] = e

		if _, ok := g.adjIn[e.TargetID]; !ok {
			g.adjIn[e.TargetID] = make(map[string]*graph.Edge)
		}
		g.adjIn[e.TargetID][e.SourceID] = e
	}
	return nil
}

func (g *Graph) GetNode(ctx context.Context, id string) (*graph.Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	n, ok := g.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	return n, nil
}

func (g *Graph) GetEdge(ctx context.Context, id string) (*graph.Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	e, ok := g.edges[id]
	if !ok {
		return nil, fmt.Errorf("edge not found: %s", id)
	}
	return e, nil
}

func (g *Graph) GetNodesByIDs(ctx context.Context, ids []string) ([]*graph.Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	res := make([]*graph.Node, 0, len(ids))
	for _, id := range ids {
		if n, ok := g.nodes[id]; ok {
			res = append(res, n)
		}
	}
	return res, nil
}

// GetNeighbors 获取节点的邻居与关联边 (direction: "out", "in", "both")
func (g *Graph) GetNeighbors(ctx context.Context, nodeID string, direction string) ([]*graph.Node, []*graph.Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	neighborIDs := make(map[string]struct{})
	edgeList := make([]*graph.Edge, 0)

	if direction == "out" || direction == "both" || direction == "" {
		if out, ok := g.adjOut[nodeID]; ok {
			for tgt, edge := range out {
				neighborIDs[tgt] = struct{}{}
				edgeList = append(edgeList, edge)
			}
		}
	}

	if direction == "in" || direction == "both" || direction == "" {
		if in, ok := g.adjIn[nodeID]; ok {
			for src, edge := range in {
				neighborIDs[src] = struct{}{}
				edgeList = append(edgeList, edge)
			}
		}
	}

	nodes := make([]*graph.Node, 0, len(neighborIDs))
	for nid := range neighborIDs {
		if n, ok := g.nodes[nid]; ok {
			nodes = append(nodes, n)
		}
	}

	return nodes, edgeList, nil
}

// GetSubgraph 执行 BFS 多跳子图遍历，返回触达的所有节点与边
func (g *Graph) GetSubgraph(ctx context.Context, startNodeIDs []string, maxHops int) ([]*graph.Node, []*graph.Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visitedNodes := make(map[string]bool)
	visitedEdges := make(map[string]bool)

	currentLevel := make([]string, 0)
	for _, id := range startNodeIDs {
		if _, ok := g.nodes[id]; ok {
			visitedNodes[id] = true
			currentLevel = append(currentLevel, id)
		}
	}

	if maxHops <= 0 {
		maxHops = 1
	}

	for hop := 0; hop < maxHops; hop++ {
		if len(currentLevel) == 0 {
			break
		}
		nextLevel := make([]string, 0)

		for _, currID := range currentLevel {
			// Outbound edges
			if out, ok := g.adjOut[currID]; ok {
				for tgt, edge := range out {
					visitedEdges[edge.ID] = true
					if !visitedNodes[tgt] {
						visitedNodes[tgt] = true
						nextLevel = append(nextLevel, tgt)
					}
				}
			}
			// Inbound edges
			if in, ok := g.adjIn[currID]; ok {
				for src, edge := range in {
					visitedEdges[edge.ID] = true
					if !visitedNodes[src] {
						visitedNodes[src] = true
						nextLevel = append(nextLevel, src)
					}
				}
			}
		}
		currentLevel = nextLevel
	}

	resultNodes := make([]*graph.Node, 0, len(visitedNodes))
	for nid := range visitedNodes {
		if n, ok := g.nodes[nid]; ok {
			resultNodes = append(resultNodes, n)
		}
	}

	resultEdges := make([]*graph.Edge, 0, len(visitedEdges))
	for eid := range visitedEdges {
		if e, ok := g.edges[eid]; ok {
			resultEdges = append(resultEdges, e)
		}
	}

	return resultNodes, resultEdges, nil
}

func (g *Graph) GetAllNodes(ctx context.Context) ([]*graph.Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*graph.Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (g *Graph) GetAllEdges(ctx context.Context) ([]*graph.Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges := make([]*graph.Edge, 0, len(g.edges))
	for _, e := range g.edges {
		edges = append(edges, e)
	}
	return edges, nil
}

func (g *Graph) UpsertCommunity(ctx context.Context, comm *graph.Community) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.communities[comm.ID] = comm
	return nil
}

func (g *Graph) GetCommunity(ctx context.Context, id string) (*graph.Community, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	c, ok := g.communities[id]
	if !ok {
		return nil, fmt.Errorf("community not found: %s", id)
	}
	return c, nil
}

func (g *Graph) GetCommunitiesByLevel(ctx context.Context, level int) ([]*graph.Community, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	res := make([]*graph.Community, 0)
	for _, c := range g.communities {
		if c.Level == level {
			res = append(res, c)
		}
	}
	return res, nil
}

func (g *Graph) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes = make(map[string]*graph.Node)
	g.edges = make(map[string]*graph.Edge)
	g.adjOut = make(map[string]map[string]*graph.Edge)
	g.adjIn = make(map[string]map[string]*graph.Edge)
	g.communities = make(map[string]*graph.Community)
	return nil
}

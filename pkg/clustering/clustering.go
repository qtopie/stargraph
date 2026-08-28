package clustering

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/qtopie/stargraph/pkg/graph"
	"github.com/qtopie/stargraph/pkg/storage"
)

// Config 图聚类配置
type Config struct {
	MaxClusterSize int
	Seed           int64
}

// DefaultConfig 默认聚类配置
func DefaultConfig() Config {
	return Config{
		MaxClusterSize: 10,
		Seed:           42,
	}
}

// Clusterer 负责在图拓扑上发现连通子图与层次化社区 (分层连通分量与贪心社区划分)
type Clusterer struct {
	cfg Config
}

// NewClusterer 创建聚类器
func NewClusterer(cfg Config) *Clusterer {
	if cfg.MaxClusterSize <= 0 {
		cfg.MaxClusterSize = 10
	}
	return &Clusterer{cfg: cfg}
}

// DetectCommunities 在 GraphStorage 上执行层次化图拓扑聚类，返回多层级社区定义
func (c *Clusterer) DetectCommunities(ctx context.Context, g storage.GraphStorage) ([]*graph.Community, error) {
	nodes, err := g.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all nodes: %w", err)
	}
	if len(nodes) == 0 {
		return nil, nil
	}

	edges, err := g.GetAllEdges(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all edges: %w", err)
	}

	// 1. 构建无向邻接关系
	adj := make(map[string]map[string]bool)
	for _, n := range nodes {
		adj[n.ID] = make(map[string]bool)
	}
	for _, e := range edges {
		if _, ok := adj[e.SourceID]; ok {
			adj[e.SourceID][e.TargetID] = true
		}
		if _, ok := adj[e.TargetID]; ok {
			adj[e.TargetID][e.SourceID] = true
		}
	}

	// 2. 查找连通分量 (Connected Components) 作为基底社区
	visited := make(map[string]bool)
	var rawClusters [][]string

	for _, n := range nodes {
		if visited[n.ID] {
			continue
		}
		var cluster []string
		queue := []string{n.ID}
		visited[n.ID] = true

		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			cluster = append(cluster, curr)

			for neighbor := range adj[curr] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}
		rawClusters = append(rawClusters, cluster)
	}

	// 3. 构建 Level 0 社区 (微观社区，按 MaxClusterSize 进行细分)
	var level0Communities []*graph.Community
	commIdx := 0

	for _, cluster := range rawClusters {
		// 如果一个连通分量超过 MaxClusterSize，做简单分片
		for i := 0; i < len(cluster); i += c.cfg.MaxClusterSize {
			end := i + c.cfg.MaxClusterSize
			if end > len(cluster) {
				end = len(cluster)
			}
			chunkNodes := cluster[i:end]

			nodeIDSet := make(map[string]bool)
			for _, nid := range chunkNodes {
				nodeIDSet[nid] = true
			}

			// 收集该社区涉及的边
			var commEdgeIDs []string
			for _, e := range edges {
				if nodeIDSet[e.SourceID] || nodeIDSet[e.TargetID] {
					commEdgeIDs = append(commEdgeIDs, e.ID)
				}
			}

			commID := fmt.Sprintf("comm:l0:%d", commIdx)
			commIdx++

			level0Communities = append(level0Communities, &graph.Community{
				ID:        commID,
				Level:     0,
				Title:     fmt.Sprintf("Community L0 #%d", commIdx),
				NodeIDs:   chunkNodes,
				EdgeIDs:   commEdgeIDs,
				Rating:    float64(len(chunkNodes)),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})
		}
	}

	// 4. 构建 Level 1 宏观社区 (自底向上合并)
	var allCommunities []*graph.Community
	allCommunities = append(allCommunities, level0Communities...)

	if len(level0Communities) > 1 {
		// 简单合并为 1~2 个宏观社区 (Level 1)
		l1NodeSet := make(map[string]bool)
		l1EdgeSet := make(map[string]bool)
		for _, c0 := range level0Communities {
			for _, nid := range c0.NodeIDs {
				l1NodeSet[nid] = true
			}
			for _, eid := range c0.EdgeIDs {
				l1EdgeSet[eid] = true
			}
		}

		l1Nodes := make([]string, 0, len(l1NodeSet))
		for nid := range l1NodeSet {
			l1Nodes = append(l1Nodes, nid)
		}
		sort.Strings(l1Nodes)

		l1Edges := make([]string, 0, len(l1EdgeSet))
		for eid := range l1EdgeSet {
			l1Edges = append(l1Edges, eid)
		}
		sort.Strings(l1Edges)

		l1Comm := &graph.Community{
			ID:        "comm:l1:0",
			Level:     1,
			Title:     "Global Macro Community L1",
			NodeIDs:   l1Nodes,
			EdgeIDs:   l1Edges,
			Rating:    float64(len(l1Nodes)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		allCommunities = append(allCommunities, l1Comm)
	}

	return allCommunities, nil
}

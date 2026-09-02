package query

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/graph"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/search"
	"github.com/qtopie/stargraph/pkg/storage"
)

// DRIFTConfig DRIFT (Dynamic Reasoning and Inference with Flexible Traversal) 检索配置
type DRIFTConfig struct {
	MaxDepth         int     // 最大路径穿梭深度 (默认 3)
	BeamWidth        int     // 每跳保留的分支宽度 (默认 5)
	MinScore         float64 // 剪枝最低相似度/权重分 (默认 0.05)
	MaxContextChunks int     // 最大组装 Chunk 数量 (默认 8)
}

// DefaultDRIFTConfig 默认 DRIFT 检索配置
func DefaultDRIFTConfig() DRIFTConfig {
	return DRIFTConfig{
		MaxDepth:         3,
		BeamWidth:        5,
		MinScore:         0.05,
		MaxContextChunks: 8,
	}
}

// DRIFTSearcher DRIFT 动态路径穿梭检索引擎
type DRIFTSearcher struct {
	llmClient   llm.Client
	embedClient llm.EmbeddingClient
	graphStore  storage.GraphStorage
	vectorStore storage.VectorStorage
	kvStore     storage.KVStorage
	cfg         DRIFTConfig
}

// NewDRIFTSearcher 创建 DRIFT 检索引擎实例
func NewDRIFTSearcher(
	llmClient llm.Client,
	embedClient llm.EmbeddingClient,
	graphStore storage.GraphStorage,
	vectorStore storage.VectorStorage,
	kvStore storage.KVStorage,
	cfg ...DRIFTConfig,
) *DRIFTSearcher {
	c := DefaultDRIFTConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if c.MaxDepth <= 0 {
		c.MaxDepth = 3
	}
	if c.BeamWidth <= 0 {
		c.BeamWidth = 5
	}
	if c.MaxContextChunks <= 0 {
		c.MaxContextChunks = 8
	}

	return &DRIFTSearcher{
		llmClient:   llmClient,
		embedClient: embedClient,
		graphStore:  graphStore,
		vectorStore: vectorStore,
		kvStore:     kvStore,
		cfg:         c,
	}
}

type scoredNode struct {
	node  *graph.Node
	score float64
	depth int
}

// Search 执行 DRIFT 启发式动态穿梭检索
func (d *DRIFTSearcher) Search(ctx context.Context, req *search.Request) (*search.Result, error) {
	if req.Query == "" {
		return &search.Result{Answer: "Query is empty"}, nil
	}

	// 1. 种子实体定位 (Seed Entity Localization)
	seedNodes, err := d.locateSeeds(ctx, req.Query, d.cfg.BeamWidth)
	if err != nil {
		return nil, fmt.Errorf("drift seed localization: %w", err)
	}

	// 2. 启发式多跳动态穿梭与剪枝 (Dynamic Path Traversal & Heuristic Pruning)
	visitedNodes := make(map[string]*graph.Node)
	traversedEdges := make(map[string]*graph.Edge)
	chunkIDSet := make(map[string]struct{})

	currentFrontier := make([]*scoredNode, 0, len(seedNodes))
	for _, sn := range seedNodes {
		visitedNodes[sn.node.ID] = sn.node
		currentFrontier = append(currentFrontier, sn)
		for _, cid := range sn.node.SourceChunkIDs {
			chunkIDSet[cid] = struct{}{}
		}
	}

	maxDepth := d.cfg.MaxDepth
	if req.MaxHops > 0 {
		maxDepth = req.MaxHops
	}

	for depth := 1; depth <= maxDepth && len(currentFrontier) > 0; depth++ {
		nextCandidates := make([]*scoredNode, 0)
		decay := 1.0 / float64(depth)

		for _, current := range currentFrontier {
			_, edges, err := d.graphStore.GetNeighbors(ctx, current.node.ID, "both")
			if err != nil {
				continue
			}

			for _, edge := range edges {
				targetID := edge.TargetID
				if targetID == current.node.ID {
					targetID = edge.SourceID
				}

				traversedEdges[edge.ID] = edge
				for _, cid := range edge.SourceChunkIDs {
					chunkIDSet[cid] = struct{}{}
				}

				if _, seen := visitedNodes[targetID]; seen {
					continue
				}

				targetNode, err := d.graphStore.GetNode(ctx, targetID)
				if err != nil || targetNode == nil {
					continue
				}

				// 计算候选节点启发式分数：前驱得分 + 边权重 * 衰减系数
				edgeWeight := edge.Weight
				if edgeWeight <= 0 {
					edgeWeight = 1.0
				}
				candScore := current.score + (edgeWeight * decay)

				if candScore >= d.cfg.MinScore {
					nextCandidates = append(nextCandidates, &scoredNode{
						node:  targetNode,
						score: candScore,
						depth: depth,
					})
				}
			}
		}

		// Beam 剪枝：按得分降序排序，仅保留 Top BeamWidth
		sort.Slice(nextCandidates, func(i, j int) bool {
			return nextCandidates[i].score > nextCandidates[j].score
		})

		currentFrontier = make([]*scoredNode, 0)
		for i, cand := range nextCandidates {
			if i >= d.cfg.BeamWidth {
				break
			}
			if _, exists := visitedNodes[cand.node.ID]; !exists {
				visitedNodes[cand.node.ID] = cand.node
				currentFrontier = append(currentFrontier, cand)
				for _, cid := range cand.node.SourceChunkIDs {
					chunkIDSet[cid] = struct{}{}
				}
			}
		}
	}

	// 3. 从 KV 存储中拉取精准证据 Chunks
	sourceChunks := make([]*document.Chunk, 0)
	if d.kvStore != nil {
		for cid := range chunkIDSet {
			data, err := d.kvStore.Get(ctx, cid)
			if err == nil && len(data) > 0 {
				var chunk document.Chunk
				if err := json.Unmarshal(data, &chunk); err == nil {
					sourceChunks = append(sourceChunks, &chunk)
					if len(sourceChunks) >= d.cfg.MaxContextChunks {
						break
					}
				}
			}
		}
	}

	// 4. 组装高精度紧凑上下文 (Compact Focused Context)
	nodeList := make([]*graph.Node, 0, len(visitedNodes))
	for _, n := range visitedNodes {
		nodeList = append(nodeList, n)
	}

	edgeList := make([]*graph.Edge, 0, len(traversedEdges))
	for _, e := range traversedEdges {
		edgeList = append(edgeList, e)
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("### Traversed Graph Knowledge (Entities & Relations):\n")
	for _, n := range nodeList {
		fmt.Fprintf(&contextBuilder, "- Entity [%s] (%s): %s\n", n.Name, n.Type, n.Description)
	}
	for _, e := range edgeList {
		fmt.Fprintf(&contextBuilder, "- Relation [%s -> %s] (%s): %s\n", e.SourceID, e.TargetID, e.Relation, e.Description)
	}

	if len(sourceChunks) > 0 {
		contextBuilder.WriteString("\n### Document Text Evidence:\n")
		for _, c := range sourceChunks {
			fmt.Fprintf(&contextBuilder, "--- Chunk [%s] ---\n%s\n", c.ID, c.Content)
		}
	}

	// 5. LLM 生成精准答案
	prompt := fmt.Sprintf(`--- Role & Objective ---
You are an expert technical assistant with dynamic graph traversal capabilities (DRIFT Engine).
Answer the user's technical question accurately, concisely, and strictly based on the provided graph paths and text evidence.

--- Context & Evidence ---
%s

--- User Question ---
%s

--- Grounded Response ---
`, contextBuilder.String(), req.Query)

	answer, err := d.llmClient.Complete(ctx, prompt, llm.WithTemperature(0.1))
	if err != nil {
		return nil, fmt.Errorf("drift llm response generation: %w", err)
	}

	return &search.Result{
		Answer:       strings.TrimSpace(answer),
		SourceChunks: sourceChunks,
		Nodes:        nodeList,
		Edges:        edgeList,
		Metadata: map[string]any{
			"search_mode":     "drift",
			"traversed_nodes": len(nodeList),
			"traversed_edges": len(edgeList),
		},
	}, nil
}

func (d *DRIFTSearcher) locateSeeds(ctx context.Context, query string, topK int) ([]*scoredNode, error) {
	seeds := make([]*scoredNode, 0)
	seen := make(map[string]struct{})

	// 优先通过向量检索种子节点
	if d.embedClient != nil && d.vectorStore != nil {
		qVec, err := d.embedClient.Embed(ctx, query)
		if err == nil {
			vResults, err := d.vectorStore.Search(ctx, qVec, topK*2, map[string]interface{}{"type": "entity"})
			if err == nil {
				for _, vr := range vResults {
					if !strings.HasPrefix(vr.ID, "entity:") {
						continue
					}
					node, err := d.graphStore.GetNode(ctx, vr.ID)
					if err == nil && node != nil {
						if _, exists := seen[node.ID]; !exists {
							seen[node.ID] = struct{}{}
							seeds = append(seeds, &scoredNode{
								node:  node,
								score: float64(vr.Score),
								depth: 0,
							})
						}
					}
					if len(seeds) >= topK {
						break
					}
				}
			}
		}
	}

	// 文本关键词匹配种子补全 (针对 0-Token 纯离线场景)
	if len(seeds) < topK {
		qWords := strings.Fields(strings.ToUpper(query))
		allNodes, _ := d.graphStore.GetAllNodes(ctx)
		for _, n := range allNodes {
			if _, exists := seen[n.ID]; exists {
				continue
			}
			normName := strings.ToUpper(n.Name)
			matched := false
			for _, w := range qWords {
				if len(w) > 2 && (strings.Contains(normName, w) || strings.Contains(w, normName)) {
					matched = true
					break
				}
			}
			if matched {
				seen[n.ID] = struct{}{}
				seeds = append(seeds, &scoredNode{
					node:  n,
					score: 1.0,
					depth: 0,
				})
				if len(seeds) >= topK {
					break
				}
			}
		}
	}

	return seeds, nil
}

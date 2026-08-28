package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/indexing"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/search"
	"github.com/qtopie/stargraph/pkg/storage"
)

// LocalSearcher 负责基于实体向量匹配与多跳子图遍历的精确局部推理
type LocalSearcher struct {
	llmClient   llm.Client
	embedClient llm.EmbeddingClient
	graphStore  storage.GraphStorage
	vectorStore storage.VectorStorage
	kvStore     storage.KVStorage
}

// NewLocalSearcher 创建 Local Search 实例
func NewLocalSearcher(
	llmClient llm.Client,
	embedClient llm.EmbeddingClient,
	graphStore storage.GraphStorage,
	vectorStore storage.VectorStorage,
	kvStore storage.KVStorage,
) *LocalSearcher {
	return &LocalSearcher{
		llmClient:   llmClient,
		embedClient: embedClient,
		graphStore:  graphStore,
		vectorStore: vectorStore,
		kvStore:     kvStore,
	}
}

// Search 执行 Local Search 查询
func (s *LocalSearcher) Search(ctx context.Context, req *search.Request) (*search.Result, error) {
	if req.TopK <= 0 {
		req.TopK = 5
	}
	if req.MaxHops <= 0 {
		req.MaxHops = 2
	}

	// 1. 向量相似度检索种子实体
	var matchedEntityIDs []string
	if s.embedClient != nil && s.vectorStore != nil {
		qVec, err := s.embedClient.Embed(ctx, req.Query)
		if err == nil {
			vResults, err := s.vectorStore.Search(ctx, qVec, req.TopK, map[string]interface{}{"type": "entity"})
			if err == nil {
				for _, vr := range vResults {
					matchedEntityIDs = append(matchedEntityIDs, vr.ID)
				}
			}
		}
	}

	// 降级：如果向量未命中，尝试直接用所有节点或名字匹配
	if len(matchedEntityIDs) == 0 {
		allNodes, _ := s.graphStore.GetAllNodes(ctx)
		for _, n := range allNodes {
			if strings.Contains(strings.ToLower(req.Query), strings.ToLower(n.Name)) ||
				strings.Contains(strings.ToLower(n.Name), strings.ToLower(req.Query)) {
				matchedEntityIDs = append(matchedEntityIDs, n.ID)
			}
		}
		if len(matchedEntityIDs) == 0 && len(allNodes) > 0 {
			// 若全未命中，取前几个作为上下文
			limit := req.TopK
			if limit > len(allNodes) {
				limit = len(allNodes)
			}
			for i := 0; i < limit; i++ {
				matchedEntityIDs = append(matchedEntityIDs, allNodes[i].ID)
			}
		}
	}

	// 2. 多跳子图遍历 (1~2 Hops Subgraph Expansion)
	subgraphNodes, subgraphEdges, err := s.graphStore.GetSubgraph(ctx, matchedEntityIDs, req.MaxHops)
	if err != nil {
		return nil, fmt.Errorf("get subgraph: %w", err)
	}

	// 3. 关联原始文档 Chunk 与社区报告
	chunkIDSet := make(map[string]struct{})
	for _, n := range subgraphNodes {
		for _, cid := range n.SourceChunkIDs {
			chunkIDSet[cid] = struct{}{}
		}
	}
	for _, e := range subgraphEdges {
		for _, cid := range e.SourceChunkIDs {
			chunkIDSet[cid] = struct{}{}
		}
	}

	sourceChunks := make([]*document.Chunk, 0)
	if s.kvStore != nil {
		for cid := range chunkIDSet {
			data, err := s.kvStore.Get(ctx, cid)
			if err == nil && len(data) > 0 {
				var chunk document.Chunk
				if err := json.Unmarshal(data, &chunk); err == nil {
					sourceChunks = append(sourceChunks, &chunk)
				}
			}
		}
	}

	// 4. 构建上下文 (Context Prompt)
	var ctxBuilder strings.Builder

	ctxBuilder.WriteString("-----Entities-----\n")
	for _, n := range subgraphNodes {
		fmt.Fprintf(&ctxBuilder, "ID: %s, Name: %s, Type: %s, Description: %s\n", n.ID, n.Name, n.Type, n.Description)
	}

	ctxBuilder.WriteString("\n-----Relationships-----\n")
	for _, e := range subgraphEdges {
		fmt.Fprintf(&ctxBuilder, "Source: %s -> Target: %s, Relation: %s, Description: %s\n", e.SourceID, e.TargetID, e.Relation, e.Description)
	}

	if len(sourceChunks) > 0 {
		ctxBuilder.WriteString("\n-----Source Text Units-----\n")
		for _, ch := range sourceChunks {
			fmt.Fprintf(&ctxBuilder, "[%s]: %s\n", ch.ID, ch.Content)
		}
	}

	// 5. LLM 生成回答
	systemPrompt := fmt.Sprintf(indexing.LocalSearchSystemPrompt, ctxBuilder.String())
	answer, err := s.llmClient.CompleteWithSystem(ctx, systemPrompt, req.Query)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	return &search.Result{
		Answer:       answer,
		SourceChunks: sourceChunks,
		Nodes:        subgraphNodes,
		Edges:        subgraphEdges,
		Metadata: map[string]any{
			"matched_entities": len(matchedEntityIDs),
			"subgraph_nodes":   len(subgraphNodes),
			"subgraph_edges":   len(subgraphEdges),
		},
	}, nil
}

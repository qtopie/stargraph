package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qtopie/stargraph/pkg/graph"
	"github.com/qtopie/stargraph/pkg/indexing"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/search"
	"github.com/qtopie/stargraph/pkg/storage"
)

// GlobalSearcher 负责基于社区报告的层次化 Map-Reduce 宏观总结
type GlobalSearcher struct {
	llmClient  llm.Client
	graphStore storage.GraphStorage
}

// NewGlobalSearcher 创建 Global Search 实例
func NewGlobalSearcher(llmClient llm.Client, graphStore storage.GraphStorage) *GlobalSearcher {
	return &GlobalSearcher{
		llmClient:  llmClient,
		graphStore: graphStore,
	}
}

type mapResultJSON struct {
	Points []struct {
		Description string  `json:"description"`
		Score       float64 `json:"score"`
	} `json:"points"`
}

// Search 执行 Global Search (Map-Reduce 检索)
func (s *GlobalSearcher) Search(ctx context.Context, req *search.Request) (*search.Result, error) {
	// 1. 获取指定或所有层级的社区报告
	var targetCommunities []*graph.Community
	var err error

	if req.MaxLevel > 0 {
		targetCommunities, err = s.graphStore.GetCommunitiesByLevel(ctx, req.MaxLevel)
	} else {
		// 默认优先获取 Level 0 或全部
		targetCommunities, err = s.graphStore.GetCommunitiesByLevel(ctx, 0)
		if len(targetCommunities) == 0 {
			targetCommunities, err = s.graphStore.GetCommunitiesByLevel(ctx, 1)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("get communities: %w", err)
	}

	if len(targetCommunities) == 0 {
		// 降级：无可用的社区报告，退化为基础直接回答
		ans, err := s.llmClient.Complete(ctx, "Query: "+req.Query+"\n(No knowledge graph community reports available)")
		if err != nil {
			return nil, err
		}
		return &search.Result{
			Answer: ans,
		}, nil
	}

	// 2. Map 阶段：将社区报告分组输入 LLM 生成中间观察点
	var reportBuilder strings.Builder
	for _, comm := range targetCommunities {
		fmt.Fprintf(&reportBuilder, "=== Community: %s (Level %d) ===\nSummary: %s\nRating: %.1f\n\n",
			comm.Title, comm.Level, comm.Summary, comm.Rating)
	}

	mapPrompt := fmt.Sprintf(indexing.GlobalSearchMapPrompt, req.Query, reportBuilder.String())
	mapResp, err := s.llmClient.Complete(ctx, mapPrompt, llm.WithJSONMode(true))
	if err != nil {
		return nil, fmt.Errorf("global search map phase: %w", err)
	}

	var mapResult mapResultJSON
	var keyFindings strings.Builder

	if err := json.Unmarshal([]byte(mapResp), &mapResult); err == nil && len(mapResult.Points) > 0 {
		for _, pt := range mapResult.Points {
			fmt.Fprintf(&keyFindings, "- %s (Score: %.1f)\n", pt.Description, pt.Score)
		}
	} else {
		keyFindings.WriteString(mapResp)
	}

	// 3. Reduce 阶段：整合关键洞察生成宏观全局总结
	reducePrompt := fmt.Sprintf(indexing.GlobalSearchReducePrompt, req.Query, keyFindings.String())
	finalAnswer, err := s.llmClient.Complete(ctx, reducePrompt)
	if err != nil {
		return nil, fmt.Errorf("global search reduce phase: %w", err)
	}

	return &search.Result{
		Answer:      finalAnswer,
		Communities: targetCommunities,
		Metadata: map[string]any{
			"communities_analyzed": len(targetCommunities),
		},
	}, nil
}

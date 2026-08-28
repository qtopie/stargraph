package stargraph_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/qtopie/stargraph/pkg"
	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/indexing"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/search"
)

// MockLLMClient 模拟 LLM 响应，用于确定性单元测试与 Harness 校验
type MockLLMClient struct{}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string, opts ...llm.Option) (string, error) {
	if strings.Contains(prompt, "-Goal-") {
		// Entity extraction mock response
		return fmt.Sprintf(`("entity"%s"StarGraph"%s"technology"%s"StarGraph is a high performance GraphRAG engine written in pure Go.")%s("entity"%s"CosmosStar"%s"organization"%s"CosmosStar is the parent ecosystem.")%s("relationship"%s"StarGraph"%s"CosmosStar"%s"StarGraph belongs to the CosmosStar ecosystem."%s9)%s`,
			indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
			indexing.RecordDelimiter,
			indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
			indexing.RecordDelimiter,
			indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
			indexing.CompletionDelimiter,
		), nil
	}

	if strings.Contains(prompt, "Community") || strings.Contains(prompt, "community") {
		return `{"title":"StarGraph Ecosystem","summary":"A cluster focusing on StarGraph and CosmosStar graph capabilities.","rating":8.5}`, nil
	}

	if strings.Contains(prompt, "GlobalSearchMapPrompt") || strings.Contains(prompt, "points") {
		return `{"points":[{"description":"StarGraph provides Go-native GraphRAG capabilities.","score":9.0}]}`, nil
	}

	return "StarGraph is a pure Go lightweight GraphRAG core engine designed for high performance.", nil
}

func (m *MockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string, opts ...llm.Option) (string, error) {
	return "StarGraph is an in-memory GraphRAG engine connected to the CosmosStar ecosystem.", nil
}

// MockEmbedClient 模拟 Embedding 生成
type MockEmbedClient struct{}

func (m *MockEmbedClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}

func (m *MockEmbedClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = []float32{0.1, 0.2, 0.3, 0.4}
	}
	return res, nil
}

func TestStarGraph_EndToEnd(t *testing.T) {
	ctx := context.Background()
	mockLLM := &MockLLMClient{}
	mockEmbed := &MockEmbedClient{}

	engine := stargraph.NewEngine(mockLLM, mockEmbed, stargraph.DefaultConfig())
	defer func() { _ = engine.Close() }()

	doc := &document.Document{
		ID:      "doc-stargraph-01",
		Content: "StarGraph is a lightweight GraphRAG core engine developed for CosmosStar. It features native Go performance and sub-graph traversal.",
	}

	// 1. 测试索引流水线
	if err := engine.Insert(ctx, doc); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// 2. 测试 Local Search 查询
	localReq := &search.Request{
		Query:   "What is StarGraph?",
		Mode:    search.ModeLocal,
		TopK:    5,
		MaxHops: 2,
	}
	localRes, err := engine.Query(ctx, localReq)
	if err != nil {
		t.Fatalf("Local Query failed: %v", err)
	}
	if localRes.Answer == "" {
		t.Errorf("expected non-empty local search answer")
	}
	if len(localRes.Nodes) == 0 {
		t.Errorf("expected matched nodes in local search result")
	}

	// 3. 测试 Global Search 查询
	globalReq := &search.Request{
		Query: "Give me an overview summary of all components",
		Mode:  search.ModeGlobal,
	}
	globalRes, err := engine.Query(ctx, globalReq)
	if err != nil {
		t.Fatalf("Global Query failed: %v", err)
	}
	if globalRes.Answer == "" {
		t.Errorf("expected non-empty global search answer")
	}

	// 4. 测试 Auto 路由模式
	autoReq := &search.Request{
		Query: "What is StarGraph?",
		Mode:  search.ModeAuto,
	}
	autoRes, err := engine.Query(ctx, autoReq)
	if err != nil {
		t.Fatalf("Auto Query failed: %v", err)
	}
	if autoRes.Answer == "" {
		t.Errorf("expected non-empty auto search answer")
	}
}

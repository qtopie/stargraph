package query

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/graph"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/search"
	"github.com/qtopie/stargraph/pkg/storage/memory"
)

type mockDRIFTLLM struct{}

func (m *mockDRIFTLLM) Complete(ctx context.Context, prompt string, opts ...llm.Option) (string, error) {
	if strings.Contains(prompt, "DRIFT Engine") {
		return "The I2C deadlock is caused by missing pull-up resistor on CLK_PIN when REG_I2C_CTRL is reset.", nil
	}
	return "Mock response", nil
}

func (m *mockDRIFTLLM) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string, opts ...llm.Option) (string, error) {
	return m.Complete(ctx, userPrompt, opts...)
}

type mockDRIFTEmbed struct{}

func (m *mockDRIFTEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{1.0, 0.0, 0.0}, nil
}

func (m *mockDRIFTEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = []float32{1.0, 0.0, 0.0}
	}
	return res, nil
}

func TestDRIFTSearch(t *testing.T) {
	ctx := context.Background()
	graphStore := memory.NewGraph()
	vectorStore := memory.NewVector()
	kvStore := memory.NewKV()
	llmClient := &mockDRIFTLLM{}
	embedClient := &mockDRIFTEmbed{}

	// 1. 设置节点
	nodeA := &graph.Node{
		ID:             "entity:REG_I2C_CTRL",
		Name:           "REG_I2C_CTRL",
		Type:           "REGISTER",
		Description:    "Primary I2C control register",
		SourceChunkIDs: []string{"chunk-1"},
	}
	nodeB := &graph.Node{
		ID:             "entity:CLK_PIN",
		Name:           "CLK_PIN",
		Type:           "PIN",
		Description:    "I2C Clock line pin requiring external pull-up",
		SourceChunkIDs: []string{"chunk-2"},
	}
	nodeC := &graph.Node{
		ID:             "entity:I2C_DEADLOCK",
		Name:           "I2C_DEADLOCK",
		Type:           "FAULT",
		Description:    "Bus lockup condition when clock line is held low",
		SourceChunkIDs: []string{"chunk-2"},
	}

	_ = graphStore.BatchUpsertNodes(ctx, []*graph.Node{nodeA, nodeB, nodeC})
	_ = vectorStore.UpsertVector(ctx, nodeA.ID, []float32{1.0, 0.0, 0.0}, map[string]interface{}{"type": "entity"})

	// 2. 设置多跳边
	edgeAB := &graph.Edge{
		ID:             "rel:REG_I2C_CTRL<->CLK_PIN",
		SourceID:       nodeA.ID,
		TargetID:       nodeB.ID,
		Relation:       "DRIVES",
		Description:    "Register drives the clock pin state",
		Weight:         1.0,
		SourceChunkIDs: []string{"chunk-1"},
	}
	edgeBC := &graph.Edge{
		ID:             "rel:CLK_PIN<->I2C_DEADLOCK",
		SourceID:       nodeB.ID,
		TargetID:       nodeC.ID,
		Relation:       "TRIGGERS_ON_FAILURE",
		Description:    "Floating clock line causes deadlock",
		Weight:         0.9,
		SourceChunkIDs: []string{"chunk-2"},
	}
	_ = graphStore.BatchUpsertEdges(ctx, []*graph.Edge{edgeAB, edgeBC})

	// 3. 设置证据 Chunks
	c1Data, _ := json.Marshal(&document.Chunk{
		ID:      "chunk-1",
		Content: "REG_I2C_CTRL directly drives the CLK_PIN timing.",
	})
	c2Data, _ := json.Marshal(&document.Chunk{
		ID:      "chunk-2",
		Content: "If CLK_PIN lacks a pull-up resistor, I2C_DEADLOCK will occur on bus reset.",
	})
	_ = kvStore.Set(ctx, "chunk-1", c1Data)
	_ = kvStore.Set(ctx, "chunk-2", c2Data)

	// 4. 执行 DRIFT 搜索
	searcher := NewDRIFTSearcher(llmClient, embedClient, graphStore, vectorStore, kvStore, DRIFTConfig{
		MaxDepth:  3,
		BeamWidth: 5,
	})

	req := &search.Request{
		Query:   "Trace cause of I2C deadlock with REG_I2C_CTRL",
		Mode:    search.ModeDRIFT,
		MaxHops: 2,
	}

	result, err := searcher.Search(ctx, req)
	if err != nil {
		t.Fatalf("DRIFT Search failed: %v", err)
	}

	if result.Answer == "" {
		t.Fatalf("Expected non-empty answer")
	}

	if len(result.Nodes) < 2 {
		t.Errorf("Expected at least 2 traversed nodes, got %d", len(result.Nodes))
	}
	if len(result.Edges) < 1 {
		t.Errorf("Expected at least 1 traversed edge, got %d", len(result.Edges))
	}
	if len(result.SourceChunks) == 0 {
		t.Errorf("Expected retrieved source chunks")
	}
}

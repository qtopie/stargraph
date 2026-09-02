package indexing

import (
	"context"
	"testing"

	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/storage/memory"
)

func TestCooccurrenceExtractor(t *testing.T) {
	ctx := context.Background()
	graphStore := memory.NewGraph()
	vectorStore := memory.NewVector()

	cfg := CooccurrenceConfig{
		WindowSize:     10,
		MinCooccurFreq: 2,
		MinEdgeWeight:  0.05,
	}

	extractor := NewCooccurrenceExtractor(graphStore, vectorStore, nil, cfg)

	chunks := []*document.Chunk{
		{
			ID: "chunk-01",
			Content: "The microcontroller STM32F4 communicates via REG_I2C_CTRL register. " +
				"When CLK_EDGE_RISING triggers, REG_I2C_CTRL updates the peripheral status.",
		},
		{
			ID: "chunk-02",
			Content: "In datasheet section 4, REG_I2C_CTRL is configured for fast mode. " +
				"Ensure CLK_EDGE_RISING timing constraint is respected during high frequency transmission.",
		},
	}

	if err := extractor.ExtractChunks(ctx, chunks); err != nil {
		t.Fatalf("ExtractChunks failed: %v", err)
	}

	// 验证实体生成
	nodeReg, err := graphStore.GetNode(ctx, "entity:REG_I2C_CTRL")
	if err != nil || nodeReg == nil {
		t.Fatalf("Expected entity REG_I2C_CTRL to be created, got error: %v", err)
	}

	nodeClk, err := graphStore.GetNode(ctx, "entity:CLK_EDGE_RISING")
	if err != nil || nodeClk == nil {
		t.Fatalf("Expected entity CLK_EDGE_RISING to be created, got error: %v", err)
	}

	// 验证共现边生成
	_, edges, err := graphStore.GetNeighbors(ctx, "entity:REG_I2C_CTRL", "both")
	if err != nil {
		t.Fatalf("GetNeighbors failed: %v", err)
	}
	if len(edges) == 0 {
		t.Fatalf("Expected cooccurrence edge between REG_I2C_CTRL and CLK_EDGE_RISING")
	}

	foundCooccur := false
	for _, e := range edges {
		if (e.SourceID == "entity:REG_I2C_CTRL" && e.TargetID == "entity:CLK_EDGE_RISING") ||
			(e.SourceID == "entity:CLK_EDGE_RISING" && e.TargetID == "entity:REG_I2C_CTRL") {
			foundCooccur = true
			if e.Relation != "CO_OCCURS_WITH" {
				t.Errorf("Expected relation CO_OCCURS_WITH, got %s", e.Relation)
			}
			if len(e.SourceChunkIDs) != 2 {
				t.Errorf("Expected edge to reference 2 chunks, got %d", len(e.SourceChunkIDs))
			}
		}
	}

	if !foundCooccur {
		t.Errorf("Did not find expected cooccurrence edge")
	}
}

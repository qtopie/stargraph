package indexing

import (
	"context"
	"fmt"
	"testing"

	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/storage/memory"
)

type mockLLMCounter struct {
	completeCalls int
}

func (m *mockLLMCounter) Complete(ctx context.Context, prompt string, opts ...llm.Option) (string, error) {
	m.completeCalls++
	return fmt.Sprintf(`("entity"%s"RISCV"%s"arch"%s"RISC-V architecture")%s`,
		TupleDelimiter, TupleDelimiter, TupleDelimiter,
		CompletionDelimiter,
	), nil
}

func (m *mockLLMCounter) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string, opts ...llm.Option) (string, error) {
	m.completeCalls++
	return "ok", nil
}

func TestLazyIndexing(t *testing.T) {
	ctx := context.Background()
	graphStore := memory.NewGraph()
	vectorStore := memory.NewVector()
	llmClient := &mockLLMCounter{}

	extractor := NewExtractor(llmClient, nil, graphStore, vectorStore, ExtractorConfig{
		MaxConcurrency: 1,
	})

	chunks := []*document.Chunk{
		{
			ID:      "chunk-1",
			Content: "RISCV is an open standard instruction set architecture (ISA).",
		},
	}

	if err := extractor.ExtractChunks(ctx, chunks); err != nil {
		t.Fatalf("ExtractChunks failed: %v", err)
	}

	// 验证实体已抽取
	node, err := graphStore.GetNode(ctx, "entity:RISCV")
	if err != nil || node == nil {
		t.Fatalf("Expected entity:RISCV to exist in graphStore")
	}

	// 在 Lazy 模式下，不运行 reportBuilder.BuildAllCommunityReports
	// 验证社区报告数保持为 0
	communities, err := graphStore.GetCommunitiesByLevel(ctx, 0)
	if err != nil {
		t.Fatalf("GetCommunitiesByLevel failed: %v", err)
	}
	if len(communities) != 0 {
		t.Errorf("Expected 0 community reports in lazy mode, got %d", len(communities))
	}
}

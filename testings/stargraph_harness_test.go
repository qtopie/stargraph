package testings

import (
	"context"
	"fmt"
	"strings"
	"testing"

	stargraph "github.com/qtopie/stargraph/pkg"
	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/indexing"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/search"
)

type harnessMockLLM struct{}

func (m *harnessMockLLM) Complete(ctx context.Context, prompt string, opts ...llm.Option) (string, error) {
	if strings.Contains(prompt, "-Goal-") {
		return fmt.Sprintf(`("entity"%s"Domour"%s"technology"%s"Domour is an AI assistant architecture.")%s("entity"%s"CosmosStar"%s"organization"%s"Parent organization.")%s("relationship"%s"Domour"%s"CosmosStar"%s"Domour is part of CosmosStar."%s10)%s`,
			indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
			indexing.RecordDelimiter,
			indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
			indexing.RecordDelimiter,
			indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
			indexing.CompletionDelimiter,
		), nil
	}
	if strings.Contains(prompt, "Community") || strings.Contains(prompt, "community") {
		return `{"title":"Domour Core Community","summary":"Knowledge cluster around Domour and CosmosStar.","rating":9.0}`, nil
	}
	if strings.Contains(prompt, "GlobalSearchMapPrompt") || strings.Contains(prompt, "points") {
		return `{"points":[{"description":"Domour acts as intelligent assistant.","score":9.5}]}`, nil
	}
	return "Domour is an advanced AI assistant platform.", nil
}

func (m *harnessMockLLM) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string, opts ...llm.Option) (string, error) {
	return "Domour provides local and global graph reasoning capabilities.", nil
}

type harnessMockEmbed struct{}

func (m *harnessMockEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.5, 0.5, 0.5}, nil
}

func (m *harnessMockEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = []float32{0.5, 0.5, 0.5}
	}
	return res, nil
}

func TestHarness_FullPipeline(t *testing.T) {
	ctx := context.Background()
	engine := stargraph.NewEngine(&harnessMockLLM{}, &harnessMockEmbed{}, stargraph.DefaultConfig())
	defer func() { _ = engine.Close() }()

	doc := &document.Document{
		ID:      "harness-doc-01",
		Content: "Domour is designed by CosmosStar to power intelligent assistants with graph-enhanced knowledge retrieval.",
	}

	if err := engine.Insert(ctx, doc); err != nil {
		t.Fatalf("Engine Insert failed: %v", err)
	}

	// Test Local Search
	res, err := engine.Query(ctx, &search.Request{
		Query:   "Tell me about Domour",
		Mode:    search.ModeLocal,
		TopK:    5,
		MaxHops: 2,
	})
	if err != nil {
		t.Fatalf("Local query failed: %v", err)
	}
	if res.Answer == "" || len(res.Nodes) == 0 {
		t.Errorf("Expected populated local search result, got empty or no nodes")
	}

	// Test Global Search
	resGlobal, err := engine.Query(ctx, &search.Request{
		Query: "Summarize all key themes",
		Mode:  search.ModeGlobal,
	})
	if err != nil {
		t.Fatalf("Global query failed: %v", err)
	}
	if resGlobal.Answer == "" {
		t.Errorf("Expected populated global search result, got empty")
	}
}

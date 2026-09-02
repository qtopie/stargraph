package benchmarkcases_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	stargraph "github.com/qtopie/stargraph/pkg"
	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/search"
)

type OrientExpressFixture struct {
	Title  string `json:"title"`
	Chunks []struct {
		ID      string `json:"id"`
		Part    string `json:"part"`
		Title   string `json:"title"`
		Content string `json:"content"`
	} `json:"chunks"`
}

type mockOrientLLM struct{}

func (m *mockOrientLLM) Complete(ctx context.Context, prompt string, opts ...llm.Option) (string, error) {
	if strings.Contains(prompt, "DRIFT Engine") {
		return "Mary Debenham is linked to Armstrong Family and was former governess, seeking justice against Cassetti.", nil
	}
	if strings.Contains(prompt, "GlobalSearchMapPrompt") || strings.Contains(prompt, "points") {
		return `{"points":[{"description":"All 12 passengers are relatives/servants of the Armstrong tragedy acting as a 12-person jury.","score":9.8}]}`, nil
	}
	if strings.Contains(prompt, "Community") {
		return `{"title":"Armstrong Vengeance Jury","summary":"Collective conspiracy of 12 passengers avenging Daisy Armstrong.","rating":9.5}`, nil
	}
	return "The murder was committed collectively by the 12 passengers.", nil
}

func (m *mockOrientLLM) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string, opts ...llm.Option) (string, error) {
	if strings.Contains(systemPrompt, "MARY_DEBENHAM") || strings.Contains(systemPrompt, "CASSETTI") {
		return "Mary Debenham has deep motive as former governess of Daisy Armstrong against murderer Cassetti.", nil
	}
	return "Analysis of Orient Express case.", nil
}

func TestOrientExpress_BenchmarkPipeline(t *testing.T) {
	ctx := context.Background()

	// 1. 读取原著 6-Chunk 事实语料 Fixture
	data, err := os.ReadFile("../../harness/fixtures/orient_express.json")
	if err != nil {
		t.Fatalf("Failed to read orient_express.json fixture: %v", err)
	}

	var fixture OrientExpressFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("Failed to parse fixture JSON: %v", err)
	}

	if len(fixture.Chunks) != 6 {
		t.Fatalf("Expected 6 narrative chunks, got %d", len(fixture.Chunks))
	}

	// 2. 转换并载入 StarGraph (采用 AGRAG 0-Token 极速建图 + DRIFT 检索)
	cfg := stargraph.DefaultConfig()
	cfg.IndexMode = stargraph.IndexModeLazy
	cfg.ExtractorType = stargraph.ExtractorTypeCooccurrence
	cfg.CooccurrenceConfig.MinCooccurFreq = 1

	mockLLM := llm.Client(&mockOrientLLM{})
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("COPILOT_PROVIDER_API_KEY")
	}
	baseURL := os.Getenv("DEEPSEEK_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("COPILOT_PROVIDER_BASE_URL")
	}
	baseURL = strings.TrimSuffix(baseURL, "/anthropic")
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	model := os.Getenv("DEEPSEEK_MODEL")
	if model == "" {
		model = os.Getenv("COPILOT_MODEL")
	}
	if model == "" || model == "deepseek-v4-flash" {
		model = "deepseek-chat"
	}

	if apiKey != "" {
		t.Logf("Using live DeepSeek LLM for Orient Express Benchmark (%s, %s)", baseURL, model)
		mockLLM = llm.NewOpenAIClient(baseURL, apiKey, model)
	}

	mockEmbed := &BenchmarkMockEmbed{}

	engine := stargraph.NewEngine(mockLLM, mockEmbed, cfg)
	defer func() { _ = engine.Close() }()

	docs := make([]*document.Document, 0, len(fixture.Chunks))
	for _, c := range fixture.Chunks {
		docs = append(docs, &document.Document{
			ID:      c.ID,
			Content: c.Content,
			Metadata: map[string]interface{}{
				"part":  c.Part,
				"title": c.Title,
			},
		})
	}

	if err := engine.Insert(ctx, docs...); err != nil {
		t.Fatalf("Engine Insert failed: %v", err)
	}

	// 3. 关卡 1: DRIFT 动态穿梭检索 (玛丽·德本汉隐藏动机)
	req1 := &search.Request{
		Query:   "Mary Debenham relationship with Samuel Ratchett",
		Mode:    search.ModeDRIFT,
		MaxHops: 3,
	}
	res1, err := engine.Query(ctx, req1)
	if err != nil {
		t.Fatalf("Query 1 failed: %v", err)
	}
	t.Logf("\n[东方快车关卡 1: DRIFT 动态穿梭推理回答]:\n%s\n", res1.Answer)

	// 4. 关卡 2: Local Search (拓扑闭环分析)
	req2 := &search.Request{
		Query:   "Are the alibis between passengers reliable or cyclical?",
		Mode:    search.ModeLocal,
		MaxHops: 2,
	}
	res2, err := engine.Query(ctx, req2)
	if err != nil {
		t.Fatalf("Query 2 failed: %v", err)
	}
	if res2.Answer == "" {
		t.Errorf("Expected non-empty answer for Case 2")
	}
	t.Logf("\n[东方快车关卡 2: 拓扑闭环分析回答]:\n%s\n", res2.Answer)

	// 5. 关卡 3: Global Search (十二人复仇陪审团宏观推演)
	req3 := &search.Request{
		Query: "Summarize global pattern and collective motive among all 12 passengers",
		Mode:  search.ModeGlobal,
	}
	res3, err := engine.Query(ctx, req3)
	if err != nil {
		t.Fatalf("Query 3 failed: %v", err)
	}
	if res3.Answer == "" {
		t.Errorf("Expected non-empty answer for Case 3")
	}
	t.Logf("\n[东方快车关卡 3: 全员宏观推演回答]:\n%s\n", res3.Answer)
}

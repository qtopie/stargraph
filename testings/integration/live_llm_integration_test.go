package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	stargraph "github.com/qtopie/stargraph/pkg"
	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/search"
)

// TestLiveLLM_RealWorldGraphRAG 在提供了真实 API Key 时执行真实端到端测试；
// 若无真实 API Key 则自动跳过 (Skip)，不阻塞 CI/本地快速单元测试。
func TestLiveLLM_RealWorldGraphRAG(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	chatModel := os.Getenv("OPENAI_CHAT_MODEL")
	embedModel := os.Getenv("OPENAI_EMBED_MODEL")

	if apiKey == "" {
		apiKey = os.Getenv("DEEPSEEK_API_KEY")
		if apiKey != "" && baseURL == "" {
			baseURL = "https://api.deepseek.com"
			chatModel = "deepseek-chat"
		}
	}

	if apiKey == "" && baseURL == "" {
		t.Skip("跳过真实 LLM 集成测试：未设置 OPENAI_API_KEY 或 DEEPSEEK_API_KEY")
	}

	if chatModel == "" {
		chatModel = "gpt-4o-mini"
	}
	if embedModel == "" {
		embedModel = "text-embedding-3-small"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	llmClient := llm.NewOpenAIClient(baseURL, apiKey, chatModel)
	embedClient := llm.NewOpenAIClient(baseURL, apiKey, embedModel)

	engine := stargraph.NewEngine(llmClient, embedClient, stargraph.DefaultConfig())
	defer func() { _ = engine.Close() }()

	doc1 := &document.Document{
		ID:      "live-doc-1",
		Content: "Alice is the project lead for Project Phoenix at CosmosStar.",
	}
	doc2 := &document.Document{
		ID:      "live-doc-2",
		Content: "Project Phoenix relies on Quantum DB for high-throughput temporal data.",
	}
	doc3 := &document.Document{
		ID:      "live-doc-3",
		Content: "Quantum DB is designed and maintained by Bob.",
	}

	t.Log("Connecting to real LLM to index 3 documents...")
	if err := engine.Insert(ctx, doc1, doc2, doc3); err != nil {
		t.Fatalf("Live Insert failed: %v", err)
	}

	query := "How is Alice connected to Bob?"
	t.Logf("Running real LLM Local Search for query: %s", query)

	res, err := engine.Query(ctx, &search.Request{
		Query:   query,
		Mode:    search.ModeLocal,
		TopK:    3,
		MaxHops: 2,
	})
	if err != nil {
		t.Fatalf("Live Query failed: %v", err)
	}

	t.Logf("Live LLM Answer:\n%s", res.Answer)
	if !strings.Contains(strings.ToLower(res.Answer), "quantum db") || !strings.Contains(strings.ToLower(res.Answer), "phoenix") {
		t.Errorf("Expected answer to mention bridge entities (Quantum DB, Phoenix)")
	}
}

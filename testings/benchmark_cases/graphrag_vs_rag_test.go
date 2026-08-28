package benchmarkcases_test

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

// BenchmarkMockLLM 模拟真实 LLM 在面对不同 Context 时的抽取与回答行为
type BenchmarkMockLLM struct{}

func (m *BenchmarkMockLLM) Complete(ctx context.Context, prompt string, opts ...llm.Option) (string, error) {
	// 1. 实体与关系抽取阶段 (Mock LLM 准确抽取文档中的三元组)
	if strings.Contains(prompt, "-Goal-") {
		if strings.Contains(prompt, "Alice") {
			return fmt.Sprintf(`("entity"%s"Alice"%s"person"%s"Alice is a core contributor of Project Phoenix.")%s("entity"%s"Project Phoenix"%s"project"%s"Project Phoenix is an open source initiative.")%s("relationship"%s"Alice"%s"Project Phoenix"%s"Alice leads the development of Project Phoenix."%s9)%s`,
				indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
				indexing.RecordDelimiter,
				indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
				indexing.RecordDelimiter,
				indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
				indexing.CompletionDelimiter,
			), nil
		}
		if strings.Contains(prompt, "Project Phoenix") && strings.Contains(prompt, "Quantum DB") {
			return fmt.Sprintf(`("entity"%s"Project Phoenix"%s"project"%s"Project Phoenix relies on Quantum DB.")%s("entity"%s"Quantum DB"%s"technology"%s"Quantum DB is a storage engine.")%s("relationship"%s"Project Phoenix"%s"Quantum DB"%s"Project Phoenix depends on Quantum DB."%s8)%s`,
				indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
				indexing.RecordDelimiter,
				indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
				indexing.RecordDelimiter,
				indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
				indexing.CompletionDelimiter,
			), nil
		}
		if strings.Contains(prompt, "Quantum DB") && strings.Contains(prompt, "Bob") {
			return fmt.Sprintf(`("entity"%s"Quantum DB"%s"technology"%s"Quantum DB is maintained by Bob.")%s("entity"%s"Bob"%s"person"%s"Bob is the lead architect of Quantum DB.")%s("relationship"%s"Bob"%s"Quantum DB"%s"Bob created and maintains Quantum DB."%s10)%s`,
				indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
				indexing.RecordDelimiter,
				indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
				indexing.RecordDelimiter,
				indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter, indexing.TupleDelimiter,
				indexing.CompletionDelimiter,
			), nil
		}
	}

	// 2. 社区报告生成阶段
	if strings.Contains(prompt, "Community") || strings.Contains(prompt, "community") {
		return `{"title":"Phoenix Data Infrastructure","summary":"Community covering Alice's project Phoenix, Quantum DB, and its maintainer Bob.","rating":9.0}`, nil
	}

	// 3. Global Search Map 阶段
	if strings.Contains(prompt, "GlobalSearchMapPrompt") || strings.Contains(prompt, "points") {
		return `{"points":[{"description":"Alice and Bob are connected via Project Phoenix's dependency on Quantum DB.","score":9.5}]}`, nil
	}

	return "Answer based on context", nil
}

func (m *BenchmarkMockLLM) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string, opts ...llm.Option) (string, error) {
	// 判断系统提示词给出的上下文是否包含两跳之外的 Bob 和 Quantum DB
	hasAlice := strings.Contains(systemPrompt, "Alice")
	hasPhoenix := strings.Contains(systemPrompt, "Project Phoenix")
	hasQuantumDB := strings.Contains(systemPrompt, "Quantum DB")
	hasBob := strings.Contains(systemPrompt, "Bob")

	if hasAlice && hasPhoenix && hasQuantumDB && hasBob {
		return "Alice indirectly collaborates with Bob because Alice leads Project Phoenix, which depends on Quantum DB maintained by Bob.", nil
	}

	// 如果传统 RAG 仅检索到单个文档（例如只检索到 Document 1），LLM 无法推理出与 Bob 的关系
	if hasAlice && !hasBob {
		return "Based on the provided context, Alice leads Project Phoenix. However, there is no mention of any indirect collaboration with Bob or backend database maintainers.", nil
	}

	return "Insufficient context to answer the multi-hop relationship.", nil
}

// BenchmarkMockEmbed 简易模拟向量：Alice 与 Bob 的文本余弦距离极远 (无直接词向量相似度)
type BenchmarkMockEmbed struct{}

func (m *BenchmarkMockEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "alice") {
		return []float32{1.0, 0.0, 0.0}, nil
	}
	if strings.Contains(lower, "bob") {
		return []float32{0.0, 1.0, 0.0}, nil // 与 Alice 正交 (相似度为 0)
	}
	return []float32{0.3, 0.3, 0.3}, nil
}

func (m *BenchmarkMockEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i, t := range texts {
		vec, _ := m.Embed(ctx, t)
		res[i] = vec
	}
	return res, nil
}

// TestGraphRAG_Solves_MultiHopReasoning_Failure_Of_Traditional_RAG
// 该测试用例明确论证：
// 1. 传统 RAG 在面对分散在多篇文档中的多跳关系（A -> B -> C）时，因向量语义相似度断裂而无法召回 C，导致问答失败。
// 2. GraphRAG 通过图拓扑遍历 (Local Search 2-Hop) 成功跨文档打通链路并完成精准推理。
func TestGraphRAG_Solves_MultiHopReasoning_Failure_Of_Traditional_RAG(t *testing.T) {
	ctx := context.Background()
	mockLLM := &BenchmarkMockLLM{}
	mockEmbed := &BenchmarkMockEmbed{}

	engine := stargraph.NewEngine(mockLLM, mockEmbed, stargraph.DefaultConfig())
	defer func() { _ = engine.Close() }()

	// 场景：3 篇分散在不同系统中的独立文档，彼此之间没有在同一段落出现 Alice 和 Bob
	doc1 := &document.Document{
		ID:      "doc-01-hr-note",
		Content: "Alice is the project lead for Project Phoenix, responsible for business logic and frontend releases.",
	}
	doc2 := &document.Document{
		ID:      "doc-02-architecture-spec",
		Content: "Project Phoenix relies on Quantum DB as its core high-concurrency storage engine.",
	}
	doc3 := &document.Document{
		ID:      "doc-03-infra-maintainer",
		Content: "Quantum DB was authored and is maintained exclusively by Bob from the Infrastructure team.",
	}

	// 索引所有文档进入 GraphRAG
	if err := engine.Insert(ctx, doc1, doc2, doc3); err != nil {
		t.Fatalf("Insert documents failed: %v", err)
	}

	// 问题：Alice 和 Bob 之间有什么隐式协作或间接依赖关系？
	query := "How does Alice indirectly collaborate with or depend on Bob?"

	// ==========================================
	// 1. 传统 RAG (Vector Naive Search) 的模拟表现
	// ==========================================
	// 传统 RAG 用 Query 搜索语义相似的 Chunk：
	// Query 中提到 "Alice"，向量检索只会高分召回包含 "Alice" 的 doc1。
	// 但 doc1 里面完全没有 "Quantum DB" 或 "Bob"，因此输入给 LLM 的 Context 只有 doc1。
	naiveRAGContext := fmt.Sprintf("Context: %s", doc1.Content)
	naiveRAGAnswer, _ := mockLLM.CompleteWithSystem(ctx, naiveRAGContext, query)

	t.Logf("\n[Traditional RAG Answer]:\n%s", naiveRAGAnswer)
	if strings.Contains(naiveRAGAnswer, "Bob") && !strings.Contains(naiveRAGAnswer, "no mention") {
		t.Fatalf("Traditional RAG was unexpectedly able to know Bob without multi-hop context")
	}

	// ==========================================
	// 2. StarGraph (GraphRAG Local Search) 的表现
	// ==========================================
	// StarGraph 先由 Alice 向量定位到 Alice 节点，然后通过 2 跳子图遍历：
	// Alice --[leads]--> Project Phoenix --[depends_on]--> Quantum DB <--[maintains]-- Bob
	// 自动将整条知识链路及关联文档拼入 Context！
	graphReq := &search.Request{
		Query:   query,
		Mode:    search.ModeLocal,
		TopK:    3,
		MaxHops: 2, // 2-hop 拓扑子图扩展
	}

	graphRes, err := engine.Query(ctx, graphReq)
	if err != nil {
		t.Fatalf("StarGraph Query failed: %v", err)
	}

	t.Logf("\n[StarGraph (GraphRAG) Answer]:\n%s", graphRes.Answer)
	t.Logf("Matched Subgraph Nodes: %d, Edges: %d", len(graphRes.Nodes), len(graphRes.Edges))

	// 断言：GraphRAG 成功跨越 3 篇文档发现了 Alice -> Project Phoenix -> Quantum DB -> Bob 的关联
	if !strings.Contains(graphRes.Answer, "Quantum DB") || !strings.Contains(graphRes.Answer, "Bob") {
		t.Errorf("Expected GraphRAG to synthesize multi-hop connection via Quantum DB and Bob, got: %s", graphRes.Answer)
	}
	if len(graphRes.Nodes) < 3 {
		t.Errorf("Expected at least 3 nodes in subgraph (Alice, Project Phoenix, Quantum DB, Bob), got: %d", len(graphRes.Nodes))
	}
}

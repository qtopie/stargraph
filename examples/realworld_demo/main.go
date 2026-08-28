package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	stargraph "github.com/qtopie/stargraph/pkg"
	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/graph"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/search"
)

func main() {
	// 从环境变量读取真实的 LLM API 配置 (支持 OpenAI, DeepSeek, Ollama, SiliconFlow 等)
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
		fmt.Println("=========================================================================")
		fmt.Println("⚠️  未检测到真实 OPENAI_API_KEY 或 DEEPSEEK_API_KEY 环境变量！")
		fmt.Println("请在终端运行如下环境变量后重试真实 LLM 交互:")
		fmt.Println("  export OPENAI_API_KEY='sk-...'")
		fmt.Println("  export OPENAI_BASE_URL='https://api.openai.com/v1' (或 https://api.deepseek.com)")
		fmt.Println("  export OPENAI_CHAT_MODEL='gpt-4o' (或 deepseek-chat)")
		fmt.Println("  export OPENAI_EMBED_MODEL='text-embedding-3-small'")
		fmt.Println("=========================================================================")
		return
	}

	if chatModel == "" {
		chatModel = "gpt-4o-mini"
	}
	if embedModel == "" {
		embedModel = "text-embedding-3-small"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fmt.Println("🚀 正在连接真实 LLM 与 Embedding 服务...")
	fmt.Printf("   BaseURL: %s\n", baseURL)
	fmt.Printf("   ChatModel: %s, EmbedModel: %s\n\n", chatModel, embedModel)

	llmClient := llm.NewOpenAIClient(baseURL, apiKey, chatModel)
	embedClient := llm.NewOpenAIClient(baseURL, apiKey, embedModel)

	engine := stargraph.NewEngine(llmClient, embedClient, stargraph.DefaultConfig())
	defer func() { _ = engine.Close() }()

	// 真实多文档场景 (分散在不同系统的 3 篇独立文档)
	docs := []*document.Document{
		{
			ID: "doc-01-personnel",
			Content: `Alice is the Director of Product Engineering at Cosmos Dynamics. 
She is leading the high-priority initiative known as Project Phoenix, which aims to build the next-generation autonomous flight navigation system.`,
		},
		{
			ID: "doc-02-system-architecture",
			Content: `Project Phoenix architecture specifications:
The autonomous navigation core does not use legacy relational storage. Instead, it strictly relies on Quantum DB, an in-memory distributed temporal graph engine, for millisecond-level path calculations.`,
		},
		{
			ID: "doc-03-infra-ownership",
			Content: `Infrastructure Component Registry:
Quantum DB was invented and is maintained by Bob, Principal Systems Architect in the Core Infrastructure Division. Any schema alterations or performance issues must be routed directly to Bob.`,
		},
	}

	fmt.Printf("📥 正在向 StarGraph 索引 %d 篇真实业务文档 (切分 -> LLM 并发抽取实体/关系 -> 聚类)...\n", len(docs))
	startIdx := time.Now()
	if err := engine.Insert(ctx, docs...); err != nil {
		log.Fatalf("❌ 真实 LLM 索引失败: %v", err)
	}
	fmt.Printf("✅ 索引完成，耗时: %v\n\n", time.Since(startIdx))

	query := "How does Alice indirectly collaborate with or depend on Bob, and what technical component links them together?"
	fmt.Printf("❓ 用户提问 (多跳跨文档推理):\n   \"%s\"\n\n", query)

	// 1. 执行 Local Search
	fmt.Println("🔍 正在执行 StarGraph Local Search (实体向量初筛 + 2跳子图拓扑遍历 + LLM 推理)...")
	startQuery := time.Now()
	res, err := engine.Query(ctx, &search.Request{
		Query:   query,
		Mode:    search.ModeLocal,
		TopK:    3,
		MaxHops: 2,
	})
	if err != nil {
		log.Fatalf("❌ 查询失败: %v", err)
	}

	fmt.Printf("⏱️ 查询耗时: %v\n\n", time.Since(startQuery))
	fmt.Println("================== [真实 LLM 生成的 GraphRAG 回答] ==================")
	fmt.Println(strings.TrimSpace(res.Answer))
	fmt.Println("=====================================================================")

	// 2. 打印抽取到的知识图谱拓扑
	fmt.Printf("\n📊 拓扑子图抽取结果统计 (Matched Subgraph Nodes: %d, Edges: %d):\n", len(res.Nodes), len(res.Edges))
	for _, n := range res.Nodes {
		fmt.Printf("   • [Node] %s (%s): %s\n", n.Name, n.Type, n.Description)
	}
	for _, e := range res.Edges {
		fmt.Printf("   • [Edge] %s --[%s]--> %s\n", e.SourceID, e.Relation, e.TargetID)
	}

	// 3. 导出标准前端 JSON 格式
	jsonViz, _ := graph.ToNodeLinkJSON(res.Nodes, res.Edges)
	fmt.Println("\n🌐 前端可视化标准 Node-Link JSON (可直接放入 Web 页面渲染):")
	fmt.Println(string(jsonViz))
}

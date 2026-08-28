<div align="center">

# 🌟 StarGraph (Domour Cortex)

**轻量级、高性能、纯原生 Go 实现的 GraphRAG 核心引擎**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Zero-CGO](https://img.shields.io/badge/CGO-已禁用%20(纯原生%20Go)-brightgreen)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Build & Test](https://img.shields.io/badge/测试门禁-通过-success)](./scripts/check.sh)

[English](./README.md) | [中文文档](./README-zh.md)

</div>

---

## 📖 项目简介

**StarGraph** 是专为个人/团队智能助理（如 **Domour Copilot**）与 **CosmosStar** 生态设计研发的轻量级、高性能、可嵌入式 **GraphRAG**（图增强检索生成）核心引擎。

不同于传统基于 Python 的重型 GraphRAG 实现，**StarGraph** 采用纯原生 Go 语言重构，**彻底摒弃 Python 运行时、C++ 动态链接库绑定（Zero-CGO）与重型外部图数据库**，具备极速启动、几十兆超低内存占用，以及全平台（x86_64、ARM64、RISC-V 边缘硬件）单二进制交叉编译部署的极致优势。

---

## ✨ 核心特性

- **⚡ 纯原生 Go & 零 CGO 依赖**：严格遵循 Go 1.22+ 规范，`CGO_ENABLED=0` 零环境依赖，开箱即用。
- **✂️ 递归语义分块 (Recursive Semantic Chunking)**：基于自然语义边界（段落、句子）递归分块，配合滑动窗口重叠（Overlap），从算法层面杜绝段落与语义腰斩。
- **🚀 并发三元组抽取 (Worker Pool)**：高效 Goroutine 协程池并发抽取实体与关系，自动聚合权重、跨 Chunk 关联并持久化溯源元数据。
- **🌐 内存拓扑聚类与层次社区报告**：内置连通子图与多层级社区划分算法（Level 0 / Level 1），自动化自底向上并发生成结构化社区摘要报告。
- **🔍 自适应双通道检索引擎**：
  - **Local Search（局部精准推理）**：实体向量初筛 $\rightarrow$ BFS 1~2 跳拓扑子图扩展 $\rightarrow$ 确定性跨文档多跳推理。
  - **Global Search（全局宏观总结）**：层次化社区报告 $\rightarrow$ Map-Reduce 分布式宏观全景总结。
  - **Auto Intent Router（意图路由器）**：根据用户查询特征自适应选择最佳检索模式。
- **📊 多模态前端可视化标准导出**：开箱支持一键导出为 **Node-Link JSON**（专供 D3.js / Cytoscape / ECharts / React Flow 渲染）、**Graphviz DOT** 和 **GraphML**。
- **🔌 统一 OpenAI 兼容客户端**：直接支持 OpenAI、DeepSeek、Ollama、vLLM、SiliconFlow 等主流大模型服务。

---

## 🏗️ 核心架构

```text
┌─────────────────────────────────────────────────────────────┐
│                    StarGraph (Pure Go)                      │
├─────────────────┬─────────────────────┬─────────────────────┤
│   LLM & Embed   │   存储抽象层 (Storage)│     图检索与推理    │
├─────────────────┼─────────────────────┼─────────────────────┤
│ • OpenAI 协议   │ 1. 纯原生内存存储   │ • BFS 多跳子图推理  │
│   (DeepSeek,    │    - Memory KV      │ • 分层社区拓扑聚类  │
│    Ollama,      │    - Pure Go Vector │ • Local Search      │
│    vLLM 等)     │    - 拓扑邻接表 Graph│ • Global Search     │
│                 │ 2. SurrealDB (规划) │ • Map-Reduce 报告   │
│                 │    - 文档+向量+RELATE│ • 自适应意图路由    │
└─────────────────┴─────────────────────┴─────────────────────┘
```

---

## 🚀 快速上手

### 1. 安装依赖

```bash
go get github.com/qtopie/stargraph
```

### 2. 基础使用示例

```go
package main

import (
	"context"
	"fmt"
	"log"

	stargraph "github.com/qtopie/stargraph/pkg"
	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/search"
)

func main() {
	ctx := context.Background()

	// 1. 初始化 LLM 与 Embedding 客户端 (兼容 OpenAI / DeepSeek / Ollama)
	llmClient := llm.NewOpenAIClient("https://api.openai.com/v1", "your-api-key", "gpt-4o")
	embedClient := llm.NewOpenAIClient("https://api.openai.com/v1", "your-api-key", "text-embedding-3-small")

	// 2. 初始化 StarGraph 引擎
	engine := stargraph.NewEngine(llmClient, embedClient, stargraph.DefaultConfig())
	defer engine.Close()

	// 3. 索引多篇业务文档 (切分 -> 并发三元组抽取 -> 图聚类 -> 社区报告生成)
	docs := []*document.Document{
		{
			ID:      "doc-1",
			Content: "Alice 是 CosmosStar 旗下 Project Phoenix 的负责人，负责智能助理核心业务架构。",
		},
		{
			ID:      "doc-2",
			Content: "Project Phoenix 核心依赖 Quantum DB 这一高并发时序图存储引擎。",
		},
		{
			ID:      "doc-3",
			Content: "Quantum DB 由基础架构团队的 Bob 独家研发与维护。",
		},
	}

	if err := engine.Insert(ctx, docs...); err != nil {
		log.Fatalf("索引文档失败: %v", err)
	}

	// 4. Local Search：跨文档多跳复杂推理
	localRes, err := engine.Query(ctx, &search.Request{
		Query:   "Alice 和 Bob 之间有什么间接依赖或协作关系？",
		Mode:    search.ModeLocal,
		TopK:    3,
		MaxHops: 2,
	})
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}

	fmt.Printf("Local Search 回答:\n%s\n", localRes.Answer)

	// 5. Global Search：Map-Reduce 全局宏观全景总结
	globalRes, err := engine.Query(ctx, &search.Request{
		Query: "请总结当前知识图谱中的核心组织结构与技术架构关系。",
		Mode:  search.ModeGlobal,
	})
	if err != nil {
		log.Fatalf("全局查询失败: %v", err)
	}

	fmt.Printf("\nGlobal Search 回答:\n%s\n", globalRes.Answer)
}
```

---

## 🎯 GraphRAG vs 传统向量 RAG 核心优势

为什么传统向量 RAG 在面对跨文档多跳推理时会失效，而 StarGraph 可以完美解决：

```text
用户提问: "Alice 和 Bob 之间有什么协作关系？"

[传统向量 RAG]:
❌ 失败：因 Doc 1（Alice）与 Doc 3（Bob）完全没有直接的关键词或语义重叠，向量检索无法关联召回。
   回答："根据上下文，Alice 负责 Phoenix。文中未提及 Bob。"

[StarGraph GraphRAG]:
✅ 拓扑连通：Alice --(leads)--> Phoenix --(depends_on)--> Quantum DB <--(maintains)-- Bob
   回答："Alice 间接依赖 Bob，因为 Alice 领导的 Project Phoenix 依赖由 Bob 维护的 Quantum DB。"
```

*(可直接在本地运行测试用例验证：[`testings/benchmark_cases/graphrag_vs_rag_test.go`](./testings/benchmark_cases/graphrag_vs_rag_test.go))。*

---

## 📊 前端图可视化导出

StarGraph 内置了多种标准格式导出能力，纯 Go 实现、零外部依赖：

```go
import "github.com/qtopie/stargraph/pkg/graph"

// 1. Web 前端标准 Node-Link JSON（供 D3.js, Cytoscape, ECharts, React Flow 等直接渲染）
jsonData, _ := graph.ToNodeLinkJSON(res.Nodes, res.Edges)

// 2. Graphviz DOT 格式（供命令行生成 SVG/PNG 架构图）
dotString := graph.ToDOT(res.Nodes, res.Edges, "MyGraph")

// 3. GraphML 格式（供 Gephi 桌面专业图论分析软件导入）
_ = graph.ToGraphML(fileWriter, res.Nodes, res.Edges)
```

---

## 🛠️ 质量与工程门禁

StarGraph 严格执行基于 Spec-Driven Development (SDD) 与 Harness Engineering 的质量门禁：

```bash
# 运行全套代码格式检查、静态 Lint、并发竞态检测及 Harness 评估断言
./scripts/check.sh
```

---

## 📄 开源许可证

本项目基于 [MIT 许可证](LICENSE) 开源。

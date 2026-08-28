<div align="center">

# 🌟 StarGraph (Domour Cortex)

**A Lightweight, High-Performance, Pure Go GraphRAG Core Engine**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Zero-CGO](https://img.shields.io/badge/CGO-Disabled%20(Pure%20Go)-brightgreen)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Build & Test](https://img.shields.io/badge/Tests-Passing-success)](./scripts/check.sh)

[English](./README.md) | [中文文档](./README-zh.md)

</div>

---

## 📖 Overview

**StarGraph** is a lightweight, embeddable, and high-concurrency **GraphRAG** (Graph Retrieval-Augmented Generation) core engine written in pure Go. It is designed specifically for personal and team intelligent assistants (such as the **Domour Copilot** and **CosmosStar** ecosystem).

Unlike traditional Python-based GraphRAG implementations, **StarGraph** eliminates all Python runtimes, C++ dynamic library bindings (Zero-CGO), and heavyweight external graph databases, enabling instant startup, ultra-low memory footprint, and single-binary deployment on any platform (x86_64, ARM64, and RISC-V).

---

## ✨ Key Features

- **⚡ Zero-CGO & Pure Go**: Built with pure Go 1.22+ standards. Zero external C/C++ or Python dependencies, compiling into a tiny, standalone binary.
- **✂️ Recursive Semantic Chunking**: Hierarchical document chunking based on natural semantic boundaries (paragraphs, sentences) with sliding window overlap, ensuring context integrity without semantic chopping.
- **🚀 Concurrent Triple Extraction (Worker Pool)**: Efficient Goroutine worker pool that extracts entities and relationships in parallel, automatically aggregating weights and deduplicating cross-chunk nodes.
- **🌐 In-Memory & Hierarchical Graph Clustering**: Built-in connected component and multi-level community detection (Level 0 / Level 1) with automated bottom-up LLM community report summarization.
- **🔍 Dual-Channel Adaptive Retrieval**:
  - **Local Search**: Entity vector retrieval $\rightarrow$ BFS 1~2 hop subgraph traversal $\rightarrow$ deterministic multi-hop reasoning.
  - **Global Search**: Hierarchical community reports $\rightarrow$ Map-Reduce macro panorama summarization.
  - **Auto Intent Router**: Dynamically routes queries between Local and Global search.
- **📊 Standard Visualization Exports**: Native export to **Node-Link JSON** (for D3.js / Cytoscape / ECharts / React Flow), **Graphviz DOT**, and **GraphML**.
- **🔌 Unified OpenAI-Compatible Client**: Seamlessly connects to OpenAI, DeepSeek, Ollama, vLLM, and SiliconFlow.

---

## 🏗️ Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                    StarGraph (Pure Go)                      │
├─────────────────┬─────────────────────┬─────────────────────┤
│   LLM & Embed   │   Storage Engines   │   Graph Reasoning   │
├─────────────────┼─────────────────────┼─────────────────────┤
│ • OpenAI API    │ 1. In-Memory (Built)│ • BFS Subgraph (1-2)│
│   (DeepSeek,    │    - Memory KV      │ • Hierarchical Comm │
│    Ollama,      │    - Pure Go Vector │ • Local Search      │
│    vLLM, etc.)  │    - Graph Adj-List │ • Global Search     │
│                 │ 2. SurrealDB (Plan) │ • Map-Reduce Report │
│                 │    - Doc+Vector+Rel │ • Auto Router       │
└─────────────────┴─────────────────────┴─────────────────────┘
```

---

## 🚀 Quick Start

### 1. Installation

```bash
go get github.com/qtopie/stargraph
```

### 2. Basic Example

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

	// 1. Initialize LLM & Embedding clients (OpenAI / DeepSeek / Ollama compatible)
	llmClient := llm.NewOpenAIClient("https://api.openai.com/v1", "your-api-key", "gpt-4o")
	embedClient := llm.NewOpenAIClient("https://api.openai.com/v1", "your-api-key", "text-embedding-3-small")

	// 2. Initialize StarGraph engine
	engine := stargraph.NewEngine(llmClient, embedClient, stargraph.DefaultConfig())
	defer engine.Close()

	// 3. Index documents (Chunking -> Concurrent Extraction -> Clustering -> Community Reports)
	docs := []*document.Document{
		{
			ID:      "doc-1",
			Content: "Alice leads Project Phoenix at CosmosStar. The initiative builds intelligent autonomous assistants.",
		},
		{
			ID:      "doc-2",
			Content: "Project Phoenix strictly relies on Quantum DB for high-throughput temporal knowledge storage.",
		},
		{
			ID:      "doc-3",
			Content: "Quantum DB was designed and is maintained exclusively by Bob from the Infrastructure team.",
		},
	}

	if err := engine.Insert(ctx, docs...); err != nil {
		log.Fatalf("Insert failed: %v", err)
	}

	// 4. Local Search: Multi-hop reasoning across disconnected documents
	localRes, err := engine.Query(ctx, &search.Request{
		Query:   "How does Alice depend on or collaborate with Bob?",
		Mode:    search.ModeLocal,
		TopK:    3,
		MaxHops: 2,
	})
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Printf("Local Search Answer:\n%s\n", localRes.Answer)

	// 5. Global Search: Map-Reduce macro panorama summarization
	globalRes, err := engine.Query(ctx, &search.Request{
		Query: "Summarize the overall organizational structure and technology stack.",
		Mode:  search.ModeGlobal,
	})
	if err != nil {
		log.Fatalf("Global query failed: %v", err)
	}

	fmt.Printf("\nGlobal Search Answer:\n%s\n", globalRes.Answer)
}
```

---

## 🎯 GraphRAG vs Traditional Vector RAG

Why traditional Vector RAG fails on multi-hop reasoning while StarGraph succeeds:

```text
Query: "How does Alice depend on Bob?"

[Traditional RAG]:
❌ Fails because Doc 1 (Alice) and Doc 3 (Bob) share no direct keyword/vector similarity.
   Output: "Based on context, Alice leads Phoenix. No mention of Bob."

[StarGraph GraphRAG]:
✅ Traverses topology: Alice --(leads)--> Phoenix --(depends_on)--> Quantum DB <--(maintains)-- Bob
   Output: "Alice depends on Bob because Alice leads Project Phoenix, which relies on Quantum DB maintained by Bob."
```

*(See [`testings/benchmark_cases/graphrag_vs_rag_test.go`](./testings/benchmark_cases/graphrag_vs_rag_test.go) for verifiable benchmarks).*

---

## 📊 Graph Visualization Export

Export the knowledge graph in multiple standard formats with zero external dependencies:

```go
import "github.com/qtopie/stargraph/pkg/graph"

// 1. Web-friendly Node-Link JSON (for D3.js, Cytoscape, ECharts, React Flow)
jsonData, _ := graph.ToNodeLinkJSON(res.Nodes, res.Edges)

// 2. Graphviz DOT format (for SVG architecture diagrams)
dotString := graph.ToDOT(res.Nodes, res.Edges, "MyGraph")

// 3. GraphML format (for Gephi desktop network analysis)
_ = graph.ToGraphML(fileWriter, res.Nodes, res.Edges)
```

---

## 🛠️ Testing & Quality Gates

StarGraph adheres strictly to Spec-Driven Development (SDD) and Harness Engineering:

```bash
# Run all format checks, linting, race condition tests, and harness assertions
./scripts/check.sh
```

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).

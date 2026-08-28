# Architecture Decision Records (ADR) - StarGraph (Domour Cortex)

本文档记录 StarGraph 项目的核心架构设计决策与组件选型权衡 (Trade-offs)。

---

## ADR-001: 纯 Go 原生实现与零 CGO 约束 (Zero-CGO Pure Go)

### 背景与痛点
- `nano-graphrag` 原版基于 Python 生态开发，依赖 Python 运行时、重型动态库绑定与复杂的虚拟环境部署。
- Go 生态中许多向量库（如 Faiss、HNSWLib）和分词库依赖 CGO 绑定，导致跨平台交叉编译极其困难（无法直接编译为 Linux ARM64 或 RISC-V 目标二进制），且存在 C++ 内存泄漏风险。

### 决策
1. **全面采用纯 Go (Go 1.22+) 原生实现**，严格遵守 `CGO_ENABLED=0` 编译规范。
2. **拒绝外部重型代理框架与 Python 依赖**，产物为单个高并发无依赖二进制文件。

### 影响与收益
- ✅ 完美支持 Linux/macOS/Windows 及边缘硬件（如 Milk-V RISC-V）一键跨平台交叉编译。
- ✅ 极速启动，内存占用仅为几十 MB 级别。

---

## ADR-002: 组件裁剪与精炼体系 (Component De-bloat Matrix)

### 背景
原版包含大量重型、重复且依赖外部 C++/Java 环境的组件（如 Neo4j, Milvus, Faiss, HNSWLib, Bedrock 独立适配等）。

### 决策与选型矩阵

| 组件类别 | 原版组件 | StarGraph 选型决策 | Rationale |
| :--- | :--- | :--- | :--- |
| **LLM & Embed** | OpenAI, Bedrock, DeepSeek, Ollama | **统一 OpenAI 协议 Client** (`pkg/llm`) | 现代大模型服务（DeepSeek, Ollama, vLLM, OpenRouter）均标准兼容 OpenAI API，一个优雅的 Client 即可通吃全部。 |
| **Vector DB** | Faiss, HNSWLib, Milvus, NanoVectorDB | **纯 Go 内存向量 + SurrealDB/Qdrant HTTP** | 彻底消除 CGO 依赖；小规模由内存余弦计算支撑，大规模/持久化由 SurrealDB 或纯 HTTP REST 向量引擎支撑。 |
| **Graph DB** | NetworkX, Neo4j | **内存邻接表拓扑 + SurrealDB Graph (`RELATE`)** | 剔除 Java 依赖的 Neo4j 与 Python NetworkX，内存图零依赖，SurrealDB 提供多模态图数据库持久化。 |
| **Chunking** | by token size, by text splitter | **原生带重叠滑动窗口分块器** (`pkg/indexing`) | 纯 Go 原生切分，无需引入臃肿的 C/Rust Tokenizer 动态库。 |
| **Visualization**| GraphML (XML) | **Node-Link JSON + 轻量 Export 转换工具** | 拥抱 Web 前端标准 JSON，支持 D3/Cytoscape.js/Domour UI 原生渲染，按需提供 DOT/GraphML 导出。 |

---

## ADR-003: 多模态图数据导出标准 (Graph Export Formats)

### 背景
传统 GraphML 为 XML 格式，对于现代 Web 前端与 AI 助理画布不够亲和，但部分用户仍需使用 Gephi 等桌面工具。

### 决策
StarGraph 提供轻量、零依赖的导出支持（`pkg/graph/export.go`）：
1. **Node-Link JSON**（`ToNodeLinkJSON`）：**默认标准**，输出 `{ nodes: [...], links: [...] }`，直接喂给 Web 前端（React Flow, ECharts, Cytoscape.js）。
2. **Graphviz DOT**（`ToDOT`）：纯文本拓扑语言，用于架构图与命令行一键生成 SVG。
3. **GraphML**（`ToGraphML`）：生成标准 XML，兼容 Gephi 桌面图论分析。

---

## ADR-004: 存储分层架构与持久化优先级 (Storage Tiering)

### 决策
1. **第一层级 (Default & Unit Tests)**: `pkg/storage/memory`（已实现），全并发安全内存 KV、Vector、Graph，随起随用。
2. **第二层级 (Production Persistence)**: `pkg/storage/surreal`（开发规划中），基于 SurrealQL 统一存储 Document、Vector 与 RELATE Graph 边。
3. **第三层级 (Optional Massive Vector Engine)**: `pkg/storage/qdrant`（可选插件），基于纯 Go HTTP REST 通信，满足亿级独立向量检索需求。

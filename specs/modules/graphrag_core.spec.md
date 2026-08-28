# Module Spec: StarGraph Core Engine (inspired by nano-graphrag)

## 1. Overview
StarGraph 是纯原生 Go 实现的轻量高性能 GraphRAG 核心引擎。本 Spec 规范定义以下核心功能：
1. **LLM 抽象与并发抽取 (Worker Pool)**: 统一 Chat/Complete 与 Embedding 接口，支持实体 (Entity) 与关系 (Edge/Relationship) 批量并发提取与合并。
2. **Text Chunking 分块器**: 支持基于 Token/字符与重叠窗口的文档分块。
3. **Graph Storage & In-Memory Clustering**: 提供并发安全的内存 KV、Vector、Graph 存储，并在拓扑图上实现层次化聚类算法。
4. **Community Report 生成流水线**: 为聚类社区自底向上总结生成层次化摘要。
5. **Local Search & Global Search 检索通道**:
   - Local Search: 实体向量定位 -> 1~2跳子图遍历 -> 关联文本块/关系/社区报告聚合 -> 确定性推理。
   - Global Search: 层次社区报告 -> Map-Reduce 并发宏观全景总结。
6. **StarGraph Engine Facade**: 统一提供 `Insert(docs)` 与 `Query(req)` 顶层入口。

## 2. Interface Contract

### 2.1 LLM & Embedding (`pkg/llm`)
- `LLMClient`: `Complete(ctx, prompt, opts) (string, error)`
- `EmbeddingClient`: `EmbedStrings(ctx, texts) ([][]float32, error)`
- `Extractor`: 并发抽取并解析实体与关系。

### 2.2 In-Memory Storage (`pkg/storage/memory`)
- `MemoryKV`: 并发安全 KV 存储。
- `MemoryVector`: 向量余弦相似度计算与 Top-K 检索。
- `MemoryGraph`: 节点/边邻接表、多跳子图遍历与度数统计。

### 2.3 Clustering (`pkg/clustering`)
- `Clusterer`: `Cluster(ctx, g storage.GraphStorage) (map[string]*graph.Community, error)`

### 2.4 Query Engine (`pkg/query`)
- `LocalSearch`: `Search(ctx, req *search.Request) (*search.Result, error)`
- `GlobalSearch`: `Search(ctx, req *search.Request) (*search.Result, error)`

## 3. Acceptance Criteria (BDD)

### Feature: Document Chunking
#### Scenario 1: [SPEC-CHUNK-001] Document token-based chunking with overlap
- **Given** A text document with 2000 words
- **When** Executing `ChunkDocument(doc, chunkSize=500, overlap=100)`
- **Then** Correct chunks are generated with proper `ChunkIndex` and valid metadata.
- **Mapped Test:** `pkg/indexing/splitter_test.go:TestChunkDocument`

### Feature: Entity & Relationship Extraction
#### Scenario 2: [SPEC-EXTRACT-001] Concurrent Entity/Relation Extraction
- **Given** Multiple text chunks and a mock LLM returning structured entities/triples
- **When** Calling `Extractor.ExtractChunks(ctx, chunks)`
- **Then** Nodes and Edges are merged, weights aggregated, and stored into GraphStorage and VectorStorage.
- **Mapped Test:** `pkg/indexing/extractor_test.go:TestExtractor_ExtractChunks`

### Feature: Local & Global Search Execution
#### Scenario 3: [SPEC-QUERY-001] Local Subgraph Search
- **Given** Stored entities, relations, chunks, and community reports in GraphStorage
- **When** Executing `LocalSearch.Search(ctx, req)`
- **Then** Relevant entity vector matches trigger 1-hop/2-hop subgraph context assembly and LLM response.
- **Mapped Test:** `pkg/query/local_search_test.go:TestLocalSearch`

#### Scenario 4: [SPEC-QUERY-002] Global Map-Reduce Community Search
- **Given** Hierarchical community reports at Level 0/1/2
- **When** Executing `GlobalSearch.Search(ctx, req)`
- **Then** Map-reduce prompts evaluate community reports and reduce into comprehensive global summary.
- **Mapped Test:** `pkg/query/global_search_test.go:TestGlobalSearch`

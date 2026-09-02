# Module Spec: Lazy Indexing & AGRAG Co-occurrence Extractor

## 1. Overview
本模块定义 StarGraph 的低成本快速索引能力，包括：
1. **AGRAG-style 0-LLM 共现抽取器 (`CooccurrenceExtractor`)**：基于 Term 提取与滑动窗口共现矩阵，自动构建实体节点与加权关系边，不消耗 LLM Token。
2. **Lazy Indexing 流水线**：跳过自底向上社区聚类与报告预生成，直接写入图存储与向量索引。

## 2. Interface Contract

### 2.1 Cooccurrence Extractor (`pkg/indexing`)
```go
type CooccurrenceConfig struct {
    WindowSize     int     // 滑动窗口大小 (默认 15 words/tokens)
    MinCooccurFreq int     // 最小共现频次 (默认 2)
    MinEdgeWeight  float64 // 最小边权重阈值
}

type CooccurrenceExtractor struct {
    config CooccurrenceConfig
}

func NewCooccurrenceExtractor(cfg CooccurrenceConfig) *CooccurrenceExtractor
func (e *CooccurrenceExtractor) ExtractChunks(ctx context.Context, chunks []*document.Chunk) ([]*graph.Node, []*graph.Edge, error)
```

### 2.2 IndexMode Configuration (`pkg/stargraph`)
```go
type IndexMode string

const (
    IndexModeEager IndexMode = "eager" // 抽取 -> 聚类 -> 全量生成社区报告 (传统 GraphRAG)
    IndexModeLazy  IndexMode = "lazy"  // 抽取 -> 索引，跳过预生成社区报告 (低成本 Lazy 模式)
)
```

## 3. Acceptance Criteria (BDD)

### Scenario 1: [SPEC-AGRAG-001] 0-LLM Co-occurrence Extraction
- **Given** A list of text chunks containing technical terms (e.g. `REG_I2C_CTRL`, `CLK_EDGE_RISING`)
- **When** Invoking `CooccurrenceExtractor.ExtractChunks(ctx, chunks)`
- **Then** Nodes for unique terms are created and edges connect terms co-occurring within the sliding window, without any LLM API calls.
- **Mapped Test:** `pkg/indexing/cooccurrence_test.go:TestCooccurrenceExtractor`

### Scenario 2: [SPEC-LAZY-001] Lazy Indexing Pipeline Bypass
- **Given** `StarGraph` instance configured with `IndexModeLazy`
- **When** Calling `engine.Insert(ctx, docs)`
- **Then** Chunks, Nodes, and Edges are indexed, but zero Community Reports are generated, completing significantly faster.
- **Mapped Test:** `pkg/indexing/lazy_test.go:TestLazyIndexing`

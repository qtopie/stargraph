# Module Spec: DRIFT Dynamic Traversal Search Engine

## 1. Overview
本模块定义 DRIFT (Dynamic Reasoning and Inference with Flexible Traversal) 检索引擎：
1. **Seed Entity Retrieval**：基于向量与文本快速锁定种子节点。
2. **Dynamic Path Traversal & Heuristic Pruning**：从种子节点出发进行多步启发式路径穿梭与剪枝，提取最相关链路。
3. **Focused Context Generation**：聚合高精度紧凑上下文并由 LLM 生成答案。

## 2. Interface Contract

### 2.1 DRIFT Engine (`pkg/query`)
```go
type DRIFTConfig struct {
    MaxDepth       int     // 最大穿梭探索深度 (默认 2~3)
    BeamWidth      int     // 启发式路径保留宽度 (默认 5)
    MinEdgeWeight  float64 // 边权重探索门限
    MaxContextSize int     // 最大组装上下文 Token 数
}

type DRIFTSearch struct {
    graphStorage  storage.GraphStorage
    vectorStorage storage.VectorStorage
    kvStorage     storage.KVStorage
    llmClient     llm.LLMClient
    embedClient   llm.EmbeddingClient
    config        DRIFTConfig
}

func NewDRIFTSearch(...) *DRIFTSearch
func (d *DRIFTSearch) Search(ctx context.Context, req *search.Request) (*search.Result, error)
```

## 3. Acceptance Criteria (BDD)

### Scenario 1: [SPEC-DRIFT-001] Seed Entity Localization & Multi-hop Traversal
- **Given** A graph with interconnected nodes and edges
- **When** Calling `DRIFTSearch.Search(ctx, req)` with a specific hardware diagnosis query
- **Then** Seeds are located via vector search, relevant paths up to `MaxDepth` are traversed with pruning, and context is assembled without scanning full community summaries.
- **Mapped Test:** `pkg/query/drift_search_test.go:TestDRIFTSearch`

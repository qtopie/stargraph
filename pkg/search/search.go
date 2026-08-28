package search

import (
	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/graph"
)

// Mode 检索模式定义
type Mode string

const (
	ModeLocal  Mode = "local"  // 子图遍历精准推理
	ModeGlobal Mode = "global" // 社区报告 Map-Reduce 宏观总结
	ModeHybrid Mode = "hybrid" // 稠密向量 + BM25 混合检索
	ModeAuto   Mode = "auto"   // 自适应 Intent Router 智能路由
)

// Request 统一查询请求参数
type Request struct {
	Query    string                 `json:"query"`
	Mode     Mode                   `json:"mode"`
	TopK     int                    `json:"top_k"`
	MaxHops  int                    `json:"max_hops"`  // Local Search 最大拓扑跳数 (默认 1-2)
	MaxLevel int                    `json:"max_level"` // Global Search 社区层级
	Filter   map[string]interface{} `json:"filter,omitempty"`
}

// Result 统一检索输出
type Result struct {
	Answer       string             `json:"answer"`
	SourceChunks []*document.Chunk  `json:"source_chunks,omitempty"`
	Nodes        []*graph.Node      `json:"nodes,omitempty"`
	Edges        []*graph.Edge      `json:"edges,omitempty"`
	Communities  []*graph.Community `json:"communities,omitempty"`
	Metadata     map[string]any     `json:"metadata,omitempty"`
}

package graph

import (
	"time"
)

// Node 代表图谱中的核心实体 (Entity)。
type Node struct {
	ID             string                 `json:"id"`                  // 全局唯一标识，如 "entity:elon_musk"
	Name           string                 `json:"name"`                // 实体规范名称
	Type           string                 `json:"type"`                // 实体类型 (如 Organization, Person, Concept)
	Description    string                 `json:"description"`         // 聚合语义描述
	SourceChunkIDs []string               `json:"source_chunks"`       // 来源 chunk 列表
	Embedding      []float32              `json:"embedding,omitempty"` // 实体语义向量 (用于 Local Search 向量相似度初筛)
	Weight         float64                `json:"weight"`              // 实体重要度/频次权重
	Properties     map[string]interface{} `json:"properties,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// Edge 代表实体之间的语义关系或交互连接 (Relationship/Triple)。
type Edge struct {
	ID             string                 `json:"id"`            // 全局关系标识
	SourceID       string                 `json:"source_id"`     // 源实体 ID
	TargetID       string                 `json:"target_id"`     // 目标实体 ID
	Relation       string                 `json:"relation"`      // 关系类型 (如 FOUNDED, BELONGS_TO)
	Description    string                 `json:"description"`   // 关系描述与上下文摘要
	Weight         float64                `json:"weight"`        // 关系强度
	SourceChunkIDs []string               `json:"source_chunks"` // 来源 chunk 列表
	Properties     map[string]interface{} `json:"properties,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// Community 代表图拓扑聚类生成的层次化社区结构。
type Community struct {
	ID         string                 `json:"id"`       // 社区全局 ID
	Level      int                    `json:"level"`    // 层次级别 (0 为最底层微观社区，数值越大越宏观)
	Title      string                 `json:"title"`    // 社区主题标题
	Summary    string                 `json:"summary"`  // 社区宏观总结报告 (Global Search 核心素材)
	Rating     float64                `json:"rating"`   // 社区重要度评分
	NodeIDs    []string               `json:"node_ids"` // 包含实体 ID 列表
	EdgeIDs    []string               `json:"edge_ids"` // 包含关系 ID 列表
	Embedding  []float32              `json:"embedding,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

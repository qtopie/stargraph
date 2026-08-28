package document

import (
	"time"
)

// Document 代表原始输入文档或外部上下文单元（如笔记、工单、对话历史）。
type Document struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// Chunk 代表切分后的语义文本块，是知识抽取的原子单元。
type Chunk struct {
	ID         string                 `json:"id"`
	DocID      string                 `json:"doc_id"`
	Content    string                 `json:"content"`
	Tokens     int                    `json:"tokens"`
	ChunkIndex int                    `json:"chunk_index"`
	Embedding  []float32              `json:"embedding,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

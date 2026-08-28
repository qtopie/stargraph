package storage

import (
	"context"

	"github.com/qtopie/stargraph/pkg/graph"
)

// KVStorage 定义键值/文档持久化接口（用于文档 Chunk、元数据与缓存）。
type KVStorage interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, val []byte) error
	Delete(ctx context.Context, key string) error
	BatchGet(ctx context.Context, keys []string) (map[string][]byte, error)
	BatchSet(ctx context.Context, kvMap map[string][]byte) error
	Close() error
}

// VectorSearchResult 向量检索单项结果
type VectorSearchResult struct {
	ID       string                 `json:"id"`
	Score    float32                `json:"score"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// VectorStorage 定义密集向量索引与最近邻检索接口。
type VectorStorage interface {
	UpsertVector(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error
	BatchUpsertVectors(ctx context.Context, ids []string, vectors [][]float32, metadatas []map[string]interface{}) error
	Search(ctx context.Context, queryVector []float32, topK int, filter map[string]interface{}) ([]*VectorSearchResult, error)
	DeleteVector(ctx context.Context, id string) error
	Close() error
}

// GraphStorage 定义拓扑图存储接口，支持节点/边的增删查及多跳子图遍历 (Local Search)。
type GraphStorage interface {
	// 实体与关系操作
	UpsertNode(ctx context.Context, node *graph.Node) error
	UpsertEdge(ctx context.Context, edge *graph.Edge) error
	BatchUpsertNodes(ctx context.Context, nodes []*graph.Node) error
	BatchUpsertEdges(ctx context.Context, edges []*graph.Edge) error

	GetNode(ctx context.Context, id string) (*graph.Node, error)
	GetEdge(ctx context.Context, id string) (*graph.Edge, error)
	GetNodesByIDs(ctx context.Context, ids []string) ([]*graph.Node, error)

	// 拓扑邻居与多跳子图检索
	GetNeighbors(ctx context.Context, nodeID string, direction string) ([]*graph.Node, []*graph.Edge, error)
	GetSubgraph(ctx context.Context, startNodeIDs []string, maxHops int) (nodes []*graph.Node, edges []*graph.Edge, err error)

	// 全图拓扑导出（用于分层聚类 Leiden 算法）
	GetAllNodes(ctx context.Context) ([]*graph.Node, error)
	GetAllEdges(ctx context.Context) ([]*graph.Edge, error)

	// 社区报告存储
	UpsertCommunity(ctx context.Context, comm *graph.Community) error
	GetCommunity(ctx context.Context, id string) (*graph.Community, error)
	GetCommunitiesByLevel(ctx context.Context, level int) ([]*graph.Community, error)

	Close() error
}

package stargraph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qtopie/stargraph/pkg/clustering"
	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/indexing"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/query"
	"github.com/qtopie/stargraph/pkg/search"
	"github.com/qtopie/stargraph/pkg/storage"
	"github.com/qtopie/stargraph/pkg/storage/memory"
)

// Config StarGraph 引擎整体配置
type Config struct {
	SplitterConfig  indexing.SplitterConfig
	ExtractorConfig indexing.ExtractorConfig
	ClusterConfig   clustering.Config
}

// DefaultConfig 默认引擎配置
func DefaultConfig() Config {
	return Config{
		SplitterConfig:  indexing.DefaultSplitterConfig(),
		ExtractorConfig: indexing.DefaultExtractorConfig(),
		ClusterConfig:   clustering.DefaultConfig(),
	}
}

// Engine StarGraph 核心引擎 Facade
type Engine struct {
	cfg           Config
	llmClient     llm.Client
	embedClient   llm.EmbeddingClient
	kvStore       storage.KVStorage
	vectorStore   storage.VectorStorage
	graphStore    storage.GraphStorage
	extractor     *indexing.Extractor
	reportBuilder *indexing.CommunityReportBuilder
	queryEngine   *query.Engine
}

// Option 配置选项
type Option func(*Engine)

// WithKVStorage 设置自定义 KV 存储
func WithKVStorage(kv storage.KVStorage) Option {
	return func(e *Engine) {
		e.kvStore = kv
	}
}

// WithVectorStorage 设置自定义向量存储
func WithVectorStorage(vec storage.VectorStorage) Option {
	return func(e *Engine) {
		e.vectorStore = vec
	}
}

// WithGraphStorage 设置自定义图存储
func WithGraphStorage(g storage.GraphStorage) Option {
	return func(e *Engine) {
		e.graphStore = g
	}
}

// NewEngine 创建 StarGraph 引擎实例
func NewEngine(llmClient llm.Client, embedClient llm.EmbeddingClient, cfg Config, opts ...Option) *Engine {
	e := &Engine{
		cfg:         cfg,
		llmClient:   llmClient,
		embedClient: embedClient,
		kvStore:     memory.NewKV(),
		vectorStore: memory.NewVector(),
		graphStore:  memory.NewGraph(),
	}

	for _, opt := range opts {
		opt(e)
	}

	e.extractor = indexing.NewExtractor(e.llmClient, e.embedClient, e.graphStore, e.vectorStore, e.cfg.ExtractorConfig)
	clusterer := clustering.NewClusterer(e.cfg.ClusterConfig)
	e.reportBuilder = indexing.NewCommunityReportBuilder(e.llmClient, e.graphStore, clusterer)

	localSearch := query.NewLocalSearcher(e.llmClient, e.embedClient, e.graphStore, e.vectorStore, e.kvStore)
	globalSearch := query.NewGlobalSearcher(e.llmClient, e.graphStore)
	e.queryEngine = query.NewEngine(localSearch, globalSearch)

	return e
}

// Insert 文档索引流水线：切分 Chunk -> 并发 LLM 抽取实体/关系 -> 图拓扑聚类 -> 生成社区报告
func (e *Engine) Insert(ctx context.Context, docs ...*document.Document) error {
	if len(docs) == 0 {
		return nil
	}

	// 1. 文档分块
	chunks := indexing.SplitDocuments(docs, e.cfg.SplitterConfig)
	if len(chunks) == 0 {
		return nil
	}

	// 2. 持久化 Chunks 至 KV 存储
	kvMap := make(map[string][]byte, len(chunks))
	for _, chunk := range chunks {
		data, err := json.Marshal(chunk)
		if err == nil {
			kvMap[chunk.ID] = data
		}
	}
	if err := e.kvStore.BatchSet(ctx, kvMap); err != nil {
		return fmt.Errorf("store chunks kv: %w", err)
	}

	// 3. 并发抽取实体与关系 (Worker Pool)
	if err := e.extractor.ExtractChunks(ctx, chunks); err != nil {
		return fmt.Errorf("extract chunks: %w", err)
	}

	// 4. 图拓扑聚类与社区摘要构建
	if err := e.reportBuilder.BuildAllCommunityReports(ctx); err != nil {
		return fmt.Errorf("build community reports: %w", err)
	}

	return nil
}

// Query 统一检索入口 (支持 Local, Global, Hybrid 与 Auto 模式)
func (e *Engine) Query(ctx context.Context, req *search.Request) (*search.Result, error) {
	return e.queryEngine.Query(ctx, req)
}

// Close 释放所有存储资源
func (e *Engine) Close() error {
	_ = e.kvStore.Close()
	_ = e.vectorStore.Close()
	_ = e.graphStore.Close()
	return nil
}

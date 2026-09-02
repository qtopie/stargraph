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

// IndexMode 索引流水线模式
type IndexMode string

const (
	IndexModeEager IndexMode = "eager" // 完整构建：抽取 -> 聚类 -> 自底向上预生成所有社区报告
	IndexModeLazy  IndexMode = "lazy"  // 惰性构建：抽取 -> 向量索引，跳过全量社区报告预生成 (极速低成本)
)

// ExtractorType 抽取器类型
type ExtractorType string

const (
	ExtractorTypeLLM          ExtractorType = "llm"          // 经典并发 LLM 语义抽取三元组
	ExtractorTypeCooccurrence ExtractorType = "cooccurrence" // 0-Token 纯 CPU 统计学共现抽取 (AGRAG 架构)
)

// Config StarGraph 引擎整体配置
type Config struct {
	IndexMode          IndexMode
	ExtractorType      ExtractorType
	SplitterConfig     indexing.SplitterConfig
	ExtractorConfig    indexing.ExtractorConfig
	CooccurrenceConfig indexing.CooccurrenceConfig
	ClusterConfig      clustering.Config
	DRIFTConfig        query.DRIFTConfig
}

// DefaultConfig 默认引擎配置
func DefaultConfig() Config {
	return Config{
		IndexMode:          IndexModeEager,
		ExtractorType:      ExtractorTypeLLM,
		SplitterConfig:     indexing.DefaultSplitterConfig(),
		ExtractorConfig:    indexing.DefaultExtractorConfig(),
		CooccurrenceConfig: indexing.DefaultCooccurrenceConfig(),
		ClusterConfig:      clustering.DefaultConfig(),
		DRIFTConfig:        query.DefaultDRIFTConfig(),
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
	extractor     indexing.ChunkExtractor
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

// WithIndexMode 设置索引模式 (Eager / Lazy)
func WithIndexMode(mode IndexMode) Option {
	return func(e *Engine) {
		e.cfg.IndexMode = mode
	}
}

// WithExtractorType 设置抽取器类型 (LLM / Cooccurrence)
func WithExtractorType(extType ExtractorType) Option {
	return func(e *Engine) {
		e.cfg.ExtractorType = extType
	}
}

// NewEngine 创建 StarGraph 引擎实例
func NewEngine(llmClient llm.Client, embedClient llm.EmbeddingClient, cfg Config, opts ...Option) *Engine {
	if cfg.IndexMode == "" {
		cfg.IndexMode = IndexModeEager
	}
	if cfg.ExtractorType == "" {
		cfg.ExtractorType = ExtractorTypeLLM
	}

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

	// 依据 ExtractorType 实例化相应抽取器
	if e.cfg.ExtractorType == ExtractorTypeCooccurrence {
		e.extractor = indexing.NewCooccurrenceExtractor(e.graphStore, e.vectorStore, e.embedClient, e.cfg.CooccurrenceConfig)
	} else {
		e.extractor = indexing.NewExtractor(e.llmClient, e.embedClient, e.graphStore, e.vectorStore, e.cfg.ExtractorConfig)
	}

	clusterer := clustering.NewClusterer(e.cfg.ClusterConfig)
	e.reportBuilder = indexing.NewCommunityReportBuilder(e.llmClient, e.graphStore, clusterer)

	localSearch := query.NewLocalSearcher(e.llmClient, e.embedClient, e.graphStore, e.vectorStore, e.kvStore)
	globalSearch := query.NewGlobalSearcher(e.llmClient, e.graphStore)
	driftSearch := query.NewDRIFTSearcher(e.llmClient, e.embedClient, e.graphStore, e.vectorStore, e.kvStore, e.cfg.DRIFTConfig)
	e.queryEngine = query.NewEngine(localSearch, globalSearch, driftSearch)

	return e
}

// Insert 文档索引流水线：切分 Chunk -> 抽取实体/关系 -> (可选) 图聚类与生成社区报告
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

	// 3. 抽取实体与关系 (Worker Pool 或 0-Token 共现抽取)
	if err := e.extractor.ExtractChunks(ctx, chunks); err != nil {
		return fmt.Errorf("extract chunks: %w", err)
	}

	// 4. 如果是 Eager 模式，才执行昂贵的全量社区聚类与报告预生成；Lazy 模式直接跳过
	if e.cfg.IndexMode == IndexModeEager {
		if err := e.reportBuilder.BuildAllCommunityReports(ctx); err != nil {
			return fmt.Errorf("build community reports: %w", err)
		}
	}

	return nil
}

// Query 统一检索入口 (支持 Local, Global, DRIFT, Hybrid 与 Auto 模式)
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

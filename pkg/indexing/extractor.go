package indexing

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/graph"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/storage"
)

// ExtractorConfig 实体与关系并发抽取配置
type ExtractorConfig struct {
	MaxConcurrency int    // 并发协程数 (Worker Pool)
	EntityTypes    string // 识别实体类型，逗号分隔
}

// DefaultExtractorConfig 默认抽取配置
func DefaultExtractorConfig() ExtractorConfig {
	return ExtractorConfig{
		MaxConcurrency: 4,
		EntityTypes:    DefaultEntityTypes,
	}
}

// ChunkExtractor 实体与关系抽取接口契约 (支持 LLM 语义抽取与 0-Token 统计学共现抽取)
type ChunkExtractor interface {
	ExtractChunks(ctx context.Context, chunks []*document.Chunk) error
}

// Extractor 负责并发调用 LLM 从文档分块中提取三元组并写入存储
type Extractor struct {
	llmClient   llm.Client
	embedClient llm.EmbeddingClient
	graphStore  storage.GraphStorage
	vectorStore storage.VectorStorage
	cfg         ExtractorConfig
}

// NewExtractor 创建抽取器实例
func NewExtractor(
	llmClient llm.Client,
	embedClient llm.EmbeddingClient,
	graphStore storage.GraphStorage,
	vectorStore storage.VectorStorage,
	cfg ExtractorConfig,
) *Extractor {
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 4
	}
	if cfg.EntityTypes == "" {
		cfg.EntityTypes = DefaultEntityTypes
	}
	return &Extractor{
		llmClient:   llmClient,
		embedClient: embedClient,
		graphStore:  graphStore,
		vectorStore: vectorStore,
		cfg:         cfg,
	}
}

// RawRecord 抽取出的原始节点或边
type rawNode struct {
	name        string
	entityType  string
	description string
	chunkID     string
}

type rawEdge struct {
	source      string
	target      string
	description string
	weight      float64
	chunkID     string
}

var recordRegex = regexp.MustCompile(`\((.*?)\)`)

// ExtractChunks 并发处理 Chunks，提取实体/关系并完成合并与入库
func (e *Extractor) ExtractChunks(ctx context.Context, chunks []*document.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	semaphore := make(chan struct{}, e.cfg.MaxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	allRawNodes := make(map[string][]*rawNode) // normalizedName -> rawNodes
	allRawEdges := make(map[string][]*rawEdge) // edgeKey (src:tgt) -> rawEdges
	var firstErr error

	for _, chunk := range chunks {
		wg.Add(1)
		go func(c *document.Chunk) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()

			nodes, edges, err := e.extractSingleChunk(ctx, c)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			for _, n := range nodes {
				norm := strings.ToUpper(strings.TrimSpace(n.name))
				allRawNodes[norm] = append(allRawNodes[norm], n)
			}
			for _, ed := range edges {
				src := strings.ToUpper(strings.TrimSpace(ed.source))
				tgt := strings.ToUpper(strings.TrimSpace(ed.target))
				var key string
				if src < tgt {
					key = src + "<->" + tgt
				} else {
					key = tgt + "<->" + src
				}
				allRawEdges[key] = append(allRawEdges[key], ed)
			}
			mu.Unlock()
		}(chunk)
	}

	wg.Wait()
	if firstErr != nil {
		return fmt.Errorf("extraction error: %w", firstErr)
	}

	// 合并并持久化 Nodes
	mergedNodes := make([]*graph.Node, 0, len(allRawNodes))
	for normName, rNodes := range allRawNodes {
		if len(rNodes) == 0 {
			continue
		}
		var descBuilder strings.Builder
		chunkSet := make(map[string]struct{})
		entityType := rNodes[0].entityType

		for i, rn := range rNodes {
			if i > 0 {
				descBuilder.WriteString(" | ")
			}
			descBuilder.WriteString(rn.description)
			chunkSet[rn.chunkID] = struct{}{}
			if rn.entityType != "" {
				entityType = rn.entityType
			}
		}

		chunkIDs := make([]string, 0, len(chunkSet))
		for cid := range chunkSet {
			chunkIDs = append(chunkIDs, cid)
		}

		node := &graph.Node{
			ID:             "entity:" + normName,
			Name:           rNodes[0].name,
			Type:           entityType,
			Description:    descBuilder.String(),
			SourceChunkIDs: chunkIDs,
			Weight:         float64(len(rNodes)),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		// 生成 Entity 向量用于 Local Search
		if e.embedClient != nil {
			vec, err := e.embedClient.Embed(ctx, node.Name+": "+node.Description)
			if err == nil {
				node.Embedding = vec
				if e.vectorStore != nil {
					_ = e.vectorStore.UpsertVector(ctx, node.ID, vec, map[string]interface{}{
						"type":        "entity",
						"entity_name": node.Name,
					})
				}
			}
		}

		mergedNodes = append(mergedNodes, node)
	}

	if err := e.graphStore.BatchUpsertNodes(ctx, mergedNodes); err != nil {
		return fmt.Errorf("batch upsert nodes: %w", err)
	}

	// 合并并持久化 Edges
	mergedEdges := make([]*graph.Edge, 0, len(allRawEdges))
	for key, rEdges := range allRawEdges {
		if len(rEdges) == 0 {
			continue
		}
		var descBuilder strings.Builder
		chunkSet := make(map[string]struct{})
		var totalWeight float64

		for i, re := range rEdges {
			if i > 0 {
				descBuilder.WriteString(" | ")
			}
			descBuilder.WriteString(re.description)
			chunkSet[re.chunkID] = struct{}{}
			totalWeight += re.weight
		}

		chunkIDs := make([]string, 0, len(chunkSet))
		for cid := range chunkSet {
			chunkIDs = append(chunkIDs, cid)
		}

		srcNorm := "entity:" + strings.ToUpper(strings.TrimSpace(rEdges[0].source))
		tgtNorm := "entity:" + strings.ToUpper(strings.TrimSpace(rEdges[0].target))

		edge := &graph.Edge{
			ID:             "rel:" + key,
			SourceID:       srcNorm,
			TargetID:       tgtNorm,
			Relation:       "RELATED_TO",
			Description:    descBuilder.String(),
			Weight:         totalWeight / float64(len(rEdges)),
			SourceChunkIDs: chunkIDs,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		mergedEdges = append(mergedEdges, edge)
	}

	if err := e.graphStore.BatchUpsertEdges(ctx, mergedEdges); err != nil {
		return fmt.Errorf("batch upsert edges: %w", err)
	}

	return nil
}

func (e *Extractor) extractSingleChunk(ctx context.Context, chunk *document.Chunk) ([]*rawNode, []*rawEdge, error) {
	prompt := fmt.Sprintf(
		EntityExtractionPrompt,
		e.cfg.EntityTypes,
		TupleDelimiter, TupleDelimiter, TupleDelimiter,
		TupleDelimiter, TupleDelimiter, TupleDelimiter, TupleDelimiter,
		RecordDelimiter,
		CompletionDelimiter,
		chunk.Content,
	)

	resp, err := e.llmClient.Complete(ctx, prompt, llm.WithTemperature(0.1))
	if err != nil {
		return nil, nil, err
	}

	// 解析抽取结果
	nodes := make([]*rawNode, 0)
	edges := make([]*rawEdge, 0)

	matches := recordRegex.FindAllStringSubmatch(resp, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		content := match[1]
		parts := strings.Split(content, TupleDelimiter)
		if len(parts) < 2 {
			continue
		}

		recType := strings.Trim(strings.TrimSpace(parts[0]), `"'`)

		if strings.EqualFold(recType, "entity") && len(parts) >= 4 {
			name := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			eType := strings.Trim(strings.TrimSpace(parts[2]), `"'`)
			desc := strings.Trim(strings.TrimSpace(parts[3]), `"'`)
			if name != "" {
				nodes = append(nodes, &rawNode{
					name:        name,
					entityType:  eType,
					description: desc,
					chunkID:     chunk.ID,
				})
			}
		} else if strings.EqualFold(recType, "relationship") && len(parts) >= 5 {
			src := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			tgt := strings.Trim(strings.TrimSpace(parts[2]), `"'`)
			desc := strings.Trim(strings.TrimSpace(parts[3]), `"'`)
			wStr := strings.Trim(strings.TrimSpace(parts[4]), `"'`)
			w, _ := strconv.ParseFloat(wStr, 64)
			if w <= 0 {
				w = 1.0
			}
			if src != "" && tgt != "" {
				edges = append(edges, &rawEdge{
					source:      src,
					target:      tgt,
					description: desc,
					weight:      w,
					chunkID:     chunk.ID,
				})
			}
		}
	}

	return nodes, edges, nil
}

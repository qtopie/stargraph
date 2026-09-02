package indexing

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/graph"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/storage"
)

// CooccurrenceConfig 0-LLM 统计学共现抽取器配置 (AGRAG 范式)
type CooccurrenceConfig struct {
	WindowSize     int      // 滑动窗口大小 (默认 15 tokens/words)
	MinCooccurFreq int      // 最小共现频次 (默认 2)
	MinEdgeWeight  float64  // 最小边权重门限
	CustomKeywords []string // 可选：领域预设关键词白名单
}

// DefaultCooccurrenceConfig 默认共现抽取配置
func DefaultCooccurrenceConfig() CooccurrenceConfig {
	return CooccurrenceConfig{
		WindowSize:     15,
		MinCooccurFreq: 2,
		MinEdgeWeight:  0.1,
	}
}

// DefaultStopWords 通用中英文停用词表
var DefaultStopWords = map[string]struct{}{
	"the": {}, "is": {}, "at": {}, "which": {}, "on": {}, "a": {}, "an": {}, "and": {}, "or": {},
	"in": {}, "for": {}, "to": {}, "of": {}, "with": {}, "by": {}, "from": {}, "as": {}, "that": {},
	"this": {}, "these": {}, "those": {}, "it": {}, "its": {}, "be": {}, "are": {}, "was": {}, "were": {},
	"will": {}, "can": {}, "may": {}, "should": {}, "have": {}, "has": {}, "had": {}, "do": {}, "does": {},
	"did": {}, "not": {}, "but": {}, "if": {}, "then": {}, "else": {}, "when": {}, "where": {}, "why": {},
	"how": {}, "all": {}, "any": {}, "both": {}, "each": {}, "few": {}, "more": {}, "most": {}, "other": {},
	"some": {}, "such": {}, "no": {}, "nor": {}, "too": {}, "very": {}, "s": {}, "t": {}, "can't": {},
	"的": {}, "了": {}, "在": {}, "是": {}, "我": {}, "有": {}, "和": {}, "就": {}, "不": {}, "人": {},
	"都": {}, "一": {}, "一个": {}, "上": {}, "也": {}, "很": {}, "到": {}, "说": {}, "要": {}, "去": {},
	"你": {}, "会": {}, "着": {}, "没有": {}, "看": {}, "好": {}, "自己": {}, "这": {}, "它": {},
}

var termSplitterRegex = regexp.MustCompile(`[\s,;:!?"'()\[\]{}<>/\\|+=*&^%$#@~` + "`" + `]+`)

// CooccurrenceExtractor 纯统计学共现图抽取器 (无需调用 LLM，零 Token 消耗)
type CooccurrenceExtractor struct {
	graphStore  storage.GraphStorage
	vectorStore storage.VectorStorage
	embedClient llm.EmbeddingClient
	cfg         CooccurrenceConfig
	stopWords   map[string]struct{}
	mu          sync.Mutex
}

// NewCooccurrenceExtractor 创建 0-Token 统计学共现图抽取器
func NewCooccurrenceExtractor(
	graphStore storage.GraphStorage,
	vectorStore storage.VectorStorage,
	embedClient llm.EmbeddingClient,
	cfg CooccurrenceConfig,
) *CooccurrenceExtractor {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 15
	}
	if cfg.MinCooccurFreq <= 0 {
		cfg.MinCooccurFreq = 2
	}
	return &CooccurrenceExtractor{
		graphStore:  graphStore,
		vectorStore: vectorStore,
		embedClient: embedClient,
		cfg:         cfg,
		stopWords:   DefaultStopWords,
	}
}

// ExtractChunks 从文本块中通过统计学与滑动窗口提取实体与关系边并入库
func (e *CooccurrenceExtractor) ExtractChunks(ctx context.Context, chunks []*document.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 统计词频与共现频次
	termFreq := make(map[string]int)                                 // term -> freq
	termDocCount := make(map[string]int)                             // term -> doc/chunk count
	termChunks := make(map[string]map[string]struct{})               // term -> chunkIDs
	termContexts := make(map[string][]string)                        // term -> sample context snippets
	cooccurFreq := make(map[string]map[string]int)                   // termA -> termB -> freq
	cooccurChunks := make(map[string]map[string]map[string]struct{}) // termA -> termB -> chunkIDs

	totalChunks := len(chunks)

	for _, chunk := range chunks {
		tokens := e.tokenize(chunk.Content)
		if len(tokens) == 0 {
			continue
		}

		chunkTerms := make(map[string]struct{})
		for i, tok := range tokens {
			norm := strings.ToUpper(tok)
			termFreq[norm]++
			chunkTerms[norm] = struct{}{}

			if _, exists := termChunks[norm]; !exists {
				termChunks[norm] = make(map[string]struct{})
			}
			termChunks[norm][chunk.ID] = struct{}{}

			// 保存上下文片段作为实体描述
			if len(termContexts[norm]) < 3 {
				start := i - 5
				if start < 0 {
					start = 0
				}
				end := i + 6
				if end > len(tokens) {
					end = len(tokens)
				}
				snippet := strings.Join(tokens[start:end], " ")
				termContexts[norm] = append(termContexts[norm], snippet)
			}

			// 滑动窗口共现统计
			wEnd := i + e.cfg.WindowSize
			if wEnd > len(tokens) {
				wEnd = len(tokens)
			}

			for j := i + 1; j < wEnd; j++ {
				other := strings.ToUpper(tokens[j])
				if norm == other {
					continue
				}

				// 维护无向序
				u, v := norm, other
				if u > v {
					u, v = v, u
				}

				if _, exists := cooccurFreq[u]; !exists {
					cooccurFreq[u] = make(map[string]int)
					cooccurChunks[u] = make(map[string]map[string]struct{})
				}
				cooccurFreq[u][v]++

				if _, exists := cooccurChunks[u][v]; !exists {
					cooccurChunks[u][v] = make(map[string]struct{})
				}
				cooccurChunks[u][v][chunk.ID] = struct{}{}
			}
		}

		for t := range chunkTerms {
			termDocCount[t]++
		}
	}

	// 1. 构建与持久化实体节点 (Nodes)
	nodes := make([]*graph.Node, 0, len(termFreq))
	for term, freq := range termFreq {
		// 过滤极低频无效词 (至少出现 1 次)
		if freq < 1 {
			continue
		}

		// 计算 TF-IDF 基础重要度
		df := termDocCount[term]
		idf := math.Log(float64(totalChunks+1)/float64(df+1)) + 1.0
		weight := float64(freq) * idf

		chunkIDs := make([]string, 0, len(termChunks[term]))
		for cid := range termChunks[term] {
			chunkIDs = append(chunkIDs, cid)
		}

		desc := fmt.Sprintf("Technical term '%s' mentioned in %d chunks. Contexts: %s",
			term, len(chunkIDs), strings.Join(termContexts[term], " | "))

		node := &graph.Node{
			ID:             "entity:" + term,
			Name:           term,
			Type:           "TECHNICAL_TERM",
			Description:    desc,
			SourceChunkIDs: chunkIDs,
			Weight:         weight,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		// 如果提供了 EmbeddingClient，生成向量用于索引
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

		nodes = append(nodes, node)
	}

	if err := e.graphStore.BatchUpsertNodes(ctx, nodes); err != nil {
		return fmt.Errorf("cooccurrence batch upsert nodes: %w", err)
	}

	// 2. 构建与持久化关系边 (Edges)
	edges := make([]*graph.Edge, 0)
	for u, neighbors := range cooccurFreq {
		for v, freq := range neighbors {
			if freq < e.cfg.MinCooccurFreq {
				continue
			}

			// 基于 Jaccard / 共现强度的边权重计算
			freqU := termFreq[u]
			freqV := termFreq[v]
			jaccardWeight := float64(freq) / float64(freqU+freqV-freq)
			if jaccardWeight < e.cfg.MinEdgeWeight && freq < 3 {
				continue
			}

			cSet := cooccurChunks[u][v]
			chunkIDs := make([]string, 0, len(cSet))
			for cid := range cSet {
				chunkIDs = append(chunkIDs, cid)
			}

			edgeID := fmt.Sprintf("rel:%s<->%s", u, v)
			desc := fmt.Sprintf("Statistical co-occurrence between '%s' and '%s' (freq=%d, weight=%.3f)", u, v, freq, jaccardWeight)

			edge := &graph.Edge{
				ID:             edgeID,
				SourceID:       "entity:" + u,
				TargetID:       "entity:" + v,
				Relation:       "CO_OCCURS_WITH",
				Description:    desc,
				Weight:         jaccardWeight,
				SourceChunkIDs: chunkIDs,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}

			edges = append(edges, edge)
		}
	}

	if err := e.graphStore.BatchUpsertEdges(ctx, edges); err != nil {
		return fmt.Errorf("cooccurrence batch upsert edges: %w", err)
	}

	return nil
}

// tokenize 过滤停用词并提取有效专有名词与标识符
func (e *CooccurrenceExtractor) tokenize(text string) []string {
	rawTokens := termSplitterRegex.Split(text, -1)
	filtered := make([]string, 0, len(rawTokens))

	for _, token := range rawTokens {
		tok := strings.TrimSpace(token)
		if len(tok) <= 1 {
			continue
		}

		lower := strings.ToLower(tok)
		if _, isStop := e.stopWords[lower]; isStop {
			continue
		}

		// 保留纯字母、下划线、数字混合的专业符号（如 REG_I2C_CTRL, CLK_PIN1, STM32F4）
		hasValidChar := false
		for _, r := range tok {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				hasValidChar = true
				break
			}
		}

		if hasValidChar {
			filtered = append(filtered, tok)
		}
	}

	return filtered
}

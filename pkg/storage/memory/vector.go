package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/qtopie/stargraph/pkg/storage"
)

type vectorEntry struct {
	id       string
	vector   []float32
	norm     float32
	metadata map[string]interface{}
}

// Vector 纯原生内存密集向量存储 (余弦相似度 + Top-K 检索)
type Vector struct {
	mu      sync.RWMutex
	entries map[string]*vectorEntry
}

// NewVector 创建内存向量存储实例
func NewVector() *Vector {
	return &Vector{
		entries: make(map[string]*vectorEntry),
	}
}

func calcNorm(v []float32) float32 {
	var sum float64
	for _, val := range v {
		sum += float64(val * val)
	}
	return float32(math.Sqrt(sum))
}

func cosineSimilarity(v1, v2 []float32, norm1, norm2 float32) float32 {
	if norm1 == 0 || norm2 == 0 || len(v1) != len(v2) {
		return 0
	}
	var dot float64
	for i := 0; i < len(v1); i++ {
		dot += float64(v1[i] * v2[i])
	}
	return float32(dot / (float64(norm1) * float64(norm2)))
}

func (m *Vector) UpsertVector(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vCopy := make([]float32, len(vector))
	copy(vCopy, vector)

	m.entries[id] = &vectorEntry{
		id:       id,
		vector:   vCopy,
		norm:     calcNorm(vCopy),
		metadata: metadata,
	}
	return nil
}

func (m *Vector) BatchUpsertVectors(ctx context.Context, ids []string, vectors [][]float32, metadatas []map[string]interface{}) error {
	if len(ids) != len(vectors) {
		return fmt.Errorf("ids length %d does not match vectors length %d", len(ids), len(vectors))
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := 0; i < len(ids); i++ {
		vCopy := make([]float32, len(vectors[i]))
		copy(vCopy, vectors[i])

		var meta map[string]interface{}
		if metadatas != nil && i < len(metadatas) {
			meta = metadatas[i]
		}

		m.entries[ids[i]] = &vectorEntry{
			id:       ids[i],
			vector:   vCopy,
			norm:     calcNorm(vCopy),
			metadata: meta,
		}
	}
	return nil
}

func (m *Vector) Search(ctx context.Context, queryVector []float32, topK int, filter map[string]interface{}) ([]*storage.VectorSearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	qNorm := calcNorm(queryVector)
	if qNorm == 0 {
		return nil, nil
	}

	type scoredItem struct {
		id       string
		score    float32
		metadata map[string]interface{}
	}

	scored := make([]scoredItem, 0, len(m.entries))

	for _, entry := range m.entries {
		// 简单 filter 匹配
		if filter != nil {
			match := true
			for k, v := range filter {
				if entry.metadata == nil || entry.metadata[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		score := cosineSimilarity(queryVector, entry.vector, qNorm, entry.norm)
		scored = append(scored, scoredItem{
			id:       entry.id,
			score:    score,
			metadata: entry.metadata,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if topK > len(scored) {
		topK = len(scored)
	}

	results := make([]*storage.VectorSearchResult, 0, topK)
	for i := 0; i < topK; i++ {
		results = append(results, &storage.VectorSearchResult{
			ID:       scored[i].id,
			Score:    scored[i].score,
			Metadata: scored[i].metadata,
		})
	}

	return results, nil
}

func (m *Vector) DeleteVector(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.entries, id)
	return nil
}

func (m *Vector) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]*vectorEntry)
	return nil
}

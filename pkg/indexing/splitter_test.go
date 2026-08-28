package indexing_test

import (
	"strings"
	"testing"

	"github.com/qtopie/stargraph/pkg/document"
	"github.com/qtopie/stargraph/pkg/indexing"
)

func TestSplitDocument_ParagraphIntegrity(t *testing.T) {
	// 场景：两段各自独立的完整段落。
	// 段落 A 讲 StarGraph，段落 B 讲 Quantum DB。
	p1 := "StarGraph is a lightweight GraphRAG core engine built in pure Go. It supports hierarchical Leiden clustering and Local/Global search."
	p2 := "Quantum DB is an in-memory high-throughput temporal graph database maintained by Bob."
	content := p1 + "\n\n" + p2

	doc := &document.Document{
		ID:      "doc-semantic-split",
		Content: content,
	}

	// 设定 ChunkSize 刚好容纳一个段落 (比如 150 字符)
	cfg := indexing.SplitterConfig{
		ChunkSize:    150,
		ChunkOverlap: 20,
	}

	chunks := indexing.SplitDocument(doc, cfg)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks for 2 separate paragraphs, got %d", len(chunks))
	}

	// 验证第一段完全包含在 Chunk 0，第二段完全包含在 Chunk 1，段落内容没有被从中间腰斩
	if !strings.Contains(chunks[0].Content, "StarGraph") || strings.Contains(chunks[0].Content, "Quantum DB") {
		t.Errorf("Chunk 0 did not maintain paragraph 1 integrity: %s", chunks[0].Content)
	}

	if !strings.Contains(chunks[1].Content, "Quantum DB") {
		t.Errorf("Chunk 1 did not contain paragraph 2: %s", chunks[1].Content)
	}
}

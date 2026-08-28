package indexing

import (
	"crypto/md5"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/qtopie/stargraph/pkg/document"
)

// SplitterConfig 分块参数配置
type SplitterConfig struct {
	ChunkSize    int      // 单块期望最大大小 (按字符/Token 估算)
	ChunkOverlap int      // 重叠窗口大小
	Separators   []string // 层次化语义分隔符优先级列表 (从高到低：段落 -> 换行 -> 句子 -> 空格)
}

// DefaultSeparators 默认自然语义分隔符 (支持中英文段落与句子结构)
var DefaultSeparators = []string{
	"\n\n", // 双换行 (段落分界)
	"\n",   // 单换行
	"。\n",  // 中文句末换行
	".\n",  // 英文句末换行
	"。",    // 中文句号
	"！",    // 中文感叹号
	"？",    // 中文问号
	". ",   // 英文句号+空格
	"! ",   // 英文感叹号
	"? ",   // 英文问号
	"；",    // 中文分号
	"; ",   // 英文分号
	" ",    // 空格 (单词边界)
	"",     // 兜底：单字符强制切分
}

// DefaultSplitterConfig 默认分块配置
func DefaultSplitterConfig() SplitterConfig {
	return SplitterConfig{
		ChunkSize:    1000,
		ChunkOverlap: 100,
		Separators:   DefaultSeparators,
	}
}

// SplitDocument 将单个输入文档按自然语义边界 (段落/句子) 进行递归切分，确保完整段落与语义单元不被腰斩
func SplitDocument(doc *document.Document, cfg SplitterConfig) []*document.Chunk {
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 1000
	}
	if cfg.ChunkOverlap < 0 || cfg.ChunkOverlap >= cfg.ChunkSize {
		cfg.ChunkOverlap = 100
	}
	if len(cfg.Separators) == 0 {
		cfg.Separators = DefaultSeparators
	}

	content := strings.TrimSpace(doc.Content)
	if content == "" {
		return nil
	}

	// 1. 执行递归语义分块
	rawChunks := recursiveSplit(content, cfg.Separators, cfg.ChunkSize, cfg.ChunkOverlap)

	// 2. 包装为带唯一 MD5 与元数据的原子 Chunk
	chunks := make([]*document.Chunk, 0, len(rawChunks))
	for i, chunkText := range rawChunks {
		chunkText = strings.TrimSpace(chunkText)
		if chunkText == "" {
			continue
		}

		chunkID := fmt.Sprintf("chunk-%x", md5.Sum([]byte(doc.ID+":"+chunkText)))
		tokens := utf8.RuneCountInString(chunkText)

		chunks = append(chunks, &document.Chunk{
			ID:         chunkID,
			DocID:      doc.ID,
			Content:    chunkText,
			Tokens:     tokens,
			ChunkIndex: i,
			Metadata:   doc.Metadata,
			CreatedAt:  time.Now(),
		})
	}

	return chunks
}

// recursiveSplit 递归语义切分算法 (Recursive Character Splitting)
func recursiveSplit(text string, separators []string, chunkSize, chunkOverlap int) []string {
	if utf8.RuneCountInString(text) <= chunkSize {
		return []string{text}
	}

	// 选择当前级别适用的分隔符
	separator := ""
	newSeparators := make([]string, 0)
	for i, s := range separators {
		if s == "" || strings.Contains(text, s) {
			separator = s
			newSeparators = separators[i+1:]
			break
		}
	}

	// 按当前分隔符拆解为小片段
	var splits []string
	if separator != "" {
		splits = strings.Split(text, separator)
	} else {
		// 已经没有可用分隔符，按单个 rune 拆分兜底
		for _, r := range text {
			splits = append(splits, string(r))
		}
	}

	// 将小片段智能合并为 <= chunkSize 的块，并在块与块之间保持 chunkOverlap
	var goodSplits []string
	for _, s := range splits {
		if s == "" {
			continue
		}
		// 如果单个子片段依然超过 chunkSize 且还有下一级更细的分隔符，则递归切分它
		if utf8.RuneCountInString(s) > chunkSize && len(newSeparators) > 0 {
			nested := recursiveSplit(s, newSeparators, chunkSize, chunkOverlap)
			goodSplits = append(goodSplits, nested...)
		} else {
			goodSplits = append(goodSplits, s)
		}
	}

	// 根据当前分隔符重新合并，保证段落/句子尽量完整
	return mergeSplits(goodSplits, separator, chunkSize, chunkOverlap)
}

// mergeSplits 贪心合并片段，若当前块未超 chunkSize 则持续吸纳下一个完整段落/句子
func mergeSplits(splits []string, separator string, chunkSize, chunkOverlap int) []string {
	var docs []string
	var currentDoc []string
	currentLen := 0

	for _, piece := range splits {
		pieceLen := utf8.RuneCountInString(piece)
		sepLen := 0
		if len(currentDoc) > 0 {
			sepLen = utf8.RuneCountInString(separator)
		}

		if currentLen+pieceLen+sepLen <= chunkSize {
			// 当前块还能容纳下这个完整段落/句子，直接并入
			currentDoc = append(currentDoc, piece)
			currentLen += pieceLen + sepLen
		} else {
			// 当前块已满，封包
			if len(currentDoc) > 0 {
				docStr := strings.Join(currentDoc, separator)
				docs = append(docs, docStr)

				// 滑动保留重叠窗口 (Overlap)
				for currentLen > chunkOverlap && len(currentDoc) > 0 {
					removed := currentDoc[0]
					currentDoc = currentDoc[1:]
					currentLen -= utf8.RuneCountInString(removed)
					if len(currentDoc) > 0 {
						currentLen -= utf8.RuneCountInString(separator)
					}
				}
			}
			currentDoc = append(currentDoc, piece)
			currentLen += pieceLen
		}
	}

	if len(currentDoc) > 0 {
		docStr := strings.Join(currentDoc, separator)
		docs = append(docs, docStr)
	}

	return docs
}

// SplitDocuments 批量切分文档列表
func SplitDocuments(docs []*document.Document, cfg SplitterConfig) []*document.Chunk {
	allChunks := make([]*document.Chunk, 0)
	for _, doc := range docs {
		chunks := SplitDocument(doc, cfg)
		allChunks = append(allChunks, chunks...)
	}
	return allChunks
}

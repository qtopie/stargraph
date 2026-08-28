package indexing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/qtopie/stargraph/pkg/clustering"
	"github.com/qtopie/stargraph/pkg/graph"
	"github.com/qtopie/stargraph/pkg/llm"
	"github.com/qtopie/stargraph/pkg/storage"
)

// CommunityReportBuilder 负责对聚类社区调用 LLM 自底向上生成结构化摘要报告
type CommunityReportBuilder struct {
	llmClient  llm.Client
	graphStore storage.GraphStorage
	clusterer  *clustering.Clusterer
}

// NewCommunityReportBuilder 创建社区报告生成器
func NewCommunityReportBuilder(
	llmClient llm.Client,
	graphStore storage.GraphStorage,
	clusterer *clustering.Clusterer,
) *CommunityReportBuilder {
	return &CommunityReportBuilder{
		llmClient:  llmClient,
		graphStore: graphStore,
		clusterer:  clusterer,
	}
}

type communityReportJSON struct {
	Title             string  `json:"title"`
	Summary           string  `json:"summary"`
	Rating            float64 `json:"rating"`
	RatingExplanation string  `json:"rating_explanation"`
}

// BuildAllCommunityReports 执行聚类并在所有社区上生成报告存入 GraphStorage
func (b *CommunityReportBuilder) BuildAllCommunityReports(ctx context.Context) error {
	comms, err := b.clusterer.DetectCommunities(ctx, b.graphStore)
	if err != nil {
		return fmt.Errorf("detect communities: %w", err)
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 4)
	var mu sync.Mutex
	var firstErr error

	for _, comm := range comms {
		wg.Add(1)
		go func(c *graph.Community) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()

			if err := b.generateSingleReport(ctx, c); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			if err := b.graphStore.UpsertCommunity(ctx, c); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
		}(comm)
	}

	wg.Wait()
	if firstErr != nil {
		return fmt.Errorf("generate community reports: %w", firstErr)
	}
	return nil
}

func (b *CommunityReportBuilder) generateSingleReport(ctx context.Context, comm *graph.Community) error {
	nodes, err := b.graphStore.GetNodesByIDs(ctx, comm.NodeIDs)
	if err != nil {
		return err
	}

	var inputBuilder strings.Builder
	inputBuilder.WriteString("-----Entities-----\n")
	for _, n := range nodes {
		fmt.Fprintf(&inputBuilder, "- %s (%s): %s\n", n.Name, n.Type, n.Description)
	}

	prompt := fmt.Sprintf(CommunityReportPrompt, inputBuilder.String())
	resp, err := b.llmClient.Complete(ctx, prompt, llm.WithJSONMode(true))
	if err != nil {
		// 容错降级：若 LLM 失败，生成基于实体列表的基础汇总
		comm.Title = fmt.Sprintf("Community (Level %d)", comm.Level)
		comm.Summary = fmt.Sprintf("Community consisting of %d entities: %s", len(nodes), strings.Join(comm.NodeIDs, ", "))
		comm.Rating = 5.0
		comm.UpdatedAt = time.Now()
		return nil
	}

	var rep communityReportJSON
	if err := json.Unmarshal([]byte(resp), &rep); err == nil && rep.Title != "" {
		comm.Title = rep.Title
		comm.Summary = rep.Summary
		if rep.Rating > 0 {
			comm.Rating = rep.Rating
		}
	} else {
		comm.Title = fmt.Sprintf("Community (Level %d)", comm.Level)
		comm.Summary = resp
		comm.Rating = 5.0
	}
	comm.UpdatedAt = time.Now()
	return nil
}

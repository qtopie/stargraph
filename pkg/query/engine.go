package query

import (
	"context"
	"strings"

	"github.com/qtopie/stargraph/pkg/search"
)

// Engine 检索引擎统一路由与执行器
type Engine struct {
	localSearcher  *LocalSearcher
	globalSearcher *GlobalSearcher
	driftSearcher  *DRIFTSearcher
}

// NewEngine 创建统一查询引擎
func NewEngine(localSearcher *LocalSearcher, globalSearcher *GlobalSearcher, driftSearcher ...*DRIFTSearcher) *Engine {
	var ds *DRIFTSearcher
	if len(driftSearcher) > 0 {
		ds = driftSearcher[0]
	}
	return &Engine{
		localSearcher:  localSearcher,
		globalSearcher: globalSearcher,
		driftSearcher:  ds,
	}
}

// Query 执行统一分发查询
func (e *Engine) Query(ctx context.Context, req *search.Request) (*search.Result, error) {
	if req.Mode == "" || req.Mode == search.ModeAuto {
		req.Mode = e.routeIntent(req.Query)
	}

	switch req.Mode {
	case search.ModeLocal:
		return e.localSearcher.Search(ctx, req)
	case search.ModeGlobal:
		return e.globalSearcher.Search(ctx, req)
	case search.ModeDRIFT:
		if e.driftSearcher != nil {
			return e.driftSearcher.Search(ctx, req)
		}
		return e.localSearcher.Search(ctx, req)
	default:
		// 默认兜底使用 Local Search
		return e.localSearcher.Search(ctx, req)
	}
}

// routeIntent 轻量级自适应 Intent Router
func (e *Engine) routeIntent(query string) search.Mode {
	q := strings.ToLower(query)
	// 宏观概览类关键词走 Global Search
	macroKeywords := []string{"summary", "overview", "overall", "main themes", "all", "relationship between all", "总结", "全景", "概述", "核心主题", "宏观"}
	for _, kw := range macroKeywords {
		if strings.Contains(q, kw) {
			return search.ModeGlobal
		}
	}

	// 链路追踪/硬件排障类关键词优先走 DRIFT Search
	driftKeywords := []string{"drift", "path", "debug", "diagnose", "trace", "cause", "排查", "死锁", "时序", "路径", "根因", "故障"}
	for _, kw := range driftKeywords {
		if strings.Contains(q, kw) && e.driftSearcher != nil {
			return search.ModeDRIFT
		}
	}

	// 默认走精确拓扑推理 Local Search
	return search.ModeLocal
}

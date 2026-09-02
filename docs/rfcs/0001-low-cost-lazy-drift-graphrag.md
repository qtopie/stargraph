# RFC-0001: 低成本高性能 GraphRAG 混合架构演进 (Lazy Indexing + 0-LLM 共现图谱 + DRIFT Search)

- **Status:** Under Review
- **Author:** Antigravity Architecture Team
- **Created Date:** 2026-08-28
- **Target Specs:** `specs/modules/lazy_agrag_extractor.spec.md`, `specs/modules/drift_search.spec.md`

---

## 1. Summary (方案概述)

为了应对海量专业技术文档（如数万份芯片 Datasheet、电路手册等千万级 Token 场景）在建图与检索时面临的 **高昂 LLM 费用（“构建税”）** 与 **全量社区聚类耗时高** 的瓶颈，本提案为 StarGraph 引入三层降本增效核心架构：
1. **Lazy Indexing (惰性构建)**：在建库期省略昂贵的全量多层级 Community Report 预生成，将聚合计算推迟至查询期按需执行。
2. **AGRAG 统计学抽取器 (0-Token 快速建图)**：引入基于词频逆文档频率（TF-IDF）与滑动窗口共现分析的纯 CPU 统计学关系抽取器，提供完全零 LLM 调用的快速建图能力。
3. **DRIFT Search (动态路径穿梭检索)**：支持从种子实体出发的多步启发式穿梭与动态剪枝，取代全景暴搜，将单次查询上下文精准控制在极小范围。

---

## 2. Motivation & Cost Analysis (痛点与成本分析)

### 2.1 痛点
- **建库贵且慢**：面对数万份文档（约 7500 万 Token），传统 Eager GraphRAG 需全量调用 LLM 抽取实体/关系并层层生成社区报告，建库费用高达数千美元且频繁更新时开销剧增。
- **全局查询冗余**：对于技术排障类问题，Global Search 阅读全图社区报告会产生大量不相关的 Token 消耗。

### 2.2 降本效果预估
| 指标 | 传统 Eager GraphRAG | StarGraph 优化混合模式 (AGRAG + Lazy + DRIFT) | 降幅 |
| :--- | :--- | :--- | :--- |
| **初次建库 LLM 费用** | ~$3,000 - $8,000 | **$0** (纯 AGRAG 统计) 或 **<$50** (轻量 Embedding) | **>99%** |
| **建库耗时 (3万份文档)** | 数十小时 (受 API 速率限制) | **< 15 分钟** (本地 CPU 并行处理) | **>95%** |
| **单次精准排障查询费用** | ~$0.10 - $0.50 (Global) | **<$0.01** (DRIFT 动态穿梭) | **>90%** |

---

## 3. Detailed Design (详细架构设计)

### 3.1 索引管线重构：支持三种抽取模式
在 `pkg/indexing` 中定义通用抽取器接口 `Extractor`：
```go
type Extractor interface {
    ExtractChunks(ctx context.Context, chunks []*document.Chunk) ([]*graph.Node, []*graph.Edge, error)
}
```
* **Mode 1: LLMExtractor (已有)**：高精度，大模型语义抽取三元组。
* **Mode 2: CooccurrenceExtractor (AGRAG-style, 新增)**：
  - 分词并过滤停用词，提取专业术语/大写标识符（如 `REG_I2C_*`, `GPIO_PIN_*`）。
  - 在同一 Chunk 内使用滑动窗口统计 Term 共现频率与 TF-IDF 权重。
  - 共现超过阈值 $W_{min}$ 的实体自动连接语义边，边权重由共现度计算。
  - **特点**：0-LLM 调用，纯内存矩阵/滑动窗口计算，极速高吞吐。

### 3.2 索引策略：Eager vs Lazy
在 `pkg/stargraph.go` 配置中支持：
- `IndexMode: IndexModeEager`：抽取图谱 + 运行 Leiden 聚类 + 预先自底向上生成所有 Community Reports。
- `IndexMode: IndexModeLazy`：抽取图谱 + 建立向量索引，**跳过**全量 Community 报告预生成。

### 3.3 查询引擎升级：DRIFT Search 动态穿梭检索
在 `pkg/query` 中新增 `DRIFTSearch` 引擎：
1. **Seed Entity Retrieval**：基于用户 Query，通过向量相似度与关键词混合召回 Top-K 种子实体。
2. **Dynamic Path Traversal (穿梭探索)**：
   - 从种子实体出发，评估出边语义权重与关联 Chunk 相关性。
   - 启发式评分（Heuristic Score）选择最有价值的 $N$ 条路径进行深度探索（最大 Depth 默认 2~3）。
   - 剪枝（Pruning）：剔除低关联度邻域，防止上下文膨胀。
3. **Context Assembly & Answer Generation**：
   - 提取探索路径上的关键节点、边描述与高权重 Chunk 片段。
   - 组装紧凑 Prompt（通常 < 1500 Token），由 LLM 生成精准排障解答。

---

## 4. Implementation Steps & Milestones (实施计划与交付里程碑)

### 阶段一：索引层与 0-LLM 共现抽取器 (P1)
- [ ] 编写 Spec 契约 `specs/modules/lazy_agrag_extractor.spec.md`。
- [ ] 实现 `pkg/indexing/cooccurrence.go`（TF-IDF + 共现窗口抽取器）。
- [ ] 在 `pkg/stargraph.go` 中引入 `IndexModeLazy`，跳过无必要的社区报告生成。
- [ ] 编写基准测试与单元测试：验证 0-LLM 抽取的构建性能与吞吐。

### 阶段二：检索层与 DRIFT Search 动态穿梭 (P2)
- [ ] 编写 Spec 契约 `specs/modules/drift_search.spec.md`。
- [ ] 实现 `pkg/query/drift.go`（动态路径遍历、启发式打分、局部上下文裁剪）。
- [ ] 在 `pkg/query/engine.go` 与 `IntentRouter` 中集成 DRIFT 查询模式。
- [ ] 编写单元测试与多跳路径推理测试用例。

### 阶段三：集成验证、Harness 评估与基准报告 (P3)
- [ ] 编写大规模数据模拟夹具（`harness/fixtures/datasheet_mock.json`）。
- [ ] 更新 Harness Runner（`harness/runners/spec_runner.sh`）以覆盖 Lazy、AGRAG、DRIFT 契约。
- [ ] 执行全套自动化验证 `./scripts/check.sh`。
- [ ] 输出性能与 Token 成本对比基准报告。

---

## 5. Security & Performance Considerations (性能与安全考量)
- **并发安全**：共现抽取器与 DRIFT 遍历完全基于只读/并发安全内存结构设计，确保多请求并发下无锁争用。
- **内存占用控制**：共现抽取器采用稀疏共现矩阵与阈值过滤，防止图边数量发生组合爆炸。
- **离线安全与隐私**：AGRAG 模式与本地嵌入模式支持 100% 离线脱网运行，无数据外发风险。

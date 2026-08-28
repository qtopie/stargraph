# Project Layout & Module Boundaries

## Directory Structure
- `specs/`: Single Source of Truth (SSOT) 规范与契约
- `harness/`: Harness 评估与测试夹具工程
  - `harness/fixtures/`: 自动化测试用例静态数据与 Mock 存根
  - `harness/mocks/`: 外部依赖、数据库与 RPC 接口桩服务
  - `harness/runners/`: BDD 场景套件执行器与不变量断言点
  - `harness/docker-compose.yml`: 容器化测试沙盒定义
- `docs/`: 方案设计、RFCs、Bug RCA、规范文档
  - `docs/testing/guidelines.md`: 全局测试规范与工具链指引
  - `docs/testing/harness-engineering.md`: Harness 评估与测试套件指南
- `testings/`: 自动化集成测试与 E2E 测试集
- `scripts/`: 构建与校验自动化工具链（`check.sh`, `check-harness.sh`, `check-spec-drift.sh`）

## Go Package Architecture Guidelines
- **Package by Feature / Subdomain (禁止水平通用层 package)**:
  - 代码按领域能力与业务边界组织，如 `pkg/graph` (图实体/拓扑)、`pkg/document` (文档/Chunk)、`pkg/search` (检索/路由)、`pkg/storage` (存储驱动)。
  - **严禁创建 `domain`, `models`, `types`, `common`, `util` 等通用/杂货铺包**，避免循环导入与语义丧失。
- 依赖调用关系由外向内单向流动，保持无环图（DAG）。

## Module Dependencies Rule
- 模块间依赖必须保持单向流转，禁止循环依赖。
- 业务代码实现必须时刻保持与 `specs/` 契约一致。
- 单元测试放在与被测代码同级的目录下；集成/E2E 测试统一放在 `testings/` 下；测试夹具与桩集中在 `harness/` 中。

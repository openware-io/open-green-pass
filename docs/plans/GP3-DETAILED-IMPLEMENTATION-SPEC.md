# GreenPass · GP3 AI 治理实施细化（开发前规格）

> 状态：草案，2026-10-02。依据 `IMPLEMENTATION-PLAN.md` 的 GP3-01～07、`TECH-DESIGN.md`、`ENGINEERING-SPEC.md` 与当前代码审阅形成。
>
> 本文不修改既有实施计划，也不把“参考实现”表述为真实环境能力。`api/openapi.yaml` 仍是所有对外 HTTP 契约的唯一来源；本文给出的端点和事件为应进入该契约/事件版本文件的规格，而非已存在接口。

## 0. 审阅结论与统一约束

### 0.1 已有基础与明确缺口

| 能力 | 已有基础 | 尚未具备的生产闭环 |
|---|---|---|
| GP3-01 AI 网关 | `internal/ai` 有 `ModelProviderPort`、fallback 顺序、进程内并发信号量、`request_id` 校验和 fake provider 测试 | 真实 LiteLLM/供应商适配、超时/重试分类、业务归因、价格快照、持久化模型配置、受控密钥、与 trusted 成本账本的真实 MeterPort |
| GP3-02 W1 生成管道 | 治理域已有仓库、分支、版本化用例；执行 W2 已演示 Temporal 组装方式 | `GenBatch` 权威表、W1 workflow/activity、审核 signal、源码快照、种子/质量结果、按 change 写入与可回退 |
| GP3-03 成本趋势 | `cost_line_item` 是 append-only 权威明细，已有总览/明细/单用例历史查询 | Timescale 扩展、时序投影、连续聚合、趋势 API、对账与回补机制 |
| GP3-04 审计锚定 | `aud_event`、哈希计算、append API 和 RLS 已有 | 并发串链、确定性编码规范、锚点表/调度、不可变外部落点、独立验证器与 API |
| GP3-05 模型治理 | `tgt_target.model_binding` JSONB 和内存 `ModelConfig` 类型存在 | 模型池/价格/审批/白名单权威模型、绑定与解析服务、管理 API、密钥引用与审计 |
| GP3-06 篡改/变异 | PRD/原型已有“断言弱化、跳过标记、变异分数”展示和门禁语义 | 受支持语言/框架范围、分析器/执行器、证据模型、结果 API、门禁接入 |
| GP3-07 DB 权限与安全 | 迁移创建了 `gp_trusted_writer` 角色；代码按 Trusted 应用端口集中写审计/成本 | 真实 `GRANT/REVOKE`、独立连接凭据、append-only DB 保护、角色集成测试、备份/密钥/沙箱安全基线 |

### 0.2 先决的架构决议（不得由编码自行猜定）

下列决议会改变数据模型、运行时安全边界或成本口径，必须由产品/架构负责人确认后才能宣称对应条目“开发完成”。未决时，可以完成不依赖其取值的接口、端口、fake 与单测。

1. **模型供应商与网关策略**：首个真实 provider 是否统一经 LiteLLM；网关基址、认证方式、允许的模型清单、fallback 顺序、失败重试边界与单团队/单模型预算。
2. **模型配置权威归属**：工程规范将 `mdl_*` 登记为 Governance，现有 `internal/ai/domain.ModelConfig` 又表达同一概念。应明确“Governance 是模型配置和绑定唯一写者、AI 只解析只读快照”或调整既有分域；不得双写。
3. **密钥托管与锚点外部落点**：选择 Kubernetes Secret、Vault 或云 KMS；审计锚点选择支持不可变保留的对象存储、外部时间戳服务或其他可独立核验载体。没有此决议只能做内部锚点，不得称“独立锚定”。
4. **Timescale 可用性与保留策略**：部署是否提供 TimescaleDB 扩展；趋势时间粒度、保留期、刷新延迟和可接受的最终一致窗口。
5. **AI 专项的首批支持范围**：篡改静态规则覆盖哪些语言/测试框架；变异执行首个支持的 runner（例如 Go testing 或 Java/JUnit）以及性能预算、门禁阈值与误报处理人。
6. **安全基线**：生产/私有化的 RPO/RTO、备份介质、密钥轮换周期、沙箱信任边界与允许出网策略。GP3-07 的“Security 横切”须按这套基线验收，不能以本地开发配置代替。

### 0.3 共同工程规则

- 分层固定为 `api -> application -> domain <- infra`；领域层不得依赖 Temporal、HTTP、数据库驱动、Redis 或 provider SDK。
- 新的 HTTP request/response 先落 `api/openapi.yaml`，再运行 `npm run api:check`；事件要定义稳定的 `type`、`version`、`event_id`、`occurred_at`、`team_id`、关联 ID 与幂等键，不能直接传领域对象。
- 权威数据只能写入所属域；AI 写成本只能经 Trusted 的显式 `MeterPort`/内部契约，不能直接写 `cost_line_item`。审计只能经 Trusted 审计端口。
- 所有时间 UTC 存储；所有业务表带 `team_id` 并启用 RLS；迁移只增不改。敏感输入、完整 prompt/response、API key 不写日志和审计 payload。
- 每个新增 workflow/activity、外部模型调用、成本写入和锚定操作带 `trace_id`、`request_id`、`team_id`、`target_id`、`gen_batch_id`（按链路最小集合）。

## 1. 跨 GP3 的契约与事件基线

### 1.1 内部版本化事件信封

若 W1、成本时序投影、审计锚定采用异步连接，新增 `pkg/protocol` 或明确的 infra event 包（不要放领域对象）定义下列最小信封：

```json
{
  "event_id": "evt_...",
  "type": "gp.ai.invocation.v1",
  "version": 1,
  "occurred_at": "2026-10-02T00:00:00Z",
  "team_id": 100,
  "trace_id": "...",
  "idempotency_key": "...",
  "data": {}
}
```

首批事件：`gp.ai.invocation.v1`、`gp.generation.batch-state-changed.v1`、`gp.generation.reviewed.v1`、`gp.cost.recorded.v1`、`gp.audit.anchored.v1`、`gp.quality.analysis-completed.v1`。事件表或 Stream 消费方必须以 `event_id` 去重；失败消费可重放，不能重写权威成本或审计事件。

### 1.2 建议的开发顺序

1. **先完成 GP3-07 的最小可信写边界设计和角色集成测试骨架**，再让 AI 调用把成本/审计当作真实依赖。
2. **GP3-05（模型池/绑定/价格快照）→ GP3-01（真实网关）**：没有批准模型、价格和密钥引用，网关无法给出可审计的真实成本。
3. **GP3-02 W1**：依赖可用网关、治理域用例端口和 Trusted 审计/成本端口；可先以 fake provider/workflow test 开发。
4. **GP3-03 趋势投影**：以已稳定的 `cost_line_item` 为唯一账本，不反向影响写入链。
5. **GP3-04 锚定/独立校验**：可与趋势并行，但其并发链修复是前置；外部锚点依赖决议。
6. **GP3-06 篡改/变异**：静态检测可先行；真实变异运行依赖 GP2 对应沙箱/runner 的环境准备和首批语言决定。

## 2. GP3-01 统一 AI 网关（LiteLLM → 自研）

### 2.1 目标与边界

把所有 AI 调用收敛到一个应用入口，确保：每次调用都有业务归因与 `request_id`；仅允许已审批模型；可配置 timeout/fallback/并发/预算；成功或终态失败都形成最小审计痕迹；成功调用的用量只向 Trusted 成本账本写一次。首个版本不提供任意 prompt 代理和不受控的用户自定义 base URL。

现有 `GenerateRequest` 仅含 `request_id/model/prompt/max_tokens`，不足以计量归因；现有 `Gateway.seen` 是进程内永久 map，不能作为跨进程幂等真相，也不应在长期服务中无限增长。

### 2.2 必须确认的外部依赖

- 首个实际接入：LiteLLM 统一代理，还是直接接入一个指定 provider；其 TLS、认证 header、健康检查和 API 限流契约。
- 模型能力矩阵（文本生成、结构化输出、判定、embedding 等）及首批仅支持的能力。
- provider 返回 token 不可用时的计量政策：拒绝计费、采用明确的估算规则，或记录“usage unavailable”；不能静默写 0 并当成真实成本。
- 请求和响应保留策略：默认只保留 hash/长度/结构化摘要；若要留原文用于“决策痕迹”，需指定加密存储、访问权限与保留期。

### 2.3 OpenAPI 与事件契约

网关本身首先是内部服务，不建议暴露通用 `POST /ai/generate` 给浏览器。对外 API 由 GP3-02 的生成批次和 GP3-05 的模型管理承接。必须增加的可查询契约：

- `GET /models`：仅返回当前团队可见且经权限过滤的模型元数据，不返回 `api_key_ref`、base URL 私密部分或价格密钥。
- `GET /targets/{targetId}/model-binding`：返回该对象生效绑定（包括 fallback policy 的非敏感摘要与版本）。
- 运营查询若确有需求，新增 `GET /ai/invocations`，按时间/target/batch/request_id 分页，只返回 usage、耗时、选路与错误分类；此端点是否公开给 admin/viewer 必须由权限矩阵决定。

`gp.ai.invocation.v1.data` 最少包括 `request_id`、`invocation_id`、`target_id`、`gen_batch_id?`、`run_id?`、`case_id?`、`biz_point`、`model_config_id`、`resolved_model`、`price_version_id`、`tokens_in/out?`、`latency_ms`、`outcome`、`error_class?`。成本写入使用同一个 `request_id + biz_point + attempt` 的显式幂等键；若允许同一业务点多次模型尝试，`attempt` 必须进入键，不能让 fallback 覆盖第一次调用的审计事实。

### 2.4 数据模型与迁移

在先决决议“配置归属”确认后新增：

- `mdl_model`：`id/team_id/name/provider/model_key/capabilities/status/endpoint_ref/api_key_ref/default_timeout_ms/max_tokens_in/max_tokens_out/created_*`。`api_key_ref` 只存 Secret/KMS 引用。
- `mdl_binding`：`id/team_id/target_id/capability/model_id/fallback_model_ids_json/policy_version/status/created_*`。核心查询列必须是普通列；fallback 列表可为扩展 JSONB，但选择结果要写快照。
- `cost_price`：`id/model_id/currency/input_per_1k/output_per_1k/effective_from/effective_to/status/approved_by`。调用时将单价版本/单价快照复制至 `cost_line_item` 或调用事实表，历史账本不被后续调价改写。
- `ai_invocation`（AI 域调用事实，非账本）：`id/team_id/request_id/biz_point/target_id/gen_batch_id/run_id/case_id/model_id/price_id/attempt/status/tokens_*/latency_ms/error_class/created_at`，唯一键至少为 `(team_id, request_id, biz_point, attempt)`。

`mdl_` 已在工程规范登记，`cost_` 归 Trusted；`ai_` 尚未登记表前缀。若需要 AI 调用事实表，必须先在工程规范登记前缀后再建迁移。

### 2.5 分层职责

- **domain**：`InvocationIntent`、`ResolvedModel`、`Usage`、`RoutingPolicy` 不变量；端口 `ProviderPort`、`InvocationRepository`、`CostMeterPort`、`ModelResolverPort`。
- **application**：校验 request/biz context，读取生效绑定，冻结调用价格/路由快照，选择 primary/fallback，按错误分类决定是否尝试 fallback，持久化调用事实，调用 Trusted meter；不直接依赖 LiteLLM SDK。
- **infra**：LiteLLM HTTP client（按调用 deadline）、provider 错误映射、数据库 repository、Secret resolver、OTel instrumentation、受限并发器；meter adapter 只调用 Trusted 明确端口。
- **api**：模型/调用查询 DTO、鉴权、分页、错误码映射；不得把 prompt 或 domain 对象直接返回。

### 2.6 验收测试

- domain：不批准模型、能力不匹配、空 request ID、无可用 fallback、预算超过的拒绝。
- application：primary 成功；可重试错误 fallback；不可重试 4xx 不 fallback；每次可计量调用只提交一次账本；调用重放不重复记账。
- infra：mock LiteLLM 覆盖 timeout、429、5xx、无 usage、响应格式异常；真实环境只在受控 fake/测试模型中做 smoke。
- api/契约：模型/绑定查询可租户隔离，敏感字段不泄露，OpenAPI 类型更新。
- 可观测：一个调用能关联 invocation、cost_line_item、audit 事件和 trace；日志不含 prompt/key。

### 2.7 依赖

依赖 P1 的 target/RLS/cost/audit；依赖 GP3-05 的审批模型、绑定和价格；GP3-02/W1 反向依赖本项；GP2 仅在 AI 被用于执行判定时需要其运行上下文。

## 3. GP3-02 完整 W1 用例生成管道（Temporal）

### 3.1 目标与边界

实现可重试、可审计、人工审核的生成批次：冻结源码版本 → 解析并归一化 → AI 生成候选种子 → 质量检查 → 等待审核 → 原子提交用例版本或终止/回退。生成的是候选，不得在人工审核前修改 `cas_case/cas_version` 的当前版本。

“回退”需区分两种语义：**审核前取消**只终止 batch/保留证据，不写用例版本；**审核后回退**必须由治理域新增一条 `change_type=rollback` 的版本记录，引用原版本内容，不能物理删除已经发布的版本。

### 3.2 必须确认的外部依赖

- SCM 首批类型及读凭据（Git/具体 GitHub、GitLab、Gitee）；拉取的是 commit SHA 还是可变 branch。建议必须冻结 `repo_id + branch + head_sha + requested_version`。
- 可解析的上游输入（OpenAPI、代码、需求、接口文档等）与允许文件大小/路径白名单；源码/文档快照放对象存储还是临时卷。
- 人工审核角色（owner/admin/tester 中谁可 approve/reject/rollback）与审核超时政策（超时保持 review、自动拒绝或升级）。
- 生成质量的首批客观规则：重复、缺失、格式/场景覆盖、可执行性；AI 评价若参与，必须走 GP3-01 且单独计量。

### 3.3 OpenAPI、Signal 与事件

建议新增以下契约（路径和 schema 以 `api/openapi.yaml` 落地为准）：

- `POST /generation/batches`：创建批次，输入 `target_id/repo_id/branch/version?/source_selectors/idempotency_key`；返回 `GenerationBatch`（202 或 201 由“启动 workflow 是否同步确认”统一决定）。
- `GET /generation/batches/{id}`：批次元数据、当前状态、冻结 SHA、步骤状态和失败摘要。
- `GET /generation/batches/{id}/seeds`：候选用例分页，包含 source locator、质量结论、可审核摘要，不返回完整 prompt。
- `POST /generation/batches/{id}/review`：`decision=approve|reject`、`comment?`、`expected_state_version`。审核动作必须幂等，过期状态返回 409。
- `POST /generation/batches/{id}/rollback`：仅适用于已提交 batch；输入 rollback 理由和 optimistic version。
- `GET /generation/batches/{id}/events`：如需前端实时进度，沿用有租户头的 fetch stream 模式，不能依赖无法携带团队头的原生 EventSource。

Temporal signal 固定为 `generation-review` 与 `generation-rollback`，payload 含 batch ID、决策、actor、request ID、state version。事件分别发出 `gp.generation.batch-state-changed.v1` 与 `gp.generation.reviewed.v1`；每次状态迁移（含 reject/timeout）都审计。

### 3.4 数据模型与迁移

`gen_` 已登记给 AI 生成批次，可新增：

- `gen_batch`：`id/team_id/target_id/repo_id/branch/requested_version/head_sha/state/state_version/workflow_id/idempotency_key/parent_batch_id/created_by/created_at/...`；唯一 `(team_id,idempotency_key)`。
- `gen_source_snapshot`：输入对象引用、content hash、parser version、selector、object URI；原始大内容不塞主库。
- `gen_seed`：候选用例的稳定编号、batch ID、source ref、normalized spec JSONB、dedupe hash、quality state、approval state；唯一 `(batch_id,dedupe_hash)`。
- `gen_review`：append-only 审核动作，含 actor/decision/comment/state_version/occurred_at。
- `gen_step`（若需可恢复的应用查询投影）：活动名、attempt、状态、开始/结束、非敏感错误摘要。Temporal history 不是业务查询 API 的替代。

不在 `gen_batch` 内直接存完整 prompt/response；如保留决策痕迹，存加密对象引用、hash、保留策略 ID。

### 3.5 分层职责与 workflow 划分

- **domain**：`GenBatch` 状态机 `enqueued → fetching → parsing → generating → quality_check → review → committing → approved|rejected|rolled_back|failed`；只定义状态转换和审核/回退不变量。
- **application**：创建/查询批次、审核命令、治理域 CaseVersion 提交端口、Trusted 审计端口、AI 网关端口；每个 activity 可重入。
- **workflow**：仅编排、活动重试/超时、Signal 等待、ContinueAsNew 边界；不得访问 DB、HTTP 或 provider。workflow ID 采用稳定的 `generation/<team>/<batch>`，启动须处理已存在的幂等请求。
- **activities/infra**：FetchSource、ParseAndNormalize、GenerateSeeds、QualityGate、CommitVersions、EmitAudit；SCM/对象存储/provider/PG 实现各自通过端口注入。
- **governance integration**：用例版本写入由 Governance application 的显式 command 完成，AI 不得直接写 `cas_*` 表。

### 3.6 验收测试

- workflow：正常批准、审核拒绝、审核超时、signal 重放、activity 重试、worker 重启后继续、审核与 rollback 竞态。
- application：同 idempotency key 重复创建只得到同一 batch；重复 seed 去重；提交失败不产生半组 case version；回退生成新版本而不删除历史。
- integration：fake SCM + fake AI + testcontainer PG/Temporal；跨团队读取/审核被拒绝；每个状态迁移有审计链事件，AI 成本与 batch/request 对得上。
- E2E（环境具备后）：冻结 SHA 与提交版本可回查；前端进度、审核和版本历史一致。

### 3.7 依赖

依赖 P1 的 repo/case version/RLS；依赖 GP2-02 的 Temporal 基础设施和 worker 运行方式；强依赖 GP3-01 网关、GP3-05 生效模型、GP3-07 Trusted 写权限；可在没有真实 SCM/provider 时先完成 fake workflow 测试。

## 4. GP3-03 成本趋势（Timescale）

### 4.1 目标与边界

在不改变 `cost_line_item` 为唯一权威账本的前提下，提供团队/对象/用例/模型/业务类别的时间趋势和历史对比。趋势表是**可重建投影**，不可成为计费或门禁判定的唯一来源；总览继续以权威明细聚合/可对账为准。

### 4.2 必须确认的外部依赖

- 目标 PostgreSQL 是否启用 TimescaleDB，版本和扩展安装权限；不具备时的降级方案是普通 PG 汇总查询还是暂不交付趋势。
- 时间桶（首版建议小时 + 天）、刷新延迟、保留期/压缩策略、报表的时区展示。
- “存量执行成本”的识别规则：按 `case_id`、case version、场景还是测试数据版本比较；不先定义基线，不能宣称递减。

### 4.3 OpenAPI 与事件契约

新增只读端点建议为：

- `GET /cost/trend?from&to&bucket=hour|day&target_id?&case_id?&model?&category?`，响应包含 `bucket_start/category/amount/tokens_in/tokens_out/call_count` 和查询口径。
- `GET /cost/compare/{caseId}` 现有端点保留明细历史；可增加 `baseline`/`group_by`，但破坏性语义变更需 OpenAPI 兼容窗口。
- `GET /cost/reconciliation?from&to` 仅 admin/审计角色，返回权威账本金额、趋势投影金额、差异和刷新时间。

`gp.cost.recorded.v1` 以写入成功的 `cost_line_item.id` 为 source ID，投影消费者按 `event_id` 或 source ID 去重，绝不能把 AI provider 事件直接当成本账本。

### 4.4 数据模型与迁移

不要把已有 `cost_line_item` 直接改为 hypertable：其现有 `UNIQUE(idempotency_key)` 未必满足 Timescale 分区键约束，直接转换风险高。建议新增可重建投影：

- `ts_cost_fact`：`occurred_at/team_id/target_id/run_id/case_id/model/biz_category/biz_point/source_cost_id/amount/tokens_in/tokens_out`；以 `occurred_at` 建 hypertable，索引 `(team_id, occurred_at DESC)` 与常用筛选组合。
- `ts_cost_projection_checkpoint`：消费者位点、最后 event/source ID、更新时间、错误摘要；只用于投影恢复。
- 连续聚合 `cagg_cost_hourly`、`cagg_cost_daily`（具体对象名和是否物化由 DBA/Timescale 决议确认）；不在趋势表保存可变业务事实。

迁移必须先检查扩展与权限，提供明确失败信息；不应在生产迁移里静默安装未知扩展。回填读取 `cost_line_item`，带 source ID 去重，可重复执行。

### 4.5 分层职责

- **trusted/domain**：成本趋势查询值对象和账本-投影对账规则；不会重新计算单条金额。
- **application**：按 tenant/filter 查询连续聚合；触发/编排回填；对账返回差异状态。
- **infra**：Timescale SQL、projection consumer/定时刷新、checkpoint repository；运行时检测 Timescale 能力。
- **api**：查询参数严格限制时间区间/粒度/分页，DTO 标明 bucket 时区和 `refreshed_at`。

### 4.6 验收测试

- 账本写 N 条不同类别/模型/租户成本，趋势金额、token、调用数逐桶精确汇总；总额与账本相等。
- 重放同一成本事件不重复；消费者中断后从 checkpoint 恢复；延迟数据会在下一刷新窗口出现。
- 跨 tenant、跨 target、跨 case 过滤隔离；历史比较对同一基线计算正确。
- Timescale testcontainer/CI 镜像不可用时，明确标记为环境前置，不用普通 PG 测试冒充 continuous aggregate 验收。

### 4.7 依赖

强依赖 GP1 成本账本、GP3-01 的真实计量；与 GP3-02 并行但其生成成本会成为数据源；基础设施依赖 Timescale 部署，和 GP2 无直接代码阻塞。

## 5. GP3-04 审计哈希链锚定与独立校验

### 5.1 目标与边界

将现有 append-only 事件链升级为：同团队事件在并发写入下仍严格单链；定期生成可验证锚点；可由不依赖业务服务写权限的工具/API 验证指定区间、发现缺失/篡改/链断裂。锚点能证明“某时刻前的链头已被固定”，但不能阻止拥有数据库超级权限的攻击者重写所有本地数据；这一威胁边界必须在产品文案中明确。

当前 `LatestHash()` 与 `Append()` 是两个独立数据库操作；并发 append 可读取同一个 prev hash。现有 hash 对 `map[string]any` 的序列化没有显式跨语言 canonical JSON 规范。这两点必须先修正，才能进行锚定。

### 5.2 必须确认的外部依赖

- 锚点的外部不可变载体：优先支持 Object Lock/WORM 的对象存储，或经合规认可的 RFC3161 时间戳服务；若只写普通数据库表，只能叫“内部快照”。
- 锚定周期（例如按时间和每 N 条双阈值）、允许延迟、锚点存留期、密钥签名算法和 public key 发布方式。
- 独立验证 API 是对全部审计读者开放、仅 auditor/admin，还是以一次性 verification token 提供；这会影响 payload 脱敏和认证设计。

### 5.3 OpenAPI 与事件契约

建议新增：

- `GET /audit/events`：按 team/asset/时间/keyset 分页，脱敏后返回 event header/hash，不能把未授权 payload 原样公开。
- `POST /audit/verify`：提交 `from_event_id?/to_event_id?/anchor_id?`，返回 `valid/checked_count/first_invalid_event_id?/reason/anchor_reference`；长区间可返回 202 + verification job。
- `GET /audit/anchors` 与 `GET /audit/anchors/{id}`：锚点元数据、区间、链头 hash、外部对象 URI 的非敏感引用、签名/证书信息。

`gp.audit.anchored.v1` 数据包括 `anchor_id/team_id/first_event_id/last_event_id/first_hash/last_hash/event_count/manifest_hash/external_reference/anchored_at/key_id`。验证结果为查询结果，不应追加为新的业务审计事件，避免验证行为污染被验证链；锚定动作本身应 append 审计。

### 5.4 数据模型与迁移

- `aud_chain_head`：`team_id` 主键、`last_event_id/last_hash/chain_seq/updated_at`；仅是控制状态，可更新。append 事务对该行 `SELECT ... FOR UPDATE`，插入 event 后更新 head，保证单链。
- `aud_anchor`：append-only，包含区间 event/seq/hash、manifest hash、签名、`external_ref`、状态、时间和创建者；锚点表本身同样受最小写权限保护。
- `aud_verify_job`（仅在异步验证需要时）：请求摘要、状态、范围、结果/错误；不能将巨大的全量验证细节当作审计 payload。
- 追加 `aud_event.chain_seq` 或等价逻辑顺序字段，按 `(team_id, chain_seq)` 唯一，避免同一时间戳的排序歧义。

哈希输入必须写成可跨语言实现的二进制规范，例如：`SHA-256(version || prev_hash_bytes || chain_seq_be || RFC3339Nano_UTC || UTF-8(op) || UTF-8(asset) || asset_id_be || canonical_json(payload))`。实际规范版本保存于事件，历史算法不得改写。

### 5.5 分层职责

- **domain**：哈希规范版本、链块/锚点值对象、校验失败分类；无 JSON/DB 依赖（canonical payload 由上层提供 bytes/hash）。
- **application**：一个事务内 append/head 更新的用例、锚点选择、manifest 构建、验证编排；不能吞掉 audit 写失败。
- **infra**：PG 锁与事务、canonical JSON adapter、对象存储 WORM/timestamp client、签名器、只读独立 verifier CLI 的数据 reader。
- **api**：审计查询/验证 DTO、严格鉴权和 payload 脱敏。独立 CLI 建议为 `cmd/auditverify`，使用只读 DSN 与 anchor manifest，不复用 server 内存状态。

### 5.6 验收测试

- 并发 append 同租户 100+ 次：`chain_seq` 连续、无分叉、每块 prev/hash 校验成功；跨租户独立链。
- 修改 payload/hash/prev_hash、删除中间 event、调整时间均被 verifier 指出第一失效点。
- 生成 anchor 后，用独立 CLI 在无写凭据的环境验证 DB 区间和外部 manifest；外部对象不匹配时失败。
- 无锚点区间返回“链本身有效但未被外部锚定”，而不是误报通过。

### 5.7 依赖

依赖 GP1 审计表与 GP3-07 角色边界；W1、网关、篡改检测都应使用稳定的 append port。可与 GP3-03 并行；不依赖 GP2 真实执行器。

## 6. GP3-05 模型治理与白名单

### 6.1 目标与边界

让“团队可用模型、工程绑定、默认模型、审批/禁用、价格版本、能力限制”成为可审计且可执行的权威配置。`tgt_target.model_binding` 的 JSONB 只适合作为迁移期兼容读模型，不能继续承担完整模型生命周期；否则无法做 FK、状态、审批或价格历史。

### 6.2 必须确认的产品决策

- 谁可以发起接入、审批、禁用和设置默认（建议以 owner/admin 为候选，须正式确认）。
- 白名单级别：团队级、工程级，还是环境级；私有模型 endpoint 是否允许同一团队多个。
- 默认选择规则：target 绑定覆盖 team 默认，是否允许 ancestor asset 继承，能力不匹配时是 fallback 还是拒绝。
- 价格由人工配置、provider 同步还是合同价表；币种、税费和生效时间规则。

### 6.3 OpenAPI 契约

- `GET/POST /models`，`GET/PATCH /models/{id}`：模型元数据、状态、能力、价格摘要；写 API 不返回 key ref。
- `POST /models/{id}/approval`：`approve|reject|disable` + reason，带 optimistic revision。
- `GET/PUT /targets/{targetId}/model-binding`：绑定 `model_id/capability/fallback_model_ids?/parameters?`；参数需 schema 白名单，不能接受任意 provider passthrough。
- `GET /models/effective?target_id=&capability=`：供前端显示和排障，返回选中来源（target/team default/fallback）与版本，不暴露敏感项。

所有变更 append `model.*` 审计事件；对审批外的模型调用返回明确 `MODEL_NOT_APPROVED`/`MODEL_NOT_ALLOWED`，不是降级到任意模型。

### 6.4 数据模型与分层职责

使用 GP3-01 的 `mdl_model/mdl_binding/cost_price`，增加：

- `mdl_approval`：`id/model_id/requested_by/decision/decided_by/reason/revision/occurred_at` append-only。
- `mdl_team_default`：若 team 默认不放在 `mdl_model`，用 `(team_id, capability)` 唯一的指针表，便于原子切换和审计。

**Governance application** 负责模型/绑定生命周期和资产权限；**AI application** 通过只读 `ModelResolverPort` 取得不可变 `ResolvedModel`，随后只执行路由；**Trusted** 只记录带 model/price snapshot 的成本，不决定白名单。现有 `tgt_target.model_binding` 的迁移策略必须是：回填为 `mdl_binding` → 双读校验 → 新写只写新表 → 删除旧写路径（不改历史迁移）。

### 6.5 验收测试与依赖

- 权限：viewer/tester 无权审批；无资产权限者不能读/修改绑定；RLS 隔离。
- 选择：target 绑定优先、团队默认回退、禁用/未批准/能力不匹配全部拒绝或按已确认规则 fallback。
- 价格：调用时冻结 price version，之后价格变更不改历史成本。
- 迁移：旧 JSONB 绑定可回填且逐条对账；双读期发现不一致必须报警。

依赖 P1 target/租户，强依赖 GP3-07 保护密钥引用，随后解除 GP3-01/W1 的模型配置阻塞；与 GP2 无直接运行时依赖。

## 7. GP3-06 篡改检测与变异

### 7.1 目标与边界

把“测试未被弱化/跳过”和“用例对被测行为有杀伤力”转化为可回查证据与门禁输入。首版应先交付可解释、低风险的静态篡改规则；真实 mutation 必须限于一个已决定语言/框架和 runner，不能声称跨语言通用。

### 7.2 必须确认的范围（本项编码门槛）

1. 首批语言、测试框架和测试文件定位规则。
2. 静态规则集：例如 skip/disable、空测试、无断言、恒真断言、捕获异常不失败、宽泛 mock；每条规则的严重度和误报豁免流程。
3. mutation 算子、每次最大 mutant 数/超时、隔离 sandbox、可重试条件、阈值和因 flaky 测试导致的判定政策。
4. 结果归属：按 `case_version`、源 commit 还是 run；门禁是 block、warn 还是仅展示。
5. 新表前缀：现有规范没有 quality/tamper/mutation 前缀。必须经架构确认登记，例如归 `cas_`（用例质量结果）还是新增专用前缀，不能临时造表名。

### 7.3 OpenAPI、事件与数据模型

在范围确认后新增：

- `POST /cases/{caseId}/quality-analyses`：创建静态检测/变异任务，输入 case version、mode、规则集版本；对一次运行/CI 触发则应从 run/batch API 引用而非重复定义。
- `GET /quality-analyses/{id}`、`GET /cases/{caseId}/quality-findings`：返回 summary、finding、证据 hash/locator、规则/算子版本和关联 run/batch。
- 如需人工豁免：`POST /quality-findings/{id}/waivers`，必须有角色、理由、失效时间与审计；不能直接改 finding 状态。

事件：`gp.quality.analysis-completed.v1`、`gp.quality.finding-created.v1`。数据最少拆为 analysis job、finding、mutation execution、mutant result；原始源码/完整日志走受控对象存储，DB 保存 hash/reference。结果写入 gate 的输入快照，后续复判使用同一规则版本与证据，不重新猜测。

### 7.4 分层职责

- **domain**：规则严重度、finding 状态、mutation score 计算、豁免有效性；不能依赖 AST/测试 runner。
- **application**：选择兼容 analyzer、冻结规则集/源码版本、创建任务、合并结果、提交 Gate 输入与 Trusted 审计。
- **infra**：语言 AST/static analyzer、mutation runner adapter、对象存储证据、GP2 runner/sandbox adapter；mutation 仅在隔离资源池执行。
- **api**：结果/豁免 DTO、权限和审计。门禁的 Rego 接收结构化 summary，不直接执行 analyzer。

### 7.5 验收测试与依赖

- 静态黄金样本：每条规则有命中/不命中/允许豁免样本，规则版本变化可回归。
- mutation：给定小型 fixture，预期 mutant 总数、killed/survived/timeout 数和 score；超时/资源耗尽不会污染主执行队列。
- 门禁：低于已确认阈值阻断或告警，finding/豁免/规则版本和审计可回查。
- 安全：上传/拉取的代码只在 sandbox，日志脱敏，恶意测试不能访问控制面凭据。

静态 slice 依赖 P1 case version、GP3-04 audit、GP3-05 规则/权限（若 AI 辅助分析则 GP3-01）；mutation slice 强依赖 GP2 K8s/runner/资源配额的真实集成。

## 8. GP3-07 可信域 DB 写权限与 Security 横切

### 8.1 目标与边界

让“Trusted 是审计/成本/门禁结果唯一写者”在 PostgreSQL 权限层可独立验证，而不只依赖 Go 包约定；同时给出可测试的备份、密钥和 sandbox 最小安全基线。现有迁移只创建 `gp_trusted_writer`，没有 `GRANT/REVOKE`，当前 server/worker 仍使用同一 `DBDSN`，因此尚未满足工程规范 §8.1。

### 8.2 必须确认的环境决策

- 生产数据库角色由迁移用户创建，还是由 DBA/Helm bootstrap job 预建；受管 PG 往往不允许应用迁移创建角色。
- `gp_server`、`gp_worker`、`gp_trusted_writer`、只读 verifier/report reader 的实际连接字符串/Secret 命名和 rotation 流程。
- 哪些 trusted 表必须独占写：至少 `aud_event`、`cost_line_item`、`gate_result`；`gate_rule` 的编辑也应由 trusted application 承担，是否同角色写需确认。
- 备份目的地、RPO/RTO、恢复演练频率、加密/KMS、对象存储 WORM 与 sandbox 的网络/服务账号策略。

### 8.3 迁移、角色和运行时装配

新增迁移/部署 bootstrap（不改 000006/000007）：

1. 创建或校验角色，不将密码写入迁移；角色凭据由 Secret/外部密钥系统供给。
2. 对 trusted 表撤销 `PUBLIC`、`gp_server`、`gp_worker` 的 INSERT/UPDATE/DELETE；仅授予查询所需的 SELECT；`gp_trusted_writer` 获得最小 INSERT/SELECT（以及必要的 `gate_rule` 写权限）。
3. 对 `aud_event/cost_line_item/gate_result` 设置 DB 级 append-only 保护：触发器对 UPDATE/DELETE 抛错；RLS 强制并保证运行角色不能绕过。超级用户/对象 owner 的越权不在应用可防范围，需用审计锚点和运维权限治理覆盖。
4. `cmd/server`/`cmd/worker` 通过普通 DSN 装配治理/执行读写，通过独立 trusted-writer DSN 仅注入 Trusted infra。禁止把 trusted DSN 传入 execution/governance 包。
5. 为审计并发链实现 GP3-04 的 head-lock append；成本写与对应审计的原子性（或明确 outbox/补偿策略）必须文档化并测试。

### 8.4 安全最小交付物

- **密钥**：配置中仅允许 `*_SECRET_REF`/Secret mount；启动期拒绝明显明文 key；日志、panic 和审计 payload 统一脱敏；轮换后旧凭据失效的 smoke。
- **备份恢复**：加密备份、权限隔离、至少一次从备份恢复至隔离库并运行迁移/审计验证；记录实际 RPO/RTO，而不是只存在 Helm values。
- **Sandbox**：runner pod 使用 non-root、read-only root filesystem、drop capabilities、资源 limit、独立 service account、NetworkPolicy 默认拒绝且只允许明确 SUT/必要依赖出站；不挂载控制面 DB/Temporal/Secret 凭据。
- **供应链**：镜像固定来源/扫描、依赖漏洞门禁、K8s manifests 不提交秘钥。具体工具可选，但结果必须可复查。

### 8.5 验收测试

- 使用四类真实 PG 凭据测试：普通 server/worker 直接 `INSERT aud_event/cost_line_item/gate_result` 被拒绝；trusted writer 可按 RLS 写入；跨租户读写被拒绝；verifier 只读。
- 使用任意 SQL 尝试 UPDATE/DELETE append-only 表失败；审计/成本应用路径仍成功。
- 运行时测试证明 governance/execution 包没有获得 trusted-writer DSN，静态配置检查避免同一 DSN 误装配。
- 备份恢复演练和 pod security/NetworkPolicy 验证必须在真实 kind/目标集群环境执行；缺环境时只能标记为待验收。

### 8.6 依赖

GP3-07 是 GP3-01/02/03/04/05/06 可信写入的横切前置。其 DB 权限最小 slice 可以先于真实外部环境编码并用 testcontainer role tests 覆盖，但安全/恢复/沙箱验收依赖部署环境和用户决议。

## 9. GP3 阶段完成定义（建议）

只有同时满足下列条件，才能把 GP3 从“方案细化/代码完成”提升为“阶段验收完成”：

1. 经批准模型的调用经统一网关完成，模型、价格、token、request ID、batch/run/case 归因可追溯，重复/重试不双计。
2. W1 在 Temporal 上完成至少一个冻结 SHA 的真实或受控 fake 闭环：生成、质量检查、审核、提交、回退均产生可查状态与审计。
3. 成本趋势由可重建投影生成，任意验收时间窗可与 `cost_line_item` 对账为零差异或有明确延迟说明。
4. 审计链在并发写下无分叉，至少一个锚点能在独立只读验证器中核验；篡改/缺失样本被检出。
5. 模型白名单/审批/目标绑定在 API、运行时路由和成本快照三处一致；密钥未泄露。
6. 首批静态篡改规则和已确认范围内的 mutation 能生成可复现证据，门禁使用版本化结果。
7. trusted 写权限在真实 PG 角色测试中被强制；备份恢复、密钥和 sandbox 基线完成环境验收。

在真实 LiteLLM/SCM/Timescale/对象存储/Temporal/K8s 环境未验收前，任何项只能称“代码/测试替身完成”，不能称“可用于真实测试”。

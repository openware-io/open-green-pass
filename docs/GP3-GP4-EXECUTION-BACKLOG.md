# GP3 / GP4 可执行开发清单

> 版本：2026-10-02。本文是 `GP3-DETAILED-IMPLEMENTATION-SPEC.md`、`GP4-IMPLEMENTATION-SPEC-DRAFT.md` 的执行层清单。
> 每个批次都必须先完成“输入/决策”，再实现“代码产物”，最后执行“门禁”。未满足外部决策或真实依赖时，只能完成标为 `可先做` 的部分，不能把 fake、参考实现或本地测试写成生产完成。

## 0. 不可破坏的工程锚点

1. `api/openapi.yaml` 是唯一 HTTP 契约；修改后执行 `npm run api:generate`、`npm run api:check`。
2. 分层固定为 `api -> application -> domain <- infra`。domain 不依赖 HTTP、PG、Redis、Temporal、Kubernetes 或供应商 SDK。
3. 权威数据只由所属域写入：`cas_*` 由 governance，`cost_*`/`aud_*` 由 trusted，AI/平台域只能经端口调用。
4. 所有业务表必须有 `team_id`、RLS、UTC 时间；迁移只新增，不修改既有历史语义。
5. 事件使用版本化 envelope，消费者按 `event_id` 幂等；不能直接序列化领域对象作为事件或 HTTP 响应。
6. 任何真实集成完成必须同时具备：代码、契约测试、真实依赖验证、审计证据；只有代码和 fake 测试时标记为“参考实现”。

## 1. 总门禁与执行方式

### 1.1 每个编码批次的固定输出

- 一个独立 commit，commit message 使用 `feat/fix/refactor: ...`。
- 迁移（如有）、OpenAPI（如有）、生成类型（如有）、domain/application/infra/api 代码和测试。
- `docs/IMPLEMENTATION-PLAN.md` 对应条目更新为“代码完成/真实验收待完成”，不能直接写“完成”。
- 证据：测试命令、依赖版本、未验证项和阻塞决策。

### 1.2 固定验证命令

```text
go test ./...
go vet ./cmd/... ./internal/... ./pkg/...
git diff --check
npm run api:check
npm run typecheck
npm run lint:eslint
npm run build
```

只有所有适用命令通过，批次才可进入代码审查；真实集成另外执行对应的 kind/Temporal/Redis/供应商验收，不以本地命令替代。

## 2. 开始前必须锁定的决策

| 编号 | 决策 | 影响任务 | 未锁定时允许做什么 |
|---|---|---|---|
| D-01 | 首个 AI provider：LiteLLM 或指定供应商；模型、能力、fallback、超时、重试 | GP3-01/02/05 | 可做 provider port、fake、错误分类、配置校验 |
| D-02 | 模型配置唯一写域：治理域或 AI 域（二选一） | GP3-01/05 | 可做只读 resolver 接口和领域不变量，禁止双写 |
| D-03 | Secret/KMS 方案 | GP3-01/05/07、GP4-03/04 | 可做 `secret_ref` 端口，禁止存明文 key |
| D-04 | Timescale 是否可用、桶粒度、保留期、最终一致窗口 | GP3-03 | 可做投影端口和普通 PG fake，不能宣称趋势生产完成 |
| D-05 | 审计外部锚定载体 | GP3-04/GP4-04 | 可做内部锚点、独立 verifier，不能宣称外部锚定 |
| D-06 | 变异首批语言/框架、runner、阈值 | GP3-06 | 可做 provider-independent 静态规则和结果模型 |
| D-07 | 首批身份源与 RBAC 继承/拒绝规则 | GP4-01/03 | 可做 Principal/Permission domain 和 fake auth |
| D-08 | 首批 CI 平台和回写类型 | GP4-02 | 可做 connector/outbox/幂等投递和 fake provider |
| D-09 | 私有化拓扑、K8s 版本、依赖由客户提供还是内置 | GP4-04/05 | 可做 Helm schema、readiness/version、dry-run |
| D-10 | 租户分区策略与 cutover/RPO/RTO | GP4-05 | 可做 route resolver、placement 状态机、校验 CLI |
| D-11 | 压测目标、SLO、观测栈和 on-call | GP4-06 | 可做指标字典、load harness、dashboard/alert 模板 |

## 3. GP3 执行批次

### GP3-07A：可信写边界骨架（可先做）

**输入**：现有 `aud_event`、`cost_line_item`、`gp_trusted_writer` 迁移和 trusted application。

**代码产物**：

- `internal/trusted/domain`：append-only、幂等键、actor/team 不变量。
- `internal/trusted/infra`：writer/reader 分离的连接配置端口；测试 fake。
- 新增角色集成测试：非 writer 不能 INSERT/UPDATE/DELETE；writer 只能追加允许表。
- 安全日志过滤器：不得输出 token、手机号、Authorization、prompt/response 原文。

**门禁**：domain 测试、SQL 权限测试、跨租户 RLS 测试；真实 PG role 测试通过后才可标记 GP3-07 写权限完成。

### GP3-05A：模型池、绑定、价格快照（依赖 D-02/D-03）

**迁移**：`mdl_model`、`mdl_binding`、`cost_price`；所有表含 `team_id`、状态、版本和审计字段；密钥只存 `secret_ref`。

**domain/application**：`ModelConfig`、`ModelBinding`、`PriceSnapshot`、`ResolveModel`；禁止未批准模型、能力不匹配模型和越权绑定。

**API**：先更新 OpenAPI，再实现 `GET /models`、`PUT /targets/{id}/model-binding`、`GET /targets/{id}/model-binding`；响应不得泄漏 key、完整 endpoint 或内部 secret。

**门禁**：跨 team 读取为空；白名单外拒绝；价格快照不可被后续调价改写；审计绑定/审批动作。

### GP3-01A：AI 网关 provider-independent 硬化（依赖 GP3-05A 接口）

**代码产物**：`InvocationIntent`、`ResolvedModel`、`Usage`、`ProviderPort`、错误分类、deadline、fallback policy、`CostMeterPort`；调用事实 repository 端口；事件 `gp.ai.invocation.v1`。

**必须实现**：request_id/biz context 校验；primary 成功；可重试错误才 fallback；不可重试 4xx 不 fallback；attempt 进入幂等键；无 usage 按已决策策略处理；prompt/key 不落日志。

**真实 provider**：D-01 锁定后实现 LiteLLM/指定 provider adapter；HTTP timeout、429/5xx、响应格式异常、健康检查和受控 secret resolver。

**门禁**：fake provider 单测先通过；真实 provider smoke、成本账本一对一、重复请求不双计后才标生产接入完成。

### GP3-02A：W1 生成批次骨架（依赖 GP3-01A、GP2-02）

**迁移**：`gen_batch`、`gen_source_snapshot`、`gen_seed`、`gen_review`、可选 `gen_step`；唯一 `(team_id,idempotency_key)` 和 `(batch_id,dedupe_hash)`。

**状态机**：`enqueued → fetching → parsing → generating → quality_check → review → committing → approved|rejected|rolled_back|failed`。

**Workflow/Activity**：FetchSource、ParseAndNormalize、GenerateSeeds、QualityGate、WaitReview、CommitVersions、EmitAudit；Workflow 不访问 DB/HTTP；Signal 固定 `generation-review`、`generation-rollback`。

**API**：`POST/GET /generation/batches`、`GET /generation/batches/{id}/seeds`、`POST /generation/batches/{id}/review`、`POST /generation/batches/{id}/rollback`；审核带 `expected_state_version`，过期返回 409。

**门禁**：重复创建返回同 batch；拒绝不写 `cas_version`；批准只通过 governance command 写版本；回退新增 rollback 版本；每次状态迁移有审计。

### GP3-03A：成本趋势投影（依赖 GP3-01/02 成本事件、D-04）

**迁移**：`ts_cost_fact`、`ts_cost_projection_checkpoint`；`cost_line_item` 仍是唯一权威账本。

**代码产物**：投影 consumer、事件去重、失败重放、hour/day trend query、reconciliation query；Timescale 不可用时只允许显式普通 PG 降级模式。

**API**：`GET /cost/trend`、受限 `GET /cost/reconciliation`；响应包含 bucket、口径、projection_at、差异。

**门禁**：重复事件不重复投影；回放后金额与权威明细一致；跨 team 隔离；连续聚合刷新延迟符合 D-04。

### GP3-04A：审计锚定与 verifier（依赖 GP3-07、D-05）

**代码产物**：规范化 payload 编码、并发串链/唯一性修复、`aud_anchor`、anchor writer port、独立 `cmd/audit-verify`、验证 API/CLI。

**门禁**：篡改任意 hash/prev/payload 可检出；验证器不使用 writer 权限；外部载体不可变性和恢复演练通过后才算锚定完成。

### GP3-06A：篡改分析/变异最小闭环（依赖 D-06、GP2 runner）

**代码产物**：规则版本、mutation candidate、mutation result、quality event、证据 hash、门禁输入适配器；首批只实现 D-06 确定的一种语言/框架。

**门禁**：固定样例能检出已知弱化；误报/跳过有明确状态；runner 失败与分析失败区分；不把“静态规则命中”写成真实变异分数。

### GP3-07B：安全专项（依赖 D-03/D-05/D-06、安全基线）

**交付物**：备份恢复脚本/演练记录、secret rotation runbook、沙箱出网策略、依赖漏洞与镜像扫描、审计/成本跨域写拒绝测试。

**完成门禁**：RPO/RTO、密钥轮换、备份恢复、沙箱逃逸和跨域 DB 权限测试均有证据；否则保持“安全骨架完成”。

## 4. GP4 执行批次

### GP4-03A：Principal 与认证端口（依赖 D-07，可先做）

**domain**：`Principal`、身份声明、会话状态、认证结果不变量；不依赖 JWT/OIDC/微信 SDK。

**application/api**：认证 middleware 只产生已验证 Principal；`x-gp-team-id` 不再作为生产身份来源；开发 fake provider 仅限 dev/test 配置。

**门禁**：伪造 team/user header 不能越权；过期/错误 issuer/audience/token 拒绝；日志无 token/claim 原文。

### GP4-01A：资产级 RBAC（依赖 GP4-03A、D-07）

**迁移**：`iam_member`、`iam_asset_grant`、`iam_role`/权限字典；核心字段普通列，不把权限判断藏在 JSONB。

**domain/application**：角色 `owner/admin/tester/viewer`，权限 `full/edit/exec/view/none`；继承、覆盖、拒绝优先规则必须按 D-07 固定。

**代码接入顺序**：先 `AuthorizationPort` 和 use-case tests，再接 targets/cases/runs/gates/cost/reports；最后移除仅靠 header 的 bypass。

**门禁**：矩阵测试覆盖资产继承、跨 team、无权限、viewer 写操作、exec 非 edit；每次拒绝写 trusted 审计。

### GP4-02A：CI connector/outbox（依赖 D-08，可先做）

**迁移**：`cicd_connection`、`cicd_inbox`、`cicd_delivery`、`cicd_outbox`；provider secret 只存引用，入站 payload 存 hash/最小摘要。

**代码产物**：通用 connector port、签名校验 port、inbox 幂等、outbox 投递/重试/退避、门禁状态回写、CLI 骨架。

**事件**：`cicd.run.requested.v1`、`cicd.gate.result.v1`；所有投递以 delivery key 幂等。

**门禁**：重复 webhook 只创建一个 run；失败重试不重复回写；未选 provider 时只跑 fake contract test，不能宣称 GitLab/Jenkins/GitHub 完成。

### GP4-04A：私有化交付骨架（依赖 D-09，可先做）

**交付物**：`deploy/helm/gp` Chart、values schema、server/worker、migration Job、readiness/version、Secret 引用、NetworkPolicy、非 root 默认、安装/升级/回滚 runbook。当前可先交付 `/readyz`（仅 PostgreSQL 探针）和 `/version`；Redis/Temporal/对象存储探针必须在对应客户端接入后逐项增加，不能以单一 DB 探针代表全部依赖。

**代码产物**：`cmd/migrate` 或受控 migration Job、`cmd/audit-verify`、build metadata、前端 embed/产物校验。

**门禁**：干净 kind 安装、迁移、ready、最小闭环、N→N+1 升级和失败退出通过；未确定依赖拓扑时只交付 chart 骨架。

### GP4-05A：TenantRoute/Placement（依赖 D-10，可先做）

**domain**：Placement 状态机 `shared → preparing → migrating → verifying → cutover → stable|rollback`；fail-closed。

**迁移/代码**：placement registry、resolver port、dry-run、row count/hash/审计连续性校验；不执行真实客户 cutover。

**门禁**：客户端不能指定 schema/endpoint；错误 route 拒绝请求；迁移失败可回滚；真实 shared→schema/database 方案需 D-10 后验证。

### GP4-06A：观测与压测骨架（依赖 D-11，可先做）

**代码产物**：有限标签的 HTTP/run/queue/worker/pool/依赖指标；trace 关联；PrometheusRule、dashboard、runbook 模板；版本化 load harness。

**禁止**：指标标签不能包含 team/user/run/case/target ID；不能在未定 SLO 时写硬编码“达标”阈值。

**门禁**：指标单测、敏感字段过滤、故障注入告警演练、固定 load profile 可复现；真实容量结论等待 D-11 和 GP2 共享基础设施。

**当前代码状态（2026-10-06）**：已接入进程内 Prometheus exposition 骨架：`GET /metrics` 输出 HTTP 请求计数与时延的 count/sum，标签严格限制为 `method`、静态 `route` 与 `status` class；原始 URL、team/user/run/case/target ID 均不会进入指标。实现包含低基数与动态路径拒绝测试。该 recorder 是进程内、非持久化适配器，尚未接入 Prometheus/OTel、run/queue/worker/pool/依赖事件、告警规则、dashboard、load harness 或任何 SLO 阈值；因此不构成可用性或容量达标结论。

## 5. 推荐执行顺序与并行关系

```text
GP3-07A ─┬─> GP3-05A ─> GP3-01A ─> GP3-02A
         ├─> GP3-04A
         └─> GP3-03A
GP2-02 ────────────────────────────────┘
GP3-06A 依赖 D-06 + GP2-01 runner

GP4-03A ─> GP4-01A
GP4-02A ───────────────────────────────┐
GP4-04A ───────────────────────────────┼─> 真实 GP4 集成
GP4-05A ─> 依赖 GP2-03/07/08 真实共享实现 ┘
GP4-06A 依赖 D-11 + 稳定候选架构
```

### 5.1 立即可排期的第一批

1. GP3-07A：可信写权限测试骨架和安全日志过滤。
2. GP3-05A：模型/绑定/价格快照 domain、迁移、fake API。
3. GP4-03A：Principal/认证 port、fake provider、安全测试。
4. GP4-01A：RBAC domain、迁移、AuthorizationPort 和矩阵测试。
5. GP4-04A：Helm/chart/readiness/version/migration skeleton。
6. GP4-06A：指标字典和 load harness skeleton。

### 5.2 必须等待决策/环境的批次

- GP3-01 真实 provider、GP3-03 Timescale 生产投影、GP3-04 外部锚定、GP3-06 真实变异执行。
- GP4-02 真实 CI provider、GP4-03 真实微信/SSO/SMS、GP4-04 客户拓扑安装、GP4-05 真实租户迁移、GP4-06 容量达标。

## 6. 单批完成定义

一个批次只有同时满足以下条件才能标记“代码完成”：

- 契约、迁移、分层代码和测试全部提交；
- 所有适用固定门禁命令通过；
- 对外接口已生成类型并有 DTO；
- 租户、权限、幂等、时间、错误边界有测试；
- 文档明确列出未完成的真实依赖；
- 不使用 fake/本地实现冒充跨进程、真实 provider、真实集群或性能达标。

阶段只有在真实依赖集成、故障/安全/容量验收证据齐全后，才能标记 GP3/GP4 DoD 完成。

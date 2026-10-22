# 工程规范合规审查（2026-10-22）

## 0. 修订记录（2026-10-22）

审查后已按规范更新方案：

- `def_`/`cov_` 已登记到 `ENGINEERING-SPEC §8.2`；
- `cmd/dsh-gateway` 已登记到 topology/服务目录；
- `open-green-pass-dsh-gateway` 镜像已登记 §12；
- ADR-001 已记录 Go gateway + DSH Node 外部运行时决策；
- 数据所有权、RLS/`gp_trusted_writer`、迁移编号、OpenAPI/测试/沙箱约束已写入方案与规格；
- 原型色彩已按 DESIGN-SPEC 修正并发布 `0.1.95-SNAPSHOT`。

结论更新：**设计/登记层面已合规；实施层仍需按迁移、OpenAPI、RLS 授权、部署脚本和 egress 验证逐项落地。**

## 结论

**方案方向可行；审查时实施层不严格，现已完成设计与登记修正。**

主要问题不是架构思路，而是实施层缺少：领域归属、表前缀登记、RLS/可信写权限、迁移编号、OpenAPI 契约、测试分层、部署入口与镜像登记。以下 P0 项必须在 M0 编码前修正。

## 合规矩阵

| # | 规范 | 状态 | 说明 |
|---|---|---|---|
| 1 | 新增服务/域先定义限界上下文、聚合、权威表、跨域契约 | ✅ 设计已补 | `def_`/`cov_` 归属、gateway 部署单元已登记；实现待做 |
| 2 | 表前缀登记 | ✅ 已登记 | `def_`/`cov_` 已登记；`evd_` 复用；映射见方案 0.3 |
| 3 | 多租户 RLS + 可信域独立写角色 | ⚠️ 部分 | 方案提到 team_id，但未写 RLS 策略、`gp_trusted_writer`、可信表边界 |
| 4 | 迁移只增不改、编号、migcheck | ⚠️ 部分 | 有 DDL 草案，无迁移编号、up/down、migcheck/testcontainers |
| 5 | OpenAPI 唯一契约、生成类型 | ⚠️ 部分 | `/findings`、`/coverage` 未写入 `api/openapi.yaml`；原型为本地数据 |
| 6 | 测试分层（domain/application/infra/api/e2e） | ❌ 不合规 | 未定义 finding 状态机、覆盖算法、网关 stdio、RLS 的测试 |
| 7 | 发布入口唯一、镜像命名、kind 校验 | ⚠️ 镜像已登记，部署待接 | `open-green-pass-dsh-gateway` 已登记 §12；脚本/manifest/CI 待实现 |
| 8 | Runner 沙箱/网络安全基线 | ⚠️ 部分 | 提到非 root/只读，但缺 token automount、seccomp、cap drop、egress 验证 |
| 9 | 可观测性/成本单点计量 | ⚠️ 部分 | 有指标草案，缺 OTel 关联、告警、成本归集 |
| 10 | DESIGN-SPEC 色彩 | ✅ 已修 | sky/violet 已改为 emerald/amber/red/slate，0.1.95 发布 |

## P0 必须修正

### 1. 领域归属与服务边界

建议：

| 能力 | 归属 | 权威数据 |
|---|---|---|
| 缺陷生命周期 | `internal/governance` 新增 Defect 聚合 | `def_finding`、`def_event`、`def_suggestion` |
| 证据元数据 | `internal/trusted` | `evd_evidence`（`evd_` 已登记） |
| 覆盖账本 | `internal/governance`（测试覆盖） | `cov_cell`、`cov_waiver` |
| 生成 job | `internal/ai` | `gen_job`、`gen_job_event` |
| 契约盘点 | `internal/governance` | `ctr_endpoint`、`ctr_parameter` 等 |

- 跨域只经 OpenAPI/端口，不直连对方表。
- `evd_evidence` 在 Trusted 域，写权限归 `gp_trusted_writer`；Governance 通过 Trusted API/端口创建证据，不直接写。
- 缺陷状态流转写 `aud_event`，由 Trusted 域统一写哈希链。

### 2. 表前缀

| 逻辑名 | 物理表 | 前缀状态 |
|---|---|---|
| finding | `def_finding` | **新前缀 `def_`，需架构登记** |
| finding_event | `def_event` | 同上 |
| finding_suggestion | `def_suggestion` | 同上 |
| finding_evidence | `evd_evidence` | 已登记 `evd_` |
| coverage_cell | `cov_cell` | **新前缀 `cov_`，需架构登记** |
| coverage_waiver | `cov_waiver` | 同上 |
| generation_job | `gen_job` | 已登记 `gen_` |
| contract_endpoint | `ctr_endpoint` | 已登记 `ctr_` |

在 `ENGINEERING-SPEC §8.2` 登记 `def_`、`cov_` 之前，不得创建迁移。

### 3. RLS / 可信写 / 审计

每张表必须：

- 带 `team_id`，启用 RLS 策略；
- `def_*`/`cov_*` 归 Governance，`gp_server`/`gp_worker` 可写；
- `evd_evidence` 归 Trusted，`gp_trusted_writer` 写，server/worker 只读；
- 缺陷创建/确认/关闭、证据上传/读取、覆盖 waiver 审批写 `aud_event`；
- 不允许 `UPDATE/DELETE` 证据对象；元数据不可变，纠错用新版本/新记录。

### 4. 迁移

- 新增 `migrations/0000NN_defect_finding.up/down.sql`、`0000NN_coverage_cell.up/down.sql`、`0000NN_evidence.up/down.sql`；
- 只增不改；`down` 可回滚；
- `migcheck` + testcontainers PG；
- 不运行时 AutoMigrate；不在 server main 建表。

### 5. OpenAPI

必须补：

```text
GET/POST /findings
GET/PATCH /findings/{id}
POST /findings/{id}/transition
POST /findings/{id}/suggestions
POST /findings/{id}/evidence:presign
POST /findings/{id}/evidence
GET /findings/{id}/evidence/{eid}
GET /coverage/cells
GET /coverage/summary
POST /coverage/waivers
```

- 更新 `api/openapi.yaml`，运行 `npm run api:generate`，前端只使用生成类型。
- 错误 envelope、分页、过滤、幂等键、权限错误写进契约。
- `/internal/dsh/*` 只走集群内 API，不暴露 ingress，不写入公开 OpenAPI。

### 6. 测试

最低要求：

| 层 | 测试 |
|---|---|
| domain | finding 状态机、严重级别、dedupe、Cell ID/枚举/去重、覆盖公式 |
| application | 创建/确认/关闭 finding、waiver、生成 job 编排、事务/幂等 |
| infra | PG RLS、迁移、证据元数据、MinIO/SeaweedFS、testcontainers |
| api | OpenAPI 契约、权限、错误码、分页、签名 URL |
| dsh gateway | fake stdio JSON-RPC：initialize/prompt/wait/cancel/crash/resume |
| e2e | 一个模块：Cell → 生成 → 执行 → finding → evidence → gate |
| security | 跨租户读写拒绝、源码写尝试、token 脱敏、证据越权 |

全部 `go test -race`；`make check` 必须通过。

## P1 修正

### 7. 部署/镜像

- 新镜像命名需登记：建议 `ghcr.io/openware-io/open-green-pass-dsh-gateway:X.Y.Z-SNAPSHOT`。
- 更新 `ENGINEERING-SPEC §12`、`deploy/k8s/`、`scripts/dev/init-kind.ps1`、`deploy-kind.ps1`、CI、发布记录模板。
- 多阶段构建：Go gateway 静态二进制 + 固定 Node/DSH 版本；非 root；只读根；SBOM/vuln scan。
- `gp-dsh-gateway` 只内网 Service，不暴露 ingress；readiness/liveness。
- 发布记录必须包含 gateway commit/image digest，和 IM 不变更核对。

### 8. 沙箱/网络安全

- 专用 ServiceAccount，`automountServiceAccountToken: false`。
- 非 root、`RuntimeDefault` seccomp、drop ALL caps、allowPrivilegeEscalation false、只读根。
- DSH 进程按 runner 对待，不跑在 server/worker 容器。
- egress：先 staging 验证 default-deny + 明确 allowlist（模型 API、对象存储、GP API），验证通过再启用；不要声称已隔离。
- `DEEPSEEK_API_KEY` 用 Secret 注入；记录为 DSH 运行时例外，禁止日志/回显。

### 9. 可观测性/成本

- Temporal → gateway → DSH 传播 trace_id；gateway 记录 job_id/session_id/worker_id。
- 指标低基数：state/queue/worker/error class；不要 job_id/team_id 指标标签。
- 成本由 AI 网关单点计量；DSH token usage 回写 `cost_line_item`，不散落。
- 告警：队列积压、worker crash、超时率、证据上传失败。

### 10. 设计规范

- 已修：原型 `sky`/`violet` 改为 emerald/amber/red/slate。
- 后续新增页面必须使用 DESIGN-SPEC 状态色，不引入新色系。

## 已符合/已修正

- 源码只读、无 PR/commit/写源码入口：符合产品边界。
- 证据截图必需、视频建议、SHA-256/私有访问：方向符合安全基线。
- 一 job 一进程、进程组终止、workspace 销毁：符合隔离要求。
- 原型色彩已按 DESIGN-SPEC 修正。

## 待你确认的 4 个规范例外

1. 新增 `def_`、`cov_` 表前缀（需登记到 ENGINEERING-SPEC §8.2）。
2. 新增 `open-green-pass-dsh-gateway` 镜像名（需登记到 §12）。
3. `gp-dsh-gateway` 作为 Go 部署单元 + DSH Node 子进程的外部运行时例外（需 ADR）。
4. `DEEPSEEK_API_KEY` 通过环境变量注入 DSH 子进程（模型凭据规范的例外，需安全评审）。

## 结论

**当前方案不能直接进入 M0 编码。** 先完成：

1. 登记 `def_`/`cov_` 前缀；
2. 明确 Governance/Trusted/AI 数据所有权与 RLS/可信写；
3. 写迁移编号与 up/down；
4. 补 OpenAPI 与生成类型；
5. 登记 gateway 镜像与部署入口；
6. 补测试分层与沙箱/网络基线。

这 6 项完成后，方案才算“严格遵守工程规范”，再开始 `finding`/`evidence` 后端和 DSH SDK 网关实现。

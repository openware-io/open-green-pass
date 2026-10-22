# GP AI 测试引擎（DSH SDK 网关）可执行方案

## 0. 结论

- **DSH 官方支持自动化运行**：`dsh --profile headless "job"`、`dsh --profile sdk`（JSON-RPC stdio）、`dsh --profile acp`（stdio）。
- **DSH 没有官方的多租户 HTTP 服务**；`dsh web` 是交互式 Web UI，不适合作为 GP 的生产后端。
- **核心采用 `dsh --profile sdk` 长驻进程 + `gp-dsh-gateway` 进程池/队列**；headless 只用于批处理、CI 和降级，不作为主链路。
- SDK 一个进程可承载多个 SDK session，但 GP 仍需自己做 HTTP/队列、进程池、workspace/session 隔离、超时和崩溃恢复。
- 不在 DSH 之上重做平台；在 GreenPass 内新增 **AI 测试引擎子系统**，DSH 只做规划、生成、分析和建议。

## 1. 边界与原则

1. 产品源码只读：GP 不写仓库、不建 PR、不提交 commit、不修改源码。
2. GP 负责：覆盖账本、执行调度、证据、缺陷、建议、门禁、审计。
3. DSH 负责：覆盖规划、用例生成、失败分析、修复建议；**不拥有执行和门禁**。
4. 测试资产（用例、测试脚本、测试数据）可以写，属于 GP 自己的数据。
5. 不允许“改断言直到通过”；产品缺陷必须进入缺陷流程。
6. 所有 AI 输出必须经过 Schema/规则校验，不能直接入库或直接执行。

## 2. DSH 官方能力与部署选择

| 模式 | 官方用途 | 通信方式 | 在 GP 中的定位 |
|---|---|---|---|
| `dsh --profile sdk` | 长驻 JSON-RPC 运行时，按 `sessionId` 开 session | stdio | **核心：gp-dsh-gateway 的进程池** |
| `dsh --profile headless "job"` | 一次任务，输出最终答案后退出 | stdout | 批处理 / CI / 降级，不作为主链路 |
| `dsh --profile acp` | 自动化客户端 | ACP over stdio | 编辑器/外部 Agent 集成 |
| `dsh web` | Web UI | HTTP/WebSocket | 人工交互，不作为后端 |

### 为什么 SDK 更合适

Headless 的本质确实是“一次性调用”：

- 每次冷启动，模型连接、工具、上下文缓存都要重建；
- 一次任务结束进程退出，不能复用 session；
- 多步流程（规划 → 生成 → 校验 → 修复）要么塞进一个超长 prompt，要么反复重启；
- 对流式进度、取消、会话恢复不友好。

SDK 模式：

- 一个长驻进程可承载多个 `sessionId`；
- `initialize` 握手一次，之后 `session/prompt`、`session/wait`、`session/event` 持续交互；
- 支持 `session/resume` 恢复已持久化 session；
- 适合 GP 的模块级多步生成、失败分析、修复建议；
- 但传输是 stdio，GP 必须自己做 HTTP/队列网关、进程池、隔离和超时。

### 采用 SDK 网关

```text
GP Orchestrator / Temporal
  -> gp-dsh-gateway (HTTP/内部 API)
  -> job queue (Temporal + gateway 内部有界队列)
  -> worker pool: 每个 worker 一个 dsh --profile gp-test-engine 子进程
  -> JSON-RPC initialize/session/prompt/wait
  -> 结果 + transcript/tool calls -> GP 入库
```

Headless 保留为：

- CI 离线批处理；
- 网关不可用时的降级；
- 单次快速验证/对比实验。

## 3. 目标架构

```text
GreenPass Web / API
        |
        v
AI Test Engine Orchestrator (Go, Temporal activities)
  |-- Contract Inventory Service
  |-- Coverage Planner (deterministic + DSH Planner)
  |-- Case Generator (DSH Generator + static validator)
  |-- gp-dsh-gateway (HTTP/queue, worker pool)
  |     |-- worker 1: dsh --profile gp-test-engine (stdio JSON-RPC)
  |     |-- worker 2: dsh --profile gp-test-engine
  |     `-- worker N: ...
  |-- Execution Orchestrator (existing worker + runners)
  |-- Failure Triage (rules + DSH Analyst)
  |-- Findings / Suggestions Service
  |-- Coverage Gate / Report / Audit
        |
        v
PostgreSQL (coverage ledger, cases, findings, results)
```

## 4. 数据模型

### 4.1 契约与覆盖

```text
ai_contract_endpoint
  id, team_id, project_id, service_id, method, path,
  operation_id, auth_mode, source_file, source_line,
  request_schema, response_schema, status_codes, version

ai_contract_parameter
  id, endpoint_id, location(path/query/header/body),
  name, type, required, enum_values, min, max, pattern, source

ai_contract_error
  id, endpoint_id, http_status, code, message, source

ai_contract_role
  id, endpoint_id, role, tenant_scope, allowed, source

ai_contract_state
  id, endpoint_id, from_state, to_state, allowed, source

ai_coverage_cell
  id, team_id, project_id, endpoint_id, dimension,
  equivalence_class, role, state, priority, required,
  status(planned/generated/executed/passed/failed/blocked/waived),
  case_id, evidence, waiver_reason

ai_generation_job
  id, team_id, project_id, service_id, module,
  state(queued/running/succeeded/failed/timeout),
  dsh_run_id, input_ref, output_ref, token_usage,
  started_at, finished_at, error
```

### 4.2 用例与执行

```text
cas_case
  + coverage_cell_ids jsonb
  + oracle_source text
  + source_snapshot_sha text

run_case_result
  + failure_class(product/case/environment/data/expected_change)
  + finding_id
  + evidence_ref
```

### 4.3 缺陷与建议

```text
finding
  id, team_id, project_id, service_id, endpoint_id,
  repository_id, repo_url, branch, commit_sha,
  title, description, severity(P0/P1/P2/P3), category,
  status(open/triaged/confirmed/in_progress/pending_regression/closed/false_positive),
  expected, actual, root_cause, impact,
  reproduction jsonb, evidence_ref, source_ref,
  owner_id, linked_run_id, linked_case_id, dedupe_hash,
  created_by, created_at, updated_at

finding_evidence
  id, finding_id, kind(request/response/log/trace/screenshot/video),
  storage_uri, mime_type, width, height, duration_ms,
  sha256, redacted, created_at

finding_evidence_policy
  - 截图：UI/实机用例必需（P0/P1 必须 100%）
  - 视频：关键流程/复杂交互建议提供
  - 服务端保存 MIME、尺寸、时长、SHA-256；证据只读不可篡改
  - 证据与 finding/run/case/commit 绑定，支持大图/视频播放与下载审计

finding_suggestion
  id, finding_id, summary, approach, affected_files jsonb,
  confidence, source, status(draft/accepted/rejected)

finding_event
  id, finding_id, action, actor_id, payload, created_at
```

## 5. API

### 5.1 契约与覆盖

```text
POST /ai/contracts:ingest          # 触发契约盘点
GET  /ai/contracts                 # 端点/参数/错误/角色/状态
POST /ai/coverage:plan             # 生成 coverage cells
GET  /ai/coverage/cells            # 覆盖账本
POST /ai/coverage/waivers          # 豁免
```

### 5.2 生成

```text
POST /ai/generation/jobs           # 创建模块级生成任务
GET  /ai/generation/jobs/{id}      # 状态/结果
POST /ai/generation/jobs/{id}:retry
GET  /ai/generation/jobs/{id}/cases
```

### 5.3 缺陷

```text
GET  /findings
POST /findings
GET  /findings/{id}
PATCH /findings/{id}
POST /findings/{id}/transition
POST /findings/{id}/suggestions
GET  /findings/{id}/suggestions
POST /findings/{id}/evidence:presign   # 获取截图/视频上传地址
POST /findings/{id}/evidence           # 登记证据元数据（MIME/尺寸/时长/SHA-256）
GET  /findings/{id}/evidence/{eid}     # 受控读取/播放，审计
POST /runs/{id}/triage
POST /runs/{id}/findings
```

所有接口使用团队 RLS、角色授权和审计；源码引用只返回 `file:line`，不返回仓库写权限。

## 6. DSH SDK 网关设计

### 6.1 组件

```text
gp-dsh-gateway (Go, Deployment, 2+ replicas)
  |-- job queue + idempotency + state machine
  |-- worker pool (per workspace/tenant)
  |     |-- dsh --profile gp-test-engine (child process, stdio JSON-RPC)
  |     `-- one SDK process can host multiple sessionId
  |-- session/event ingestion -> transcript/tool calls
  `-- result validator -> GP API/Temporal
```

- **网关**：HTTP/内部 API、队列、进程池、超时、重试、取消、事件采集。
- **SDK 进程**：`dsh --profile gp-test-engine`，stdio JSON-RPC；一个进程可承载多个 session，但默认一个 job/workspace 一个进程，隔离优先。
- **GP orchestrator**：Temporal workflow/activity 调网关，不直接管 DSH 进程。

### 6.2 镜像与 Profile

```text
ghcr.io/openware-io/open-green-pass-dsh-gateway:<version>
  base: node:22-slim
  install: @deepseek-ai/dsh@<pinned>
  profile: /opt/gp/dsh-profile/gp-test-engine
  user: nonroot
  entrypoint: /opt/gp/gateway/gateway
```

`gp-test-engine` profile：

- 继承 `dsh-base` + `dsh-sdk-app`，保留 SDK JSON-RPC server；
- 只启用读文件、grep、目录树、读 OpenAPI/路由/日志；
- 禁用 `write`、`edit`、`bash`/`pwsh` 写操作、git commit/push、部署命令；
- 禁止非模型网络外联；
- `DSH_HOME=/tmp/dsh/<worker>`，不挂载用户 home，不共享 session 状态；
- 不启用 Web、插件市场、自动更新。

> 不要用 `sdk-minimal` 作为生产 profile：它带 persistent shell 和 danger-full-access，不适合只读源码场景。

### 6.3 网关 API 与队列

```text
POST /internal/dsh/jobs
  { job_id, team_id, module, prompt_ref, workspace_ref, model_route, timeout_ms }
GET  /internal/dsh/jobs/{job_id}
GET  /internal/dsh/jobs/{job_id}/events
DEL  /internal/dsh/jobs/{job_id}        # 取消
```

状态机：

```text
queued -> starting -> running -> succeeded
                           |-> failed
                           |-> timeout
                           |-> cancelled
```

- `job_id` 幂等；重复提交返回同一 job。
- 队列有界，按团队/模块限流；队列满返回 `queue_full`。
- Temporal 负责跨步骤编排和重试；网关内部队列负责进程池调度。
- 事件流保存 `session.event`（tool call、token、状态），用于审计和前端进度。

### 6.4 SDK 会话流程

```text
1. gateway spawn dsh --profile gp-test-engine (stdio)
2. send initialize { provider, model, cwd, maxTokens }
3. send session/prompt { sessionId, content: prompt }
4. stream session/event notifications
5. send session/wait { sessionId }
6. parse final assistant message -> JSON Schema
7. persist transcript/result; kill or reuse session
```

- `session/resume` 用于恢复被中断的持久化 session。
- SDK 协议没有原生 cancel 方法；取消通过网关超时/终止进程/释放 session 实现，并在 GP 记录 `cancelled`。
- 一个 job 的多步流程（规划 → 生成 → 校验 → 修复）复用同一 `sessionId`，保持上下文。

### 6.5 Worker 池与隔离

- 一个 worker = 一个 `dsh` 子进程 + 一个 workspace + 一个 `DSH_HOME`。
- 默认一个 job 一个 worker；高并发时按团队/模块分池。
- 源码 workspace 只读挂载；测试脚本/输出在独立读写目录。
- 后端进程崩溃：网关捕获退出码，标记失败，按策略重启；session 可从持久化恢复。
- 每个 worker 设置 CPU/内存/时长上限；空闲回收。

### 6.6 K8s 部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gp-dsh-gateway
spec:
  replicas: 2
  template:
    spec:
      serviceAccountName: gp-dsh-gateway
      containers:
        - name: gateway
          image: ghcr.io/openware-io/open-green-pass-dsh-gateway:<version>
          env:
            - name: DSH_PROFILE
              value: gp-test-engine
            - name: DSH_MAX_WORKERS
              value: "4"
            - name: DEEPSEEK_API_KEY
              valueFrom:
                secretKeyRef: { name: gp-dsh-deepseek, key: token }
          resources:
            requests: { cpu: "1", memory: "2Gi" }
            limits: { cpu: "4", memory: "8Gi" }
          volumeMounts:
            - { name: repo-cache, mountPath: /workspace/repos, readOnly: true }
            - { name: work, mountPath: /tmp }
      volumes:
        - name: repo-cache
          persistentVolumeClaim: { claimName: gp-dsh-repos }
        - name: work
          emptyDir: {}
```

- 网关用 PVC/对象存储缓存只读 checkout；每个 job 的 workspace 在 `/tmp` 下独立。
- 网络策略只允许模型 API 和 GP API。
- 不用 `dsh web`，不暴露 3080。

### 6.7 Headless 兜底

保留一个 `gp-dsh-headless` Job 仅用于：

- CI 离线批处理；
- 网关不可用时的降级；
- 单次快速验证/对照实验。

它是“一次性调用”，不作为主链路；结果协议与 SDK 模式一致。

### 6.8 结果协议

DSH 最终输出必须符合 JSON Schema：

```json
{
  "job_id": "...",
  "module": "...",
  "coverage_cells": ["..."],
  "cases": [],
  "findings": [],
  "gaps": [],
  "risks": []
}
```

网关校验失败标记 `invalid_output`，不入库；原始 transcript 保留在对象存储用于审计。

## 7. 覆盖方法

### 7.1 100% 的定义

只对**范围内**的覆盖模型承诺 100%：

- 每个端点：happy、authn、authz、validation、documented errors；
- 每个参数：必填缺失、类型/格式、边界、枚举；
- 每个文档化状态码/错误码：至少一条；
- 每个角色/租户：正例/反例；
- 每个状态迁移：允许/禁止；
- 每个写操作：幂等/重复/冲突；
- 每个列表：分页/空结果/非法过滤。

不承诺无限输入空间 100%。

### 7.2 Cell 生成

```text
for endpoint in inventory:
  for dimension in required_dimensions(endpoint):
    for equivalence_class in classes(endpoint, dimension):
      cell = new CoverageCell(...)
```

多参数组合用 pairwise；不可达组合记录 waiver。

### 7.3 生成顺序

1. 确定性抽取 OpenAPI/路由/controller/DTO/错误码；
2. 规则枚举 Cell；
3. DSH Planner 补充语义场景（业务状态、权限、数据一致性）；
4. Pairwise 去重；
5. 按模块分批生成用例；
6. 静态校验；
7. 执行；
8. 更新账本；
9. 门禁。

## 8. 生成与校验

### 8.1 输入

每个 Job 只输入：

- 一个模块的源码/路由/DTO；
- 该模块的 coverage cells；
- 已有用例（避免重复）；
- 错误码/角色/状态契约；
- 输出 Schema。

### 8.2 输出

每条用例必须声明：

```json
{
  "case": {...},
  "coverage_cell_ids": ["..."],
  "source_refs": [{"file": "...", "line": 1}],
  "oracle": "contract|observed|inferred",
  "confidence": "high|medium|low"
}
```

### 8.3 静态校验

- method/path 在 inventory 中存在；
- 参数属于该端点；
- 断言包含 status + 业务字段/错误码；
- source_refs 在冻结快照中；
- coverage_cell_ids 都属于本模块；
- 无重复 dedupe_hash；
- 禁止输出源码 patch、git 命令、写文件指令。

## 9. 执行、归因与建议

### 9.1 执行

- 源码只读挂载；
- 测试脚本/数据来自 GP 对象存储；
- 每批用例独立 fixture/租户；
- 采集请求/响应（脱敏）、日志、Trace、环境版本、冻结 SHA；
- 超时、重试、阻塞原因显式记录。

### 9.2 归因

```text
product       -> 创建 finding，进入缺陷流程
case          -> DSH Critic 建议修复用例，重跑（有次数上限）
environment   -> 修复环境/fixture，不改用例
data          -> 修复测试数据
expected_change -> 人工确认后更新 oracle
```

禁止自动改断言让测试通过。

### 9.3 建议

DSH Analyst 输出：

- 根因假设与证据；
- 影响范围；
- 建议修复方案；
- 建议修改点（文件:行，文本建议）；
- 回归用例建议；
- 置信度。

建议只展示/导出，**不提供“应用/提交”按钮**。

## 10. UI 调整

| 页面 | 调整 |
|---|---|
| 导航 | 新增“缺陷中心”，显示未闭环 P0/P1 |
| 测试执行 | 结果 + 失败归因 + 证据 + “创建缺陷” |
| 缺陷中心 | 列表/详情/流转/去重/负责人/建议/导出 |
| 用例生成 | 增加覆盖计划、Cell 数、只读源码标识 |
| 用例管理 | 用例声明 coverage cells；测试资产可编辑 |
| 质量门禁 | 缺陷门禁 + 覆盖门禁 |
| 测试报告 | 覆盖、缺陷、根因、建议、风险 |
| 测试历史 | 运行关联缺陷数/回归结果 |
| 被测对象 | 源码只读徽标、SCM 只读范围 |
| 测试追溯 | 需求 → 用例 → 运行 → 缺陷 |

## 11. 安全与只读护栏

- SCM 连接只申请 `Contents: Read`、`Metadata: Read`；拒绝写/管理 scope。
- GP 后端无仓库写 API；无 commit/push/PR 代码路径。
- DSH 工具白名单只读；无 shell 写、无 git 写、无部署权限。
- Job 非 root、只读根文件系统、独立 `DSH_HOME`、网络策略仅模型 API。
- 所有源码读取、AI 调用、缺陷流转写审计。
- 缺陷建议可导出到 Jira/飞书，但不回写源码。

## 12. 交付计划

### M0（1 周）：只读护栏 + 缺陷模型

- SCM scope 校验和只读标识；
- `finding`/`finding_event`/`finding_evidence` 表（含 repository_id/repo_url/branch/commit_sha）；
- 执行结果增加 `failure_class`；
- 缺陷中心列表/详情/流转 UI 骨架；
- 截图证据优先：上传/预览/大图/哈希；视频列为建议项；
- 验收：失败用例可创建缺陷，P0/P1 有截图，源码写权限为 0。

### M1（2 周）：覆盖账本 + SDK 网关 + 单模块生成

- `ai_contract_*`、`ai_coverage_cell`、`ai_generation_job`；
- OpenAPI/路由盘点；规则枚举 Cell；
- `gp-dsh-gateway` Deployment + SDK worker pool + 有界队列；
- `gp-test-engine` profile（只读工具、SDK JSON-RPC）；
- 选择一个模块（如 `user`）完成“规划 → 生成 → 校验 → 修复”多步 session；
- 静态校验 + 入库；
- 验收：该模块 100% 必需 Cell 有生成用例；无源码写操作；同一 session 可连续多轮。

### M2（2 周）：执行闭环 + 失败归因 + 建议

- 执行沙箱、fixtures、证据采集；
- `POST /runs/{id}/triage`；
- DSH Analyst 生成根因/建议；
- 缺陷中心关联运行/用例/Cell；
- 验收：P0/P1 失败可归因并生成建议；误报可标记。

### M3（2 周）：并行模块 + 覆盖门禁 + 报告

- gateway worker pool 按 service/module 并行；
- 覆盖账本门禁：未覆盖 Cell = 0（含 waiver）；
- 缺陷门禁：未关闭 P0/P1 = 0；
- 报告：覆盖/缺陷/建议/回归；
- 验收：`open-chat-admin` 18 模块全部纳入；端到端可复跑。

### M4（后续）：增量生成 + 多租户隔离

- 按 diff 只生成受影响 Cell；
- 历史缺陷回归推荐；
- 多团队 worker 池/配额/隔离；
- 网关水平扩展；headless 仅保留 CI/降级。

## 13. 验收指标

以 `open-chat-admin` 2.2.1 为基线：

| 指标 | 目标 |
|---|---|
| 端点盘点 | 87/87 |
| 模块覆盖 | 18/18 |
| 必需 Cell 设计覆盖 | 100%（含 waiver） |
| 用例执行率 | ≥ 90% |
| 失败归因率 | 100% 失败有分类 |
| 缺陷证据完整率 | 100% P0/P1 有请求/响应/日志/版本/截图 |
| 截图证据 | UI/实机用例 100% 有截图；关键流程建议视频 |
| 源码写操作 | 0 |
| P0/P1 缺陷建议 | 100% 有根因/建议/回归建议 |

## 14. 风险

| 风险 | 应对 |
|---|---|
| DSH 无原生 HTTP 多租户 | `gp-dsh-gateway` + worker pool + 自建队列；headless 仅兜底 |
| SDK 进程崩溃/会话恢复 | 网关捕获退出码；`session/resume` 恢复持久化 session；自动重试 |
| 无原生 cancel | 网关超时/终止进程/释放 session；GP 标记 cancelled |
| 长驻进程内存/上下文膨胀 | 每 job/workspace 独立进程；空闲回收；限制 session 数/token |
| 模型输出不稳定 | JSON Schema + 静态校验 + Critic；不直接入库 |
| 测试数据污染 | 每批 fixture/租户；清理任务；只读源码 |
| 误报 | 失败归因 + 证据 + 人工确认；禁止自动改 oracle |
| 大项目规模 | 按模块并行；覆盖账本；增量生成 |
| DSH 版本漂移 | 固定版本/profile；镜像锁定；升级走发布流程 |

## 15. 待决策

1. 网关 worker 池是一进程多 session，还是一 session 一进程？（默认后者，隔离优先）
2. 缺陷是否对接 Jira/飞书，还是先内部闭环？
3. 覆盖门槛按项目配置还是全局默认？
4. 外部预签名/对象存储测试环境由谁提供？
5. 建议是否允许展示 diff 预览（只读，不应用）？

## 16. 结论

- DSH 官方支持 SDK（stdio JSON-RPC）/headless/acp，不支持多租户 HTTP 服务。
- 核心采用 **SDK 长驻进程 + `gp-dsh-gateway` + 自建队列/进程池**；headless 只做 CI/降级。
- GP 要补的是“覆盖账本 + 执行证据 + 缺陷模型 + 只读护栏”，不是再做一个 DSH。
- 先做 M0/M1，用单模块证明“覆盖 Cell → SDK 多步 session → 静态校验 → 执行 → 缺陷/建议”的闭环，再并行扩展到全项目。

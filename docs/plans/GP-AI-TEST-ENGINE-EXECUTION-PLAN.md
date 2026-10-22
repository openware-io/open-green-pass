# GP AI 测试引擎（DSH Runner）可执行方案

## 0. 结论

- **DSH 官方支持自动化运行**：`dsh --profile headless "job"`、`dsh --profile sdk`（JSON-RPC stdio）、`dsh --profile acp`（stdio）。
- **DSH 没有官方的多租户 HTTP 服务**；`dsh web` 是交互式 Web UI，不适合作为 GP 的生产后端。
- 一期采用 **headless 每模块一个 K8s Job**；二期再评估 `sdk` 长驻网关。
- 不在 DSH 之上重做平台；在 GreenPass 内新增 **AI 测试引擎子系统**，DSH 只做规划、生成、分析和建议。

## 1. 边界与原则

1. 产品源码只读：GP 不写仓库、不建 PR、不提交 commit、不修改源码。
2. GP 负责：覆盖账本、执行调度、证据、缺陷、建议、门禁、审计。
3. DSH 负责：覆盖规划、用例生成、失败分析、修复建议；**不拥有执行和门禁**。
4. 测试资产（用例、测试脚本、测试数据）可以写，属于 GP 自己的数据。
5. 不允许“改断言直到通过”；产品缺陷必须进入缺陷流程。
6. 所有 AI 输出必须经过 Schema/规则校验，不能直接入库或直接执行。

## 2. DSH 官方能力与部署选择

| 模式 | 官方用途 | 通信方式 | 适用 |
|---|---|---|---|
| `dsh --profile headless "job"` | 一次任务，输出最终答案后退出 | stdout | **一期 DSH Runner Job** |
| `dsh --profile sdk` | SDK 客户端 | JSON-RPC over stdio | 二期长驻网关 |
| `dsh --profile acp` | 自动化客户端 | ACP over stdio | 编辑器/外部 Agent 集成 |
| `dsh web` | Web UI | HTTP/WebSocket | 人工交互，不作为后端 |

### 一期选择：Headless Job

```text
GP Orchestrator
  -> create K8s Job (gp-dsh-runner)
  -> dsh --profile gp-test-engine "prompt"
  -> read output JSON
  -> validate -> persist
```

优点：隔离好、超时/资源可控、无长连接、无多租户状态。
缺点：每次启动有冷启动成本；需要 Job 控制器和结果收集。

### 二期选择：SDK 网关

- 长驻 Pod 运行 `dsh --profile sdk`；
- 外层 `gp-dsh-gateway` 暴露 HTTP/队列；
- 每个 session 独立 `DSH_HOME`/workspace；
- 需要解决并发、会话隔离、超时、崩溃恢复。

## 3. 目标架构

```text
GreenPass Web / API
        |
        v
AI Test Engine Orchestrator (Go, Temporal activities)
  |-- Contract Inventory Service
  |-- Coverage Planner (deterministic + DSH Planner)
  |-- Case Generator (DSH Generator + static validator)
  |-- DSH Runner Controller (K8s Jobs)
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
  title, description, severity(P0/P1/P2/P3), category,
  status(open/triaged/confirmed/in_progress/pending_regression/closed/false_positive),
  expected, actual, root_cause, impact,
  reproduction jsonb, evidence_ref, source_ref,
  owner_id, linked_run_id, linked_case_id, dedupe_hash,
  created_by, created_at, updated_at

finding_evidence
  id, finding_id, kind(request/response/log/trace/screenshot),
  uri, sha256, redacted, created_at

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
POST /runs/{id}/triage
POST /runs/{id}/findings
```

所有接口使用团队 RLS、角色授权和审计；源码引用只返回 `file:line`，不返回仓库写权限。

## 6. DSH Runner 设计

### 6.1 镜像

```text
ghcr.io/openware-io/open-green-pass-dsh-runner:<version>
  base: node:22-slim
  install: @deepseek-ai/dsh@<pinned>
  profile: /opt/gp/dsh-profile/gp-test-engine
  user: nonroot
  entrypoint: /opt/gp/runner/run.sh
```

`run.sh`：

```sh
#!/bin/sh
set -eu
PROMPT="$(cat /workspace/input/prompt.txt)"
exec dsh --profile gp-test-engine "$PROMPT"
```

### 6.2 Profile

`gp-test-engine` 只启用：

- 读文件、grep、目录树；
- 读取 OpenAPI/路由/日志；
- 输出 JSON 到 stdout；
- 禁止：写文件、shell 写操作、git commit/push、网络外联（除模型 API）。

`DSH_HOME` 使用 Job 临时目录；不挂载用户 home；不共享 profile 状态。

### 6.3 K8s Job 规格

`repo` 由 orchestrator 预置的 initContainer 以只读方式 checkout 到 emptyDir，主容器只读挂载；主容器不能写仓库。

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: gp-dsh-<job-id>
spec:
  backoffLimit: 0
  activeDeadlineSeconds: 900
  template:
    spec:
      restartPolicy: Never
      serviceAccountName: gp-dsh-runner
      containers:
        - name: dsh
          image: ghcr.io/openware-io/open-green-pass-dsh-runner:<version>
          env:
            - name: DSH_HOME
              value: /tmp/dsh-home
            - name: DEEPSEEK_API_KEY
              valueFrom:
                secretKeyRef: { name: gp-dsh-deepseek, key: token }
          resources:
            requests: { cpu: "500m", memory: "1Gi" }
            limits: { cpu: "2", memory: "4Gi" }
          volumeMounts:
            - { name: input, mountPath: /workspace/input, readOnly: true }
            - { name: repo, mountPath: /workspace/repo, readOnly: true }
            - { name: work, mountPath: /tmp }
      volumes:
        - name: input
          configMap: { name: gp-dsh-<job-id> }
        - name: repo
          emptyDir: {}
        - name: work
          emptyDir: {}
```

仓库由 initContainer 以只读方式 checkout，主容器只读挂载；测试脚本/输出走 `/workspace/input` 和 stdout。

### 6.4 结果协议

DSH 输出必须符合 JSON Schema：

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

Runner 校验失败直接标记 `invalid_output`，不入库。

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
- `finding`/`finding_event`/`finding_evidence` 表；
- 执行结果增加 `failure_class`；
- 缺陷中心列表/详情/流转 UI 骨架；
- 验收：失败用例可创建缺陷，源码写权限为 0。

### M1（2 周）：覆盖账本 + 单模块 DSH 生成

- `ai_contract_*`、`ai_coverage_cell`、`ai_generation_job`；
- OpenAPI/路由盘点；规则枚举 Cell；
- `gp-dsh-runner` 镜像 + K8s Job + 结果协议；
- 选择一个模块（如 `user`）生成 20-40 条用例；
- 静态校验 + 入库；
- 验收：该模块 100% 必需 Cell 有生成用例；无源码写操作。

### M2（2 周）：执行闭环 + 失败归因 + 建议

- 执行沙箱、fixtures、证据采集；
- `POST /runs/{id}/triage`；
- DSH Analyst 生成根因/建议；
- 缺陷中心关联运行/用例/Cell；
- 验收：P0/P1 失败可归因并生成建议；误报可标记。

### M3（2 周）：并行模块 + 覆盖门禁 + 报告

- 按 service/module 并行 Job；
- 覆盖账本门禁：未覆盖 Cell = 0（含 waiver）；
- 缺陷门禁：未关闭 P0/P1 = 0；
- 报告：覆盖/缺陷/建议/回归；
- 验收：`open-chat-admin` 18 模块全部纳入；端到端可复跑。

### M4（后续）：SDK 长驻 + 增量生成

- `dsh --profile sdk` 网关；
- 按 diff 只生成受影响 Cell；
- 历史缺陷回归推荐。

## 13. 验收指标

以 `open-chat-admin` 2.2.1 为基线：

| 指标 | 目标 |
|---|---|
| 端点盘点 | 87/87 |
| 模块覆盖 | 18/18 |
| 必需 Cell 设计覆盖 | 100%（含 waiver） |
| 用例执行率 | ≥ 90% |
| 失败归因率 | 100% 失败有分类 |
| 缺陷证据完整率 | 100% P0/P1 有请求/响应/日志/版本 |
| 源码写操作 | 0 |
| P0/P1 缺陷建议 | 100% 有根因/建议/回归建议 |

## 14. 风险

| 风险 | 应对 |
|---|---|
| DSH 无原生 HTTP 多租户 | 一期 headless Job；二期 SDK 网关 |
| Job 冷启动/成本 | 模块级批处理；缓存 profile；限制 token |
| 模型输出不稳定 | JSON Schema + 静态校验 + Critic；不直接入库 |
| 测试数据污染 | 每批 fixture/租户；清理任务；只读源码 |
| 误报 | 失败归因 + 证据 + 人工确认；禁止自动改 oracle |
| 大项目规模 | 按模块并行；覆盖账本；增量生成 |
| DSH 版本漂移 | 固定版本/profile；镜像锁定；升级走发布流程 |

## 15. 待决策

1. 一期是否只做 `headless`，还是直接上 `sdk` 网关？
2. 缺陷是否对接 Jira/飞书，还是先内部闭环？
3. 覆盖门槛按项目配置还是全局默认？
4. 外部预签名/对象存储测试环境由谁提供？
5. 建议是否允许展示 diff 预览（只读，不应用）？

## 16. 结论

- DSH 官方支持 headless/sdk/acp 自动化，不支持多租户 HTTP 服务；一期用 headless Job 是正确路径。
- GP 要补的是“覆盖账本 + 执行证据 + 缺陷模型 + 只读护栏”，不是再做一个 DSH。
- 先做 M0/M1，用单模块证明“覆盖 Cell → DSH 生成 → 静态校验 → 执行 → 缺陷/建议”的闭环，再并行扩展到全项目。

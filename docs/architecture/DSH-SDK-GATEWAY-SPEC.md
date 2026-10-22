# DSH SDK 网关规格（GP-DSH-GATEWAY-SPEC）

| 项目 | 值 |
|---|---|
| 状态 | 实施规格 v1（2026-10-22） |
| 范围 | `gp-dsh-gateway`、worker pool、DSH SDK 会话生命周期 |
| 上游 | `@deepseek-ai/dsh`（固定版本） |
| 关联方案 | `docs/plans/GP-AI-TEST-ENGINE-EXECUTION-PLAN.md` |

## 0. 工程合规

- 部署单元：`cmd/dsh-gateway`（Go）；DSH 是外部 Node 运行时子进程，不承载 GP 业务域、不直接访问 GP DB。
- 新镜像名 `ghcr.io/openware-io/open-green-pass-dsh-gateway:X.Y.Z-SNAPSHOT` 需登记到 ENGINEERING-SPEC §12；登记前不得发布。
- 网关只通过 GP API/事件回写，不直连 `gp_` 库；job 状态由 GP AI 域持久化。
- 必须纳入 `init-kind.ps1`/`deploy-kind.ps1`/`deploy/k8s/`/CI；只内网 Service，不暴露 ingress。
- Runner 沙箱：专用 SA、`automountServiceAccountToken: false`、非 root、RuntimeDefault seccomp、drop ALL caps、allowPrivilegeEscalation false、只读根。
- egress 先在 staging 做 default-deny + allowlist 验证，通过后再启用；不声称已隔离。
- `DEEPSEEK_API_KEY` 通过 Secret 注入，记录为运行时例外，禁止日志/回显。

## 1. 目标与非目标

### 目标

- 为 GP 提供稳定的 DSH SDK 调用入口：提交 job、查询状态、流式事件、取消、结果。
- 一个 job 内支持多步：规划 → 生成 → 校验 → 修复，复用同一 `sessionId`。
- 进程、workspace、`DSH_HOME`、凭据严格隔离；源码只读。
- 超时/取消/崩溃后可清理，不污染后续 job。

### 非目标

- 不把 `dsh web` 暴露为后端。
- 不做多租户共享 session。
- 不允许 DSH 写源码、执行 git 写操作或部署。
- 不把 GP 的覆盖账本/缺陷状态机搬进 DSH。

## 2. SDK 协议基线

DSH `--profile sdk` 使用换行分隔的 JSON-RPC 2.0 over stdio。GP 只依赖以下方法/通知：

| 方向 | 方法/通知 | 用途 |
|---|---|---|
| client→server | `initialize` | 选择 provider/model、cwd、maxTokens，完成握手 |
| client→server | `session/prompt` | 投递用户消息，返回 `messageId`（入队回执） |
| client→server | `session/wait` | 等待 session 空闲/完成 |
| client→server | `session/resume` | 从持久化 session 恢复 |
| client→server | `session/working-directory/set` | 设置 job workspace |
| client→server | `shutdown` | 正常关闭 |
| server→client | `session.event` | 持久化事件流（消息、工具调用等） |
| server→client | `session.status` | whole-agent running/idle |
| server→client | `subagent.started` / `subagent.finished` | 子 Agent 生命周期 |

注意：

- `session/prompt` 只返回入队回执，不代表完成；完成以 `session/wait` 或终态事件为准。
- SDK 没有原生 cancel 方法；取消由网关终止进程实现。
- 协议字段以固定版本 `@deepseek-ai/dsh` 的 `dsh-sdk-protocol` 类型为准；本规格中的 JSON 是网关内部契约，不替代上游类型。

## 3. 进程模型

### 决策

**一 job 一进程 + warm pool；同 job 复用 session；不跨 job 复用进程。**

```text
gateway
  ├─ warm worker A (dsh process, idle)
  ├─ warm worker B (dsh process, idle)
  └─ active job 1 -> worker A (session gp-job-1)
       active job 2 -> worker B (session gp-job-2)
```

- `DSH_MAX_WORKERS`：最大并发 worker。
- `DSH_WARM_WORKERS`：预热进程数（建议 2）。
- 一个 worker 同时只服务一个 job；job 结束回收/重置。
- 同一 job 的多步交互复用同一个 `sessionId`，不重启进程。
- 不跨租户、跨仓库、跨 job 复用进程。

### 为什么

- 工具权限、文件系统、上下文、内存、崩溃影响都在进程级隔离。
- SDK 支持一进程多 session，但一期不拿隔离换密度。
- warm pool 只降低冷启动，不改变“一 job 一进程”。

## 4. 网关 API

内部 API，仅 GP API/worker 可访问；使用 mTLS/服务账号或内部 token。

### 4.1 创建 job

```http
POST /internal/dsh/jobs
Content-Type: application/json

{
  "job_id": "gp-gen-2902560158584832-user-01",
  "team_id": 1001,
  "project_id": 2876690473259008,
  "module": "user",
  "profile": "gp-test-engine",
  "prompt_ref": "s3://gp-ai-input/.../prompt.txt",
  "workspace_ref": "s3://gp-repos/open-chat-admin/a4487643.tar.zst",
  "model": { "provider": "deepseek", "model": "deepseek-v4-pro" },
  "max_tokens": 64000,
  "timeout_ms": 900000,
  "idempotency_key": "gen:2902560158584832:user:01"
}
```

响应：

```json
{ "job_id": "...", "state": "queued", "created_at": "..." }
```

- `job_id` 由 GP 生成；网关幂等。
- `prompt_ref`/`workspace_ref` 为对象存储引用，不内联大文本。
- 网关只接受已签名的内部请求，并校验 team/project 权限。

### 4.2 查询状态

```http
GET /internal/dsh/jobs/{job_id}
```

```json
{
  "job_id": "...",
  "state": "running",
  "session_id": "gp-job-...",
  "worker_id": "w-3",
  "started_at": "...",
  "last_heartbeat_at": "...",
  "token_usage": { "input": 12345, "output": 6789 },
  "error": null
}
```

### 4.3 事件流

```http
GET /internal/dsh/jobs/{job_id}/events?after=123
```

返回 SSE 或分页 JSON；事件至少包含：

```json
{
  "seq": 124,
  "type": "session.event",
  "session_id": "...",
  "at": "...",
  "payload": { "kind": "tool_call|message|status", "data": {} }
}
```

事件落对象存储/审计，不长期放内存。

### 4.4 取消

```http
DELETE /internal/dsh/jobs/{job_id}
```

- 进入 `cancelling`，网关终止进程组，清理 workspace，标记 `cancelled`。
- 已产生的 transcript/事件保留。

## 5. Job 状态机

```text
queued
  -> starting
  -> running
  -> succeeded
  -> failed
  -> timeout
  -> cancelled
```

规则：

- `queued` 超时未调度 → `timeout`（`queue_timeout`）。
- `starting` 失败 → `failed`（`initialize_failed`/`worker_unavailable`）。
- `running` 心跳超时/超过 `timeout_ms` → `timeout`。
- 收到取消 → `cancelling` → `cancelled`。
- 成功必须满足：`session/wait` 返回 + 最终输出通过 Schema 校验。
- 状态只允许单向推进；终态不可变。

## 6. Worker 生命周期

```text
spawn
  -> health check
  -> initialize { provider, model, cwd, maxTokens }
  -> session/prompt { sessionId, content }
  -> loop: session.event
  -> session/wait
  -> validate output
  -> persist transcript
  -> terminate process / recycle
```

- `sessionId` 使用 `gp-<job_id>`；由网关生成并持久化映射。
- 每次 prompt 前确保 initialize 已成功。
- 启动后必须设置 `working-directory` 到只读 workspace。
- 输出只从最终 assistant 消息或结构化结果事件读取；stdout 不能混入日志。
- job 结束后进程退出；warm pool 补位。

## 7. 取消/超时/清理

SDK 无原生 cancel，采用进程级终止：

1. 标记 `cancelling`，停止接收新 prompt。
2. 发送 SIGTERM，宽限 10s。
3. 未退出：SIGKILL 进程组（Linux）或 Job Object/`taskkill /T`（Windows）。
4. 删除 workspace、临时文件、`DSH_HOME`（保留 transcript/证据到对象存储）。
5. 释放队列租约；记录 `cancel_reason`。
6. 不复用该进程/workspace。

看门狗：

- `heartbeat` 每 10s；60s 无心跳视为卡死。
- `max_runtime`、`max_tokens`、`max_tool_calls` 任一超限即终止。
- 进程退出码/信号写入 job error。

## 8. 崩溃恢复

- 网关 supervisor 捕获 worker 退出；job 标 `process_crashed`。
- `session/resume` 只在**新进程/新 workspace**中恢复持久化 session；不恢复被 kill 的进程。
- 是否自动恢复由 job `retry_policy` 决定；默认 infra 错误重试 1 次，policy/invalid output 不重试。
- reaper 定期清理孤儿进程、临时目录、过期 job。
- 网关重启后从 DB 恢复 queued/running 状态；running 的 worker 不存在则标 `process_crashed`。

## 9. 事件映射

DSH 事件 → GP 事件：

| DSH | GP |
|---|---|
| `session.event` (assistant message) | `ai.message` |
| `session.event` (tool call/result) | `ai.tool_call` / `ai.tool_result` |
| `session.status` | `ai.status` |
| `subagent.started/finished` | `ai.subagent` |
| `session/wait` 完成 | `ai.completed` |
| 进程退出/超时 | `ai.failed` / `ai.timeout` |

GP 事件落 `ai_job_event`，前端只读；工具参数/结果脱敏后保存。

## 10. 队列、租约、幂等

- 队列：Temporal 负责跨步骤编排；网关内部有界队列负责 worker 调度。
- 租约：worker 领取 job 后设置 `lease_id`/`lease_until`；心跳续租。
- 幂等：`idempotency_key` 唯一；重复提交返回同一 job。
- 背压：队列满返回 `queue_full`；GP 按团队/模块限流。
- 公平性：按 team/project 加权轮询，避免单团队占满 worker。

## 11. 安全 Profile

- `gp-test-engine` profile：`dsh-base` + `dsh-sdk-app`，固定版本。
- 工具白名单：读文件、grep、目录树、读 OpenAPI/日志。
- 禁用：`write`、`edit`、shell 写、git commit/push、部署、插件市场、Web。
- 源码 workspace 只读挂载；`DSH_HOME` 临时目录；不挂载用户 home。
- 网络：仅模型 API + GP 内部 API；默认拒绝其他外联。
- 凭据：`DEEPSEEK_API_KEY` 来自 Secret；不写日志、不回显。
- 输出：stdout 只允许 JSON-RPC；日志走 stderr。

## 12. 资源与配额

| 配置 | 建议默认 |
|---|---|
| `DSH_MAX_WORKERS` | 4 |
| `DSH_WARM_WORKERS` | 2 |
| `DSH_JOB_TIMEOUT` | 15 min |
| `DSH_MAX_TOKENS` | 64k/job |
| `DSH_MAX_TOOL_CALLS` | 200/job |
| `DSH_RECYCLE_AFTER_JOBS` | 1（一期不复用） |
| 单 worker 内存 | 1-2 GiB |
| 单 worker CPU | 1-2 core |

按团队配额；超限返回 `quota_exceeded`。

## 13. 可观测性

指标：

- `dsh_jobs_total{state}`
- `dsh_job_duration_seconds`
- `dsh_queue_depth`
- `dsh_worker_active`
- `dsh_tokens_total`
- `dsh_tool_calls_total`
- `dsh_cancel_total`
- `dsh_process_crash_total`

日志：job_id、team、module、worker、state、error code；不记录 token/源码内容。

追踪：Temporal workflow ID ↔ job_id ↔ session_id ↔ GP batch。

## 14. 错误码与重试

| 错误码 | 含义 | 重试 |
|---|---|---|
| `queue_full` | 队列满 | 稍后重试 |
| `worker_unavailable` | 无可用 worker | 是 |
| `initialize_failed` | 模型/路由握手失败 | 是（限 1 次） |
| `model_route_failed` | 模型不可用 | 是（限 1 次） |
| `prompt_rejected` | SDK 拒绝 prompt | 否 |
| `policy_violation` | 触发只读/工具策略 | 否 |
| `timeout` | 超时 | 可配置 |
| `cancelled` | 用户取消 | 否 |
| `process_crashed` | 进程崩溃 | 是（限 1 次） |
| `invalid_output` | JSON/Schema 校验失败 | 否（交 Critic） |

## 15. 关键时序

### 成功

```text
GP -> POST /jobs
gateway -> enqueue -> worker
gateway -> dsh initialize
gateway -> session/prompt
dsh -> session.event* -> gateway -> GP events
gateway -> session/wait
dsh -> done
gateway -> validate -> GET /jobs -> succeeded
```

### 超时

```text
GP -> POST /jobs
gateway -> running
watchdog -> timeout
gateway -> SIGTERM/SIGKILL -> cleanup
gateway -> job timeout -> GP
```

### 崩溃

```text
worker exits nonzero
gateway -> process_crashed
gateway -> retry (if allowed) with new worker/workspace
gateway -> session/resume (optional)
```

## 16. 验收

- 单 job 多步（规划→生成→校验）只启动一个 DSH 进程、一个 session。
- 取消/超时后进程组消失、workspace 删除、下一 job 正常。
- 网关重启后 queued/running job 状态可恢复/清理。
- 任何路径都不写源码；工具策略测试通过。
- 事件、token、工具调用可追溯。
- 并发 4 worker 下无 session 串扰。

## 17. 待实现顺序

1. 网关骨架 + job API + 状态机 + 队列。
2. worker 进程管理 + SDK initialize/prompt/wait + 事件落库。
3. 超时/取消/清理 + reaper。
4. 输出 Schema 校验 + GP 入库。
5. warm pool、配额、指标。
6. 崩溃恢复 + session/resume。

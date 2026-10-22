# 执行与 Fixtures 规格（GP-EXECUTION-FIXTURE-SPEC）

| 项目 | 值 |
|---|---|
| 状态 | 实施规格 v1（2026-10-22） |
| 范围 | 执行调度、fixtures、环境隔离、证据采集、失败分类 |
| 关联 | `GP-AI-TEST-ENGINE-EXECUTION-PLAN.md`、`SECURITY-BASELINE.md` |

## 1. 目标

- 每条用例在可复现、隔离、可清理的环境中执行。
- 执行结果有证据：请求/响应/日志/Trace + 截图/视频。
- 失败先归因，再决定是产品缺陷、用例缺陷、环境/数据问题还是预期变更。
- 破坏性用例不污染共享环境。

## 2. 执行单元

```text
ExecutionRun (run_run)
  └─ RunFixture (run_fixture)         # 本次 run 的数据命名空间/资源
       └─ CaseExecution (run_case_result)
            ├─ Attempt (run_attempt)
            └─ Evidence (evd_evidence)
```

- 一个 run 一个 fixture 命名空间；默认不复用。
- 用例之间共享只读参考数据；可变数据按 run/用例隔离。
- 执行顺序不影响结果；禁止依赖前一条用例的副作用。

## 3. 环境模型

| 类型 | 用途 | 隔离 |
|---|---|---|
| `shared-readonly` | 字典/配置/只读资源 | 共享，只读 |
| `isolated-run` | 用户/订单/发布/审批等可变数据 | 每 run 独立命名空间 |
| `destructive` | 删除、审批、发布、支付回调 | 专用 fixture 租户 + 快照/补偿 |
| `external-sandbox` | 预签名/对象存储/第三方 | 专用测试桶/账号 |

- 执行前检查环境版本：`env_runtime` vs 目标版本，不一致 `blocked`。
- 环境租约：`env_lease` 防止同一环境被并发破坏性使用。
- 环境健康检查失败：`environment_defect`，不计产品失败。

## 4. Fixture 模型

```text
run_fixture
  id, team_id, run_id, namespace, manifest jsonb,
  state(preparing/ready/cleaning/cleaned/failed),
  expires_at, cleanup_state, created_at, updated_at
```

`manifest` 示例：

```json
{
  "tenant": "gp-run-4821",
  "roles": ["admin", "operator", "viewer"],
  "resources": [
    { "kind": "user", "count": 3, "prefix": "gp-run-4821" },
    { "kind": "group", "count": 1 }
  ],
  "cleanup": ["users", "groups", "uploads"]
}
```

规则：

- 优先通过业务 API 创建/清理；只有专用测试库允许 DB seed/reset。
- 所有可变数据带 `run_id` 前缀；幂等键带 run ID。
- 清理在成功/失败/超时都执行；异步 GC 兜底。
- 清理失败进入告警，不静默。
- fixture 不可跨 run 复用；只读参考数据除外。

## 5. 执行状态机

```text
queued
  -> preparing
  -> running
  -> collecting
  -> completed
  -> failed
  -> blocked
  -> cancelled
```

- `preparing`：环境检查、fixture seed、租约。
- `running`：逐用例执行，记录 attempt。
- `collecting`：证据上传/哈希/缩略图。
- `completed`：所有用例有终态。
- `blocked`：环境/版本/fixture 不满足，不算产品失败。
- `cancelled`：用户取消，清理 fixture。

## 6. Runner 适配

| Runner | 用例类型 | 证据 |
|---|---|---|
| HTTP/API | 请求/响应/契约 | request/response/log/trace |
| Browser | Web UI | screenshot/video + console/network |
| Mobile/real device | App/小程序 | screenshot/video + device log |
| Custom | 脚本/规则/压测 | stdout/stderr + artifacts |

Runner 接口：

```go
type Runner interface {
    Prepare(ctx context.Context, fixture FixtureContext) error
    Execute(ctx context.Context, tc CaseExecution) (CaseResult, error)
    Collect(ctx context.Context, tc CaseExecution) ([]EvidenceRef, error)
    Cleanup(ctx context.Context, fixture FixtureContext) error
}
```

- Runner 只读源码；测试脚本/数据来自 GP 对象存储。
- 浏览器/移动 runner 必须采集截图；关键流程录视频。
- 证据上传走 `evd_evidence` 预签名流程。

## 7. 失败分类与重试

| class | 判定 | 处理 |
|---|---|---|
| `product_defect` | 契约不符/5xx/状态错误/安全 | 创建 finding |
| `case_defect` | 数据非法/断言写错/选择器过期 | 修复用例，保留原断言 |
| `environment_defect` | 服务未起/版本不符/网络 | 修复环境/重试 |
| `data_defect` | fixture 冲突/污染 | 重建 fixture |
| `expected_change` | 契约确实变更 | 人工批准后更新 oracle |
| `flaky` | 非确定性 | 隔离/观察，不计产品缺陷 |

重试：

- 仅 `environment_defect`/基础设施错误自动重试，最多 2 次。
- `product_defect` 不重试（除非确认偶现，转 flaky）。
- 重试必须新 attempt，保留前次证据。
- 同一用例连续 flaky 进入 quarantine，不参与门禁通过率。

## 8. 证据采集

- 执行器在断言失败时自动抓取：请求/响应（脱敏）、日志、Trace、截图。
- 视频由 runner 根据用例类型决定；关键流程默认开。
- 证据带 `run_id/case_id/attempt/commit/env` 元数据。
- 上传失败重试；仍失败标 `evidence_incomplete`，P0/P1 不得关闭。
- 证据只读；不覆盖历史 attempt。

## 9. 安全

- Runner 专用 SA、no token automount、非 root、seccomp、drop caps、只读根。
- 源码只读挂载；workspace 临时可写。
- 测试命令只在 runner 执行，不在 server/worker。
- egress 按环境 allowlist；staging 验证后启用 default-deny。
- 凭据用 Secret 引用；不写日志/证据。

## 10. 数据模型

```sql
CREATE TABLE run_fixture (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  run_id BIGINT NOT NULL,
  namespace TEXT NOT NULL,
  manifest JSONB NOT NULL,
  state TEXT NOT NULL,
  expires_at TIMESTAMPTZ,
  cleanup_state TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE env_lease (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  env_id BIGINT NOT NULL,
  run_id BIGINT NOT NULL,
  owner TEXT NOT NULL,
  lease_until TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- `run_`/`env_` 前缀已登记。
- RLS + `team_id`；清理任务独立 worker。
- 迁移编号在 M0 中追加（`000037_execution_fixture`）。

## 11. API

```text
POST /runs/{id}/fixtures:prepare
GET  /runs/{id}/fixtures
POST /runs/{id}/fixtures:cleanup
POST /envs/{id}/lease
DEL  /envs/{id}/lease
GET  /runs/{id}/results
POST /runs/{id}/triage
```

## 12. 验收

- 同一 run 重复执行结果稳定；无跨 run 数据污染。
- 破坏性用例使用独立 fixture，清理成功。
- 环境版本不符直接 blocked，不产生产品缺陷。
- P0/P1 失败有截图 + 请求/响应/日志。
- 重试/取消/超时都执行清理；无孤儿 fixture。
- `go test -race` + testcontainers 覆盖 fixture/lease/cleanup。

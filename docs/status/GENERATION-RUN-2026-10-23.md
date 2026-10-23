# GenerationRun：一次生成任务的顶层模型（2026-10-23）

## 目标

把“项目全量 / 仓库全量 / 指定服务”统一成一次**生成任务（run）**，不再由前端循环创建互不相关的批次。

```text
GenerationRun  一次用户操作
  └─ GenerationUnit  一个服务
       └─ gen_batch  现有 DSH/Temporal 执行单元
            └─ Candidate
```

## 数据模型

### gen_run

- `scope_type`: `project | repo | service`
- `scope_id`: 对应项目 / 仓库 / 服务
- `state`: `queued | running | succeeded | partial_failed | failed | cancelled`
- `total_units / completed_units / failed_units`
- 预留 `tokens_in / tokens_out / cost_amount`
- `idempotency_key`：每次点击独立

### gen_run_unit

- `run_id`
- `service_id / repo_id`
- `batch_id`：指向 `gen_batch`
- `state`: `queued | running | succeeded | failed | cancelled`
- `error_summary`

`gen_batch` 作为服务级执行单元保留，避免复制 DSH/Temporal 状态机。

## API

```text
POST   /generation/runs
GET    /generation/runs?project_id=...
GET    /generation/runs/{id}
POST   /generation/runs/{id}/cancel
```

创建请求：

```json
{
  "project_id": 1,
  "scope_type": "repo",
  "scope_id": 2,
  "idempotency_key": "ui-run-...",
  "units": [
    {
      "service_id": 3,
      "repo_id": 2,
      "source_path": "services/foo",
      "ref_type": "branch",
      "ref_name": "main"
    }
  ]
}
```

后端在 `Create` 中：

1. 创建 `gen_run` 与全部 `gen_run_unit`；
2. 逐服务调用现有 `GenerationBatchService.Create`；
3. 回填 `batch_id`；
4. 单个服务失败只标记该 unit，run 进入 `partial_failed`；
5. 全部成功则 run 运行中，等待候选审核。

## 进度

`GET /generation/runs/{id}` 会读取每个 unit 的 batch 状态并汇总：

- `approved` → unit succeeded
- `failed/rejected/rolled_back` → unit failed
- 其他 → unit running
- 全部 terminal → run `succeeded / partial_failed / failed`

## 取消

`POST /generation/runs/{id}/cancel`：

- 遍历所有活动 unit；
- 调用 batch 级取消（停止 Temporal/DSH）；
- batch 取消会删除该批 `gen_seed`、`gen_source_snapshot`；
- 已 approved 的 unit 不回滚，保留为已完成；
- run 进入 `cancelled`，保留 tombstone。

## 验证

| 场景 | 结果 |
|---|---|
| 创建单服务 run | 202，run 和 batch 同时创建 |
| run 取消 | run cancelled；batch cancelled；seeds=0；snapshot=0 |
| run 成功 | 单服务批次审批后 run=succeeded，completed=1 |
| 迁移 | schema 45 |
| Go 测试 | `go test ./...` 通过 |
| 前端类型检查 | 通过 |

测试 ID：

- 取消：run `2905909840089088`
- 成功：run `2905913900863488`

## 成本审计

`gen_run` 已预留 tokens/cost 字段，但**尚未写入**。

原因：DSH gateway 还没有返回 usage。下一步：

1. 从 DSH `assistant/attempt` 事件提取 tokens；
2. gateway `Result` / HTTP 响应增加 `tokens_in/tokens_out`；
3. 生成完成/取消时写 `cost_line_item`（`case_generate`）；
4. 聚合到 `gen_run`。

当前不能写 0 成本冒充已接入。

## 已知问题

- 审核仍以 batch 为单位；run 卡可跳转到 unit 的 batch，但还没有 run 级批量审核。
- 失败 unit 的 batch 数据目前保留用于排障，尚未做 TTL janitor。
- `ListByProject` 目前逐 run 查询 unit，后续可改聚合查询。
- run 成本字段为 0，等待 DSH usage。

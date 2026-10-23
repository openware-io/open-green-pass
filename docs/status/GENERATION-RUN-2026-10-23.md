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

`units` 可以为空：后端按 scope 查询已确认服务并自动使用仓库默认分支；`custom` 仍提交 units。\n\n后端在 `Create` 中：

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
- 最终回归：run `2906002403921920`（run 级审核后 succeeded）
- 成本：batch `2905988727508992`，tokens `234 / 4483`，写 `case_generate` 成本行

## 成本审计

已接入 DSH usage：

- 从 DSH `assistant/attempt` 事件读取 `inputTokens` / `outputTokens`；
- gateway `Result` / HTTP 响应增加 `tokens_in/tokens_out`；
- 写入 `gen_batch.tokens_in/tokens_out`；
- 写 `cost_line_item`：
  - `biz_point=case_generate`
  - `model=dsh/dsh-sdk`
  - `unit_price=GP_GENERATION_UNIT_PRICE`
- `gen_run` 聚合 tokens 和 amount。

未配置 `GP_GENERATION_UNIT_PRICE` 时金额为 0，但 tokens 会保留；需要运营配置单价。

## 已补充

- run 级审核：`POST /generation/runs/{id}/review`，对当前待审核 unit 批量 approve/reject。
- TTL：迁移 47 的 `gp.purge_generation_artifacts`；worker 每小时清理 failed/rejected/rolled_back/cancelled 批次超过 `GP_GENERATION_RETENTION_HOURS`（默认 24h）的候选和源码快照。
- 用例 code 冲突：复用已存在 `cas_case`，避免重复 key 导致入库失败。

## 已知问题

- `GP_GENERATION_UNIT_PRICE` 未配置时金额为 0。
- `ListByProject` 目前逐 run 查询 unit，后续可改聚合查询。
- 失败批次保留一段时间排障，由 TTL 清理。

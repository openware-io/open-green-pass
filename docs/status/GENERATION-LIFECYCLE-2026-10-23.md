# 生成任务生命周期：取消、清理、成本审计（2026-10-23）

## 取消与清理（已完成）

### API

`POST /api/generation/batches/{id}/cancel`

```json
{
  "expected_state_version": 6,
  "request_id": "ui-cancel-..."
}
```

- 允许状态：`enqueued/fetching/parsing/generating/quality_check/review`
- 终态新增：`cancelled`
- 取消后：
  - Temporal workflow 收到 Cancel
  - DSH 轮询感知 context 取消并调用 gateway cancel
  - 删除 `gen_seed`
  - 删除 `gen_source_snapshot`
  - `gen_batch` 保留为 tombstone，供审计

### UI

批次详情卡右上角新增“取消生成”，仅活动状态显示。

### 验证

批次 `2905774286147584`：

- 取消前：`review`
- 取消后：`cancelled`
- `gen_seed = 0`
- `gen_source_snapshot = 0`

## 重新生成

- 每次生成使用新的 UUID idempotency key；
- 新批次独立，不复用已取消批次；
- 已取消/失败批次不会成为系统基线；
- 系统基线只选 `approved` 批次。

## 失败与异常

- 生成活动失败会进入 `failed`，不写入正式用例；
- 审批入库时质量重跑已修复（使用 normalized manifest）；
- 单条候选失败不阻断整批，失败候选标记 `rejected`。

## 成本审计（未完成）

当前：

- **执行 run 已接入** `cost_line_item`，biz point `case_execute`；
- **生成批次尚未接入**成本审计。

原因：

- 生成走 DSH SDK，gateway 当前只返回文本输出，没有返回 token usage；
- 没有 usage 就无法写准确的 `tokens_in/tokens_out`，不能伪造金额。

需要后续：

1. 从 DSH `assistant/attempt` 事件提取 usage；
2. 在 `DSHJob`/gateway HTTP 响应中增加 tokens 字段；
3. 生成完成时写 `cost_line_item`：
   - `biz_point=case_generate`
   - `model=dsh-sdk` 或实际模型
   - `tokens_in/tokens_out`
4. 取消的 job 按实际已消耗 usage 记录，不按完成计费。

## 仍存在的风险

- 取消与活动写入存在极短竞争窗口；取消后应立即清理，但极端情况下需 janitor 兜底。
- 失败批次当前保留候选/源码快照用于排障，尚未做 TTL 自动清理。
- DSH 进程工作目录由 runner defer 清理；gateway 崩溃场景还需容器重启策略 + 临时目录清理。

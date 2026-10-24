# GreenPass 覆盖率 waiver API（0.1.161-SNAPSHOT）

## 功能

新增显式、可审计的覆盖率豁免能力：

```text
POST   /coverage/waivers
GET    /coverage/waivers?project_id=
DELETE /coverage/waivers/{id}
```

请求示例：

```json
{
  "cell_ids": [2907533493374976],
  "reason": "media fixture unavailable",
  "owner_id": 42,
  "expires_at": "2026-12-31T00:00:00Z",
  "evidence": {"fixture": "media"}
}
```

## 行为

- `cov_waiver` 持久化 reason / owner / approved_by / expires_at / evidence。
- `cov_cell.waiver_id` 指向 waiver，状态改为 `waived`。
- 门禁汇总将 waived Cell 从 required 分母排除，同时保留 `waived` 计数。
- 撤销 waiver 后 Cell 回到 `planned`，重新进入 required。
- 必须填写原因；过期时间必须是未来；证据必须是合法 JSON。

## 验证

```text
POST   -> 201，created_at 正常
GET    -> 200
DELETE -> 204
waiver 生效时 total_required 441 -> 440
撤销后 total_required 440 -> 441
```

## 版本

```text
0.1.161-SNAPSHOT
schema 48 clean
```

# GreenPass kind 发布记录：0.1.157-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.157-SNAPSHOT` |
| schema | 48 clean |
| 变更 | 支持用例 `blocked_reason`，缺测试数据时标记 blocked 而不是伪造 pass/fail |
| 验证 | user run 9 pass / 3 blocked；coverage 18 passed / 6 blocked |

## 说明

- HTTP Runner 在发送请求前识别 `blocked_reason`。
- blocked 用例不发起 HTTP 请求，记录原因作为证据。
- 用于非管理员 token、可删除专用用户等当前不可得的测试数据。

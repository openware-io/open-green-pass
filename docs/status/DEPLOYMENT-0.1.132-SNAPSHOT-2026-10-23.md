# GreenPass kind 发布记录：0.1.132-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.132-SNAPSHOT` |
| 部署 checkout | `a5eaedb` |
| schema | 44 |
| server / worker | `sha256:168d07f324a43db7282c40823e79f3717247cbd87df83f588b6a4ded7dd2bf49` |
| web | `sha256:3780149e8ee7d7e161e9edd589abd5ff189fab918739040dc7bea1779dc8f00e` |
| migrate | `sha256:e3a4b38a743124ca1598fb5a4b98ddb1740413e156c84ab1e110fd9b072e0545` |
| dsh-gateway | `sha256:5893ec2fdee905c40ae8eefa08571352f261d4d325d0d7069a082fba62c5b1ef` |

## 新增

- 生成批次 `cancelled` 状态（迁移 44）
- `POST /generation/batches/{id}/cancel`
- 取消时停止 Temporal/DSH，删除 `gen_seed` 与 `gen_source_snapshot`
- 批次详情“取消生成”按钮

## 验证

| 检查 | 结果 |
|---|---|
| 批次 `2905774286147584` | cancelled |
| gen_seed 清理 | 0 |
| gen_source_snapshot 清理 | 0 |
| 新批次审批 | 0.1.130 已验证 |
| DSH 生成 | 真实 DSH |
| IM 隔离 | 重试后通过 |

## 说明

- 成本审计：执行已接入；生成尚未接入 token usage。
- 失败批次目前保留候选用于排障，尚未做 TTL 清理。

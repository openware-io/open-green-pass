# GreenPass kind 发布记录：0.1.138-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.138-SNAPSHOT` |
| 部署 checkout | `bd490a4` |
| schema | 47 |
| server / worker | `sha256:19ce2e9d5f7b158052f1868606b9ed3bb9e7f9b6e2baccc102f8a88d5a31d64d` |
| web | `sha256:dc0e7216a1ae1c4bd5633518722b1ed3f2f13af08ac26e22b7589959eedaa0f6` |
| migrate | `sha256:cdc6683474b69164ba9bc3019547feb4f6894c040b210410d57385ddfb13f19f` |
| dsh-gateway | `sha256:6e63ce34bee06d027dceec49b3d822b1ce339348cb5b00fa978847d40cb43dc3` |

## 完成内容

- GenerationRun / GenerationUnit 顶层模型。
- run 级审核：批量 approve/reject 当前待审核 unit。
- DSH usage 回传、tokens 入库、`case_generate` 成本行、run 聚合。
- 生成用例 code 冲突修复：复用已存在 `cas_case`。
- TTL janitor：清理终态批次超过保留期的候选和源码快照。

## 验证

| 检查 | 结果 |
|---|---|
| 创建/取消 run | 通过 |
| run 级审核 | run `2906002403921920` succeeded |
| tokens | batch `2905988727508992`：234 / 4483 |
| 成本行 | `case_generate` 已写入，unit_price 未配置时 amount=0 |
| 用例入库 | 通过（code 冲突已修复） |
| TTL | 函数清理 6 行；seeds/snapshot 归零 |
| schema | 47 |

## 配置

- `GP_GENERATION_UNIT_PRICE`：每千 token 单价；未配置时金额为 0。
- `GP_GENERATION_RETENTION_HOURS`：终态生成数据保留小时数，默认 24。

## 已知问题

- `ListByProject` 仍是 N+1 查询。
- 成本单价未在部署中设置，当前 tokens 有值、金额为 0。

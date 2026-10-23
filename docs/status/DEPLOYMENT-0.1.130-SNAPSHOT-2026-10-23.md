# GreenPass kind 发布记录：0.1.130-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.130-SNAPSHOT` |
| 部署 checkout | `09777c0` |
| schema | 43 |
| server / worker | `sha256:1837269ca94117dbe717eda500576b11e7be7ff92ce375cd666a74c70045020e` |
| web | `sha256:e32c41dc55c11fd652e0f163e9ebcac5dba0aef20f1ae60dde72c98ad7edcd03` |
| dsh-gateway | `sha256:81dd1e6e528ddc3cd191ed681f7f9edceb1ba03e071d2117bf27d247df45e0a8` |

## 修复

- 审批时质量重跑使用 `normalized.Files`，不再因源码内容已清理而误判。
- 单条候选质量失败不阻断整批。
- `CommitSeeds` 允许只提交质量通过候选，失败候选标记 rejected。

## 验证

| 检查 | 结果 |
|---|---|
| 新批次 `2905722991808512` | approved |
| seed | 6 passed / 6 approved |
| 审批入库 | 成功 |
| DSH 生成 | 真实 DSH |
| IM 隔离 | 重试后通过 |

## 说明

- 原失败批次 `2905666997489664` 不能原地恢复，需要重新生成。
- `platform-order-service` 发布期间仍有重启，导致 GP 部署隔离检查需重试。

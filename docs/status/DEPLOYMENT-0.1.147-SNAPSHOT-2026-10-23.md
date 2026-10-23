# GreenPass kind 发布记录：0.1.147-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.147-SNAPSHOT` |
| 部署 checkout | `7061a2f` |
| schema | 48 clean |
| server / worker | `sha256:cb9c0079b253b31753a1c0a7d67e44561edce2cdc5f934054ba94b03cf2be999` |
| web | `sha256:633731d2e58854e87ecfe154c1ef36009ab8acf788ef0e7a6001168ebf051817` |
| migrate | `sha256:3ad37386f8448f6c1e180c2fb111c30f1a4093efe1dca7e9f0386cf945701a19` |
| dsh-gateway | `sha256:16270d8b040f12cc0f95da0368531b054d025429f283c249b552a92aa6a532ec` |

## 变更

- 模型价目表 `cost_price` 扩展：model_key、输入/输出/缓存价、生效时间。
- `cost_line_item` 拆分单价和缓存 token。
- 生成成本按实际模型解析价目；无价目标记 `price_missing`。
- DSH 返回实际模型（`deepseek-flash`）。
- 成本审计页新增价目配置与缺失单价告警。
- 移除 `GP_GENERATION_UNIT_PRICE` 混合价。

## 验证

| 检查 | 结果 |
|---|---|
| 价目创建 | 201 |
| 实际模型 | `dsh/deepseek-flash` |
| 成本 | 234/3137 → ¥0.025564 |
| price_missing | false |
| run | `2906337427951616` succeeded |
| 成本明细 API | 200 |
| 页面 | 价目表/模型/金额显示 |
| schema | 48 clean |

## 说明

- 价目单位：人民币/千 token。
- 历史行保留当时单价。
- `GP_GENERATION_UNIT_PRICE` 不再使用。
- 迁移 48 修复了既有 `cost_price` 表与 dirty 状态。

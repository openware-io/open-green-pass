# GreenPass kind 发布记录：0.1.136-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.136-SNAPSHOT` |
| 部署 checkout | `c6bfb78` |
| schema | 45 |
| server / worker | `sha256:957e92433d2cedb5f2e7236303a725d12b100e8ba76fb329c4dc1e388a63c41e` |
| web | `sha256:416fc2b8e178a9bf540f31691325849de4ffd9d2df6c8bbd9f8cf1e81151ddea` |
| migrate | `sha256:8685b68c4d6748d6a5f541334f3bacbb429f02b92660ee8cce8b7c35100fc713` |
| dsh-gateway | `sha256:4a754f95b18fa479c0646a02b5042a11c03ee94ffabd054e2d8bc81fcd286e0d` |

## 核心变化

- 新增 `gen_run` / `gen_run_unit`（迁移 45）。
- 生成页改为创建**一次生成任务**，由后端展开服务单元并逐个创建 batch。
- 任务卡展示总体进度和每个服务状态。
- 取消整个任务会级联取消服务批次并清理候选/源码快照。
- 静态“覆盖计划”原型数据只在对 mock 对象时显示，真实项目隐藏。

## 验证

| 检查 | 结果 |
|---|---|
| 创建 run | 202，run + batch 创建成功 |
| run 取消 | cancelled；batch cancelled；seeds=0；snapshot=0 |
| run 成功 | run=succeeded，completed=1 |
| 前端构建 | 通过 |
| Go 测试 | 通过 |
| schema | 45 |

测试：

- 取消 run `2905909840089088`
- 成功 run `2905913900863488`

## 未完成

- 生成成本未写入（DSH 未回传 usage）。
- 审核仍按 batch，未做 run 级批量审核。
- 失败批次/unit 的 TTL 清理未实现。

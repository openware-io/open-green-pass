# GreenPass kind 发布记录：0.1.143-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.143-SNAPSHOT` |
| 部署 checkout | `c83bc3b` |
| schema | 47 |
| server / worker | `sha256:21b0f3a8f408b2fa7823317915a059f90a319397e2a8f29ddaea53608da8951a` |
| web | `sha256:40d43bc46a757aa27467f1f0b4d0a276237e9afbd146b0964e845760a091d0c6` |
| migrate | `sha256:cdc6683474b69164ba9bc3019547feb4f6894c040b210410d57385ddfb13f19f` |
| dsh-gateway | `sha256:9e615794cce91280384ad03a3156b1c53f763fb856ad7827c9abc625af2fd8b5` |

## 收口

- 最近任务下拉的状态改为中文。
- 生成页明确“最近生成任务”和“新建生成任务”两个区域。
- 批次列表只展示当前选中 run 的 unit。
- 全量模式由后端展开服务。

## 验证

| 检查 | 结果 |
|---|---|
| 最近任务下拉 | 5 个 run，中文状态 |
| 当前 run 批次过滤 | 1 个批次 |
| 页面加载 | 通过（无头 Chrome） |
| Go 测试 | 通过 |
| 前端 typecheck | 通过 |
| schema | 47 |

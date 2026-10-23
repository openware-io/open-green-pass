# GreenPass kind 发布记录：0.1.142-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.142-SNAPSHOT` |
| 部署 checkout | `40f536c` |
| schema | 47 |
| server / worker | `sha256:21b0f3a8f408b2fa7823317915a059f90a319397e2a8f29ddaea53608da8951a` |
| web | `sha256:329f08aa9cfbfa610a7db994ff5379d46f40fa22c1e0f8b5001dfaf7f27ba3f6` |
| migrate | `sha256:cdc6683474b69164ba9bc3019547feb4f6894c040b210410d57385ddfb13f19f` |
| dsh-gateway | `sha256:da1f7671ffe33db84b93fecdd6f16242cb2a97ea4ae1a6779d57bfbc8d2c2b5b` |

## UI 收口

- 生成页明确区分“最近生成任务”和“新建生成任务”。
- 批次列表只展示当前选中 run 的 unit，不再把 20+ 历史批次全铺开。
- 支持最近任务下拉切换。
- `cancelled` 状态中文展示；取消批次显示“任务已取消，候选和源码快照已清理”。
- 全量模式不再由前端逐个构造 unit。

## 验证

| 检查 | 结果 |
|---|---|
| 页面登录 | 有头/无头浏览器均可加载 |
| 最近任务下拉 | 5 个 run 可选 |
| 批次过滤 | 当前 run 只显示 1 个批次 |
| `cancelled` 文案 | 中文 |
| 前端 typecheck | 通过 |
| Go 测试 | 通过 |
| schema | 47 |

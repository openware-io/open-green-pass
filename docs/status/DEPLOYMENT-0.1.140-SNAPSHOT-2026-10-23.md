# GreenPass kind 发布记录：0.1.140-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.140-SNAPSHOT` |
| 部署 checkout | `57999ee` |
| schema | 47 |
| server / worker | `sha256:21b0f3a8f408b2fa7823317915a059f90a319397e2a8f29ddaea53608da8951a` |
| web | `sha256:2d3eb5d801c7dfabe9af5fa96c26f99cab6684ba1ce19d92b9c3ff92ec6812ab` |
| migrate | `sha256:cdc6683474b69164ba9bc3019547feb4f6894c040b210410d57385ddfb13f19f` |
| dsh-gateway | `sha256:f7e2aefd348c9669c79bbffe121056ed2b87471177caab7cb7de5c29d9aa2c5b` |

## 变更

- 项目全量/仓库全量由后端按 scope 展开服务，不再由前端逐个构造 unit。
- 前端只在“指定服务”模式提交 units 和逐服务分支/Tag。
- 全量模式不会因为某个服务的代码引用未加载而阻塞。

## 验证

| 检查 | 结果 |
|---|---|
| service scope + units=[] | 202，后端解析 1 个服务 |
| run 取消 | cancelled |
| 全量 UI | “开始生成” / run 审核按钮存在 |
| Go 测试 | 通过 |
| 前端 typecheck | 通过 |
| schema | 47 |

测试：

- run `2906023962578944`（后端 scope 解析 + 取消）
- 前序 run `2906002403921920`（run 级审核成功，用例入库）
- 成本 batch `2905988727508992`（tokens 234/4483）

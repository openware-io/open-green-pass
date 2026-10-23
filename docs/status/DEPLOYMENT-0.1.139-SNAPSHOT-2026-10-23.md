# GreenPass kind 发布记录：0.1.139-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.139-SNAPSHOT` |
| 部署 checkout | `4c8c7be` |
| schema | 47 |
| server / worker | `sha256:21b0f3a8f408b2fa7823317915a059f90a319397e2a8f29ddaea53608da8951a` |
| web | `sha256:d2a7e1c285fc8295668a730fe50d8b96017571634b3d81882dcbb9489e207215` |
| migrate | `sha256:cdc6683474b69164ba9bc3019547feb4f6894c040b210410d57385ddfb13f19f` |
| dsh-gateway | `sha256:9393dbd9ef465464ef9c2174db5743f94f87758f13d6e307b0a6806ff46e4667` |

## 变更

- `POST /generation/runs` 支持 `units=[]`：
  - 后端按 `scope_type/scope_id` 查询已确认服务；
  - 自动使用仓库默认分支；
  - 不再要求前端展开全量服务列表。
- `custom` 指定服务仍由前端提交 units，保留逐服务分支/Tag。

## 验证

| 检查 | 结果 |
|---|---|
| service scope + units=[] | 202，自动解析 1 个服务 |
| run 取消 | cancelled |
| 默认分支 | 使用仓库 default_branch |
| Go 测试 | 通过 |
| 前端 typecheck | 通过 |

测试 run：`2906023962578944`（创建后取消）。

# GreenPass kind 发布记录：0.1.91-SNAPSHOT（2026-10-22）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.1.91-SNAPSHOT` |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `352d491f29ae8dabe4316ad2a43ccec0298de83d` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.91-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-22 14:45（Asia/Shanghai） |

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.1.91-SNAPSHOT` | `sha256:42ade7d5f3779c83cbbd130d8dc26378798132de553a1b189744a8bf77dc0198` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.1.91-SNAPSHOT` | `sha256:8c3a99628a102ad5794015c6da6d0c41dab4bb389e14046917192c8d8cd57d64` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.1.91-SNAPSHOT` | `sha256:db97f78aefe5472add1f2cfe6bd0fe05389ad813e60c33dcdbb269bbe55824ab` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.1.91-SNAPSHOT`。

## 修复

### 当前使用模型可选

模型页顶部增加“项目默认模型”栏：

- 项目下拉：`im-saas` 等项目。
- 模型下拉：该项目当前绑定模型，选项来自该团队已审批模型。
- 切换即调用 `PUT /targets/{id}/model-binding`，并提示成功/失败。
- 生成页也保留同样的切换入口。

### 官方模型发现反馈

- “获取官方模型列表”成功时 toast 显示“已获取 N 个官方模型”，并在弹窗内显示“已从官方 `/models` 获取 N 个模型”。
- 失败时显示具体错误 toast，不再静默。
- 模型选择改为可点击卡片：已启用为绿色 + “已启用”，新模型为可勾选卡片 + Check 图标，不再使用视觉上像未选中的原生 disabled checkbox。
- 已启用模型在“管理模型”中锁定，避免误改；项目使用哪个模型由顶部“项目默认模型”选择。

## 验收

| 检查 | 结果 |
|---|---|
| 项目默认模型栏 | `im-saas` / `deepseek / deepseek-v4-pro` |
| 切换 Flash | 成功，绑定变为 `deepseek / deepseek-flash` |
| 恢复 V4 Pro | 成功，绑定恢复 `deepseek / deepseek-v4-pro` |
| 获取官方模型 | toast “已获取 2 个官方模型”，弹窗显示 `deepseek-flash`、`deepseek-v4-pro` |
| 弹窗状态 | 两个模型卡片均为绿色“已启用” |
| 生成页选择 | 仍支持项目模型切换 |

## 迁移与运行检查

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-1-91-snapshot` |
| 结果 | `Complete` |
| Schema 版本 | `33` |
| Dirty | `false` |
| `/api/readyz` | 200，`database/redis/temporal=ok` |
| 登录 / `/api/auth/me` | 200 / 200 |
| `open-im-local` 快照核对 | 脚本内置检查通过 |

## 镜像清理

- Docker 本地清理 `0.1.90`，当前仅保留 `0.1.91`。
- 两个 kind 节点删除 `0.1.90` 的 server/web/migrate tag，当前仅保留 `0.1.91`。

## 结论

`0.1.91-SNAPSHOT` 已补上项目默认模型选择，并把官方模型发现的结果/错误显式反馈；模型页从“只能看”变为“可切换当前使用模型”。

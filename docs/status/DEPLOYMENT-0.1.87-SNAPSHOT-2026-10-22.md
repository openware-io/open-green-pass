# GreenPass kind 发布记录：0.1.87-SNAPSHOT（2026-10-22）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.1.87-SNAPSHOT`（`0.1.86` 之后的下一 patch） |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `4ca70f7fe2da23f4fd25314f3a04c83ddf9a15e3` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.87-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-22 13:48（Asia/Shanghai） |

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.1.87-SNAPSHOT` | `sha256:ddf9a89aaae947620c2adeb2f3d4521b27f2ca0f7a4d86b7a124a25bc6908819` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.1.87-SNAPSHOT` | `sha256:e0889a3b8fbb3a84bc594d1c92e20b16554f03fe0f30a29a4b6aedf834f8c116` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.1.87-SNAPSHOT` | `sha256:db97f78aefe5472add1f2cfe6bd0fe05389ad813e60c33dcdbb269bbe55824ab` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.1.87-SNAPSHOT`。

## 模型选择

0.1.86 已把模型配置改成“一个供应商 + 多个模型”；0.1.87 在生成页增加项目级模型选择：

- 模型横幅显示当前绑定与“已配置模型 N 个”。
- 多模型时出现“生成模型”下拉，选项来自该团队已审批的模型（当前为 `deepseek / deepseek-v4-pro` 与 `deepseek / deepseek-flash`）。
- 切换后调用 `PUT /targets/{id}/model-binding`，立即更新项目绑定并提示。
- 项目仍默认绑定 `deepseek / deepseek-v4-pro`；可随时在生成页切换。

## 验收

| 检查 | 结果 |
|---|---|
| 模型下拉 | 存在，选项为 `deepseek / deepseek-flash`、`deepseek / deepseek-v4-pro` |
| 切换 Flash | 成功，服务端绑定变为 `deepseek / deepseek-flash` |
| 恢复 V4 Pro | 成功，服务端绑定恢复为 `deepseek / deepseek-v4-pro` |
| 模型目录 | DeepSeek 两个模型 `approved`，共享 `gp-model-1001-deepseek-api` |
| 生成批次 | `2902560158584832`，Tag `2.2.1`，5 条候选通过质量校验，状态 `review` |

## 迁移与运行检查

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-1-87-snapshot` |
| 结果 | `Complete` |
| Schema 版本 | `33` |
| Dirty | `false` |
| `/api/readyz` | 200，`database/redis/temporal=ok` |
| 登录 / `/api/auth/me` | 200 / 200 |
| `open-im-local` 快照核对 | 脚本内置检查通过 |

## 镜像清理

- Docker 本地清理 `0.1.86`，当前仅保留 `0.1.87`。
- 两个 kind 节点删除 `0.1.86` 的 server/web/migrate tag，当前仅保留 `0.1.87`。
- `0.3.x` 仍为空。

## 结论

`0.1.87-SNAPSHOT` 已通过唯一入口发布；供应商级模型配置与项目级模型选择均已可用，DeepSeek 两个模型可在生成页切换，生成链路保持正常。

# GreenPass kind 发布记录：0.1.86-SNAPSHOT（2026-10-22）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.1.86-SNAPSHOT`（`0.1.85` 之后的下一 patch） |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `e1734f9ed2c0584c92e09215779c6e66923ccaff` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.86-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-22 13:43（Asia/Shanghai） |

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.1.86-SNAPSHOT` | `sha256:ddf9a89aaae947620c2adeb2f3d4521b27f2ca0f7a4d86b7a124a25bc6908819` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.1.86-SNAPSHOT` | `sha256:1f58c7592f849df6fa5468b2071c93969273980dec5bf7f3201d6578782e8500` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.1.86-SNAPSHOT` | `sha256:db97f78aefe5472add1f2cfe6bd0fe05389ad813e60c33dcdbb269bbe55824ab` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.1.86-SNAPSHOT`。

## 模型配置交互

问题：连接按“模型”创建，新增同一供应商的另一个模型需要重新走连接表单并再次输入 Token；编辑时模型复选框未绑定选中状态，用户看不出当前已启用哪些模型。

修复：

- 连接按供应商聚合展示：一个供应商一行，模型以 chips 展示；新增模型不再重复创建供应商连接。
- 每条未配置模型提供“启用”按钮；已配置供应商存在时，启用请求携带空 Token，服务端复用现有 K8s Secret，仅新增模型行。
- “管理供应商模型”弹窗：已启用模型显示选中与“已启用”，新模型可勾选；Token 留空沿用已保存凭据；编辑时供应商/协议锁定。
- 表内未配置且无供应商连接时提供“配置”，打开表单并预选该模型。
- 修正 DeepSeek 预置模型：`deepseek-v4.1-flash` → `deepseek-flash`，与 `GET /v1/models` 实际返回一致。
- 后端 `POST /model-connections` 支持空 Token：按 provider/protocol 读取既有 Secret 进行探测；Secret 不存在时仍返回 `token_required`。

## 模型验收

| 项目 | 值 |
|---|---|
| 供应商 | `deepseek`，1 个供应商连接 |
| 模型 | `deepseek-v4-pro`（ID `2902559098867712`）、`deepseek-flash`（ID `2902611112300544`） |
| 共享 Secret | `gp-model-1001-deepseek-api` |
| 空 Token 添加 Flash | 200 `{"status":"configured"}`，未重新写入 Secret |
| 项目绑定 | `im-saas` 仍为 `deepseek / deepseek-v4-pro` |
| 生成批次 | `2902560158584832`，Tag `2.2.1`，5 条候选通过质量校验，状态 `review` |

## 迁移与运行检查

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-1-86-snapshot` |
| 结果 | `Complete`，completionTime `2026-10-22T05:44:13Z` |
| Schema 版本 | `33` |
| Dirty | `false` |
| `/api/readyz` | 200，`database/redis/temporal=ok` |
| 登录 / `/api/auth/me` | 200 / 200 |
| `open-im-local` 快照核对 | 脚本内置检查通过 |

## 镜像清理

- Docker 本地清理 `0.1.85`，当前仅保留 `0.1.86`。
- 两个 kind 节点删除 `0.1.85` 的 server/web/migrate tag，当前仅保留 `0.1.86`。
- `0.3.x` 仍为空。

## 结论

`0.1.86-SNAPSHOT` 已通过唯一入口发布；模型配置改为“供应商一次配置、模型按需启用”，空 Token 复用 Secret 的路径已实测通过；DeepSeek 两个真实模型均已配置，项目绑定和生成链路保持正常。

# GreenPass kind 发布记录：0.1.88-SNAPSHOT（2026-10-22）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.1.88-SNAPSHOT`（`0.1.87` 之后的下一 patch） |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `4209d6427baa482c47c866a5bb5bf661e56db59c` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.88-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-22 14:13（Asia/Shanghai） |

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.1.88-SNAPSHOT` | `sha256:42ade7d5f3779c83cbbd130d8dc26378798132de553a1b189744a8bf77dc0198` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.1.88-SNAPSHOT` | `sha256:f89bf6a60d569edf4d99fe9fbeba0ee42f90b430628799841fbae92f96a9d218` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.1.88-SNAPSHOT` | `sha256:db97f78aefe5472add1f2cfe6bd0fe05389ad813e60c33dcdbb269bbe55824ab` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.1.88-SNAPSHOT`。

## 修复：移除硬编码模型目录

问题：`modelPresets` 把 GPT-6 Astra、GPT-5.6 Terra/Luna、Claude Opus 5.5、DeepSeek V4 Pro/V4.1 Flash、Qwen3.8 等模型写死在代码里，并非供应商官方列表；用户无法从真实模型中选择，且会展示不存在的版本。

修复：

- 删除硬编码模型列表；只保留 4 个供应商默认配置（OpenAI / Anthropic / DeepSeek / Qwen 的协议与 Base URL）。
- 新增 `POST /model-provider-models`：使用供应商 Token 调用官方 `GET /models`，返回真实模型 ID；Token 为空时复用该供应商已保存的 Secret。
- `POST /model-connections` 保存前重新读取官方列表，模型 ID 不在列表中返回 `invalid_model_selection`。
- 前端配置对话框改为“填写 Token → 获取官方模型列表 → 勾选模型”；不再展示内置模型版本。
- 模型表只展示已配置模型，模型 ID 来自 `mdl_model`；页面明确“模型 ID 来自供应商官方 `/models`”。
- DeepSeek 实际列表：`deepseek-flash`、`deepseek-v4-pro`。

## 验收

| 检查 | 结果 |
|---|---|
| `GET /model-catalog` | 返回 4 个供应商默认配置 + 2 个已配置模型 |
| 官方模型发现 | `POST /model-provider-models` 200，返回 `deepseek-flash`、`deepseek-v4-pro` |
| 空 Token 复用 | DeepSeek 发现与新增 Flash 均成功，未重写 Secret |
| 非法模型 | 保存非官方模型 ID 返回 400 `invalid_model_selection` |
| 页面 | 不再出现 GPT-6 / Claude Opus / Qwen3.8 等硬编码版本；对话框展示真实模型 |
| 项目绑定 | `im-saas` 仍为 `deepseek / deepseek-v4-pro` |
| 生成批次 | `2902560158584832`，Tag `2.2.1`，5 条候选通过质量校验，状态 `review` |

## 迁移与运行检查

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-1-88-snapshot` |
| 结果 | `Complete` |
| Schema 版本 | `33` |
| Dirty | `false` |
| `/api/readyz` | 200，`database/redis/temporal=ok` |
| 登录 / `/api/auth/me` | 200 / 200 |
| `open-im-local` 快照核对 | 脚本内置检查通过 |

## 镜像清理

- Docker 本地清理 `0.1.87`，当前仅保留 `0.1.88`。
- 两个 kind 节点删除 `0.1.87` 的 server/web/migrate tag，当前仅保留 `0.1.88`。
- `0.3.x` 仍为空。

## 结论

`0.1.88-SNAPSHOT` 已移除系统内置模型版本，模型列表改为供应商官方 `/models` 动态发现；DeepSeek 两个真实模型、项目绑定和生成链路均保持正常。

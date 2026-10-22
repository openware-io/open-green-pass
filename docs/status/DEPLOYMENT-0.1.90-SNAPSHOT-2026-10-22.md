# GreenPass kind 发布记录：0.1.90-SNAPSHOT（2026-10-22）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.1.90-SNAPSHOT` |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `c4c067588c0a69b81d466321dc5b92fb28594c29` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.90-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-22 14:31（Asia/Shanghai） |

`0.1.89` 是同一次信息架构重做的中间版本（供应商合并为一段），已被 `0.1.90` 的对齐表格取代；`0.1.89` 镜像已清理。

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.1.90-SNAPSHOT` | `sha256:42ade7d5f3779c83cbbd130d8dc26378798132de553a1b189744a8bf77dc0198` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.1.90-SNAPSHOT` | `sha256:ff6e96a383e89cb081e35378d68f704ced7380d7e36ee8ae1e6507fda223e0d1` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.1.90-SNAPSHOT` | `sha256:db97f78aefe5472add1f2cfe6bd0fe05389ad813e60c33dcdbb269bbe55824ab` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.1.90-SNAPSHOT`。

## 页面重做

原页面：4 张大供应商卡（0 个已配置也占整卡）+ “已配置供应商”重复一段 + “已配置模型”再重复一张表，同一信息出现三次，空卡和空表占满页面。

新页面：**单张供应商表**，一行一个供应商：

| 列 | 内容 |
|---|---|
| 供应商 | 图标、名称、API/CC 协议 |
| 状态 | 已连接/未配置 + Token 已保存/未保存 |
| Base URL | 已连接用实际地址；未配置用厂商默认地址 |
| 模型 | 已配置模型 chips（点击查看配置）；未配置显示“未配置模型” |
| 操作 | 已连接“管理模型”；未配置“配置” |

同时：

- 删除独立“已配置供应商”和“已配置模型”两段，避免同一数据重复三遍。
- 未配置供应商的按钮改为描边，不再整页绿色实心按钮。
- 模型 ID 仍来自供应商官方 `/models`，没有硬编码模型版本。
- “管理模型”弹窗保留官方模型发现和空 Token 复用 Secret。

## 验收

| 检查 | 结果 |
|---|---|
| 页面结构 | 只有一段“供应商”表，无“已配置供应商/已配置模型”旧标题 |
| 行数 | 4 个供应商：OpenAI、Anthropic、DeepSeek、Qwen |
| DeepSeek | 已连接、Token 已保存、2 个模型 chips、管理模型入口 |
| 未配置供应商 | 显示未配置、默认 Base URL、描边“配置”按钮 |
| 模型数据 | `deepseek-v4-pro`、`deepseek-flash`，均来自官方列表 |
| 空卡/空表 | 已删除 |

## 迁移与运行检查

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-1-90-snapshot` |
| 结果 | `Complete` |
| Schema 版本 | `33` |
| Dirty | `false` |
| `/api/readyz` | 200，`database/redis/temporal=ok` |
| 登录 / `/api/auth/me` | 200 / 200 |
| `open-im-local` 快照核对 | 脚本内置检查通过 |

## 镜像清理

- Docker 本地清理 `0.1.89`，当前仅保留 `0.1.90`。
- 两个 kind 节点删除 `0.1.88`、`0.1.89` 的 server/web/migrate tag，当前仅保留 `0.1.90`。
- `0.3.x` 仍为空。

## 结论

`0.1.90-SNAPSHOT` 已完成模型页信息架构重做：供应商、连接状态、Base URL、模型和操作收敛到一张表；官方模型发现与项目模型选择保持正常。

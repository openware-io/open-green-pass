# GreenPass kind 发布记录：0.1.84-SNAPSHOT（2026-10-22）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.1.84-SNAPSHOT`（`0.1.83` 之后的下一 patch） |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `11c9ebf2e4e44fe2c04cef67e9937f21959903ca` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.84-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-22 12:26（Asia/Shanghai） |

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.1.84-SNAPSHOT` | `sha256:b58c2457b565bebd575548cf40f83470db60586f5ccab50ce6344dfdaab0cfb2` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.1.84-SNAPSHOT` | `sha256:ef59695d4ab42a6b4a5dc26938906bc510a8dbe9cfb46adf72b6368dae68de50` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.1.84-SNAPSHOT` | `sha256:db97f78aefe5472add1f2cfe6bd0fe05389ad813e60c33dcdbb269bbe55824ab` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.1.84-SNAPSHOT`。

## 变更

- 生成页主操作由“批量生成用例”改为“生成用例”，仅选中服务时显示数量后缀，例如“生成用例（1）”，避免单选场景的批量语义歧义。
- 同步调整辅助文案：“为所选服务创建生成批次”“选择多个服务时会为每个服务创建独立批次”“确认创建”“可在上方加入生成范围”。
- 其余功能与 0.1.83 一致（SCM 加固、会话过期修复、Web 缓存策略）。

## 迁移与运行检查

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-1-84-snapshot` |
| 结果 | `Complete`，completionTime `2026-10-22T04:26:34Z` |
| Schema 版本 | `33` |
| Dirty | `false` |
| `/` | 200 |
| `/api/healthz` | 200 |
| `/api/readyz` | 200，`database/redis/temporal=ok` |
| 登录 / `/api/auth/me` | 200 / 200 |
| `open-im-local` 快照核对 | 脚本内置检查通过 |

## UI 验收

- 生成页按钮文本为 **“生成用例”**，不再出现“批量生成用例”。
- 勾选 1 个服务时按钮为 **“生成用例（1）”**；代码使用数量后缀动态渲染。
- 辅助文案不再使用“批量操作”的歧义表达。
- 浏览器强制重新加载后截图确认；页面无 `forbidden`，分支 `develop/2.2.0-auth` 可选。

## SCM 验收

| 检查 | 结果 |
|---|---|
| 连接 | `im-saas-readonly-20261021`（`2900333722734592`），`verified` |
| 仓库绑定 | 8/8 `verified` |
| `open-chat-admin` 分支 | `refs:refresh` 200，4 分支 + 3 Tag |

## 镜像清理

- `init-kind.ps1` 已清理 Docker 本地 `0.1.83` tag，仅保留 `0.1.84`。
- 两个 kind 节点已删除 `0.1.83` 的 server/web/migrate tag；当前仅保留 `0.1.84` 对应 tag。
- 非规范 `0.3.x` 仍保持为空（上一轮已清理）。

## 过程说明

- 本次为 0.1.83 之后的一次 patch 发布，未跳 minor 版本。
- `local-build` 未推送 GHCR；分支 `fix/scm-pat-auth-type` 未合并/推送。
- 主机使用 PowerShell `7.6.7` 执行唯一入口脚本。

## 结论

`0.1.84-SNAPSHOT` 已完成构建、迁移、发布、readiness/login、UI 文案和 SCM 8 仓库验收；版本序列保持 `0.1.83 -> 0.1.84`。

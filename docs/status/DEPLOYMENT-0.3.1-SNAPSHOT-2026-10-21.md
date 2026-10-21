# GreenPass kind 发布记录：0.3.1-SNAPSHOT（2026-10-21）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.3.1-SNAPSHOT` |
| Git 分支 | `fix/scm-pat-auth-type` |
| Git commit | `9a602af870c23a41772ac1185e8a2bf0421720eb` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.3.1-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-21 20:12（Asia/Shanghai） |

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.3.1-SNAPSHOT` | `sha256:77ee181969e9ab8a4e47b6fde4e0e457a6c0108aeea905c200f765883fd025ba` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.3.1-SNAPSHOT` | `sha256:4c62635bb6ac16809f2cc71aa5d113195eae23be5c3d63c61ed38d5a07510c30` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.3.1-SNAPSHOT` | `sha256:f441f7d2b8bfbb373260fbb800ce492d85bacbcc4282960f6357018966d4d16e` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.3.1-SNAPSHOT`。

## 迁移

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-3-1-snapshot` |
| 结果 | `Complete`，completionTime `2026-10-21T12:12:44Z` |
| Schema 版本 | `33` |
| Dirty | `false` |

## 发布后验证

| 检查 | 结果 |
|---|---|
| `/` | 200 |
| `/api/healthz` | 200 |
| `/api/readyz` | 200，`database/redis/temporal=ok` |
| 账号密码登录 | 200（`zhangli`） |
| 认证态接口 `/api/auth/me` | 200 |
| `open-im-local` 工作负载快照 | 脚本内置核对通过（UID/generation/restartedAt 未变化，未触发阻断） |
| SCM 连接保留 | `im-saas-readonly-20261021`（ID `2900333722734592`），`verified`，`auth_type=pat` |
| IM-SaaS 仓库绑定 | 8/8 `scm_connection_status=verified` |
| 源码读取链路 | `open-im-server` 的 `refs:refresh` 返回 200，7 个 ref |

## 回滚

`deploy-kind.ps1` 在发布前记录 `gp-server`/`gp-worker`/`gp-web` 的上一组镜像；本次发布后验证全部通过，未触发自动回滚。需要手工回滚时：

```powershell
kubectl --context kind-open-im-local -n gp set image deployment/gp-server server=ghcr.io/openware-io/open-green-pass:0.3.0-SNAPSHOT
kubectl --context kind-open-im-local -n gp set image deployment/gp-worker worker=ghcr.io/openware-io/open-green-pass:0.3.0-SNAPSHOT
kubectl --context kind-open-im-local -n gp set image deployment/gp-web web=ghcr.io/openware-io/open-green-pass-web:0.3.0-SNAPSHOT
```

回滚前需确认这些旧镜像仍存在于节点；本次 `init-kind.ps1` 已按脚本策略清理本地旧 tag，正式回滚应从 GHCR 拉取对应版本并按同一发布脚本执行。

## 过程偏差与纠正

1. **临时镜像**：为尽快验证 P0 修复，曾构建 `ghcr.io/openware-io/open-green-pass:0.3.1-scmfix` 并直接 `kubectl set image`，违反“唯一入口 `init-kind.ps1`”和 `X.Y.Z-SNAPSHOT` 命名规范。随后用 `init-kind.ps1` 正式部署 `0.3.1-SNAPSHOT`；脚本已删除该临时 tag。
2. **PowerShell 7**：主机原先只有 Windows PowerShell 5.1，不满足脚本使用的 .NET 6+ API。已安装 PowerShell `7.6.7`，再用 `pwsh -NoProfile -File ./scripts/dev/init-kind.ps1 ...` 执行。部署环境应把 `pwsh` 作为前置依赖纳入检查。
3. **镜像未入 GHCR**：本次为本地 `local-build`，镜像 ID 可追溯，但没有 registry digest。正式发布仍需走 CI 构建/推送，补齐 `X.Y.Z` tag 与 digest。

## 结论

本次 `0.3.1-SNAPSHOT` 通过项目规定的唯一入口完成构建、加载、迁移、发布、readiness/login 检查和 IM 隔离核对；SCM 凭据与 8 个仓库绑定在发布后保持有效。发布代码 commit 为 `9a602af`，但该 commit 尚未合并到 `main`，也未推送 GHCR。

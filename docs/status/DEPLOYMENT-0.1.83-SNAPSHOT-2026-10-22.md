# GreenPass kind 发布记录：0.1.83-SNAPSHOT（2026-10-22）

## 版本修正

本项目开发镜像沿用 `0.1.x-SNAPSHOT` 序列；2026-10-16 的浏览器验收基线是 `0.1.81` / `0.1.82-SNAPSHOT`。此前出现的 `0.3.0`～`0.3.5` 是对 minor 版本的越级跳版，不符合版本规范，不能作为发布基线。

本次已恢复原序列，发布 **`0.1.83-SNAPSHOT`**（`0.1.82` 之后的下一 patch），并删除 Docker 与 kind 两个节点上的全部 `0.3.x` 镜像 tag。

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.1.83-SNAPSHOT` |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `0566039e74b4cedd6798338282ee6a83645e4aae` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.83-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-22 11:59（Asia/Shanghai） |

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.1.83-SNAPSHOT` | `sha256:b58c2457b565bebd575548cf40f83470db60586f5ccab50ce6344dfdaab0cfb2` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.1.83-SNAPSHOT` | `sha256:3ab5629845e3ffa5001c27f4e94955b9a97d313e2273e0628f78d2d785e36514` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.1.83-SNAPSHOT` | `sha256:db97f78aefe5472add1f2cfe6bd0fe05389ad813e60c33dcdbb269bbe55824ab` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.1.83-SNAPSHOT`。

## 迁移与运行检查

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-1-83-snapshot` |
| 结果 | `Complete`，completionTime `2026-10-22T03:59:33Z` |
| Schema 版本 | `33` |
| Dirty | `false` |
| `/` | 200 |
| `/api/healthz` | 200 |
| `/api/readyz` | 200，`database/redis/temporal=ok` |
| 登录 / `/api/auth/me` | 200 / 200 |
| `open-im-local` 快照核对 | 脚本内置检查通过（未触发 IM workload changed） |

## SCM 验收

| 检查 | 结果 |
|---|---|
| 现有连接 | `im-saas-readonly-20261021`（ID `2900333722734592`），`verified`，`auth_type=pat` |
| 仓库绑定 | 8/8 `scm_connection_status=verified` |
| 重新验证 | 200，耗时 9.5s；遍历 8 个绑定仓库 |
| `open-chat-admin` 分支 | `refs:refresh` 200，4 分支 + 3 Tag；页面显示 `develop/2.2.0-auth` 和 SHA `a44876435f69` |
| 页面 | 8 个“源码连接已验证”；生成页分支/Tag 可选，无 `forbidden` |
| 可信审计 | create/verify/bind/delete/rotate 事件均存在；verify 的 payload `bound_repository_count=8` |

## 会话过期修复

用户页面停留超过 12 小时后 `gp_session` 过期，开发环境的 header fallback 会让只读页面继续显示，但 `catalogEditor` 返回 403，表现为“分支尚未同步 + forbidden”。本次修复：

- `/auth/me` 返回 401 时清空前端登录态；
- `ProtectedLayout` 在页面重新可见或每 5 分钟校验会话，过期自动回登录页；
- 生成页刷新分支/Tag 遇到 401/403 时先校验会话，过期跳登录页，角色不足则显示明确提示。

浏览器实测：重新登录后点击“刷新分支 / Tag”，下拉出现 `develop/2.2.0-auth`、`develop/2.3.0`、`develop/2.4.0-hotel`、`main`，无 `forbidden`。

## 镜像清理

已在 Docker 与 kind 节点 `open-im-local-worker`、`open-im-local-control-plane` 删除以下非规范 tag：

- `open-green-pass:0.3.0-SNAPSHOT` ～ `0.3.5-SNAPSHOT`
- `open-green-pass:0.3.1-scmfix`
- `open-green-pass-web:0.3.0-SNAPSHOT` ～ `0.3.5-SNAPSHOT`
- `open-green-pass-migrate:0.3.0-SNAPSHOT` ～ `0.3.5-SNAPSHOT`

清理后两个节点仅保留 `0.1.83-SNAPSHOT` 的 server/web/migrate tag；Docker 本地同样仅保留 `0.1.83-SNAPSHOT`。

## 过程说明

1. 为修正版本号，先按唯一入口重新发布 `0.1.83-SNAPSHOT`，再清理旧 `0.3.x` 镜像；未直接替换运行中 Pod。
2. 清理 kind 节点 tag 使用 `ctr -n k8s.io images rm <tag>`，先确认删除共享 image ID 的 tag 不会移除 `0.1.83` tag，再批量执行。
3. 主机使用 PowerShell `7.6.7` 执行脚本；`local-build` 未推送 GHCR。
4. 分支 `fix/scm-pat-auth-type` 未合并/推送。

## 结论

`0.1.83-SNAPSHOT` 已恢复项目 `0.1.x` 版本序列，并通过唯一入口完成构建、加载、迁移、发布、readiness/login、IM 隔离和 SCM 8 仓库验收；非规范 `0.3.x` 镜像已从 Docker 与 kind 节点清理。

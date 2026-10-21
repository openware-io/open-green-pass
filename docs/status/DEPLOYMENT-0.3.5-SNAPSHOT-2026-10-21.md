# GreenPass kind 发布记录：0.3.5-SNAPSHOT（2026-10-21）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.3.5-SNAPSHOT` |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `1b7dd8bdc541bb482f405abdf6a29353aaef6937` |
| SCM 代码提交 | `aeced3f`（加固）、`229284d`（verify 全仓库）、`1b7dd8b`（secondary rate limit 重试） |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.3.5-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-21 20:52（Asia/Shanghai） |

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.3.5-SNAPSHOT` | `sha256:b58c2457b565bebd575548cf40f83470db60586f5ccab50ce6344dfdaab0cfb2` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.3.5-SNAPSHOT` | `sha256:dca17ea3d0873319ce1a0213b2fa2b674c2dc1ee05816c8bf92e996975b9a45f` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.3.5-SNAPSHOT` | `sha256:db97f78aefe5472add1f2cfe6bd0fe05389ad813e60c33dcdbb269bbe55824ab` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.3.5-SNAPSHOT`。

## 迁移与运行检查

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-3-5-snapshot` |
| 结果 | `Complete`，completionTime `2026-10-21T12:52:07Z` |
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
| 重新验证（0.3.5） | 200，耗时 16.8s；遍历 8 个绑定仓库的 Metadata + 根 Contents |
| 重新验证审计 | `scm.connection.verified`，payload `{provider=github, auth_type=pat, bound_repository_count=8}` |
| 源码读取 | `open-im-server` 的 `refs:refresh` 200，7 个 ref |
| 生命周期闭环 | 0.3.2/0.3.3 阶段已验证：临时连接创建→绑定 `open-website`→轮换→删除；临时 Secret 删除，生产连接与 8 仓库绑定恢复 |
| 页面 | 8 个“源码连接已验证”；SCM 弹窗显示 PAT 表单、重新验证/轮换/删除 |

## 瞬时限流与重试

0.3.4 首次对生产连接执行“重新验证”时，Token 探测遇到一次 GitHub 瞬时限流（HTTP 403/429，5.3s 失败），连接被短暂标记为 `failed`；立即重试成功（12.9s，8/8 恢复）。0.3.5 已补充 **403 + `Retry-After`/`X-RateLimit-Reset` 的 secondary rate limit 重试**，并在 0.3.5 验证中一次通过（16.8s）。

## Web 缓存与发布可见性

- `/` 与 `index.html`：`Cache-Control: no-cache, must-revalidate`
- `/assets/<hashed>.js`：`Cache-Control: public, max-age=31536000, immutable`
- 本次 bundle：`index-BwH6v0Le.js`
- 说明：发布前已缓存旧 `index.html` 的浏览器需要强制刷新一次；之后会按 no-cache 获取新 shell。

## 过程偏差与说明

1. `0.3.2`、`0.3.3`、`0.3.4` 是中间 SNAPSHOT 发布；`0.3.5` 为最终发布版本。`init-kind.ps1` 每次都会清理旧 tag。
2. 0.3.5 构建首次因 `gcr.io` 拉取 distroless 基础镜像 EOF 失败；重试 `docker pull` 成功后，用同一唯一入口重新发布，未改用临时镜像或直连替换。
3. 主机原先只有 Windows PowerShell 5.1，无法执行脚本使用的 .NET 6+ API；已安装 PowerShell `7.6.7` 作为部署前置依赖。
4. 本次仍是 `local-build`，镜像 ID 可追溯但未推送 GHCR，也没有 registry digest；正式发布需走 CI 构建/推送。
5. 分支 `fix/scm-pat-auth-type` 未合并到 `main`，未推送远端；需要评审/PR 后合入。

## 回滚

`deploy-kind.ps1` 在发布前记录上一组镜像；本次所有验证通过，未触发自动回滚。由于脚本清理了旧本地 tag，回滚到 `0.3.4` 需要从 GHCR 拉取或重新构建，再走同一发布入口。

## 结论

`0.3.5-SNAPSHOT` 通过项目唯一入口完成构建、加载、迁移、发布、readiness/login、IM 隔离检查；SCM 加固功能（错误处理、校验、审计、删除/轮换、前端交互、分页/退避、verify 全仓库探测、secondary limit 重试）已在真实 kind 环境验证。发布 checkout 为 `1b7dd8b`，尚未合并/推送。

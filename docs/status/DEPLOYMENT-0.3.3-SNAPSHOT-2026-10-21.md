# GreenPass kind 发布记录：0.3.3-SNAPSHOT（2026-10-21）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.3.3-SNAPSHOT` |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `5e994b67b93d76e5d9997e7a10193ff21bc2844a` |
| SCM 加固代码提交 | `aeced3f` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.3.3-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-21 20:42（Asia/Shanghai） |

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.3.3-SNAPSHOT` | `sha256:15f0db4f880b1194d5e5ab46d4c2ca691672eb9f94436d0c031280602efef0d1` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.3.3-SNAPSHOT` | `sha256:dca17ea3d0873319ce1a0213b2fa2b674c2dc1ee05816c8bf92e996975b9a45f` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.3.3-SNAPSHOT` | `sha256:f441f7d2b8bfbb373260fbb800ce492d85bacbcc4282960f6357018966d4d16e` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.3.3-SNAPSHOT`。

## 迁移与运行检查

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-3-3-snapshot` |
| 结果 | `Complete`，completionTime `2026-10-21T12:42:18Z` |
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
| `open-im-server` 源码读取 | `refs:refresh` 200，7 个 ref |
| 重新验证 | 200，并产生 `scm.connection.verified` 审计 |
| 可信审计 | 观察到 created / verified / bound / rotated / deleted 事件 |
| 临时连接闭环 | 创建 `2900594650091520` → 绑定 `open-website` → 轮换 → 删除；临时 Secret 已删除，生产连接与 8 仓库绑定已恢复 |
| 页面 | 8 个“源码连接已验证”；SCM 弹窗显示 PAT 表单、重新验证/轮换/删除 |

## Web 缓存与发布可见性

- `/` 与 `index.html`：`Cache-Control: no-cache, must-revalidate`
- `/assets/<hashed>.js`：`Cache-Control: public, max-age=31536000, immutable`
- 本次 bundle：`index-BwH6v0Le.js`
- 说明：发布前浏览器可能缓存了旧 `index.html`；升级后首次访问建议强制刷新一次，之后会按 no-cache 正常获取新 shell。

## 过程偏差与说明

1. `0.3.2-SNAPSHOT` 是中间发布（SCM 加固代码），随后发现 Web shell 缓存导致旧 bundle 仍可被加载，补充 nginx 缓存策略后以 `0.3.3-SNAPSHOT` 重新发布；`0.3.3` 为最终发布版本，`init-kind.ps1` 已清理 `0.3.2` 本地 tag。
2. 主机原先只有 Windows PowerShell 5.1，无法执行脚本使用的 .NET 6+ API；已安装 PowerShell `7.6.7` 作为部署前置依赖。
3. 本次仍是 `local-build`，镜像 ID 可追溯但未推送 GHCR，也没有 registry digest；正式发布需走 CI 构建/推送。
4. 分支 `fix/scm-pat-auth-type` 未合并到 `main`，未推送远端；需要评审/PR 后合入。

## 回滚

`deploy-kind.ps1` 在发布前记录上一组镜像；本次所有验证通过，未触发自动回滚。由于 `init-kind.ps1` 已按策略清理 `0.3.2` 本地 tag，若要回滚到 `0.3.2`，需要从 GHCR 拉取或重新构建对应版本，再按同一脚本发布。

## 结论

`0.3.3-SNAPSHOT` 通过项目唯一入口完成构建、加载、迁移、发布、readiness/login、IM 隔离检查；SCM 加固功能（错误处理、校验、审计、删除/轮换、前端交互、分页/退避）已在真实 kind 环境验证。发布代码 commit 为 `5e994b6`（SCM 代码 `aeced3f`），尚未合并/推送。

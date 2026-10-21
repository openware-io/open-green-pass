# SCM 源码读取凭据接入问题清单（2026-10-21）

## 结论先行

- 环境：`http://greenpass.localhost:30080`（kind `open-im-local`，命名空间 `gp`），team `1001`。
- 项目：`im-saas`（`2876690473259008`），8 个仓库、27 个目录服务。
- GitHub App：未配置（`GET /api/scm-connections/github-app` 返回 409）。本机回调只有 HTTP，不满足 GitHub App 的公网 HTTPS 回调要求，因此本次使用系统保留的 fine-grained PAT 兼容入口。
- 本次已完成：创建只读 fine-grained PAT 连接 `im-saas-readonly-20261021`（连接 ID `2900333722734592`，`auth_type=pat`，`credential_status=verified`），并将 8 个仓库全部绑定到该连接；`GET /api/projects/.../repositories` 返回 8/8 `verified`。
- 端到端验证：对 `open-im-server` 调用 `POST /api/projects/.../repositories/2876819928027136/refs:refresh` 返回 200，读取到 4 个分支和 3 个 Tag，证明连接和 Secret 解析链路可用。
- 过程中发现并**已修复**一个 P0：PAT 连接创建必然失败（`auth_type` 空字符串违反数据库 CHECK，接口却返回 409）。修复为显式写入 `AuthType: "pat"`，提交为 `9a602af`（分支 `fix/scm-pat-auth-type`）。
- 部署规范纠正：首次为尽快验证曾临时构建 `0.3.1-scmfix` 并直接 `kubectl set image`，违反“唯一入口 `init-kind.ps1`”和 `X.Y.Z-SNAPSHOT` 命名规范。现已改由 `scripts/dev/init-kind.ps1 -Version 0.3.3-SNAPSHOT -ImageSource local-build` 重新部署，server/worker/web/migrate 全部为 `0.3.3-SNAPSHOT`，临时 tag 已由脚本清理；发布记录见 [`DEPLOYMENT-0.3.1-SNAPSHOT-2026-10-21.md`](DEPLOYMENT-0.3.1-SNAPSHOT-2026-10-21.md) 与 [`DEPLOYMENT-0.3.3-SNAPSHOT-2026-10-21.md`](DEPLOYMENT-0.3.3-SNAPSHOT-2026-10-21.md)。

## 问题清单

| ID | 优先级 | 领域 | 问题与证据 | 影响 | 建议处理 | 状态 |
|---|---|---|---|---|---|---|
| SCM-01 | P0 | 连接创建 | `POST /scm-connections` 构造 `infra.SCMConnection` 时未设置 `AuthType`，插入时 `auth_type=''` 违反 `repo_scm_connection` 的 `CHECK (auth_type IN ('pat','github_app'))`。实测：Secret 已创建、DB 无行、接口返回 `409 scm_connection_name_conflict`。 | 所有 PAT 连接都无法创建，功能实际不可用。 | 已显式设置 `AuthType: "pat"`，提交 `9a602af`，并按规范部署 `0.3.1-SNAPSHOT`；补一条接口级回归测试并走 PR 合并。 | 已修复；0.3.3 已发布；待评审/合并 |
| SCM-02 | P0 | 错误处理/一致性 | 同一处 `store.Create` 的任何错误都被映射为 `409 scm_connection_name_conflict`；且 Secret 先于 DB 插入创建，插入失败后留下孤儿 Secret。本次已手工删除 `gp-scm-1001-2900329347747840`。 | 运维无法从状态码区分重名、RLS、约束、DB 故障；泄漏 Secret 且难清理。 | 仅对唯一键冲突返回 409，其余返回 500 并记录去敏错误；插入失败时删除刚创建的 Secret，或改为可补偿的事务/outbox 流程；补集成测试。 | 已修复（错误映射+Secret 清理+delete RBAC），随 0.3.3 发布 |
| SCM-03 | P1 | 前端状态 | `TargetPage` 的 `scmConnections` 初始为空，只在打开“源码读取连接”弹窗时加载；仓库行和绑定下拉在页面加载后仍显示“尚未配置/未绑定”，即使后台已有已验证连接。 | 用户误以为凭据没保存，重复录入；绑定入口不可用。 | 在页面/项目切换时加载连接；区分 loading/error/empty；绑定下拉使用同一份已加载数据。 | 已修复（页面加载即拉取连接+独立弹窗组件），随 0.3.3 发布 |
| SCM-04 | P1 | 可发现性 | PAT 录入入口藏在 `<details>` 的“兼容方式：fine-grained PAT”中；GitHub App 未配置时只显示“尚未由平台管理员配置”，没有直接引导到 PAT。 | 新用户不知道该在哪里粘贴 Token，正是本次“怎么录入”的直接原因。 | 未配置 GitHub App 时，把“使用只读 PAT”提升为主操作；附创建 PAT 的最短路径和权限说明；配置 App 后再把 App 作为推荐路径。 | 已修复（未配置 App 时 PAT 表单默认可见），随 0.3.3 发布 |
| SCM-05 | P1 | 校验强度 | `probeGitHubReadToken` 只请求 `GET https://api.github.com/rate_limit`，不检查 Token 对目标仓库的 Contents/Metadata 读取权限，也不拒绝 classic Token 的写/管理 scope。 | 无仓库权限的 Token、甚至带 `admin:org` 的 Token 也会显示“已验证”；违反最小权限与“已验证”语义。 | 创建时校验至少一个目标仓库的 Metadata + Contents 读取；检查 `X-OAuth-Scopes`，拒绝写/管理 scope；绑定仓库时再做一次仓库级探测。 | 已修复（拒绝 classic scope；绑定时校验仓库 Contents），随 0.3.3 发布 |
| SCM-06 | P1 | 生命周期 | 没有 `DELETE /scm-connections/{id}`，UI 也没有删除/轮换入口；撤销 Token 或 Secret 只能手工改 DB/K8s。 | 泄露、过期、离职、误录的凭据无法在平台内安全下线；轮换不可审计。 | 增加删除/轮换 API、Secret 清理、解除仓库绑定、审计事件；轮换采用“新建-验证-切换-撤销旧”流程。 | 已修复（DELETE + rotate API/UI + 解绑+Secret 清理），随 0.3.3 发布 |
| SCM-07 | P1 | 可信审计 | SCM 连接创建、验证、绑定、失败均未写入 trusted audit；`internal/trusted` 与 `internal/governance/application` 中没有 SCM 事件。 | 凭据变更不可追溯，和 GP4 对“凭证变更必须审计”的要求不一致。 | 为 `scm.connection.created/verified/bound/deleted` 定义审计事件，写 actor、team、connection_id、repo_id、结果，不写 Token。 | 已修复（create/verify/bind/delete/rotate 写 trusted audit），随 0.3.3 发布 |
| SCM-08 | P1 | 文档一致性 | `docs/runbooks/GITHUB-APP-CONNECTION-RUNBOOK.md` 写“平台不会接收个人 Token”，但代码/UI 把 PAT 作为一等兼容路径；README 仍称“前端原型、默认 mock、原型阶段不做后端业务”，与当前 Go 后端、真实 SCM/生成能力不符。 | 运维和安全评审对“是否允许 PAT”产生分歧；新成员按 README 无法理解真实运行方式。 | 明确 PAT 的适用范围（本地/无 HTTPS 回调/存量兼容）和安全边界；更新 README 的定位、运行方式、真实 API/SCM 章节；在 `docs/README.md` 索引补充 SCM 文档。 | 本轮已更新接入手册/问题清单；README 待统一 |
| SCM-09 | P1 | 部署约束 | GitHub App 的 Setup URL / OAuth 回调必须是 GitHub 可访问的 HTTPS 地址；当前只有 `http://greenpass.localhost:30080`。本次未配置 App，改用 PAT。 | 本地/内网环境无法走推荐的 GitHub App 路径；只能使用 PAT。 | 提供受控 HTTPS 隧道或开发回调方案；在 UI 中明确“当前环境不支持 App，原因是什么”；不要把 `greenpass.localhost` 当作可用的 GitHub 回调。 | 已记录，需环境/隧道方案（非代码修复） |
| SCM-10 | P2 | App 连接可读性 | `installation_account_login` 字段从未写入；App 连接名称固定为“GitHub App 安装 #id”。 | 多组织/多安装时无法从列表识别连接归属。 | 在 OAuth 回调中用 installation 详情写入 `installation_account_login`，列表展示账号/组织名。 | 已修复（写入 installation_account_login 并展示），随 0.3.3 发布 |
| SCM-11 | P2 | 配置完整性 | `githubApp.configured()` 不要求 `GP_GITHUB_APP_FRONTEND_URL`；缺失时 OAuth 成功回调返回 JSON，而不是回到前端。 | 用户完成授权后停留在 API JSON 页面。 | 把 FrontendURL 纳入配置校验，或提供安全的默认回跳页并提示管理员补配置。 | 已修复（FrontendURL 纳入 configured 校验），随 0.3.3 发布 |
| SCM-12 | P2 | 身份模型 | GitHub App state 的 actor 使用 `parsePositiveID(principal.Subject)`；OIDC/微信等非数值 subject 会变成 0，可能造成同团队内 state 归属碰撞。 | 真实身份接入后授权状态可能串用户。 | 用字符串 actor ID 或稳定哈希存储；补非数值 subject 的测试。 | 已修复（非数值 subject 稳定哈希 actor），随 0.3.3 发布 |
| SCM-13 | P2 | 绑定校验 | `BindSCMConnection` 只校验连接属于同团队且状态为 configured/verified，不校验该连接能否读取目标仓库。 | 可把 A 仓库的 Token 绑到 B 仓库，直到刷新/生成时才失败。 | 绑定前调用目标仓库的 Metadata/Contents 探测；失败明确提示仓库级权限不足。 | 已修复（绑定前校验目标仓库 Contents），随 0.3.3 发布 |
| SCM-14 | P2 | 状态语义 | 仓库查询只把 `configured/verified` 视为已绑定；连接变成 `failed` 后仓库显示“missing/未绑定”，而不是“连接失效”。 | 故障被误判为未配置，增加排查成本。 | 在仓库投影中保留 `failed` 状态和失败摘要，UI 显示“连接验证失败/重新验证”。 | 已修复（仓库行展示“验证失败”），随 0.3.3 发布 |
| SCM-15 | P2 | 大仓库/限流 | `ListRefs` 只取 `per_page=100`，没有分页；SCM 请求没有限流/退避/条件请求处理。 | 分支/Tag 超过 100 的仓库会截断；接近 GitHub 限流时直接失败。 | 增加分页、`Link` 解析、429/403 退避和可观测指标；对超大仓库给出明确上限。 | 已修复（分页+429/5xx 退避），随 0.3.3 发布 |
| SCM-16 | P2 | 发布流程 | `scripts/dev/deploy-kind.ps1` 不创建/保留 GitHub App 配置；`gp-app.yaml` 仍使用 `VERSION` 占位。本次曾临时用 `0.3.1-scmfix` 直接替换镜像，违反“唯一入口 + X.Y.Z-SNAPSHOT”规范；已改用 `init-kind.ps1` 重新部署 `0.3.1-SNAPSHOT` 并清理临时 tag。 | 重新初始化集群或按清单重部署会丢失 App 配置；若继续容忍临时 tag/直连替换，会破坏发布可追溯性。 | 把 App ID/Client/私钥 Secret 引用纳入受控配置；在部署脚本中显式保留已有 Secret；CI/评审阻止临时 tag 和非脚本部署。 | 本次已纠正；App 配置注入仍待固化 |
| SCM-17 | P2 | 本次操作安全 | 为完成录入，PAT 在会话聊天中被传递；该 Token 是 fine-grained、仅公共仓库只读、默认 30 天过期，但仍应视为已暴露。 | 会话/日志留存可能造成凭据泄露。 | 完成验收后轮换该 PAT：新建 Token → 更新平台 Secret → 重新验证 → 在 GitHub 撤销旧 Token；不要把 Token 写入文档、脚本或提交。 | 待处理（需用户轮换 Token） |
| SCM-18 | P2 | 重新验证范围 | `POST /scm-connections/{id}/verify` 只验证 Token 本身，不遍历该连接已绑定的仓库。 | 仓库权限被收回后，连接仍显示“已验证”，直到下次刷新/生成/绑定才失败。 | verify 时遍历绑定仓库做 Contents 探测；任一失败则标记连接 failed 并列出仓库。 | 未修复（后续） |
| SCM-19 | P2 | 审计一致性 | SCM 审计写入是 best-effort：审计失败只记日志，凭据变更已生效。 | trusted audit 不可用时会出现“已变更但无审计”。 | 把审计与凭据变更放入可补偿流程，或在审计不可用时 fail-closed 并提示重试。 | 未修复（后续） |
| SCM-20 | P2 | 发布可见性 | nginx 未给 `index.html` 设置 `Cache-Control`；发布后浏览器可能继续用旧 shell，看不到新的 SCM 弹窗/按钮。本次验收已实际遇到，需 cache-bust 才加载新 bundle。 | 用户误以为修复未发布。 | `index.html` 使用 `no-cache, must-revalidate`；`/assets/` 使用 immutable 长缓存；发布验收前强制刷新一次。 | 已修复（web.conf），随 0.3.3-SNAPSHOT 发布 |

## 本次操作记录（不含 Token）

1. 确认 `GET /api/scm-connections/github-app` 返回 `409 github_app_not_configured`，`GET /api/scm-connections` 为空。
2. 使用 fine-grained PAT 对 8 个仓库逐一执行 GitHub API Metadata + Contents 读取，均返回 200。
3. 首次 `POST /api/scm-connections` 返回 `409`，但 DB 无记录、K8s 出现 `gp-scm-1001-2900329347747840`。定位为 `auth_type` 空字符串违反 CHECK；删除孤儿 Secret。
4. 修复 `internal/governance/api/scm_connection_http.go`，`gofmt`，`go test -count=1 ./internal/governance/...` 通过，提交 `9a602af`（分支 `fix/scm-pat-auth-type`）。
5. 先临时用 `0.3.1-scmfix` 直连 `kubectl set image` 完成验证；随后按规范执行 `pwsh ./scripts/dev/init-kind.ps1 -Version 0.3.1-SNAPSHOT -ImageSource local-build`，server/worker/web/migrate 全部部署为 `0.3.1-SNAPSHOT`，迁移 Job `gp-migrate-0-3-1-snapshot` 完成，脚本的 IM 隔离/readiness/login 检查通过。
6. 重新创建连接：`201`，ID `2900333722734592`，`auth_type=pat`，`credential_status=verified`。
7. 对 8 个仓库执行 `PATCH /api/projects/2876690473259008/repositories/{repoID}/scm-connection`，全部 `200 {"status":"bound"}`。
8. 复核 `GET /api/projects/2876690473259008/repositories`：8/8 `scm_connection_status=verified`。
9. 对 `open-im-server` 执行 `refs:refresh`：200，返回 7 个 ref（4 分支 + 3 Tag）。
10. 复核 K8s Secret `gp-scm-1001-2900333722734592` 存在；数据库只保存 `k8s://gp/...` 引用，不保存 Token。
11. 浏览器复核“被测对象”页：8 个仓库均显示“源码连接已验证”，项目统计仍为 8 仓库 / 27 服务。

## 2026-10-21 SCM 加固批次（0.3.3-SNAPSHOT）

代码提交 `aeced3f`（分支 `fix/scm-pat-auth-type`），覆盖：

- 后端：错误映射改为仅唯一键冲突 409，其余 500；Secret 创建失败/DB 插入失败双向清理；新增 `DELETE /scm-connections/{id}` 与 `POST /scm-connections/{id}/rotate`；删除时先解绑仓库，PAT Secret 清理、App 共享私钥不删除；K8s Role 增加 `delete`。
- 校验：PAT 创建改为 `GET /user` + `X-OAuth-Scopes` 检查，拒绝 classic/写权限 Token；绑定仓库前用目标仓库 Metadata + Contents 做仓库级探测，失败返回 422 `repository_not_readable`。
- 审计：create/verify/bind/delete/rotate 写 trusted audit，payload 不含 Token；审计失败记录日志但不阻断已完成的凭据变更（残留风险见 SCM-18）。
- App：`configured()` 要求 `GP_GITHUB_APP_FRONTEND_URL`；OAuth 回调写入 `installation_account_login`；state actor 对非数值 subject 使用稳定哈希。
- 读取：分支/Tag 分页（最多 1000 个，超限显式失败）；429/5xx 有限退避重试。
- 前端：新增 `SCMConnectionDialog`；页面加载即拉取连接；未配置 App 时 PAT 表单默认可见；支持重新验证、轮换、删除；失败连接显示“源码连接验证失败”。
- 测试：`internal/governance/api`、`internal/ai/infra` 新增单元测试（in-memory transport，规避本机 TCP 隔离）；`go test -count=1 ./internal/... ./cmd/... ./pkg/...`、`go vet`、`npm run typecheck`、`npm run lint:eslint`、`npm run build`、`npm run api:check` 通过。

## 残留问题

- SCM-18：`verify` 只重新验证 Token 本身，不遍历该连接已绑定的全部仓库；已绑定仓库的权限收回要等到下次 `refs:refresh`/生成/重新绑定才暴露。建议后续让 verify 校验所有绑定仓库。
- SCM-19：审计写入是 best-effort；若 trusted audit 不可用，凭据变更已生效但审计缺失。若要求 fail-closed，需要把审计与凭据变更放入可补偿事务。
- SCM-16：GitHub App 的 App ID/Client/私钥 Secret 仍未纳入 `deploy-kind.ps1` 的受控配置注入；本地部署仍以 PAT 为主。
- SCM-17：本次会话中传递的 PAT 仍需人工轮换。

## 建议的处理顺序

1. 发布 `0.3.3-SNAPSHOT` 并复核 8 个仓库仍为 `verified`。
2. 用临时连接验证创建→绑定→轮换→解绑→删除的完整闭环。
3. 统一 README / GitHub App 手册的 PAT 口径（SCM-08），并补 SCM-18/19 的后续任务。

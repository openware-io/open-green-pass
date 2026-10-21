# IM-SaaS 源码读取凭据接入手册

适用对象：GreenPass 管理员/团队 owner；目标是把 `im-saas` 项目的代码仓库绑定到一条已验证的 GitHub 只读连接。

本文以 2026-10-21 的本地 kind 环境为基线：

- 页面：`http://greenpass.localhost:30080`
- team：`1001`
- 项目：`im-saas`（`2876690473259008`）
- 仓库：8 个（`open-chat-*`、`open-im-server`、`open-saas-*`、`open-website`）
- 服务：27 个

## 1. 先选接入方式

| 方式 | 何时使用 | 前置条件 | 凭据落点 |
|---|---|---|---|
| GitHub App（推荐） | 生产/共享环境，有 GitHub 可访问的公网 HTTPS 域名 | 管理员创建 App、注入 App ID/Client/私钥、配置 Setup URL 与 OAuth 回调 | 私钥只在 K8s Secret；运行时换短时 installation token |
| Fine-grained PAT（本地/存量兼容） | 本地 kind、内网、没有公网 HTTPS 回调 | 用户能在 GitHub 创建只读 fine-grained PAT | Token 存在 K8s Secret；数据库只存 `k8s://...` 引用 |

> 当前 `http://greenpass.localhost:30080` 不能作为 GitHub App 的真实回调地址。GitHub 要求回调是它可访问的 HTTPS 地址；本地环境没有隧道时，请使用 fine-grained PAT，不要反复尝试配置 App。

无论哪种方式，平台都不应该在数据库、日志、审计载荷或前端响应中保存/返回 Token 本体。

## 2. 创建只读 fine-grained PAT

1. 打开 <https://github.com/settings/personal-access-tokens/new>。
2. **Token name**：`greenpass-im-saas-readonly-YYYYMMDD`。
3. **Resource owner**：个人账号即可（8 个仓库都是 public）；如果要覆盖私有仓库，选 `openware-io` 并走组织审批。
4. **Expiration**：30 天或团队规定的更短周期。
5. **Repository access**：
   - 仅 public 仓库：选 **Public repositories**。
   - 私有仓库：选 **Only select repositories**，只勾选需要测试的仓库，不要选 All repositories。
6. **Permissions**：
   - `Repository permissions` → `Contents: Read-only`
   - `Metadata: Read-only`（必选/自动）
   - 不要授予 `Administration`、`Actions`、`Secrets`、`Members` 等写/管理权限。
7. 点击 **Generate token**，只复制一次。

安全要求：

- 不要把 Token 发到聊天、Issue、文档、提交或普通环境变量文件。
- 本次录入使用的 Token 已出现在会话中，完成验收后应按第 6 节轮换。
- 不要把 Token 写进 `VITE_*`；前端永远不接触 Token。

## 3. 在 GreenPass 录入连接

1. 用 owner/admin 登录 GreenPass。
2. 进入 **被测对象**，在项目下拉框选择 `im-saas`。
3. 点击项目条上的 **源码读取连接**。
4. 未配置 GitHub App 时，弹窗会直接显示 **使用只读 fine-grained PAT** 表单；如果已配置 App，展开 **兼容方式：fine-grained PAT**。
5. 填写：
   - 连接名称：`im-saas-readonly-YYYYMMDD`（同一团队内唯一）
   - GitHub fine-grained PAT：粘贴第 2 步生成的 Token
6. 点击 **验证并保存连接**。成功后会显示“已验证”，页面提示 Token 已安全保存。
7. 关闭弹窗，回到仓库列表。

> 保存时平台会校验 Token 类型：带 `repo`、`admin:org`、`workflow` 等 classic scope 的 Token 会被拒绝。Token 对具体仓库的 Contents 权限在“绑定仓库”时再做一次仓库级探测；因此“Token 已验证”不等于“所有目标仓库都可读”。

如果连接状态为 **验证失败**：在弹窗中对该连接点 **重新验证**；Token 过期/被撤销时点 **轮换** 粘贴新 Token。页面加载时会自动拉取连接列表，不再需要先打开弹窗触发加载。

## 4. 绑定 8 个仓库

对每个仓库：

1. 展开仓库行。
2. 在 **源码读取连接** 下拉中选择 `im-saas-readonly-YYYYMMDD · 已验证`。
3. 选择后平台会先用该连接读取目标仓库的 Metadata 和根 Contents，再保存绑定；仓库行应显示 **源码连接已验证**。
4. 若返回 `422 repository_not_readable`，说明该 Token 没有这个仓库的 Contents 读取权限；检查 PAT 的 Repository access / Contents 权限，或换用覆盖该仓库的 Token。

> 绑定校验会额外调用一次 GitHub API，首次绑定可能比之前多 1～2 秒。

如果 UI 下拉为空或绑定失败，可用 API 批量绑定（需要 owner/admin 的 `gp_session` Cookie）：

```powershell
$base = 'http://127.0.0.1:30080/api'
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$login = @{ username = $env:GP_USER; password = $env:GP_PASSWORD } | ConvertTo-Json -Compress
Invoke-WebRequest -Uri "$base/auth/login" -Method Post -Headers @{ Host = 'greenpass.localhost' } `
  -ContentType 'application/json' -Body $login -WebSession $session -UseBasicParsing | Out-Null

$projectId = 2876690473259008
$connectionId = 2900333722734592  # 以 GET /scm-connections 的实际 ID 为准
$repoIds = @(
  2876819927601152, 2876819927732224, 2876819927863296, 2876819927928832,
  2876819928027136, 2876819928092672, 2876819928190976, 2876819928256512
)
foreach ($repoId in $repoIds) {
  $body = @{ scm_connection_id = $connectionId } | ConvertTo-Json -Compress
  Invoke-WebRequest -Uri "$base/projects/$projectId/repositories/$repoId/scm-connection" `
    -Method Patch -Headers @{ Host = 'greenpass.localhost' } -ContentType 'application/json' `
    -Body $body -WebSession $session -UseBasicParsing | Out-Null
}
```

## 5. 验收

### 5.1 列表与状态

```powershell
# 连接列表（不会返回 Token）
Invoke-WebRequest -Uri "$base/scm-connections" -Headers @{ Host = 'greenpass.localhost' } `
  -WebSession $session -UseBasicParsing | Select-Object -ExpandProperty Content

# 仓库列表：8/8 应为 scm_connection_status=verified
$repos = (Invoke-WebRequest -Uri "$base/projects/$projectId/repositories" `
  -Headers @{ Host = 'greenpass.localhost' } -WebSession $session -UseBasicParsing).Content | ConvertFrom-Json
$repos | Select-Object url, scm_connection_id, scm_connection_status | Format-Table
```

期望：

- `connections` 中有且只有一条本次连接，`credential_status=verified`，`auth_type=pat`。
- 8 个仓库的 `scm_connection_id` 都等于本次连接 ID，`scm_connection_status=verified`。

### 5.2 真实读取分支/Tag

```powershell
Invoke-WebRequest -Uri "$base/projects/$projectId/repositories/2876819928027136/refs:refresh" `
  -Method Post -Headers @{ Host = 'greenpass.localhost' } -WebSession $session -UseBasicParsing
```

期望返回 200 和分支/Tag 列表。2026-10-21 的实测结果：`open-im-server` 返回 7 个 ref（`develop/2.2.0-auth`、`develop/2.3.0`、`develop/2.4.0-hotel`、`main`、`2.2.1`、`naming-unified-20261014`、`v2.2.0`）。

### 5.3 Secret 与数据库边界

```powershell
kubectl --context kind-open-im-local -n gp get secret gp-scm-1001-<connectionId>
kubectl --context kind-open-im-local -n gp exec deploy/gp-postgres -- psql -U gp -d gp `
  -c "select id,name,credential_status,auth_type,secret_ref from repo_scm_connection;"
```

期望：Secret 存在；数据库只有 `k8s://gp/gp-scm-...` 引用，没有 Token 明文。

### 5.4 生成前的最小验证

不要为了“验证凭据”直接跑完整生成（会消耗模型额度并产生批次）。先做 `refs:refresh`；需要进一步确认时，再在“被测对象”对单个服务执行一次技术画像分析，确认 Worker 能读取源码。执行前确认模型 Secret、项目模型绑定和环境登记都已就绪。

## 6. 轮换与撤销

产品已提供连接级轮换与删除（SCM-06/07），优先使用页面：

1. 在 GitHub 新建一条同权限的只读 fine-grained PAT。
2. 在“被测对象 → 源码读取连接”的已配置连接行点 **轮换**，粘贴新 Token，保存。
3. 平台会用新 Token 重新验证并写入原 Secret 的 `token` 键；连接 ID、仓库绑定和 Secret 名不变。
4. 回 GitHub 撤销旧 Token。
5. 不再使用时点该连接的 **删除**：平台会先解除所有仓库绑定，再删除 DB 记录和 PAT Secret；GitHub App 连接只删元数据，不删除共享私钥 Secret。

手工兜底（页面不可用或需要审计留痕时）：

```powershell
kubectl --context kind-open-im-local -n gp get secret | Select-String 'gp-scm'
kubectl --context kind-open-im-local -n gp create secret generic gp-scm-1001-<id> `
  --from-literal=token='<new-token>' --dry-run=client -o yaml | kubectl apply -f -
```

然后在页面点 **重新验证**，确认 `verified`，再执行一次 `refs:refresh`。

> 注意：轮换/删除/绑定/验证都会写 trusted audit；Token 不会出现在审计载荷。删除后无法从页面恢复，需要重新创建连接。

## 7. 故障排查

| 现象 | 可能原因 | 处理 |
|---|---|---|
| `409 github_app_not_configured` | 平台未注入 `GP_GITHUB_APP_*` | 本地/无 HTTPS 环境改用 PAT；生产按 `GITHUB-APP-CONNECTION-RUNBOOK.md` 配置 App |
| `502 github_connection_failed` | Token 失效、被撤销、网络不通、GitHub 限流，或使用了 classic/写权限 Token | 用 `GET https://api.github.com/user` 验证 Token；确认 `X-OAuth-Scopes` 为空；重新生成 fine-grained 只读 Token |
| `409 scm_connection_name_conflict` | 名称已被同团队连接占用（现在只在该唯一键冲突时返回） | 换一个唯一名称 |
| `500 scm_connection_create_failed` | 数据库约束、RLS 或连接写入失败 | 查 `gp-server` 日志和 `repo_scm_connection` 表；失败时平台会自动删除刚创建的 Secret |
| `500 secret_store_failed` | `GP_SCM_SECRET_NAMESPACE` 为空、ServiceAccount 无 Secret 权限、Secret 重名 | 检查 `gp-controller` Role（需 create/get/update/delete）和命名空间；确认 `GP_SCM_SECRET_NAMESPACE=gp` |
| `422 repository_not_readable` | 连接无法读取目标仓库 Metadata/根 Contents | 检查 PAT 的 Repository access 和 `Contents: Read-only`；或换用覆盖该仓库的连接 |
| `409 github_app_rotation_not_supported` | 对 GitHub App 连接调用轮换 | App 通过平台私钥轮换；不要按连接替换 Token |
| 保存成功但仓库仍“未绑定” | 只创建了连接，没有绑定仓库 | 按第 4 节绑定；页面加载会自动拉取连接列表 |
| 刷新分支/Tag 失败 | Token 没有目标仓库 Contents 读取权限；分支名含特殊字符；仓库超过 1000 个分支/Tag | 检查 PAT 权限；换一个分支测试；超过 1000 会显式失败，见 SCM-15 |
| 生成时提示“仓库尚未绑定可用的 GitHub 只读连接” | 仓库的 `scm_connection_id` 为空，或连接状态不是 configured/verified | 重新绑定；检查连接状态；必要时重新验证 |
| 授权 App 后停在 JSON 页面 | `GP_GITHUB_APP_FRONTEND_URL` 未配置 | 补配置并重启 `gp-server`；见 SCM-11 |

## 8. 相关文档

- [GitHub App 源码读取连接配置手册](./GITHUB-APP-CONNECTION-RUNBOOK.md)
- [版本基线驱动的测试资产治理](../architecture/VERSION-BASELINE-TESTING.md)
- [本次问题清单](../status/SCM-CREDENTIAL-ISSUES-2026-10-21.md)
- [工程规范](../engineering/ENGINEERING-SPEC.md)

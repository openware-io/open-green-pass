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
3. 点击右上/项目条上的 **源码读取连接**。
4. 如果页面提示“GitHub App 尚未由平台管理员配置”，展开 **兼容方式：fine-grained PAT**。
5. 填写：
   - 连接名称：`im-saas-readonly-YYYYMMDD`（同一团队内唯一）
   - GitHub fine-grained PAT：粘贴第 2 步生成的 Token
6. 点击 **验证并保存连接**。成功后会显示“已验证”，页面提示 Token 已安全保存。
7. 关闭弹窗，回到仓库列表。

> 当前版本的已知交互问题：页面刚加载时连接列表可能为空，仓库行显示“尚未配置/未绑定”。先打开一次“源码读取连接”弹窗触发加载；后续版本应改为页面加载即加载（见问题清单 SCM-03）。

## 4. 绑定 8 个仓库

对每个仓库：

1. 展开仓库行。
2. 在 **源码读取连接** 下拉中选择 `im-saas-readonly-YYYYMMDD · 已验证`。
3. 选择后立即调用绑定接口并刷新；仓库行应显示 **源码连接已验证**。

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

当前版本没有删除/轮换连接的产品入口（见问题清单 SCM-06），按以下手工流程处理：

1. 在 GitHub 新建一条同权限的只读 fine-grained PAT。
2. 找到旧连接的 Secret 名：
   ```powershell
   kubectl --context kind-open-im-local -n gp get secret | Select-String 'gp-scm'
   ```
3. 用新 Token 更新 Secret 的 `token` 键（不要改 Secret 名，避免改 DB 引用）：
   ```powershell
   kubectl --context kind-open-im-local -n gp create secret generic gp-scm-1001-<id> `
     --from-literal=token='<new-token>' --dry-run=client -o yaml | kubectl apply -f -
   ```
4. 在平台点击该连接的 **重新验证**，确认 `verified`；再执行一次 `refs:refresh`。
5. 回 GitHub 撤销旧 Token。
6. 若连接已泄露或不再使用，删除 Secret 并把 `repo_scm_connection` 记录/仓库绑定清掉；删除前先解绑仓库，避免生成时引用失效。

> 更好的做法是尽快实现 SCM-06 的删除/轮换 API 和 SCM-07 的审计事件，把手工作业变成可审计的产品流程。

## 7. 故障排查

| 现象 | 可能原因 | 处理 |
|---|---|---|
| `409 github_app_not_configured` | 平台未注入 `GP_GITHUB_APP_*` | 本地/无 HTTPS 环境改用 PAT；生产按 `GITHUB-APP-CONNECTION-RUNBOOK.md` 配置 App |
| `502 github_connection_failed` | Token 失效、被撤销、网络不通或 GitHub 限流 | 用 `GET https://api.github.com/rate_limit` 和 `GET /repos/{owner}/{repo}` 验证 Token；重新生成 |
| `409 scm_connection_name_conflict` | 名称已被同团队连接占用；也可能是 DB 约束/RLS 错误被误映射 | 换唯一名称；若仍失败，查 `gp-server` 日志和 `repo_scm_connection` 表；注意 SCM-02 的误导性 409 |
| `500 secret_store_failed` | `GP_SCM_SECRET_NAMESPACE` 为空、ServiceAccount 无 Secret 权限、Secret 重名 | 检查 `gp-controller` Role 和命名空间；确认 `GP_SCM_SECRET_NAMESPACE=gp` |
| 保存成功但仓库仍“未绑定” | 只创建了连接，没有绑定仓库；或页面连接列表未加载 | 按第 4 节绑定；先打开一次连接弹窗；刷新页面 |
| 刷新分支/Tag 失败 | Token 没有目标仓库 Contents 读取权限；分支名含特殊字符；仓库超过 100 个分支/Tag | 检查 PAT 的 Repository access 和 Contents 权限；换一个分支测试；大仓库见 SCM-15 |
| 生成时提示“仓库尚未绑定可用的 GitHub 只读连接” | 仓库的 `scm_connection_id` 为空，或连接状态不是 configured/verified | 重新绑定；检查连接状态；必要时重新验证 |
| 授权 App 后停在 JSON 页面 | `GP_GITHUB_APP_FRONTEND_URL` 未配置 | 补配置并重启 `gp-server`；见 SCM-11 |

## 8. 相关文档

- [GitHub App 源码读取连接配置手册](./GITHUB-APP-CONNECTION-RUNBOOK.md)
- [版本基线驱动的测试资产治理](../architecture/VERSION-BASELINE-TESTING.md)
- [本次问题清单](../status/SCM-CREDENTIAL-ISSUES-2026-10-21.md)
- [工程规范](../engineering/ENGINEERING-SPEC.md)

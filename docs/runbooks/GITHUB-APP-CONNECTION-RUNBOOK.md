# GitHub App 源码读取连接配置手册

## 目标与边界

GreenPass 使用 GitHub App 读取被测仓库。平台不会接收个人 Token：数据库只记录团队、App 安装 ID 和私钥引用；运行时才由 App 私钥交换短时 installation token。Fine-grained PAT 仅用于兼容既有连接。

授权流程如下：

```text
GreenPass 用户 → GitHub 安装 App → GitHub Setup URL
→ GreenPass 发起 GitHub 用户授权（一次性 state，10 分钟）
→ 校验用户可见该 installation → 保存团队连接
```

Setup URL 带回的 `installation_id` 不能单独作为可信输入；平台必须完成随后 GitHub 用户授权，并用用户 token 查询该用户可见的安装列表。用户 token 仅在该请求内使用，不写入数据库、Secret、日志或审计载荷。

## GitHub App 设置

以管理员身份在 GitHub 创建或更新 App：

| 项目 | 值 |
| --- | --- |
| Repository permissions | `Contents: Read`、`Metadata: Read` |
| Setup URL | `https://<greenpass-domain>/api/scm-connections/github-app/callback` |
| User authorization callback URL | `https://<greenpass-domain>/api/scm-connections/github-app/oauth-callback` |
| 安装范围 | 由组织/用户在 GitHub 界面选择所需仓库；仅授予测试所需仓库 |

`<greenpass-domain>` 必须是 GitHub 可访问的 HTTPS 域名。`greenpass.localhost` 仅适合本地页面验证，不能作为 GitHub 的真实回调地址。

下载 App 的 RSA 私钥，并妥善保存 App ID、Client ID、Client secret。不要把私钥、Client secret 提交到代码库或写进普通环境变量文件。

## Kubernetes 注入

先创建单独的私钥 Secret。`private-key` 是固定键名，下面命令不打印私钥内容：

```powershell
kubectl --context kind-open-im-local -n gp create secret generic gp-github-app `
  --from-file=private-key=C:\secure\greenpass-github-app.private-key.pem
```

再更新现有 `gp-app-config`，保留其中已有的数据库、Redis、对象存储等键；以下配置值为示例占位，须替换为真实值：

```text
GP_GITHUB_APP_ID=<GitHub App ID>
GP_GITHUB_APP_CLIENT_ID=<GitHub App Client ID>
GP_GITHUB_APP_CLIENT_SECRET=<GitHub App Client secret>
GP_GITHUB_APP_INSTALL_URL=https://github.com/apps/<app-slug>/installations/new
GP_GITHUB_APP_PRIVATE_KEY_REF=k8s://gp/gp-github-app#private-key
GP_GITHUB_APP_OAUTH_CALLBACK_URL=https://<greenpass-domain>/api/scm-connections/github-app/oauth-callback
GP_GITHUB_APP_FRONTEND_URL=https://<greenpass-domain>
```

重启 `gp-server` 后，在“被测对象 → 源码读取连接”中应出现“前往 GitHub 授权”。完成授权后，将连接绑定到具体仓库；分支/Tag 刷新、技术画像和源码冻结都复用这个已验证连接。

## 验收与轮换

1. 管理员完成一次 GitHub App 安装和用户授权；页面显示一条“App 授权、已验证”的连接。
2. 将连接绑定到一个私有仓库，刷新分支/Tag；确认无个人 Token 出现在浏览器请求、数据库或日志。
3. 发起技术画像，确认 Worker 可以按需换取 installation token 并读取授权目录。
4. 测试 `state` 复用、过期和不同 GreenPass 用户回调，均应被拒绝。
5. 私钥轮换：先在 GitHub 生成新私钥，更新 `gp-github-app/private-key`，滚动重启 server/worker，验证后才撤销旧私钥。Client secret 轮换同理更新 `gp-app-config`。

当安装删除、仓库权限收回或 token 交换失败时，连接重新验证应失败；平台必须阻断源码刷新和生成，不能回退到匿名读取或过期 Token。

# GreenPass kind 发布记录：0.1.85-SNAPSHOT（2026-10-22）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.1.85-SNAPSHOT`（`0.1.84` 之后的下一 patch） |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `6ebe99974152cc80364b751c7dbb6eac8954e249` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.85-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-22 13:09（Asia/Shanghai） |

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.1.85-SNAPSHOT` | `sha256:b58c2457b565bebd575548cf40f83470db60586f5ccab50ce6344dfdaab0cfb2` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.1.85-SNAPSHOT` | `sha256:680db4e0c5f5d427180c778efea95ec4cba334d4ce49e619e25cc7333c0115d1` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.1.85-SNAPSHOT` | `sha256:db97f78aefe5472add1f2cfe6bd0fe05389ad813e60c33dcdbb269bbe55824ab` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.1.85-SNAPSHOT`。

## 交互修复

问题：生成页的“代码生成来源”只作用于当前查看服务，而“确认创建”面板仍显示仓库默认分支（`develop/2.2.0-auth`）；用户选了 Tag `2.2.1` 后看起来像“设置未生效”，且多处选择入口关系不清。

修复：

- 确认面板改为逐服务显示 **本次代码引用**（分支/Tag + 提交 SHA），并可在确认面板内直接切换分支/Tag。
- 创建批次不再静默回退默认分支；缺少代码引用时明确报错并阻止创建。
- 打开确认面板前先为所有已选服务加载分支/Tag，避免“未选择”被误创建。
- 原“代码生成来源”改名为“当前查看服务的代码来源”，并注明批量确认时在确认面板逐服务复核。
- 主操作按钮为“生成用例（N）”，确认按钮为“确认创建 N 个批次”。

## 设置生效验证

使用浏览器网络拦截验证实际请求（拦截后 abort，不产生真实批次）：

```json
{
  "target_id": 2876690473259008,
  "repo_id": 2876819927601152,
  "service_id": 2876847384236032,
  "source_path": ".",
  "ref_type": "tag",
  "branch": "2.2.1",
  "idempotency_key": "ui-batch-..."
}
```

确认页面选择 Tag `2.2.1` 后，请求体为 `ref_type=tag`、`branch=2.2.1`，并显示提交 SHA `a44876435f69`，说明设置已生效。

## 迁移与运行检查

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-1-85-snapshot` |
| 结果 | `Complete` |
| Schema 版本 | `33` |
| Dirty | `false` |
| `/api/readyz` | 200，`database/redis/temporal=ok` |
| 登录 / `/api/auth/me` | 200 / 200 |
| `open-im-local` 快照核对 | 脚本内置检查通过 |

## SCM 验收

| 检查 | 结果 |
|---|---|
| 连接 | `im-saas-readonly-20261021`，`verified` |
| 仓库绑定 | 8/8 `verified` |
| `open-chat-admin` 分支 | `refs:refresh` 200，4 分支 + 3 Tag |

## 镜像清理

- `init-kind.ps1` 清理 Docker 本地 `0.1.84`，仅保留 `0.1.85`。
- 两个 kind 节点删除 `0.1.84` 的 server/web/migrate tag；当前仅保留 `0.1.85`。
- `0.3.x` 仍为空。

## 结论

`0.1.85-SNAPSHOT` 已通过唯一入口发布；生成用例的代码引用在确认面板逐服务可见、可改、实际进入请求体（`ref_type=tag`, `branch=2.2.1`），不再静默使用默认分支。

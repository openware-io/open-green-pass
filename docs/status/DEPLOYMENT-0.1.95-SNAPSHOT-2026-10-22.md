# GreenPass kind 发布记录：0.1.95-SNAPSHOT（2026-10-22）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.1.95-SNAPSHOT` |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `c86478ca9ab8270fb3e67cf8eebdbe7db568f812` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.95-SNAPSHOT -ImageSource local-build` |
| 部署时间 | 2026-10-22 17:20（Asia/Shanghai） |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:42ade7d5f3779c83cbbd130d8dc26378798132de553a1b189744a8bf77dc0198` |
| web | `sha256:4dfc6379489b23fee8d049c656acd4206ecc5c4dcd88c2e6e60721225f78f447` |
| migrate | `sha256:db97f78aefe5472add1f2cfe6bd0fe05389ad813e60c33dcdbb269bbe55824ab` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.1.95-SNAPSHOT`。

## 变更

- 按 `DESIGN-SPEC` 修正缺陷中心原型的色彩：移除 `sky`/`violet`，统一为 emerald/amber/red/slate。
- P2 严重级别改为 slate；修复中改为 emerald；待回归改为 amber；视频/证据图标改为 slate/emerald。
- 无后端/数据结构变更。

## 工程合规审查

本版同时提交了 [`ENGINEERING-COMPLIANCE-REVIEW-2026-10-22.md`](ENGINEERING-COMPLIANCE-REVIEW-2026-10-22.md)：

- 结论：AI 测试引擎方案方向可行，但实施前仍有 P0 合规缺口。
- 已在方案/规格中加入表前缀映射、领域归属、RLS/可信写、迁移/OpenAPI/测试/部署/沙箱约束。
- `def_`/`cov_` 新前缀、`open-green-pass-dsh-gateway` 镜像、DSH Node 子进程例外仍需正式登记/评审。

## 迁移与检查

| 项目 | 值 |
|---|---|
| Job | `gp-migrate-0-1-95-snapshot` |
| 结果 | `Complete` |
| Schema | `33` |
| `/api/readyz` | 200，database/redis/temporal ok |
| IM 隔离 | 通过 |

## 镜像清理

- Docker 本地清理 `0.1.94`，当前仅保留 `0.1.95`。
- 两个 kind 节点删除 `0.1.94`，当前仅保留 `0.1.95`。

## 说明

本版仍是原型；未实现 finding/evidence 后端、覆盖账本或 DSH SDK 网关。合规审查中的 P0 项完成前不进入 M0 编码。

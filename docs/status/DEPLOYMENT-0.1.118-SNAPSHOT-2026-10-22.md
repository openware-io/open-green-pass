# GreenPass kind 发布记录：0.1.118-SNAPSHOT（2026-10-22）

## 基线

| 项目 | 值 |
|---|---|
| 版本 | `0.1.118-SNAPSHOT` |
| 部署 checkout | `6ee04d6` + 工作区改动（提交 `0e450ab`） |
| schema | 43 |
| 入口 | `scripts/dev/init-kind.ps1 -Version 0.1.118-SNAPSHOT -ImageSource local-build` |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:502b51d07cf837715f6a7e677ef25357253becd98c84d10f8cee109a9400aaad` |
| web | `sha256:e99331fafaf27e1ab486d782fe3a06860ebf82d5508f0256ddace6c00269013c` |
| migrate | `sha256:eca81db9d1dce3a687205263a736aa8247d74f9172b077f0ff62f9c4cb6c9eac` |
| dsh-gateway | `sha256:2ec11979d1e16a3938c3682318fe260aac77c7488f390019f271e5f576ee78a3` |

## 新增

- API 安全响应头（nosniff / DENY / no-referrer / Permissions-Policy / no-store）
- 请求体 2MiB 上限
- 安全中间件单测

## 验收

| 检查 | 结果 |
|---|---|
| 安全响应头 | 全部存在 |
| `Cache-Control` | `no-store` |
| 租户隔离（team 1002） | finding/evidence/gate 均 403 |
| `go test ./...` | 通过 |
| `npm run typecheck` | 通过 |
| `npm run api:check` | 通过 |
| IM 隔离 | 通过 |

## 说明

- 安全头/限流属于 API 层；TLS/HSTS 由 ingress/边缘负责。
- 对象存储预签名、真实执行器、GHCR 推送、备份演练仍未完成。

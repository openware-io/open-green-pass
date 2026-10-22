# GreenPass kind 发布记录：0.1.99-SNAPSHOT（2026-10-22）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 部署版本 | `0.1.99-SNAPSHOT` |
| 部署 checkout | `66c36f0` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.99-SNAPSHOT -ImageSource local-build` |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:e7585e6670eb0bdfa159c9e99755e6ab5119eb7a37efc43c1b7854eb53c983fe` |
| web | `sha256:8d014539ab36058ad1bd32bdb8c4861f939989ad04fe9c9bf9c2d0e66cc4db0a` |
| migrate | `sha256:d80ca2f243c0c53dc8d17bd634c92934389b4a781b1818e1547e9ff17813f250` |

## 数据库

- Schema `40`
- 新增 `ctr_endpoint`、`ctr_parameter`、`ctr_error`
- `cov_cell` 增加 `module`

## 验收

| 检查 | 结果 |
|---|---|
| 迁移 Job `gp-migrate-0-1-99-snapshot` | Complete |
| `/api/readyz` | 200 |
| `/api/coverage/plan` | 201，24 Cell |
| `/api/coverage/summary` | total_required=24 |
| 缺陷中心 | 6 条真实数据保持可用 |
| IM 隔离 | 通过 |

## 部署过程

首次和第二次部署均因 `open-im-local` 工作负载在部署窗口内变化（`platform-admin-service`、`saas-admin`）触发隔离校验，脚本自动回滚到 `0.1.98`。等待两套 IM Deployment generation 稳定 60 秒后重试成功，未绕过检查。

## 说明

- 本版完成覆盖账本 API；DSH gateway、生成用例、case_id 回填仍未完成，M1 仍在进行中。
- 覆盖账本当前 design_covered=0 是预期，等待生成用例。

# GreenPass kind 发布记录：0.1.98-SNAPSHOT（2026-10-22）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.1.98-SNAPSHOT` |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `dbb27c9` + M0 工作区改动（部署后提交为 `06a5375`，迁移与代码一致） |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.98-SNAPSHOT -ImageSource local-build` |
| 部署时间 | 2026-10-22 21:34（Asia/Shanghai） |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:b14cc6c4b5361b7c25f72df9cf1fca301471334a131309bfb5befde3c3939b58` |
| web | `sha256:22f35804b89e56381616c14376d358ff45edb3972a730ce772806d81db5e4dc4` |
| migrate | `sha256:0083c447f5e7b608ef2638e329e99f72ba0bee42a2796265f2eca93ce1049e86` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.1.98-SNAPSHOT`。

## 数据库

| 项目 | 值 |
|---|---|
| 迁移 Job | `gp-migrate-0-1-98-snapshot` |
| 结果 | `Complete` |
| Schema | `38` |
| 新表 | `def_finding`、`def_event`、`def_suggestion`、`cov_cell`、`cov_waiver`、`evd_evidence`、`run_fixture`、`env_lease` |
| 现有表变更 | `cas_case` 加覆盖/来源列；`run_case_result` 加 `failure_class`/`finding_id` |

## 验收

| 检查 | 结果 |
|---|---|
| 首页/readyz | 200 |
| 登录 | 200 |
| `GET /api/findings` | 6 条真实数据 |
| 创建缺陷 | 201 |
| 重复创建 | 200（dedupe） |
| 状态流转 | 200 |
| 缺陷中心 UI | API 数据 6 条；详情/时间线/状态按钮可用 |
| 审计 | `finding.created` 6、`finding.transition` 7 |
| IM 隔离 | 通过 |

## 说明

- 首个 0.1.96 部署发现创建缺陷 500：`def_event.payload` 为 NULL 违反 NOT NULL；0.1.97 修复并验证。
- 0.1.98 修正前端“无截图仍显示截图标记”的问题。
- 本版是 M0-Dev 最小闭环，截图/证据上传、覆盖 API、gateway 尚未实现。

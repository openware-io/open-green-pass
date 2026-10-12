# GreenPass 文档目录规范

所有长期有效、需要评审或需要随代码维护的文档必须进入 `docs/`，禁止在仓库根目录、`design/`、`.outputs/` 或源码目录中散落方案文档。

## 分类

| 目录 | 内容 | 命名规则 |
| --- | --- | --- |
| `product/` | 产品需求、业务口径 | `PRD.md` 或 `<DOMAIN>-PRD.md` |
| `architecture/` | 系统架构、技术设计、UI 设计规范 | `<TOPIC>-DESIGN.md` 或 `<TOPIC>-SPEC.md` |
| `engineering/` | 工程、编码、安全、发布规范 | `<TOPIC>-SPEC.md` |
| `plans/` | 实施计划、可执行方案、任务清单 | `<SCOPE>-IMPLEMENTATION-PLAN.md`、`<SCOPE>-BACKLOG.md` |
| `runbooks/` | 运维、恢复、故障处理手册 | `<TOPIC>-RUNBOOK.md` |
| `status/` | 阶段性进展快照 | `DEVELOPMENT-STATUS-YYYY-MM-DD.md` |
| `services/` | 服务职责、入口、依赖和运行方式 | `README.md` 或 `<SERVICE>.md` |

## 文件位置约束

- 仓库根目录只保留构建入口、依赖清单、许可证、贡献指南、变更日志和顶层 `README.md`。
- 生成物统一写入 `dist/`、`bin/` 或 `.outputs/`，三者均不得提交。
- 调试日志统一写入 `.outputs/logs/<YYYY-MM-DD>/`，禁止写根目录 `dev.log`。
- 本地回放载荷写入 `.outputs/replay/`，禁止写根目录。
- 临时设计稿放在忽略目录 `design/`；需要保留的设计必须整理后进入 `docs/architecture/`。
- 本地密钥只允许放在忽略的 `.env` 或 Kubernetes Secret；仓库仅提交 `.env.example` 占位模板。
- 文档移动后必须同步修复仓库内链接，并在本索引中登记。
- 本地空间清理统一执行 `make clean` 或 `pwsh -NoProfile -File scripts/dev/clean-local.ps1`；先用 `-WhatIf` 预览目标。

## 索引

- 产品：[`product/PRD.md`](product/PRD.md)
- 架构：[`architecture/TECH-DESIGN.md`](architecture/TECH-DESIGN.md)、[`architecture/DESIGN-SPEC.md`](architecture/DESIGN-SPEC.md)
- 工程：[`engineering/ENGINEERING-SPEC.md`](engineering/ENGINEERING-SPEC.md)、[`engineering/SECURITY-BASELINE.md`](engineering/SECURITY-BASELINE.md)
- 计划：[`plans/IMPLEMENTATION-PLAN.md`](plans/IMPLEMENTATION-PLAN.md)、[`plans/GP3-GP4-EXECUTION-BACKLOG.md`](plans/GP3-GP4-EXECUTION-BACKLOG.md)
- 运维：[`runbooks/GP-DATABASE-RECOVERY-RUNBOOK.md`](runbooks/GP-DATABASE-RECOVERY-RUNBOOK.md)、[`runbooks/OBSERVABILITY-RUNBOOK.md`](runbooks/OBSERVABILITY-RUNBOOK.md)
- 状态：[`status/DEVELOPMENT-STATUS-2026-10-08.md`](status/DEVELOPMENT-STATUS-2026-10-08.md)
- 服务：[`services/README.md`](services/README.md)

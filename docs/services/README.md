# 服务目录

| 入口 | 职责 | 主要依赖 | 运行方式 |
| --- | --- | --- | --- |
| `cmd/server` | HTTP API、认证授权、治理与执行入口 | PostgreSQL、Redis、Temporal、Kubernetes API | Helm `gp-server` Deployment |
| `cmd/worker` | 异步任务与工作流处理 | PostgreSQL、Redis、Temporal | Helm `gp-worker` Deployment |
| `cmd/migrate` | 数据库只增迁移 | PostgreSQL | Helm pre-upgrade Migration Job |
| `cmd/auditverify` | 审计链离线校验 | PostgreSQL | 受控运维任务 |
| `cmd/schedreplay` | 调度载荷回放 | Redis、服务配置 | 本地/受控运维命令，输出写 `.outputs/replay/` |

新增 `cmd/*` 入口时必须同步更新本表；复杂服务可拆分为本目录下独立文档。

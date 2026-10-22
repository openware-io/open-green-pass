# GreenPass kind 发布记录：0.1.103-SNAPSHOT（2026-10-22）

## 基线

| 项目 | 值 |
|---|---|
| 版本 | `0.1.103-SNAPSHOT` |
| 部署 checkout | `6d14b61`（部署时基于工作区改动，提交后一致） |
| 入口 | `scripts/dev/init-kind.ps1 -Version 0.1.103-SNAPSHOT -ImageSource local-build` |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:365f18c70e6a12589ee6e75ebce22c9ceb136f1f7edaf9818608fc691b3b6e6a` |
| web | `sha256:ad252a8b56a385e7f68c3ad16b27712203571fac66dcd2ced922916ffbc8a635` |
| migrate | `sha256:e50ed633efa164484422fa3f2202edd860f5d8441aa2f9fd30591bf69a82f964` |
| dsh-gateway | `sha256:4fcfc266cfd4fbc4188326ba2e69824c47151d02994a9eca6ce0185e840e5dc5` |

## 部署内容

- `gp-dsh-gateway` Deployment/Service/SA（内部 8090，不暴露 ingress）
- `gp-dsh-auth` Secret（随机 token，首次创建，后续保留）
- server `GP_DSH_GATEWAY_URL` + `GP_DSH_INTERNAL_TOKEN`
- NetworkPolicy 允许 gp 内部 8090
- 迁移 42：`gen_job.updated_by`，修复 `gp.set_audit()` 触发器

## 验收

| 检查 | 结果 |
|---|---|
| `gp-dsh-gateway` | 1/1 Running |
| `/api/readyz` | 200 |
| `POST /api/ai/generation-jobs` | 202，external_job_id 写入 |
| `GET /api/ai/generation-jobs/{id}` | succeeded，output=`stub:gp-...` |
| 覆盖账本 | 24 required Cell |
| 缺陷中心 | 6 条真实数据 |
| IM 隔离 | 通过（期间 gateway/saas-admin 有外部 rollout，等待稳定后重试） |

## 说明

- 当前 `GP_DSH_RUNNER=stub`；真实 SDK runner 代码已存在，但镜像未包含 Node/DSH，未启用。
- 生成用例、`cov_cell.case_id` 回填仍未完成，M1 仍在进行中。

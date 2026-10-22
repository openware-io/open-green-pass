# GreenPass kind 发布记录：0.1.100-SNAPSHOT（2026-10-22）

## 基线

| 项目 | 值 |
|---|---|
| 版本 | `0.1.100-SNAPSHOT` |
| 部署 checkout | `5b761bd` + generation job 工作区改动（提交 `7f23db9`） |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.100-SNAPSHOT -ImageSource local-build` |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:e62c0ba6f8e3921d99d40905535737eaf8fece2f54da8969804dd77c7bd973bb` |
| web | `sha256:ad252a8b56a385e7f68c3ad16b27712203571fac66dcd2ced922916ffbc8a635` |
| migrate | `sha256:6a1917cd82b5bb8a6ab2ee17156bc7043185edb84a2b95226c7f1796d67a6717` |

## 数据库

- Schema `41`
- 新增 `gen_job`、`gen_job_event`
- 覆盖账本 24 Cell 保持

## 验收

| 检查 | 结果 |
|---|---|
| 迁移 41 | Complete |
| `/api/readyz` | 200 |
| `/api/coverage/summary` | total_required=24 |
| `/api/ai/generation-jobs` | 400 `DSH gateway not configured`（预期：未配置 gateway URL） |
| 缺陷中心 | 6 条真实数据可用 |
| IM 隔离 | 通过 |

## 说明

- 生成 Job 控制面已落库/API/客户端；当前集群未配置 `GP_DSH_GATEWAY_URL`，因此提交返回 400。
- `gen_job` 产生 1 条 failed 记录，属预期的配置缺失验证。
- 真实 DSH SDK runner、gateway 镜像/Deployment 尚未接入；M1 仍在进行中。

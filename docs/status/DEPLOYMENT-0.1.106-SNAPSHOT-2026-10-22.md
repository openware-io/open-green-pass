# GreenPass kind 发布记录：0.1.106-SNAPSHOT（2026-10-22）

## 基线

| 项目 | 值 |
|---|---|
| 版本 | `0.1.106-SNAPSHOT` |
| 部署 checkout | `02a65db` + 工作区改动（提交 `d32a0dd`） |
| schema | 43 |
| 入口 | `scripts/dev/init-kind.ps1 -Version 0.1.106-SNAPSHOT -ImageSource local-build` |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:1f563e60dede6b957cda3274942d28518ddba98334c2d121b22ff76bb7a685ee` |
| web | `sha256:950f7a65805035421ff1a25c154b22f955c407f7823fd7d6041e2ada91b694bd` |
| migrate | `sha256:eca81db9d1dce3a687205263a736aa8247d74f9172b077f0ff62f9c4cb6c9eac` |
| dsh-gateway | `sha256:9644a5c222579b8c1db2c2a73b3d304721b6791995c0c62a65d08b95ccd1882c` |

## 新增

- 迁移 43：`cov_cell/run_fixture/ctr_endpoint.updated_by`，修复 `gp.set_audit()` 更新失败
- `POST /generation/jobs/{id}/cases:import`
  - 解析 DSH 输出 JSON 数组
  - 创建 `cas_case`/`cas_version`
  - 回填 `cov_cell.case_id` + `status=generated`
- `extractCaseArray` 支持 markdown code fence
- 本地真实 DSH SDK 生成 12 条 user 用例并导入

## 验收

| 检查 | 结果 |
|---|---|
| 迁移 43 | Complete |
| user Coverage Cell | 24 |
| design_covered | 24 |
| uncovered | 0 |
| 生成用例 | 12 条 |
| `cov_cell.case_id` 回填 | 24/24 |
| 真实 DSH 生成 | 12 条 JSON，导入成功 |
| 缺陷中心 | 6 条真实数据 |
| IM 隔离 | 通过 |

## 说明

- 集群内 gateway 仍为 `GP_DSH_RUNNER=stub`；真实 SDK runner 已在本地用 `dsh --profile sdk` 验证（smoke + 12 条 user 用例生成）。
- 生产启用真实 runner 仍需包含 Node/DSH 的镜像；当前网络无法拉取 Node base image。
- M1 功能验收（覆盖 24/24 + 12 用例 + 无源码写）已达成；M1-V 的 egress/真实 gateway 运行仍需补齐。

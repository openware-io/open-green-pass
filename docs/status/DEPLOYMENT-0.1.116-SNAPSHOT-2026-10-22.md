# GreenPass kind 发布记录：0.1.116-SNAPSHOT（2026-10-22）

## 基线

| 项目 | 值 |
|---|---|
| 版本 | `0.1.116-SNAPSHOT` |
| 部署 checkout | `122b049` + 工作区改动（提交 `612c394`） |
| schema | 43 |
| 入口 | `scripts/dev/init-kind.ps1 -Version 0.1.116-SNAPSHOT -ImageSource local-build` |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:3dfa4d8ea87a13ccecd68b6da053e3ecb709f59c78e4c15bf5e779e128494525` |
| web | `sha256:e99331fafaf27e1ab486d782fe3a06860ebf82d5508f0256ddace6c00269013c` |
| migrate | `sha256:eca81db9d1dce3a687205263a736aa8247d74f9172b077f0ff62f9c4cb6c9eac` |
| dsh-gateway | `sha256:ea215475891416b91eb997f2fd151d2cf54c09d868e918bbd8e0d48dd2f1686a` |

## 新增

- `POST /api/runs/{id}/coverage-sync`
- 执行结果按 `case_id` 回写 `cov_cell.status`
- `executed` 统计包含 passed/failed/blocked

## 验收

| 检查 | 结果 |
|---|---|
| run 9001 coverage-sync | 200，updated=1 |
| 关联 Cell 状态 | `failed` |
| gate executed | 2（该 case 关联 2 个 Cell） |
| execution_coverage | 0.083 |
| gate_passed | false（预期，未达 90%） |
| IM 隔离 | 通过（首次部署 502 后回滚，重试成功） |

## 说明

- 首次 0.1.115 部署因 ingress 502 回滚；重试成功。
- M3 仍需真实执行全部用例、18 模块全量、报告增强。

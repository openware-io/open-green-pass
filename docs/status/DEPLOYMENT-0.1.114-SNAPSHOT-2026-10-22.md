# GreenPass kind 发布记录：0.1.114-SNAPSHOT（2026-10-22）

## 基线

| 项目 | 值 |
|---|---|
| 版本 | `0.1.114-SNAPSHOT` |
| 部署 checkout | `cdbc69d` + 工作区改动（提交 `6f1dea3`） |
| schema | 43 |
| 入口 | `scripts/dev/init-kind.ps1 -Version 0.1.114-SNAPSHOT -ImageSource local-build` |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:a9316eb27481e067957e216be6d3ebd1186fe590da3bfa2bde851687f5dc8326` |
| web | `sha256:e99331fafaf27e1ab486d782fe3a06860ebf82d5508f0256ddace6c00269013c` |
| migrate | `sha256:eca81db9d1dce3a687205263a736aa8247d74f9172b077f0ff62f9c4cb6c9eac` |
| dsh-gateway | `sha256:db1daf9a34c5a9f469dada3c2869e04ce852eb48c363c77b1b998f5155358086` |

## 新增

- `GET /api/gates/coverage`
- design=100%、execution≥90%、未关闭 P0/P1=0
- 返回 blockers

## 验收

| 检查 | 结果 |
|---|---|
| `user` 门禁 | 200 |
| design_coverage | 1.0 |
| execution_coverage | 0 |
| open_p1 | 2 |
| gate_passed | false |
| blockers | execution coverage、open P0/P1 |
| IM 隔离 | 通过 |

## 说明

- M3 门禁已可用；18 模块、真实执行回写、报告仍未完成。
- 当前 `execution_coverage=0` 是预期，因为用例尚未真实执行。

# GreenPass kind 发布记录：0.1.110-SNAPSHOT（2026-10-22）

## 基线

| 项目 | 值 |
|---|---|
| 版本 | `0.1.110-SNAPSHOT` |
| 部署 checkout | `11205ce` + 工作区改动（提交 `d3292a2`） |
| schema | 43 |
| 入口 | `scripts/dev/init-kind.ps1 -Version 0.1.110-SNAPSHOT -ImageSource local-build` |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:d5c89764d9693f0402aba38b30f10cefe1a11c96d69115075fa8e72e3c178ff7` |
| web | `sha256:810f8169c1f9446055aae9a9bbf1b2cab4cf196b4298ea0a50e285b864e6dc4a` |
| migrate | `sha256:eca81db9d1dce3a687205263a736aa8247d74f9172b077f0ff62f9c4cb6c9eac` |
| dsh-gateway | `sha256:9cf092987f0cfe2a74880e0b2ea50b463ce367655627e69926f7e7f6b30cc551` |

## 新增

- `POST /api/runs/{id}/triage`
- `CaseResult.failure_class` / `finding_id`
- execution → governance `FindingPort`
- `run_case_result` 关联 finding

## 验收

| 检查 | 结果 |
|---|---|
| `/api/runs/9001/case-results` | 200 |
| `/api/runs/9001/triage` | 201 |
| finding 创建 | `2903748672557056` |
| result 关联 | `failure_class=product`、`finding_id` 已写 |
| 缺陷详情 | linked_run=9001、linked_case 正确 |
| IM 隔离 | 通过 |

## 说明

- triage 使用手工 seed 的 failed result 验证；真实 run 执行/fixtures/evidence 仍待 M2 后续。
- 当前集群 gateway 为 stub；M2 归因链不依赖 gateway。

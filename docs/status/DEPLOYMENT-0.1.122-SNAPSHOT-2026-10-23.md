# GreenPass kind 发布记录：0.1.122-SNAPSHOT（2026-10-23）

## 基线

| 项目 | 值 |
|---|---|
| 版本 | `0.1.122-SNAPSHOT` |
| 部署 checkout | `e6d3676`（含环境页/HTTP runner） |
| schema | 43 |
| 入口 | `scripts/dev/init-kind.ps1 -Version 0.1.122-SNAPSHOT -ImageSource local-build` |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:365705d1d39bf93c2db240e4dddcdb2a998fbb62c1b65ad19b4eed03e6e80400` |
| web | `sha256:cead57d614f34787861848751c723883483294b77a19da967ea36773293e2c95` |
| migrate | `sha256:eca81db9d1dce3a687205263a736aa8247d74f9172b077f0ff62f9c4cb6c9eac` |
| dsh-gateway | `sha256:18deae3bf83d606ab5bc4f7b41e413093749e8dd3a3cfc198128186a0cca156f` |

## 新增

- 测试环境参数页 `/target-env`
- `GET /targets/{id}/env/runtime`
- `HTTPRunner`（`GP_RUNNER_TYPE=http`）
- `GP_EXECUTION_BEARER_TOKEN` 从 `gp-im-token` 注入

## 验收

| 检查 | 结果 |
|---|---|
| `/target-env` | 200 |
| im-saas env 配置 | 200/已登记 |
| 真实用例 | 201 |
| 真实 run `2904774788288512` | done |
| 真实结果 | pass, http_status=200 |
| 证据 hash | `sha256:956c6611...` |
| IM 隔离 | 通过（平台服务波动后重试） |

## 修复

- `run_case_result.failure_class` 空字符串 → NULL，解决真实执行 500。

## 说明

- 真实用例尚未绑定 Coverage Cell；M3 execution coverage 需下一步绑定。
- im-saas admin 密码已临时重置为测试密码。

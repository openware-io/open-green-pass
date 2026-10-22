# GreenPass kind 发布记录：0.1.112-SNAPSHOT（2026-10-22）

## 基线

| 项目 | 值 |
|---|---|
| 版本 | `0.1.112-SNAPSHOT` |
| 部署 checkout | `d4ea909` + 工作区改动（提交 `348d863`） |
| schema | 43 |
| 入口 | `scripts/dev/init-kind.ps1 -Version 0.1.112-SNAPSHOT -ImageSource local-build` |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:a80794dfd7ebb474bdbfdb8be47c5b8d23df83a3ecacc4fc991b9b9af108723d` |
| web | `sha256:d20aa18eb15c080a42c4de9c511088d58184cbc7aae5cb97cb6a3945ae5f6ce5` |
| migrate | `sha256:eca81db9d1dce3a687205263a736aa8247d74f9172b077f0ff62f9c4cb6c9eac` |
| dsh-gateway | `sha256:91ebae298264f90c2e7bd41fd43bd63879787c96034eada6df1a2ac55c86c531` |

## 新增

- `POST /runs/{id}/fixtures:prepare`
- `GET /runs/{id}/fixtures`
- `POST /runs/{id}/fixtures:cleanup`
- `POST /envs/{id}/lease`
- `DELETE /envs/{id}/lease`

## 验收

| 检查 | 结果 |
|---|---|
| fixture prepare | 201，namespace/manifest/expiry |
| fixture list | 200 |
| fixture cleanup | 200，state=cleaned/cleanup=done |
| env lease | 201 |
| env release | 200，lease 删除 |
| triage | 201，finding 关联保持 |
| IM 隔离 | 通过 |

## 说明

- fixture 清理曾出现一次 500，后续重新创建 fixture 后清理成功；已加错误日志。
- evidence 上传（截图/视频）尚未实现；M2 仍未完全关闭。

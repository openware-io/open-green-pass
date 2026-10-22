# GreenPass kind 发布记录：0.1.113-SNAPSHOT（2026-10-22）

## 基线

| 项目 | 值 |
|---|---|
| 版本 | `0.1.113-SNAPSHOT` |
| 部署 checkout | `f0b7a39` + 工作区改动（提交 `5b3ad5a`） |
| schema | 43 |
| 入口 | `scripts/dev/init-kind.ps1 -Version 0.1.113-SNAPSHOT -ImageSource local-build` |

## 镜像

| 制品 | Image ID |
|---|---|
| server / worker | `sha256:cef898228dbe3f81c9a658af754452245af062ae4c73b861176245b7f6de51c4` |
| web | `sha256:e99331fafaf27e1ab486d782fe3a06860ebf82d5508f0256ddace6c00269013c` |
| migrate | `sha256:eca81db9d1dce3a687205263a736aa8247d74f9172b077f0ff62f9c4cb6c9eac` |
| dsh-gateway | `sha256:b305bff58f89bfe83fc1de251adc37a6ffba14e569f5a17f32a14bd002dacb14` |

## 新增

- Trusted 证据元数据：
  - `POST /findings/{id}/evidence`
  - `GET /findings/{id}/evidence`
  - `GET /findings/{id}/evidence/{eid}`
- `evd_evidence` 写入（`gp_trusted_writer`）
- 校验：kind/MIME/大小/SHA-256；截图≤10MB、视频≤100MB

## 验收

| 检查 | 结果 |
|---|---|
| 登记截图证据 | 201 |
| 证据列表 | 200 |
| `storage_uri`/mime/sha256/尺寸 | 正确返回 |
| triage finding | 关联保持 |
| fixtures/lease | 保持可用 |
| IM 隔离 | 通过 |

## 说明

- 当前只登记对象存储元数据；预签名上传/下载未实现（需 S3 客户端）。
- 真实 run 执行、fixtures 自动接线、UI 证据展示仍未完成；M2 未完全关闭。

# GreenPass kind 发布记录：0.1.152-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.152-SNAPSHOT` |
| schema | 48 clean |
| 变更 | 浏览器证据上传代理、SeaweedFS S3 IAM、110MB ingress 上限 |
| 验证 | 14B/3MiB 上传、content 下载、预签名读写 |

## 说明

- `POST /findings/{id}/evidence/upload`
- `GET /findings/{id}/evidence/{eid}/content`
- `gp-s3-auth` Secret 挂载到 SeaweedFS
- `proxy-body-size: 110m`

## 注意

- JSON API 仍受 2MiB 上限保护；仅 `/evidence/upload` 例外。
- 证据文件上限：截图/日志 10MiB，视频 100MiB。

# M2：证据上传代理与 S3 认证（2026-10-23）

## 新增

- `POST /findings/{id}/evidence/upload?kind=...&sha256=...`
  - 原始二进制 body
  - `Content-Length` 必须为 10MiB 以内（video 100MiB）
  - 服务端流式写 S3，计算 SHA-256
  - 写入 `evd_evidence`
- `GET /findings/{id}/evidence/{eid}/content`
  - 服务端流式下载
- 证据上传路径从全局 2MiB JSON body limit 中豁免；handler 自限 10/100MiB。
- Ingress `proxy-body-size: 110m`。
- SeaweedFS 启用 S3 IAM：
  - `gp-s3-auth` Secret 挂载 `/etc/seaweedfs/s3.json`
  - `-s3.config=/etc/seaweedfs/s3.json`
  - `signingKey` + identity `gpminio`
- `deploy-kind.ps1` 自动创建 `gp-s3-auth`（使用 gp-app-config 的 key/secret）。

## 验证

- 14B 文本上传：201
- 3MiB 文件上传：201，`size_bytes=3145728`
- `GET .../content`：200，内容一致
- 预签名 PUT/GET 仍可用（集群内）

## 限制

- 浏览器使用上传代理，不再依赖集群内预签名 URL。
- 预签名 URL 仍只适合集群内工具。
- UI Findings 页尚未接入文件选择与上传按钮。

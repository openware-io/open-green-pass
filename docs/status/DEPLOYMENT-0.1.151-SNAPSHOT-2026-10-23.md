# GreenPass kind 发布记录：0.1.151-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.151-SNAPSHOT` |
| 部署 checkout | `c1f3368` |
| schema | 48 clean |
| server / worker | `sha256:17b961430d8cde8038426af7531f4e982665733360d27fa8ab96dc0c4e649c90` |
| web | `sha256:633731d2e58854e87ecfe154c1ef36009ab8acf788ef0e7a6001168ebf051817` |
| migrate | `sha256:149a4ae65e4d54f4b0c48adcf439492834eff2b6931039e6b7a3a32d99446b0a` |
| dsh-gateway | `sha256:ab0968d896f41f28f26179b0f310ef2209786ee22280a7a8834b1f3f76810df4` |
| 变更 | S3 证据预签名 API；M2 证据上传/下载链路 |
| 验证 | presign 201、upload 200、metadata 201、download 200 |

## 说明

- `POST /findings/{id}/evidence/presign`
- `GET /findings/{id}/evidence/{eid}/download-url`
- 预签名 URL 当前使用集群内 SeaweedFS endpoint，浏览器需外部入口或代理。
- 构建使用 `GOPROXY=https://goproxy.cn,direct`（Dockerfile / Dockerfile.migrate）。

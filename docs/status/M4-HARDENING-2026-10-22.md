# M4 加固进展（2026-10-22）

## 已完成

### API 安全基线

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`
- `Permissions-Policy` 限制 camera/microphone/geolocation
- API 响应 `Cache-Control: no-store`
- 请求体上限 2MiB（大证据必须走对象存储）

验证：

```text
HTTP/1.1 401 Unauthorized
Cache-Control: no-store
Permissions-Policy: camera=(), microphone=(), geolocation=()
Referrer-Policy: no-referrer
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
```

### 租户隔离

使用 `x-gp-team-id: 1002` 访问 team 1001 数据：

- finding：403
- evidence：403
- coverage gate：403

### 生产配置保护（已有）

- `GP_ENV=prod` 时 `GP_TRUSTED_DB_DSN` 必填且不能与主 DSN 相同
- 单元测试覆盖缺失/共享凭证场景

## 尚未完成（M4）

- 对象存储 **预签名上传/下载**（当前证据仅元数据）
- DSH gateway 真实 Node/DSH 镜像与 egress 白名单
- 数据库备份/恢复演练与 PITR
- 配额/限流压测与容量基线
- 生产镜像 GHCR 推送与签名
- 部署到非 kind 环境
- 真实执行器（当前 run 结果仍为手工 seed；coverage-sync 已可用）

## 结论

M4 的 API 安全基线已落地并验证；生产化关键项仍受环境（网络/对象存储/镜像仓库）限制。

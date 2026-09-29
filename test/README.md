# 集成测试基座 (test/)

本目录放置需要**真实基础设施**的集成测试（kind 集群内 PG/Redis/Temporal/对象存储(SeaweedFS)）。

## 约定

- 文件用 `//go:build integration` 标记，默认 `go test ./...` 不编译（依赖真实环境）。
- 通过环境变量读取依赖地址（见 `setup.Deps`，默认 127.0.0.1 端口契约，对应 GP0-03/.env 避让 im-saas）。
- 依赖未就绪时 `setup.SkipIfUnavailable` 自动跳过，避免 CI 无集群时误报失败。
- 显式跳过：`GP_SKIP_INTEGRATION=1`。
- CI 使用 testcontainers 回退（见 IMPLEMENTATION-PLAN GP0-09）。

## 运行方式

```bash
# 本地已就绪 kind 集群后
go test -tags=integration ./test/...
```

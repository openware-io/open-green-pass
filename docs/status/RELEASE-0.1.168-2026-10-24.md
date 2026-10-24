# GreenPass 0.1.168-SNAPSHOT 功能记录（2026-10-24）

## 新增

- GatePage 增加“真实覆盖缺口”面板：
  - 输入项目 ID
  - 调用 `GET /coverage/gaps`
  - 展示 required/mapped/passed/failed/blocked/waived/unmapped
  - 按模块列表展示
- 运行时 `/version` 返回真实 version/commit/built_at

## 验证

```text
/version = 0.1.168-SNAPSHOT
commit   = f7a503e54a6a0446be817a5db6006f8c631e7320
built_at = 2026-10-24T02:56:05Z
UI bundle 包含“真实覆盖缺口”
go test ./...        PASS
npm run typecheck    PASS
arbiter 26/26        PASS
```

## M3

```text
required=417
mapped=105
passed=90
failed=7
blocked=8
unmapped=312
waived=24
design_coverage=25.2%
```

## 未完成

```text
M3 312 Cell 缺测试数据/合同/fixture
M4 egress 在 kind CNI 下未生效
P0/P1 未关闭
0.1.168 镜像尚未推送/签名
```

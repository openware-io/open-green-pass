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

## 镜像\n\n```text\nweb  sha256:efdcb6543390102c1365a05202f27a38fe26f76fbb3cf646405934e5fd63e6d0\ndsh  sha256:10bb114da7614d0336efd5c78e7b2f11313a83d17c9707f865036715d1a5fd37\n```\n\n## 未完成

```text
M3 312 Cell 缺测试数据/合同/fixture
M4 egress 在 kind CNI 下未生效
P0/P1 未关闭
镜像已推送；server 复用 0.1.167 已签名 digest，web/dsh 新 digest 已签名验证
```

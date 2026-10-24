# GreenPass 0.1.162-SNAPSHOT 功能完成记录（2026-10-24）

## 新增功能

### 覆盖率 waiver API

```text
POST   /coverage/waivers
GET    /coverage/waivers?project_id=
DELETE /coverage/waivers/{id}
```

- 持久化 reason / owner / approved_by / expires_at / evidence
- Cell 标记 `waived`，门禁分母排除
- 过期 waiver 自动失效，Cell 重新计入 required
- 撤销后 Cell 回到 planned
- OpenAPI 已登记

### 验证

```text
创建 -> 201
列表 -> 200
撤销 -> 204
生效时 total_required 441 -> 440
过期/撤销后 440 -> 441
```

## 版本

```text
0.1.162-SNAPSHOT
schema 48 clean
arbiter 26/26 PASS
```

## 镜像

```text
open-green-pass:0.1.162-SNAPSHOT
sha256:928bda7705176cd302ee39b250ca34c7a7502fcb110800d9077e610e9b55fa5b

open-green-pass-dsh-gateway:0.1.162-SNAPSHOT
sha256:5c22e47421233cf8cde51ef8275503e06ddfad9bbd2c60a48a6d6690527cf68d
```

server 与 dsh-gateway 新 digest 已 cosign 签名并 verify 通过；web/migrate 复用已签名的 digest。

## 仍未完成（非功能开发）

```text
M3 336 个 Cell 缺测试数据/合同
M4 egress 在 kind 默认 CNI 下未生效
P0/P1 未关闭
```

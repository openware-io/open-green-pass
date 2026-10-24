# GreenPass 0.1.164-SNAPSHOT 功能与发布验证（2026-10-24）

## 版本

```text
0.1.164-SNAPSHOT
schema 48 clean
arbiter 26/26 PASS
```

## 新增功能

```text
GET  /coverage/gaps
POST /coverage/waivers
GET  /coverage/waivers
DELETE /coverage/waivers/{id}
```

- 覆盖缺口按模块/维度聚合
- waiver 可审计、可过期、可撤销
- waiver 生效时从门禁分母排除，过期自动恢复
- ExecPage 展示真实 CaseResult evidence，不再展示 mock 哈希

## 镜像

```text
open-green-pass:0.1.164-SNAPSHOT
sha256:aafcd0b33012e4dfc170d0c2a56fe9628696470a094ec4ce93dfd075226bcd44
（已签名并 verify 通过）

open-green-pass-web:0.1.164-SNAPSHOT
sha256:083dd2c5fa24ffcea54d0582840a7f0b89dddec4000f7aa242100d0b7d57ec11
（已签名并 verify 通过）
```

dsh-gateway / migrate 的 digest 与上一版相同，沿用已签名 digest；GHCR 网络抖动导致 0.1.164 tag 未全部重新推送。

## 当前 M3

```text
required=441
mapped=105
passed=90
failed=7
blocked=8
unmapped=336
```

## 未完成（外部条件）

```text
M3 336 Cell 缺测试数据/合同
M4 egress NetworkPolicy 在 kind CNI 下未生效
P0/P1 未关闭
```

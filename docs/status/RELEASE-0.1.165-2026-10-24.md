# GreenPass 0.1.165-SNAPSHOT 发布验证（2026-10-24）

## 版本

```text
0.1.165-SNAPSHOT
schema 48 clean
arbiter 26/26 PASS
```

## 功能

```text
GET /coverage/gaps 增加 waived 统计
```

## 镜像

```text
open-green-pass:0.1.165-SNAPSHOT
sha256:58181ea3c94b2637b3ac0ca82a09f5251917519beb754c8ba8118bd9179b3c06
（签名并 verify 通过）

open-green-pass-web:0.1.165-SNAPSHOT
sha256:083dd2c5fa24ffcea54d0582840a7f0b89dddec4000f7aa242100d0b7d57ec11
（沿用已签名 digest）

open-green-pass-migrate:0.1.165-SNAPSHOT
sha256:d16d64417dcca673d1fa184012e071620195f340fd420711fc34b84fb46fe38f
（沿用已签名 digest）

open-green-pass-dsh-gateway:0.1.165-SNAPSHOT
sha256:910a980275c904bc97cbfa2ac077c545adc63ef73f3ec0237072c7136b726cda
（签名并 verify 通过）
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
```

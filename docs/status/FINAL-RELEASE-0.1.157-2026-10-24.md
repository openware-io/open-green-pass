# GreenPass 0.1.157-SNAPSHOT 发布验证（2026-10-24）

## 版本

```text
version = 0.1.157-SNAPSHOT
schema  = 48 clean
```

## 集群

```text
gp-server       2/2
gp-worker       2/2
gp-web          1/1
gp-dsh-gateway  1/1
```

## 仲裁

```text
26/26 PASS
```

## GHCR

已推送并远程拉取成功：

```text
ghcr.io/openware-io/open-green-pass:0.1.157-SNAPSHOT
digest sha256:efc2df3d1fc38f318b69e2d535cb66fa70e9c2050663f7636c3295d9decd6c9f
```

4 个镜像均已 cosign 签名，`cosign verify` 全部通过。

## 备份恢复

```text
cas_case=32
gen_run=7
cost_price=1
```

## M2

浏览器证据上传/下载已验证：

```text
14B 上传 201
3MiB 上传 201
content 下载 200
```

## M3

```text
18/18 模块有覆盖账本和执行入口
required=427
mapped=102
passed=88
failed=6
blocked=8
design_coverage=23.9%
execution_coverage=23.9%
```

仍未达到全量维度覆盖。

## M4

```text
备份恢复：通过
GHCR 推送：通过
cosign 签名：通过
egress NetworkPolicy：未生效（kind 默认 CNI）
```

## 发布结论

```text
M2 / M4 备份与制品链路可验证
M3 全量覆盖未完成；M4 egress 未验证
不能宣称整个目标完成
```

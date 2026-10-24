# GreenPass 0.1.167-SNAPSHOT 发布验证（2026-10-24）

## 版本

```text
0.1.167-SNAPSHOT
schema 48 clean
arbiter 26/26 PASS
```

## 新增

运行时可读取真实构建元数据：

```json
{
  "version": "0.1.167-SNAPSHOT",
  "commit": "59225d9c53a41ab5dbccc95a892aefd326e4e971",
  "built_at": "2026-10-24T02:46:43.0609982Z",
  "api_version": "0.1.0"
}
```

`deploy-kind.ps1` 在部署时注入：

```text
GP_VERSION
GP_COMMIT
GP_BUILT_AT
```

## 镜像

```text
open-green-pass:0.1.167-SNAPSHOT
sha256:c1db29a52df91fe5c4b91f003f2320d61b2991ef2edc85b151e204241c1b3b55
（已签名并 verify 通过）
```

web/migrate/dsh-gateway 的 digest 与上一版相同，沿用已签名 digest。

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

# GreenPass 0.1.169-SNAPSHOT 功能记录（2026-10-24）

## 新增

- GatePage 覆盖率豁免面板：
  - 输入 Cell ID、原因、owner、过期日期
  - 创建 waiver
  - 按项目加载 waiver
  - 撤销 waiver
- 覆盖缺口面板保持可用

## 验证

```text
UI bundle 包含「覆盖率豁免」「真实覆盖缺口」
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
0.1.169 镜像尚未推送/签名
```

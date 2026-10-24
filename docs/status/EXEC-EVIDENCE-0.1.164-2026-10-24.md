# 执行证据接线（0.1.164-SNAPSHOT）

## 变更

`ExecPage` 的真实运行结果不再展示 mock 证据：

- 读取 `CaseResult.evidence`
- 展示：
  - 证据哈希
  - 日志条数
  - 截图数量
  - 用例状态
- 无真实结果时显示“暂无真实执行证据”

原先固定的：

```text
sha256:a3f8…c21d
minio://evidence/
✓ 已锚定
```

已移除，避免把原型数据当成真实证据。

## 验证

```text
npm run typecheck PASS
arbiter 26/26 PASS
UI bundle 包含「执行证据」
```

## 版本

```text
0.1.164-SNAPSHOT
schema 48 clean
```

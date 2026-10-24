# 重复执行修复（0.1.171-SNAPSHOT）

## 问题

`POST /runs` 创建的 run 已由 schedule dispatcher 自动入队执行。调用方再调用：

```text
POST /runs/{id}/execute
```

会再次执行同一 run，导致用例出现 `attempt_seq=1/2` 两条结果，重复消耗配额、证据和成本。

复现：

```text
run 2908158313598976
61 cases
122 results
```

## 修复

`executeRun` handler 在 schedule queue 启用时：

```text
queued / scheduled -> 202 Accepted
```

不再同步执行，等待调度器执行。

未启用 schedule queue 时仍允许手动执行，保持测试/无调度模式兼容。

## 验证

run `2908188712701952`：

```text
POST /execute -> 202
final state -> done
case result count:
  2908157048131584 -> 1
  2908157057044480 -> 1
```

没有重复 attempt。

## finding

```text
2908160974196736  P1
手动 execute 与调度器重复执行同一 run
```

已按 triage -> confirm -> pending_regression -> close 关闭。

## 版本

```text
0.1.171-SNAPSHOT
schema 48 clean
```

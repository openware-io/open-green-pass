# Generation Arbiter（生成改造仲裁监控）

## 目的

独立判断“生成用例改造”是否全量完成，不依赖实现者的自述。

仲裁代理只读检查运行时、数据库、API、页面和测试；发现缺口时返回非零，修复后重新运行，直到全部通过。

## 运行

```powershell
$env:GP_ARBITER_USERNAME = "zhangli"
$env:GP_ARBITER_PASSWORD = "<password>"
pwsh -NoProfile -File .\scripts\dev\arbiter-generation.ps1
```

快速检查（跳过 Go/前端测试）：

```powershell
pwsh -NoProfile -File .\scripts\dev\arbiter-generation.ps1 -SkipTests
```

## 检查范围

### 运行态

- gp-server / gp-worker / gp-web / gp-dsh-gateway ready
- schema 版本 48 且非 dirty

### 数据

- `gen_run` / `gen_run_unit` / `cost_price`
- `cost_price` 的 model/input/output/cache/effective 字段
- `cost_line_item` 的拆分单价、cache token、price_missing
- TTL 函数 `gp.purge_generation_artifacts`

### API

- 登录
- run 列表 / run 详情
- 成本价目
- 成本明细

### 证据

- 取消 run 的候选与源码快照已清理
- 存在 succeeded run
- 存在 `case_generate` 且金额 > 0 的成本行
- 实际模型为 `dsh/deepseek-*`
- run 聚合 tokens / cost

### 页面

- 最近生成任务 / 新建生成任务
- run 级审核按钮
- 模型价目表

### 测试

- `go test ./...`
- `npm run typecheck`

## 输出

- 控制台逐项 PASS/FAIL
- `docs/status/ARBITER-COMPLETION-2026-10-23.md`
- 任一项失败：exit code 1

## 仲裁规则

1. 不以“代码已提交”判定完成，只以运行时证据判定。
2. 不以“页面能打开”代替 API 与数据检查。
3. 成本必须有真实模型、真实 token、非零金额；只有 token 没有价格不算完成。
4. 取消必须证明候选和源码快照为 0。
5. 测试和 typecheck 必须真实执行。
6. 失败项修复后必须重跑，不允许保留已知 FAIL。

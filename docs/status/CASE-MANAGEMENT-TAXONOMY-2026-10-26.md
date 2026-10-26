# 用例管理专业化改造（0.1.179-SNAPSHOT）

## 分类模型

采用 QA 资产分层：

```text
业务域 business_domain
  -> 功能 feature
    -> 模块 module
      -> 操作 operation
```

并增加：

```text
测试类型 case_type：functional / api / security / performance / compatibility
优先级 priority：P0 / P1 / P2 / P3
自动化状态 automation_status：automated / manual
```

## 数据

migration 49：

```text
cas_case.business_domain
cas_case.feature
cas_case.module
cas_case.operation
cas_case.case_type
cas_case.priority
cas_case.automation_status
```

回填：

```text
migration 50：从 coverage_cell_ids 补 module/domain/type
migration 51：从中文标题推断 module/domain
migration 52：从 m3- 用例 code 前缀推断 module/domain
```

当前：

```text
356 cases
338 classified
```

未分类的 18 条保留空值，UI 显示“未分类”，不伪造。

## API

```text
POST  /cases                 创建时写入 taxonomy
GET   /cases?module=&priority=
PATCH /cases/{id}/metadata   只改分类，不创建脚本版本
```

## UI

用例资产库：

```text
业务域 -> 功能 -> 模块 -> 操作
类型 / 优先级 / 自动化
按模块、优先级筛选
“编辑分类”行操作
```

## 生成页

“最近生成任务”在 succeeded/failed/cancelled 时提供：

```text
再次生成
```

不需要回到页面顶部重新配置；沿用当前项目/仓库/分支/服务上下文创建新任务。

## 验证

```text
POST /cases taxonomy -> 201，字段回显
PATCH /cases/{id}/metadata -> 200，operation/P0 更新
UI bundle 包含“业务域”“再次生成”
```

## 追加：筛选、排序与按操作分组（0.1.180）

新增筛选：

```text
业务域 / 功能 / 模块 / 操作 / 优先级
```

默认排序：

```text
业务域 -> 功能 -> 模块 -> 操作 -> 标题
```

生成用例的 operation 自动回填为：

```text
METHOD /api/v1/...
```

因此同一端点的 happy / authz / invalid_filter / pagination 等覆盖会归入同一操作分组。当前：

```text
357 cases
347 有 operation
```

表格按分组渲染，组头显示：

```text
业务域 / 功能 / 模块 / 操作（N 条）
```

这样同一操作的多种覆盖用例连续排列，便于阅读和评审。

## 0.1.181 追加

- 新建用例自动从 `script.request.method/path` 生成 `operation`，例如 `GET /api/v1/admin/groups`。
- 用例列表默认按 `业务域 → 功能 → 模块 → 操作 → 标题` 排序。
- 页面按操作分组渲染，组头显示覆盖条数。
- 筛选支持业务域、功能、模块、操作、优先级。

验证：

```text
auto_operation = GET /api/v1/admin/groups
security 用例按 DELETE /... 等操作分组
0.1.181-SNAPSHOT / schema 53 / arbiter 26/26 PASS
```

## 0.1.182 追加

- 业务域/功能/模块/操作筛选改为下拉选择，选项来自当前用例库。
- 补齐已分类用例的 feature/business_domain：
  ```text
  358 cases
  339 module / 339 feature / 339 domain
  ```
- 未分类 19 条保留空值并显示“未分类”。

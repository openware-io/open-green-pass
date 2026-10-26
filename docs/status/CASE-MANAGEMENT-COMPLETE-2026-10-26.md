# 用例管理专业化收口（0.1.189）

## 1. 分类与筛选

```text
业务域 -> 功能 -> 模块 -> 操作
```

全部可下拉筛选，另支持优先级、评审状态。

## 2. 排序与分组

默认：

```text
业务域 -> 功能 -> 模块 -> 操作 -> 标题
```

表格按操作分组；同一端点的 happy/authz/invalid_filter/pagination 等覆盖连续排列。

生成导入时 `CaseImportService` 也按 taxonomy 排序返回，数据库列表按同样顺序查询。

## 3. 业务动作字典

`formatOperation` 将 METHOD PATH 转为：

```text
登录 · POST /api/v1/auth/login
查询安全治理 · GET /api/v1/admin/security/violations
创建/提交群组管理 · POST /api/v1/admin/groups
```

原始方法/路径保留。

## 4. 需求/缺陷追溯

新增字段：

```text
requirement_ref  需求/用户故事引用
defect_ref       关联缺陷引用
```

创建、详情展示、metadata PATCH 均支持。

## 5. 评审状态

新增：

```text
review_status   draft / pending / approved / rejected
review_comment  评审意见
```

列表：

- 评审状态筛选
- “通过”“退回”行操作
- 详情展示评审状态和意见
- 筛选清空同时清空评审筛选

## 6. QA 规格字段

```text
preconditions 前置条件
steps         测试步骤
expected_result 预期结果
```

写入版本脚本 JSON，详情结构化展示。

## 7. 验证

```text
version 0.1.189
schema 55
go test ./... PASS
typecheck PASS
arbiter 26/26 PASS
```

API 验证：

```text
create review=pending, requirement_ref=REQ-003, defect_ref=BUG-003
PATCH review_status=approved -> approved
taxonomy 保持不变
```

当前：

```text
358 cases
339 classified
19 未分类
```

## 0.1.192 全局用例库

- 用例管理默认加载全部项目，不再绑定当前被测对象。
- “全部项目”下拉可筛选单个项目。
- 表格分组键：项目 → 业务域 → 功能 → 模块 → 操作。
- 新建用例仍选择目标项目；创建后刷新全局库。
- 跨项目列表仍按每个 target 单独授权查询，不绕过租户/权限。

## 0.1.193 仓库维度

- `cas_case.repo_id` 提升到用例主体，migration 56。
- 从 current version `source_repo_id` 回填：32/361 条已有仓库。
- 用例库分组改为：
  ```text
  项目 -> 仓库 -> 业务域 -> 功能 -> 模块 -> 操作
  ```
- 新增“全部仓库”筛选。
- 新建用例可选择当前项目下的仓库；repo_id 写入用例。
- 旧用例没有仓库时显示“未绑定仓库”，不伪造。

## 0.1.195 修正

- 修复筛选/分组标签前的游离单引号（编辑 JSX 时误插入的字符）。
- 回填 m3-/GEN- 管理端用例的仓库：352/361 已绑定 open-chat-admin。
- 剩余 9 条没有可靠仓库来源，显示“未绑定仓库”，不伪造。
- 新建用例表单可选择当前项目下的仓库，写入 repo_id。

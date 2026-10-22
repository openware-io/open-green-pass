# DSH 用例生成对照评估（2026-10-22）

## 1. 实验设置

| 项目 | 值 |
|---|---|
| 仓库 | `open-chat-admin` 2.2.1 |
| 冻结提交 | `a44876435f69f78723e23c04d2ff1d59e1d05f96`（Tag 2.2.1） |
| 规模 | 77 个文件；18 个 API 模块；87 个 HTTP 端点（81 个 Bearer 保护、4 个公开、2 个预签名对象存储） |
| 基线 | GreenPass 当前链路：GitHub 文件快照 + 单轮 Prompt + DeepSeek V4 Pro，批次 `2902560158584832` |
| 实验组 | DSH `--profile headless`，同一仓库只读分析，使用 DeepSeek 路由 |
| 实验约束 | 只读；不修改文件、不安装依赖、不执行测试；输出严格 JSON |

## 2. 结果对照

| 指标 | 当前 GreenPass 单轮 API | DSH headless |
|---|---|---|
| 用例数 | 5 | 40 |
| 覆盖 API 模块 | 5 / 18 | 18 / 18 |
| 直接触达端点 | 5 / 87（5.7%） | 43 / 87（49.4%） |
| HTTP 方法 | 仅 GET | GET 18 / POST 14 / PUT 3 / DELETE 5 |
| 场景分类 | 5 条 normal | 22 normal / 3 boundary / 7 error / 1 auth / 3 idempotency / 1 concurrency / 3 data |
| 断言总数 | 5（每条仅 status=200） | 170（平均 4.25 条/用例） |
| 来源引用 | 5 个文件 | 40/40 条含文件+行号 |
| 优先级 | 无 | P0 9 / P1 22 / P2 9（DSH 自报 10/19/11，与用例不一致） |
| 耗时 | 约 40 秒 | 约 5 分钟 |
| 输出规模 | 约 5 条 JSON | 约 445 KB（含推理+JSON） |

### 基线样例

5 条用例全部是“GET 列表 → 期望 200”，没有参数、鉴权、错误、边界、数据一致性断言。例如：

```json
{
  "title": "获取消息列表",
  "request": {"method": "GET", "path": "/admin/messages", "body": null},
  "assertions": [{"type": "status", "expected_status": 200}]
}
```

### DSH 样例

DSH 的用例包含前置条件、步骤、请求体、具体断言、清理和源码位置。例如登录：

```json
{
  "id": "TC-001",
  "module": "auth",
  "priority": "P0",
  "category": "normal",
  "title": "管理员用户名密码登录成功并签发管理员会话",
  "request": {"method": "POST", "path": "/api/v1/auth/login", "body": {"username": "${ADMIN_USERNAME}", "password": "${ADMIN_PASSWORD}"}},
  "assertions": [
    "response.status === 200",
    "typeof response.body.access_token === 'string' && response.body.access_token.length > 20",
    "response.body.user && response.body.user.role === 'admin'",
    "followup.status === 200 && Array.isArray(followup.body.items)"
  ],
  "source": {"file": "src/api/auth.js", "line": 3}
}
```

## 3. DSH 明显更好的地方

- 能从 `src/api/*.js`、`src/views`、`src/router` 建出 18 模块、87 端点的清单，而不是只取几个文件。
- 能识别请求头契约、401/403 处理、分页 envelope、状态枚举、错误码表。
- 能区分正常/边界/错误/权限/幂等/并发/数据一致性，而不是每个模块只写一条 GET。
- 能指出 `group`、`server` 是无 UI 消费方的模块，`server/logs.vue`/`performance.vue` 使用 mock，降低契约可信度。
- 能给出 gaps/risks，例如缺少后端 OpenAPI、角色矩阵、fixtures、预签名上传、幂等语义。

## 4. 仍然不满足“全量、可靠、专业、覆盖全面”的地方

### 4.1 覆盖率仍不足

40 条用例只直接触达 43/87 个端点（49.4%）。DSH 自己列出的未覆盖项包括：

- 登录限流/锁定、SSO、忘记密码成功路径、重置密码成功；
- 用户 403/`USER_NOT_FOUND`/`CANNOT_DELETE_SELF`；
- 群成员/解散群；
- 配置 `PUT /admin/config`、批量更新原子性；
- 敏感词更新/删除、违规创建、举报完整状态机；
- 媒体上传完成、分片签名/完成、断点续传、取消、对象状态、校验和；
- 客户端发布 update/rollout/pause/resume/withdraw/archive、SemVer/SHA 校验、下载域名限制；
- 监控 server、统计趋势、账号注销日志等。

### 4.2 断言可靠性不足

- 18 个模块中 12 个没有可见的稳定错误码；7 条 error 用例里只有 4 条断言了具体 domain code，其余只断言“code 非空”。
- 没有 403/角色矩阵/租户隔离用例；源码里只有 token 存在性检查，无法从代码推出权限语义。
- 没有 fixtures/数据库重置；`approve` 等不可逆状态只能靠“重置 fixture DB”清理。
- 媒体/客户端发布涉及预签名对象存储，源码无法证明 bucket、CORS、ETag、清理策略。
- 幂等/乐观锁语义是推断：`expectedRowVersion` 从哪来、冲突码是什么，源码没有完整契约。
- DSH 自报 P0/P1/P2 为 10/19/11，实际用例统计是 9/22/9，说明它自己的汇总也可能出错，必须做程序化校验。

### 4.3 没有执行验证

这次是只读源码分析，没有执行任何用例。用例的 expected status/body 很多是**推断**，不是运行结果。对于“可靠”的要求，必须至少执行一轮并修正断言。

## 5. 对“大型项目全量生成”的判断

**不满足。** DSH 能显著提升“从代码解析出用例清单”的质量，但单次 headless 分析仍然不是大型项目全量生成的可行方案。

原因：

1. **上下文和工具预算**：77 个文件、87 个端点已经需要约 5 分钟、445 KB 输出；大型单体/多仓项目有上千端点，单个 agent 会超时、截断或漏读。
2. **覆盖率不是线性增长**：40 条覆盖 49.4% 端点；要覆盖到 80-90%，需要按模块/端点/场景分层生成，而不是让一个 agent “尽量多写”。
3. **源码不等于契约**：后端状态码、角色矩阵、幂等语义、预签名上传、数据约束不在前端源码里；纯代码解析无法生成可靠 oracle。
4. **没有执行闭环**：生成后必须跑起来，才能验证断言、测试数据、清理和状态机；仅静态生成无法保证“可靠”。
5. **缺少覆盖模型**：专业覆盖需要需求/风险/接口 schema/历史缺陷共同定义，不能只用“文件覆盖率”或“用例条数”衡量。

## 6. 建议的工程化路径

### 阶段 1：DSH 作为“模块级用例设计师”

- 以 service/module 为单位调用 DSH，而不是整仓一次。
- 输入：模块源码 + 路由 + OpenAPI（若有）+ 现有用例 + 风险清单。
- 输出：20-40 条候选 + 覆盖矩阵 + 未覆盖原因。
- 人工审核后进入 GreenPass 批次，不做“直接全量入库”。

### 阶段 2：生成 + 执行闭环

- GreenPass Temporal 增加执行 activity：准备测试环境/租户/账号/fixtures。
- 对可执行 API 用例跑一轮，记录实际 status/body，回写候选断言。
- 失败的用例进入修复/人工审核，而不是直接标记“通过”。

### 阶段 3：分层多 Agent + 覆盖账本

- 项目 → 服务 → 模块 → 端点 → 场景分层。
- 共享 coverage ledger，避免重复和漏项。
- 每个模块独立 DSH job，汇总后做去重、优先级和风险加权。
- 对大型项目按 diff 增量生成，而不是每次全量。

### 阶段 4：DSH runner 产品化

- `dsh --profile headless` 每模块一个 K8s Job，或 `dsh --profile sdk` 长驻服务。
- 独立 workspace、工具白名单、非 root、只读根文件系统、超时/配额。
- 保存 transcript、tool calls、token 成本，映射 GreenPass 审计。

## 7. 结论

- **DSH 比当前单轮 API 强很多**：5 → 40 条、5 → 18 个模块、5.7% → 49.4% 端点触达、1 → 4.25 条断言/用例。
- **但“大型项目全量生成可靠且专业覆盖全面的用例”仍然不满足**：49.4% 端点覆盖、12/18 模块无稳定错误码、无执行验证、无角色/租户矩阵、无 fixtures、无预签名上传验证。
- **正确目标不是“让一个 Agent 一次生成全量”**，而是“分层生成 + 覆盖账本 + 执行反馈 + 人工审核”的组合。
- DSH 适合做其中的**模块级分析与用例设计器**，不适合作为单次全量生成器。

## 8. 产物

- [DSH 40 条用例与覆盖矩阵](DSH-GENERATION-EVALUATION-2026-10-22.cases.json)
- [当前基线 5 条用例](DSH-GENERATION-EVALUATION-2026-10-22.baseline.json)
- DSH 原始输出（含推理，未入库）：`%TEMP%\gp-dsh-eval\dsh-output.txt`

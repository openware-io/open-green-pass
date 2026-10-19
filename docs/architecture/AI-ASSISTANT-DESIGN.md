# GreenPass AI 助手架构设计

> 状态：架构评审稿 v0.1 · 2026-10-15
>
> 本文回答两个问题：GreenPass 应在哪里提供 AI 助手，以及如何让“随时对话”的显式助手与“自动参与流程”的隐式助手共用一套受控运行时。本文先定边界和接口，不直接引入业务实现。

## 1. 结论先行

GreenPass 不应增加一个孤立的聊天页面，也不应让每个业务模块各自调用模型。建议增加一个横切的 **Assistant Runtime（助手运行时）**，位于现有 `internal/ai` 能力之上、业务域之旁：

```text
显式入口                         隐式入口
全局助手 / 页面助手 / 命令操作       领域事件 / Temporal Activity / CI 触发
        \                              /
         v                            v
              Assistant Runtime
     会话、上下文、技能、工具、审批、流式事件
                     |
          Context / Tool / Policy Ports
       _________/__________|____________\_________
      v                    v                       v
  治理域                执行域                  可信域
  对象/用例/契约        Run/证据/资源           门禁/审计/成本
                     |
                 AI Gateway
          模型路由、密钥、计量、成本、追踪
```

核心判断：

1. **助手是控制面能力，不是新的业务真相源。** 用例、运行、门禁和审计的最终状态仍由现有领域服务负责。
2. **显式助手与隐式助手共用运行时，但触发策略不同。** 显式由用户发起，隐式由事件和工作流发起。
3. **AI 输出先成为可追溯的 Artifact，再由策略决定是否允许执行。** 模型不能直接写数据库、调用任意 HTTP 或执行测试命令。
4. **确定性规则优先于模型判断。** OPA、版本校验、权限、配额和状态机是硬约束；AI 只能解释、补充和提出候选方案。
5. **隐式助手不操作界面；显式助手以界面引导为首选。** 后台能力只产出观察、诊断、草稿与待处理事项；用户主动求助后，助手在当前 UI 中定位、解释、逐步引导和预填，由用户确认提交。
6. **每个助手操作均有可验证审计。** 无论是协助操作、自动操作、被拒绝操作，还是界面代理动作，均有操作主体、委托人、权限决策、输入摘要、结果和证据哈希。
7. **第一阶段不引入独立向量数据库。** GreenPass 的主要事实已经存在于 PostgreSQL、审计链和 S3 证据中，先做结构化上下文和证据引用，后续有明确召回问题再增加 pgvector。

## 2. 两类助手的职责边界

### 2.1 显式助手（用户随时唤起）

显式助手解决“我现在想知道什么、下一步该做什么”：

| 入口 | 典型问题 | 默认权限 | 输出 |
|---|---|---|---|
| 全局助手 | “为什么本次门禁失败？”、“帮我找出最近变慢的服务” | 当前用户可见数据的只读权限 | 解释、证据引用、导航、建议 |
| 页面上下文助手 | 在画像页问“这个服务缺哪些测试”；在运行页问“失败根因是什么” | 当前页面资产 + 用户权限 | 局部分析、对比、下钻 |
| 命令操作 | “生成 API 边界用例草案”、“为本次失败创建复测计划” | 读 + 生成候选物 | Draft、计划、待审批动作 |
| 审批对话 | “采用这 3 条用例并提交新版本” | 需要对应写权限和二次确认 | 调用领域命令，返回提交结果 |

显式助手必须支持：

- 流式输出和中断；
- 当前团队、资产层级、页面、Run、Case 等上下文的显式展示；
- 每条事实的来源（数据库记录、证据对象、审计事件、规则版本）；
- 将建议转为草稿，而不是直接修改生产数据；
- 对有副作用的动作显示影响范围、成本估计和明确的下一步界面操作；默认由用户在页面完成确认与提交。

### 2.2 隐式助手（流程中的参与者）

隐式助手解决“流程运行时自动补足认知工作”：

- **观察者**：订阅事件，识别异常、缺口、回归和风险；
- **分析者**：读取限定上下文，形成诊断或评分解释；
- **规划者**：生成用例草案、复测计划、资源建议或修复建议；
- **验证者**：用确定性规则、执行结果和证据检查模型输出；
- **提案者**：发布可供人或策略消费的 Artifact。

隐式助手默认是 **observe → infer → propose**，运行在后台、没有浏览器会话、也不发送 UI 指令。它把结果写为 Artifact、通知或待处理事项；默认不自动执行 `commit`。只有明确配置为低风险、幂等且可回退的后台动作，才允许自动提交；例如写入“分析结果”或创建待审核草稿。它不能：

- 绕过 OPA 门禁、版本校验、RBAC、配额或资源冲突检查；
- 直接修改用例正式版本、门禁规则、Run 状态或审计链；
- 读取当前租户之外的数据；
- 把未经验证的模型文本当作测试证据或合规结论。

## 3. 运行时分层

### 3.1 Interaction Layer

前端提供三个可复用组件，而不是每页自建聊天逻辑：

1. `AssistantLauncher`：全局唤起、快捷键、当前资产和页面上下文注入。
2. `AssistantPanel`：以右侧完整 Dock 工作区承载会话、流式消息、证据引用、建议卡与“带我操作”入口；默认不加遮罩、不锁定主页面，面板严格绑定浏览器可视内容区高度，消息区占用剩余高度、输入区固定底部。宽度按视口以 `clamp` 自适应，主内容区同步收窄；顶部入口是可见的开关状态，使用户能对话与操作并行进行。这是显式助手的主入口，不以独立协作页替代用户对话。
3. `GuidedStepCard`：高亮当前页面目标、解释目的和风险、预填合法草稿、等待用户完成或跳过。

页面只声明 `AssistantContextDescriptor`（当前对象、页面、可用动作、时间范围），不拼接 Prompt，不直接调用模型。

### 3.2 Assistant Runtime

建议新增 `internal/assistant`，保持现有分层：

```text
internal/assistant/
  api/            HTTP/SSE、请求校验、响应映射
  application/    会话、Turn、技能编排、审批、幂等
  domain/         Session/Turn/Run/Artifact/ToolPolicy 等纯领域对象
  infra/          PG、对象存储、事件订阅、工具适配器
  workflow/       Temporal 中的长时分析和隐式助手工作流
```

`internal/assistant/domain` 只依赖标准库、`pkg` 和本域接口；不得依赖 Temporal、数据库驱动或具体模型 SDK。现有 `internal/ai` 继续负责模型网关与生成管道，助手通过端口调用它，不复制模型路由和计量逻辑。

运行时包含以下能力：

- **Session Manager**：会话生命周期、团队/用户/资产范围、过期与归档；
- **Turn Orchestrator**：一个用户问题或一个事件触发的一次助手运行；
- **Skill Registry**：技能名称、输入输出 Schema、上下文需求、风险级别、预算；
- **Context Assembler**：从授权后的结构化查询和证据索引组装上下文快照；
- **Tool Broker**：只暴露注册工具，执行前做权限、参数、租户和副作用检查；
- **Proposal/Approval Manager**：把动作和草稿转为可审批对象；
- **Artifact Store**：保存结构化输出、证据引用、模型版本和决策链；
- **Event Publisher**：输出 SSE/事件总线通知，不把模型内部思考作为审计内容。

### 3.3 Context Layer

上下文必须是可复现的 `ContextSnapshot`，包含：

```text
tenant_id / actor_id / asset_scope / permission_snapshot
page / trigger / target_id / service_id / run_id / case_id
facts[]:       结构化事实及来源 ID
evidence[]:    S3 对象、sha256、证据类型及权限校验结果
policies[]:    OPA 规则版本、模型策略、资源配额
history[]:     经过筛选的历史结果和审计事件
redactions[]:  脱敏规则和被删除字段
```

上下文组装器不能接受模型自行提供的 SQL、URL 或对象键。每种上下文由一个类型化 Provider 提供，例如 `TargetContextProvider`、`RunContextProvider`、`GateContextProvider`。上下文快照写入对象存储或数据库引用，便于重放和审计；敏感正文按数据分类决定是否持久化。

### 3.4 Skill Layer

技能不是自由 Prompt，而是版本化、可测试的能力契约：

```text
Skill {
  id, version, trigger_kind
  input_schema, output_schema
  required_context[]
  allowed_tools[]
  risk_class: observe | propose | commit
  model_policy, budget_policy
  validation_policy
}
```

第一批建议技能：

| 技能 | 类型 | 结果 |
|---|---|---|
| `target.profile.explain` | 显式/隐式 | 服务画像解释、证据和置信度 |
| `generation.plan` | 显式 | 用例生成计划与覆盖缺口草案 |
| `generation.seed.review` | 隐式 | 生成种子质量问题、重复和缺失建议 |
| `case.coverage.gap` | 显式/隐式 | 需求、契约、用例覆盖缺口 |
| `case.assertion.review` | 隐式 | 断言强度与可变异性建议 |
| `run.failure.triage` | 隐式/显式 | 失败归因、证据链、复测计划 |
| `contract.impact.explain` | 隐式/显式 | 契约 diff 影响和消费者风险 |
| `gate.failure.explain` | 显式 | 门禁事实、规则、修复路径解释 |
| `report.summarize` | 隐式 | 报告摘要，不能改写原始结果 |
| `audit.anomaly.detect` | 隐式 | 成本、执行、操作异常候选 |

现有 `AnalyzeServiceProfile` 应逐步迁移为 `target.profile.explain` 技能的一个实现；数据库更新仍通过治理域应用服务完成，避免保留第二条画像写入路径。

### 3.5 Tool Layer

工具按副作用分为三层：

| 层级 | 示例 | 是否需要确认 |
|---|---|---|
| R0 Read | 读对象画像、Run、Case、门禁、审计、证据元数据 | 否，继承用户权限 |
| R1 Draft | 生成用例草案、复测计划、报告摘要、门禁解释 | 否，但必须标注 AI 产物 |
| R2 Command | 创建用例版本、启动 Run、提交审核、写回 CI | 是；由领域权限和动作策略双重检查 |
| R3 Sensitive | 修改门禁规则、模型连接、团队权限、删除/回退 | 默认禁用；仅在团队策略启用后，经强化确认和双人审批执行 |

Tool Broker 的唯一写入方式是调用现有应用服务端口，例如 `CaseService.CreateVersion`、`RunService.Create/Execute`、`GateService.Evaluate`。禁止工具适配器直接写业务表。每次 R2 调用都必须带 `request_id`、`idempotency_key`、`actor_id`、`assistant_run_id`。

### 3.6 AI 协助操作 GreenPass：显式界面引导优先，语义命令兜底

“AI 协助操作界面”分为三个明确模式。前两个是产品默认能力；第三个是例外能力，不能反过来定义日常交互：

| 模式 | 触发方式 | 用户看到的行为 | 实际执行路径 | 默认值 |
|---|---|---|---|
| 界面引导 | 用户显式提出“带我做/怎么操作” | 定位页面、逐步说明、展开详情、预填未提交表单、展示结果 | 前端受限 UI 指令 + 用户亲自提交领域命令 | 开启，首选 |
| 后台建议 | 事件或流程触发 | 通知中心/当前页面显示风险、建议和可跳转入口 | 后台 Artifact → 待处理事项；不操作 UI | 开启 |
| 受委托命令 | 用户明确选择“由助手执行” | AI 展示影响与预览，用户再次确认后完成一条受限命令 | Assistant Command Gateway → 领域应用服务 → 数据库/工作流 | 默认关闭，按次授权 |

**日常主路径是用户在界面完成操作，不是 AI 替人点击或替人提交。** 每个可操作能力先注册为 `GuidanceDescriptor`：目标路由、可高亮的语义 action、前置条件、解释文案、可预填字段 Schema、成功判据和关联 `CommandDescriptor`。它让助手能把“去哪里、为什么、填什么、提交后看什么”在界面中讲清楚，而不是只在聊天框给出文字步骤。

每个业务写入仍对应 `CommandDescriptor`，例如 `case.create_draft`、`case.commit_version`、`run.create`、`run.execute`、`generation.review`。用户点击页面原有提交按钮时，仍由页面调用该领域命令；助手只观察权威 SSE/查询结果并引导下一步。只有用户在预览中明确选择“由助手执行”时，模型才可产生“调用哪个已注册命令 + 合法参数”的候选，Tool Broker 再验证 Schema 和策略后调用领域服务。这样同一操作无论来自页面按钮、CLI、CI 还是例外的 AI 委托，都经过相同的状态机、版本校验、配额、冲突检测和审计端口。

**界面代理是一个窄能力层。** React 页面发布 `UIActionManifest`，供助手使用的条目只包含 `action_id`、页面/资产范围、可见文案、输入 Schema、风险级别和对应的 `CommandDescriptor`；不暴露任意 CSS selector、DOM 查询或 JavaScript 执行能力。助手可向浏览器发送下列受白名单限制的指令：

```text
ui.navigate(route, entity_ids)
ui.focus(action_id)
ui.prefill(action_id, schema_validated_draft)
ui.open_evidence(evidence_id)
ui.show_command_preview(command_id, impact)
```

这些指令只在用户显式发起的引导会话内发送，只能改变当前用户可见的界面状态，不能提交表单。真正提交始终由用户点击页面原有操作并调用后端命令 API，成功后助手通过 SSE/查询回显权威状态、解释结果并给出下一步。隐式助手不具备这些 UI 指令。若后续确有 API 未覆盖的遗留页面，可在隔离的无头浏览器中增加“视觉验证代理”，但它只验证渲染结果，**不得持有用户会话 Cookie、不得绕过 Command Gateway、不得作为写入方式**。

为少数受委托操作创建短时 `Delegation`：它绑定 `delegator_user_id`、团队、资产范围、命令集合、风险上限、过期时间、一次性 nonce 与 `assistant_run_id`。执行时重新鉴权；委托不是角色提升，AI 服务身份本身没有 owner/admin 权限。它不能由隐式助手或普通对话文本创建，必须由用户在命令预览中显式确认。

## 4. 显式与隐式接入矩阵

| 业务位置 | 显式助手 | 隐式助手 | 真正的业务写入者 |
|---|---|---|---|
| 被测对象画像 | 解释评分、追问风险、生成分析任务 | 画像分析、置信度不足提醒 | 治理域服务 |
| 上游源与生成 | 生成计划、查看批次、解释种子 | 解析、质量检查、缺口和重复检测 | Generation Workflow / CaseService |
| 用例库 | 查找覆盖缺口、生成草案、比较版本 | 断言强度、变异建议、漂移检测 | CaseService |
| 测试执行 | 解释当前进度、失败原因、生成复测计划 | 失败聚类、 flaky 候选、资源异常 | RunService / Temporal |
| 契约测试 | 解释破坏性变更和影响消费者 | diff 影响分析、缺口用例建议 | Contract domain |
| 质量门禁 | 解释“为何阻断”、列出事实和规则 | 门禁前风险预检、异常结果提醒 | OPA + GateService |
| 测试历史/报告 | 问答、横向比较、摘要 | 趋势异常、报告草稿 | ReportService |
| 审计/成本 | 查询归因、解释成本变化 | 异常调用、预算预警 | Trusted domain |
| CI/CD | 解释某次回写结果 | 事件触发分析和建议 | CI/CD outbox |

隐式助手不得同步阻塞核心执行链路。门禁前预检、失败分析和报告摘要采用旁路事件；只有生成管道中已存在的 AI Activity 才作为该工作流的必要步骤，并由 Temporal 重试、暂停和恢复。

## 5. 事件与工作流接入

### 5.1 事件统一为事实，不把 Prompt 当事件

建议逐步建立领域事件命名约定，先复用现有 Temporal/Redis 设施，量级需要时再拆独立事件总线：

```text
target.profile.requested / completed
generation.batch.quality_checked / review_required / committed
case.version.created / coverage_changed
run.started / case_failed / completed
contract.diff.detected
gate.evaluated
report.generated
cost.threshold_exceeded
```

事件只包含租户、资产、实体 ID、版本、发生时间和来源，不携带不受控的模型正文。消费者自行按权限读取上下文。

### 5.2 Temporal 使用边界

- 显式对话的短请求：HTTP + 流式响应，超时即结束，可由用户重试；
- 显式触发的长任务：创建 `AssistantRun`，转 Temporal Workflow，前端订阅状态；
- 隐式分析：事件触发 Temporal Workflow，具备幂等键、重试、预算和取消；
- 人工审批：Temporal Signal 或应用服务命令，不把审批状态藏在聊天消息里；
- 领域状态变更：由业务 Workflow/Service 完成，助手 Workflow 只能调用端口。

推荐的隐式工作流形态：

```text
ReceiveEvent
  -> AuthorizeAndSnapshot
  -> SelectSkillAndModel
  -> GatherContext
  -> GenerateCandidate
  -> DeterministicValidate
  -> PersistArtifact
  -> (optional) CreateApprovalTask
  -> EmitAuditAndCost
```

## 6. 数据与 API 契约

### 6.1 逻辑数据模型

新增表应遵守现有租户、RLS、审计字段和迁移只增规则。建议最小集合：

| 实体 | 关键字段 | 说明 |
|---|---|---|
| `assistant_session` | team, actor, scope, mode, status, expires_at | 显式会话或隐式任务容器 |
| `assistant_turn` | session, request_id, trigger, input_ref, context_ref, status | 一次用户问题/事件处理 |
| `assistant_run` | turn, skill, model, workflow_id, budget, status | 可重试的技能执行 |
| `assistant_tool_call` | run, tool, risk, args_hash, result_ref, status | 工具调用和幂等记录 |
| `assistant_delegation` | delegator, scope, command allow-list, risk ceiling, nonce, expires_at, revoked_at | 用户显式授予的短时、可撤销操作权；默认不创建 |
| `assistant_action` | delegation, command/UI action, sequence, input_hash, outcome, evidence_ref | 每一个 AI 界面引导、后台建议或例外执行动作的 append-only 操作账本 |
| `assistant_artifact` | run, type, target, payload_ref, evidence_refs, confidence, decision | AI 产物/草稿/提案 |
| `assistant_approval` | artifact, actor, decision, reason, decided_at | 人工确认记录 |
| `assistant_feedback` | artifact/turn, actor, label, comment | 质量闭环，不修改原始事实 |

Prompt、模型响应和大体量上下文不直接塞进业务表；使用对象存储引用、sha256、数据分类和保留期限。审计链记录引用、摘要和哈希，不记录完整敏感正文。

### 6.2 对外 API

在 `api/openapi.yaml` 中增加版本化契约：

```text
POST /assistant/sessions
POST /assistant/sessions/{id}/turns
GET  /assistant/sessions/{id}/events       (SSE)
GET  /assistant/runs/{id}
GET  /assistant/artifacts/{id}
POST /assistant/artifacts/{id}/approve
POST /assistant/artifacts/{id}/reject
POST /assistant/delegations
POST /assistant/actions/{id}/confirm
GET  /assistant/actions/{id}
```

请求上下文只允许传实体 ID 和页面标识，不允许前端上传“系统 Prompt”或绕过上下文组装器。SSE 事件建议包括 `run_started`、`message_delta`、`citation`、`artifact_ready`、`approval_required`、`action_preview`、`action_confirmed`、`action_completed`、`run_completed`、`run_failed`。

### 6.3 统一计量

所有助手调用必须继续经过现有 AI Gateway，并增加业务点：

```text
assistant_chat
assistant_analysis
assistant_draft
assistant_validation
```

成本明细仍写入 trusted `cost_line_item`，关联 `assistant_run_id`、`target_id`、`run_id`、`case_id`（可空）。助手不得在前端或独立服务重复计算金额。

## 7. 安全、可信和可解释性

### 7.1 权限

权限检查顺序固定为：认证身份 → 当前团队 → 资产级 RBAC → 数据分类 → 技能策略 → 工具风险 → 资源配额。助手不能因为“用户在聊天中说自己是管理员”而提升权限；工具执行时重新读取服务端权限，不能只相信创建会话时的快照。

### 7.2 防提示注入和数据外泄

- 仓库 README、Issue、测试日志、网页内容全部视为不可信数据，不得改变技能规则或工具权限；
- 结构化工具参数用 Schema 校验，模型不得返回可执行 shell、任意 SQL 或任意 URL；
- 模型输入前做密钥、Token、Cookie、个人信息和内部地址脱敏；
- 输出通过 JSON Schema、长度、枚举、引用存在性和租户归属校验；
- 外部模型默认只收到必要证据；私有化模型可由团队策略指定；
- 每个模型、技能、Prompt 模板和验证器都要有版本，结果引用完整版本组合。

### 7.3 可信输出

助手输出至少分为：

```text
fact       已从系统事实读取，可引用
inference  基于事实的推断，必须有置信度和依据
proposal   可供人/策略采纳的建议
command    已通过权限和确认后执行的领域命令
```

门禁结果、测试通过率、版本匹配、证据哈希等系统事实不可由模型覆盖。报告中要区分“测试事实”和“AI 分析”，避免把建议误读为验证结论。

### 7.4 AI 操作审计：记录意图、授权、执行和结果

现有 `aud_event` 是可信域的 append-only 哈希链，必须作为 AI 操作审计的最终锚点；不能只把对话历史当日志。审计的最小因果链为：

```text
用户/事件触发
  -> assistant.run.started
  -> assistant.action.proposed
  -> assistant.action.authorized | denied | expired
  -> assistant.action.dispatched
  -> domain.command.succeeded | failed
  -> assistant.action.completed
```

显式界面引导也必须写 `assistant_action`：`ui.navigate`、`ui.focus`、`ui.prefill`、`ui.open_evidence` 和 `ui.show_command_preview`。后台建议写 `assistant.artifact.created`、`assistant.notification.created` 等语义事件，但没有 `ui.*` 操作。这里记录的是一次**语义操作**及其目标，不记录鼠标移动、逐字符输入或模型 token 流，避免高噪声且不泄露输入正文。

每条操作记录至少含以下不可变信封；敏感正文存对象存储引用或仅存哈希，绝不能进审计 payload：

```text
action_id / sequence / request_id / trace_id / assistant_run_id
actor_kind=assistant / assistant_identity / delegated_by_user_id
team_id / target_scope / command_or_ui_action / risk_class
input_hash / context_snapshot_hash / model_and_skill_version
authorization_mode=assist|delegated|policy_automation
authorization_decision / idempotency_key / outcome / error_code
result_entity_refs / evidence_hashes / started_at / completed_at
```

数据库演进采用只增迁移：`assistant_action` 为 append-only 明细账；其每行按 `action_id + sequence` 去重并计算行哈希。可信域把这些行（或连续小批次的 Merkle root）锚定进 `aud_event.payload`，该 payload 已参与现有哈希链计算。这样“每一个操作均可逐条查询”，同时审计链可证明对应动作批次未被改写。R2/R3 命令的 `proposed/authorized/dispatched` 必须在执行前同步获得可信审计写入确认；审计不可用即拒绝执行。完成事件可经事务 outbox 重试，但命令收据和幂等键必须先持久化，确保最终一定能补齐并可对账。R0/R1 高密度 UI 操作可以毫秒级批量锚定，但先写入本地 append-only 账本，不能因聚合丢失单条操作。

现有 `aud_event.actor` 保留为发起委托的已认证用户；AI 身份、委托和命令收据写入受哈希保护的操作信封。后续迁移可为常用检索字段添加投影列和索引，但不能只存投影列而漏掉哈希信封。审计页需支持按“委托人、AI 身份、会话、技能、命令、风险、结果、资产、时间”追溯，并可从领域变更反查完整助手操作链。

### 7.5 操作安全

- **最小授权与重新鉴权**：每个命令执行前重新检查认证、团队、资产 RBAC、数据分类、委托范围、版本前置条件和配额；不得复用过期会话权限。
- **确认与职责分离**：界面引导中的正式提交由用户完成；例外 R2 委托命令才显示命令预览、影响对象、预计资源/成本和确认人。R3 需要团队策略显式启用、两名不同主体审批和短时二次验证。权限、模型连接、保留策略和审计配置不进入普通自动化白名单。
- **能力令牌而非用户 Cookie**：浏览器不向模型、worker 或 UI 代理暴露 `gp_session`；后端签发一次性、窄作用域、绑定 `action_id` 与 `idempotency_key` 的内部能力令牌。
- **抗提示注入**：仓库、日志、Issue、截图 OCR 和模型输出都不能创建委托、扩大 allow-list 或直接触发命令；只有服务器签发的 Delegation 和确认 API 有效。
- **并发与重放防护**：所有命令要求幂等键；涉及版本对象要求 expected version；Run 仍由既有冲突锁、配额和 Temporal 状态机裁决。相同 `action_id` 的重试只返回同一命令收据。
- **失败安全**：模型、审计、授权、策略、命令 Schema 或工具响应任一不可用时，命令 fail closed；界面只能展示“未执行”，不得把计划说成已完成。

### 7.6 性能与可用性

- **交互路径轻量化**：显式短问答、UI 导航与预填走同步 API/SSE；上下文只传实体 ID，服务端按版本化快照查询。后台分析、批量草稿和例外委托命令转 Temporal，避免占用 HTTP 连接或阻塞 Run/Gate 主链。
- **上下文和证据预算**：按技能定义最大对象数、字节数、Token、时长和成本；证据先取元数据、摘要和哈希，需要正文时按最小片段读取。缓存键必须包含租户、权限、实体版本和脱敏策略，权限变化即失效。
- **工具隔离与限流**：Tool Broker 按团队/资产/命令/模型做并发配额、超时、熔断和退避；AI 推理与执行资源池分开，助手负载不能挤占 CI 和测试运行资源。
- **审计吞吐**：状态变更命令采用同步意图锚定；高频辅助 UI 操作进入 append-only action ledger 后做短批 Merkle 锚定。审计写入队列有长度、延迟和漏锚指标，超过阈值降级为只读/仅辅助模式。
- **可恢复性**：`assistant_action`、领域命令收据、Temporal workflow ID 与审计锚点可交叉对账；worker 崩溃后根据命令收据判断“未发起/已成功/未知”，绝不盲目重放有副作用操作。

## 8. 可观测性和验收指标

每次助手运行统一贯穿：`request_id → session_id → turn_id → assistant_run_id → model_request_id → cost_line_item → audit_event`。

首批指标：

- 首 token 延迟、完整响应延迟、超时率、取消率；
- 工具调用成功率、审批通过率、幂等重试率；
- 引用覆盖率、事实校验失败率、无依据结论率；
- 每技能 Token/金额、预算超限率、模型 fallback 率；
- 隐式建议采纳率、误报率、对核心工作流的额外延迟；
- 跨租户访问拒绝、提示注入拦截、敏感信息脱敏命中。

验收必须包含：同一 `ContextSnapshot` 可重放、相同幂等键不重复执行、撤销/拒绝不会写入领域事实、RLS 隔离测试、工具越权测试、模型不可用时降级为确定性结果或明确失败。

## 9. 分阶段落地

### A0：运行时骨架和显式只读助手

- 新增 `internal/assistant` 端口、会话/Turn/Artifact 数据模型；
- 前端全局入口 + 上下文面板；
- 只开放 R0 Read 和 R1 Draft；
- 接入现有 AI Gateway、成本、审计和 SSE；
- 先实现 `gate.failure.explain`、`run.failure.triage`、`target.profile.explain`。

同时落实 `UIActionManifest`、`GuidanceDescriptor`、`assistant_action` append-only 账本和 R0/R1 审计，但不授予任何写入委托。隐式能力只产生后台 Artifact 和待处理事项。

### A1：隐式分析旁路

- 引入领域事件出站和 Assistant Temporal Workflow；
- 将现有服务画像分析收敛为技能实现；
- 接入生成质量、失败聚类、契约影响、报告摘要；
- 所有结果落 Artifact，不自动修改正式用例和门禁。

### A2：受控动作和审批

- 建立 R2 Tool Broker 和统一审批卡；
- 支持“生成用例草稿 → 人工审核 → CaseService 提交”；
- 支持“失败分析 → 创建复测 Run”，但仍由 RunService 做版本、配额和冲突检查；
- 增加预算、技能开关、团队级模型和保留策略。

同阶段才评估少数短时 Delegation、命令收据、R2 前置审计锚定和按“人 + AI”维度的操作审计视图；R3 仍保持关闭。默认产品体验仍是显式界面引导 + 用户提交。

### A3：质量闭环和策略化自动化

- 反馈、采纳、误报数据进入技能评估；
- 仅对明确低风险动作开放自动提交；
- 把助手建议纳入质量报告和审计视图；
- 在有稳定召回指标后，再评估 pgvector 或外部知识库。

## 10. 需要锁定的架构决策

本设计建议评审时明确以下不可模糊项：

1. 助手是否允许创建“待审核用例草稿”（建议允许），但不允许直接提交正式版本；
2. 隐式分析是否默认旁路（建议是），核心 Run 和 Gate 不因助手不可用而阻塞；
3. 哪些 R2 动作允许自动执行（建议第一阶段为空）；
4. 是否允许 R3 敏感命令；建议仅对私有化团队以“双人审批 + 短时二次验证 + 逐命令开关”启用；
5. 团队是否可以配置外部模型发送的数据范围（建议必须可以）；
6. AI 产物与操作审计的默认保留期、导出权限和跨团队共享规则；
7. 是否将助手成本单列在审计页面，还是先沿用 `analysis` 类别并通过 `assistant_run_id` 下钻（建议先扩展业务点，保持总览口径统一）。

最终落地标准不是“有一个能聊天的页面”，而是：用户能在任何业务上下文中得到有证据的解释；流程能自动产生可审阅的建议；所有建议都能回到输入、模型、工具、规则、权限、成本和最终领域结果。

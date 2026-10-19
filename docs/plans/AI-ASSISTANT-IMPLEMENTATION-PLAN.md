# GreenPass AI 协作实施方案

> 状态：评审用实施稿 v0.1 · 2026-10-15
>
> 依据：[AI-ASSISTANT-DESIGN.md](../architecture/AI-ASSISTANT-DESIGN.md)、[TECH-DESIGN.md](../architecture/TECH-DESIGN.md)、[ENGINEERING-SPEC.md](../engineering/ENGINEERING-SPEC.md)。
>
> 本方案将“隐式后台建议 + 显式对话与界面引导 + 全量可验证审计”拆为可交付开发包。默认产品体验是 **用户先在全局或页面上下文对话框中表达意图、追问和确认处理方式；需要操作时，再由 AI 在当前界面带着操作，用户自己提交正式命令**；AI 代执行不属于首期范围。

## 1. 产品与交互边界

### 1.1 三种模式

| 模式 | 触发 | AI 做什么 | AI 不做什么 | 用户的动作 |
|---|---|---|---|---|
| 隐式后台建议 | Run、门禁、生成、契约等领域事件 | 分析事实、创建 Artifact、通知待处理事项 | 无浏览器会话、不导航、不预填、不提交 | 在建议队列查看或忽略 |
| 显式对话与界面引导 | 用户点“AI 对话 / 带我操作”或主动提问 | 对话澄清、解释、定位、突出当前控件、预填未提交表单、观察结果 | 不点击正式提交、不代表用户做决定 | 在对话中追问，选择操作，再自行提交或中断 |
| 委托执行（后续） | 用户在命令预览明确选择 | 调用一条已批准领域命令 | 不批量自主执行、不提升权限 | 逐次确认，接收结果 |

### 1.2 首期应呈现的四条用户路径

1. **门禁阻断**：后台产生“契约变更 / 弱断言”建议 → 用户在门禁页点“AI 带我处理” → 查看证据 → 预填复测范围 → 用户在执行页创建 Run → AI 回显权威结果。
2. **生成质量异常**：后台发现种子重复或覆盖缺口 → 用户在生成页开启引导 → 跳到对应批次和缺口 → 预填审核意见或生成范围 → 用户审批/提交。
3. **执行失败**：后台归类 flaky、环境或产品失败 → 用户在历史页开启引导 → 查看证据 → 预填复测计划 → 用户决定是否启动。
4. **成本异常**：后台发现成本阈值超限 → 用户从通知中心进入成本审计 → AI 解释归因与可选优化路径；不自动改变模型或预算。

## 2. 原型范围与验收口径

已实现的前端原型入口：

- 导航栏新增“AI 协作”；
- 顶部全局按钮“AI 对话”是开关：打开后显示选中态和“已开启”，再次点击或在面板关闭即可收起。右侧完整 Dock 严格绑定浏览器可视内容区高度；面板不加遮罩、不锁定主页面，宽度随视口自适应，主内容区随之收窄，用户可一边对话一边继续操作；
- 质量门禁页新增“AI 带我处理”；
- 对话框支持提问、追问、建议问题和“带我操作”；`#/assistant` 展示后台建议、由对话发起的显式步骤引导、预填草稿和审计链。

原型验证目标不是模型回答质量，而是确认心智模型：

- 用户能一眼区分“后台告诉我”与“正在带我操作”；
- 引导步骤明确显示当前动作、下一个页面和用户提交点；
- “预填”明确标记为未提交；
- 审计中能区分后台 Artifact、`ui.*` 引导和用户领域命令；
- 不出现“AI 已替我提交”或隐式助手操控界面的暗示。

## 3. A0：数据与领域骨架

### A0-01：新增 assistant 域与迁移

**目标**：建立可审计的助手生命周期，不让聊天消息成为业务事实。

**新增包**：

```text
internal/assistant/
  api/ application/ domain/ infra/ workflow/
```

**新增表**（表前缀建议 `ast_`，在工程表前缀登记后实施）：

| 表 | 关键键 | 写入方式 |
|---|---|---|
| `ast_session` | `team_id, actor_id, mode, scope_json, status` | 会话创建/关闭 |
| `ast_turn` | `session_id, request_id, trigger_kind, context_ref, status` | 一次用户问题或事件 |
| `ast_run` | `turn_id, skill_id, skill_version, model_ref, workflow_id, status` | 技能一次运行 |
| `ast_artifact` | `run_id, type, target_ref, payload_ref, confidence, status` | 后台建议、草稿、分析 |
| `ast_action` | `action_id, sequence, run_id, kind, input_hash, outcome` | 所有语义化引导/建议动作，append-only |
| `ast_approval` | `artifact_id, actor_id, decision, reason` | 人工采用/拒绝 |

所有表带 `team_id`、RLS、审计字段和必要索引。`ast_action` 禁止 UPDATE/DELETE；`action_id + sequence` 唯一。助手大正文、证据快照和 Prompt/输出存对象存储受控引用，表内只存 hash、摘要与引用。

**领域对象与端口**：

```go
type GuidanceAction struct { ID, Route, ActionID string; EntityRefs []EntityRef; DraftRef string }
type AssistantArtifact struct { ID, RunID int64; Type, Status string; EvidenceRefs []string }
type ActionLedgerPort interface { Append(ctx context.Context, action Action) error }
type ContextSnapshotPort interface { Build(ctx context.Context, input ContextInput) (Snapshot, error) }
type SkillPort interface { Run(ctx context.Context, input SkillInput) (Artifact, error) }
```

**验证**：迁移只增校验、RLS 跨团队拒绝、`ast_action` UPDATE/DELETE 失败、相同 `request_id` 重放不创建第二个 Turn。

### A0-02：统一审计桥接

**目标**：assistant 明细账与现有 `aud_event` 哈希链互相可追溯。

- `ast_action` 写入后，以 `assistant.action.*` 操作和 `action_id / sequence / action_hash` 进入 trusted `AuditService`；
- 后台建议至少记录 `assistant.artifact.created`；
- 显式引导记录 `ui.navigate`、`ui.focus`、`ui.prefill`、`ui.open_evidence`、`ui.guide.completed|cancelled`；
- 正式提交继续由原业务服务写 `case.*`、`run.*`、`gate.*` 审计；其 payload 关联 `assistant_run_id` 和 `guidance_action_id`；
- 审计写失败时，R0/R1 只显示“未记录，已停止引导”；任何未来 R2 命令 fail closed。

**验证**：从 `run.create` 反查其引导 action、Artifact、上下文 hash 与用户；从某次 `ui.prefill` 反查不包含表单敏感正文。

## 4. A1：显式界面引导

### A1-01：前端 Guidance Manifest

每个可引导的页面能力显式注册，不准让模型扫描 DOM 或构造 CSS selector：

```ts
type GuidanceDescriptor = {
  id: string;
  route: string;
  target: 'gate-rule' | 'case-selector' | 'run-confirmation';
  preconditions: string[];
  explanation: string;
  draftSchema?: z.ZodType;
  successSignal: 'sse.run-created' | 'query.gate-loaded';
};
```

首批 Manifest：

| 页面 | action_id | 引导能力 |
|---|---|---|
| 门禁 | `gate.open-rule` | 定位阻断规则、展示策略与证据 |
| 用例 | `cases.prefill-retest-scope` | 预填受影响用例筛选 |
| 执行 | `run.confirm-retest` | 展示环境/场景/用例范围，等待用户提交 |
| 历史 | `history.open-evidence` | 打开并解释指定证据 |
| 生成 | `generation.open-quality-gap` | 定位质量检查缺口与候选批次 |

### A1-02：引导会话状态机

```text
idle -> requested -> context_ready -> guiding
guiding -> draft_prefilled -> awaiting_user_command
awaiting_user_command -> observed_success | observed_failure | cancelled | expired
```

- 仅用户点击入口后创建 `ast_session(mode=explicit_guidance)`；
- 每一步 SSE 推送 `guide_step_changed`，页面保留可见“退出引导”；
- 预填只在浏览器本地表单状态发生，后端只审计 Schema 校验后的字段 hash；
- 用户点击原页面按钮后，后端响应或 SSE 含关联 ID，转入 `observed_*`；
- 用户离开路由、取消或超时，记录 `ui.guide.cancelled`，不补发任何命令。

### A1-03：当前原型扩展为真实组件

原型页面中的硬编码步骤在实现时抽成以下组件：

```text
src/assistant/
  GuidanceLauncher.tsx
  GuidanceRail.tsx
  GuidedStepCard.tsx
  AssistantPanel.tsx
  AssistantAuditTimeline.tsx
  manifests/*.ts
```

`Layout` 只挂载 Launcher 与全局会话容器；业务页面通过 descriptor 注册目标并接收 `focus/prefill` 状态，不把 Prompt 或模型调用写入页面组件。

**验收**：门禁阻断引导能跨 `/gate → /cases → /exec → /history` 保留步骤；用户手工提交成功后，助手只读回执并进入下一步；用户取消后没有 Run、Case 或 Gate 写入。

## 5. A2：隐式后台建议

### A2-01：领域事件与技能

首先采用 existing Temporal/Redis 基础设施和 outbox；事件只含实体 ID、版本与团队，不含 Prompt 正文：

```text
gate.evaluated            -> gate.failure.explain
run.case_failed           -> run.failure.triage
generation.quality_checked -> generation.seed.review
contract.diff.detected    -> contract.impact.explain
cost.threshold_exceeded   -> audit.anomaly.detect
```

每个技能配置：输入 Schema、最大上下文字节/token、模型策略、预计预算、超时、验证器、Artifact Schema 和通知策略。模型输出先过 JSON Schema 和引用权限校验；不能验证则保存失败摘要，不创建误导性的建议。

### A2-02：待处理事项与通知

- 后台 Assistant Workflow 只写 Artifact / 待处理事项；
- 通知中心显示建议标题、优先级、事实来源、建议动作和“带我处理”；
- 点击“带我处理”才创建显式 `GuidanceSession`；
- 同一实体/规则/版本的相同建议用 idempotency key 合并，避免告警风暴；
- 模型不可用时保留确定性告警和“AI 分析暂不可用”。

**验收**：事件工作流没有任何 `ui.*` 调用；后台建议在无在线用户时仍可产生；通知数量受去重、限流和租户配额控制。

## 6. A3：安全、性能和可运维性

### 6.1 权限与数据安全

1. 上下文读取与页面提交均用当前已认证 Principal + team RLS + 资产 RBAC；
2. 每一步引导重新判断目标对象是否仍可见；权限变化立即停止会话；
3. 仓库、日志、截图 OCR、模型输出均是不可信数据，不能改变 Manifest、步骤、权限或导航 allow-list；
4. Prompt 仅由服务端模板与 ContextSnapshot 构造；密钥、Cookie、Token、手机号、内部地址先脱敏；
5. 浏览器接收的是受签名且短时有效的引导事件，不接收能执行 JS、调用任意 URL 或自动提交表单的指令。

### 6.2 性能策略

| 路径 | SLA/限制 | 处理方式 |
|---|---|---|
| UI 引导事件 | 交互首帧 < 500ms（不含模型回答） | 使用预定义 descriptor，本地高亮，SSE 回执 |
| 短问答 | 受模型预算约束 | 结构化上下文 + 流式输出 + 可取消 |
| 后台分析 | 不阻塞 Run/Gate | Temporal 异步、团队/技能并发上限、熔断 |
| 证据读取 | 限定文件、页数和字节 | 先元数据/摘要，按需取分段，S3 短时签名 |
| 审计 | 关键状态同步、引导动作批量锚定 | `ast_action` 先落 append-only，再短批锚定 `aud_event` |

推理资源池与测试执行资源池分离；当模型预算、队列深度或审计锚定延迟越阈值时，系统降级为“查看已有建议 + 手动操作”，不影响 CI/门禁主链。

### 6.3 可观测性和测试

统一传播：`request_id → session_id → turn_id → assistant_run_id → action_id → model_request_id → audit_event`。

测试清单：

- domain：状态机、风险等级、幂等键、Action hash；
- application：权限降级、取消、过期、重放、审计失败；
- API：SSE 顺序、跨租户不可见、输入 Schema 校验；
- UI：Manifest target 不存在时可恢复、跨页会话、预填不提交、取消恢复；
- integration：从 Gate 阻断到用户创建复测 Run 的整条审计链；
- chaos：Temporal 重试、模型超时、S3 不可用、Audit Store 不可用时 fail-safe/fail-closed 行为。

## 7. 后续才评估的委托执行

只有 A0–A3 的引导、审计、权限和用户采纳率验证稳定后，才单独评审 R2 委托命令。前置条件：

- 命令有独立 Schema、幂等键、版本前置条件和可读回执；
- 用户在命令预览中逐次确认；
- 先同步写 `assistant.action.proposed/authorized/dispatched` 审计锚点；
- AI 身份没有用户 Cookie 或管理员权限；
- 首批仅允许可回退或低影响命令，R3 敏感命令继续关闭。

这不是“让 AI 会点按钮”的阶段，而是对少数领域命令增加受控调用入口；没有上述条件就不开发。

## 8. 评审决策点

1. 首期是否以“门禁阻断带我处理”作为唯一真实闭环；建议是。
2. 后台建议默认保留多久、谁可见、是否可导出。
3. 是否允许预填含敏感数据字段；建议首期只预填实体 ID、环境、场景和用例范围。
4. 后台建议通知采用站内待处理事项还是额外消息渠道；建议先站内。
5. R2 委托执行是否进入路线图；建议只保留设计，不排入首期开发。

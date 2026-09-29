# GreenPass · 工程规范（Go）

> 状态：生效（v1.0）
> 适用范围：`open-green-pass` 后端工作区（GreenPass 服务仓库）、构建脚本、CI 与本地开发环境
> 定位：GreenPass 后端（Go + Temporal）的全量工程规范。参考 `open-im-server` 的 Java 工程治理骨架（模块边界 / 依赖方向 / 分层职责 / 日志埋点 / 构建门禁 / DB 表前缀 / 迁移只增不改 / 可观测 / 分层测试 / 发布），并按 Go 技术栈 + 业界工程实践（Google Go Style、Uber Go Style、字节 CloudWeGo 工程规范、golangci-lint 生态）**改写为 Go 原生表达**，非机械照抄。
> 前端 UI/交互规范见 [DESIGN-SPEC.md](./DESIGN-SPEC.md)，本规范只约束后端工程与前后端接口契约。

---

## 1. 目标与适用范围

1. 构建必须可复现：固定 Go toolchain、锁定 `go.mod` / `go.sum`，不依赖开发者本机隐式环境。
2. 依赖方向必须表达部署与领域边界：领域内核零第三方依赖、跨域只经稳定契约、可运行入口集中在 `cmd/`。
3. 质量门禁必须集中治理并强制失败，任何例外须有架构决策记录、负责人、到期时间、退出条件。
4. 单测/构建通过不等于功能正确；不得以跳过关键校验、排除源码制造"假绿"。
5. 与产品侧 PRD（需求语义）、DESIGN-SPEC（前端 UI/交互）并行演进；任何原型/PRD 变更须同步影响本规范相关章节。

---

## 2. 工程拓扑与模块职责

### 2.1 拓扑（单体优先 + 可拆部署单元）

```text
green-pass/
├── cmd/                     # 可运行入口 = 部署单元；单体阶段仅 server，按域可拆出独立 cmd
│   ├── server/              # 聚合服务入口：装配全部域 handler 与中间件
│   └── worker/              # 异步 worker 入口：AI 生成管道 / 执行调度 / 事件消费
├── internal/                # 私有实现，禁止外部导入（Go internal 编译期边界）
│   ├── gateway/             # 接入域：路由 / 中间件 / 鉴权 / 租户上下文 / 限流
│   ├── governance/          # 治理域：被测对象 / 用例 / 契约 / 报告 / 团队权限 / 模型配置
│   ├── execution/           # 执行域：调度 / 沙箱 / 真机 / 浏览器 / 版本校验 / 资源配额
│   ├── ai/                  # AI 编排域：模型网关 / Token 计量 / 生成管道 worker
│   ├── trusted/             # 可信域：审计哈希链 / 质量门禁 / 成本归因
│   └── platform/            # 平台支撑：多租户 / 可观测 / 配置 / 公共错误 / 分页
├── pkg/                     # 可复用纯技术包（无业务语义）：协议常量 / 时钟 / 哈希链工具
├── api/                     # 对外契约：OpenAPI(openapi.yaml) + 生成类型（前端/BFF 消费）
├── migrations/              # 数据库迁移（golang-migrate：000001_xxx.up/down.sql）
├── deploy/                  # docker / helm / k8s / 私有化单二进制打包
├── scripts/                 # validate 门禁脚本（单文件入口）
├── test/                    # 跨域集成 / e2e（testcontainers 拉起依赖）
├── .github/workflows/       # CI
├── .env / .env.example      # 共享本地开发基线（.env 入版本控制，见 §11）
├── Makefile                 # 工程门禁唯一入口
├── go.mod / go.sum          # 依赖锁定
└── go.work（可选，多仓库联调时启用）
```

### 2.2 模块职责边界

| 模块 | 职责 | 允许内容 | 禁止内容 |
|---|---|---|---|
| `cmd/*` | 仅装配与启动 | main、装配、读取配置、注册 handler/worker | 业务规则、领域逻辑、直接持久化 |
| `internal/gateway` | 接入与横切 | 路由、鉴权/租户上下文、限流、DTO 转换、调用 application | 领域实现、Repository、事务 |
| `internal/<domain>/api` | 入站适配（HTTP handler） | 请求/响应 DTO、参数校验、鉴权上下文提取、DTO↔Command/Result | 直接调 Repository、PO、领域对象序列化 |
| `internal/<domain>/application` | 用例编排 | Command/Query/Result、事务边界、幂等、权限协调、调领域 + 端口 | 框架/中间件依赖、PO、ORM 查询 |
| `internal/<domain>/domain` | 领域内核 | 聚合/实体/值对象/领域事件/仓储契约/领域策略 | **任何第三方依赖**（GORM/Redis/Temporal/HTTP/JSON 序列化实现） |
| `internal/<domain>/infra` | 技术适配 | 仓储实现、PO 映射、缓存、事件投递、配置 | API DTO、业务规则 |
| `pkg/` | 跨域纯技术 | 无业务语义的工具/协议/基础设施抽象 | 领域对象、业务 Repository |
| `api/` | 对外契约 | OpenAPI/生成 DTO | 内部实现、PO、Mapper |
| `migrations/` | 数据迁移 | 只增不改的版本脚本 | 运行时 ddl、业务逻辑 |

### 2.3 服务域（限界上下文）划分

GreenPass 后端按限界上下文组织，单体阶段各自清晰、需要时独立部署：

| 限界上下文 | 域 | 核心聚合 | 权威数据 |
|---|---|---|---|
| 治理 Governance | `internal/governance` | 被测对象(Target)、用例(Case/版本化)、契约(Contract)、报告(Report)、团队(Tenant/Team)、成员权限(AssetRBAC)、模型配置(Model) | `gp_` 库 RLS |
| 执行 Execution | `internal/execution` | 测试运行(Run)、资源配额(Quota)、调度任务(Job/Temporal)、环境版本(EnvVersion) | `gp_` + Temporal + Redis |
| AI 编排 AI | `internal/ai` | 模型网关、Token 计量(CostLineItem 写入 trusted)、生成批次(GenBatch)、决策痕迹 | `gp_` + 模型网关 |
| 可信 Trusted | `internal/trusted` | 门禁规则(GateRule/OPA)、门禁结果、审计链(AuditEvent)、成本归因 | `gp_`（append-only） |
| 接入 Gateway | `internal/gateway` | —（无领域，纯接入） | 无权威数据 |

- **跨域调用**：同步只经 `api/` OpenAPI 契约或内部显式端口；异步只经版本化事件。**禁止跨域直连对方数据库/PO**。
- **数据所有权**：每张权威表有且仅有一个所属限界上下文，其他域只读只经契约/查询端口；审计表 `aud_*` 归 Trusted 域唯一写。

---

## 3. 依赖规则与分层边界

### 3.1 领域内核零第三方依赖（最硬约束）

`internal/<domain>/domain` 只允许依赖：

- 标准库（含 `time`、`errors`、`slices`、`maps` 等）
- 同 domain 内其它 domain 类型
- `pkg/` 中无业务语义的纯工具

**禁止 import**：`gorm.io/gorm`、`gin-gonic/gin`、`go.temporal.io/sdk`、`redis`、任何 HTTP client、`encoding/json` 之外的具体序列化实现、任何数据库驱动、`context` 之外的框架上下文。

> 理由（对齐 im 的 DDD + Google Go 分层）：领域模型承载核心业务规则（聚合不变量/状态迁移/门禁计算/成本归因），必须与框架、中间件解耦，才能被纯单元测试覆盖、不随技术栈漂移。持久化映射由 `infra.persistence.po` 承担。

### 3.2 依赖方向

```text
其他域/网关 ────> 本域稳定契约(OpenAPI / 端口接口)
                  ^
                  │
本域 application ─> domain <──── infra 实现 domain 端口
```

- `api(handler) → application → domain ← infra`。
- 同包导入方向用 `golangci-lint` 的 `depguard` 在 **编译前门禁** 强制（见 §7），不靠 Code Review 自觉。

### 3.3 对象隔离（Go 版）

| 对象 | 归属层 | 用途 | 禁止 |
|---|---|---|---|
| DTO（req/resp） | `api` | 外部协议输入输出 | 进 domain/infra |
| Command/Query/Result | `application` | 用例输入/查询/输出 | 当 handler 入参、当 DB 映射 |
| Aggregate/Entity/ValueObject | `domain` | 业务状态与不变量 | 直接序列化、直接映射 DB、直接返回接口 |
| PO / 映射结构 | `infra` | 表映射（GORM model / sqlc row） | 泄露至 application/domain/api |
| Event / RPC 消息 | `infra` 或 `pkg` | 跨域/跨服务协议 | 替代领域事件/领域对象 |

- 转换器：`api` 负责 DTO↔Command/Result；`infra` 负责 PO↔领域对象。**PO 不得穿透 Repository 实现边界**。
- 跨域稳定契约放 `api/`（OpenAPI 生成类型），服务私有模型不跨域。

---

## 4. 代码风格规范（融合 Google Go Style / Uber Go Style / 字节实践）

### 4.1 命名

- 包名：全小写、单数、无分隔（`governance`、`execution`、`domainmodel`）；避免 `common`/`utils` 万能包。
- 导出标识：`PascalCase`；未导出：`camelCase`。
- 常用缩写：`ID`、`URL`、`HTTP`、`API` 全大写（`caseID`、`getURL`）；不逐字母转驼峰（`UserID` 而非 `UserId`）。
- 常量：导出常量全大写缩写如 `MaxRetry`；**错误变量/类型**：`ErrNotFound`（值）、`NotFoundError`（类型）。
- 接收者：短名（`c *Case`），统一值/指针，禁混用。
- 通用命名约定：`NewXxx()` 构造；`GetXxx` 仅在获取非平凡值时用；布尔谓词 `IsXxx/HasXxx/CanXxx`。

### 4.2 错误处理（Google/Uber 核心）

- 一律返回显式 error，**禁止**吞错；`errors.Is`/`errors.As` 判断；不裸 `fmt.Errorf`，用 `fmt.Errorf("...: %w", err)` 包裹并保留 `%w`。
- 错误信息首字母小写、不带句号（`"target not found"`）。
- 区分：`domain` 定义业务错误（`ErrGateBlocked`、`ErrCaseVersionConflict`）；`infra` 定义技术错误映射；跨层用 `errors.As` 识别领域错误，不泄露 DB/ORM 错误。
- 对可恢复场景明确降级语义：`log.Warn + 返回降级结果`，禁止静默 continue。

### 4.3 并发（Go 原生纪律）

- goroutine 必须**可解释生命周期**：谁创建、何时退出、异常如何回收；用 `errgroup.Group`（golang.org/x/sync）管理扇出。
- `context.Context` 作为函数/方法第一个参数，贯穿 API→application→domain 端口→infra；遵守超时/取消。
- 避免数据竞争：所有共享状态必须通过 `-race` 验证；优先 channel/immutable，慎用裸 `sync.Mutex` 保护大块。
- 关闭用 `sync.Once` 或 `context.WithCancel`，不裸 `defer cancel` 于错误路径之外。

### 4.4 接口与抽象

- **小接口、由消费者定义**：`domain` 定义仓储端口（`FindByID(ctx, id)`），`infra` 实现；不为"以后可能用"预建接口。
- 避免过度抽象：单实现端口仍走接口（因分层边界 + 可测试），但禁止接口套接口。
- 导出类型必须有 godoc 注释（见 §5）；`exported` 但未注释会被 `golangci-lint`（revive/doc）门禁。

### 4.5 包组织与副作用

- 禁 `init()` 副作用（除注册 driver 等必要且显式标注场景）；配置/连接在 `main` 或显式组装期注入。
- 类型零值尽量可用；禁止隐式全局可变状态（包级 `var` 尽量只放不可变常量）。
- 避免包级可变单例（DB/Redis/Temporal client 必须依赖注入，不包级 `var db *gorm.DB`）。

---

## 5. 注释规范

1. 每个导出类型/函数必须有 godoc 注释，以被注释标识符开头（`// CaseRepo ...`）；说明业务意图、边界、幂等与失败语义，不只写"做了什么"。
2. 影响权限、一致性、顺序性、幂等、补偿、重试、降级、状态流转的代码必须解释"为什么这样设计"。
3. 中文注释为主，禁止乱码、占位符残留、机翻不通顺、与实现不一致的注释；迁移代码时同步更新注释归属。
4. 每个 `cmd/*` 服务与每个限界上下文在 `docs/services/README.md` 维护职责、入口、依赖与运行方式。

---

## 6. 日志规范（slog 结构化 + OTel 关联）

采用标准库 `log/slog`（Go 1.21+）结构化日志，Handler 接 OTel/收集器，禁止 `fmt.Println` 排查。

### 6.1 必须埋点位置

- 外部输入入口：HTTP handler、Temporal workflow/activity、定时任务、事件 consumer 入口
- 关键输出边界：DB 权威写入、Redis 广播、事件发布、模型网关调用、WebSocket/SSE 下行
- 异常：重试、补偿、降级、吞异常、启动/关闭失败
- 关键状态切换：任务入队/调度/暂停恢复、用例入库/回退、门禁判定、报告生成、审计锚定

### 6.2 关联字段（每条关键日志必须携带）

`trace_id` / `request_id` / `run_id` / `case_id` / `tenant_id` / `target_id` / `gen_batch_id`——按链路选择最小集合，写为 slog 属性（`slog.String("run_id", r.ID)`）。跨进程经 OTel 传播 span 上下文。

### 6.3 级别

| 级别 | 用途 |
|---|---|
| `Info` | 关键业务状态变化、关键边界输入输出（含调用模型/Tokens） |
| `Warn` | 可恢复异常、降级、非法输入、权限拒绝、重试挂起、资源排队 |
| `Error` | 真正失败、链路中断、数据不一致风险、启动失败、关键资源不可用 |
| `Debug` | 高频明细，仅本地排障，主链路理解不得依赖 Debug |

### 6.4 敏感与审计

- 禁止打印：密钥、Token、完整消息/提示词正文、身份证/手机号、模型 API Key；正文只记长度/摘要/业务标识。
- **审计日志与运行日志分离**：审计走 `aud_*`（Trusted 域，append-only + 哈希链，独立保留策略与权限），不得与调试日志混存、不可随意删除。

---

## 7. 构建与校验门禁（统一入口）

### 7.1 工具链与可复现

- Go toolchain 固定：`go 1.24` + `go.mod` 声明 `toolchain`；`go.sum` 全量锁定，禁 `go get latest` / floating 版本。
- 本地与 CI 同一 `Makefile` 入口；所有门禁串行执行（`make check` 单进程），避免并发 `clean` 竞争。

### 7.2 门禁层级（映射 im 的 invoke-engineering-validation 统一入口）

```makefile
validate : gofmt/goimports 一致性 + go vet + staticcheck          # 静态
lint     : golangci-lint run  # 含 depguard(依赖边界)/revive(godoc)/gocyclo/errcheck/rowserrcheck
test     : go test -race ./... --cover   # 分层单元 + 集成(本地标记)
build    : go build ./cmd/... && go vet ./...
vuln     : govulncheck ./...
migcheck : 迁移脚本只增不改校验 + RLS/表前缀扫描
check    : validate + lint + test + build + vuln + migcheck       # 全量，CI 用
```

- **依赖边界（depguard）**：`domain` 只许 stdlib+pkg+同域；`application` 禁 GORM/HTTP client/Temporal/Redis；`cmd` 之外的包禁自写 `main`。规则进 `golangci.yml` 并纳入 `make check`，不靠自觉。
- **例外**：任何门禁例外必须有架构决策记录、负责人、到期时间、退出条件；禁止长期全局 `nolint` 掩盖。
- `make check` 输出统一落 `.outputs/logs/<date>/`，临时调试日志不得写仓库根。

### 7.3 变更最小验证矩阵

| 变更范围 | 最小验证 | 必须追加 |
|---|---|---|
| 单域 domain/application/infra | `make test` 对应包 | 领域/应用测试、迁移专项 |
| 跨域契约 / api OpenAPI | 契约校验 + 前端类型生成 | 消费方契约测试、`make check` |
| 迁移 / RLS / 表结构 | `migcheck` | 数据迁移验证 + testcontainers |
| Makefile / go.mod 依赖 | `make validate` | 根 `make check`、依赖树审查 |
| Temporal workflow/activity | 对应测试 | 工作流重试/暂停/幂等验证 |

---

## 8. 数据库规范（PostgreSQL + 多租户 RLS）

### 8.1 库与多租户

- 单一权威库 `gp_`（PostgreSQL 15+），TimescaleDB 承担时序（成本趋势/执行历史）为独立时序库或同库 hypertable。
- **多租户**：默认共享库 + `Row-Level Security` 按 `team_id` 强制行级隔离 + 应用层资产级 RBAC；大客户私有化可整库隔离。RLS 为强制层，非可选。**可信域权威表（`aud_*`/`cost_*`/`gate_result`）除 RLS 外须用独立 PG 角色 `gp_trusted_writer` 独占写权限，`gp_server`/`gp_worker` 仅只读**——应用层端口约定只是第二道防线，DB 层须可独立强制（防同 team 内跨域直写审计/成本）。
- 所有业务表必须带 `team_id`（RLS 依据），审计/时序表按 RLS 或租户列隔离。

### 8.2 领域表前缀登记（GreenPass）

> 语义同 im 的"已登记领域前缀 + 单数业务名词"，缩写全局唯一、语义稳定，禁止临时自造。新增前缀必须先架构确认并登记。

| 限界上下文 | 领域 | 前缀 | 示例 |
|---|---|---|---|
| Governance | 被测对象树 | `tgt_` | `tgt_target`、`tgt_level` |
| Governance | 用例（版本化） | `cas_` | `cas_case`、`cas_version`、`cas_exec` |
| Governance | 契约 | `ctr_` | `ctr_contract` |
| Governance | 报告 | `rpt_` | `rpt_report`、`rpt_block` |
| Governance | 团队/租户 | `tnt_` | `tnt_team` |
| Governance | 成员/资产权限 | `iam_` | `iam_member`、`iam_asset_perm` |
| Governance | 模型配置 | `mdl_` | `mdl_model`、`mdl_binding` |
| Governance | 被测仓库 | `repo_` | `repo_repo`、`repo_branch` |
| Execution | 测试运行 | `run_` | `run_run`、`run_result`、`run_case_result` |
| Execution | 资源配额 | `res_` | `res_quota`、`res_occupation` |
| Execution | 环境版本 | `env_` | `env_runtime`、`env_check` |
| AI | 生成批次 | `gen_` | `gen_batch`、`gen_seed` |
| Trusted | 门禁 | `gate_` | `gate_rule`、`gate_result` |
| Trusted | 成本归因 | `cost_` | `cost_line_item`、`cost_price` |
| Trusted | 审计哈希链 | `aud_` | `aud_event`（append-only） |
| Trusted | 测试证据 | `evd_` | `evd_evidence`（元数据，文件在 S3） |
| — | 时序（趋势） | `ts_` | `ts_cost_trend`（hypertable） |

### 8.3 表/索引/审计字段

- 表名、列名 `snake_case`；表名单数名词，关联表业务主体在前（`tgt_service`、`cas_case_version`）。
- 索引命名：唯一 `uk_<table>_<semantic>`、普通 `idx_<table>_<semantic>`。
- 主键：`id bigint`（Snowflake 或 PG 序列）+ 业务唯一键；关键表含 `idempotency_key` 幂等键（事件/生成/成本写入）。
- 审计字段：`created_by`/`created_at`/`updated_by`/`updated_at`（`0`=系统任务）+ 可选 `deleted_at`（软删）；应用写入必须填操作者。
- 禁止 JSON 列作为核心查询字段（JSONB 存扩展属性可以，不参与核心筛选）；核心查询路径必须有二级索引。
- 所有表显式字符集/时区一致（UTF-8、UTC 存时间）。

### 8.4 迁移（golang-migrate，只增不改）

- 迁移唯一权威：`migrations/000NNN_<desc>.up/down.sql`；**历史脚本只增不改**，修正结构新增后续版本。
- 禁止运行时 `AutoMigrate` / ddl 自动改表；禁止在 main 链路执行建表。
- 拥有权威数据的域是迁移唯一所有者；跨域不得修改他人迁移。
- 生产/开发重建走统一脚本，输出落 `.outputs/`。

---

## 9. 可观测性（OTel + Prometheus + Grafana）

1. **三信号齐备**：OTel logs/metrics/traces，经 `otel-collector` → Prometheus/Loki/Tempo → Grafana；执行/生成链路必须有 span。
2. **关联贯穿**：HTTP/worker/Temporal/事件统一传播 `trace_id`；每条关键日志带业务关联 id（§6.2）。
3. **指标有限标签**：禁用户/run/case/target 等高基数字段作指标标签；标签集必须有限且登记。
4. **业务指标**：执行吞吐/时延/成功率、队列深度、模型调用量/Token/成本、门禁阻断率、队列积压、资源占用率；**成本指标由 AI 网关单点计量，禁散落**。
5. **告警**：每个告警必须有明确影响、阈值、责任方、处置手册；仅有面板不算可运维。
6. **审计独立**：`aud_*` 保留策略与访问权限独立于运行日志，不可变（哈希链锚定，见 TECH-DESIGN §可信域）。

---

## 10. 测试规范（分层 + 面向行为）

| 层 | 测试 | 工具 | 约束 |
|---|---|---|---|
| `domain` | 聚合不变量/状态迁移/值对象校验/领域事件 | `testing` + testify 可选；纯 table-driven | **不启动 DB/Redis/Temporal/HTTP** |
| `application` | 用例编排、事务语义、仓储端口调用、幂等/回退 | fake/替身实现端口 | 不依赖真实基础设施 |
| `infra` | PO 映射、仓储实现、迁移、缓存、事件投递 | `testcontainers-go`（PG/Redis/MinIO/Temporal dev） | 可重复隔离环境 |
| `api` | 参数校验、鉴权、DTO 转换、HTTP 状态/错误 | `net/http/httptest` + OpenAPI 契约校验 | 不以 API 测试代替领域规则测试 |
| `cmd`/e2e | 装配、跨域闭环 | `test/` + testcontainers 全栈 | 每条发布主线一条冒烟 |

- 新增/修改聚合行为必须先补领域测试；跨域事件变更必须更新契约测试。
- 全部 `go test -race`；关键路径表驱动 + 黄金/快照测试（报告渲染）。
- 测试不依赖开发者本机隐式环境；外部依赖经 testcontainers 或替身。

---

## 11. 环境与本地开发

- `.env` 是版本控制中的共享本地开发基线（映射 im：环境差异用受管环境变量覆盖，生产/共享凭据不入库）；`.env.example` 提供模板。
- 本地一键调试：GP 组件以 **Helm 部署到 kind 集群内命名空间（ns: gp）**；本地调试 = 直接部署到 kind，或 `go run ./cmd/server` + 依赖经 kind 代理暴露后连接（port-forward / NodePort / ingress-nginx + kind extraPortMappings）。**不使用 docker 部署形态做调试**；`docker-compose` 仅保留给本机 testcontainers（集成测试回退，非首选）。
- 宿主机透传端口避让同环境既有服务（PG 5433 / Redis 6380 / MinIO 9100+9101 / Temporal 专属 7233+8080）；`.env` 指向宿主机端口。
- 调试输出统一 `.outputs/logs/`；禁把临时日志写仓库根。
- Windows PowerShell 批量改写源码必须显式 UTF-8（禁裸 `Set-Content`/`Out-File` 缺编码）。

---

## 12. 发布与 CI（GitHub Actions）

- **语义化版本 + git tag**：`vX.Y.Z`；发布前跑全量 `make check`；正式镜像用 tag，禁 `image@sha256` 运行时依赖。
- CI 流水线：`lint+test` → `build` → `govulncheck` → `migcheck` → 镜像构建 → 推送；任一门禁失败不合并。
- 镜像：多阶段 `Dockerfile`（builder 用固定 toolchain + `CGO_ENABLED=0` 静态二进制 + `go:embed` 打包配置/迁移/前端产物）；私有化可单二进制交付。
- 部署：Docker/Helm（Temporal/Redis/PG/Timescale/MinIO 内网化）；密钥经 Secret/受管环境注入，禁入库。
- 发布后验证制品可启动、迁移成功、关键链路可观测；失败制品按策略撤回，不静默覆盖。

---

## 13. 前后端接口契约（整合 DESIGN-SPEC）

- 前端沿用 React 19 + Vite + TS + Tailwind4 + shadcn/ui（见原型），与后端**纯 API 解耦**：`api/openapi.yaml` 为唯一契约，前端类型由契约生成，禁止各自手写漂移。
- 实时执行进度用 WebSocket/SSE；错误/状态语义与 DESIGN-SPEC 的状态色（emerald/amber/red/slate）保持一致。
- 前端 mock（`src/data/mock.ts`）演进为 Mock Service Worker，与真实 API 同构，前端开发不阻塞后端。
- 审计/成本/门禁口径（总览=Σ明细、可溯源）由后端单点计算保证，前端只展示不重算。

---

## 14. 变更检查清单

- 新增服务/域前：确认归属 `internal/<domain>`，明确限界上下文、聚合根、权威表、跨域契约、领域事件与数据所有权。
- 新增依赖前：检查是否已有 `go.mod` 治理；确认仅该包需要；禁浮动态版本。
- 改迁移/RLS/表结构前：`migcheck` + testcontainers 专项；登记表前缀；只增不改。
- 改契约（OpenAPI）前：契约校验 + 前端类型生成 + 消费方契约测试。
- 改 Makefile/go.mod 后：`make validate` + 根 `make check`。
- 提交前：全量 `make check` 通过才可声明"完成"；任一门禁失败不得跳过、排除或降级。

---

> 工程规范 v1.0 · 2026-09-29 · 生效。参考 open-im-server 工程治理骨架 + Google/Uber/字节 Go 工程实践。与 DESIGN-SPEC（前端）并行生效。

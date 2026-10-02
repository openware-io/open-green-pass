# GreenPass · 实施执行方案（Implementation Plan）

> 状态：待执行（v0.3，**全阶段可执行**：P0/P1 实现级 + P2–P4 任务级施工图）
> 依据约束：**[ENGINEERING-SPEC.md](./ENGINEERING-SPEC.md)**（硬约束）+ **[TECH-DESIGN.md](./TECH-DESIGN.md)** §9（技术方案）+ **[DESIGN-SPEC.md](./DESIGN-SPEC.md)**（前端 UI/交互）。
> 定位：**照单执行**——用户说「开始」即按本方案编号任务执行，每项含「实施步骤 / 关键文件 / 验证」，阶段含完成定义（DoD）。
> 环境前提：本地 kind 集群已运行 **im saas（ns: im-saas，不动）**；GP 软隔离接入（ns: gp + ns: gp-runner）；**im saas 是 GP 首个被测对象（SUT）**；已确认：本地调试不用 docker 部署形态（go run + kind 依赖代理）、宿主机端口避让 im-saas。

---

## 0. 执行总则（硬标准）

1. **先做后验**：任务「完成」= DoD 满足 + 对应 `make check`/测试全绿；不得跳过/排除制造假绿。
2. **规范即底线**（ENGINEERING-SPEC）：`domain` 零三方依赖；`api→application→domain←infra`；表前缀登记；迁移只增不改；RLS 强制；分层测试；slog+OTel。
3. **口径自洽**（贯穿）：总览=Σ明细、版本可溯源、成本/审计单点计量。
4. **依赖顺序**：P0 → P1 串行；P1 内先 governance（被测对象/用例）→ execution → trusted/AI。
5. **部署形态与本地调试（硬约束）**：GP 全部组件（server/worker + 基础设施）以 **Helm 部署到 kind 集群内命名空间**（ns: gp 控制面+基础设施、ns: gp-runner 执行沙箱）；本地调试 = 直接部署到 kind，或 **`go run ./cmd/server` + 依赖经 kind 代理暴露后连接**（port-forward / NodePort / ingress-nginx + kind extraPortMappings）；**不使用 docker 部署形式做调试**。
6. **宿主机端口避让**：GP 依赖透传用独立宿主机端口段（PG 5433 / Redis 6380 / MinIO 9100+9101），避开 im-saas 占用（Temporal 为 GP 专属保留 7233/8080）。
7. **里程碑确认点**：P0 完成 → 用户确认 → P1；P1 首个 SUT 闭环 → 用户验收 → P2–P4（已按任务级施工图预输出 GP2-*/GP3-*/GP4-*，按图执行，阶段 DoD → 用户验收 → 下一阶段）。

---

## 1. P0 工程骨架（脚手架）——实现级

### GP0-01 目录骨架 + Go Module
**实施步骤**
1. `cd` 到 GP 后端服务仓库根（`D:\projects\cnb-oss\open-green-pass`），`git checkout` 干净分支。
2. 初始化 module：`go mod init github.com/openware-io/open-green-pass`，`go 1.24`。
3. 建目录（含 `.gitkeep`）：
```
cmd/server cmd/worker
internal/gateway{/middleware,/httpx}
internal/governance{/api,/application,/domain,/infra}
internal/execution{/api,/application,/domain,/infra}
internal/ai{/api,/application,/domain,/infra}
internal/trusted{/api,/application,/domain,/infra}
internal/platform{/config,/errors,/paging,/observability,/rls}
pkg/{protocol,clock,hashchain,id}
api/ migrations/ deploy/{helm,k8s} scripts/validate test/
```
**关键文件**
- `go.mod`：`module github.com/openware-io/open-green-pass` / `go 1.24` / `toolchain go1.24.x`（固定）。
- 每个 `internal/<domain>/domain/README.md`：声明"本包零第三方依赖，只允许 stdlib + pkg + 同域"。
**验证**：`go build ./...` 通过；`go list ./...` 输出全包。

### GP0-02 Makefile + golangci 门禁
**实施步骤**
1. 建 `Makefile` + `.golangci.yml` + `scripts/validate/validate-migrations.sh`（跨平台主脚本）+ `validate-migrations.ps1`（Windows 原生等价）。
2. 依赖锁定：`go get` 后 `go mod tidy && go mod verify`。
**关键文件（Makefile targets）**
```makefile
validate : gofumpt -l . && goimports -l . && go vet ./... && staticcheck ./...
lint     : golangci-lint run
test     : go test -race ./...
build    : go build ./cmd/... && go vet ./cmd/...
vuln     : govulncheck ./...
migcheck : ./scripts/validate/validate-migrations.sh   # 跨平台（Linux CI + 本地 git-bash/WSL）；Windows 原生 PowerShell 可另跑同规则 .ps1
check    : validate lint test build vuln migcheck
```
**`.golangci.yml` 关键（depguard 依赖边界——最核心）**
```yaml
linters:
  enable: [depguard, revive, errcheck, rowwerrcheck, gocyclo, gofumpt, goimports, gosec]
linters-settings:
  depguard:
    rules:
      domain-zero-thirdparty: &depdomain
        files: ["**/internal/*/domain/**"]
        allow: ["$gostd", "github.com/openware-io/open-green-pass/pkg"]
      application-no-orm:
        files: ["**/internal/*/application/**"]
        deny: [{pkg: "gorm.io/gorm", desc: "application 禁 ORM"}, {pkg: "go.temporal.io/sdk", desc: "application 禁 Temporal SDK"}]
  gocyclo:
    min-complexity: 15
```
**验证**：`make check` 全绿；临时在 `governance/domain` 引入 `gorm.io/gorm` → depguard 报错（证明边界生效）后移除。

### GP0-03 基础设施 kind + Helm + 命名空间 + 端口
**实施步骤**
1. kind 装 `local-path-provisioner`（或 hostPath PV）：`kubectl apply -f https://.../deploy.yaml` 或 `helm install local-path ...`。
2. 建命名空间：`deploy/k8s/namespaces.yaml` → `kubectl apply -f`（含 `gp`、`gp-runner`）。
3. 基础设施 Helm 到 ns: gp（各自独立，不复用 im-saas）：
```
helm repo add bitnami https://charts.bitnami.com/bitnami && helm install gp-postgres bitnami/postgresql -n gp --set auth.database=gp_,auth.postgresPassword=...
helm install gp-redis  bitnami/redis          -n gp --set auth.password=...
kubectl apply -f deploy/k8s/seaweedfs.yaml    # 对象存储：MinIO 社区版已归档(410 Gone)，改用 S3 兼容 SeaweedFS（svc/gp-seaweedfs，S3 on 9000）
helm repo add temporalio https://helm.temporal.io && helm install gp-temporal temporalio/temporal -n gp --set server.config.storeProvider.postgres={...}
```
4. 网络/资源隔离：`deploy/k8s/gp-runner-policy.yaml`（NetworkPolicy：仅出向 im-saas 被测服务 + gp 对象存储(SeaweedFS)；ResourceQuota + LimitRange 硬顶）。
**关键文件**
- `deploy/k8s/namespaces.yaml`：`apiVersion v1 kind Namespace`，name `gp` / `gp-runner`。
- `deploy/helm/gp/values*.yaml`：各 chart 覆盖（PG 库名 gp_、对象存储桶(SeaweedFS)、Temporal 指向 gp-postgres）。
**宿主机端口透传（避让 im-saas，见 §0-6）**：
```
kubectl port-forward -n gp svc/gp-postgres-postgresql 5433:5432
kubectl port-forward -n gp svc/gp-redis-master 6380:6379
kubectl port-forward -n gp svc/gp-temporal 7233:7233
kubectl port-forward -n gp svc/gp-temporal-web 8080:8080
kubectl port-forward -n gp svc/gp-seaweedfs 9100:9000   # 对象存储 S3（SeaweedFS，无独立 console）
```
`.env`：`DB_HOST=127.0.0.1:5433`、`RedisAddr=127.0.0.1:6380`、`TemporalAddr=127.0.0.1:7233`、`MinIOEndpoint=127.0.0.1:9100`。 建议落 `scripts/dev/kind-forward.ps1`：一次性后台拉起上述 port-forward（含 start/stop/cleanup），避免每次手工 kubectl；端口已避让 im-saas。
**验证**：`helm ls -n gp` 全 deployed；`kubectl get pods -n gp` ready；`kubectl get ns` 含 im-saas(未动)/gp/gp-runner；NetworkPolicy/Quota 生效；`kubectl get svc -n im-saas` 核对 GP 宿主端口与其错开。

### GP0-04 迁移基线 + RLS
**实施步骤**
1. `golang-migrate` 初始化 `migrations/`（`000001_init.up/down.sql`）。
2. `internal/platform/rls` 建 RLS 函数 + 策略。
**关键文件（`migrations/000001_init.up.sql` 要点）**
```sql
-- 审计字段列集（供各表套用）：created_by/created_at/updated_by/updated_at/deleted_at
CREATE FUNCTION gp.set_tenant() RETURNS trigger AS $$ BEGIN
  NEW.team_id := current_setting('gp.team_id', true)::bigint; RETURN NEW; END $$ LANGUAGE plpgsql;
-- RLS 策略模板：USING (team_id = current_setting('gp.team_id')::bigint)  对 tgt_*/cas_*/run_*/cost_*/aud_* 等业务表启用
ALTER TABLE ... ENABLE ROW LEVEL SECURITY;
```
`scripts/validate/validate-migrations.sh`（Windows 等价 `.ps1`）：校验脚本名 `000NNN_*.up/down.sql`、历史只增不改、表前缀归属（对照 ENGINEERING-SPEC §8 登记表）。
**验证**：`golang-migrate up` 成功；`make migcheck` 通过；重跑幂等。

### GP0-05 platform 基础包 + pkg 基础包
**pkg 基础包（P0 同落地；P1 端口契约/审计链/幂等依赖）**：pkg/id（Snowflake 发号）、pkg/hashchain（prev_hash 链）、pkg/clock、pkg/protocol（契约常量）。验证：go test ./pkg/... 全绿。
**关键文件（要点）**
- `config/config.go`：`type Config struct{ DB, Redis, Temporal, MinIO, ... }`；从 `.env`/环境变量加载；生产凭据不提交。
- `errors/errors.go`：`type DomainError struct{ Code, Message }` + `Is/As` 支持 + 技术错误→领域错误映射。
- `paging/paging.go`：`Page{Limit, Offset} + Sort + Total`，统一 `Parse(ctx)` + `Response`。
- `observability/observability.go`：`slog` Handler 接 OTel；`WithRunID/WithCaseID/WithTenant(...)` 字段 helper；敏感脱敏（key/token/正文）。
**验证**：`go test ./internal/platform/...` 全绿（单测覆盖 Is/As、分页边界、日志字段）。

### GP0-06 gateway 接入层
**关键文件**
- `middleware/tenant.go`：从 JWT/上下文提取 `team_id` → `current_setting('gp.team_id')`（RLS 依据）+ context。
- `middleware/request_id.go`：生成/透传 `request_id`。
- `middleware/authz.go`：RBAC+资产级鉴权（P1 接 RBACRepository）。
- `middleware/audit.go`：写 `aud_event` 端口（P1 接 trusted）。
- `httpx/response.go`：统一 `{code,message,data,request_id}` + 错误编码 + 分页。
**验证**：httptest 验证 tenant 提取、request_id 透传、错误格式统一。

### GP0-07 四域骨架 + 端口契约
**关键文件（端口接口签名，P1 实现）**
```go
// governance/domain/port
type TargetRepository interface { FindByID(ctx, id) (*Target, error); ListByTeam(ctx, teamID) ([]*Target, error); Save(ctx, *Target) error }
type CaseRepository interface { SaveVersion(ctx, *Case) error; FindVersion(ctx, caseID, ver) (*CaseVersion, error); History(ctx, caseID) ([]*CaseVersion, error) }
// execution/domain/port
type RunRepository interface { Save(ctx, *Run) error; FindByID(ctx, id) (*Run, error) }
type QuotaPort interface { Acquire(ctx, teamID, targetID, ownerID, res Resource) error; Release(...) error }
// ai/domain/port
type ModelGatewayPort interface { Generate(ctx, req) (*GenResult, error); Meter(requestID, biz, tokens) error }
// trusted/domain/port
type GateRepository interface { Load(ctx, targetID) (*GateRule, error); SaveResult(ctx, *GateResult) error }
type AuditRepository interface { Append(ctx, *AuditEvent) error; Verify(ctx, id) (*AuditEvent, error) }   // append-only
type CostRepository interface { Insert(ctx, *CostLineItem) error; Summarize(ctx, q) (*CostSummary, error) }
```
**验证**：`go build ./...`；depguard 确认 domain 无三方依赖。

### GP0-08 CI（GitHub Actions）
**关键文件（`.github/workflows/ci.yml`）**
```yaml
jobs:
  check:
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5  # go-version-file: go.mod
      - run: make validate lint test build vuln
      - run: make migcheck
```
**要点**：固定 toolchain；go.sum 锁定；禁 floating；任一门禁失败不合并。
**验证**：本地 `actionlint` 或 yaml 语法校验通过。

### GP0-09 集成测试基座（连 kind 真实依赖）
**实施步骤**
1. `test/` 建集成测试；依赖统一从 kind 代理（与 GP0-03 端口一致），不另起 docker。
2. 有集群连真实 PG/Redis/MinIO/Temporal；CI 无集群时 `testcontainers-go` 仅作回退（非首选）。
**验证**：一条连接真实 PG 的 Repository 集成测试跑通（隔离可重复）。

**P0 完成定义（DoD）**：目录骨架 + `make check` 全绿；基础设施 Helm 到 kind（ns: gp）且代理暴露可连（`go run ./cmd/server` 连真实依赖通过）；软隔离命名空间 + RLS + 迁移基线就绪；四域骨架与端口契约建立；CI 就绪；im-saas 未被改动。**→ 交付 P0 小结，请用户确认后进入 P1。**

---

## 2. P1 最小闭环（im saas 为首个 SUT）——实现级

> 目标：一个被测对象（im saas）跑通「绑仓库 → 版本校验 → 生成用例(带 change) → 执行 1 个 API 用例 → 证据 → 门禁 → 报告 → 成本可下钻对比」。**口径自洽为硬验收**。P1 在 P0 验收后执行；依赖 P0 端口契约与 pkg 基础包（id/hashchain）。

### GP1-01 被测对象树 + 仓库绑定（governance）✅ 已交付（9527d39）
**实施步骤**
1. 迁移 `000002_target_repo.up.sql`：建 `tgt_target`（L0 工程/L1 服务组/L2 服务/L3 模块）+ `repo_repo`/`repo_branch`（DDL 见 TECH-DESIGN §9.3）+ 启用 RLS。
2. governance/domain：`Target` 聚合（`AttachRepo`/`SetModelBinding`/`ResolveVersionChain`）+ `TargetRepository` 端口实现（PO 在 infra）。
3. governance/api：`POST /targets`（建树节点）、`POST /targets/{id}/repo`（绑仓库/分支/版本）、`GET /targets`（树 + 维度筛选，DESIGN-SPEC）。
4. 登记首个 SUT：team 下建 im-saas 工程→服务→模块节点 + 绑仓库（仓库+分支+版本）。
**关键文件**
- `migrations/000002_target_repo.up/down.sql`
- `internal/governance/{domain,application,api,infra}/*`（Target/Repo）
- `api/openapi.yaml` 补 `/targets` 契约
**验证**
- 迁移 up + `make migcheck` 通过；RLS：跨 team 查询返回空。
- httptest：`POST /targets/{id}/repo` 成功；`GET /targets` 树正确。

### GP1-02 用例版本化（governance）✅ 已交付（待提交）
**实施步骤**
1. 迁移 `000003_case.up.sql`：`cas_case`/`cas_version`（`change_type`/`source_repo_id`/`source_branch`/`script_json` + `current_version`）+ RLS。
2. governance/domain：`Case` 聚合（`CreateVersion`/`MarkChange`/`RollbackTo`）+ `CaseRepository`（`SaveVersion`/`FindVersion`/`History`/`Rollback`）。
3. api：`POST /cases`、`POST /cases/{id}/versions`（新版本/change）、`POST /cases/{id}/rollback`（回退=新增指向旧内容的新版本，非物理删）、`GET /cases/{id}/history`（版本对比）。
4. 首个用例种子：绑定 im-saas 服务，生成 1 个 API 用例（`script_json`=HTTP 定义）。
**验证**：版本化正确；change 标记可辨；回退后 `current_version` 指向旧内容新版本；history 完整；`make check` 全绿。

### GP1-03 版本校验（execution）✅ 已交付——"测了没白测"
**实施步骤**
1. 迁移 `000004_env.up.sql`：`env_runtime`/`env_check`（`result: match/mismatch/unknown`）+ RLS。
2. execution/domain：`EnvVersion` 聚合 + 版本比对端口；目标版本 vs 环境运行版本比对。
3. api：`POST /targets/{id}/version-check`；`POST /runs` 前置调用。
4. `mismatch` → 阻断执行 + 写 `env_check` + 入审计链（`aud_event`）。
**验证**：目标≠环境 → 阻断 + 审计留痕；相等 → 放行；幂等重跑一致。

### GP1-04 单 API 场景执行（execution）✅ 编排闭环已交付（K8s Job Runner 已接入 gp1-04-b ✅）
**实施步骤**
1. 迁移 `000005_run.up.sql`：`run_run` + `run_case_result`（含 `attempt_seq`，支持重试/重跑）+ RLS。
2. execution/domain：`Run` 聚合（`StartRun`/`CollectEvidence`/`ApplyGate`/`ProduceReport`，状态机）+ `RunRepository`。
3. K8s Job 沙箱：`deploy/k8s/gp-runner/job-template.yaml`（ns: gp-runner，NetworkPolicy 仅出向 im-saas + gp MinIO）；execution/infra `RunnerPort`（建 Job、轮询、回收、日志→MinIO）。
4. api：`POST /runs`（支持当次用例勾选/用例树范围筛选——PRD）、`GET /runs/{id}`（实时 SSE）、`GET /runs/{id}/case-results`。
5. `CollectEvidence`：截图/日志→MinIO + sha256；服务级截图开关（策略下发）。
**验证**：状态机推进（Queued→VersionCheck→Scheduled→Running→Collect→Gate→Report→Done/Failed）；用例结果落 `run_case_result`（`attempt_seq` 递增支持重试）；证据在 MinIO；失败可重跑。

### GP1-05 门禁（trusted，OPA）✅ 已交付（声明式策略 P1；完整 OPA 引擎 P2）
**实施步骤**
1. 迁移 `000006_gate.up.sql`：`gate_rule`/`gate_result` + RLS + `gp_trusted_writer` 角色。
2. trusted/domain：`GateRule`（Rego 装载）+ `GateResult` + `GateRepository`；execution→trusted 经 `GatePort`。
3. 策略即代码：覆盖率/断言门禁 Rego 进 git（`deploy/rego/`）；`PUT /gates/rules`。
4. 判定结果入审计链。
**验证**：覆盖率不足→`blocked`；满足→`pass`；判定入 `aud_event`；`make check` 全绿。

### GP1-06 成本明细（trusted）——口径单点 ✅ 已交付
**实施步骤**
1. 迁移 `000007_cost.up.sql`：`cost_line_item`（`biz_category`/`biz_point`/`idempotency_key` + `UNIQUE(idempotency_key)`）+ RLS + `gp_trusted_writer`。
2. trusted/domain：`CostLineItem` + `CostRepository`（`Insert`/`Summarize`/`CompareHistory`）。
3. ai 域 `MeterPort`→trusted：P1 用 mock 计量端口，生成/执行各写一条（`biz_category` 分 generate/execute）。
4. api：`GET /cost/overview`（总览=Σ明细，按团队/对象/大类）、`GET /cost/items`（生成/执行分大类）、`GET /cost/compare/{caseId}`（历史对比）。
**验证**：总览=Σ明细；幂等键防重复（同 `request_id` 不双计）；历史对比显示存量成本持平/递减；`make check` 全绿。

### GP1-07 报告（HTML）✅ 已交付
**实施步骤**
1. 迁移 `000008_report.up.sql`：`rpt_report`（`kind: project/service/case`）+ RLS。
2. trusted/domain：`Report` + `ReportRepository`；服务端 HTML 模板（`internal/trusted/.../report/templates` 用 `go:embed` 打包）。
3. api：`GET /reports/{id}/export?fmt=html`；证据经 MinIO 签名 URL 嵌入 + 哈希标注。
4. P1 单场景报告（API 场景）；结构预留跨场景合编（P2）。
**验证**：报告渲染正确、证据可访问；export=html 返回可打开；`make check` 全绿。

### GP1-08 前端对接（契约先行）⏳ 契约生成、真实执行链路、报告控制、被测对象、用例管理、成本审计、门禁真实区、运行 SSE、运行历史列表和 MSW 基础联调层已交付；其余页面真实接口接入待完成
**实施步骤**
1. `api/openapi.yaml` 定稿（含 `/targets /cases /runs /gates /cost /reports`）→ openapi-typescript 生成类型。
2. 前端 `mock.ts` → Mock Service Worker（与真实 API 同构）；API client 接入。**当前：** 已由 `openapi-typescript` 生成 `src/api/generated.ts`，`src/api/client.ts` 仅消费生成类型；`/exec` 的真实运行控制已接入 targets/cases 查询、运行创建（支持勾选真实用例 ID）、版本校验、执行、暂停/恢复和结果查询；运行创建响应已使用执行域 DTO；`/history` 已接入真实 `GET /runs` 列表；`/targets` 已接入真实对象查询、节点创建和仓库绑定；`/cases` 已接入真实 targets/cases 查询、创建、版本提交、历史、回退和删除；`/audit-cost` 已接入真实成本总览、明细与单用例历史对比；`/gate` 已接入真实运行门禁判定和结果查询。已加入显式 `VITE_GP_MOCK_API=true` 的 MSW 浏览器联调层，处理器只覆盖已定义 OpenAPI 路径，真实 `VITE_GP_API_BASE` 优先。版本运行时、门禁规则/结果、成本、运行列表返回均已补齐明确 OpenAPI schema，client 不再为这些接口使用松散 `Record<string, unknown>`。真实调用须显式配置 `VITE_GP_API_BASE` 与数值型 `VITE_GP_TEAM_ID`，不会将原型 ID 写入后端。真实用例版本历史已通过 API DTO 输出，领域对象不直接序列化。
3. 实时：`/runs/{id}` SSE；状态色板映射（`pass→emerald/fail→red/pending→amber`，DESIGN-SPEC §2.1）。**当前：** 已增加 `/runs/{id}/events` SSE 契约和执行域状态流：服务端首帧立即返回，随后按权威仓储状态变化轮询推送；前端使用带 `x-gp-team-id` 的 fetch 流式读取，终态自动关闭连接。当前实现不依赖 Redis，适合单实例/代理验证；多副本共享 Pub/Sub 仍属于 GP2-07。
4. 列表页查询条件（PRD）。
**验证**：前端直连本地 server（依赖经 kind 代理）跑通闭环；SSE 实时更新；色板一致。

**P1 完成定义（DoD）**：im saas 首个 SUT 全闭环跑通；版本校验阻断生效；成本总览=Σ明细且分大类/可下钻/历史对比；报告 HTML 可导出；`make check` 全绿；前端契约接入。**→ 交付 P1 验收，用户确认后进入 P2。**

---

## 3. P2 异构执行 + 调度（任务级施工图）

> 定位：从单 API 场景扩展到 12 类异构场景 + 完整调度器 + 水平扩展（§10）。每项 DoD 用 `make check` + 集成测试验证。

- **GP2-01 异构执行引擎抽象 + 场景接入**：执行器抽象与场景注册已完成（`internal/execution/infra/runner`）：Playwright/STF/k6 适配器输出统一 `Plan`，API/浏览器/真机/压测资源类型路由、证据哈希、失败传播均有单测；server/worker 支持 `GP_RUNNER_TYPE=scenario`。**当前剩余**：将 `CommandExecutor` 接到真实 Playwright 浏览器集群、STF 真机池、k6/Locust 资源池，并补对应 kind 集成测试；在真实资源池接入完成前不得宣称 GP2-01 阶段 DoD 完成。
- **GP2-02 完整 W2 迁移到 Temporal ✅ 已交付（执行链纳入 Temporal 编排）**：执行流水线全量迁 Temporal Workflow（VersionCheck→AcquireResources→Dispatch→Collect→Gate→Cost→Report）；`cmd/worker` 注册 W2 多实例。关键文件：`internal/execution/workflow/execute_run.go`。验证：workflow 测试 + 暂停/恢复/重试 + 崩溃恢复。
- **GP2-03 多级配额 + 公平队列（Redis Streams CG + 原子 Lua + WFQ）**：调度契约与本地参考实现已完成（`internal/execution/domain/quota.go`、`internal/execution/infra/quota`）：租户→工程→负责人三级配额、资源类型隔离、原子扣减/释放、加权公平队列均有单测与并发验证。**当前剩余**：接入 Redis Streams Consumer Group + Lua 原子脚本，并在 kind Redis 上完成跨进程/多 worker 集成测试；Redis 接入前不得宣称 GP2-03 DoD 完成。
- **GP2-04 冲突检测 + Pause/Resume**：暂停/恢复控制边界已完成：application 通过 `WorkflowController` 发送 Temporal `pause`/`resume` signal，workflow 在版本校验后等待恢复；资源互斥已有 `ConflictPort` 与线程安全进程内参考实现，并已接入 `RunService.ExecuteRun` 的目标级独占边界，冲突返回 HTTP 409。**当前剩余**：冲突事件写入审计链、跨进程 Redis/数据库实现、Temporal 多 worker 集成验证；因此 GP2-04 DoD 尚未完成。
- **GP2-05 截图开关策略下发 ✅ 已交付**：服务级截图开关按对象策略下发（防高并发性能开销，PRD R-TEST-13）。验证：策略生效；无截图场景证据=日志+hash。
- **GP2-06 跨场景报告合编（PDF/Word）**：跨 run 合编查询、汇总和 HTML/Markdown 输出已完成（`POST /reports/bundle`，tenant scoped）；已新增 `POST /reports/bundle/export` 的 PDF/DOCX 下载契约、纯 Go 参考渲染器、OpenAPI 生成类型和前端客户端。**当前剩余**：真实字体/中文排版、Office/PDF 阅读器兼容性、证据资源嵌入和部署运行时集成验证；因此 GP2-06 阶段 DoD 尚未完成。
- **GP2-07 水平扩展 P2（§10）**：server 多副本 + worker 多实例 + 调度器无状态化（P1 起无状态代码实装）。关键文件：`deploy/helm/gp`（replica）、SSE 共享订阅（Redis Pub/Sub）。验证：加副本吞吐线性上升（压测基线 §10.10）。
- **GP2-08 资源池注册 + 多执行集群**：本地资源池注册/能力选择/容量 reserve-release 参考实现已完成（`internal/execution/infra/pool`），当前剩余资源池持久化、心跳和真实多集群接入验证。

## 4. P3 AI 治理 + 生成管道 W1（任务级施工图）

- **GP3-01～GP3-07**：已补充可开发实施规格 `docs/GP3-DETAILED-IMPLEMENTATION-SPEC.md`，覆盖各项目标边界、现有缺口、用户/架构决策、契约与事件、迁移、分层职责、测试验收和依赖顺序。当前仍按下述状态执行：GP3-01 统一 AI 网关参考实现已完成，真实 LiteLLM/供应商、超时和成本接入待完成；GP3-02～GP3-07 均未达到阶段 DoD，须按规格逐项开发和验收。
- **GP3-02～GP3-07**：目标与验收保持不变，具体实施拆分、契约、迁移、分层边界和前置决策以 `docs/GP3-DETAILED-IMPLEMENTATION-SPEC.md` 为准；在真实依赖和用户决策未确定前，不宣称代码或 DoD 完成。

## 5. P4 平台化（任务级施工图）

- **GP4-01～GP4-06**：已补充可开发实施规格 `docs/GP4-IMPLEMENTATION-SPEC-DRAFT.md`，覆盖目标、前置决策/外部依赖、OpenAPI/事件、迁移、分层、测试验收和 GP2/GP3 依赖。当前仍不得宣称 GP4 完成：身份源/RBAC、CI 首批平台、私有化拓扑、租户分区策略和 SLO 需先定稿。
- **GP4-02～GP4-06**：目标与验收保持不变，具体实施拆分、契约、迁移、分层边界和前置决策以 `docs/GP4-IMPLEMENTATION-SPEC-DRAFT.md` 为准；真实 provider、集群和容量验收仍待用户决策与环境准备。

---

## 6. 执行节奏与确认点

| 阶段 | 串行门槛 | 确认点 |
|---|---|---|
| P0 骨架 | — | P0 DoD → 用户确认 → P1 |
| P1 最小闭环 | P0 完成 | P1 首个 SUT 闭环 → 用户验收 → P2 |
| P2 / P3 / P4 | 前一阶段验收 | 各阶段已输出任务级施工图（GP2-*/GP3-*/GP4-*），按图执行；阶段 DoD → 用户验收 → 下一阶段 |

**开始信号**：用户确认「开始」→ 从 **GP0-01 目录骨架** 起照 GP0-* 执行；每项完成即 `make check`/测试验证；P0 结束交付小结。

---

> 实施执行方案 v0.3（全阶段可执行）· 2026-09-29 · 待执行。约束依据 ENGINEERING-SPEC + TECH-DESIGN §9/§10 + DESIGN-SPEC；im saas 首个 SUT；本地软隔离；宿主机端口避让 im-saas；P0/P1 实现级 + P2–P4 任务级施工图。

// EXPORTS: 实体数据 + 派生指标（METRICS 从明细计算，保证跨页自洽）
// 原则：仪表盘 KPI 一律由 REQUIREMENTS/CASES/CONTRACTS/GATE_RULES 推导，禁止硬编码对不上明细的数值

// ============ 被测对象画像 ============
export interface ITestTarget {
  name: string;
  kind: string;
  producer: string;
  version: string;
  qualityScore: number;
  stage: '接收' | '理解' | '生成' | '执行' | '门禁' | '审计';
  riskCount: number;
  passRate: number;
  desc: string;
}

export const TEST_TARGET: ITestTarget = {
  name: 'sys-payment-platform',
  kind: 'AI 生成微服务系统',
  producer: 'codegen-agent / ai-agent-3',
  version: '2.5.0',
  qualityScore: 65,
  stage: '门禁',
  riskCount: 3,
  passRate: 78.3,
  desc: '由编码 Agent 生成的支付域微服务集群，进入质量门禁验收，检出 3 项风险待处理',
};

// 质量评分构成：各维度得分与权重，加权和 ≈ qualityScore（78）
export interface IQualityDim {
  label: string;
  score: number;      // 该维度 0-100
  weight: number;     // 权重占比（0-1）
  note: string;
}

export const QUALITY_DIMS: IQualityDim[] = [
  { label: '需求覆盖率', score: 75, weight: 0.25, note: '3/4 需求已覆盖' },
  { label: '门禁通过', score: 50, weight: 0.3, note: '3/6 规则通过' },
  { label: '契约健康', score: 80, weight: 0.2, note: '4/5 契约通过' },
  { label: '用例有效', score: 67, weight: 0.15, note: '变异验证通过率' },
  { label: '追溯完整', score: 50, weight: 0.1, note: '2/4 需求完整追溯' },
];

// AI 生成过程还原：被测对象如何由 AI 产出（从需求到提交验收）
export interface IGenTrace {
  seq: number;
  step: string;
  actor: string;
  time: string;
  desc: string;
  status: 'done' | 'active' | 'blocked';
}

export const GEN_TRACE: IGenTrace[] = [
  { seq: 1, step: '规格解析', actor: 'ai-agent-1', time: '09-22 14:02', desc: '读取 Jira 需求与 Confluence 设计，抽取状态机与接口', status: 'done' },
  { seq: 2, step: '架构生成', actor: 'codegen-agent', time: '09-22 16:40', desc: '产出服务拆分、数据模型与 OpenAPI 契约', status: 'done' },
  { seq: 3, step: '代码生成', actor: 'codegen-agent', time: '09-23 09:15', desc: '生成 svc-auth / svc-payment / svc-order 及端侧代码', status: 'done' },
  { seq: 4, step: '自检回放', actor: 'ai-agent-2', time: '09-23 11:30', desc: '基线 42 用例回放通过，检出弱断言 3 处', status: 'done' },
  { seq: 5, step: '提交验收', actor: 'ai-agent-3', time: '09-24 10:25', desc: '提交至质量门禁，进入当前验收阶段', status: 'active' },
];

// 资产风险明细：每个服务的覆盖/门禁/风险（与 ASSET_TREE 同源）
export interface IAssetRisk {
  name: string;
  type: '服务' | '应用' | '端';
  coverage: number;
  gate: '通过' | '阻断';
  risk: string | null;
}

export const ASSET_RISKS: IAssetRisk[] = [
  { name: 'svc-auth', type: '服务', coverage: 100, gate: '通过', risk: null },
  { name: 'svc-user', type: '服务', coverage: 100, gate: '通过', risk: null },
  { name: 'svc-payment', type: '服务', coverage: 50, gate: '阻断', risk: '契约破坏性变更 · 断言弱化' },
  { name: 'svc-order', type: '服务', coverage: 100, gate: '通过', risk: null },
  { name: 'web-frontend', type: '应用', coverage: 100, gate: '通过', risk: null },
  { name: 'mobile-ios', type: '端', coverage: 100, gate: '通过', risk: null },
  { name: 'mobile-android', type: '端', coverage: 100, gate: '通过', risk: null },
];

// ============ 资产树（被测资产）============
export interface IAsset {
  id: string;
  name: string;
  type: 'service' | 'service-group' | 'app' | 'end';
  coverage: number;
  gateRate: number;
}

export interface IAssetNode extends IAsset {
  children?: IAssetNode[];
}

export const ASSET_TREE: IAssetNode = {
  id: 'sys-payment-platform',
  name: 'sys-payment-platform',
  type: 'service-group',
  coverage: 75,
  gateRate: 50,
  children: [
    {
      id: 'sg-identity',
      name: '身份服务组',
      type: 'service-group',
      coverage: 100,
      gateRate: 100,
      children: [
        { id: 'svc-auth', name: 'svc-auth', type: 'service', coverage: 100, gateRate: 100 },
        { id: 'svc-user', name: 'svc-user', type: 'service', coverage: 100, gateRate: 100 },
      ],
    },
    {
      id: 'sg-payment',
      name: '支付服务组',
      type: 'service-group',
      coverage: 50,
      gateRate: 0,
      children: [
        { id: 'svc-payment', name: 'svc-payment', type: 'service', coverage: 50, gateRate: 0 },
        { id: 'svc-order', name: 'svc-order', type: 'service', coverage: 100, gateRate: 100 },
      ],
    },
    {
      id: 'sg-frontend',
      name: '前端与端',
      type: 'service-group',
      coverage: 100,
      gateRate: 100,
      children: [
        { id: 'web-frontend', name: 'web-frontend', type: 'app', coverage: 100, gateRate: 100 },
        { id: 'mobile-ios', name: 'mobile-ios', type: 'end', coverage: 100, gateRate: 100 },
        { id: 'mobile-android', name: 'mobile-android', type: 'end', coverage: 100, gateRate: 100 },
      ],
    },
  ],
};

// ============ 需求（追溯链来源）============
export interface IRequirement {
  id: string;
  title: string;
  assets: string[];
  testPoints: number;
  cases: number;
  executed: number;
  traceability: '完整' | '部分' | '缺口';
}

export const REQUIREMENTS: IRequirement[] = [
  { id: 'REQ-101', title: '用户登录与鉴权', assets: ['svc-auth', 'svc-user', 'web', 'mobile'], testPoints: 5, cases: 12, executed: 12, traceability: '完整' },
  { id: 'REQ-102', title: '用户注册流程', assets: ['svc-auth', 'web'], testPoints: 3, cases: 9, executed: 9, traceability: '完整' },
  { id: 'REQ-103', title: '支付订单创建', assets: ['svc-payment', 'svc-order', 'web'], testPoints: 5, cases: 18, executed: 14, traceability: '部分' },
  { id: 'REQ-104', title: '退款与对账', assets: ['svc-payment'], testPoints: 0, cases: 0, executed: 0, traceability: '缺口' },
];

// ============ 治理事件 ============
export interface IGovernanceEvent {
  type: 'danger' | 'success' | 'warning';
  title: string;
  desc: string;
  meta: string;
}

export const GOVERNANCE_EVENTS: IGovernanceEvent[] = [
  { type: 'danger', title: '门禁阻断 · #4821', desc: 'svc-payment 检出 4 项质量风险，判定为阻断', meta: '5 分钟前 · gate-engine' },
  { type: 'danger', title: '契约变更告警 · svc-payment', desc: '/v2/refund 破坏性变更，影响 3 个消费者', meta: '2 分钟前 · 契约注册表' },
  { type: 'success', title: '用例种子生成 · 12 条', desc: '来自 OpenAPI 契约变更，通过质量验证', meta: '8 分钟前 · 生成管道' },
  { type: 'warning', title: '跨服务追溯链更新', desc: 'REQ-103 追溯链扩展到 svc-order', meta: '15 分钟前 · 追溯引擎' },
];

// ============ 测试用例 ============
export interface ICase {
  id: string;
  title: string;
  asset: string;
  source: string;
  type: '单元' | '集成' | 'Web' | '移动' | '安全';
  assertion: '强' | '中' | '弱';
  mutation: number | null;
  status: '已激活' | '冲突' | '待审核';
}

export const CASES: ICase[] = [
  { id: 'TC-2024-001', title: '正常登录流程验证', asset: 'svc-auth', source: 'REQ-101 + OpenAPI', type: '单元', assertion: '强', mutation: 92, status: '已激活' },
  { id: 'TC-2024-005', title: '用户状态查询', asset: 'svc-user', source: 'REQ-101', type: '单元', assertion: '强', mutation: 88, status: '已激活' },
  { id: 'TC-2024-006', title: 'Web 端登录页面 E2E', asset: 'web-frontend', source: 'REQ-101', type: 'Web', assertion: '强', mutation: 85, status: '已激活' },
  { id: 'TC-2024-095', title: '支付回调幂等性验证', asset: 'svc-payment', source: 'REQ-103 + OpenAPI', type: '集成', assertion: '中', mutation: 64, status: '已激活' },
  { id: 'TC-2024-118', title: '退款金额计算', asset: 'svc-payment', source: 'REQ-104', type: '单元', assertion: '弱', mutation: 31, status: '已激活' },
  { id: 'TC-2024-125', title: '移动端真机登录测试', asset: 'mobile-ios', source: 'REQ-101', type: '移动', assertion: '强', mutation: 79, status: '已激活' },
  { id: 'TC-2024-130', title: 'ASVS L2 认证安全检查', asset: 'svc-auth', source: 'REQ-101 + ASVS', type: '安全', assertion: '强', mutation: null, status: '已激活' },
];

// ============ 契约测试 ============
export interface IContract {
  id: string;
  provider: string;
  endpoint: string;
  consumers: string;
  status: 'pass' | 'pending' | 'fail';
}

export const CONTRACTS: IContract[] = [
  { id: 'CT-001', provider: 'svc-auth', endpoint: '/v2/login', consumers: 'web, ios, android', status: 'pass' },
  { id: 'CT-002', provider: 'svc-auth', endpoint: '/v2/token/refresh', consumers: 'web, ios, android', status: 'pass' },
  { id: 'CT-003', provider: 'svc-payment', endpoint: '/v2/refund', consumers: 'web, ios, svc-order', status: 'fail' },
  { id: 'CT-004', provider: 'svc-order', endpoint: '/v2/orders', consumers: 'web, ios, android', status: 'pass' },
  { id: 'CT-005', provider: 'svc-user', endpoint: '/v2/profile', consumers: 'web, ios, android', status: 'pass' },
];

// ============ 执行流（当前批次实时片段）============
export interface IFlowItem {
  id: string;
  title: string;
  asset: string;
  status: '通过' | '失败' | '执行中' | '阻塞';
  duration: string;
}

export const FLOW_ITEMS: IFlowItem[] = [
  { id: 'TC-2024-001', title: '正常登录流程 · svc-auth', asset: 'svc-auth', status: '通过', duration: '0.8s' },
  { id: 'TC-2024-005', title: '用户状态查询 · svc-user', asset: 'svc-user', status: '通过', duration: '1.2s' },
  { id: 'TC-2024-006', title: 'Web 端登录 E2E · web-frontend', asset: 'web-frontend', status: '通过', duration: '5.6s' },
  { id: 'TC-2024-095', title: '支付回调幂等性 · svc-payment', asset: 'svc-payment', status: '失败', duration: '3.1s' },
  { id: 'TC-2024-118', title: '退款金额计算 · svc-payment', asset: 'svc-payment', status: '失败', duration: '2.3s' },
  { id: 'TC-2024-125', title: '移动端真机登录 · mobile-ios', asset: 'mobile-ios', status: '通过', duration: '8.2s' },
  { id: 'TC-2024-131', title: '并发登录压力测试 · svc-auth', asset: 'svc-auth', status: '阻塞', duration: '—' },
];

export interface IExecRun {
  label: string;
  branch: string;
  progress: number;
  passed: number;
  failed: number;
  queued: number;
  running: number;
  blocked: number;
  total: number;
  eta: string;
}

// 总览统计与实时片段同源：passed/failed 与 FLOW_ITEMS 状态一致
const flowPassed = FLOW_ITEMS.filter((f) => f.status === '通过').length;
const flowFailed = FLOW_ITEMS.filter((f) => f.status === '失败').length;
const flowBlocked = FLOW_ITEMS.filter((f) => f.status === '阻塞').length;
const flowRunning = FLOW_ITEMS.filter((f) => f.status === '执行中').length;

export const EXEC_RUN: IExecRun = {
  label: '当前运行 · CI #4821',
  branch: 'svc-payment · feature/refund-v2',
  progress: 96,
  passed: flowPassed,
  failed: flowFailed,
  queued: 0,
  running: flowRunning,
  blocked: flowBlocked,
  total: FLOW_ITEMS.length,
  eta: '预计剩余 1m 12s',
};

// ============ 上游源适配器 ============
export interface IUpstreamAdapter {
  source: string;
  system: string;
  extract: string;
  seeds: number;
  status: '已同步' | '验证中';
}

export const UPSTREAM_ADAPTERS: IUpstreamAdapter[] = [
  { source: '需求文档', system: 'Jira', extract: '需求ID、验收标准、优先级', seeds: 18, status: '已同步' },
  { source: '设计文档', system: 'Confluence', extract: '状态机、数据模型、接口设计', seeds: 6, status: '已同步' },
  { source: 'API 契约', system: 'OpenAPI / Proto', extract: '接口定义、请求/响应、错误码', seeds: 12, status: '验证中' },
  { source: '代码变更', system: 'Git PR', extract: '变更范围、影响分析', seeds: 7, status: '已同步' },
  { source: '生产追踪', system: 'APM Trace', extract: '真实用户行为路径', seeds: 3, status: '已同步' },
  { source: '缺陷报告', system: 'Jira Bug', extract: '缺陷描述、复现步骤', seeds: 2, status: '已同步' },
];

export const ADAPTER_SEED_TOTAL = UPSTREAM_ADAPTERS.reduce((s, a) => s + a.seeds, 0); // 48

export interface IGenerationStage {
  label: string;
  meta: string;
  icon: 'source' | 'adapter' | 'seed' | 'quality' | 'review' | 'storage';
}

export const GENERATION_STAGES: IGenerationStage[] = [
  { label: '上游源', meta: `${UPSTREAM_ADAPTERS.length} 类源接入`, icon: 'source' },
  { label: '适配器', meta: '格式统一', icon: 'adapter' },
  { label: '用例种子', meta: `${ADAPTER_SEED_TOTAL} 条待验证`, icon: 'seed' },
  { label: '质量验证', meta: '变异 + 断言 + 重复', icon: 'quality' },
  { label: '人工审核', meta: '32 条待审核', icon: 'review' },
  { label: '用例入库', meta: '16 条已入库', icon: 'storage' },
];

// ============ 质量门禁 ============
export interface IGateRule {
  name: string;
  detail: string;
  status: 'pass' | 'block';
  evidence?: string[];
  aiNote?: string;
}

export const GATE_RULES: IGateRule[] = [
  { name: '覆盖率阈值', detail: '变更文件覆盖率 86.4% ≥ 阈值 80%', status: 'pass' },
  { name: '断言强度检测', detail: '检测到 3 处断言弱化行为', status: 'block', evidence: ['TC-2024-118:42 → assertTrue(true) 替代金额校验', 'TC-2024-118:67 → 断言被 try/except 包裹', 'TC-2024-095:31 → @pytest.mark.skip 新增'] },
  { name: '契约门禁 · CT-003', detail: '/v2/refund 存在破坏性变更，2 个消费者契约测试未更新', status: 'block' },
  { name: '变异测试分数', detail: 'TC-2024-118 变异分数 31% < 阈值 70%', status: 'block' },
  { name: '跨服务追溯完整性', detail: '涉及 4 个资产节点的用例全部绑定需求', status: 'pass' },
  { name: '原始测试回放', detail: '基线 42 个用例在产出代码上全部通过', status: 'pass' },
];

export const GATE_TOTAL = GATE_RULES.length;
export const GATE_PASS = GATE_RULES.filter((r) => r.status === 'pass').length;
export const GATE_BLOCK = GATE_RULES.filter((r) => r.status === 'block').length;

// ============ 审计日志 ============
export interface IAuditLog {
  seq: string;
  time: string;
  type: '门禁阻断' | '契约告警' | '篡改检测' | '种子生成' | '冲突检测' | '执行通过';
  message: string;
  actor: string;
  asset: string;
  hash: string;
}

export const AUDIT_LOGS: IAuditLog[] = [
  { seq: '#12847', time: '2026-09-26 10:30:12', type: '门禁阻断', message: 'CI #4821 门禁判定为阻断 · 4 项规则未通过', actor: 'gate-engine', asset: 'svc-payment', hash: 'sha256:0a1f…' },
  { seq: '#12846', time: '2026-09-26 10:28:00', type: '契约告警', message: 'CT-003 /v2/refund 破坏性变更 · 影响 3 消费者', actor: 'contract-registry', asset: 'svc-payment', hash: 'sha256:7b2c…' },
  { seq: '#12845', time: '2026-09-26 10:27:30', type: '篡改检测', message: '检测到 TC-2024-118 断言弱化 · assertTrue(true)', actor: 'tamper-detector', asset: 'svc-payment', hash: 'sha256:9c3e…' },
  { seq: '#12844', time: '2026-09-26 10:26:12', type: '种子生成', message: '来自 OpenAPI:/v2/refund 生成 12 条用例种子', actor: 'generation-pipeline', asset: 'svc-payment', hash: 'sha256:6b2d…' },
  { seq: '#12843', time: '2026-09-26 10:25:48', type: '冲突检测', message: 'TC-2024-095 并发修改冲突 · 已隔离待协调', actor: 'conflict-coordinator', asset: 'svc-payment', hash: 'sha256:3e7a…' },
  { seq: '#12842', time: '2026-09-26 10:25:12', type: '执行通过', message: 'TC-2024-001 执行通过 · 证据已锚定', actor: 'executor-07', asset: 'svc-auth', hash: 'sha256:b1a7…' },
];

// ============ 并发与资源 ============
export interface IConcurrencyRow {
  name: string;
  type: '服务' | '端';
  running: number;
  queued: number;
  quota: number;
  usage: number;
}

export const CONCURRENCY_ROWS: IConcurrencyRow[] = [
  { name: 'svc-auth', type: '服务', running: 18, queued: 4, quota: 30, usage: 60 },
  { name: 'svc-payment', type: '服务', running: 24, queued: 8, quota: 30, usage: 80 },
  { name: 'svc-order', type: '服务', running: 12, queued: 2, quota: 20, usage: 60 },
  { name: 'web-frontend', type: '端', running: 22, queued: 6, quota: 40, usage: 55 },
  { name: 'mobile-ios', type: '端', running: 8, queued: 12, quota: 20, usage: 60 },
  { name: 'mobile-android', type: '端', running: 10, queued: 20, quota: 20, usage: 50 },
];

export interface IConflictEvent {
  level: 'danger' | 'warning' | 'info';
  title: string;
  desc: string;
  meta: string;
}

export const CONFLICT_EVENTS: IConflictEvent[] = [
  { level: 'danger', title: '用例写-写冲突', desc: 'TC-2024-095 被两个 Agent 同时修改', meta: '10:26:12 · 已隔离待协调' },
  { level: 'warning', title: '跨资产资源抢占', desc: 'svc-payment 抢占 mobile-android 的 K8s 配额', meta: '10:22:45 · 被抢占任务已重新排队' },
  { level: 'info', title: '契约验证并发冲突', desc: 'CT-003 两个消费者同时验证，已串行化', meta: '10:18:30 · 无影响' },
];

export interface IResourcePool {
  name: string;
  used: number;
  total: number;
  level: 'success' | 'warning' | 'danger';
  note: string;
}

export const RESOURCE_POOLS: IResourcePool[] = [
  { name: 'Android 真机池', used: 8, total: 10, level: 'warning', note: '排队 3 · 平均等待 2m 14s' },
  { name: 'iOS 真机池', used: 4, total: 8, level: 'success', note: '排队 0 · 平均等待 0s' },
  { name: '浏览器实例', used: 42, total: 100, level: 'success', note: '排队 0 · 平均等待 0s' },
  { name: 'K8s 执行沙箱', used: 188, total: 200, level: 'danger', note: '排队 52 · 平均等待 4m 38s' },
];

// ============ 导航（闭环流程，非平铺视图）============
export interface INavItem { path: string; label: string; badge?: string; children?: { path: string; label: string }[] }
export const NAV_GROUPS: { title: string; items: INavItem[] }[] = [
  {
    title: '测试闭环',
    items: [
      { path: '/target', label: '被测对象画像' },
      { path: '/generation', label: '上游源与生成' },
      { path: '/cases', label: '测试用例库' },
      { path: '/exec', label: '测试执行' },
      { path: '/contracts', label: '契约测试', badge: '1 告警' },
      { path: '/gate', label: '质量门禁' },
      { path: '/history', label: '测试历史' },
    ],
  },
  {
    title: '可信与审计',
    items: [
      { path: '/audit', label: '审计管理', children: [
        { path: '/audit', label: '审计总览' },
        { path: '/audit-cost', label: '成本审计' },
        { path: '/audit-exec', label: '执行审计' },
        { path: '/audit-op', label: '操作审计' },
      ]},
      { path: '/trace', label: '需求追溯' },
      { path: '/concurrency', label: '并发与资源' },
    ],
  },
  {
    title: '组织与模型',
    items: [
      { path: '/teams', label: '团队与权限' },
      { path: '/models', label: 'AI 模型配置' },
    ],
  },
];

// ============ 派生仪表盘指标（全部从明细计算）============
export interface IDashboardMetric {
  label: string;
  value: number;
  suffix: string;
  color: 'primary' | 'success' | 'warning';
  note: string;
}

function pct(part: number, total: number): number {
  return total === 0 ? 0 : Math.round((part / total) * 1000) / 10;
}

export const METRICS: IDashboardMetric[] = (() => {
  const totalReq = REQUIREMENTS.length;
  const coveredReq = REQUIREMENTS.filter((r) => r.traceability !== '缺口').length;
  const fullTrace = REQUIREMENTS.filter((r) => r.traceability === '完整').length;

  const mutations = CASES.filter((c) => c.mutation !== null).map((c) => c.mutation as number);
  const effective = mutations.filter((m) => m >= 70).length;

  const contractPass = CONTRACTS.filter((c) => c.status === 'pass').length;
  const contractTotal = CONTRACTS.length;

  return [
    { label: '需求覆盖率', value: pct(coveredReq, totalReq), suffix: '%', color: coveredReq === totalReq ? 'success' : 'warning', note: `${coveredReq} / ${totalReq} 需求已覆盖` },
    { label: '用例有效率', value: pct(effective, mutations.length), suffix: '%', color: 'primary', note: '变异测试验证通过率' },
    { label: '门禁通过率', value: pct(GATE_PASS, GATE_TOTAL), suffix: '%', color: GATE_BLOCK > 0 ? 'warning' : 'success', note: `通过 ${GATE_PASS} · 阻断 ${GATE_BLOCK}` },
    { label: '契约测试通过率', value: pct(contractPass, contractTotal), suffix: '%', color: contractPass === contractTotal ? 'success' : 'warning', note: `${contractPass} / ${contractTotal} 契约通过` },
    { label: '追溯完整率', value: pct(fullTrace, totalReq), suffix: '%', color: fullTrace === totalReq ? 'success' : 'warning', note: `${fullTrace} / ${totalReq} 需求完整追溯` },
  ];
})();


// ============ 组织与多租户（团队切分 + 成员管理）============
export interface ITeam {
  id: string;
  name: string;
  desc: string;
  plan: string;
  memberCount: number;
  projectCount: number;
  createdAt: string;
  color: 'emerald' | 'indigo' | 'amber';
}

export const TEAMS: ITeam[] = [
  { id: 'team-1', name: '支付中台研发组', desc: '负责支付域微服务的研发与质量治理', plan: '企业版 · 50 席位', memberCount: 28, projectCount: 6, createdAt: '2025-03-12', color: 'emerald' },
  { id: 'team-2', name: '移动端产品组', desc: 'iOS / Android 客户端测试与发布管控', plan: '企业版 · 30 席位', memberCount: 15, projectCount: 4, createdAt: '2025-06-08', color: 'indigo' },
  { id: 'team-3', name: '基础架构 QA 组', desc: '平台基建与 AI 质量治理专项团队', plan: '专业版 · 20 席位', memberCount: 9, projectCount: 3, createdAt: '2025-09-01', color: 'amber' },
];

export const CURRENT_TEAM_ID = 'team-1';

export type Role = 'owner' | 'admin' | 'tester' | 'viewer';

export const ROLE_META: { role: Role; label: string; desc: string }[] = [
  { role: 'owner', label: '团队所有者', desc: '管理团队、席位与账单，最高权限' },
  { role: 'admin', label: '管理员', desc: '管理成员、工程、模型与门禁策略' },
  { role: 'tester', label: '测试工程师', desc: '创建/执行用例、查看判定并处理风险' },
  { role: 'viewer', label: '只读成员', desc: '仅查看结果与审计，不可变更' },
];

// 权限矩阵：行=能力点，列=角色（✓ 允许 / ✕ 拒绝）
export const ROLE_MATRIX: { capability: string; owner: boolean; admin: boolean; tester: boolean; viewer: boolean }[] = [
  { capability: '团队与席位管理', owner: true, admin: false, tester: false, viewer: false },
  { capability: '成员与角色管理', owner: true, admin: true, tester: false, viewer: false },
  { capability: '工程 / 模型配置', owner: true, admin: true, tester: false, viewer: false },
  { capability: '门禁策略编辑', owner: true, admin: true, tester: false, viewer: false },
  { capability: '用例创建与执行', owner: true, admin: true, tester: true, viewer: false },
  { capability: '风险处理 / 重新判定', owner: true, admin: true, tester: true, viewer: false },
  { capability: '查看结果与审计', owner: true, admin: true, tester: true, viewer: true },
];

export interface ITeamMember {
  id: string;
  name: string;
  email: string;
  role: Role;
  status: 'active' | 'invited' | 'disabled';
  lastActive: string;
}

export const TEAM_MEMBERS: ITeamMember[] = [
  { id: 'u-1', name: '张立', email: 'zhang.li@corp.com', role: 'owner', status: 'active', lastActive: '刚刚' },
  { id: 'u-2', name: '李娜', email: 'li.na@corp.com', role: 'admin', status: 'active', lastActive: '5 分钟前' },
  { id: 'u-3', name: '王强', email: 'wang.qiang@corp.com', role: 'tester', status: 'active', lastActive: '12 分钟前' },
  { id: 'u-4', name: '陈晨', email: 'chen.chen@corp.com', role: 'tester', status: 'active', lastActive: '1 小时前' },
  { id: 'u-5', name: '赵敏', email: 'zhao.min@corp.com', role: 'viewer', status: 'active', lastActive: '昨天' },
  { id: 'u-6', name: '刘洋', email: 'liu.yang@corp.com', role: 'tester', status: 'invited', lastActive: '待接受邀请' },
  { id: 'u-7', name: '孙浩', email: 'sun.hao@corp.com', role: 'viewer', status: 'disabled', lastActive: '10 天前' },
];

// ============ AI 模型池 + 每工程模型绑定 ============
export interface IAIModel {
  id: string;
  name: string;
  vendor: string;
  capability: string;
  cost: string;
  latency: string;
  isDefault: boolean;
  tags: string[];
}

export const AI_MODELS: IAIModel[] = [
  { id: 'doubao-1.5-pro', name: '豆包 1.5 Pro', vendor: '字节跳动', capability: '代码生成 / 用例 / 判定', cost: '中', latency: '快', isDefault: true, tags: ['中文最优', '企业级合规'] },
  { id: 'deepseek-v3', name: 'DeepSeek-V3', vendor: '深度求索', capability: '代码生成 / 推理判定', cost: '低', latency: '中', isDefault: false, tags: ['高性价比'] },
  { id: 'gpt-4o', name: 'GPT-4o', vendor: 'OpenAI', capability: '多模态 / 复杂判定', cost: '高', latency: '中', isDefault: false, tags: ['生态成熟'] },
  { id: 'claude-3.5', name: 'Claude 3.5 Sonnet', vendor: 'Anthropic', capability: '代码理解 / 契约分析', cost: '高', latency: '中', isDefault: false, tags: ['长上下文'] },
  { id: 'self-llama3', name: '自研 Llama3 微调', vendor: '内部私有化', capability: '契约 / 变异验证', cost: '内部', latency: '中', isDefault: false, tags: ['数据不出域'] },
];

// 每个工程（被测对象）绑定一个测试所用 AI 模型
export interface IProjectModel {
  projectId: string;
  projectName: string;
  modelId: string;
  coverage: number;
}

export const PROJECT_MODELS: IProjectModel[] = [
  { projectId: 'sys-payment-platform', projectName: 'sys-payment-platform', modelId: 'doubao-1.5-pro', coverage: 65 },
  { projectId: 'app-user-center', projectName: 'app-user-center', modelId: 'deepseek-v3', coverage: 72 },
  { projectId: 'web-admin-console', projectName: 'web-admin-console', modelId: 'gpt-4o', coverage: 68 },
  { projectId: 'mobile-app', projectName: 'mobile-app', modelId: 'claude-3.5', coverage: 61 },
  { projectId: 'risk-engine', projectName: 'risk-engine', modelId: 'self-llama3', coverage: 74 },
];


// ============ 资产联动画像：按所选资产实时派生（数据自洽）============
export interface IAssetProfile {
  name: string;
  qualityScore: number;
  passRate: number;
  stage: string;
  riskCount: number;
  dims: { label: string; score: number; weight: number; note: string }[];
  gateRules: { name: string; detail: string; status: 'pass' | 'block'; evidence?: string[] }[];
}

// 依据资产的 coverage / gateRate 派生画像与门禁，保证加权评分、阻断数与明细自洽
export function assetToProfile(asset: IAsset): IAssetProfile {
  const C = asset.coverage;
  const G = asset.gateRate;
  // 加权评分 = 0.25*C + 0.3*G + 0.2*85 + 0.15*max(0,C-8) + 0.1*C
  const qualityScore = Math.round(0.25 * C + 0.3 * G + 0.2 * 85 + 0.15 * Math.max(0, C - 8) + 0.1 * C);

  const gateRules: IAssetProfile['gateRules'] = [
    { name: '覆盖率阈值', detail: `变更文件覆盖率 ${C}% ${C >= 85 ? '≥' : '<'} 阈值 80%`, status: C >= 85 ? 'pass' : 'block' },
    { name: '断言强度检测', detail: G >= 85 ? '未发现断言弱化行为' : '检测到断言弱化行为，需人工复核', status: G >= 85 ? 'pass' : 'block', evidence: G >= 85 ? undefined : ['TC-2024-118:42 → assertTrue(true) 替代金额校验'] },
    { name: '契约门禁 · CT-003', detail: G >= 85 ? '/v2/refund 契约变更已兼容验证' : '/v2/refund 存在破坏性变更，消费者契约测试未更新', status: G >= 85 ? 'pass' : 'block' },
    { name: '变异测试分数', detail: `${C}% ${C >= 75 ? '≥' : '<'} 阈值 70%`, status: C >= 75 ? 'pass' : 'block' },
    { name: '跨服务追溯完整性', detail: '用例全部绑定上游需求，无孤儿用例', status: 'pass' },
    { name: '原始测试回放', detail: C >= 70 ? '基线用例在产出代码上全部通过' : '基线用例存在未通过项', status: C >= 70 ? 'pass' : 'block' },
  ];
  const riskCount = gateRules.filter((r) => r.status === 'block').length;

  const dims = [
    { label: '需求覆盖率', score: C, weight: 0.25, note: `${Math.round((C / 100) * 4 * 10) / 10} / 4 需求已覆盖` },
    { label: '门禁通过', score: G, weight: 0.3, note: `${Math.round((G / 100) * 6)} / 6 规则通过` },
    { label: '契约健康', score: 85, weight: 0.2, note: '4/5 契约通过' },
    { label: '用例有效', score: Math.max(0, C - 8), weight: 0.15, note: '变异验证通过率' },
    { label: '追溯完整', score: C, weight: 0.1, note: `${C >= 80 ? '4' : C >= 60 ? '3' : '2'} / 4 需求完整追溯` },
  ];

  return {
    name: asset.name,
    qualityScore,
    passRate: G,
    stage: riskCount > 0 ? '门禁' : '执行',
    riskCount,
    dims,
    gateRules,
  };
}


// ============ 资产权限分配：工程负责人持完整权限，可对成员精细分配权限与资源 ============
export type AssetPerm = 'full' | 'edit' | 'exec' | 'view' | 'none';

export const ASSET_PERM_META: { perm: AssetPerm; label: string; desc: string; color: string }[] = [
  { perm: 'full', label: '完整', desc: '等同于负责人，可再分配权限与资源', color: 'text-purple-600 bg-purple-50' },
  { perm: 'edit', label: '编辑', desc: '创建/修改用例与门禁判定', color: 'text-emerald-600 bg-emerald-50' },
  { perm: 'exec', label: '执行', desc: '运行用例、查看执行证据', color: 'text-indigo-600 bg-indigo-50' },
  { perm: 'view', label: '只读', desc: '仅查看该资产结果与审计', color: 'text-slate-500 bg-slate-100' },
  { perm: 'none', label: '无权限', desc: '对该资产不可见、不可操作', color: 'text-red-500 bg-red-50' },
];

export interface IAssetMemberAlloc {
  memberId: string;
  perm: AssetPerm;
  quota: number;    // 并发执行配额（条）
  sandbox: number;  // K8s 沙箱配额（个）
}

export interface IAssetAccess {
  assetId: string;
  assetName: string;
  type: string;
  ownerId: string;               // 工程负责人（完整权限）
  members: IAssetMemberAlloc[];  // 成员分配（负责人之外的成员）
}

export const ASSET_ACCESS: IAssetAccess[] = [
  {
    assetId: 'svc-payment', assetName: 'svc-payment', type: '服务', ownerId: 'u-1',
    members: [
      { memberId: 'u-2', perm: 'edit', quota: 6, sandbox: 3 },
      { memberId: 'u-3', perm: 'exec', quota: 4, sandbox: 2 },
      { memberId: 'u-4', perm: 'exec', quota: 2, sandbox: 1 },
      { memberId: 'u-5', perm: 'view', quota: 0, sandbox: 0 },
    ],
  },
  {
    assetId: 'svc-auth', assetName: 'svc-auth', type: '服务', ownerId: 'u-2',
    members: [
      { memberId: 'u-1', perm: 'full', quota: 8, sandbox: 4 },
      { memberId: 'u-3', perm: 'edit', quota: 4, sandbox: 2 },
      { memberId: 'u-5', perm: 'view', quota: 0, sandbox: 0 },
    ],
  },
  {
    assetId: 'svc-order', assetName: 'svc-order', type: '服务', ownerId: 'u-3',
    members: [
      { memberId: 'u-2', perm: 'edit', quota: 5, sandbox: 2 },
      { memberId: 'u-4', perm: 'exec', quota: 3, sandbox: 1 },
    ],
  },
  {
    assetId: 'web-frontend', assetName: 'web-frontend', type: '应用', ownerId: 'u-4',
    members: [
      { memberId: 'u-3', perm: 'edit', quota: 6, sandbox: 3 },
      { memberId: 'u-5', perm: 'view', quota: 0, sandbox: 0 },
    ],
  },
  {
    assetId: 'mobile-ios', assetName: 'mobile-ios', type: '端', ownerId: 'u-2',
    members: [
      { memberId: 'u-4', perm: 'exec', quota: 2, sandbox: 1 },
    ],
  },
];


// ============ 成本审计：所有 AI 场景成本关联，总览/明细自洽（金额由单价表×token 派生）============
export const MODEL_PRICE: Record<string, { in: number; out: number }> = {
  'doubao-1.5-pro': { in: 1.5, out: 4.5 },
  'deepseek-v3': { in: 0.4, out: 1.2 },
  'gpt-4o': { in: 18, out: 70 },
  'claude-3.5': { in: 25, out: 100 },
  'self-llama3': { in: 1, out: 3 },
};

// 金额 = 输入token/1M × 输入单价 + 输出token/1M × 输出单价（¥，取整）
export function aiCost(modelId: string, tokensIn: number, tokensOut: number): number {
  const p = MODEL_PRICE[modelId] ?? MODEL_PRICE['doubao-1.5-pro'];
  return Math.round((tokensIn / 1e6) * p.in + (tokensOut / 1e6) * p.out);
}

export type CostKind = 'gen' | 'exec';
export interface ICostDetail {
  id: string;
  caseId: string;
  asset: string;
  source: string;       // 触发来源（上游源）
  modelId: string;
  kind: CostKind;
  isLegacy: boolean;    // 存量用例（执行成本应持平或递减）
  tokensIn: number;
  tokensOut: number;
  time: string;
}

const COST_GEN_RAW: ICostDetail[] = [
  { id: 'GEN-2041-01', caseId: 'TC-2024-118', asset: 'svc-payment', source: 'OpenAPI /v2/refund', modelId: 'doubao-1.5-pro', kind: 'gen', isLegacy: false, tokensIn: 40000000, tokensOut: 10000000, time: '09-24 10:26' },
  { id: 'GEN-2041-02', caseId: 'TC-2024-095', asset: 'svc-payment', source: 'OpenAPI + 缺陷报告', modelId: 'doubao-1.5-pro', kind: 'gen', isLegacy: false, tokensIn: 60000000, tokensOut: 15000000, time: '09-23 18:42' },
  { id: 'GEN-2041-03', caseId: 'TC-2024-130', asset: 'svc-auth', source: 'ASVS L2 安全清单', modelId: 'doubao-1.5-pro', kind: 'gen', isLegacy: false, tokensIn: 80000000, tokensOut: 25000000, time: '09-22 11:20' },
  { id: 'GEN-2041-04', caseId: 'TC-2024-131', asset: 'svc-auth', source: '生产追踪路径', modelId: 'doubao-1.5-pro', kind: 'gen', isLegacy: false, tokensIn: 100000000, tokensOut: 46000000, time: '09-21 15:05' },
];

const COST_EXEC_RAW: ICostDetail[] = [
  { id: 'EXEC-4821-01', caseId: 'TC-2024-001', asset: 'svc-auth', source: '存量回放', modelId: 'deepseek-v3', kind: 'exec', isLegacy: true, tokensIn: 8000000, tokensOut: 500000, time: '09-24 10:25' },
  { id: 'EXEC-4821-02', caseId: 'TC-2024-005', asset: 'svc-user', source: '存量回放', modelId: 'deepseek-v3', kind: 'exec', isLegacy: true, tokensIn: 6000000, tokensOut: 300000, time: '09-24 10:25' },
  { id: 'EXEC-4821-03', caseId: 'TC-2024-006', asset: 'web-frontend', source: '存量回放', modelId: 'deepseek-v3', kind: 'exec', isLegacy: true, tokensIn: 10000000, tokensOut: 800000, time: '09-24 10:26' },
  { id: 'EXEC-4821-04', caseId: 'TC-2024-125', asset: 'mobile-ios', source: '存量回放', modelId: 'deepseek-v3', kind: 'exec', isLegacy: true, tokensIn: 12000000, tokensOut: 600000, time: '09-24 10:27' },
  { id: 'EXEC-4821-05', caseId: 'TC-2024-095', asset: 'svc-payment', source: '新增用例', modelId: 'deepseek-v3', kind: 'exec', isLegacy: false, tokensIn: 40000000, tokensOut: 2000000, time: '09-24 10:28' },
  { id: 'EXEC-4821-06', caseId: 'TC-2024-118', asset: 'svc-payment', source: '新增用例', modelId: 'deepseek-v3', kind: 'exec', isLegacy: false, tokensIn: 50000000, tokensOut: 3000000, time: '09-24 10:29' },
];

export const COST_GEN_DETAIL = COST_GEN_RAW.map((d) => ({ ...d, amount: aiCost(d.modelId, d.tokensIn, d.tokensOut) }));
export const COST_EXEC_DETAIL = COST_EXEC_RAW.map((d) => ({ ...d, amount: aiCost(d.modelId, d.tokensIn, d.tokensOut) }));

export const COST_GEN_TOTAL = COST_GEN_DETAIL.reduce((s, d) => s + d.amount, 0);
export const COST_EXEC_TOTAL = COST_EXEC_DETAIL.reduce((s, d) => s + d.amount, 0);
export const COST_EXEC_LEGACY_TOTAL = COST_EXEC_DETAIL.filter((d) => d.isLegacy).reduce((s, d) => s + d.amount, 0);
export const COST_EXEC_NEW_TOTAL = COST_EXEC_DETAIL.filter((d) => !d.isLegacy).reduce((s, d) => s + d.amount, 0);

// 场景聚合（生成/执行来自明细和，其余为独立场景）→ 总览 = 场景之和，自洽
export interface ICostScenario { scenario: string; modelId: string; amount: number; note: string; tokensIn: number; tokensOut: number }
export const COST_SCENARIOS: ICostScenario[] = [
  { scenario: '用例生成', modelId: 'doubao-1.5-pro', amount: COST_GEN_TOTAL, note: 'AI 生成用例种子 + 质量验证', tokensIn: COST_GEN_RAW.reduce((s, d) => s + d.tokensIn, 0), tokensOut: COST_GEN_RAW.reduce((s, d) => s + d.tokensOut, 0) },
  { scenario: '测试执行辅助', modelId: 'deepseek-v3', amount: COST_EXEC_TOTAL, note: '存量+新增用例执行的 AI 分析', tokensIn: COST_EXEC_RAW.reduce((s, d) => s + d.tokensIn, 0), tokensOut: COST_EXEC_RAW.reduce((s, d) => s + d.tokensOut, 0) },
  { scenario: '质量判定', modelId: 'gpt-4o', amount: aiCost('gpt-4o', 60000000, 24000000), note: '门禁重判 / 断言强度分析', tokensIn: 60000000, tokensOut: 24000000 },
  { scenario: '契约分析', modelId: 'claude-3.5', amount: aiCost('claude-3.5', 18000000, 4000000), note: 'OpenAPI 变更 diff / 消费者影响', tokensIn: 18000000, tokensOut: 4000000 },
  { scenario: '变异/篡改检测', modelId: 'self-llama3', amount: aiCost('self-llama3', 40000000, 12000000), note: '变体注入与篡改比对', tokensIn: 40000000, tokensOut: 12000000 },
];

export const COST_TOTAL = COST_SCENARIOS.reduce((s, c) => s + c.amount, 0);
export const COST_TOTAL_TOKENS_IN = COST_SCENARIOS.reduce((s, c) => s + c.tokensIn, 0);
export const COST_TOTAL_TOKENS_OUT = COST_SCENARIOS.reduce((s, c) => s + c.tokensOut, 0);
export const COST_AICALLS = 3842;
export const COST_MOM_CHANGE = -18; // 环比下降 %
export const COST_PER_CASE = 1.2;   // 平均每用例成本（¥）

// 近 14 天成本趋势：total 波动；exec（存量执行成本）持平或递减
export const COST_TREND: { day: string; total: number; exec: number }[] = [
  { day: '09-11', total: 34, exec: 9.8 },
  { day: '09-12', total: 32, exec: 9.6 },
  { day: '09-13', total: 38, exec: 9.5 },
  { day: '09-14', total: 35, exec: 9.2 },
  { day: '09-15', total: 30, exec: 9.0 },
  { day: '09-16', total: 41, exec: 8.8 },
  { day: '09-17', total: 36, exec: 8.6 },
  { day: '09-18', total: 33, exec: 8.3 },
  { day: '09-19', total: 29, exec: 8.1 },
  { day: '09-20', total: 31, exec: 7.8 },
  { day: '09-21', total: 28, exec: 7.4 },
  { day: '09-22', total: 27, exec: 7.0 },
  { day: '09-23', total: 26, exec: 6.4 },
  { day: '09-24', total: 24, exec: 5.6 },
];


// ============ 审计管理：成本维度归因 + 执行审计 + 操作审计 ============
// 服务 → 研发组归属（成本归因口径）
export const SERVICE_ORG_MAP: Record<string, string> = {
  'svc-auth': 'AI支付中台', 'svc-user': '支付核心', 'svc-payment': 'AI支付中台',
  'svc-order': '支付核心', 'web-frontend': 'AI支付中台', 'mobile-ios': '前端与端', 'mobile-android': '前端与端',
};

// 成本按研发组归因（合计 = COST_TOTAL = 4599，与明细自洽）
export interface IOrgCost { id: string; name: string; amount: number }
export const COST_ORGS: IOrgCost[] = [
  { id: 'org-pay-mid', name: 'AI支付中台', amount: 3000 },
  { id: 'org-pay-core', name: '支付核心', amount: 1300 },
  { id: 'org-front', name: '前端与端', amount: 299 },
];

// 成本按工程/服务组归因
export const COST_BY_GROUP: { name: string; amount: number }[] = [
  { name: '身份服务组', amount: 1800 },
  { name: '支付服务组', amount: 2100 },
  { name: '前端与端', amount: 699 },
];

// 成本按服务归因（合计 4599）
export const COST_BY_SERVICE: { service: string; name: string; amount: number }[] = [
  { service: 'svc-auth', name: '认证服务', amount: 1100 },
  { service: 'svc-user', name: '用户服务', amount: 700 },
  { service: 'svc-payment', name: '支付服务', amount: 1500 },
  { service: 'svc-order', name: '订单服务', amount: 600 },
  { service: 'web-frontend', name: 'Web 前端', amount: 400 },
  { service: 'mobile-ios', name: 'iOS 端', amount: 180 },
  { service: 'mobile-android', name: 'Android 端', amount: 119 },
];

// 维度 × 场景 热力矩阵（行和 = 该服务成本；列和 = 4599）
export interface ICostHeatCell { 生成: number; 执行: number; 判定: number; 契约: number; 变异: number }
export const COST_HEAT: { serviceId: string; org: string; cells: ICostHeatCell }[] = [
  { serviceId: 'svc-auth', org: 'AI支付中台', cells: { 生成: 400, 执行: 80, 判定: 500, 契约: 40, 变异: 80 } },
  { serviceId: 'svc-user', org: '支付核心', cells: { 生成: 200, 执行: 40, 判定: 350, 契约: 30, 变异: 80 } },
  { serviceId: 'svc-payment', org: 'AI支付中台', cells: { 生成: 600, 执行: 120, 判定: 600, 契约: 100, 变异: 80 } },
  { serviceId: 'svc-order', org: '支付核心', cells: { 生成: 200, 执行: 40, 判定: 250, 契约: 30, 变异: 80 } },
  { serviceId: 'web-frontend', org: 'AI支付中台', cells: { 生成: 150, 执行: 30, 判定: 150, 契约: 30, 变异: 40 } },
  { serviceId: 'mobile-ios', org: '前端与端', cells: { 生成: 60, 执行: 15, 判定: 70, 契约: 20, 变异: 15 } },
  { serviceId: 'mobile-android', org: '前端与端', cells: { 生成: 40, 执行: 10, 判定: 45, 契约: 9, 变异: 15 } },
];

// 执行审计：用例执行记录（用例 · 结果 · 耗时）
export interface IExecAuditRow { caseId: string; asset: string; result: '通过' | '失败' | '阻塞'; duration: string; model: string; ts: string }
export const EXEC_AUDIT_ROWS: IExecAuditRow[] = [
  { caseId: 'TC-2024-001', asset: 'svc-auth', result: '通过', duration: '0.8s', model: 'DeepSeek-V3', ts: '09-24 10:25' },
  { caseId: 'TC-2024-005', asset: 'svc-user', result: '通过', duration: '1.2s', model: 'DeepSeek-V3', ts: '09-24 10:25' },
  { caseId: 'TC-2024-006', asset: 'web-frontend', result: '通过', duration: '5.6s', model: 'DeepSeek-V3', ts: '09-24 10:26' },
  { caseId: 'TC-2024-125', asset: 'mobile-ios', result: '通过', duration: '8.2s', model: 'DeepSeek-V3', ts: '09-24 10:27' },
  { caseId: 'TC-2024-095', asset: 'svc-payment', result: '失败', duration: '3.1s', model: 'DeepSeek-V3', ts: '09-24 10:28' },
  { caseId: 'TC-2024-118', asset: 'svc-payment', result: '失败', duration: '2.3s', model: 'DeepSeek-V3', ts: '09-24 10:29' },
  { caseId: 'TC-2024-131', asset: 'svc-auth', result: '阻塞', duration: '—', model: 'DeepSeek-V3', ts: '09-24 10:30' },
];

// 操作审计：谁 · 何时 · 做了什么
export interface IOpAuditRow { user: string; role: string; action: string; target: string; result: string; risk: '低' | '中' | '高'; ts: string }
export const OP_AUDIT_ROWS: IOpAuditRow[] = [
  { user: '张立', role: '工程负责人', action: '重新判定', target: '质量门禁 CI #4821', result: '通过', risk: '低', ts: '09-24 10:31' },
  { user: 'ai-agent-3', role: 'AI Agent', action: '生成用例种子', target: 'GEN-2041 · OpenAPI', result: '已入库 16 条', risk: '低', ts: '09-24 10:26' },
  { user: '李娜', role: '管理员', action: '修改契约', target: 'CT-003 /v2/refund', result: '校验中', risk: '高', ts: '09-24 10:28' },
  { user: '王强', role: '测试工程师', action: '触发执行', target: '测试执行 CI #4821', result: '运行中', risk: '中', ts: '09-24 10:22' },
  { user: '系统', role: 'tamper-detector', action: '篡改检测', target: 'TC-2024-118', result: '发现弱断言 3 处', risk: '高', ts: '09-24 10:27' },
  { user: '张立', role: '工程负责人', action: '导出审计报告', target: '审计日志', result: '已完成', risk: '低', ts: '09-24 10:20' },
];


// ============ 测试历史 · 测试证据 · 用例成本历史 · 服务报告 ============
// 测试运行（时间维主键）
export interface ITestRun { id: string; ts: string; branch: string; trigger: string; pass: number; fail: number; block: number; queue: number; total: number; duration: string; cost: number; gate: '通过' | '阻断' }
export const TEST_RUNS: ITestRun[] = [
  { id: 'RUN-4821', ts: '09-24 10:28', branch: 'feature/refund-v2', trigger: 'CI 提交', pass: 1192, fail: 34, block: 10, queue: 52, total: 1300, duration: '4m 22s', cost: 4599, gate: '阻断' },
  { id: 'RUN-4805', ts: '09-23 16:40', branch: 'feature/refund-v2', trigger: 'CI 提交', pass: 1262, fail: 8, block: 6, queue: 24, total: 1300, duration: '3m 58s', cost: 4210, gate: '通过' },
  { id: 'RUN-4792', ts: '09-22 11:12', branch: 'main', trigger: '定时', pass: 1278, fail: 2, block: 0, queue: 20, total: 1300, duration: '3m 41s', cost: 4056, gate: '通过' },
  { id: 'RUN-4788', ts: '09-21 09:05', branch: 'feature/trace-v2', trigger: 'CI 提交', pass: 1201, fail: 48, block: 12, queue: 39, total: 1300, duration: '4m 40s', cost: 4721, gate: '阻断' },
  { id: 'RUN-4776', ts: '09-20 18:30', branch: 'main', trigger: '定时', pass: 1279, fail: 1, block: 1, queue: 19, total: 1300, duration: '3m 36s', cost: 3980, gate: '通过' },
  { id: 'RUN-4769', ts: '09-19 14:22', branch: 'main', trigger: '手动', pass: 1268, fail: 9, block: 2, queue: 21, total: 1300, duration: '3m 50s', cost: 4145, gate: '通过' },
];

// RUN-4821 服务级报告（工程报告的子集）
export interface IServiceReport { service: string; name: string; pass: number; fail: number; block: number; total: number; coverage: number; cost: number; risk: '低' | '中' | '高'; risks: string[]; conclusion: string }
export const RUN_SERVICE_REPORTS: IServiceReport[] = [
  { service: 'svc-auth', name: '认证服务', pass: 96, fail: 1, block: 1, total: 98, coverage: 94, cost: 1100, risk: '低', risks: [], conclusion: '认证与 Token 链路稳定，通过' },
  { service: 'svc-user', name: '用户服务', pass: 62, fail: 1, block: 0, total: 63, coverage: 91, cost: 700, risk: '低', risks: [], conclusion: '用户数据查询链路稳定，通过' },
  { service: 'svc-payment', name: '支付服务', pass: 148, fail: 18, block: 5, total: 171, coverage: 73, cost: 1500, risk: '高', risks: ['退款金额计算错误', '支付回调幂等性冲突', '断言被弱化'], conclusion: '阻断：/v2/refund 存在破坏性变更且 3 处断言弱化' },
  { service: 'svc-order', name: '订单服务', pass: 89, fail: 2, block: 1, total: 92, coverage: 88, cost: 600, risk: '中', risks: ['订单状态流转边界缺失用例'], conclusion: '建议补充状态机边界用例' },
  { service: 'web-frontend', name: 'Web 前端', pass: 34, fail: 3, block: 1, total: 38, coverage: 89, cost: 400, risk: '中', risks: ['登录页 E2E 偶发超时'], conclusion: '建议增大 E2E 超时阈值' },
  { service: 'mobile-ios', name: 'iOS 端', pass: 21, fail: 4, block: 1, total: 26, coverage: 76, cost: 180, risk: '中', risks: ['真机登录回归失败'], conclusion: '需在真机池复跑验证' },
  { service: 'mobile-android', name: 'Android 端', pass: 19, fail: 5, block: 1, total: 25, coverage: 74, cost: 119, risk: '中', risks: ['K8s 沙箱资源抢占'], conclusion: '资源竞争导致排队延迟' },
];

// 单用例成本历史（同一用例多次 run 的成本，用于对比）
export interface ICaseCostPoint { run: string; ts: string; result: '通过' | '失败' | '阻塞'; cost: number; kind: '存量回放' | '新增分析' }
export const CASE_COST_HISTORY: Record<string, ICaseCostPoint[]> = {
  'TC-2024-118': [
    { run: 'RUN-4769', ts: '09-19', result: '通过', cost: 1.9, kind: '存量回放' },
    { run: 'RUN-4776', ts: '09-20', result: '通过', cost: 1.8, kind: '存量回放' },
    { run: 'RUN-4792', ts: '09-22', result: '通过', cost: 1.7, kind: '存量回放' },
    { run: 'RUN-4805', ts: '09-23', result: '通过', cost: 1.6, kind: '存量回放' },
    { run: 'RUN-4821', ts: '09-24', result: '失败', cost: 2.3, kind: '新增分析' },
  ],
  'TC-2024-095': [
    { run: 'RUN-4805', ts: '09-23', result: '通过', cost: 1.4, kind: '存量回放' },
    { run: 'RUN-4821', ts: '09-24', result: '失败', cost: 2.1, kind: '新增分析' },
  ],
  'TC-2024-001': [
    { run: 'RUN-4769', ts: '09-19', result: '通过', cost: 0.9, kind: '存量回放' },
    { run: 'RUN-4776', ts: '09-20', result: '通过', cost: 0.9, kind: '存量回放' },
    { run: 'RUN-4792', ts: '09-22', result: '通过', cost: 0.8, kind: '存量回放' },
    { run: 'RUN-4805', ts: '09-23', result: '通过', cost: 0.8, kind: '存量回放' },
    { run: 'RUN-4821', ts: '09-24', result: '通过', cost: 0.8, kind: '存量回放' },
  ],
  'TC-2024-006': [
    { run: 'RUN-4792', ts: '09-22', result: '通过', cost: 1.2, kind: '存量回放' },
    { run: 'RUN-4805', ts: '09-23', result: '通过', cost: 1.2, kind: '存量回放' },
    { run: 'RUN-4821', ts: '09-24', result: '通过', cost: 1.3, kind: '新增分析' },
  ],
};

// 测试证据（用例 → 一次执行的证据集，哈希锚定）
export interface IEvidenceItem { type: '截图' | '视频' | '日志' | '请求响应'; file: string; hash: string; ts: string; note: string }
export const TEST_EVIDENCE: Record<string, IEvidenceItem[]> = {
  'TC-2024-001': [
    { type: '截图', file: 'login-flow-01.png', hash: 'sha256:a3f8…c21d', ts: '10:25:12', note: '正常登录成功页' },
    { type: '请求响应', file: 'login-req-resp.json', hash: 'sha256:b1a7…7c2e', ts: '10:25:12', note: 'POST /v2/login 200' },
    { type: '日志', file: 'svc-auth-stdout.log', hash: 'sha256:7c2e…9a31', ts: '10:25:13', note: 'Token 签发链路' },
  ],
  'TC-2024-118': [
    { type: '截图', file: 'refund-calc-02.png', hash: 'sha256:0a1f…84bd', ts: '10:29:03', note: '退款金额计算失败现场' },
    { type: '日志', file: 'svc-payment-refund.log', hash: 'sha256:6b2d…3e7a', ts: '10:29:04', note: '断言 assertTrue(true) 弱化检测' },
    { type: '请求响应', file: 'refund-api.json', hash: 'sha256:9c3e…5f21', ts: '10:29:03', note: 'POST /v2/refund 响应结构' },
  ],
  'TC-2024-095': [
    { type: '视频', file: 'idempotency-cb.mp4', hash: 'sha256:3e7a…0c91', ts: '10:28:40', note: '支付回调幂等性冲突录制' },
    { type: '日志', file: 'svc-payment-callback.log', hash: 'sha256:b1a7…6d48', ts: '10:28:41', note: '重复回调事件' },
  ],
  'TC-2024-131': [
    { type: '日志', file: 'svc-auth-concurrency.log', hash: 'sha256:5c1e…ab77', ts: '10:30:05', note: '并发登录压力测试资源排队' },
  ],
};

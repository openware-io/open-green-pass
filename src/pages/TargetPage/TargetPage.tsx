import { useState, useMemo, useEffect } from 'react';
import { Link, useNavigate, useOutletContext } from 'react-router-dom';
import { toast } from 'sonner';
import { ASSET_TREE, METRICS, GEN_TRACE, ASSET_RISKS, AI_MODELS, PROJECT_MODELS, TEST_SCENARIOS, TESTED_REPOS, CASES, COST_GEN_DETAIL, COST_EXEC_DETAIL, assetToProfile, nodeRepos, type IAsset, type IAssetNode, type ITestedRepo } from '@/data/mock';
import { scenarioNav } from '@/context/scenarioNav';
import { KpiCard, PageHeader, PrimaryButton, GhostButton, ListFilter, Card } from '@/components/shared';
import { configuredGreenPassClient } from '@/api/runtime';
import { useObjectCatalog, type CatalogService } from '@/context/objectCatalog';
import { Brain, ShieldAlert, ChevronRight, ChevronDown, Sparkles, GitBranch, CircleDot, FileSearch, History, CheckCircle2, AlertTriangle, XCircle, Layers, Cpu, Globe, Smartphone, X, Plus, ExternalLink, Check, Plug, Files, FileText, Wallet, LoaderCircle, LockKeyhole, CircleCheck, Circle, RefreshCw, ArrowRight, ShieldCheck, ListChecks, Database, FolderTree, Box } from 'lucide-react';

const ADD_STEPS = ['基本信息', '代码源接入', '上游源', '确认'];

// 围绕当前被测对象的测试治理环节状态条（被测对象为轴心）——随 selectedAsset 从 mock 真实数据派生：
// 质量门禁与 GatePage 同源（assetToProfile.riskCount）、用例数与 CasesPage 同源（CASES）、成本与成本审计同源（COST_*_DETAIL）
// 环节状态点：绿实心=完成、琥珀=待处理、红=阻断、灰=待办
const STAGE_DOT: Record<string, { cls: string; txt: string }> = {
  done: { cls: 'bg-emerald-500', txt: 'text-emerald-600' },
  partial: { cls: 'bg-amber-500', txt: 'text-amber-600' },
  block: { cls: 'bg-red-500', txt: 'text-red-600' },
  todo: { cls: 'bg-slate-300', txt: 'text-slate-400' },
};

const DIM_BAR: Record<string, string> = {
  '需求覆盖率': 'bg-emerald-500',
  '门禁通过': 'bg-amber-500',
  '契约健康': 'bg-emerald-500',
  '用例有效': 'bg-emerald-500',
  '追溯完整': 'bg-amber-500',
};
const GEN_ICON: Record<string, typeof History> = { '规格解析': FileSearch, '架构生成': Layers, '代码生成': Sparkles, '自检回放': CheckCircle2, '提交验收': ShieldAlert };
const TYPE_LABEL: Record<string, string> = { service: '服务', 'service-group': '服务组', app: '应用', end: '端', project: '工程', module: '模块' };
const TYPE_COLOR: Record<string, string> = {
  project: 'bg-indigo-500', 'service-group': 'bg-sky-500', service: 'bg-emerald-500',
  module: 'bg-teal-500', app: 'bg-purple-500', end: 'bg-amber-500',
};
const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles };
const ASSET_SCEN: Record<string, string[]> = {
  auth: ['SCEN-01', 'SCEN-02', 'SCEN-04'],
  payment: ['SCEN-02', 'SCEN-03', 'SCEN-05'],
  order: ['SCEN-02', 'SCEN-03'],
  user: ['SCEN-01', 'SCEN-02'],
  web: ['SCEN-06', 'SCEN-07', 'SCEN-08'],
  mobile: ['SCEN-09', 'SCEN-10', 'SCEN-11'],
  metric: ['SCEN-01', 'SCEN-02', 'SCEN-04'],
  alert: ['SCEN-01', 'SCEN-02'],
};
function assetScenarios(asset: IAsset) {
  const hits = Object.entries(ASSET_SCEN).filter(([k]) => asset.name.includes(k)).flatMap(([, v]) => v);
  const pool = hits.length ? hits : (asset.type === 'app' || asset.type === 'end' ? ['SCEN-06', 'SCEN-09'] : ['SCEN-02']);
  return [...new Set(pool)].map((id) => TEST_SCENARIOS.find((s) => s.id === id)).filter((s): s is NonNullable<typeof s> => !!s);
}

// 环节状态条派生 helper：与 GatePage/CasesPage/成本审计 同源
function caseCount(asset: IAsset): number { return CASES.filter((c) => c.asset === asset.name && c.status !== '已禁用').length; }
function genCount(asset: IAsset): number { return COST_GEN_DETAIL.filter((d) => d.asset === asset.name).length; }
function objCost(asset: IAsset): number { return [...COST_GEN_DETAIL, ...COST_EXEC_DETAIL].filter((d) => d.asset === asset.name).reduce((s, d) => s + d.amount, 0); }
function costTxt(asset: IAsset): string { const v = objCost(asset); if (v <= 0) return '待产生'; return v >= 1000 ? `¥${(v / 1000).toFixed(1)}k` : `¥${v}`; }

// 子树是否包含目标节点（用于默认展开当前链路）
function containsNode(n: IAssetNode, id: string): boolean {
  if (n.id === id) return true;
  return (n.children ?? []).some((c) => containsNode(c, id));
}

// 被测对象树节点（受控展开：默认折叠非当前链路，点击节点联动画像与全局上下文）
function TreeNode({ node, activeId, onSelect, depth, openMap, onToggle }: {
  node: IAssetNode; activeId: string; onSelect: (n: IAssetNode) => void; depth: number;
  openMap: Record<string, boolean>; onToggle: (n: IAssetNode) => void;
}) {
  const hasChildren = !!node.children && node.children.length > 0;
  const active = node.id === activeId;
  const open = openMap[node.id] ?? (hasChildren && containsNode(node, activeId));
  return (
    <div>
      <div
        role="button"
        onClick={() => { if (hasChildren) onToggle(node); onSelect(node); }}
        className={'flex items-center gap-2 rounded-lg py-2 cursor-pointer select-none ' + (active ? 'bg-emerald-50 text-emerald-700 font-medium' : 'text-slate-600 hover:bg-slate-50')}
        style={{ paddingLeft: depth * 18 + 10 }}
      >
        {hasChildren
          ? (open ? <ChevronDown className="w-3.5 h-3.5 text-slate-400 flex-shrink-0" /> : <ChevronRight className="w-3.5 h-3.5 text-slate-400 flex-shrink-0" />)
          : <span className="w-3.5 flex-shrink-0" />}
        <span className={'w-2 h-2 rounded-sm flex-shrink-0 ' + (TYPE_COLOR[node.type] ?? 'bg-slate-300')} />
        <span className="text-xs truncate">{node.name}</span>
        {!hasChildren && node.repo && (
          <span className="ml-1 inline-flex items-center gap-0.5 text-[9px] text-emerald-600 flex-shrink-0"><GitBranch className="w-3 h-3" />{node.repo}</span>
        )}
        {!hasChildren && !node.repo && (
          <span className="ml-1 text-[9px] text-slate-300 flex-shrink-0">未接入源码</span>
        )}
        {hasChildren && nodeRepos(node).length > 0 && (
          <span className="ml-1 text-[9px] text-slate-400 flex-shrink-0">{nodeRepos(node).length} 仓</span>
        )}
        <span className={'ml-auto text-[9px] font-normal flex-shrink-0 ' + (node.coverage >= 85 ? 'text-emerald-500' : 'text-amber-500')}>{node.coverage}%</span>
      </div>
      {hasChildren && open && node.children!.map((c) => (
        <TreeNode key={c.id} node={c} activeId={activeId} onSelect={onSelect} depth={depth + 1} openMap={openMap} onToggle={onToggle} />
      ))}
    </div>
  );
}

type ProfileFieldKey = 'language' | 'framework';

const PROFILE_FIELDS: ReadonlyArray<readonly [ProfileFieldKey, string, string]> = [
  ['language', '语言', '例如 Java、Go、TypeScript'],
  ['framework', '框架', '例如 Spring Boot、Gin、React'],
];

function profileCompletion(service: CatalogService) {
  const missing = PROFILE_FIELDS.filter(([key]) => !service[key]?.trim()).map(([, label]) => label);
  return { complete: missing.length === 0, missing };
}

function profileConfidencePercent(service: CatalogService): number | null {
  const value = service.profile_confidence;
  if (typeof value !== 'number' || !Number.isFinite(value)) return null;
  return Math.round(value <= 1 ? value * 100 : value);
}

function profileSourceLabel(source: string): string {
  if (!source) return '未记录分析来源';
  return source.startsWith('model://') ? `模型分析 · ${source.slice('model://'.length)}` : source;
}

/** The repository root is a user-facing scope, never an implementation dot. */
function sourceDirectoryLabel(sourcePath: string): string {
  return sourcePath.trim() === '.' ? '仓库根目录' : sourcePath;
}

function profileEvidenceParts(evidence: string): { file: string; reason: string } {
  const separator = evidence.indexOf('：');
  return separator > 0
    ? { file: evidence.slice(0, separator), reason: evidence.slice(separator + 1) }
    : { file: '未标注文件', reason: evidence };
}

function analysisTimeLabel(value?: string): string {
  if (!value) return '尚未分析';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false });
}

type ServicePreparation = 'needs_analysis' | 'analyzing' | 'analysis_failed' | 'needs_completion' | 'needs_review' | 'needs_authorization' | 'ready';

function servicePreparation(service: CatalogService): ServicePreparation {
  const { complete } = profileCompletion(service);
  if (service.generation_status === 'confirmed' && service.profile_status === 'confirmed' && complete) return 'ready';
  if (service.profile_status === 'analyzing') return 'analyzing';
  if (service.profile_status === 'failed') return 'analysis_failed';
  if (service.profile_status === 'suggested' && !complete) return 'needs_completion';
  if (service.profile_status === 'suggested') return 'needs_review';
  if (service.profile_status === 'confirmed') return 'needs_authorization';
  return 'needs_analysis';
}

const PREPARATION_COPY: Record<ServicePreparation, { label: string; detail: string; tone: string }> = {
  needs_analysis: { label: '待进行 AI 分析', detail: '尚未识别技术栈与测试方式', tone: 'bg-slate-100 text-slate-600' },
  analyzing: { label: 'AI 正在分析', detail: '正在读取源码和构建配置', tone: 'bg-blue-50 text-blue-700' },
  analysis_failed: { label: '分析未完成', detail: '请重试，或转为人工补充配置', tone: 'bg-red-50 text-red-700' },
  needs_completion: { label: '画像待补齐', detail: '测试配置不完整，尚不能授权生成', tone: 'bg-amber-50 text-amber-700' },
  needs_review: { label: '待人工核验', detail: 'AI 已给出建议，需核对证据和配置', tone: 'bg-amber-50 text-amber-700' },
  needs_authorization: { label: '待授权生成', detail: '技术画像已确认，尚未允许用例生成', tone: 'bg-indigo-50 text-indigo-700' },
  ready: { label: '可生成用例', detail: '技术画像已确认且生成已获授权', tone: 'bg-emerald-50 text-emerald-700' },
};

function PreparationStep({ label, state }: { label: string; state: 'done' | 'current' | 'pending' }) {
  const palette = state === 'done'
    ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
    : state === 'current'
      ? 'border-amber-200 bg-amber-50 text-amber-700'
      : 'border-slate-200 bg-white text-slate-400';
  return <div className={`flex min-w-0 items-center gap-1.5 rounded-lg border px-2 py-1.5 text-[10px] font-medium ${palette}`}>
    {state === 'done' ? <CircleCheck className="h-3.5 w-3.5 shrink-0" /> : <Circle className="h-3.5 w-3.5 shrink-0" />}
    <span className="truncate">{label}</span>
  </div>;
}

function ServicePreparationCard({
  service,
  repoName,
  busy,
  serviceKinds,
  onKindChange,
  onAnalyze,
  onOpenProfile,
  onOpenGeneration,
  onRevokeGeneration,
}: {
  service: CatalogService;
  repoName: (repoID: number) => string;
  busy: boolean;
  serviceKinds: ReadonlyArray<readonly [string, string]>;
  onKindChange: (kind: string) => void;
  onAnalyze: () => void;
  onOpenProfile: () => void;
  onOpenGeneration: () => void;
  onRevokeGeneration: () => void;
}) {
  const preparation = servicePreparation(service);
  const copy = PREPARATION_COPY[preparation];
  const completion = profileCompletion(service);
  const confidence = profileConfidencePercent(service);
  const evidenceCount = service.profile_evidence?.length ?? 0;
  const reviewRisk = (confidence !== null && confidence < 70) || evidenceCount === 0;
  const analysisState: 'done' | 'current' | 'pending' = ['needs_completion', 'needs_review', 'needs_authorization', 'ready'].includes(preparation) ? 'done' : preparation === 'analyzing' ? 'current' : 'pending';
  const reviewState: 'done' | 'current' | 'pending' = service.profile_status === 'confirmed' ? 'done' : ['needs_completion', 'needs_review', 'analysis_failed'].includes(preparation) ? 'current' : 'pending';
  const authorizationState: 'done' | 'current' | 'pending' = preparation === 'ready' ? 'done' : preparation === 'needs_authorization' ? 'current' : 'pending';
  let action: React.ReactNode;
  if (preparation === 'ready') {
    action = <div className="flex flex-wrap items-center gap-2"><button type="button" onClick={onOpenGeneration} className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3 py-2 text-xs font-semibold text-white shadow-sm hover:bg-emerald-700"><Sparkles className="h-3.5 w-3.5" />进入用例生成<ArrowRight className="h-3.5 w-3.5" /></button><button type="button" disabled={busy} onClick={onOpenProfile} className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-medium text-slate-600 hover:border-emerald-300 hover:text-emerald-700">查看画像</button><button type="button" disabled={busy} onClick={onRevokeGeneration} className="px-2 py-2 text-[11px] text-slate-400 hover:text-red-600">暂停生成授权</button></div>;
  } else if (preparation === 'analyzing') {
    action = <button type="button" disabled className="inline-flex items-center gap-1.5 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 text-xs font-medium text-blue-700"><LoaderCircle className="h-3.5 w-3.5 animate-spin" />分析中，会自动更新</button>;
  } else if (preparation === 'needs_analysis' || preparation === 'analysis_failed') {
    action = <div className="flex flex-wrap items-center gap-2"><button type="button" disabled={busy} onClick={onAnalyze} className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3 py-2 text-xs font-semibold text-white hover:bg-emerald-700"><Sparkles className="h-3.5 w-3.5" />{preparation === 'analysis_failed' ? '重新进行 AI 分析' : '开始 AI 分析'}</button>{preparation === 'analysis_failed' && <button type="button" disabled={busy} onClick={onOpenProfile} className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-medium text-slate-600 hover:border-emerald-300 hover:text-emerald-700">人工补充画像</button>}</div>;
  } else {
    action = <button type="button" disabled={busy} onClick={onOpenProfile} className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3 py-2 text-xs font-semibold text-white hover:bg-emerald-700">{preparation === 'needs_authorization' ? <LockKeyhole className="h-3.5 w-3.5" /> : <ShieldCheck className="h-3.5 w-3.5" />}{preparation === 'needs_authorization' ? '查看画像并授权生成' : completion.complete ? '核验 AI 技术画像' : '补齐并核验画像'}<ArrowRight className="h-3.5 w-3.5" /></button>;
  }

  return <article className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm transition hover:border-emerald-200 hover:shadow-md">
    <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          {preparation === 'ready'
            ? <button type="button" onClick={onOpenGeneration} className="text-left text-sm font-semibold text-slate-800 hover:text-emerald-700 hover:underline">{service.name}</button>
            : <button type="button" onClick={onOpenProfile} title="需先完成技术画像核验与生成授权" className="text-left text-sm font-semibold text-slate-800 hover:text-emerald-700 hover:underline">{service.name}</button>}
          <span className={`rounded-full px-2 py-1 text-[10px] font-semibold ${copy.tone}`}>{copy.label}</span>
          {reviewRisk && ['needs_completion', 'needs_review', 'needs_authorization'].includes(preparation) && <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2 py-1 text-[10px] font-medium text-amber-700"><AlertTriangle className="h-3 w-3" />需注意证据风险</span>}
        </div>
        <p className="mt-1 text-xs text-slate-500">{copy.detail}</p>
        <div className="mt-3 grid gap-2 text-[11px] text-slate-600 sm:grid-cols-2 xl:grid-cols-4">
          <span className="flex min-w-0 items-center gap-1.5"><GitBranch className="h-3.5 w-3.5 shrink-0 text-slate-400" /><span className="truncate" title={repoName(service.repo_id)}>{repoName(service.repo_id)}</span></span>
          <span className="flex min-w-0 items-center gap-1.5"><Files className="h-3.5 w-3.5 shrink-0 text-slate-400" /><code className="truncate" title={sourceDirectoryLabel(service.source_path)}>{sourceDirectoryLabel(service.source_path)}</code></span>
          <span className="flex min-w-0 items-center gap-1.5"><Database className="h-3.5 w-3.5 shrink-0 text-slate-400" /><span className="truncate">{profileSourceLabel(service.profile_source)}</span></span>
          <span className="flex items-center gap-1.5"><ListChecks className="h-3.5 w-3.5 shrink-0 text-slate-400" />{confidence === null ? '置信度待产出' : `置信度 ${confidence}%`} · {evidenceCount} 条证据</span>
        </div>
      </div>
      <div className="flex shrink-0 flex-col items-start gap-2 xl:items-end">
        <label className="flex items-center gap-2 text-[10px] text-slate-400">对象类型<select value={service.kind} disabled={busy} onChange={(event) => onKindChange(event.target.value)} className="rounded-md border border-slate-200 bg-white px-2 py-1.5 text-[11px] font-medium text-slate-700 outline-none focus:border-emerald-400">{serviceKinds.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
        {action}
      </div>
    </div>
    <div className="mt-4 grid grid-cols-2 gap-2 border-t border-slate-100 pt-3 sm:grid-cols-4">
      <PreparationStep label="已登记来源" state="done" />
      <PreparationStep label="AI 分析" state={analysisState} />
      <PreparationStep label="人工核验" state={reviewState} />
      <PreparationStep label="授权生成" state={authorizationState} />
    </div>
    {preparation === 'needs_completion' && <p className="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-[11px] text-amber-800">缺少：{completion.missing.join('、')}。补齐后请核对源码证据，再明确授权 AI 生成用例。</p>}
    {preparation === 'analysis_failed' && <p className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-[11px] text-red-700">本次分析未完成，不会自动开启用例生成。可重试分析，或由测试负责人补充技术画像。</p>}
  </article>;
}

export default function TargetPage() {
  const navigate = useNavigate();
  const { selectedAsset, setSelectedAsset } = useOutletContext<{ selectedAsset: IAssetNode; setSelectedAsset: (n: IAssetNode) => void }>();
  const profile = assetToProfile(selectedAsset);
  const risks = profile.gateRules.filter((r) => r.status === 'block');
  // 环节状态条：从当前被测对象真实数据派生（与 GatePage / CasesPage / 成本审计 同源），随树切换联动
  const stageFlow = [
    { path: '/generation', label: '用例生成', icon: Sparkles, status: genCount(selectedAsset) > 0 ? 'done' : 'todo', value: genCount(selectedAsset) > 0 ? `${genCount(selectedAsset)} 用例` : '待生成' },
    { path: '/trace', label: '需求追溯', icon: GitBranch, status: 'done', value: '完整' },
    { path: '/cases', label: '用例管理', icon: Files, status: caseCount(selectedAsset) > 0 ? 'done' : 'todo', value: `${caseCount(selectedAsset)}` },
    { path: '/exec', label: '测试执行', icon: CircleDot, status: 'partial', value: '待执行' },
    { path: '/gate', label: '质量门禁', icon: ShieldAlert, status: risks.length > 0 ? 'block' : 'done', value: risks.length > 0 ? `${risks.length} 阻断` : '通过' },
    { path: '/history', label: '测试历史', icon: History, status: 'done', value: '近 6 次' },
    { path: '/report', label: '测试报告', icon: FileText, status: 'todo', value: '待生成' },
    { path: '/audit-cost', label: '成本审计', icon: Wallet, status: objCost(selectedAsset) > 0 ? 'done' : 'todo', value: costTxt(selectedAsset) },
  ];
  const [openMap, setOpenMap] = useState<Record<string, boolean>>({});
  const [q, setQ] = useState('');
  const kw = q.trim().toLowerCase();
  const riskFiltered = ASSET_RISKS.filter((a) => !kw || (a.name + a.type).toLowerCase().includes(kw));
  const riskAssets = ASSET_RISKS.filter((a) => a.gate === '阻断');
  const modelBound = PROJECT_MODELS.find((p) => p.projectId === profile.name);
  const model = modelBound && AI_MODELS.find((m) => m.id === modelBound.modelId);

  const realClient = useMemo(() => configuredGreenPassClient(), []);

  // 被测对象页 = 被测仓库与版本管理的权威来源（仓库列表可增删）
  const [repos, setRepos] = useState<ITestedRepo[]>(TESTED_REPOS);
  // 添加仓库分步录入弹窗
  const [showAdd, setShowAdd] = useState(false);
  const [addStep, setAddStep] = useState(0);
  const [addForm, setAddForm] = useState({ name: '', type: 'GitLab', url: '', node: '服务', owner: '', ver: '', auth: 'Access Token', token: '' });
  const [addBranches, setAddBranches] = useState<string[]>([]);
  const [addBrInput, setAddBrInput] = useState('');
  const [connected, setConnected] = useState(false);
  const [addSrc, setAddSrc] = useState<string[]>([]);
  const urlPrefix = addForm.type === 'GitHub' ? 'https://github.com/org/' : addForm.type === 'Gitee' ? 'https://gitee.com/' : 'https://gitlab.example.com/';
  const srcOptions = ['需求文档', '设计文档', 'API 契约', '代码变更', '生产追踪', '缺陷报告'];
  const canNext = addStep === 0 ? addForm.name.trim() !== '' && addForm.url.trim() !== ''
    : addStep === 1 ? connected
    : addStep === 2 ? addSrc.length > 0
    : true;
  const goGenerate = (r: ITestedRepo) => { try { localStorage.setItem('greenpass_pick_repo', r.name); } catch { /* ignore */ } navigate('/generation'); };
  const submitAdd = () => {
    const nr: ITestedRepo = {
      name: addForm.name.trim(), source: addForm.type, branch: addBranches[0] ?? 'main',
      branches: addBranches.length ? addBranches : ['main'], versions: [addForm.ver.trim() || 'v1.0.0'],
      synced: '2026-09-28', owner: addForm.owner.trim() || '张立', src: addSrc,
    };
    setRepos((p) => [...p, nr]);
    setShowAdd(false); setAddStep(0); setConnected(false); setAddBranches([]); setAddSrc([]);
    setAddForm({ name: '', type: 'GitLab', url: '', node: '服务', owner: '', ver: '', auth: 'Access Token', token: '' });
    toast.success('仓库已添加', { description: `${nr.name} 已纳入被测对象库，可到「生成用例」对其发起用例生成` });
  };

  if (realClient) {
    return (
      <RealTargetsPanel client={realClient} />
    );
  }

  return (
    <div>
      <PageHeader title="被测对象" desc="被测对象树 + 画像一体 · 点击左侧树节点联动右侧画像与全局上下文">
        <PrimaryButton><Link to="/gate" className="flex items-center gap-1">进入质量门禁<ChevronRight className="w-4 h-4" /></Link></PrimaryButton>
      </PageHeader>

      <RealTargetsPanel client={realClient} />

      <div className="grid grid-cols-[360px_1fr] gap-6 items-start">
        {/* ===== 左侧：被测对象树（一体化导航） ===== */}
        <aside className="space-y-5">
          <div className="card bg-white rounded-xl border border-slate-200 p-4">
            <h2 className="font-semibold text-slate-700 text-sm mb-1 flex items-center gap-1.5"><Layers className="w-4 h-4 text-emerald-500" />被测对象树</h2>
            <p className="text-[10px] text-slate-400 mb-3">四级层级：工程 → 服务组/服务 → 模块 → 应用/端 · 点击节点联动画像</p>
            <TreeNode node={ASSET_TREE} activeId={selectedAsset.id} onSelect={setSelectedAsset} depth={0}
              openMap={openMap} onToggle={(n) => setOpenMap((p) => ({ ...p, [n.id]: !(p[n.id] ?? (n.children ? containsNode(n, selectedAsset.id) : false)) }))} />
          </div>
          <div>
            <h2 className="font-semibold text-slate-700 text-sm mb-2">被测对象库全局指标</h2>
            <span className="text-[10px] text-slate-400 mb-2 block">库级汇总 · 右侧画像为当前所选节点</span>
            <div className="grid grid-cols-2 gap-2.5">
              {METRICS.map((m) => (
                <KpiCard key={m.label} label={m.label} value={`${m.value}%`} color={m.color} note={m.note} />
              ))}
            </div>
          </div>
        </aside>

        {/* ===== 右侧：当前节点画像（随树联动） ===== */}
        <div className="min-w-0 space-y-5">
          {/* 被测对象主卡：随所选节点联动 */}
          <div className="card bg-gradient-to-br from-emerald-600 to-teal-700 rounded-2xl border border-transparent p-6 text-white">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div className="flex items-center gap-4">
                <div className="w-12 h-12 rounded-xl bg-white/10 flex items-center justify-center">
                  <Brain className="w-6 h-6" />
                </div>
                <div>
                  <div className="text-xs text-emerald-100 flex items-center gap-2">
                    被测对象 · {TYPE_LABEL[selectedAsset.type] ?? selectedAsset.type}
                    <span className="text-[9px] bg-white/15 px-1.5 py-0.5 rounded-full">覆盖率 {selectedAsset.coverage}%</span>
                  </div>
                  <div className="text-xl font-bold">{profile.name}</div>
                  <div className="text-[11px] text-emerald-100 mt-1 flex items-center gap-1">
                    <Sparkles className="w-3 h-3" />门禁通过率 {profile.passRate}% · {profile.riskCount > 0 ? `${profile.riskCount} 项风险待处理` : '当前无未决风险'}
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-6">
                <div className="text-center">
                  <div className="text-3xl font-bold">{profile.qualityScore}</div>
                  <div className="text-[11px] text-emerald-100">质量评分</div>
                </div>
                <div className="text-center">
                  <div className="text-3xl font-bold text-amber-300">{profile.riskCount}</div>
                  <div className="text-[11px] text-emerald-100">未决风险</div>
                </div>
              </div>
            </div>
            <div className="mt-5">
              <div className="flex justify-between text-[11px] text-emerald-100 mb-1.5">
                <span>门禁通过率</span><span>{profile.passRate}%</span>
              </div>
              <div className="h-2 bg-white/15 rounded-full overflow-hidden">
                <div className="h-full bg-emerald-300 rounded-full" style={{ width: `${profile.passRate}%` }} />
              </div>
            </div>
            {model && (
              <div className="mt-3 pt-3 border-t border-white/15 flex items-center justify-between text-[11px] text-emerald-100">
                <span className="flex items-center gap-1.5"><Cpu className="w-3.5 h-3.5" />测试所用 AI 模型</span>
                <span className="flex items-center gap-2">
                  <span className="font-medium">{model.name}</span>
                  <span className="text-[9px] bg-white/15 px-2 py-0.5 rounded-full">{model.vendor}</span>
                </span>
              </div>
            )}
            {/* 来源溯源：被测对象 → 源码仓库 → 分支@版本（实现「来源可溯源」） */}
            {nodeRepos(selectedAsset).length > 0 ? (
              <div className="mt-3 pt-3 border-t border-white/15 text-[11px] text-emerald-100">
                <div className="flex items-center gap-1.5 mb-1.5"><GitBranch className="w-3.5 h-3.5" />来源溯源 · 源码仓库</div>
                <div className="flex flex-wrap gap-1.5">
                  {nodeRepos(selectedAsset).map((repo) => {
                    const rp = repos.find((x) => x.name === repo);
                    return (
                      <span key={repo} className="inline-flex items-center gap-1 bg-white/10 rounded-full px-2 py-0.5 font-mono">
                        {repo}<span className="text-emerald-200">{rp ? `${rp.branch}@${rp.versions[0]}` : '未同步'}</span>
                      </span>
                    );
                  })}
                </div>
              </div>
            ) : (
              <div className="mt-3 pt-3 border-t border-white/15 text-[11px] text-amber-200 flex items-center gap-1.5">
                <GitBranch className="w-3.5 h-3.5" />来源未接入源码仓库 —— 需绑定仓库方可溯源
              </div>
            )}
          </div>

          {/* 被测对象 = 轴心：环节状态条（一页一焦点，状态可见，不铺文字） */}
          <div className="card bg-white rounded-xl border border-slate-200 p-5">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><Layers className="w-4 h-4 text-emerald-500" />围绕 {profile.name} 的测试治理环节</h2>
              <span className="text-[11px] text-slate-400">状态点：绿=完成 · 琥珀=待处理 · 红=阻断 · 灰=待办 · 点击进入</span>
            </div>
            <div className="grid grid-cols-4 gap-2.5">
              {stageFlow.map((s) => {
                const Icon = s.icon;
                const dot = STAGE_DOT[s.status];
                const core = s.label === '用例生成' || s.label === '质量门禁';
                return (
                  <Link key={s.path} to={s.path}
                    className={'flex items-center gap-2.5 rounded-xl border px-3 py-2.5 transition hover:-translate-y-0.5 ' + (core ? 'bg-emerald-50/60 border-emerald-200 hover:border-emerald-300 hover:shadow-sm' : 'bg-white border-slate-200 hover:border-emerald-300 hover:shadow-sm')}>
                    <span className={'w-8 h-8 rounded-lg flex items-center justify-center shrink-0 ' + (core ? 'bg-emerald-500 text-white' : 'bg-emerald-50 text-emerald-600')}><Icon className="w-4 h-4" /></span>
                    <div className="min-w-0">
                      <div className="text-xs font-medium text-slate-700 flex items-center gap-1.5">{s.label}{core && <span className="text-[9px] text-emerald-500">核心</span>}</div>
                      <div className="flex items-center gap-1.5 mt-0.5">
                        <span className={'w-2 h-2 rounded-full flex-shrink-0 ' + dot.cls} />
                        <span className={'text-[11px] font-medium ' + dot.txt}>{s.value}</span>
                      </div>
                    </div>
                  </Link>
                );
              })}
            </div>
          </div>

          {/* 被测对象页 = 被测对象管理总入口：仓库与版本管理（权威来源） */}
          <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
            <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between flex-wrap gap-2">
              <div className="flex items-center gap-2.5">
                <h2 className="font-semibold text-slate-700 text-sm">仓库与版本管理</h2>
                <span className="text-[11px] bg-emerald-50 text-emerald-700 px-2 py-0.5 rounded-full font-medium">{repos.length} 个被测仓库</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-[10px] text-slate-400 hidden md:inline">被测对象库统一管理 · 选择「发起生成」跳转生成用例</span>
                <GhostButton onClick={() => setShowAdd(true)}>
                  <Plus className="w-3.5 h-3.5 mr-1" />添加仓库
                </GhostButton>
              </div>
            </div>
            <div className="px-5 py-2 bg-slate-50/50 border-b border-slate-100 flex items-center gap-2 text-[11px] text-slate-500">
              <ExternalLink className="w-3.5 h-3.5 text-emerald-500 shrink-0" />
              <span>仓库是<b>被测对象的资产底座</b>：此处管理仓库/分支/版本/上游源接入（树节点选中的服务将在下方高亮对应仓库）；用例生成在生成用例页从本库选择对象。</span>
            </div>
            <table className="w-full text-xs">
              <thead className="bg-slate-50 border-b border-slate-200">
                <tr className="text-left text-slate-500">
                  <th className="px-5 py-2.5 font-medium">被测仓库</th>
                  <th className="px-5 py-2.5 font-medium">代码源</th>
                  <th className="px-5 py-2.5 font-medium">分支</th>
                  <th className="px-5 py-2.5 font-medium">可用版本</th>
                  <th className="px-5 py-2.5 font-medium">接入上游源</th>
                  <th className="px-5 py-2.5 font-medium">最近同步</th>
                  <th className="px-5 py-2.5 font-medium">负责人</th>
                  <th className="px-5 py-2.5 font-medium text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {repos.map((r) => {
                  const linked = nodeRepos(selectedAsset).includes(r.name);
                  return (
                    <tr key={r.name} className={(linked ? 'bg-emerald-50/60' : 'hover:bg-slate-50')}>
                      <td className="px-5 py-3">
                        <div className="flex items-center gap-2">
                          <span className="flex items-center justify-center w-5 h-5 rounded bg-emerald-50 text-emerald-600"><GitBranch className="w-3.5 h-3.5" /></span>
                          <span className="font-medium text-slate-700">{r.name}</span>
                          {linked && <span className="text-[10px] text-emerald-600">来源仓库 · 树联动</span>}
                        </div>
                      </td>
                      <td className="px-5 py-3 text-slate-500">{r.source}</td>
                      <td className="px-5 py-3 text-slate-500">{r.branch}</td>
                      <td className="px-5 py-3"><span className="font-mono text-emerald-700">{r.versions[0]}</span><span className="text-slate-400 ml-1 text-[10px]">+{r.versions.length - 1}</span></td>
                      <td className="px-5 py-3"><span className="text-emerald-600">{r.src.length}/6 类</span></td>
                      <td className="px-5 py-3 text-slate-500">{r.synced}</td>
                      <td className="px-5 py-3 text-slate-500">{r.owner}</td>
                      <td className="px-5 py-3 text-right">
                        <button type="button" onClick={() => goGenerate(r)}
                          className="px-2.5 py-1 rounded-md text-[11px] bg-emerald-600 text-white hover:bg-emerald-700">发起生成</button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          {/* 该节点涉及的测试场景：打通 画像 ↔ 测试中心 */}
          <div className="card bg-white rounded-xl border border-slate-200 p-5">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><Layers className="w-4 h-4 text-emerald-500" />{profile.name} · 测试场景覆盖</h2>
              <span className="text-[11px] text-slate-400">关联的测试中心场景 · 点击进入对应场景闭环</span>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-3 2xl:grid-cols-4 gap-3">
              {assetScenarios(selectedAsset).map((s) => {
                const Icon = SCEN_ICON[s.icon] ?? Cpu;
                const lp = s.history[s.history.length - 1].p;
                return (
                  <button key={s.id} type="button"
                    onClick={() => { scenarioNav.go(s.id, 'cases'); navigate('/scenarios'); }}
                    className="flex items-center gap-3 border rounded-xl p-3 text-left transition-all hover:shadow-sm hover:border-emerald-300 hover:-translate-y-0.5">
                    <span className="w-9 h-9 rounded-lg bg-emerald-500 text-white flex items-center justify-center flex-shrink-0"><Icon className="w-5 h-5" /></span>
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium text-slate-800 flex items-center gap-1.5">{s.name}<span className="text-[10px] text-slate-400 font-mono">{s.id}</span></div>
                      <div className="text-[10px] text-slate-400">通过率 <span className={lp >= 90 ? 'text-emerald-600 font-medium' : 'text-amber-600 font-medium'}>{lp}%</span> · {s.form}</div>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>

          {/* AI 生成过程还原 */}
          <div className="card bg-white rounded-xl border border-slate-200 p-5">
            <div className="flex items-center justify-between mb-5">
              <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><History className="w-4 h-4 text-emerald-500" />AI 生成过程还原</h2>
              <span className="text-[11px] text-slate-400">从需求到提交验收 · 本对象由 AI 全流程产出</span>
            </div>
            <div className="flex flex-col md:flex-row gap-4">
              {GEN_TRACE.map((g) => {
                const Icon = GEN_ICON[g.step] ?? History;
                const done = g.status === 'done';
                const active = g.status === 'active';
                return (
                  <div key={g.seq} className="flex md:flex-1 items-start md:flex-col gap-3 relative">
                    <div className={`w-4 h-4 rounded-full flex-shrink-0 mt-1 md:mt-0 ${active ? 'bg-emerald-500 ring-4 ring-emerald-100' : done ? 'bg-emerald-400' : 'bg-slate-300'}`} />
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <Icon className="w-4 h-4 text-emerald-500" />
                        <span className="text-sm font-medium text-slate-700">{g.step}</span>
                        {active && <span className="text-[9px] bg-emerald-100 text-emerald-700 px-1.5 py-0.5 rounded-full">当前</span>}
                      </div>
                      <div className="text-[10px] text-slate-400 mt-0.5 font-mono">{g.actor} · {g.time}</div>
                      <div className="text-[11px] text-slate-600 mt-1 leading-relaxed">{g.desc}</div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* 质量评分构成 + 未决风险 + 风险明细 */}
          <div className="grid grid-cols-3 gap-5">
            <div className="card bg-white rounded-xl border border-slate-200 p-5">
              <h2 className="font-semibold text-slate-700 text-sm mb-4 flex items-center gap-1.5"><ShieldAlert className="w-4 h-4 text-emerald-500" />质量评分构成</h2>
              <div className="space-y-3.5">
                {profile.dims.map((d) => (
                  <div key={d.label}>
                    <div className="flex justify-between text-xs mb-1.5">
                      <span className="text-slate-600">{d.label}</span>
                      <span className="text-slate-500">{d.score}<span className="text-slate-400"> · {Math.round(d.weight * 100)}%</span></span>
                    </div>
                    <div className="h-2 bg-slate-100 rounded-full">
                      <div className={`h-full rounded-full ${DIM_BAR[d.label] ?? 'bg-emerald-500'}`} style={{ width: `${d.score}%` }} />
                    </div>
                    <div className="text-[10px] text-slate-400 mt-1">{d.note}</div>
                  </div>
                ))}
              </div>
              <div className="mt-4 pt-3 border-t border-slate-100 flex items-center justify-between text-[11px]">
                <span className="text-slate-500">综合评分（加权）</span>
                <span className="text-emerald-600 font-semibold">{profile.qualityScore} / 100</span>
              </div>
            </div>

            <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
              <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
                <h2 className="font-semibold text-slate-700 text-sm">当前未决风险 · {profile.name}</h2>
                <span className="text-[11px] text-red-500 font-medium">{risks.length} 项阻断</span>
              </div>
              <div className="divide-y divide-slate-100">
                {risks.length === 0 ? (
                  <div className="px-5 py-6 text-center text-[11px] text-slate-400">该被测对象门禁全部通过，无未决风险</div>
                ) : risks.map((r) => (
                  <div key={r.name} className="px-5 py-3.5 flex items-start gap-3">
                    <AlertTriangle className="w-4 h-4 text-red-500 flex-shrink-0 mt-0.5" />
                    <div className="flex-1">
                      <div className="text-sm font-medium text-slate-700">{r.name}</div>
                      <div className="text-[11px] text-slate-500 mt-0.5">{r.detail}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
              <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
                <h2 className="font-semibold text-slate-700 text-sm">被测对象风险明细</h2>
                <span className="text-[11px] text-slate-400">{riskAssets.length} 个阻断</span>
              </div>
              <div className="flex items-center justify-between px-5 pt-3">
                <ListFilter search={q} onSearch={setQ} />
                <span className="text-[11px] text-slate-400">共 {riskFiltered.length} / {ASSET_RISKS.length} 项</span>
              </div>
              <div className="divide-y divide-slate-100">
                {riskFiltered.map((a) => (
                  <div key={a.name} className="px-5 py-3 flex items-center gap-3">
                    <div className="flex-1">
                      <div className="text-xs font-medium text-slate-700 flex items-center gap-2">
                        {a.name}
                        <span className="text-[9px] text-slate-400 font-normal">{a.type}</span>
                      </div>
                      <div className="flex items-center gap-2 mt-1.5">
                        <div className="w-14 h-1 bg-slate-100 rounded-full overflow-hidden">
                          <div className={`h-full rounded-full ${a.coverage >= 85 ? 'bg-emerald-500' : 'bg-amber-500'}`} style={{ width: `${a.coverage}%` }} />
                        </div>
                        <span className="text-[9px] text-slate-400">{a.coverage}%</span>
                      </div>
                    </div>
                    {a.gate === '通过'
                      ? <span className="text-[10px] text-emerald-600 bg-emerald-50 px-2 py-0.5 rounded-full flex items-center gap-1 flex-shrink-0"><CheckCircle2 className="w-3 h-3" />通过</span>
                      : <span className="text-[10px] text-red-600 bg-red-50 px-2 py-0.5 rounded-full flex items-center gap-1 flex-shrink-0"><XCircle className="w-3 h-3" />阻断</span>}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* 添加仓库：四步分步录入弹窗（基本信息 → 代码源接入 → 上游源 → 确认） */}
      {showAdd && (
        <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl w-full max-w-2xl shadow-2xl max-h-[90vh] overflow-y-auto">
            <div className="px-6 py-4 border-b border-slate-200 flex items-center justify-between">
              <div>
                <h3 className="font-semibold text-slate-800 text-base">添加被测仓库</h3>
                <p className="text-[11px] text-slate-400 mt-0.5">纳入被测对象库 · 分步录入仓库/分支/版本与上游源接入</p>
              </div>
              <button type="button" onClick={() => setShowAdd(false)} className="w-8 h-8 rounded-lg hover:bg-slate-100 flex items-center justify-center text-slate-400"><X className="w-4 h-4" /></button>
            </div>

            {/* 分步导航 */}
            <div className="px-6 py-3 flex items-center gap-2 border-b border-slate-100">
              {ADD_STEPS.map((s, i) => (
                <div key={s} className="flex items-center gap-2">
                  <div className={'flex items-center gap-1.5 ' + (i === addStep ? 'text-emerald-600' : i < addStep ? 'text-emerald-500' : 'text-slate-400')}>
                    <span className={'w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-bold ' + (i === addStep ? 'bg-emerald-600 text-white' : i < addStep ? 'bg-emerald-100 text-emerald-600' : 'bg-slate-100')}>
                      {i < addStep ? <Check className="w-3 h-3" /> : i + 1}
                    </span>
                    <span className="text-xs font-medium">{s}</span>
                  </div>
                  {i < ADD_STEPS.length - 1 && <span className="w-6 h-px bg-slate-200" />}
                </div>
              ))}
            </div>

            <div className="px-6 py-5 min-h-[260px]">
              {/* 步骤1 基本信息 */}
              {addStep === 0 && (
                <div className="space-y-4">
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="text-xs text-slate-500 mb-1 block">仓库名 <span className="text-red-400">*</span></label>
                      <input value={addForm.name} onChange={(e) => setAddForm({ ...addForm, name: e.target.value })} placeholder="如 svc-notify"
                        className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                    </div>
                    <div>
                      <label className="text-xs text-slate-500 mb-1 block">代码源类型</label>
                      <select value={addForm.type} onChange={(e) => setAddForm({ ...addForm, type: e.target.value })}
                        className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white focus:outline-none focus:border-emerald-400">
                        <option>GitLab</option><option>GitHub</option><option>Gitee</option>
                      </select>
                    </div>
                    <div className="col-span-2">
                      <label className="text-xs text-slate-500 mb-1 block">仓库地址 <span className="text-red-400">*</span></label>
                      <input value={addForm.url} onChange={(e) => setAddForm({ ...addForm, url: e.target.value })} placeholder={urlPrefix + (addForm.name || 'repo')}
                        className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                    </div>
                    <div>
                      <label className="text-xs text-slate-500 mb-1 block">所属被测对象层级</label>
                      <select value={addForm.node} onChange={(e) => setAddForm({ ...addForm, node: e.target.value })}
                        className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white focus:outline-none focus:border-emerald-400">
                        <option>工程</option><option>服务组</option><option>服务</option><option>模块</option>
                      </select>
                    </div>
                    <div>
                      <label className="text-xs text-slate-500 mb-1 block">负责人</label>
                      <input value={addForm.owner} onChange={(e) => setAddForm({ ...addForm, owner: e.target.value })} placeholder="默认当前用户"
                        className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                    </div>
                  </div>
                  <div className="text-[11px] text-slate-400 bg-slate-50 rounded-lg px-3 py-2">代码源类型切换会联动仓库地址前缀（当前：{urlPrefix}…）。必填仓库名 + 地址后方可进入下一步。</div>
                </div>
              )}

              {/* 步骤2 代码源接入 */}
              {addStep === 1 && (
                <div className="space-y-4">
                  <div>
                    <label className="text-xs text-slate-500 mb-1 block">接入方式</label>
                    <div className="flex gap-2">
                      {['Access Token', 'SSH Key'].map((a) => (
                        <button key={a} type="button" onClick={() => setAddForm({ ...addForm, auth: a })}
                          className={'px-3 py-1.5 text-xs rounded-lg border ' + (addForm.auth === a ? 'bg-emerald-600 border-emerald-600 text-white' : 'border-slate-200 text-slate-500 hover:border-emerald-300')}>{a}</button>
                      ))}
                    </div>
                  </div>
                  <div>
                    <label className="text-xs text-slate-500 mb-1 block">{addForm.auth}</label>
                    <input value={addForm.token} onChange={(e) => setAddForm({ ...addForm, token: e.target.value })} placeholder={addForm.auth === 'Access Token' ? 'glpat-xxx' : 'ssh-ed25519 AAAA…'}
                      className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                  </div>
                  <button type="button" onClick={() => { if (!addForm.token.trim()) { toast('需填写凭据', { description: '请先填写接入凭据再检测连接' }); return; } setConnected(true); toast.success('连接成功 · 已获取分支列表'); }}
                    className={'px-3 py-1.5 text-xs rounded-lg border flex items-center gap-1 ' + (connected ? 'bg-emerald-50 border-emerald-200 text-emerald-600' : 'border-emerald-300 text-emerald-600 hover:bg-emerald-50')}>
                    <Plug className="w-3.5 h-3.5" />{connected ? '已连接 · 重新检测' : '检测连接'}
                  </button>
                  {connected && (
                    <div className="space-y-3">
                      <div>
                        <label className="text-xs text-slate-500 mb-1 block">默认分支</label>
                        <select value={addBranches[0] ?? 'main'} onChange={(e) => setAddBranches((p) => { const rest = p.filter((b) => b !== e.target.value); return [e.target.value, ...rest]; })}
                          className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white focus:outline-none focus:border-emerald-400">
                          {['main', 'develop', 'release', 'feature-refund-v2'].map((b) => <option key={b}>{b}</option>)}
                        </select>
                      </div>
                      <div>
                        <label className="text-xs text-slate-500 mb-1 block">维护分支（测试环境可能测试在指定分支）</label>
                        <div className="flex items-center gap-2 mb-2">
                          <input value={addBrInput} onChange={(e) => setAddBrInput(e.target.value)} placeholder="如 hotfix-1.0.1"
                            className="flex-1 text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                          <button type="button" onClick={() => { const b = addBrInput.trim(); if (b && !addBranches.includes(b)) setAddBranches((p) => [...p, b]); setAddBrInput(''); }}
                            className="text-xs px-2.5 py-1.5 border border-emerald-300 text-emerald-600 rounded-lg hover:bg-emerald-50">添加</button>
                        </div>
                        <div className="flex flex-wrap gap-2">
                          {addBranches.map((b) => (
                            <span key={b} className="flex items-center gap-1 text-[10px] bg-emerald-50 text-emerald-700 border border-emerald-200 px-2 py-0.5 rounded-md">
                              {b}<button type="button" onClick={() => setAddBranches((p) => p.filter((x) => x !== b))} className="text-emerald-400 hover:text-red-500"><X className="w-3 h-3" /></button>
                            </span>
                          ))}
                        </div>
                      </div>
                      <div>
                        <label className="text-xs text-slate-500 mb-1 block">目标版本</label>
                        <input value={addForm.ver} onChange={(e) => setAddForm({ ...addForm, ver: e.target.value })} placeholder="如 v1.0.0"
                          className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                      </div>
                    </div>
                  )}
                </div>
              )}

              {/* 步骤3 上游源 */}
              {addStep === 2 && (
                <div className="space-y-3">
                  <div className="text-xs text-slate-500">选择该被测对象接入的上游源（AI 生成用例的数据来源，至少选一类）</div>
                  <div className="grid grid-cols-2 md:grid-cols-3 gap-2.5">
                    {srcOptions.map((s) => {
                      const on = addSrc.includes(s);
                      return (
                        <button key={s} type="button" onClick={() => setAddSrc((p) => (on ? p.filter((x) => x !== s) : [...p, s]))}
                          className={'text-left border rounded-lg px-3 py-2.5 text-xs transition ' + (on ? 'bg-emerald-50 border-emerald-300' : 'border-slate-200 hover:border-emerald-300')}>
                          <div className="flex items-center justify-between">
                            <span className="font-medium text-slate-700">{s}</span>
                            <span className={'w-4 h-4 rounded border flex items-center justify-center ' + (on ? 'bg-emerald-600 border-emerald-600 text-white' : 'border-slate-300')}>{on && <Check className="w-3 h-3" />}</span>
                          </div>
                          <div className="text-[10px] text-slate-400 mt-1">{s === '需求文档' ? 'Jira' : s === '设计文档' ? 'Confluence' : s === 'API 契约' ? 'OpenAPI/Proto' : s === '代码变更' ? 'Git PR' : s === '生产追踪' ? 'APM Trace' : '缺陷库'}</div>
                        </button>
                      );
                    })}
                  </div>
                  <div className="text-[11px] text-slate-400">已选 <b className="text-emerald-600">{addSrc.length}</b> / {srcOptions.length} 类上游源</div>
                </div>
              )}

              {/* 步骤4 确认 */}
              {addStep === 3 && (
                <div className="space-y-3">
                  <div className="text-xs text-slate-500 mb-1">仓库将纳入被测对象库，并在「生成用例」作为生成输入。确认以下汇总：</div>
                  <div className="bg-slate-50 rounded-lg border border-slate-200 p-4 space-y-2 text-xs">
                    <div className="flex gap-2"><span className="text-slate-400 w-20 shrink-0">仓库</span><span className="font-medium text-slate-700">{addForm.name}@{addBranches[0] ?? 'main'}@{addForm.ver.trim() || 'v1.0.0'}</span></div>
                    <div className="flex gap-2"><span className="text-slate-400 w-20 shrink-0">代码源</span><span className="text-slate-700">{addForm.type} · {urlPrefix}{addForm.url || addForm.name}</span></div>
                    <div className="flex gap-2"><span className="text-slate-400 w-20 shrink-0">所属对象</span><span className="text-slate-700">{addForm.node} · {addForm.owner || '当前用户'}</span></div>
                    <div className="flex gap-2"><span className="text-slate-400 w-20 shrink-0">维护分支</span><span className="text-slate-700">{addBranches.length ? addBranches.join('、') : 'main'}</span></div>
                    <div className="flex gap-2"><span className="text-slate-400 w-20 shrink-0">上游源</span><span className="text-slate-700">{addSrc.length ? addSrc.join('、') : '—'}</span></div>
                  </div>
                </div>
              )}
            </div>

            <div className="px-6 py-4 border-t border-slate-200 flex items-center justify-between">
              <span className="text-[11px] text-slate-400">步骤 {addStep + 1} / {ADD_STEPS.length} · {ADD_STEPS[addStep]}</span>
              <div className="flex gap-2">
                {addStep > 0 && (
                  <button type="button" onClick={() => setAddStep((s) => s - 1)} className="px-3 py-1.5 text-xs border border-slate-200 rounded-lg text-slate-500 hover:bg-slate-50">上一步</button>
                )}
                {addStep < ADD_STEPS.length - 1 ? (
                  <button type="button" onClick={() => { if (!canNext) { toast('请完善必填项', { description: addStep === 0 ? '需填写仓库名与地址' : addStep === 1 ? '需检测连接成功' : '需至少接入一类上游源' }); return; } setAddStep((s) => s + 1); }}
                    className="flex items-center gap-1 px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg hover:bg-emerald-700">下一步 · {ADD_STEPS[addStep + 1]}<ChevronRight className="w-3.5 h-3.5" /></button>
                ) : (
                  <button type="button" onClick={submitAdd} className="flex items-center gap-1 px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg hover:bg-emerald-700"><Check className="w-3.5 h-3.5" />确认添加</button>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function RealTargetsPanel({ client }: { client: ReturnType<typeof configuredGreenPassClient> }) {
  const navigate = useNavigate();
  const catalog = useObjectCatalog();
  const { projects: targets, projectId: projectID, repos, services, repoId: selectedRepoID, refresh: refreshProject } = catalog;
  const [name, setName] = useState('');
  const [remark, setRemark] = useState('');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [messageType, setMessageType] = useState<'success'|'error'>('success');
  const [dialog,setDialog]=useState<'project'|'repos'|'services'|null>(null);
  const [repoDrafts,setRepoDrafts]=useState([{url:'',branch:'main'}]);
  const [serviceDrafts,setServiceDrafts]=useState([{repoID:'',name:'',path:'',kind:'api_service'}]);
  const [profileService, setProfileService] = useState<CatalogService | null>(null);
  const [profileDraft, setProfileDraft] = useState({ language: '', framework: '' });
  // The catalog API currently exposes a durable authorization transition, but
  // not a separate durable "profile reviewed" transition. Keep the two user
  // decisions separate in this dialog: saving a reviewed draft never enables
  // generation; authorization is a subsequent explicit action.
  const [profileReviewSaved, setProfileReviewSaved] = useState(false);
  const [requestedAnalysis, setRequestedAnalysis] = useState<number[]>([]);
  const analyzing = requestedAnalysis.length > 0 || services.some((service) => service.profile_status === 'analyzing');
  useEffect(() => {
    if (!analyzing || busy) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      const refreshed = await refreshProject();
      if (!cancelled) {
        if (!refreshed.error) setRequestedAnalysis((ids) => ids.filter((id) => refreshed.services.some((service) => service.id === id && (service.profile_status === 'pending' || service.profile_status === 'analyzing'))));
        timer = setTimeout(() => void poll(), 3000);
      }
    };
    timer = setTimeout(() => void poll(), 3000);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [analyzing, busy, projectID, refreshProject]);
  const openProfile = (service: CatalogService) => {
    setProfileService(service);
    setProfileDraft({ language: service.language, framework: service.framework });
    setProfileReviewSaved(service.profile_status === 'confirmed');
  };
  const run = async (action: () => Promise<void>) => { setBusy(true); setMessage(''); try { await action(); } catch (error) { setMessageType('error'); setMessage(error instanceof Error ? error.message : '真实 API 操作失败'); } finally { setBusy(false); } };
  const profileDraftComplete = Object.values(profileDraft).every((value) => value.trim());
  // A confirmed status is currently coupled to the server-side authorization
  // transition. Do not present it as an independently persisted review state.
  const profileAlreadyConfirmed = profileService?.profile_status === 'confirmed';
  const profileIsAuthorized = profileService?.generation_status === 'confirmed';
  const profileCanAuthorize = Boolean(profileService && profileDraftComplete && (profileAlreadyConfirmed || profileReviewSaved));
  const saveProfileReview = () => {
    const service = profileService;
    if (!service) return;
    void run(async () => {
      await client.updateProjectServiceProfile(service.project_id, service.id, {
        language: profileDraft.language.trim(), framework: profileDraft.framework.trim(),
      });
      await refreshProject();
      setProfileReviewSaved(true);
      setMessageType('success');
      setMessage(`${service.name} 的技术画像核验配置已保存，尚未授权生成。`);
    });
  };
  const authorizeProfileGeneration = () => {
    const service = profileService;
    if (!service) return;
    void run(async () => {
      if (!profileDraftComplete || (!profileAlreadyConfirmed && !profileReviewSaved)) throw new Error('请先保存并确认技术画像，再单独授权生成');
      await client.updateProjectServiceGenerationStatus(service.project_id, service.id, 'confirmed');
      await refreshProject();
      setRequestedAnalysis((ids) => ids.filter((id) => id !== service.id));
      catalog.selectService(service.id);
      setProfileService(null);
      setProfileReviewSaved(false);
      setMessageType('success');
      setMessage(`${service.name} 已完成技术画像确认，并已单独授权进入用例生成`);
    });
  };
  const selectedProject=targets.find((target)=>String(target.id)===projectID);
  const repoName=(repoID:number)=>{const repo=repos.find((item)=>item.id===repoID);return repo ? repo.url.split('/').pop()?.replace(/\.git$/,'') ?? String(repoID) : String(repoID);};
  const serviceKinds=[['api_service','后端服务'],['gateway','API 网关'],['websocket_service','WebSocket 服务'],['web_app','Web 应用'],['mobile_app','移动应用'],['desktop_app','桌面应用'],['other','其他']] as const;

  if (!client) return <Card title="真实被测对象控制" className="mb-5" extra={<span className="text-[11px] text-slate-400">原型模式</span>}><p className="p-5 text-sm text-slate-500">配置 <code className="rounded bg-slate-100 px-1.5 py-0.5">VITE_GP_API_BASE</code> 与数值型 <code className="rounded bg-slate-100 px-1.5 py-0.5">VITE_GP_TEAM_ID</code> 后启用真实对象树、创建节点和仓库绑定。</p></Card>;
  return <div className="space-y-5">
    <PageHeader title="被测对象" desc="按项目管理代码仓库与目录服务；选择服务后核验技术画像，再授权生成用例。">
      <button type="button" onClick={() => setDialog('project')} className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-medium text-slate-700 hover:border-emerald-300 hover:text-emerald-700"><Plus className="h-3.5 w-3.5" />新建项目</button>
    </PageHeader>
    <section className="rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div className="flex flex-col gap-4 border-b border-slate-100 px-5 py-5 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0"><div className="flex items-center gap-2 text-[11px] font-medium text-emerald-700"><FolderTree className="h-4 w-4" />项目范围</div><h2 className="mt-1 truncate text-xl font-semibold text-slate-800">{selectedProject?.name ?? '请选择项目'}</h2><p className="mt-1 text-xs text-slate-500">{selectedProject?.remark ?? '选择一个项目后查看其仓库与目录服务。'}</p></div>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3"><div className="rounded-lg bg-slate-50 px-4 py-2.5 text-center"><b className="block text-lg text-slate-800">{repos.length}</b><span className="text-[10px] text-slate-500">代码仓库</span></div><div className="rounded-lg bg-slate-50 px-4 py-2.5 text-center"><b className="block text-lg text-slate-800">{services.length}</b><span className="text-[10px] text-slate-500">目录服务</span></div><div className="hidden rounded-lg bg-slate-50 px-4 py-2.5 text-center sm:block"><b className="block text-lg text-slate-800">{services.filter((service) => servicePreparation(service) === 'ready').length}</b><span className="text-[10px] text-slate-500">可生成用例</span></div></div>
      </div>
      <div className="flex flex-col gap-3 bg-slate-50/70 px-5 py-3 lg:flex-row lg:items-center"><label className="min-w-0 flex-1"><span className="sr-only">当前项目</span><select value={projectID} onChange={(e) => catalog.selectProject(e.target.value)} aria-label="当前项目" disabled={catalog.loading} className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2.5 text-xs font-medium text-slate-700 outline-none focus:border-emerald-400"><option value="">选择项目</option>{targets.filter((target) => target.type === 'project').map((target) => <option key={target.id} value={target.id}>{target.name}</option>)}</select></label><div className="flex flex-wrap gap-2"><button type="button" disabled={!projectID} onClick={() => setDialog('repos')} className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2.5 text-xs font-medium text-slate-700 hover:border-emerald-300 hover:text-emerald-700 disabled:opacity-50"><GitBranch className="h-3.5 w-3.5" />添加仓库</button><button type="button" disabled={!projectID || repos.length === 0} onClick={() => setDialog('services')} className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3 py-2.5 text-xs font-semibold text-white hover:bg-emerald-700 disabled:opacity-50"><Files className="h-3.5 w-3.5" />登记服务</button></div></div>
    </section>

    {message&&<div className={`flex items-center gap-2 rounded-xl border px-4 py-3 text-xs ${messageType==='success'?'border-emerald-200 bg-emerald-50 text-emerald-700':'border-red-200 bg-red-50 text-red-700'}`}><CheckCircle2 className="h-4 w-4"/>{message}</div>}
    {catalog.error && <div role="alert" className="flex items-center justify-between rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>目录加载失败：{catalog.error}</span><button type="button" onClick={() => void refreshProject()} className="font-medium hover:underline">重试</button></div>}
    {profileService && <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/45 p-4" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) setProfileService(null); }}>
      <section role="dialog" aria-modal="true" aria-labelledby="profile-review-title" className="flex max-h-[88dvh] w-full max-w-3xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-200 px-6 py-4"><div><h3 id="profile-review-title" className="text-base font-semibold text-slate-800">核验 AI 技术画像 · {profileService.name}</h3><p className="mt-1 text-xs text-slate-500">先保存已核验的技术画像；生成授权是下一步独立决定，不会因保存画像自动开启。</p></div><button type="button" disabled={busy} aria-label="关闭技术画像" onClick={() => setProfileService(null)} className="rounded-lg p-2 text-slate-400 hover:bg-slate-100"><X className="h-4 w-4" /></button></div>
        <div className="min-h-0 space-y-5 overflow-auto p-6">
          <div className="grid gap-3 sm:grid-cols-3">
            <div className="rounded-xl border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600"><span className="block text-[10px] text-slate-400">分析范围</span><code className="mt-1 block break-all text-slate-700">{sourceDirectoryLabel(profileService.source_path)}</code></div>
            <div className="rounded-xl border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600"><span className="block text-[10px] text-slate-400">模型与来源</span><span className="mt-1 block break-all text-slate-700">{profileSourceLabel(profileService.profile_source)}</span></div>
            <div className="rounded-xl border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600"><span className="block text-[10px] text-slate-400">分析结果</span><span className="mt-1 block text-slate-700">{profileConfidencePercent(profileService) === null ? '置信度未提供' : `置信度 ${profileConfidencePercent(profileService)}%`} · {analysisTimeLabel(profileService.profile_analyzed_at)}</span></div>
          </div>
          <div className="rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-xs leading-5 text-sky-800"><strong>执行边界：</strong>测试工具和测试命令由系统从源码自动识别，仅供审查，不允许在这里填写或执行。GreenPass 当前执行已入库用例中的受限 HTTP 请求和断言；实际执行仍须在“测试执行”选择已登记环境。</div>
          <div><div className="mb-2 flex items-center justify-between gap-3"><div><h4 className="text-xs font-semibold text-slate-700">模型返回的源码证据</h4><p className="mt-1 text-[11px] text-slate-400">每项均绑定本次分析实际读取的配置文件。请核对文件内容和建议是否一致，再保存核验或授权生成。</p></div><span className={`shrink-0 rounded-full px-2 py-1 text-[10px] font-medium ${(profileService.profile_evidence?.length ?? 0) > 0 ? 'bg-emerald-50 text-emerald-700' : 'bg-red-50 text-red-700'}`}>{profileService.profile_evidence?.length ?? 0} 条证据</span></div>{profileService.profile_evidence?.length ? <ul className="space-y-2 text-xs leading-5 text-slate-600">{profileService.profile_evidence.map((evidence, index) => { const item = profileEvidenceParts(evidence); return <li key={index} className="break-words rounded-lg border border-slate-200 bg-white p-3"><span className="mr-2 inline-flex rounded bg-slate-100 px-1.5 py-0.5 font-mono text-[10px] text-slate-500">{item.file}</span>{item.reason}</li>; })}</ul> : <p className="rounded-lg bg-amber-50 p-3 text-xs text-amber-800">没有可核验的源码证据。请先重新分析，或由负责人依据代码仓库补充画像；不要仅凭模型建议授权生成。</p>}</div>
          {profileService.test_strategy && <div><h4 className="mb-2 text-xs font-semibold text-slate-700">AI 建议的测试策略</h4><p className="whitespace-pre-wrap rounded-xl bg-slate-50 p-4 text-xs leading-6 text-slate-600">{profileService.test_strategy}</p></div>}
          <div><div className="mb-3 flex flex-wrap items-center justify-between gap-2"><div><h4 className="text-xs font-semibold text-slate-700">负责人核验的技术配置</h4><p className="mt-1 text-[11px] text-slate-400">仅可修正语言和框架；测试工具与命令由系统识别，不能由用户伪造或补全。</p></div>{profileService.profile_status === 'failed' && <button type="button" disabled={busy} onClick={() => void run(async () => { await client.analyzeProjectServiceProfile(profileService.project_id, profileService.id); setRequestedAnalysis((ids) => [...ids.filter((id) => id !== profileService.id), profileService.id]); await refreshProject(); setProfileService(null); setMessageType('success'); setMessage(`${profileService.name} 已重新提交 AI 技术画像分析`); })} className="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs font-medium text-emerald-700 hover:bg-emerald-100"><RefreshCw className="h-3.5 w-3.5" />重新分析</button>}</div><div className="grid gap-4 sm:grid-cols-2">{PROFILE_FIELDS.map(([key, label, placeholder]) => <label key={key}><span className="mb-1.5 block text-xs font-medium text-slate-600">{label}</span><input value={profileDraft[key]} placeholder={placeholder} onChange={(event) => setProfileDraft((draft) => ({ ...draft, [key]: event.target.value }))} className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm outline-none focus:border-emerald-400" /></label>)}</div><div className="mt-4 grid gap-3 sm:grid-cols-2"><div className="rounded-lg border border-slate-200 bg-slate-50 p-3"><span className="block text-[10px] text-slate-400">系统识别的测试工具</span><code className="mt-1 block break-all text-xs text-slate-700">{profileService.test_tool || '未从当前源码证据识别到自动化测试工具'}</code></div><div className="rounded-lg border border-slate-200 bg-slate-50 p-3"><span className="block text-[10px] text-slate-400">系统识别的测试命令</span><code className="mt-1 block break-all text-xs text-slate-700">{profileService.test_command || '未发现可安全采用的项目测试命令'}</code></div></div></div>
          {messageType === 'error' && message && <p role="alert" className="rounded-lg bg-red-50 p-3 text-xs text-red-700">{message}</p>}
        </div>
        <div className="flex shrink-0 flex-col gap-3 border-t border-slate-200 bg-slate-50 px-6 py-4"><div className="flex flex-wrap items-start gap-2 text-[11px]"><span className={'rounded-full px-2 py-1 font-medium ' + (profileAlreadyConfirmed || profileReviewSaved ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-700')}>{profileAlreadyConfirmed || profileReviewSaved ? '步骤 1：技术画像已核验' : '步骤 1：请先保存画像核验'}</span><span className={'rounded-full px-2 py-1 font-medium ' + (profileIsAuthorized ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-500')}>{profileIsAuthorized ? '步骤 2：已授权生成' : '步骤 2：尚未授权生成'}</span></div><div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><div className="max-w-xl text-[11px] leading-5 text-slate-500"><p>保存画像核验只保存技术配置，不会开启生成。授权后才可在“生成用例”页选择此服务；之后仍可暂停授权，画像不会删除。</p><p className="mt-1 text-amber-700">当前服务端正式的“已确认”状态会在授权时落库；前端已把两个动作拆开，后端仍应独立持久化核验状态并强制状态序列。</p></div><div className="flex shrink-0 flex-wrap justify-end gap-2"><button type="button" disabled={busy} onClick={() => setProfileService(null)} className="rounded-lg border border-slate-200 bg-white px-4 py-2 text-xs text-slate-600">关闭</button><button type="button" disabled={busy || !profileDraftComplete} onClick={saveProfileReview} className="rounded-lg border border-emerald-200 bg-white px-4 py-2 text-xs font-medium text-emerald-700 hover:bg-emerald-50 disabled:opacity-50">{busy ? '保存中…' : profileAlreadyConfirmed || profileReviewSaved ? '更新画像核验' : '保存画像核验（不授权）'}</button>{!profileIsAuthorized && <button type="button" disabled={busy || !profileCanAuthorize} onClick={authorizeProfileGeneration} title={!profileCanAuthorize ? '请先保存画像核验' : undefined} className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-xs font-semibold text-white hover:bg-emerald-700 disabled:opacity-50"><ShieldCheck className="h-3.5 w-3.5" />授权生成用例</button>}</div></div></div>
      </section>
    </div>}

    <section className="rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div className="flex flex-col gap-2 border-b border-slate-100 px-5 py-4 sm:flex-row sm:items-center sm:justify-between"><div><h2 className="text-sm font-semibold text-slate-800">仓库与目录服务</h2><p className="mt-1 text-[11px] text-slate-500">先选择仓库，再处理其目录服务的技术画像与生成授权。</p></div><span className="text-[11px] text-slate-400">项目 → 仓库 → 服务</span></div>
      {repos.length === 0 && <div className="py-12 text-center text-xs text-slate-400">当前项目尚未接入仓库。请先添加仓库，再登记目录服务。</div>}
      <div className="divide-y divide-slate-100">{repos.map((repo) => { const repoServices = services.filter((service) => service.repo_id === repo.id); const selected = selectedRepoID === repo.id; return <section key={repo.id} className={selected ? 'bg-emerald-50/30' : ''}>
        <button type="button" aria-expanded={selected} aria-label={`${selected ? '收起' : '展开'}仓库 ${repoName(repo.id)} 的目录服务`} onClick={() => catalog.selectRepository(selected ? null : repo.id)} className="flex w-full items-center gap-3 px-5 py-4 text-left hover:bg-slate-50"><span className={'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ' + (selected ? 'bg-emerald-600 text-white' : 'bg-slate-100 text-slate-500')}><GitBranch className="h-4 w-4" /></span><span className="min-w-0 flex-1"><span className="block truncate text-sm font-semibold text-slate-800">{repoName(repo.id)}</span><span className="mt-0.5 block truncate font-mono text-[10px] text-slate-400">{repo.url}</span></span><span className="hidden text-right text-[11px] text-slate-500 sm:block">默认分支<br /><code className="text-slate-700">{repo.default_branch}</code></span><span className="rounded-full bg-slate-100 px-2.5 py-1 text-[11px] font-medium text-slate-600">{repoServices.length} 个服务</span><ChevronRight className={'h-4 w-4 text-slate-300 transition ' + (selected ? 'rotate-90 text-emerald-600' : '')} /></button>
        {selected && <div className="border-t border-emerald-100 bg-white px-5 py-4"><div className="mb-3 flex flex-wrap items-center justify-between gap-2"><div className="flex items-center gap-2 text-xs font-medium text-slate-700"><Box className="h-4 w-4 text-emerald-600" />目录服务 <span className="font-normal text-slate-400">按「登记 → 分析 → 核验 → 授权」推进</span></div><button type="button" onClick={() => setDialog('services')} className="text-xs font-medium text-emerald-700 hover:underline">为此仓库登记服务</button></div><div className="space-y-3">{repoServices.map((service) => <ServicePreparationCard
          key={service.id}
          service={service}
          repoName={repoName}
          busy={busy}
          serviceKinds={serviceKinds}
          onKindChange={(kind) => void run(async () => {
            await client.updateProjectServiceKind(Number(projectID), service.id, kind);
            await refreshProject();
            setMessageType('success');
            setMessage(`${service.name} 的对象类型已更新`);
          })}
          onAnalyze={() => void run(async () => {
            await client.analyzeProjectServiceProfile(Number(projectID), service.id);
            setRequestedAnalysis((ids) => [...ids.filter((id) => id !== service.id), service.id]);
            await refreshProject();
            setMessageType('success');
            setMessage(`${service.name} 已提交 AI 技术画像分析，结果会自动更新`);
          })}
          onOpenProfile={() => openProfile(service)}
          onOpenGeneration={() => { catalog.selectService(service.id); navigate('/generation'); }}
          onRevokeGeneration={() => void run(async () => {
            await client.updateProjectServiceGenerationStatus(Number(projectID), service.id, 'pending');
            await refreshProject();
            setMessageType('success');
            setMessage(`${service.name} 的用例生成授权已暂停；技术画像仍保留。`);
          })}
        />)}{repoServices.length === 0 && <div className="rounded-lg border border-dashed border-slate-200 py-8 text-center text-xs text-slate-400">该仓库尚未登记目录服务。</div>}</div></div>}
      </section>; })}</div>
    </section>

    {dialog&&<div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/45 p-4" onMouseDown={(e)=>{if(e.target===e.currentTarget&&!busy)setDialog(null);}}><div className="max-h-[88vh] w-full max-w-5xl overflow-hidden rounded-2xl bg-white shadow-2xl"><div className="flex items-center justify-between border-b border-slate-200 px-6 py-4"><div><h3 className="text-base font-semibold text-slate-800">{dialog==='project'?'新建被测项目':dialog==='repos'?'批量添加代码仓库':'批量登记目录子服务'}</h3><p className="mt-0.5 text-[11px] text-slate-400">{dialog==='project'?'创建后即可继续添加仓库':dialog==='repos'?`添加到 ${selectedProject?.name ?? '当前项目'}`:'选择仓库并填写实际源码目录，不需要记忆仓库 ID'}</p></div><button type="button" disabled={busy} onClick={()=>setDialog(null)} className="rounded-lg p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-600"><X className="h-4 w-4"/></button></div><div className="max-h-[65vh] overflow-auto p-6">
      {dialog==='project'&&<div className="space-y-4"><div><label className="mb-1.5 block text-xs font-medium text-slate-600">项目名称</label><input autoFocus value={name} onChange={e=>setName(e.target.value)} placeholder="例如 im-saas" className="w-full rounded-lg border border-slate-200 px-3 py-2.5 text-sm outline-none focus:border-emerald-400"/></div><div><label className="mb-1.5 block text-xs font-medium text-slate-600">项目备注</label><textarea value={remark} maxLength={500} onChange={e=>setRemark(e.target.value)} placeholder="说明项目用途、系统范围或测试重点" rows={4} className="w-full resize-none rounded-lg border border-slate-200 px-3 py-2.5 text-sm outline-none focus:border-emerald-400"/><div className="mt-1 text-right text-[10px] text-slate-400">{remark.length}/500</div></div></div>}
      {dialog==='repos'&&<div className="space-y-3"><div className="grid grid-cols-[1fr_180px_36px] gap-2 px-1 text-[11px] text-slate-400"><span>仓库 URL</span><span>默认分支</span><span/></div>{repoDrafts.map((draft,index)=><div key={index} className="grid grid-cols-[1fr_180px_36px] gap-2"><input value={draft.url} onChange={e=>setRepoDrafts(rows=>rows.map((row,i)=>i===index?{...row,url:e.target.value}:row))} placeholder="https://github.com/org/repo.git" className="rounded-lg border border-slate-200 px-3 py-2 text-xs outline-none focus:border-emerald-400"/><input value={draft.branch} onChange={e=>setRepoDrafts(rows=>rows.map((row,i)=>i===index?{...row,branch:e.target.value}:row))} placeholder="main" className="rounded-lg border border-slate-200 px-3 py-2 font-mono text-xs outline-none focus:border-emerald-400"/><button type="button" disabled={repoDrafts.length===1} onClick={()=>setRepoDrafts(rows=>rows.filter((_,i)=>i!==index))} className="rounded-lg text-slate-400 hover:bg-red-50 hover:text-red-500 disabled:opacity-30"><X className="mx-auto h-4 w-4"/></button></div>)}<button type="button" onClick={()=>setRepoDrafts(rows=>[...rows,{url:'',branch:'main'}])} className="flex items-center gap-1 text-xs text-emerald-600 hover:underline"><Plus className="h-3.5 w-3.5"/>增加一行</button></div>}
      {dialog==='services'&&<div className="space-y-3"><div className="grid grid-cols-[190px_1fr_1fr_150px_36px] gap-2 px-1 text-[11px] text-slate-400"><span>所属仓库</span><span>服务名称</span><span>源码目录</span><span>类型</span><span/></div>{serviceDrafts.map((draft,index)=><div key={index} className="grid grid-cols-[190px_1fr_1fr_150px_36px] gap-2"><select value={draft.repoID} onChange={e=>setServiceDrafts(rows=>rows.map((row,i)=>i===index?{...row,repoID:e.target.value}:row))} className="rounded-lg border border-slate-200 px-2 py-2 text-xs outline-none focus:border-emerald-400"><option value="">选择仓库</option>{repos.map(repo=><option key={repo.id} value={repo.id}>{repoName(repo.id)}</option>)}</select><input value={draft.name} onChange={e=>setServiceDrafts(rows=>rows.map((row,i)=>i===index?{...row,name:e.target.value}:row))} placeholder="user-service" className="rounded-lg border border-slate-200 px-3 py-2 text-xs outline-none focus:border-emerald-400"/><input value={draft.path} onChange={e=>setServiceDrafts(rows=>rows.map((row,i)=>i===index?{...row,path:e.target.value}:row))} placeholder="services/user" className="rounded-lg border border-slate-200 px-3 py-2 font-mono text-xs outline-none focus:border-emerald-400"/><select value={draft.kind} onChange={e=>setServiceDrafts(rows=>rows.map((row,i)=>i===index?{...row,kind:e.target.value}:row))} className="rounded-lg border border-slate-200 px-2 py-2 text-xs outline-none focus:border-emerald-400">{serviceKinds.map(([value,label])=><option key={value} value={value}>{label}</option>)}</select><button type="button" disabled={serviceDrafts.length===1} onClick={()=>setServiceDrafts(rows=>rows.filter((_,i)=>i!==index))} className="rounded-lg text-slate-400 hover:bg-red-50 hover:text-red-500 disabled:opacity-30"><X className="mx-auto h-4 w-4"/></button></div>)}<button type="button" onClick={()=>setServiceDrafts(rows=>[...rows,{repoID:'',name:'',path:'',kind:'api_service'}])} className="flex items-center gap-1 text-xs text-emerald-600 hover:underline"><Plus className="h-3.5 w-3.5"/>增加一行</button></div>}
    </div><div className="flex items-center justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-4"><button type="button" disabled={busy} onClick={()=>setDialog(null)} className="rounded-lg border border-slate-200 bg-white px-4 py-2 text-xs text-slate-600 hover:bg-slate-50">取消</button><button type="button" disabled={busy||(dialog==='project'&&!name.trim())||(dialog==='repos'&&!repoDrafts.some(r=>r.url.trim()))||(dialog==='services'&&!serviceDrafts.some(s=>s.repoID&&s.name.trim()&&s.path.trim()))} onClick={()=>void run(async()=>{if(dialog==='project'){const created=await client.createTarget({name:name.trim(),remark:remark.trim(),type:'project',kind:'project'});await refreshProject();catalog.selectProject(String(created.id));setName('');setRemark('');setMessageType('success');setMessage('项目创建成功');}else if(dialog==='repos'){const rows=repoDrafts.filter(r=>r.url.trim()).map(r=>({url:r.url.trim(),default_branch:r.branch.trim()||'main'}));const result=await client.batchAddProjectRepositories(Number(projectID),rows);const failed=result.results.filter(v=>v.error);setMessageType(failed.length?'error':'success');setMessage(failed.length?`${rows.length-failed.length} 条成功，${failed.length} 条失败`:`${rows.length} 个仓库全部添加成功`);setRepoDrafts([{url:'',branch:'main'}]);await refreshProject();}else{const rows=serviceDrafts.filter(s=>s.repoID&&s.name.trim()&&s.path.trim()).map(s=>({repo_id:Number(s.repoID),name:s.name.trim(),source_path:s.path.trim(),kind:s.kind,build_context:s.path.trim()}));const result=await client.batchAddProjectServices(Number(projectID),rows);setRequestedAnalysis((ids)=>[...ids,...result.results.flatMap((item)=>item.id?[item.id]:[])]);const failed=result.results.filter(v=>v.error);setMessageType(failed.length?'error':'success');setMessage(failed.length?`${rows.length-failed.length} 条成功，${failed.length} 条失败`:`${rows.length} 个子服务全部创建成功`);setServiceDrafts([{repoID:'',name:'',path:'',kind:'api_service'}]);await refreshProject();}setDialog(null);})} className="rounded-lg bg-emerald-600 px-4 py-2 text-xs font-medium text-white hover:bg-emerald-700 disabled:opacity-50">{busy?'提交中…':'确认提交'}</button></div></div></div>}
  </div>;
}

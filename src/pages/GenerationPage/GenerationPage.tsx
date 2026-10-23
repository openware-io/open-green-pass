import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';
import { UPSTREAM_ADAPTERS, TESTED_REPOS, type IAssetNode } from '@/data/mock';
import { PageHeader, GhostButton, Card, ListFilter } from '@/components/shared';
import { toast } from 'sonner';
import { Sparkles, Plug, ShieldCheck, UserCheck, Database, GitBranch, BookOpen, CloudDownload, ChevronLeft, ChevronRight, Check, Crosshair, Layers } from 'lucide-react';
import { configuredGreenPassClient } from '@/api/runtime';
import { GreenPassApiError, type GenerationBatch, type GenerationRun, type RepositoryRef } from '@/api/client';
import { loginStore } from '@/context/login';
import { useObjectCatalog } from '@/context/objectCatalog';
type GenerationSeed = { id: number; batch_id: number; case_id?: number | null; source_locator: string; normalized_spec: Record<string, unknown>; quality_state: string; approval_state: string; dedupe_hash?: string };

// A repository root is stored as "." for the source reader, but that is an
// implementation detail and should never be the primary UI label. Native
// select options are single-line, so use explicit Chinese wording instead of
// a punctuation-heavy concatenation of fields.
function sourceDirectoryLabel(sourcePath: string) {
  return !sourcePath || sourcePath === '.' ? '仓库根目录' : sourcePath;
}

function CodeRefSelect({ refs, value, disabled, onChange, label = '代码引用' }: {
  refs: RepositoryRef[];
  value?: { type: 'branch' | 'tag'; name: string };
  disabled?: boolean;
  onChange: (next: { type: 'branch' | 'tag'; name: string }) => void;
  label?: string;
}) {
  const type = value?.type ?? 'branch';
  const options = refs.filter((ref) => ref.ref_type === type);
  return <div className="flex min-w-0 gap-2">
    <select
      aria-label={`${label}类型`}
      value={type}
      disabled={disabled || !refs.length}
      onChange={(event) => {
        const nextType = event.target.value as 'branch' | 'tag';
        const first = refs.find((ref) => ref.ref_type === nextType);
        onChange({ type: nextType, name: first?.name ?? '' });
      }}
      className="rounded-md border border-slate-200 bg-white px-2 py-1.5 text-xs text-slate-700 disabled:opacity-50"
    >
      <option value="branch">分支</option>
      <option value="tag">Tag</option>
    </select>
    <select
      aria-label={label}
      value={value?.name ?? ''}
      disabled={disabled || !options.length}
      onChange={(event) => onChange({ type, name: event.target.value })}
      className="min-w-0 flex-1 rounded-md border border-slate-200 bg-white px-2 py-1.5 font-mono text-xs text-slate-700 disabled:opacity-50"
    >
      <option value="">{refs.length ? '请选择代码引用' : '尚未同步，请刷新'}</option>
      {options.map((ref) => <option key={`${ref.ref_type}:${ref.name}`} value={ref.name}>{ref.name}</option>)}
    </select>
  </div>;
}

const SEED_QUALITY_LABEL: Record<string, string> = {
  pending: '待校验',
  passed: '通过',
  failed: '未通过',
};

const SEED_APPROVAL_LABEL: Record<string, string> = {
  pending: '待审核',
  approved: '已入库',
  rejected: '已拒绝',
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
}

function stableValue(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(stableValue);
  if (isRecord(value)) return Object.fromEntries(Object.keys(value).sort().map((key) => [key, stableValue(value[key])]));
  return value;
}

// Batch/model metadata, presentation and assertions are not case identity.
// This mirrors the server's stable request-intent key, so a changed assertion
// is shown as an update instead of a removal plus a new case.
function seedFingerprint(seed: GenerationSeed): string {
  const spec = seed.normalized_spec ?? {};
  const request = isRecord(spec.request) ? spec.request : {};
  return JSON.stringify(stableValue({ scenario: spec.scenario, method: request.method, path: request.path, headers: request.headers, body: request.body }));
}

function GeneratedSeedReviewList({ seeds }: { seeds: GenerationSeed[] }) {
  return (
    <div className="space-y-2">
      {seeds.map((seed) => {
        const spec = seed.normalized_spec ?? {};
        const request = isRecord(spec.request) ? spec.request : {};
        const method = stringValue(request.method) || '—';
        const path = stringValue(request.path) || '—';
        const headers = isRecord(request.headers) ? request.headers : {};
        const assertions = Array.isArray(spec.assertions) ? spec.assertions.filter(isRecord) : [];
        const steps = stringList(spec.steps);
        const preconditions = stringList(spec.preconditions);
        const body = request.body;
        return (
          <details key={seed.id} className="group rounded-lg border border-slate-200 bg-white open:border-emerald-200">
            <summary className="flex cursor-pointer list-none flex-wrap items-center gap-2 px-3 py-2.5 text-xs">
              <span className="font-mono text-emerald-700">#{seed.id}</span>
              <span className="min-w-0 flex-1 truncate font-medium text-slate-700">{stringValue(spec.title) || '未命名候选'}</span>
              <span className="rounded bg-slate-100 px-2 py-1 font-mono text-[10px] text-slate-600">{method} {path}</span>
              <span className={'rounded px-2 py-1 text-[10px] ' + (seed.quality_state === 'passed' ? 'bg-emerald-50 text-emerald-700' : seed.quality_state === 'failed' ? 'bg-red-50 text-red-700' : 'bg-amber-50 text-amber-700')}>质量{SEED_QUALITY_LABEL[seed.quality_state] ?? seed.quality_state}</span>
              <span className="rounded bg-slate-100 px-2 py-1 text-[10px] text-slate-600">{SEED_APPROVAL_LABEL[seed.approval_state] ?? seed.approval_state}</span>
              <span className="text-[10px] text-slate-400 group-open:hidden">展开详情</span>
              <span className="hidden text-[10px] text-slate-400 group-open:inline">收起</span>
            </summary>
            <div className="grid gap-3 border-t border-slate-100 p-3 text-xs md:grid-cols-2">
              <div className="space-y-3">
                <section>
                  <div className="mb-1 font-medium text-slate-600">前置条件</div>
                  {preconditions.length ? <ul className="list-inside list-disc space-y-1 text-slate-500">{preconditions.map((item, index) => <li key={index}>{item}</li>)}</ul> : <span className="text-slate-400">无</span>}
                </section>
                <section>
                  <div className="mb-1 font-medium text-slate-600">执行步骤</div>
                  {steps.length ? <ol className="list-inside list-decimal space-y-1 text-slate-500">{steps.map((item, index) => <li key={index}>{item}</li>)}</ol> : <span className="text-slate-400">无</span>}
                </section>
                <section>
                  <div className="mb-1 font-medium text-slate-600">预期结果</div>
                  <p className="whitespace-pre-wrap text-slate-500">{stringValue(spec.expected) || '未提供'}</p>
                </section>
              </div>
              <div className="space-y-3">
                <section>
                  <div className="mb-1 font-medium text-slate-600">请求与断言</div>
                  <div className="rounded-md bg-slate-50 p-2 font-mono text-[11px] text-slate-600">{method} {path}</div>
                  {Object.keys(headers).length > 0 && <pre className="mt-1 overflow-auto rounded-md bg-slate-50 p-2 text-[10px] text-slate-500">{JSON.stringify(headers, null, 2)}</pre>}
                  {body !== undefined && body !== null && <pre className="mt-1 max-h-40 overflow-auto rounded-md bg-slate-50 p-2 text-[10px] text-slate-500">{JSON.stringify(body, null, 2)}</pre>}
                  <div className="mt-2 flex flex-wrap gap-1.5">{assertions.map((assertion, index) => <span key={index} className="rounded bg-blue-50 px-2 py-1 text-[10px] text-blue-700">{stringValue(assertion.type) === 'status' ? `状态码 = ${String(assertion.expected_status ?? '—')}` : stringValue(assertion.type) === 'body_contains' ? `响应包含“${stringValue(assertion.value)}”` : stringValue(assertion.type)}</span>)}</div>
                </section>
                <div className="break-all text-[10px] text-slate-400">源码证据：{stringValue(spec.source_locator) || '未标注'}<br />生成记录：{seed.source_locator}</div>
              </div>
            </div>
          </details>
        );
      })}
    </div>
  );
}

const QUALITY = [
  { label: '变异测试验证', val: '38 / 48', width: 79, color: 'bg-emerald-500', note: '10 条未通过，无法杀死代码变体' },
  { label: '断言强度检查', val: '42 / 48', width: 87, color: 'bg-emerald-500', note: '6 条存在弱断言嫌疑' },
  { label: '重复检测', val: '46 / 48', width: 96, color: 'bg-emerald-500', note: '2 条与已有用例语义重复' },
  { label: '追溯绑定', val: '48 / 48', width: 100, color: 'bg-emerald-500', note: '全部绑定上游源' },
];

// AI 决策痕迹：展示 AI 如何理解上游源并产出用例种子
const AI_TRACES = [
  { step: '解析', desc: 'AI 读取 OpenAPI /v2/refund 变更 diff，提取新增字段 refund_id、移除 status', out: '识别 1 处破坏性变更' },
  { step: '生成', desc: 'AI 依据契约 + 历史缺陷模式，生成 12 条用例种子覆盖正/反/边界路径', out: '12 条种子' },
  { step: '筛选', desc: 'AI 计算种子断言强度，剔除 6 条弱断言嫌疑，保留强断言', out: '保留 12 条' },
  { step: '绑定', desc: 'AI 为每条种子建立 REQ→测试点→用例 追溯链', out: '追溯完整' },
];

// 生成流程向导步骤（围绕当前被测对象）
const FLOW_STEPS = [
  { key: 'repo', label: '当前被测对象', icon: Crosshair, meta: '生成流程的轴心对象' },
  { key: 'adapter', label: '适配解析', icon: Plug, meta: '六类上游源 → 统一格式' },
  { key: 'seed', label: 'AI 生成种子', icon: Sparkles, meta: '读取代码与文档生成' },
  { key: 'quality', label: '质量验证', icon: ShieldCheck, meta: '变异 + 断言 + 重复' },
  { key: 'review', label: '人工审核', icon: UserCheck, meta: '逐条审核用例' },
  { key: 'storage', label: '用例入库', icon: Database, meta: '确认入库 · 批次记录' },
];
const FLOW_ICON: Record<string, typeof GitBranch> = Object.fromEntries(FLOW_STEPS.map((s) => [s.key, s.icon]));
// 人工审核示例清单
const AUDIT_ROWS = [
  { id: 'SD-2401', scene: '退款金额含优惠券 · 正路径', assertion: '强', from: 'REQ-104 + refund-v2' },
  { id: 'SD-2402', scene: '退款金额含优惠券 · 金额边界', assertion: '强', from: 'REQ-104 + OpenAPI' },
  { id: 'SD-2403', scene: '支付超时自动撤销 · 正常流程', assertion: '强', from: 'REQ-104 + 状态机' },
  { id: 'SD-2404', scene: '支付超时自动撤销 · 幂等重试', assertion: '中', from: 'REQ-104 + 生产追踪' },
];

const TYPE_LABEL: Record<string, string> = { service: '服务', 'service-group': '服务组', module: '模块', app: '应用', end: '端', project: '工程' };
const STAGE_LABEL: Record<string, string> = { enqueued: '待启动', fetching: '冻结源码', parsing: '解析归一化', generating: '生成候选', quality_check: '质量检查', review: '人工审核', committing: '写入用例库', approved: '已完成', rejected: '已拒绝', failed: '失败', rolled_back: '已回退' };

export default function GenerationPage() {
  const { projectId } = useObjectCatalog();
  return <GenerationPageContent key={projectId} />;
}

function GenerationPageContent() {
  const navigate = useNavigate();
  const { selectedAsset } = useOutletContext<{ selectedAsset: IAssetNode }>();
  const realClient = useMemo(() => configuredGreenPassClient(), []);
  const catalog = useObjectCatalog();
  const { repos: realProjectRepos, services: realProjectServices, serviceId: activeServiceId, projectId, selectService } = catalog;
  const realCatalogLoading = Boolean(realClient && catalog.loading);
  const [realError, setRealError] = useState('');
  const [q, setQ] = useState('');
  const kw = q.trim().toLowerCase();
  const upFiltered = UPSTREAM_ADAPTERS.filter((a) => !kw || (a.source + a.status).toLowerCase().includes(kw));

  // 被测对象 = 全局轴心（页首被测对象导航切换），生成流程围绕它展开
  const mockRepo = TESTED_REPOS.find((r) => r.name === selectedAsset.name) ?? null;
  const selectedService = realProjectServices.find((service) => service.id === activeServiceId)
    ?? null;
  const selectedServiceReady = Boolean(
    selectedService
      && selectedService.generation_status === 'confirmed'
      && selectedService.profile_status === 'confirmed'
      && [selectedService.language, selectedService.framework]
        .every((value) => Boolean(value?.trim())),
  );
  const displayName = realClient ? (selectedService?.name ?? '请选择目录服务') : selectedAsset.name;
  const realRepo = realProjectRepos.find((item) => item.id === selectedService?.repo_id) ?? null;
  const realRepoID = realRepo?.id;
  const repo = realClient
    ? (realRepo ? { name: selectedService?.name ?? selectedAsset.name, source: '代码仓库', url: realRepo.url, branches: [realRepo.default_branch], versions: [] as string[], src: [] as string[], owner: '' } : null)
    : mockRepo;
  const [genBranch, setGenBranch] = useState(() => repo?.branches?.[0] ?? '');
  const [genVer, setGenVer] = useState(() => repo?.versions?.[0] ?? '');
  const generationContextKey = [selectedAsset.id, selectedService?.id ?? '', realRepo?.id ?? '', repo?.branches?.[0] ?? '', repo?.versions?.[0] ?? ''].join(':');
  const [previousGenerationContextKey, setPreviousGenerationContextKey] = useState(generationContextKey);
  if (previousGenerationContextKey !== generationContextKey) {
    setPreviousGenerationContextKey(generationContextKey);
    setGenBranch(repo?.branches?.[0] ?? '');
    setGenVer(repo?.versions?.[0] ?? '');
  }

  // 生成流程向导状态
  const [step, setStep] = useState(1);
  const [stored, setStored] = useState(false);
  const [audit, setAudit] = useState<Record<string, string>>({});
  const [selectedBatchId, setSelectedBatchId] = useState<number | null>(null);
  const [realBatches, setRealBatches] = useState<GenerationBatch[]>([]);
  const [realRun, setRealRun] = useState<GenerationRun | null>(null);
  const reviewableRunUnits = realRun ? realRun.units.filter((unit) => unit.batch_id && realBatches.find((batch) => batch.id === unit.batch_id)?.state === 'review') : [];
  const [repositoryRefs, setRepositoryRefs] = useState<Record<number, RepositoryRef[]>>({});
  const [selectedRefs, setSelectedRefs] = useState<Record<number, { type: 'branch' | 'tag'; name: string }>>({});
  const [refsLoading, setRefsLoading] = useState(false);
  const [scmConnections, setSCMConnections] = useState<Array<{id:number;name:string;credential_status:'configured'|'verified'|'failed'}>>([]);
  const visibleBatches = useMemo(() => realClient ? realBatches : activeServiceId ? realBatches.filter((batch) => batch.service_id === activeServiceId) : realBatches, [realBatches, realClient, activeServiceId]);
  const realBatch = visibleBatches.find((batch) => batch.id === selectedBatchId) ?? visibleBatches[0] ?? null;
  const activeBatchId = realBatch?.id;
  const activeBatchStateVersion = realBatch?.state_version;
  const [selectedServiceIds, setSelectedServiceIds] = useState<number[]>([]);
  const [generationScope, setGenerationScope] = useState<'project' | 'repo' | 'custom'>('project');
  const [repoScopeId, setRepoScopeId] = useState<number | null>(null);
  const [serviceSearch, setServiceSearch] = useState('');
  const [batchConfirmOpen, setBatchConfirmOpen] = useState(false);
  const [batchPreparing, setBatchPreparing] = useState(false);
  const [batchResults, setBatchResults] = useState<Array<{ serviceId: number; name: string; ok: boolean; error?: string }>>([]);
  const [realBusy, setRealBusy] = useState(false);
  const [seedResponse, setSeedResponse] = useState<{ batchId: number; stateVersion: number; items: GenerationSeed[]; error?: string } | null>(null);
  const seedsMatch = Boolean(realBatch && seedResponse?.batchId === realBatch.id && seedResponse.stateVersion === realBatch.state_version);
  const seedsLoading = Boolean(realBatch && !seedsMatch);
  const seedsError = seedsMatch ? seedResponse?.error ?? '' : '';
  const realSeeds = useMemo(() => (seedsMatch && !seedsError ? seedResponse?.items.filter((seed) => seed.batch_id === realBatch?.id) ?? [] : []), [seedsError, seedsMatch, seedResponse?.items, realBatch?.id]);
  const [reviewBusy, setReviewBusy] = useState(false);
  const [baselineSeeds, setBaselineSeeds] = useState<GenerationSeed[] | null>(null);
  const [baselineSeedsError, setBaselineSeedsError] = useState('');
  const canReview = Boolean(realBatch?.state === 'review' && seedsMatch && !seedsError && realSeeds.length > 0 && !reviewBusy);
  const cur = FLOW_STEPS[step - 1];
  const confirmedServices = useMemo(() => realProjectServices.filter((service) => service.generation_status === 'confirmed' && service.profile_status === 'confirmed'), [realProjectServices]);
  const visibleServices = useMemo(() => {
    const term = serviceSearch.trim().toLowerCase();
    return confirmedServices.filter((service) => !term || `${service.name} ${service.source_path}`.toLowerCase().includes(term));
  }, [confirmedServices, serviceSearch]);
  const profileFor = (service: typeof confirmedServices[number]) => {
    const defaults: Record<string, { language: string; tool: string; framework: string }> = {
      api_service: { language: '未登记', tool: '未登记', framework: '未登记' },
      gateway: { language: '未登记', tool: '未登记', framework: '未登记' },
      websocket_service: { language: '未登记', tool: '未登记', framework: '未登记' },
      web_app: { language: '未登记', tool: '未登记', framework: '未登记' },
      mobile_app: { language: '未登记', tool: '未登记', framework: '未登记' },
      desktop_app: { language: '未登记', tool: '未登记', framework: '未登记' },
    };
    const fallback = defaults[service.kind] ?? defaults.api_service;
    const language = service.language || fallback.language;
    const framework = service.framework || fallback.framework;
    const tool = service.test_tool || fallback.tool;
    const command = service.test_command || '未登记';
    return { language, framework, tool, command, complete: [service.language, service.framework].every(Boolean) };
  };
  const repoScopeOptions = useMemo(() => {
    const map = new Map<number, { id: number; url: string; defaultBranch: string; serviceCount: number }>();
    for (const service of confirmedServices) {
      const repo = realProjectRepos.find((item) => item.id === service.repo_id);
      if (!repo) continue;
      const current = map.get(repo.id);
      if (current) current.serviceCount += 1;
      else map.set(repo.id, { id: repo.id, url: repo.url, defaultBranch: repo.default_branch, serviceCount: 1 });
    }
    return [...map.values()].sort((a, b) => b.serviceCount - a.serviceCount || a.url.localeCompare(b.url));
  }, [confirmedServices, realProjectRepos]);
  const activeRepoScopeId = repoScopeId ?? repoScopeOptions[0]?.id ?? null;
  const scopeServices = useMemo(() => {
    if (generationScope === 'project') return confirmedServices;
    if (generationScope === 'repo') return confirmedServices.filter((service) => service.repo_id === activeRepoScopeId);
    return confirmedServices.filter((service) => selectedServiceIds.includes(service.id));
  }, [confirmedServices, generationScope, activeRepoScopeId, selectedServiceIds]);
  const fullScope = generationScope !== 'custom';
  const selectedServices = scopeServices;
  const activeRepoRefs = realRepo ? repositoryRefs[realRepo.id] ?? [] : [];
  const selectedRef = realRepo ? selectedRefs[realRepo.id] : undefined;
  const selectedRefDetails = activeRepoRefs.find((ref) => ref.ref_type === selectedRef?.type && ref.name === selectedRef.name) ?? null;
  const candidateDiff = useMemo(() => {
    if (!baselineSeeds) return null;
    const current = new Set(realSeeds.map(seedFingerprint));
    const base = new Set(baselineSeeds.map(seedFingerprint));
    let added = 0; let unchanged = 0; let removed = 0;
    current.forEach((fingerprint) => { if (base.has(fingerprint)) unchanged += 1; else added += 1; });
    base.forEach((fingerprint) => { if (!current.has(fingerprint)) removed += 1; });
    return { added, unchanged, removed };
  }, [baselineSeeds, realSeeds]);
  const loadRepositoryRefs = useCallback(async (repoId: number) => {
    if (!realClient || !projectId) return [] as RepositoryRef[];
    const refs = await realClient.listRepositoryRefs(Number(projectId), repoId);
    setRepositoryRefs((current) => ({ ...current, [repoId]: refs }));
    setSelectedRefs((current) => {
      if (current[repoId] && refs.some((ref) => ref.ref_type === current[repoId].type && ref.name === current[repoId].name)) return current;
      const repo = realProjectRepos.find((item) => item.id === repoId);
      const preferred = refs.find((ref) => ref.ref_type === 'branch' && ref.name === repo?.default_branch) ?? refs[0];
      return preferred ? { ...current, [repoId]: { type: preferred.ref_type, name: preferred.name } } : current;
    });
    return refs;
  }, [projectId, realClient, realProjectRepos]);
  const refreshRepositoryRefs = async () => {
    if (!realClient || !projectId || !realRepo) return;
    setRefsLoading(true); setRealError('');
    try {
      const refs = await realClient.refreshRepositoryRefs(Number(projectId), realRepo.id);
      setRepositoryRefs((current) => ({ ...current, [realRepo.id]: refs }));
      setSelectedRefs((current) => {
        const currentRef = current[realRepo.id];
        if (currentRef && refs.some((ref) => ref.ref_type === currentRef.type && ref.name === currentRef.name)) return current;
        const fallback = refs.find((ref) => ref.ref_type === 'branch' && ref.name === realRepo.default_branch) ?? refs[0];
        return fallback ? { ...current, [realRepo.id]: { type: fallback.ref_type, name: fallback.name } } : current;
      });
      toast.success('代码引用已刷新', { description: `已同步 ${refs.filter((ref) => ref.ref_type === 'branch').length} 个分支和 ${refs.filter((ref) => ref.ref_type === 'tag').length} 个 Tag` });
    } catch (error) {
      if (error instanceof GreenPassApiError && (error.status === 401 || error.status === 403)) {
        await loginStore.restore();
        if (!loginStore.get()) { navigate('/login', { replace: true }); return; }
        setRealError(error.status === 403 ? '当前账号无权限刷新分支/Tag；请使用 owner/admin 登录' : '登录已过期，请重新登录');
        return;
      }
      setRealError(error instanceof Error ? error.message : '刷新代码引用失败');
    } finally { setRefsLoading(false); }
  };
  useEffect(() => { if (!realClient) return; void realClient.scmConnections().then((result) => setSCMConnections(result.connections)).catch(() => setSCMConnections([])); }, [realClient]);
  useEffect(() => {
    if (!realRepoID) return;
    queueMicrotask(() => { void loadRepositoryRefs(realRepoID).catch(() => undefined); });
  }, [loadRepositoryRefs, realRepoID]);
  useEffect(() => {
    if (!realClient || !realBatch?.base_batch_id) {
      queueMicrotask(() => { setBaselineSeeds(null); setBaselineSeedsError(''); });
      return;
    }
    let cancelled = false;
    void realClient.listGenerationSeeds(realBatch.base_batch_id).then((items) => { if (!cancelled) { setBaselineSeeds(items); setBaselineSeedsError(''); } }).catch((error) => { if (!cancelled) { setBaselineSeeds(null); setBaselineSeedsError(error instanceof Error ? error.message : '读取基线候选失败'); } });
    return () => { cancelled = true; };
  }, [realBatch?.base_batch_id, realClient]);
  // Batch selection is intentionally independent from the global header
  // context. Selecting several services must not make the header jump to the
  // last checkbox the user touched.
  const toggleService = (id: number) => setSelectedServiceIds((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  const openBatchConfirmation = async () => {
    if (!realClient) { setStep(1); return; }
    if (!confirmedServices.length) { setRealError('请先在被测对象页完成服务技术画像确认，并启用生成测试用例。'); return; }
    const fallback = confirmedServices.find((service) => service.id === activeServiceId);
    const serviceList = generationScope === 'custom' ? (selectedServices.length ? selectedServices : fallback ? [fallback] : []) : scopeServices;
    if (!serviceList.length) { setRealError('请先选择至少一个技术画像已确认且允许生成的服务。'); return; }
    setRealError('');
    setBatchPreparing(true);
    try {
      setSelectedServiceIds(serviceList.map((service) => service.id));
      for (const service of serviceList) {
        const repo = realProjectRepos.find((item) => item.id === service.repo_id);
        if (repo && !repositoryRefs[repo.id]?.length) await loadRepositoryRefs(repo.id);
      }
      setBatchConfirmOpen(true);
    } catch (error) {
      setRealError(error instanceof Error ? error.message : '准备生成参数失败');
    } finally {
      setBatchPreparing(false);
    }
  };
  const startRealBatch = async () => {
    if (!realClient) { setStep(1); return; }
    setRealBusy(true); setRealError('');
    try {
      const projectID = Number(projectId);
      if (!projectID) throw new Error('请先在被测对象页选择真实项目');
      const repos = realProjectRepos.length ? realProjectRepos : await realClient.listProjectRepositories(projectID);
      const services = generationScope === 'custom' ? confirmedServices.filter((service) => selectedServiceIds.includes(service.id)) : scopeServices;
      if (!services.length) throw new Error('请至少选择一个已确认的目录服务');
      const units: Array<{ service_id: number; repo_id: number; source_path: string; ref_type: 'branch' | 'tag'; ref_name: string }> = [];
      for (const service of services) {
        const repo = repos.find((item) => item.id === service.repo_id);
        if (!repo) throw new Error(`${service.name} 未关联代码仓库`);
        const refs = repositoryRefs[repo.id]?.length ? repositoryRefs[repo.id] : await loadRepositoryRefs(repo.id);
        const selection = fullScope ? { type: 'branch' as const, name: repo.default_branch } : selectedRefs[repo.id];
        if (!selection?.name) throw new Error(`请先为 ${service.name} 选择分支或 Tag`);
        if (!refs.some((ref) => ref.ref_type === selection.type && ref.name === selection.name)) throw new Error(`${service.name} 的代码引用未同步，请先刷新分支 / Tag`);
        units.push({ service_id: service.id, repo_id: repo.id, source_path: service.source_path, ref_type: selection.type, ref_name: selection.name });
      }
      const run = await realClient.createGenerationRun({
        project_id: projectID,
        scope_type: generationScope === 'custom' ? 'service' : generationScope,
        scope_id: generationScope === 'project' ? projectID : generationScope === 'repo' ? activeRepoScopeId ?? 0 : services[0].id,
        idempotency_key: `ui-run-${projectID}-${crypto.randomUUID()}`,
        units: generationScope === 'custom' ? units : [],
      });
      setRealRun(run);
      setBatchResults(run.units.map((unit) => ({
        serviceId: unit.service_id,
        name: services.find((service) => service.id === unit.service_id)?.name ?? String(unit.service_id),
        ok: !unit.error,
        error: unit.error,
      })));
      const batches = await realClient.listGenerationBatches(projectID);
      setRealBatches(batches);
      const firstBatch = run.units.find((unit) => unit.batch_id)?.batch_id ?? null;
      if (firstBatch) setSelectedBatchId(firstBatch);
      setSelectedServiceIds([]); setBatchConfirmOpen(false); setStep(1);
      toast.success(`已创建生成任务 #${run.id}`, { description: `${run.total_units} 个服务已展开，可在任务卡查看总体进度` });
    } catch (error) { setRealError(error instanceof Error ? error.message : '创建生成任务失败'); }
    finally { setRealBusy(false); }
  };

  useEffect(() => {
    if (!projectId || !realClient || realBusy || reviewBusy) return;
    let cancelled = false;
    let timer: number | undefined;
    const refresh = async () => {
      try {
        const next = await realClient.listGenerationBatches(Number(projectId));
        if (!cancelled) setRealBatches((current) => next.map((batch) => {
          const known = current.find((item) => item.id === batch.id);
          return known && known.state_version > batch.state_version ? known : batch;
        }));
      } catch (error) {
        if (!cancelled) setRealError(error instanceof Error ? error.message : '读取真实批次失败');
      } finally { if (!cancelled) timer = window.setTimeout(() => void refresh(), 2000); }
    };
    void refresh();
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [projectId, realClient, realBusy, reviewBusy]);
  useEffect(() => {
    if (!realClient || !projectId) return;
    let cancelled = false;
    void realClient.listGenerationRuns(Number(projectId)).then((runs) => {
      if (!cancelled && runs.length) setRealRun(runs[0]);
    }).catch(() => {});
    return () => { cancelled = true; };
  }, [realClient, projectId]);

  useEffect(() => {
    if (!realClient || !realRun?.id) return;
    let cancelled = false;
    let timer: number | undefined;
    const refresh = async () => {
      try {
        const next = await realClient.getGenerationRun(realRun.id);
        if (cancelled) return;
        setRealRun(next);
        if (next.state === 'queued' || next.state === 'running' || next.state === 'partial_failed') {
          timer = window.setTimeout(() => void refresh(), 2000);
        }
      } catch (error) {
        if (!cancelled) timer = window.setTimeout(() => void refresh(), 5000);
      }
    };
    void refresh();
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [realClient, realRun?.id]);

  useEffect(() => {
    if (!realClient || !activeBatchId || activeBatchStateVersion === undefined) return;
    const batchId = activeBatchId;
    const stateVersion = activeBatchStateVersion;
    let cancelled = false;
    let timer: number | undefined;
    const load = async () => {
      try {
        const items = await realClient.listGenerationSeeds(batchId);
        if (!cancelled) setSeedResponse({ batchId, stateVersion, items });
      } catch (error) {
        if (!cancelled) {
          setSeedResponse({ batchId, stateVersion, items: [], error: error instanceof Error ? error.message : '读取候选用例失败' });
          timer = window.setTimeout(() => void load(), 3000);
        }
      }
    };
    void load();
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [realClient, activeBatchId, activeBatchStateVersion]);
  const submitRealReview = async (decision: 'approve' | 'reject') => {
    if (!realBatch || !realClient || !canReview) return;
    setReviewBusy(true); setRealError('');
    try {
      const next = await realClient.reviewGenerationBatch(realBatch.id, {
        decision,
        expected_state_version: realBatch.state_version,
        request_id: `ui-review-${realBatch.id}-${decision}`,
      });
      setRealBatches((current) => current.map((batch) => batch.id === next.id && batch.state_version <= next.state_version ? next : batch));
    } catch (error) { setRealError(error instanceof Error ? error.message : '提交审核结果失败'); }
    finally { setReviewBusy(false); }
  };

  const cancelRealBatch = async () => {
    if (!realBatch || !realClient) return;
    setReviewBusy(true); setRealError('');
    try {
      const next = await realClient.cancelGenerationBatch(realBatch.id, {
        expected_state_version: realBatch.state_version,
        request_id: `ui-cancel-${realBatch.id}-${Date.now()}`,
      });
      setRealBatches((current) => current.map((batch) => batch.id === next.id && batch.state_version <= next.state_version ? next : batch));
      toast.success('已取消生成，临时候选和源码快照已清理');
    } catch (error) { setRealError(error instanceof Error ? error.message : '取消生成失败'); }
    finally { setReviewBusy(false); }
  };

  const cancelRealRun = async () => {
    if (!realClient || !realRun) return;
    setReviewBusy(true); setRealError('');
    try {
      const next = await realClient.cancelGenerationRun(realRun.id);
      setRealRun(next);
      toast.success('已取消整个生成任务，所属服务批次和临时数据已清理');
    } catch (error) { setRealError(error instanceof Error ? error.message : '取消生成任务失败'); }
    finally { setReviewBusy(false); }
  };

  const reviewRealRun = async (decision: 'approve' | 'reject') => {
    if (!realClient || !realRun) return;
    setReviewBusy(true); setRealError('');
    try {
      const next = await realClient.reviewGenerationRun(realRun.id, { decision, request_id: `ui-run-review-${realRun.id}-${decision}-${Date.now()}` });
      setRealRun(next);
      const batches = await realClient.listGenerationBatches(Number(projectId));
      setRealBatches(batches);
      toast.success(decision === 'approve' ? '本次任务待审核服务已全部入库' : '本次任务待审核服务已全部拒绝');
    } catch (error) { setRealError(error instanceof Error ? error.message : '批量审核失败'); }
    finally { setReviewBusy(false); }
  };

  return (
    <div className="pb-24">
      <div className="mb-5 flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 py-2.5 text-xs shadow-sm">
        <span className="font-semibold text-slate-700">{realClient ? displayName : selectedAsset.name}</span>
        <span className="text-slate-300">/</span>
        <button type="button" onClick={() => navigate('/target')} className="text-slate-500 hover:text-emerald-600">定义对象</button>
        <ChevronRight className="w-3.5 h-3.5 text-slate-300" />
        <span className="rounded-md bg-emerald-600 px-2.5 py-1 font-medium text-white">生成用例</span>
        <ChevronRight className="w-3.5 h-3.5 text-slate-300" />
        <button type="button" onClick={() => navigate('/trace')} className="text-slate-500 hover:text-emerald-600">追溯覆盖</button>
        <span className="ml-auto text-[11px] text-slate-400">{realClient ? '目录服务' : (TYPE_LABEL[selectedAsset.type] ?? selectedAsset.type)}</span>
      </div>
      <PageHeader title="生成用例" desc={realClient ? `选择已确认的目录服务，为所选服务创建一个生成任务 · 当前可生成 ${confirmedServices.length} 个服务` : '针对当前被测对象的用例生成流程 · 适配解析 → AI 生成 → 质量验证 → 审核入库'}>
        <GhostButton onClick={() => navigate('/target')}>管理被测对象</GhostButton>
      </PageHeader>

      {!realClient && <div className="mb-5 rounded-xl border border-slate-200 bg-white p-5">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold text-slate-800"><Layers className="h-4 w-4 text-emerald-600" />覆盖计划</div>
            <div className="mt-1 text-[11px] text-slate-500">生成前按端点 × 场景 × 等价类规划 Cell；生成后按账本校验覆盖，缺口不静默通过。</div>
          </div>
          <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-2.5 py-1 text-[10px] font-medium text-emerald-700"><ShieldCheck className="h-3.5 w-3.5" />源码只读 · 原型数据</span>
        </div>
        <div className="mt-4 grid grid-cols-2 gap-3 md:grid-cols-4">
          {[{ label: '端点', value: '87' }, { label: '模块', value: '18' }, { label: '覆盖 Cell', value: '412 / 428' }, { label: '规划覆盖', value: '96.3%' }].map((stat) => <div key={stat.label} className="rounded-lg border border-slate-100 bg-slate-50 px-3 py-2"><div className="text-[10px] text-slate-400">{stat.label}</div><div className="mt-0.5 text-lg font-semibold text-slate-800">{stat.value}</div></div>)}
        </div>
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-slate-100"><div className="h-full w-[96.3%] rounded-full bg-emerald-500" /></div>
        <div className="mt-2 flex flex-wrap gap-1.5 text-[10px]">
          {['normal 22', 'boundary 3', 'error 7', 'auth 1', 'idempotency 3', 'data 3', '未覆盖 16'].map((item, index) => <span key={item} className={'rounded-full px-2 py-0.5 ' + (index === 6 ? 'bg-amber-50 text-amber-700' : 'bg-slate-100 text-slate-500')}>{item}</span>)}
        </div>
      </div>}

      {realClient && <div className="mb-5 flex flex-wrap items-center gap-3 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-xs text-emerald-800">
        <span className="font-semibold">DSH 生成</span>
        <span>当前通过内部 DSH SDK gateway 生成用例；模型由 DSH Profile 配置，不再使用项目级模型绑定。</span>
        <span className="rounded-full bg-white px-2 py-0.5 text-[10px] font-medium text-emerald-700">DSH SDK</span>
      </div>}

      {realClient && realRun && <Card title={`生成任务 #${realRun.id}`} className="p-5 mb-5" extra={['queued', 'running', 'partial_failed'].includes(realRun.state) ? <button type="button" disabled={reviewBusy} onClick={() => void cancelRealRun()} className="rounded-lg border border-red-200 bg-white px-3 py-1.5 text-[11px] text-red-600 hover:bg-red-50 disabled:opacity-50">取消整个任务</button> : null}>
        <div className="flex flex-wrap items-center gap-4 text-xs">
          <span className="rounded-full bg-slate-100 px-2.5 py-1 font-medium text-slate-700">{realRun.state === 'queued' ? '排队中' : realRun.state === 'running' ? '运行中' : realRun.state === 'partial_failed' ? '部分失败' : realRun.state === 'succeeded' ? '已完成' : realRun.state === 'failed' ? '失败' : '已取消'}</span>
          <span className="text-slate-500">{realRun.completed_units} / {realRun.total_units} 个服务完成</span>
          {realRun.failed_units > 0 && <span className="text-red-600">{realRun.failed_units} 个失败</span>}
          <span className="text-slate-400">{realRun.scope_type === 'project' ? '项目全量' : realRun.scope_type === 'repo' ? '仓库全量' : '指定服务'}</span>
          <span className="text-slate-400">tokens {realRun.tokens_in + realRun.tokens_out}</span>
        </div>
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-slate-100"><div className="h-full rounded-full bg-emerald-500 transition-all" style={{ width: `${realRun.total_units ? Math.round(((realRun.completed_units + realRun.failed_units) / realRun.total_units) * 100) : 0}%` }} /></div>
        <div className="mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          {realRun.units.map((unit) => <button key={unit.id} type="button" disabled={!unit.batch_id} onClick={() => unit.batch_id && setSelectedBatchId(unit.batch_id)} className="flex items-center justify-between gap-2 rounded-lg border border-slate-100 bg-slate-50 px-3 py-2 text-left text-[11px] hover:border-emerald-200 disabled:cursor-default">
            <span className="truncate text-slate-700">{realProjectServices.find((service) => service.id === unit.service_id)?.name ?? `服务 ${unit.service_id}`}</span>
            <span className={'shrink-0 rounded-full px-2 py-0.5 ' + (unit.state === 'succeeded' ? 'bg-emerald-100 text-emerald-700' : unit.state === 'failed' ? 'bg-red-50 text-red-600' : unit.state === 'cancelled' ? 'bg-slate-200 text-slate-500' : 'bg-amber-50 text-amber-700')}>{unit.state === 'running' ? '生成中' : unit.state === 'succeeded' ? '已完成' : unit.state === 'failed' ? '失败' : unit.state === 'cancelled' ? '已取消' : '排队'}</span>
          </button>)}
        </div>
        {reviewableRunUnits.length > 0 && <div className="mt-4 flex flex-wrap items-center justify-end gap-2 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3">
          <span className="mr-auto text-[11px] text-amber-800">{reviewableRunUnits.length} 个服务待审核</span>
          <button type="button" disabled={reviewBusy} onClick={() => void reviewRealRun('reject')} className="rounded-lg border border-red-200 bg-white px-3 py-1.5 text-[11px] text-red-600 hover:bg-red-50">全部拒绝</button>
          <button type="button" disabled={reviewBusy} onClick={() => void reviewRealRun('approve')} className="rounded-lg bg-emerald-600 px-3 py-1.5 text-[11px] font-medium text-white hover:bg-emerald-700">{reviewBusy ? '提交中…' : '全部通过并入库'}</button>
        </div>}
        <p className="mt-3 text-[11px] text-slate-500">整个任务取消会级联取消所有服务批次，并删除本次任务的候选与源码快照。</p>
      </Card>}

      {realClient && <Card title="生成模式" className="p-5 mb-5">
        <div className="flex flex-wrap items-center gap-3">
          <div className="inline-flex rounded-xl border border-slate-200 bg-slate-50 p-1">
            <button type="button" onClick={() => setGenerationScope('project')} className={'rounded-lg px-3 py-2 text-xs font-semibold transition ' + (generationScope === 'project' ? 'bg-white text-emerald-700 shadow-sm' : 'text-slate-500 hover:text-slate-700')}>项目全量</button>
            <button type="button" onClick={() => setGenerationScope('repo')} className={'rounded-lg px-3 py-2 text-xs font-semibold transition ' + (generationScope === 'repo' ? 'bg-white text-emerald-700 shadow-sm' : 'text-slate-500 hover:text-slate-700')}>仓库全量</button>
            <button type="button" onClick={() => setGenerationScope('custom')} className={'rounded-lg px-3 py-2 text-xs font-semibold transition ' + (generationScope === 'custom' ? 'bg-white text-emerald-700 shadow-sm' : 'text-slate-500 hover:text-slate-700')}>指定服务</button>
          </div>
          {generationScope === 'repo' && <select value={activeRepoScopeId ?? ''} onChange={(event) => setRepoScopeId(Number(event.target.value) || null)} className="min-w-[280px] flex-1 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-700 outline-none focus:border-emerald-400">
            {repoScopeOptions.map((repo) => <option key={repo.id} value={repo.id}>{repo.url} · {repo.serviceCount} 个服务</option>)}
          </select>}
          <div className="min-w-0 flex-1 text-xs text-slate-500">
            {generationScope === 'project' && <>对 <b className="text-slate-700">{selectedAsset.name}</b> 下全部 {confirmedServices.length} 个已确认服务、{repoScopeOptions.length} 个仓库生成；每个仓库使用默认分支最新提交。</>}
            {generationScope === 'repo' && <>只对选中仓库下的 {scopeServices.length} 个服务生成，使用该仓库默认分支；不会生成其他仓库的用例。</>}
            {generationScope === 'custom' && <>自行勾选服务并逐个选择分支或 Tag；适合局部复测，不代表项目或仓库全量。</>}
          </div>
          <span className="rounded-full bg-emerald-50 px-2.5 py-1 text-[10px] font-medium text-emerald-700">可重复生成</span>
        </div>
        <div className="mt-3 flex flex-wrap gap-2 text-[10px] text-slate-500">
          <span className="rounded bg-slate-50 px-2 py-1">范围：{generationScope === 'project' ? '项目全量' : generationScope === 'repo' ? '仓库全量' : '指定服务'}</span>
          <span className="rounded bg-slate-50 px-2 py-1">服务：{scopeServices.length}</span>
          <span className="rounded bg-slate-50 px-2 py-1">仓库：{generationScope === 'project' ? repoScopeOptions.length : generationScope === 'repo' ? 1 : new Set(scopeServices.map((service) => service.repo_id)).size}</span>
          <span className="rounded bg-slate-50 px-2 py-1">代码引用：{generationScope === 'custom' ? '逐服务选择' : '仓库默认分支'}</span>
          <span className="rounded bg-slate-50 px-2 py-1">每次点击创建新任务</span>
        </div>
      </Card>}

      {realClient && fullScope && <Card title={generationScope === 'repo' ? '仓库全量范围' : '项目全量范围'} className="p-5 mb-5">
        <div className="grid gap-3 sm:grid-cols-3">
          <div className="rounded-xl border border-emerald-100 bg-emerald-50/60 px-4 py-3"><div className="text-[10px] text-emerald-700">服务</div><div className="mt-1 text-2xl font-semibold text-slate-800">{scopeServices.length}</div><div className="text-[10px] text-slate-500">{generationScope === 'repo' ? '该仓库下已确认服务' : '全部已确认服务'}</div></div>
          <div className="rounded-xl border border-slate-100 bg-slate-50 px-4 py-3"><div className="text-[10px] text-slate-500">仓库</div><div className="mt-1 text-2xl font-semibold text-slate-800">{generationScope === 'project' ? repoScopeOptions.length : 1}</div><div className="text-[10px] text-slate-500">{generationScope === 'repo' ? '当前仓库默认分支' : '各仓库默认分支'}</div></div>
          <div className="rounded-xl border border-slate-100 bg-slate-50 px-4 py-3"><div className="text-[10px] text-slate-500">DSH 任务</div><div className="mt-1 text-2xl font-semibold text-slate-800">{scopeServices.length}</div><div className="text-[10px] text-slate-500">每服务一个批次</div></div>
        </div>
        {generationScope === 'repo' && <div className="mt-3 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-[11px] text-slate-600">
          {repoScopeOptions.find((repo) => repo.id === activeRepoScopeId)?.url} · 默认分支 <b>{repoScopeOptions.find((repo) => repo.id === activeRepoScopeId)?.defaultBranch}</b>
        </div>}
        <div className="mt-4 flex flex-wrap gap-1.5">
          {scopeServices.slice(0, 18).map((service) => <span key={service.id} className="rounded-full bg-slate-100 px-2 py-1 text-[10px] text-slate-600">{service.name}</span>)}
          {scopeServices.length > 18 && <span className="rounded-full bg-slate-100 px-2 py-1 text-[10px] text-slate-500">+{scopeServices.length - 18}</span>}
        </div>
        <p className="mt-3 text-[11px] text-slate-500">{generationScope === 'repo' ? '仓库全量只覆盖当前仓库下的服务，不会生成其他仓库的用例；使用默认分支最新提交。' : '项目全量覆盖全部已确认服务；每个仓库使用默认分支最新提交，不使用单个 Tag 代表全项目。'} 点击底部生成按钮后统一确认。</p>
      </Card>}

      {realClient && !fullScope && <Card title="选择生成范围" className="p-0 mb-5 overflow-hidden">
        <div className="p-5 border-b border-slate-100 flex flex-wrap items-center gap-3">
          <div className="flex-1 min-w-[220px]"><div className="text-sm font-semibold text-slate-700">{fullScope ? '全量范围' : '已确认服务'}</div><div className="text-[11px] text-slate-400 mt-1">{fullScope ? '全量模式自动包含所有已确认服务，使用各仓库默认分支；仍可切换“指定服务”做局部生成。' : '只有“生成测试用例 = 是”的服务可以选择；选择多个服务时会为每个服务创建独立批次。'}</div></div>
          <span className="rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700">已选 {selectedServices.length} / {confirmedServices.length}</span>
          {!fullScope && <button type="button" onClick={() => setSelectedServiceIds(confirmedServices.map((service) => service.id))} className="text-xs text-emerald-700 hover:underline">全选</button>}
          {!fullScope && <button type="button" onClick={() => setSelectedServiceIds([])} className="text-xs text-slate-500 hover:underline">清空</button>}
          <input value={serviceSearch} onChange={(event) => setServiceSearch(event.target.value)} placeholder="搜索服务或源码目录" className="w-full sm:w-56 rounded-lg border border-slate-200 px-3 py-2 text-xs outline-none focus:border-emerald-400" />
        </div>
        <div className="divide-y divide-slate-100 max-h-[330px] overflow-y-auto">
          {realCatalogLoading ? <div className="p-8 text-center text-xs text-slate-500">正在读取项目目录…</div> : visibleServices.length === 0 ? <div className="p-8 text-center text-xs text-slate-500">暂无符合条件的已确认服务</div> : visibleServices.map((service) => {
            const checked = fullScope || selectedServiceIds.includes(service.id);
            const serviceRepo = realProjectRepos.find((item) => item.id === service.repo_id);
            const profile = profileFor(service);
            return <label key={service.id} className={'flex items-start gap-3 px-5 py-3 transition ' + (fullScope ? 'cursor-default' : 'cursor-pointer') + (checked ? ' bg-emerald-50/60' : ' hover:bg-slate-50')}>
              <input type="checkbox" checked={checked} disabled={fullScope} onChange={() => toggleService(service.id)} className="mt-1 h-4 w-4 accent-emerald-600" />
              <span className="min-w-0 flex-1"><span className="flex flex-wrap items-center gap-2 text-xs font-semibold text-slate-700"><span>{service.name}</span><span className="rounded bg-emerald-50 px-1.5 py-0.5 text-[10px] font-medium text-emerald-700">生成测试用例：是</span><span className="text-[10px] font-normal text-slate-400">{service.kind}</span></span><span className="mt-1 block truncate text-[11px] text-slate-500">{sourceDirectoryLabel(service.source_path)}</span><span className="mt-1 block text-[10px] text-slate-400">语言：{profile.language} · 框架：{profile.framework} · 工具：{profile.tool}</span></span>
              <span className="hidden min-w-[180px] text-right text-[10px] text-slate-400 sm:block">{serviceRepo?.url ?? '未关联仓库'}<br /><b className="font-normal text-slate-500">{serviceRepo?.default_branch ?? '未配置分支'}</b></span>
            </label>;
          })}
        </div>
      </Card>}

      {batchConfirmOpen && <div className="mb-5 rounded-xl border border-emerald-200 bg-emerald-50/60 p-5 shadow-sm">
        <div className="flex items-start justify-between gap-3"><div><div className="text-sm font-semibold text-slate-800">确认创建</div><div className="mt-1 text-xs text-slate-500">将创建一个生成任务，展开以下 {selectedServices.length} 个服务批次，创建后可逐个审核。</div></div><button type="button" onClick={() => setBatchConfirmOpen(false)} className="text-xs text-slate-500 hover:text-slate-700">取消</button></div>
        <div className="mt-4 grid gap-2 md:grid-cols-2">
          {selectedServices.map((service) => {
            const serviceRepo = realProjectRepos.find((item) => item.id === service.repo_id);
            const profile = profileFor(service);
            const refs = serviceRepo ? repositoryRefs[serviceRepo.id] ?? [] : [];
            const selection = serviceRepo ? selectedRefs[serviceRepo.id] : undefined;
            const detail = selection ? refs.find((ref) => ref.ref_type === selection.type && ref.name === selection.name) : undefined;
            return <div key={service.id} className="rounded-lg border border-emerald-100 bg-white px-3 py-2.5 text-xs">
              <div className="flex items-center justify-between gap-2">
                <div className="font-semibold text-slate-700">{service.name}</div>
                <span className={'rounded-full px-2 py-0.5 text-[10px] ' + (profile.complete ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-700')}>{profile.complete ? '条件完整' : '条件不完整'}</span>
              </div>
              <div className="mt-1 truncate text-[11px] text-slate-500">目录：{sourceDirectoryLabel(service.source_path)}</div>
              <div className="mt-1 text-[10px] text-slate-500">类型：{service.kind} · 语言：{profile.language} · 框架：{profile.framework}</div>
              <div className="mt-1 text-[10px] text-slate-400">工具：{profile.tool} · 默认分支：{serviceRepo?.default_branch ?? '未配置'}</div>
              <div className="mt-2 rounded-md border border-slate-100 bg-slate-50 p-2">
                <div className="mb-1 text-[10px] font-medium text-slate-500">本次代码引用</div>
                {fullScope
                  ? <div className="rounded-md border border-emerald-100 bg-emerald-50 px-2 py-1.5 text-[10px] text-emerald-700">默认分支：<b>{serviceRepo?.default_branch || '未配置'}</b>（全量模式不使用单个 Tag）</div>
                  : serviceRepo ? <CodeRefSelect label="生成代码引用" refs={refs} value={selection} onChange={(next) => { if (serviceRepo) setSelectedRefs((current) => ({ ...current, [serviceRepo.id]: next })); }} /> : <div className="text-[10px] text-red-600">未关联代码仓库</div>}
                <div className="mt-1 text-[10px] text-slate-500">提交 SHA：{detail?.commit_sha ? detail.commit_sha.slice(0, 12) : fullScope ? '创建后冻结默认分支最新 SHA' : '待刷新'}</div>
                {!fullScope && !selection?.name && <div className="mt-1 text-[10px] text-red-600">请选择代码引用后再创建</div>}
              </div>
            </div>;
          })}
        </div>
        <div className="mt-4 flex items-center justify-between gap-3"><span className="text-[11px] text-slate-500">每个批次按所选分支或 Tag 创建；服务端随后冻结实际提交 SHA 与服务目录。本期仅支持 HTTP/API 结构化候选。</span><button type="button" disabled={realBusy || !selectedServices.length} onClick={() => void startRealBatch()} className="rounded-lg bg-emerald-600 px-4 py-2 text-xs font-medium text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50">{realBusy ? '正在创建…' : `开始生成 ${selectedServices.length} 个服务`}</button></div>
      </div>}

      {/* 被测对象 = 全局轴心：本流程围绕当前被测对象展开（切换用页首被测对象导航） */}
      {!realClient && <Card title="当前被测对象 · 生成轴心" className="p-0 mb-5 overflow-hidden">
        <div className="p-6">
        <div className="grid grid-cols-1 lg:grid-cols-[minmax(260px,1fr)_minmax(360px,1.5fr)_auto] gap-5 items-start">
          <div className="flex items-center gap-3 min-w-0">
            <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-emerald-500 to-teal-600 flex items-center justify-center text-white shrink-0">
              <Crosshair className="w-6 h-6" />
            </div>
            <div>
              <div className="text-[11px] text-slate-400 flex items-center gap-2">
                <Layers className="w-3 h-3 text-emerald-500" />被测对象 · {realClient ? '服务' : (TYPE_LABEL[selectedAsset.type] ?? selectedAsset.type)}
                {!realClient && <span className="text-[9px] bg-emerald-50 text-emerald-700 px-1.5 py-0.5 rounded-full">覆盖率 {selectedAsset.coverage}%</span>}
              </div>
              <div className="text-xl font-bold text-slate-800">{displayName}</div>
              <div className="text-[11px] text-slate-500 mt-0.5">{realClient ? '真实项目目录中的被测服务，生成范围限定为该服务源码目录。' : '本用例生成流程的轴心对象 · 切换对象请用页首「被测对象」导航（面包屑 / 层次筛选）'}</div>
              {realClient && <div className="mt-3"><label htmlFor="generation-service" className="mb-1 block text-[10px] text-slate-400">查看服务上下文</label><select id="generation-service" value={selectedService?.id ?? ''} onChange={(event) => selectService(Number(event.target.value) || null)} className="w-full rounded-md border border-slate-200 bg-white px-2 py-1.5 text-xs text-slate-700"><option value="">请选择目录服务</option>{realProjectServices.map((service) => <option key={service.id} value={service.id}>{service.name}（{sourceDirectoryLabel(service.source_path)}）{service.generation_status !== 'confirmed' || service.profile_status !== 'confirmed' ? '，待确认' : ''}</option>)}</select>{selectedService && <div className={'mt-2 text-[10px] ' + (selectedServiceReady ? 'text-emerald-700' : 'text-amber-700')}>{selectedServiceReady ? '技术画像与生成范围已确认，可在上方加入生成范围。' : '此服务尚未完成画像确认或未启用生成；请前往被测对象页处理。'}</div>}</div>}
            </div>
          </div>

          {repo ? realClient ? (
            <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 min-w-0">
              <div className="flex items-center justify-between gap-2 text-xs font-medium text-slate-600"><span className="flex items-center gap-1.5"><GitBranch className="w-3.5 h-3.5 text-emerald-500" />当前查看服务的代码来源</span>{realRepo?.scm_connection_status==='verified'?<button type="button" disabled={refsLoading} onClick={() => void refreshRepositoryRefs()} className="rounded border border-emerald-200 bg-white px-2 py-1 text-[10px] text-emerald-700 hover:bg-emerald-50 disabled:opacity-50">{refsLoading ? '刷新中…' : '刷新分支 / Tag'}</button>:<button type="button" onClick={()=>navigate('/target')} className="rounded border border-amber-200 bg-white px-2 py-1 text-[10px] text-amber-700 hover:bg-amber-50">配置读取连接</button>}</div>
              <div className="mt-2 truncate text-xs text-slate-700" title={realRepo?.url}>{realRepo?.url}</div>
              {realRepo?.scm_connection_status!=='verified'&&<div className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] leading-5 text-amber-800">该仓库尚未绑定已验证的 GitHub 只读连接，无法安全读取分支、技术画像或冻结源码。请前往「被测对象」配置连接并绑定仓库。</div>}
              {realRepo?.scm_connection_status==='verified'&&<div className="mt-2 text-[10px] text-emerald-700">源码读取连接已验证{scmConnections.find(item=>item.id===realRepo.scm_connection_id)?.name?` · ${scmConnections.find(item=>item.id===realRepo.scm_connection_id)?.name}`:''}</div>}
              <div className="mt-3"><CodeRefSelect refs={activeRepoRefs} value={selectedRef} disabled={realRepo?.scm_connection_status!=='verified'} onChange={(next) => { if (realRepo) setSelectedRefs((current) => ({ ...current, [realRepo.id]: next })); }} /></div>
              <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-slate-500"><span>提交 SHA <b className="font-mono text-slate-700">{selectedRefDetails?.commit_sha ? selectedRefDetails.commit_sha.slice(0, 12) : '刷新后可见'}</b></span><span>服务目录 <b className="font-mono text-slate-700">{selectedService?.source_path || '未配置'}</b></span></div>
              <div className="mt-2 rounded-md border border-slate-200 bg-white px-2.5 py-2 text-[10px] leading-4 text-slate-500"><b className="text-slate-700">系统基线</b>：自动采用该项目、仓库和服务最近一次已生效的生成批次；没有已生效批次时作为首次生成。基线不能人工指定。</div>
              <p className="mt-2 text-[10px] leading-4 text-slate-400">本次只选择分支或 Tag；创建后服务端重新解析并冻结最终 SHA，历史记录不会被后续分支更新改写。</p>
              <p className="mt-1 text-[10px] leading-4 text-slate-400">此选择作用于当前查看服务所在的仓库；选择多个服务时，请在“生成用例”确认面板逐个服务复核代码引用。</p>
            </div>
          ) : (
            <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 min-w-0">
              <div className="flex items-center gap-1.5 text-xs font-medium text-slate-600"><GitBranch className="w-3.5 h-3.5 text-emerald-500" />仓库配置</div>
              <div className="mt-2 truncate text-xs text-slate-700" title={'url' in repo ? repo.url : repo.name}>{'url' in repo ? repo.url : repo.name}</div>
              <div className="mt-3 flex flex-wrap items-end gap-4">
              <div>
                <div className="text-[10px] text-slate-400">分支（生成输入）</div>
                <select value={genBranch} onChange={(e) => setGenBranch(e.target.value)}
                  className="text-xs border border-slate-200 rounded-md px-2 py-1 bg-white text-slate-700 focus:outline-none focus:border-emerald-400">
                  {repo.branches.length ? repo.branches.map((b) => <option key={b} value={b}>{b}</option>) : <option value="">未配置</option>}
                </select>
              </div>
              <div>
                <div className="text-[10px] text-slate-400">目标版本（执行前校验）</div>
                <select value={genVer} onChange={(e) => setGenVer(e.target.value)}
                  className="text-xs border border-slate-200 rounded-md px-2 py-1 bg-white text-slate-700 focus:outline-none focus:border-emerald-400">
                  {repo.versions.length ? repo.versions.map((v) => <option key={v} value={v}>{v}</option>) : <option value="">未配置版本</option>}
                </select>
              </div>
              </div>
            </div>
          ) : realClient ? (
            <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-700 min-w-0">{realCatalogLoading ? '正在读取项目目录…' : realError || catalog.error || '请选择一个已确认的目录服务后继续。'}</div>
          ) : (
            <div className="flex items-center gap-3 bg-amber-50 rounded-xl border border-amber-200 px-4 py-3 text-[12px] text-amber-700 flex-1 min-w-[300px]">
              <CloudDownload className="w-4 h-4 shrink-0" />
              该被测对象暂无关联仓库，无法生成用例——请到<b>被测对象页</b>添加仓库与上游源接入。
            </div>
          )}

          <GhostButton onClick={() => navigate('/target')}>
            <Crosshair className="w-3.5 h-3.5 mr-1" />去被测对象页切换 / 管理
          </GhostButton>
        </div>

        {repo && realClient && (
          <div className="mt-4 pt-3 border-t border-slate-100 flex items-start gap-2 text-[11px] text-slate-500">
            <CloudDownload className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
            <span>选择已同步的分支或 Tag 后创建；服务端按 <b>{selectedRef?.type === 'tag' ? 'Tag' : '分支'} {selectedRef?.name || '未选择'}</b> 与 <b>{selectedService?.source_path || '服务目录'}</b> 冻结源码。执行环境版本校验仍在「测试执行」中显式配置。</span>
          </div>
        )}
        {repo && !realClient && (
          <div className="mt-4 pt-3 border-t border-slate-100 flex items-start gap-2 text-[11px] text-slate-500">
            <CloudDownload className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
            <span>当前生成输入：<b>{'url' in repo ? repo.url : repo.name}</b> · 分支 <b>{genBranch || '未配置'}</b> · 服务目录 <b>{selectedService?.source_path || '项目级目录'}</b>。目标版本：<b>未配置</b>（需由 CI/CD 推送版本或在版本目录登记后才能执行环境校验）。</span>
          </div>
        )}
        {repo && !realClient && (
          <div className="mt-3 flex flex-wrap gap-2">
            {UPSTREAM_ADAPTERS.map((sc) => {
              const on = repo.src.includes(sc.source);
              return (
                <span key={sc.source} className={'flex items-center gap-1 text-[10px] border px-2 py-1 rounded-md ' + (on ? 'bg-emerald-50 border-emerald-200 text-emerald-700' : 'bg-slate-50 border-slate-200 text-slate-300')}>
                  {on ? <Check className="w-3 h-3 text-emerald-500" /> : <span className="w-2 h-2 rounded-sm bg-slate-200" />}
                  <span className="font-medium">{sc.source}</span>
                  <span className={on ? 'text-emerald-400' : 'text-slate-300'}>{sc.system}</span>
                </span>
              );
            })}
          </div>
        )}
        </div>
      </Card>}

      {batchResults.length > 0 && <div className="mb-5 rounded-xl border border-slate-200 bg-white px-4 py-3 text-xs"><div className="font-semibold text-slate-700">本次创建结果</div><div className="mt-2 flex flex-wrap gap-2">{batchResults.map((result) => <span key={result.serviceId} className={'rounded-full px-2.5 py-1 ' + (result.ok ? 'bg-emerald-50 text-emerald-700' : 'bg-red-50 text-red-700')}>{result.ok ? '已创建' : '失败'} · {result.name}{result.error ? `：${result.error}` : ''}</span>)}</div></div>}
      {realClient && <Card title={activeServiceId ? '当前服务生成批次' : '项目生成批次'} className="p-0 mb-5 overflow-hidden" extra={<span className="text-[11px] text-slate-400">共 {visibleBatches.length} 个 · 自动刷新</span>}>
        <div className="divide-y divide-slate-100">{visibleBatches.map((batch) => {
          const service = realProjectServices.find((item) => item.id === batch.service_id);
          const active = realBatch?.id === batch.id;
          return <button type="button" key={batch.id} onClick={() => { setSelectedBatchId(batch.id); setStep(1); }} className={'w-full px-5 py-3 text-left transition ' + (active ? 'bg-emerald-50/60' : 'hover:bg-slate-50')}>
            <div className="flex flex-wrap items-center gap-2"><span className="text-xs font-semibold text-slate-700">{service?.name ?? `服务 #${batch.service_id ?? '未绑定'}`}</span><span className={'rounded-full px-2 py-0.5 text-[10px] font-medium ' + (batch.state === 'approved' ? 'bg-emerald-100 text-emerald-700' : batch.state === 'failed' || batch.state === 'rejected' ? 'bg-red-100 text-red-700' : 'bg-amber-100 text-amber-700')}>{STAGE_LABEL[batch.state] ?? batch.state}</span><span className="ml-auto font-mono text-[10px] text-slate-400">#{batch.id}</span></div>
            <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-[10px] text-slate-500"><span>目录：{batch.source_path || service?.source_path || '项目级目录'}</span><span>{batch.ref_type === 'tag' ? 'Tag' : '分支'}：{batch.branch || '未配置'}</span><span>冻结 SHA：{batch.head_sha ? batch.head_sha.slice(0, 12) : '冻结中'}</span><span>系统基线：{batch.base_batch_id ? `已生效批次 #${batch.base_batch_id}` : '首次生成'}</span></div>
          </button>;
        })}{visibleBatches.length === 0 && <div className="px-5 py-8 text-center text-xs text-slate-500">{activeServiceId ? '当前服务暂无生成批次。选择该服务后创建批次，即可在这里查看进度和审核候选。' : '当前项目暂无生成批次。'}</div>}</div>
      </Card>}
      {realError && <div className="mb-5 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">{realError}</div>}

      {realClient && realBatch && <Card title={`真实生成流程 · ${realProjectServices.find((service) => service.id === realBatch.service_id)?.name ?? '项目级批次'} · #${realBatch.id}`} className="p-5 mb-5" extra={['enqueued', 'fetching', 'parsing', 'generating', 'quality_check', 'review'].includes(realBatch.state) ? <button type="button" disabled={reviewBusy} onClick={() => void cancelRealBatch()} className="rounded-lg border border-red-200 bg-white px-3 py-1.5 text-[11px] text-red-600 hover:bg-red-50 disabled:opacity-50">取消生成</button> : null}>
        <div className="flex flex-wrap items-center gap-2 mb-5">
          {(['enqueued', 'fetching', 'parsing', 'generating', 'quality_check', 'review', 'committing', 'approved'] as const).map((state, index) => {
            const order = ['enqueued', 'fetching', 'parsing', 'generating', 'quality_check', 'review', 'committing', 'approved'];
            const current = order.indexOf(realBatch.state);
            const done = index < current || realBatch.state === 'approved';
            const active = state === realBatch.state;
            return <div key={state} className={'flex items-center gap-1.5 text-[10px] ' + (active ? 'text-emerald-700 font-semibold' : done ? 'text-emerald-500' : 'text-slate-300')}><span className={'w-6 h-6 rounded-full flex items-center justify-center border ' + (active ? 'bg-emerald-600 text-white border-emerald-600' : done ? 'bg-emerald-50 border-emerald-200' : 'bg-white border-slate-200')}>{done ? '✓' : index + 1}</span><span>{STAGE_LABEL[state]}</span>{index < 7 && <span className="mx-1 text-slate-200">→</span>}</div>;
          })}
        </div>
        <div className="mb-4 flex flex-wrap gap-3 text-xs text-slate-500"><span>已选{realBatch.ref_type === 'tag' ? ' Tag' : '分支'} <b>{realBatch.branch}</b></span><span>冻结 SHA <b className="font-mono">{realBatch.head_sha ? realBatch.head_sha.slice(0, 12) : '冻结中'}</b></span><span>系统基线 <b>{realBatch.base_batch_id ? `已生效批次 #${realBatch.base_batch_id}` : '首次生成'}</b></span><span>候选种子 <b>{seedsLoading ? '读取中…' : realSeeds.length}</b></span><span>当前阶段 <b className="text-emerald-700">{STAGE_LABEL[realBatch.state] ?? realBatch.state}</b></span></div>
        {realBatch.base_batch_id && <div className="mb-4 rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 text-xs"><div className="font-medium text-slate-700">系统基线 #{realBatch.base_batch_id} 的候选差异</div>{baselineSeedsError ? <div className="mt-1 text-red-600">{baselineSeedsError}</div> : !candidateDiff ? <div className="mt-1 text-slate-500">正在读取基线候选…</div> : <div className="mt-2 flex flex-wrap gap-3 text-[11px]"><span className="text-emerald-700">新增 {candidateDiff.added}</span><span className="text-slate-600">保持或更新 {candidateDiff.unchanged}</span><span className="text-amber-700">拟归档 {candidateDiff.removed}</span><span className="text-slate-400">按稳定请求意图识别；审核通过后只同步当前有效用例集。</span></div>}</div>}
        {seedsError ? <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-5 text-xs text-red-700">候选用例读取失败：{seedsError}。页面将自动重试，读取成功前不能审核或执行。</div> : seedsLoading ? <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-5 text-xs text-slate-500">正在读取当前批次候选用例…</div> : realSeeds.length === 0 ? <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-5 text-xs text-slate-500">{['failed', 'rejected', 'rolled_back', 'approved', 'review'].includes(realBatch.state) ? '本批次没有可展示的候选用例，不能审核或执行。' : '正在生成候选用例，页面会自动更新进度。'}</div> : <GeneratedSeedReviewList seeds={realSeeds} />}
        {realBatch.state === 'review' && <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3"><p className="text-xs font-medium text-amber-800">请先逐条展开核对请求、断言与源码证据，再决定本次迭代是否生效。</p><p className="mt-1 text-[11px] text-amber-700">审核通过会同步一份当前有效用例：新增、更新、归档均以系统基线为准；拒绝不会改变正式用例。</p><div className="mt-3 flex justify-end gap-2"><button type="button" disabled={!canReview} onClick={() => void submitRealReview('reject')} className="text-xs px-3 py-1.5 rounded-lg border border-red-200 bg-white text-red-600 hover:bg-red-50">拒绝该迭代</button><button type="button" disabled={!canReview} onClick={() => void submitRealReview('approve')} className="text-xs px-3 py-1.5 rounded-lg bg-emerald-600 text-white hover:bg-emerald-700">{reviewBusy ? '提交中…' : '确认生效并同步用例'}</button></div></div>}
        {realBatch.state === 'approved' && <div className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-xs text-emerald-700"><span>本次迭代已生效，当前有效用例集已同步；变更记录仅保留用于审计与追溯，不作为多版本用例供日常操作。</span><div className="flex flex-wrap gap-2"><button type="button" onClick={() => navigate(`/cases?target_id=${realBatch.target_id}`)} className="rounded-lg border border-emerald-300 bg-white px-3 py-1.5 font-medium text-emerald-700 hover:bg-emerald-50">查看当前有效用例</button><button type="button" disabled={seedsLoading || Boolean(seedsError) || !realSeeds.some((seed) => seed.case_id)} onClick={() => { const ids = realSeeds.flatMap((seed) => seed.case_id ? [seed.case_id] : []); if (ids.length) navigate(`/exec?target_id=${realBatch.target_id}&case_ids=${ids.join(',')}`); }} className="rounded-lg bg-emerald-600 px-3 py-1.5 font-medium text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50">执行本次有效用例</button></div></div>}
        {realBatch.state === 'rejected' && <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 text-xs text-slate-600">该批次已被拒绝，未写入正式用例。请修正技术画像、源码范围或模型配置后重新创建批次。</div>}
        {realBatch.state === 'rolled_back' && <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800">该批次已回退；请在用例管理与审计记录中核对受影响的版本后再重新生成。</div>}
        {realBatch.state === 'failed' && <div className="mt-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><p>生成失败：{realBatch.failure_summary || '请检查源码访问、模型连接和候选质量校验。'}</p><div className="mt-2"><button type="button" onClick={() => navigate('/target')} className="font-medium underline">前往被测对象检查仓库、服务目录和技术画像</button><span className="mx-2 text-red-300">·</span><button type="button" onClick={() => navigate('/models')} className="font-medium underline">检查项目模型绑定</button></div><p className="mt-2 text-[10px] text-red-500">失败批次不会写入正式用例；修正后请创建新批次，旧批次保留用于审计。</p></div>}
      </Card>}

      {/* 用例生成流程向导（六步，围绕当前被测对象） */}
      <Card title={`用例生成流程 · 针对 ${selectedAsset.name}`} className={'p-5 mb-5 ' + (realClient ? 'hidden' : '')}>
        {/* 步骤导航 */}
        <div className="flex items-center mb-5">
          {FLOW_STEPS.map((s, i) => {
            const n = i + 1;
            const done = stored || n < step;
            const active = n === step;
            const Icon = FLOW_ICON[s.key];
            return (
              <div key={s.key} className="flex flex-1 items-center">
                <button type="button" onClick={() => { if (done || active) return; setStep(n); }}
                  className={'flex-1 text-center rounded-xl px-2 py-2.5 border-2 transition ' + (active ? 'bg-emerald-600 border-emerald-600 text-white shadow-md shadow-emerald-200' : done ? 'bg-emerald-50 border-emerald-200 text-emerald-700 cursor-pointer hover:border-emerald-300' : 'bg-white border-slate-200 text-slate-400 hover:border-slate-300')}>
                  <div className="flex items-center justify-center gap-2">
                    <span className={'relative w-9 h-9 rounded-full flex items-center justify-center ring-2 ring-offset-2 ' + (active ? 'bg-white ring-white/40 ring-offset-emerald-600' : done ? 'bg-emerald-600 ring-emerald-100 ring-offset-emerald-50' : 'bg-slate-100 ring-slate-100 ring-offset-white')}>
                      {done && !active ? (
                        <Check className="w-4 h-4 text-white" />
                      ) : (
                        <Icon className={'w-5 h-5 ' + (active ? 'text-emerald-600' : 'text-slate-500')} />
                      )}
                      <span className={'absolute -top-1 -right-1 w-4 h-4 rounded-full text-[9px] font-bold flex items-center justify-center ' + (active ? 'bg-white text-emerald-600' : done ? 'bg-white text-emerald-600' : 'bg-slate-200 text-slate-500')}>{n}</span>
                    </span>
                    <span className="text-xs font-semibold">{s.label}</span>
                  </div>
                  <div className={'text-[9px] mt-1.5 ' + (active ? 'text-emerald-100' : done ? 'text-emerald-500' : 'text-slate-300')}>{s.meta}</div>
                </button>
                {n < FLOW_STEPS.length && <div className="w-6 flex items-center text-slate-300 justify-center"><ChevronRight className="w-4 h-4" /></div>}
              </div>
            );
          })}
        </div>

        {/* 当前步骤内容 */}
        <div className="border border-slate-100 rounded-lg bg-slate-50/40 p-4 min-h-[180px]">
          {cur.key === 'repo' && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-50 text-emerald-600"><Crosshair className="w-4 h-4" /></span>
                当前被测对象（生成流程轴心 · 本流程第一步）
              </div>
              <div className="bg-white border border-emerald-200 rounded-xl p-4 flex items-start gap-3">
                <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-emerald-500 to-teal-600 flex items-center justify-center text-white shrink-0"><Crosshair className="w-5 h-5" /></div>
                <div>
                  <div className="text-[11px] text-slate-400">被测对象（轴心）</div>
                  <div className="text-lg font-bold text-slate-800">{selectedAsset.name}<span className="ml-2 text-[10px] font-normal text-slate-400">{TYPE_LABEL[selectedAsset.type] ?? selectedAsset.type} · 覆盖率 {selectedAsset.coverage}%</span></div>
                  {repo ? (
                    <div className="text-[11px] text-slate-500 mt-1">关联仓库 <b className="text-emerald-600">{repo.name}@{genBranch}@{genVer}</b> · 已接入 {repo.src.length}/{UPSTREAM_ADAPTERS.length} 类上游源 · 负责人 {repo.owner}</div>
                  ) : (
                    <div className="text-[11px] text-amber-600 mt-1">暂无关联仓库——到被测对象页添加后即可生成</div>
                  )}
                </div>
              </div>
              <div className="flex items-start gap-2 text-[11px] text-slate-500 bg-white border border-slate-200 rounded-lg px-3 py-2">
                <Crosshair className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
                <span>本生成流程<b>针对当前被测对象</b>展开：适配解析、AI 生成、质量验证、人工审核、用例入库都是围绕它（{selectedAsset.name}）的步骤。切换被测对象请用页首「被测对象」导航——切换后本流程将针对新对象。</span>
              </div>
            </div>
          )}

          {cur.key === 'adapter' && (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-50 text-emerald-700"><Plug className="w-4 h-4" /></span>
                适配解析 · {selectedAsset.name} 六类上游源 → 统一格式
              </div>
              <div className="grid grid-cols-2 md:grid-cols-3 gap-2.5">
                {UPSTREAM_ADAPTERS.map((a) => {
                  const on = repo?.src.includes(a.source);
                  return (
                    <div key={a.source} className={'bg-white border rounded-lg px-3 py-2 ' + (on ? 'border-slate-200' : 'border-slate-100 opacity-45')}>
                      <div className="flex items-center gap-1.5 text-[11px] font-medium text-slate-600">{on ? <Check className="w-3 h-3 text-emerald-500" /> : null}{a.source}</div>
                      <div className="text-[10px] text-slate-400 mt-0.5">{a.extract}</div>
                      <div className="text-[10px] text-emerald-600 mt-1">{on ? '统一为结构化规格' : '未接入'}</div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {cur.key === 'seed' && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-amber-50 text-amber-600"><Sparkles className="w-4 h-4" /></span>
                AI 生成种子 · 读取 {selectedAsset.name}@{genBranch}@{genVer} 代码与文档
              </div>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                {AI_TRACES.map((t, i) => (
                  <div key={t.step} className="bg-amber-50/50 border border-amber-100 rounded-lg p-3">
                    <div className="flex items-center gap-2 mb-1.5">
                      <span className="w-5 h-5 rounded-full bg-amber-500 text-white flex items-center justify-center text-[10px] font-bold">{i + 1}</span>
                      <span className="text-xs font-medium text-slate-700">{t.step}</span>
                    </div>
                    <div className="text-[10px] text-slate-500 leading-relaxed">{t.desc}</div>
                    <div className="mt-1.5 text-[10px] text-amber-600 font-medium flex items-center gap-1"><Sparkles className="w-3 h-3" />{t.out}</div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {cur.key === 'quality' && (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-50 text-emerald-600"><ShieldCheck className="w-4 h-4" /></span>
                质量验证 · {selectedAsset.name} 变异 + 断言 + 重复 + 追溯
              </div>
              <div className="grid grid-cols-2 gap-3">
                {QUALITY.map((qv) => (
                  <div key={qv.label} className="bg-white border border-slate-200 rounded-lg px-3 py-2">
                    <div className="flex justify-between text-xs mb-1.5">
                      <span className="text-slate-600">{qv.label}</span>
                      <span className="text-slate-500">{qv.val}</span>
                    </div>
                    <div className="h-2 bg-slate-100 rounded-full">
                      <div className={qv.color + ' h-full rounded-full'} style={{ width: qv.width + '%' }} />
                    </div>
                    <div className="text-[10px] text-slate-400 mt-1">{qv.note}</div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {cur.key === 'review' && (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-50 text-emerald-600"><UserCheck className="w-4 h-4" /></span>
                人工审核 · {selectedAsset.name} 逐条确认用例
              </div>
              {realBatch ? (
                <>
                  {seedsLoading ? <div className="text-xs text-slate-500 bg-white border border-slate-200 rounded-lg p-4">正在读取服务端候选种子…</div> : realSeeds.length === 0 ? <div className="text-xs text-slate-500 bg-white border border-slate-200 rounded-lg p-4">候选种子尚未生成，当前不能进行人工审核。请等待生成 worker 推进批次。</div> : <>
                    <GeneratedSeedReviewList seeds={realSeeds} />
                    {realBatch.state === 'review' && <div className="flex justify-end gap-2"><button type="button" disabled={!canReview} onClick={() => void submitRealReview('reject')} className="text-xs px-3 py-1.5 rounded-lg border border-red-200 text-red-600 hover:bg-red-50">拒绝批次</button><button type="button" disabled={!canReview} onClick={() => void submitRealReview('approve')} className="text-xs px-3 py-1.5 rounded-lg bg-emerald-600 text-white hover:bg-emerald-700">{reviewBusy ? '提交中…' : '全部通过并入库'}</button></div>}
                  </>}
                </>
              ) : <table className="w-full text-xs bg-white border border-slate-200 rounded-lg overflow-hidden">
                <thead className="bg-slate-50">
                  <tr className="text-left text-slate-500">
                    <th className="px-3 py-2 font-medium">种子</th>
                    <th className="px-3 py-2 font-medium">覆盖场景</th>
                    <th className="px-3 py-2 font-medium">断言</th>
                    <th className="px-3 py-2 font-medium">来源</th>
                    <th className="px-3 py-2 font-medium text-right">审核</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {AUDIT_ROWS.map((ar) => (
                    <tr key={ar.id}>
                      <td className="px-3 py-2 font-mono text-emerald-700">{ar.id}</td>
                      <td className="px-3 py-2 text-slate-600">{ar.scene}</td>
                      <td className="px-3 py-2 text-slate-500">{ar.assertion}</td>
                      <td className="px-3 py-2 text-slate-400">{ar.from}</td>
                      <td className="px-3 py-2 text-right">
                        {audit[ar.id] === '通过' ? (
                          <span className="text-emerald-600 text-[11px] flex items-center justify-end gap-1"><Check className="w-3 h-3" />已通过</span>
                        ) : audit[ar.id] === '剔除' ? (
                          <span className="text-red-500 text-[11px]">已剔除</span>
                        ) : (
                          <span className="flex justify-end gap-1.5">
                            <button type="button" onClick={() => setAudit({ ...audit, [ar.id]: '通过' })}
                              className="text-[10px] px-1.5 py-0.5 rounded border border-emerald-200 text-emerald-600 hover:bg-emerald-50">通过</button>
                            <button type="button" onClick={() => setAudit({ ...audit, [ar.id]: '剔除' })}
                              className="text-[10px] px-1.5 py-0.5 rounded border border-slate-200 text-slate-400 hover:bg-red-50 hover:text-red-500">剔除</button>
                          </span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>}
            </div>
          )}

          {cur.key === 'storage' && (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-50 text-emerald-600"><Database className="w-4 h-4" /></span>
                用例入库 · {selectedAsset.name} 确认本批用例进入用例库
              </div>
              <div className="flex items-start gap-2 text-[11px] text-slate-500 bg-white border border-slate-200 rounded-lg px-3 py-2">
                <BookOpen className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
                <span>本批经审核通过 <b>{Object.values(audit).filter((v) => v === '通过').length}</b> 条种子将生成版本化用例并入库（{selectedAsset.name}@{genBranch}@{genVer}）；生成批次 #GEN-2041 将被记录，若用错分支/版本可到用例管理页回退到本次生成之前。</span>
              </div>
            </div>
          )}
        </div>

        {/* 步骤操作 */}
        <div className="mt-4 flex items-center justify-between">
          <div className="text-[11px] text-slate-400">
            {stored ? '本批次已入库并记录，可到用例管理回退' : `步骤 ${step} / ${FLOW_STEPS.length} · ${cur.label}（针对 ${selectedAsset.name}）`}
          </div>
          <div className="flex gap-2">
            {step > 1 && (
              <button type="button" onClick={() => setStep(step - 1)}
                className="flex items-center gap-1 px-3 py-1.5 text-xs border border-slate-200 rounded-lg text-slate-500 hover:bg-slate-50">
                <ChevronLeft className="w-3.5 h-3.5" />上一步
              </button>
            )}
            {step < FLOW_STEPS.length ? (
              <button type="button" onClick={() => setStep(step + 1)}
                className="flex items-center gap-1 px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg hover:bg-emerald-700">
                下一步 · {FLOW_STEPS[step]?.label}<ChevronRight className="w-3.5 h-3.5" />
              </button>
            ) : (
              <button type="button" onClick={() => { setStored(true); toast.success('用例已入库', { description: `批次 #GEN-2041 · ${selectedAsset.name}@${genBranch}@${genVer} 的用例已入库，可到用例管理查看/回退` }); }}
                className="flex items-center gap-1 px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg hover:bg-emerald-700">
                <Check className="w-3.5 h-3.5" />确认入库
              </button>
            )}
          </div>
        </div>
      </Card>

      {/* 上游源适配器明细（随当前被测对象联动） */}
      <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
        <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between flex-wrap gap-2">
          <div className="flex items-center gap-2.5">
            <h2 className="font-semibold text-slate-700 text-sm">上游源适配器 · 明细</h2>
            {!realClient && <span className="text-[11px] bg-emerald-50 text-emerald-700 px-2 py-0.5 rounded-full font-medium">被测对象 {selectedAsset.name} · 已接入 {repo?.src.length ?? 0}/{UPSTREAM_ADAPTERS.length}</span>}
          </div>
          <span className="text-[11px] text-slate-400">{realClient ? '暂无真实上游源接口' : `共 ${upFiltered.length} / ${UPSTREAM_ADAPTERS.length} 个上游源`}</span>
        </div>
        <div className="px-5 py-2.5 bg-slate-50/50 border-b border-slate-100 flex items-center gap-2 text-[11px] text-slate-500">
          <Crosshair className="w-3.5 h-3.5 text-emerald-500 shrink-0" />
          <span>适配器明细随<b>当前被测对象</b>联动：页首切换被测对象，此处即展示该对象已接入的上游源；未接入源灰显（到被测对象页配置接入）。</span>
        </div>
        {realClient ? <div className="px-5 py-8 text-center text-xs text-slate-500">当前项目接口尚未提供上游源适配器明细，页面不展示模拟数据。</div> : <>
        <div className="flex items-center justify-between px-5 pt-3">
          <ListFilter search={q} onSearch={setQ} />
        </div>
        <table className="w-full text-xs">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-slate-500">
              <th className="px-5 py-2.5 font-medium">上游源</th>
              <th className="px-5 py-2.5 font-medium">系统</th>
              <th className="px-5 py-2.5 font-medium">提取信息</th>
              <th className="px-5 py-2.5 font-medium">本次种子</th>
              <th className="px-5 py-2.5 font-medium">本对象接入</th>
              <th className="px-5 py-2.5 font-medium">状态</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {upFiltered.map((a) => {
              const attached = repo?.src.includes(a.source);
              return (
                <tr key={a.source} className={(attached ? (a.status === '验证中' ? 'bg-emerald-50/30 hover:bg-emerald-50' : 'hover:bg-slate-50') : 'opacity-45 hover:opacity-70 hover:bg-slate-50')}>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2">
                      <span className="w-5 h-5 rounded bg-slate-100 text-slate-600 flex items-center justify-center text-[10px]">源</span>
                      <span className="text-slate-700 font-medium">{a.source}</span>
                    </div>
                  </td>
                  <td className="px-5 py-3 text-slate-500">{a.system}</td>
                  <td className="px-5 py-3 text-slate-500">{a.extract}</td>
                  <td className="px-5 py-3">
                    {attached ? <span className="text-slate-600">{a.seeds}</span> : <span className="text-slate-300">—</span>}
                  </td>
                  <td className="px-5 py-3">
                    {attached
                      ? <span className="text-[10px] text-emerald-600 flex items-center gap-1"><Check className="w-3 h-3" />已接入</span>
                      : <span className="text-[10px] text-slate-300">未接入</span>}
                  </td>
                  <td className="px-5 py-3">
                    {attached ? (
                      <span className={a.status === '验证中' ? 'bg-amber-50 text-amber-600 px-2 py-0.5 rounded-full text-[10px]' : 'bg-emerald-50 text-emerald-600 px-2 py-0.5 rounded-full text-[10px]'}>
                        {a.status}
                      </span>
                    ) : (
                      <span className="bg-slate-50 text-slate-300 px-2 py-0.5 rounded-full text-[10px]">未接入</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        </>}
      </div>

      {realClient && <div className="fixed bottom-4 right-6 z-40 flex flex-wrap items-center gap-3 rounded-2xl border border-slate-200 bg-white px-4 py-3 shadow-xl shadow-slate-900/10">
        <div className="text-[11px] leading-5 text-slate-500">
          <div className="font-semibold text-slate-800">{generationScope === 'project' ? '项目全量' : generationScope === 'repo' ? '仓库全量' : '指定服务'} · {selectedServices.length} 个服务</div>
          <div>{generationScope === 'custom' ? '逐服务代码引用' : '仓库默认分支'} · 将创建 1 个生成任务 / {selectedServices.length} 个服务批次</div>
        </div>
        <button type="button" disabled={realBusy || batchPreparing || !selectedServices.length} onClick={() => void openBatchConfirmation()} className="rounded-xl bg-emerald-600 px-5 py-2.5 text-xs font-semibold text-white shadow-sm hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50">{realBusy ? '创建中…' : batchPreparing ? '准备中…' : generationScope === 'project' ? `项目全量生成 ${scopeServices.length} 个服务` : generationScope === 'repo' ? `生成该仓库 ${scopeServices.length} 个服务` : `生成 ${selectedServices.length} 个服务`}</button>
      </div>}
    </div>
  );
}

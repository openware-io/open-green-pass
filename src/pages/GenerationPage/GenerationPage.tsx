import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';
import { UPSTREAM_ADAPTERS, TESTED_REPOS, type IAssetNode } from '@/data/mock';
import { PageHeader, GhostButton, PrimaryButton, Card, ListFilter } from '@/components/shared';
import { toast } from 'sonner';
import { Sparkles, Plug, ShieldCheck, UserCheck, Database, GitBranch, BookOpen, CloudDownload, ChevronLeft, ChevronRight, Check, Crosshair, Layers } from 'lucide-react';
import { configuredGreenPassClient } from '@/api/runtime';
import type { GenerationBatch } from '@/api/client';
import { useObjectCatalog } from '@/context/objectCatalog';
type GenerationSeed = { id: number; batch_id: number; case_id?: number | null; source_locator: string; normalized_spec: Record<string, unknown>; quality_state: string; approval_state: string };

// A repository root is stored as "." for the source reader, but that is an
// implementation detail and should never be the primary UI label. Native
// select options are single-line, so use explicit Chinese wording instead of
// a punctuation-heavy concatenation of fields.
function sourceDirectoryLabel(sourcePath: string) {
  return !sourcePath || sourcePath === '.' ? '仓库根目录' : sourcePath;
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
  const [configuredModels, setConfiguredModels] = useState<Array<{ id:number; provider:string; model_key:string; status:string }>>([]);
  const [modelBinding, setModelBinding] = useState<{ provider:string; model:string } | null>(null);
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
  const repo = realClient
    ? (realRepo ? { name: selectedService?.name ?? selectedAsset.name, source: '代码仓库', url: realRepo.url, branches: [realRepo.default_branch], versions: [] as string[], src: [] as string[], owner: '' } : null)
    : mockRepo;
  // Prototype-only inputs. In real mode the backend pins the repository's
  // registered default branch and exposes no editable release-version source.
  // Keeping them isolated prevents a real UI control from implying it changes
  // the request when it does not.
  const [genBranch, setGenBranch] = useState(() => repo?.branches?.[0] ?? '');
  const [genVer, setGenVer] = useState(() => repo?.versions?.[0] ?? '');
  const generationContextKey = [selectedAsset.id, selectedService?.id ?? '', realRepo?.id ?? '', repo?.branches?.[0] ?? '', repo?.versions?.[0] ?? ''].join(':');
  const [previousGenerationContextKey, setPreviousGenerationContextKey] = useState(generationContextKey);
  if (previousGenerationContextKey !== generationContextKey) {
    setPreviousGenerationContextKey(generationContextKey);
    setGenBranch(repo?.branches?.[0] ?? '');
    setGenVer(repo?.versions?.[0] ?? '');
  }
  useEffect(() => {
    if (!realClient) return;
    let cancelled = false;
    const projectID = Number(projectId);
    void Promise.all([realClient.modelCatalog(), projectID ? realClient.getModelBinding(projectID) : Promise.resolve(null)]).then(([catalog, binding]) => {
      if (cancelled) return;
      setConfiguredModels(catalog.models.filter((model) => model.status === 'approved'));
      setModelBinding(binding ? { provider: binding.provider, model: binding.model } : null);
    }).catch(() => { if (!cancelled) { setConfiguredModels([]); setModelBinding(null); } });
    return () => { cancelled = true; };
  }, [realClient, projectId]);

  // 生成流程向导状态
  const [step, setStep] = useState(1);
  const [stored, setStored] = useState(false);
  const [audit, setAudit] = useState<Record<string, string>>({});
  const [selectedBatchId, setSelectedBatchId] = useState<number | null>(null);
  const [realBatches, setRealBatches] = useState<GenerationBatch[]>([]);
  const visibleBatches = useMemo(() => activeServiceId ? realBatches.filter((batch) => batch.service_id === activeServiceId) : realBatches, [realBatches, activeServiceId]);
  const realBatch = visibleBatches.find((batch) => batch.id === selectedBatchId) ?? visibleBatches[0] ?? null;
  const activeBatchId = realBatch?.id;
  const activeBatchStateVersion = realBatch?.state_version;
  const [selectedServiceIds, setSelectedServiceIds] = useState<number[]>([]);
  const [serviceSearch, setServiceSearch] = useState('');
  const [batchConfirmOpen, setBatchConfirmOpen] = useState(false);
  const [batchResults, setBatchResults] = useState<Array<{ serviceId: number; name: string; ok: boolean; error?: string }>>([]);
  const [realBusy, setRealBusy] = useState(false);
  const [seedResponse, setSeedResponse] = useState<{ batchId: number; stateVersion: number; items: GenerationSeed[]; error?: string } | null>(null);
  const seedsMatch = Boolean(realBatch && seedResponse?.batchId === realBatch.id && seedResponse.stateVersion === realBatch.state_version);
  const seedsLoading = Boolean(realBatch && !seedsMatch);
  const seedsError = seedsMatch ? seedResponse?.error ?? '' : '';
  const realSeeds = seedsMatch && !seedsError ? seedResponse?.items.filter((seed) => seed.batch_id === realBatch?.id) ?? [] : [];
  const [reviewBusy, setReviewBusy] = useState(false);
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
  const selectedServices = useMemo(() => confirmedServices.filter((service) => selectedServiceIds.includes(service.id)), [confirmedServices, selectedServiceIds]);
  // Batch selection is intentionally independent from the global header
  // context. Selecting several services must not make the header jump to the
  // last checkbox the user touched.
  const toggleService = (id: number) => setSelectedServiceIds((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  const openBatchConfirmation = () => {
    if (!realClient) { setStep(1); return; }
    if (!confirmedServices.length) { setRealError('请先在被测对象页完成服务技术画像确认，并启用生成测试用例。'); return; }
    if (!selectedServices.length) {
      const fallback = confirmedServices.find((service) => service.id === activeServiceId);
      if (!fallback) { setRealError('请先选择至少一个技术画像已确认且允许生成的服务。'); return; }
      setSelectedServiceIds([fallback.id]);
    } else setSelectedServiceIds(selectedServices.map((service) => service.id));
    if (!modelBinding) { setRealError('请先在 AI 模型配置中配置并绑定一个已验证模型，再生成用例'); return; }
    setBatchConfirmOpen(true);
    setRealError('');
  };
  const startRealBatch = async () => {
    if (!realClient) { setStep(1); return; }
    setRealBusy(true); setRealError('');
    try {
      const projectID = Number(projectId);
      if (!projectID) throw new Error('请先在被测对象页选择真实项目');
      if (!modelBinding) throw new Error('当前项目未绑定已验证模型，无法生成真实用例');
      const repos = realProjectRepos.length ? realProjectRepos : await realClient.listProjectRepositories(projectID);
      const services = confirmedServices.filter((service) => selectedServiceIds.includes(service.id));
      if (!services.length) throw new Error('请至少选择一个已确认的目录服务');
      const results: Array<{ serviceId: number; name: string; ok: boolean; error?: string }> = [];
      const created: GenerationBatch[] = [];
      for (const service of services) {
        try {
          const repo = repos.find((item) => item.id === service.repo_id);
          if (!repo) throw new Error('未关联代码仓库');
          const batch = await realClient.createGenerationBatch({ target_id: projectID, repo_id: repo.id, service_id: service.id, source_path: service.source_path, branch: repo.default_branch, idempotency_key: `ui-batch-${projectID}-${service.id}-${crypto.randomUUID()}` });
          created.push(batch); results.push({ serviceId: service.id, name: service.name, ok: true });
        } catch (error) { results.push({ serviceId: service.id, name: service.name, ok: false, error: error instanceof Error ? error.message : '创建失败' }); }
      }
      setBatchResults(results);
      setRealBatches((current) => [...created, ...current.filter((batch) => !created.some((item) => item.id === batch.id))]);
      if (created.length) setSelectedBatchId((created.find((batch) => batch.service_id === activeServiceId) ?? created[0]).id);
      setSelectedServiceIds([]); setBatchConfirmOpen(false); setStep(1);
      if (created.length) toast.success(`已创建 ${created.length} 个生成批次`, { description: results.filter((item) => !item.ok).length ? `${results.filter((item) => !item.ok).length} 个服务创建失败，请查看结果` : '服务端将依次生成候选用例' });
    } catch (error) { setRealError(error instanceof Error ? error.message : '创建生成批次失败'); }
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

  return (
    <div>
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
      <PageHeader title="生成用例" desc={realClient ? `选择已确认的目录服务，批量创建生成批次 · 当前可生成 ${confirmedServices.length} 个服务` : '针对当前被测对象的用例生成流程 · 适配解析 → AI 生成 → 质量验证 → 审核入库'}>
        <GhostButton onClick={() => navigate('/target')}>管理被测对象</GhostButton>
        <PrimaryButton disabled={realBusy || !realClient} onClick={openBatchConfirmation}>{realBusy ? '创建批次…' : `批量生成用例${selectedServices.length ? `（${selectedServices.length}）` : ''}`}</PrimaryButton>
      </PageHeader>
      {realClient && <div className={'mb-5 flex flex-wrap items-center gap-3 rounded-xl border px-4 py-3 text-xs ' + (modelBinding ? 'border-emerald-200 bg-emerald-50 text-emerald-800' : 'border-amber-200 bg-amber-50 text-amber-800')}><span className="font-semibold">模型生成</span>{modelBinding ? <span>当前绑定：{modelBinding.provider} / {modelBinding.model} · 已配置模型 {configuredModels.length} 个</span> : <><span>当前项目未绑定已验证模型，暂不能生成真实用例。</span><button type="button" onClick={() => navigate('/models')} className="font-medium underline">去配置模型</button></>}</div>}

      {realClient && <Card title="选择生成范围" className="p-0 mb-5 overflow-hidden">
        <div className="p-5 border-b border-slate-100 flex flex-wrap items-center gap-3">
          <div className="flex-1 min-w-[220px]"><div className="text-sm font-semibold text-slate-700">已确认服务</div><div className="text-[11px] text-slate-400 mt-1">只有“生成测试用例 = 是”的服务可以选择，批量操作会为每个服务创建独立批次。</div></div>
          <span className="rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700">已选 {selectedServices.length} / {confirmedServices.length}</span>
          <button type="button" onClick={() => setSelectedServiceIds(confirmedServices.map((service) => service.id))} className="text-xs text-emerald-700 hover:underline">全选</button>
          <button type="button" onClick={() => setSelectedServiceIds([])} className="text-xs text-slate-500 hover:underline">清空</button>
          <input value={serviceSearch} onChange={(event) => setServiceSearch(event.target.value)} placeholder="搜索服务或源码目录" className="w-full sm:w-56 rounded-lg border border-slate-200 px-3 py-2 text-xs outline-none focus:border-emerald-400" />
        </div>
        <div className="divide-y divide-slate-100 max-h-[330px] overflow-y-auto">
          {realCatalogLoading ? <div className="p-8 text-center text-xs text-slate-500">正在读取项目目录…</div> : visibleServices.length === 0 ? <div className="p-8 text-center text-xs text-slate-500">暂无符合条件的已确认服务</div> : visibleServices.map((service) => {
            const checked = selectedServiceIds.includes(service.id);
            const serviceRepo = realProjectRepos.find((item) => item.id === service.repo_id);
            const profile = profileFor(service);
            return <label key={service.id} className={'flex cursor-pointer items-start gap-3 px-5 py-3 transition ' + (checked ? 'bg-emerald-50/60' : 'hover:bg-slate-50')}>
              <input type="checkbox" checked={checked} onChange={() => toggleService(service.id)} className="mt-1 h-4 w-4 accent-emerald-600" />
              <span className="min-w-0 flex-1"><span className="flex flex-wrap items-center gap-2 text-xs font-semibold text-slate-700"><span>{service.name}</span><span className="rounded bg-emerald-50 px-1.5 py-0.5 text-[10px] font-medium text-emerald-700">生成测试用例：是</span><span className="text-[10px] font-normal text-slate-400">{service.kind}</span></span><span className="mt-1 block truncate text-[11px] text-slate-500">{sourceDirectoryLabel(service.source_path)}</span><span className="mt-1 block text-[10px] text-slate-400">语言：{profile.language} · 框架：{profile.framework} · 工具：{profile.tool}</span></span>
              <span className="hidden min-w-[180px] text-right text-[10px] text-slate-400 sm:block">{serviceRepo?.url ?? '未关联仓库'}<br /><b className="font-normal text-slate-500">{serviceRepo?.default_branch ?? '未配置分支'}</b></span>
            </label>;
          })}
        </div>
      </Card>}

      {batchConfirmOpen && <div className="mb-5 rounded-xl border border-emerald-200 bg-emerald-50/60 p-5 shadow-sm">
        <div className="flex items-start justify-between gap-3"><div><div className="text-sm font-semibold text-slate-800">确认批量创建</div><div className="mt-1 text-xs text-slate-500">将为以下 {selectedServices.length} 个服务分别创建生成批次，创建后可逐个审核。</div></div><button type="button" onClick={() => setBatchConfirmOpen(false)} className="text-xs text-slate-500 hover:text-slate-700">取消</button></div>
        <div className="mt-4 grid gap-2 md:grid-cols-2">{selectedServices.map((service) => { const serviceRepo = realProjectRepos.find((item) => item.id === service.repo_id); const profile = profileFor(service); return <div key={service.id} className="rounded-lg border border-emerald-100 bg-white px-3 py-2.5 text-xs"><div className="flex items-center justify-between gap-2"><div className="font-semibold text-slate-700">{service.name}</div><span className={'rounded-full px-2 py-0.5 text-[10px] ' + (profile.complete ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-700')}>{profile.complete ? '条件完整' : '条件不完整'}</span></div><div className="mt-1 truncate text-[11px] text-slate-500">目录：{sourceDirectoryLabel(service.source_path)}</div><div className="mt-1 text-[10px] text-slate-500">类型：{service.kind}</div><div className="mt-1 text-[10px] text-slate-500">语言：{profile.language} · 框架：{profile.framework}</div><div className="mt-1 text-[10px] text-slate-400">工具：{profile.tool} · 命令：{profile.command} · 登记分支：{serviceRepo?.default_branch ?? '未配置'}</div></div>; })}</div>
        <div className="mt-4 flex items-center justify-between gap-3"><span className="text-[11px] text-slate-500">创建后服务端会冻结已登记仓库的默认分支与服务目录；本期仅支持 HTTP/API 结构化候选。</span><button type="button" disabled={realBusy || !selectedServices.length} onClick={() => void startRealBatch()} className="rounded-lg bg-emerald-600 px-4 py-2 text-xs font-medium text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50">{realBusy ? '正在创建…' : `确认创建 ${selectedServices.length} 个批次`}</button></div>
      </div>}

      {/* 被测对象 = 全局轴心：本流程围绕当前被测对象展开（切换用页首被测对象导航） */}
      <Card title="当前被测对象 · 生成轴心" className="p-0 mb-5 overflow-hidden">
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
              {realClient && <div className="mt-3"><label htmlFor="generation-service" className="mb-1 block text-[10px] text-slate-400">查看服务上下文</label><select id="generation-service" value={selectedService?.id ?? ''} onChange={(event) => selectService(Number(event.target.value) || null)} className="w-full rounded-md border border-slate-200 bg-white px-2 py-1.5 text-xs text-slate-700"><option value="">请选择目录服务</option>{realProjectServices.map((service) => <option key={service.id} value={service.id}>{service.name}（{sourceDirectoryLabel(service.source_path)}）{service.generation_status !== 'confirmed' || service.profile_status !== 'confirmed' ? '，待确认' : ''}</option>)}</select>{selectedService && <div className={'mt-2 text-[10px] ' + (selectedServiceReady ? 'text-emerald-700' : 'text-amber-700')}>{selectedServiceReady ? '技术画像与生成范围已确认，可在上方加入批量生成范围。' : '此服务尚未完成画像确认或未启用生成；请前往被测对象页处理。'}</div>}</div>}
            </div>
          </div>

          {repo ? realClient ? (
            <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 min-w-0">
              <div className="flex items-center gap-1.5 text-xs font-medium text-slate-600"><GitBranch className="w-3.5 h-3.5 text-emerald-500" />已冻结的生成来源</div>
              <div className="mt-2 truncate text-xs text-slate-700" title={realRepo?.url}>{realRepo?.url}</div>
              <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-slate-500"><span>登记默认分支 <b className="font-mono text-slate-700">{realRepo?.default_branch || '未配置'}</b></span><span>服务目录 <b className="font-mono text-slate-700">{selectedService?.source_path || '未配置'}</b></span></div>
              <p className="mt-2 text-[10px] leading-4 text-slate-400">创建批次后服务端会冻结当前提交；分支和版本不在此页编辑，避免与已登记仓库来源不一致。</p>
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
            <span>本页只展示已登记来源。确认创建后，服务端按 <b>{realRepo?.default_branch || '默认分支'}</b> 与 <b>{selectedService?.source_path || '服务目录'}</b> 冻结源码；执行环境的版本校验在「测试执行」中显式配置。</span>
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
      </Card>

      {batchResults.length > 0 && <div className="mb-5 rounded-xl border border-slate-200 bg-white px-4 py-3 text-xs"><div className="font-semibold text-slate-700">本次创建结果</div><div className="mt-2 flex flex-wrap gap-2">{batchResults.map((result) => <span key={result.serviceId} className={'rounded-full px-2.5 py-1 ' + (result.ok ? 'bg-emerald-50 text-emerald-700' : 'bg-red-50 text-red-700')}>{result.ok ? '已创建' : '失败'} · {result.name}{result.error ? `：${result.error}` : ''}</span>)}</div></div>}
      {realClient && <Card title={activeServiceId ? '当前服务生成批次' : '项目生成批次'} className="p-0 mb-5 overflow-hidden" extra={<span className="text-[11px] text-slate-400">共 {visibleBatches.length} 个 · 自动刷新</span>}>
        <div className="divide-y divide-slate-100">{visibleBatches.map((batch) => {
          const service = realProjectServices.find((item) => item.id === batch.service_id);
          const active = realBatch?.id === batch.id;
          return <button type="button" key={batch.id} onClick={() => { setSelectedBatchId(batch.id); if (service) selectService(service.id); setStep(1); }} className={'w-full px-5 py-3 text-left transition ' + (active ? 'bg-emerald-50/60' : 'hover:bg-slate-50')}>
            <div className="flex flex-wrap items-center gap-2"><span className="text-xs font-semibold text-slate-700">{service?.name ?? `服务 #${batch.service_id ?? '未绑定'}`}</span><span className={'rounded-full px-2 py-0.5 text-[10px] font-medium ' + (batch.state === 'approved' ? 'bg-emerald-100 text-emerald-700' : batch.state === 'failed' || batch.state === 'rejected' ? 'bg-red-100 text-red-700' : 'bg-amber-100 text-amber-700')}>{STAGE_LABEL[batch.state] ?? batch.state}</span><span className="ml-auto font-mono text-[10px] text-slate-400">#{batch.id}</span></div>
            <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-[10px] text-slate-500"><span>目录：{batch.source_path || service?.source_path || '项目级目录'}</span><span>分支：{batch.branch || '未配置'}</span><span>目标版本：{batch.version || '未配置'}</span></div>
          </button>;
        })}{visibleBatches.length === 0 && <div className="px-5 py-8 text-center text-xs text-slate-500">{activeServiceId ? '当前服务暂无生成批次。选择该服务后创建批次，即可在这里查看进度和审核候选。' : '当前项目暂无生成批次。'}</div>}</div>
      </Card>}
      {realError && <div className="mb-5 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">{realError}</div>}

      {realClient && realBatch && <Card title={`真实生成流程 · ${realProjectServices.find((service) => service.id === realBatch.service_id)?.name ?? '项目级批次'} · #${realBatch.id}`} className="p-5 mb-5">
        <div className="flex flex-wrap items-center gap-2 mb-5">
          {(['enqueued', 'fetching', 'parsing', 'generating', 'quality_check', 'review', 'committing', 'approved'] as const).map((state, index) => {
            const order = ['enqueued', 'fetching', 'parsing', 'generating', 'quality_check', 'review', 'committing', 'approved'];
            const current = order.indexOf(realBatch.state);
            const done = index < current || realBatch.state === 'approved';
            const active = state === realBatch.state;
            return <div key={state} className={'flex items-center gap-1.5 text-[10px] ' + (active ? 'text-emerald-700 font-semibold' : done ? 'text-emerald-500' : 'text-slate-300')}><span className={'w-6 h-6 rounded-full flex items-center justify-center border ' + (active ? 'bg-emerald-600 text-white border-emerald-600' : done ? 'bg-emerald-50 border-emerald-200' : 'bg-white border-slate-200')}>{done ? '✓' : index + 1}</span><span>{STAGE_LABEL[state]}</span>{index < 7 && <span className="mx-1 text-slate-200">→</span>}</div>;
          })}
        </div>
        <div className="mb-4 flex flex-wrap gap-3 text-xs text-slate-500"><span>已冻结分支 <b>{realBatch.branch}</b></span><span>候选种子 <b>{seedsLoading ? '读取中…' : realSeeds.length}</b></span><span>当前阶段 <b className="text-emerald-700">{STAGE_LABEL[realBatch.state] ?? realBatch.state}</b></span></div>
        {seedsError ? <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-5 text-xs text-red-700">候选用例读取失败：{seedsError}。页面将自动重试，读取成功前不能审核或执行。</div> : seedsLoading ? <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-5 text-xs text-slate-500">正在读取当前批次候选用例…</div> : realSeeds.length === 0 ? <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-5 text-xs text-slate-500">{['failed', 'rejected', 'rolled_back', 'approved', 'review'].includes(realBatch.state) ? '本批次没有可展示的候选用例，不能审核或执行。' : '正在生成候选用例，页面会自动更新进度。'}</div> : <GeneratedSeedReviewList seeds={realSeeds} />}
        {realBatch.state === 'review' && <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3"><p className="text-xs font-medium text-amber-800">请先逐条展开核对请求、断言与源码证据，再决定本批次是否入库。</p><p className="mt-1 text-[11px] text-amber-700">当前版本按批次整体审核；“审核通过”会把全部通过质量门的候选写入用例库，拒绝不会写入正式用例。</p><div className="mt-3 flex justify-end gap-2"><button type="button" disabled={!canReview} onClick={() => void submitRealReview('reject')} className="text-xs px-3 py-1.5 rounded-lg border border-red-200 bg-white text-red-600 hover:bg-red-50">拒绝该批次</button><button type="button" disabled={!canReview} onClick={() => void submitRealReview('approve')} className="text-xs px-3 py-1.5 rounded-lg bg-emerald-600 text-white hover:bg-emerald-700">{reviewBusy ? '提交中…' : '确认审核通过并入库'}</button></div></div>}
        {realBatch.state === 'approved' && <div className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-xs text-emerald-700"><span>本批次已审核通过，候选已写入当前项目的版本化用例库；进入用例库或执行页会保留本项目上下文。</span><div className="flex flex-wrap gap-2"><button type="button" onClick={() => navigate(`/cases?target_id=${realBatch.target_id}`)} className="rounded-lg border border-emerald-300 bg-white px-3 py-1.5 font-medium text-emerald-700 hover:bg-emerald-50">查看本项目用例库</button><button type="button" disabled={seedsLoading || Boolean(seedsError) || !realSeeds.some((seed) => seed.case_id)} onClick={() => { const ids = realSeeds.flatMap((seed) => seed.case_id ? [seed.case_id] : []); if (ids.length) navigate(`/exec?target_id=${realBatch.target_id}&case_ids=${ids.join(',')}`); }} className="rounded-lg bg-emerald-600 px-3 py-1.5 font-medium text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50">执行本批用例</button></div></div>}
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
    </div>
  );
}

import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { FLOW_ITEMS, EXEC_RUN, TEST_SCENARIOS, ENV_RUNTIME_VERSIONS, type IEnvVersionCheck } from '@/data/mock';
import { PageHeader, Card, ListFilter } from '@/components/shared';
import { configuredGreenPassClient, greenPassConnectionHint } from '@/api/runtime';
import { useObjectCatalog } from '@/context/objectCatalog';
import type { CaseResult, Run, Target, TestCase, EnvRuntime } from '@/api/client';
import { Cpu, Globe, Smartphone, Sparkles, Play, Pause, ShieldCheck, ShieldAlert, RefreshCw, Server, WifiOff, Bug, ArrowRight } from 'lucide-react';
import { toast } from 'sonner';

const STATUS_STYLE: Record<string, { dot: string; badge: string; text: string }> = {
  '通过': { dot: 'bg-emerald-500', badge: 'text-emerald-600 bg-emerald-50', text: '通过' },
  '失败': { dot: 'bg-red-500', badge: 'text-red-600 bg-red-50', text: '失败' },
  '执行中': { dot: 'bg-emerald-500', badge: 'text-emerald-600 bg-emerald-100', text: '执行中' },
  '阻塞': { dot: 'bg-amber-500', badge: 'text-amber-600 bg-amber-50', text: '阻塞' },
};
const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles };
const REAL_STATE_STYLE: Record<Run['state'], { label: string; className: string }> = {
  queued: { label: '排队中', className: 'bg-amber-50 text-amber-700' },
  version_check: { label: '版本校验中', className: 'bg-amber-50 text-amber-700' },
  scheduled: { label: '待执行', className: 'bg-amber-50 text-amber-700' },
  running: { label: '执行中', className: 'bg-emerald-100 text-emerald-700' },
  paused: { label: '已暂停', className: 'bg-amber-50 text-amber-700' },
  collect: { label: '收集证据', className: 'bg-emerald-50 text-emerald-700' },
  gate: { label: '门禁判定', className: 'bg-emerald-50 text-emerald-700' },
  report: { label: '生成报告', className: 'bg-emerald-50 text-emerald-700' },
  done: { label: '已完成', className: 'bg-emerald-50 text-emerald-700' },
  failed: { label: '已失败/阻断', className: 'bg-red-50 text-red-700' },
};
const TRIAGE_CANDIDATES = [
  { id: 'F-1042', severity: 'P0', kind: '产品缺陷', title: '支付回调验签失败仍返回 500', module: 'payment', evidence: '4/4', status: '待确认' },
  { id: 'F-1041', severity: 'P1', kind: '产品缺陷', title: 'confirmUsername 不一致仍删除用户', module: 'user', evidence: '4/4', status: '已确认' },
  { id: 'F-1036', severity: 'P2', kind: '环境问题', title: '预签名 403，分片上传无法开始', module: 'media', evidence: '4/4', status: '待回归' },
  { id: 'F-1034', severity: 'P3', kind: '用例缺陷', title: '分页越界用例数据非法（误报）', module: 'message', evidence: '4/4', status: '误报' },
];

function caseResultStyle(status: CaseResult['status']): string {
  if (status === 'pass') return 'bg-emerald-50 text-emerald-700';
  if (status === 'fail' || status === 'blocked') return 'bg-red-50 text-red-700';
  return 'bg-amber-50 text-amber-700';
}

type EnvironmentRequirement = NonNullable<EnvRuntime['requirements']>[number];

function requirementsFor(service: { kind: string; language: string; framework: string; profile_evidence?: string[] } | undefined): EnvironmentRequirement[] {
  const evidence = service?.profile_evidence?.[0] ?? '来自当前服务的技术画像与已选测试用例。';
  const base: EnvironmentRequirement[] = [
    { key: 'base_url', label: '运行入口地址', required: true, reason: '所有测试请求或浏览器访问必须指向用户确认的已运行环境。', evidence },
    { key: 'runtime_version', label: '环境运行版本', required: true, reason: '执行前需与目标代码版本比对，避免在错误版本上得到失真的结果。', evidence },
  ];
  if (service?.kind === 'web_app') return [...base,
    { key: 'credential_ref', label: '测试账号凭据引用', required: true, sensitive: true, reason: 'Web 关键旅程通常包含登录与权限校验；只填写密钥管理器中的引用名，不填写密码。', evidence },
    { key: 'browser_profile', label: '浏览器配置', required: true, reason: 'Web UI 用例需要指定受管浏览器、视口、语言和时区。', evidence },
    { key: 'test_data_profile', label: '测试数据上下文', required: true, reason: '租户、角色和预置数据会影响页面可见性与业务结果。', evidence },
  ];
  return base;
}

/** 执行任务 → 所属测试场景（清单归属标注，按标题/被测对象关键词映射） */
function flowScenario(title: string, asset: string): string {
  if (title.includes('E2E') || asset.includes('web') || asset.includes('frontend')) return 'SCEN-08';
  if (title.includes('压力测试') || title.includes('并发')) return 'SCEN-05';
  if (title.includes('真机') || asset.includes('mobile')) return 'SCEN-09';
  return 'SCEN-02';
}

export default function ExecPage() {
  const catalog = useObjectCatalog();
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedTarget = searchParams.get('target_id') ?? '';
  const requestedCases = searchParams.get('case_ids') ?? '';
  const requestKey = `${requestedTarget}:${requestedCases}`;
  const appliedRequest = useRef<{ key: string; pendingProject: string | null } | null>(null);
  const { enabled, loaded, projects, projectId, selectProject } = catalog;

  useEffect(() => {
    if (!enabled || !loaded) return;
    if (appliedRequest.current?.key !== requestKey) {
      appliedRequest.current = { key: requestKey, pendingProject: null };
      if (requestedTarget) {
        if (projects.some((project) => String(project.id) === requestedTarget)) {
          if (projectId !== requestedTarget) {
            appliedRequest.current.pendingProject = requestedTarget;
            selectProject(requestedTarget);
          }
        } else {
          toast.error('生成批次的被测项目不可用', { description: '请重新选择项目和要执行的用例。' });
        }
      }
      return;
    }
    if (appliedRequest.current.pendingProject) {
      if (projectId !== appliedRequest.current.pendingProject) return;
      appliedRequest.current.pendingProject = null;
    }
    // A header selection supersedes the original batch link. Remove the old
    // scope so returning to that project cannot silently restore its case set.
    if (requestedTarget && projectId !== requestedTarget) {
      setSearchParams((current) => {
        const next = new URLSearchParams(current);
        next.delete('target_id');
        next.delete('case_ids');
        return next;
      }, { replace: true });
    }
  }, [enabled, loaded, projects, projectId, selectProject, requestedTarget, requestKey, setSearchParams]);

  const requestedCaseIDs = useMemo(() => projectId === requestedTarget
    ? [...new Set(requestedCases.split(',').map(Number).filter((value) => Number.isSafeInteger(value) && value > 0))]
    : [], [projectId, requestedTarget, requestedCases]);
  const changeTarget = (value: string) => {
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      next.delete('target_id');
      next.delete('case_ids');
      return next;
    }, { replace: true });
    selectProject(value);
  };
  return <ExecPageContent key={`${projectId}:${requestKey}`} targets={projects} targetID={projectId} requestedCaseIDs={requestedCaseIDs} onTargetChange={changeTarget} />;
}

function ExecPageContent({ targets, targetID, requestedCaseIDs, onTargetChange }: {
  targets: Target[];
  targetID: string;
  requestedCaseIDs: number[];
  onTargetChange: (targetID: string) => void;
}) {
  const catalog = useObjectCatalog();
  const navigate = useNavigate();
  const client = useMemo(() => configuredGreenPassClient(), []);
  const [paused, setPaused] = useState(false);
  const [q, setQ] = useState('');
  const [status, setStatus] = useState('');
  const kw = q.trim().toLowerCase();
  const filtered = FLOW_ITEMS.filter((it) => {
    if (status && it.status !== status) return false;
    if (kw && !(it.id + it.title + it.asset).toLowerCase().includes(kw)) return false;
    return true;
  });
  const stats = {
    pass: EXEC_RUN.passed,
    fail: EXEC_RUN.failed,
    queued: EXEC_RUN.queued,
    running: EXEC_RUN.running,
    blocked: EXEC_RUN.blocked,
  };

  const [envChecks, setEnvChecks] = useState<IEnvVersionCheck[]>(ENV_RUNTIME_VERSIONS);
  const [cases, setCases] = useState<TestCase[]>([]);
  const [casesLoading, setCasesLoading] = useState(Boolean(client && targetID));
  const [casesError, setCasesError] = useState('');
  const [selectedCaseIDs, setSelectedCaseIDs] = useState<number[]>([]);
  const [run, setRun] = useState<Run>();
  const [results, setResults] = useState<CaseResult[]>([]);
  const [scenarioID, setScenarioID] = useState('1');
  const [environment, setEnvironment] = useState('test');
  const [targetVersion, setTargetVersion] = useState('');
  const [runtimeVersion, setRuntimeVersion] = useState('');
  const [environmentBaseURL, setEnvironmentBaseURL] = useState('');
  const [credentialRef, setCredentialRef] = useState('');
  const [browserProfile, setBrowserProfile] = useState('Chrome · 1440×900 · zh-CN · Asia/Shanghai');
  const [testDataProfile, setTestDataProfile] = useState('');
  const [requirementsConfirmed, setRequirementsConfirmed] = useState(false);
  const [registeredRuntimeKey, setRegisteredRuntimeKey] = useState('');
  const [targetBranch, setTargetBranch] = useState('main');
  const [busy, setBusy] = useState(false);
  const activeService = catalog.services.find((service) => service.id === catalog.serviceId);
  const environmentRequirements = useMemo(() => requirementsFor(activeService), [activeService]);
  const [executionMode, setExecutionMode] = useState<'web' | 'api'>('api');
  const requiresBrowserRunner = executionMode === 'web';
  const runnableCases = useMemo(
    () => cases.filter((testCase) => executionMode === 'web' ? ['ui', 'web'].includes(testCase.kind) : testCase.kind === 'api'),
    [cases, executionMode],
  );

  useEffect(() => {
    setSelectedCaseIDs((current) => current.filter((id) => runnableCases.some((testCase) => testCase.id === id)));
  }, [runnableCases]);

  useEffect(() => {
    if (!activeService) return;
    setExecutionMode(activeService.kind === 'web_app' ? 'web' : 'api');
  }, [activeService?.id, activeService?.kind]);

  useEffect(() => {
    if (!client || !targetID) return;
    let active = true;
    client.listCases({ target_id: Number(targetID) })
      .then((loadedCases) => {
        if (!active) return;
        setCases(loadedCases);
        const available = new Set(loadedCases.map((testCase) => testCase.id));
        setSelectedCaseIDs(requestedCaseIDs.filter((id) => available.has(id)));
      })
      .catch((error: unknown) => {
        if (active) setCasesError(error instanceof Error ? error.message : '无法读取当前项目用例');
      })
      .finally(() => { if (active) setCasesLoading(false); });
    return () => { active = false; };
  }, [client, targetID, requestedCaseIDs]);

  useEffect(() => {
    if (!client || !run?.id) return;
    const controller = new AbortController();
    void client.streamRunEvents(run.id, (next) => {
      setRun(next);
      if (next.state === 'done' || next.state === 'failed') {
        void client.caseResults(next.id).then(setResults).catch(() => undefined);
        controller.abort();
      }
    }, controller.signal).catch((error: unknown) => {
      if (!(error instanceof DOMException && error.name === 'AbortError')) {
        toast.error('真实运行事件流已断开', { description: error instanceof Error ? error.message : '未知错误' });
      }
    });
    return () => controller.abort();
  }, [client, run?.id]);

  const refreshRun = async (runID: number) => {
    if (!client) return;
    const [current, caseResults] = await Promise.all([client.getRun(runID), client.caseResults(runID)]);
    setRun(current);
    setResults(caseResults);
  };

  const registerEnvironment = async () => {
    if (!client) return;
    const parsedTargetID = Number(targetID);
    const version = runtimeVersion.trim();
    if (!Number.isSafeInteger(parsedTargetID) || parsedTargetID <= 0 || !environment.trim() || !version || !environmentBaseURL.trim() || !requirementsConfirmed) {
      toast.error('请完成环境需求确认，并填写环境地址、版本和标识');
      return;
    }
    setBusy(true);
    try {
      await client.registerRuntime(parsedTargetID, { env: environment.trim(), version, base_url: environmentBaseURL.trim(), requirements: environmentRequirements, confirmation: { confirmed: true, values: { base_url: environmentBaseURL.trim(), runtime_version: version }, credential_ref: credentialRef.trim() || undefined, browser_profile: browserProfile.trim() || undefined, test_data_profile: testDataProfile.trim() || undefined } });
      setRegisteredRuntimeKey(`${parsedTargetID}|${environment.trim()}|${version}|${environmentBaseURL.trim()}`);
      toast.success('测试环境地址与版本已登记', { description: '生成的 API 用例只会请求此处登记的环境 origin。' });
    } catch (error) {
      toast.error('环境登记失败', { description: error instanceof Error ? error.message : '未知错误' });
    } finally { setBusy(false); }
  };

  const submitRealRun = async () => {
    if (!client) return;
    const parsedTargetID = Number(targetID);
    const parsedScenarioID = Number(scenarioID);
    if (!Number.isSafeInteger(parsedTargetID) || parsedTargetID <= 0 || !Number.isSafeInteger(parsedScenarioID) || parsedScenarioID <= 0 || !targetVersion.trim()) {
      toast.error('请填写有效的被测对象、场景 ID 与目标版本');
      return;
    }
    if (casesLoading || casesError || selectedCaseIDs.length === 0 || selectedCaseIDs.some((id) => !runnableCases.some((testCase) => testCase.id === id))) {
      toast.error('请先选择当前项目要执行的用例', { description: '空选择不会自动扩大为全量执行。' });
      return;
    }
    if (requestedCaseIDs.length > 0) {
      const selected = new Set(selectedCaseIDs);
      const missing = requestedCaseIDs.filter((id) => !selected.has(id));
      if (missing.length > 0) {
        toast.error('生成批次用例未完整选中', { description: `请确认 ${missing.length} 条本批次用例仍在当前目标下可用。` });
        return;
      }
      const runtimeVersionValue = runtimeVersion.trim();
      const runtimeKey = `${parsedTargetID}|${environment.trim()}|${runtimeVersionValue}|${environmentBaseURL.trim()}`;
      if (!runtimeVersionValue || !environmentBaseURL.trim() || registeredRuntimeKey !== runtimeKey) {
        toast.error('请先登记测试环境地址与运行版本', { description: '本批次 API 用例不会使用模型生成的主机地址。' });
        return;
      }
    }
    setBusy(true);
    try {
      const created = await client.createRun({ target_id: parsedTargetID, scenario_id: parsedScenarioID, env: environment.trim(), target_version: targetVersion.trim(), target_branch: targetBranch.trim() || undefined, selected_case_ids: selectedCaseIDs });
      setRun(created);
      setResults([]);
      toast.success(`已创建真实运行 #${created.id}`, { description: '调度工作流将自动完成版本校验、用例执行、证据收集与报告生成。' });
    } catch (error) {
      toast.error('创建运行失败', { description: error instanceof Error ? error.message : '未知错误' });
    } finally { setBusy(false); }
  };

  const invokeRunAction = async (action: 'pause' | 'resume') => {
    if (!client || !run) return;
    setBusy(true);
    try {
      if (action === 'pause') setRun(await client.pauseRun(run.id));
      if (action === 'resume') setRun(await client.resumeRun(run.id));
      await refreshRun(run.id);
      toast.success('真实运行状态已刷新');
    } catch (error) {
      toast.error('运行操作失败', { description: error instanceof Error ? error.message : '未知错误' });
    } finally { setBusy(false); }
  };
  const recheck = () => {
    setEnvChecks(envChecks.map((v) => (v.asset === 'svc-payment' ? { ...v, runtime: 'v2.4.1', matched: true } : v)));
    toast.success('版本校验通过', { description: 'svc-payment 已部署 v2.4.1 与目标一致，可放行执行' });
  };

  const togglePause = () => {
    setPaused(!paused);
    toast.success(paused ? '已恢复全部执行器（原型模拟）' : '已暂停全部执行器（原型模拟）');
  };

  return (
    <div>
      <PageHeader title="测试执行" desc="执行总览清单 · 跨被测对象执行 · 实时进度 · 证据捕获 · 不放水不跳过">
        {!client && <div className="flex items-center gap-3">
          <span className="flex items-center gap-2 text-xs text-slate-500">
            <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span> 12 个执行器在线
          </span>
          <button onClick={togglePause}
            className={'px-3 py-1.5 text-sm border rounded-lg flex items-center gap-1.5 ' + (paused ? 'bg-emerald-600 text-white border-emerald-600' : 'border-slate-300 hover:bg-slate-50 bg-white text-slate-600')}>
            {paused ? <Play className="w-3.5 h-3.5" /> : <Pause className="w-3.5 h-3.5" />}{paused ? '恢复全部' : '暂停全部'}
          </button>
        </div>}
      </PageHeader>

      <Card title="缺陷候选与归因" className="mb-5" extra={<span className="inline-flex items-center gap-1.5 text-[11px] text-emerald-700"><ShieldCheck className="h-3.5 w-3.5" />源码只读 · 失败先归因，不自动改源码</span>}>
        <div className="p-5">
          <div className="space-y-2">
            {TRIAGE_CANDIDATES.map((item) => <div key={item.id} className="flex flex-wrap items-center gap-3 rounded-lg border border-slate-200 bg-slate-50/60 px-3 py-2.5">
              <span className={'rounded-md border px-2 py-0.5 text-[10px] font-semibold ' + (item.severity === 'P0' ? 'border-red-200 bg-red-50 text-red-700' : item.severity === 'P1' ? 'border-amber-200 bg-amber-50 text-amber-700' : 'border-slate-200 bg-white text-slate-600')}>{item.severity}</span>
              <span className="font-mono text-[10px] text-slate-400">{item.id}</span>
              <span className="text-xs font-medium text-slate-700">{item.title}</span>
              <span className="font-mono text-[10px] text-slate-400">{item.module}</span>
              <span className={'rounded-full px-2 py-0.5 text-[10px] ' + (item.kind === '产品缺陷' ? 'bg-red-50 text-red-700' : item.kind === '环境问题' ? 'bg-sky-50 text-sky-700' : 'bg-slate-100 text-slate-600')}>{item.kind}</span>
              <span className="text-[10px] text-slate-400">证据 {item.evidence}</span>
              <span className="ml-auto rounded-full bg-white px-2 py-0.5 text-[10px] text-slate-500">{item.status}</span>
            </div>)}
          </div>
          <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
            <span className="text-[11px] text-slate-400">失败 5 条 · 已归因 5/5 · 确认缺陷 2 · 待确认 1</span>
            <button onClick={() => navigate('/findings')} className="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 bg-white px-3 py-1.5 text-xs text-emerald-700 hover:bg-emerald-50"><Bug className="h-3.5 w-3.5" />进入缺陷中心<ArrowRight className="h-3.5 w-3.5" /></button>
          </div>
        </div>
      </Card>

      <Card title="真实运行控制" className="mb-5" extra={
        <span className={'inline-flex items-center gap-1.5 text-[11px] px-2 py-1 rounded-md ' + (client ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-500')}>
          {client ? <Server className="w-3.5 h-3.5" /> : <WifiOff className="w-3.5 h-3.5" />}
          {client ? '真实 API 已启用' : '原型模式'}
        </span>
      }>
        <div className="p-5">
          {client ? (
            <>
              <p className="text-[11px] text-slate-500 mb-4">连接：{greenPassConnectionHint()}。仅发送 OpenAPI 定义的数值 ID 与版本字段；页面不会将原型数据写入后端。</p>
              <div className="mb-4 flex flex-wrap items-center gap-3 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
                <span className="text-[11px] font-medium text-slate-600">执行面</span>
                <div className="inline-flex rounded-lg border border-slate-200 bg-white p-0.5">
                  <button type="button" onClick={() => setExecutionMode('web')} className={'rounded-md px-3 py-1 text-[11px] ' + (executionMode === 'web' ? 'bg-emerald-600 text-white' : 'text-slate-600')}>Web UI</button>
                  <button type="button" onClick={() => setExecutionMode('api')} className={'rounded-md px-3 py-1 text-[11px] ' + (executionMode === 'api' ? 'bg-emerald-600 text-white' : 'text-slate-600')}>API 接口</button>
                </div>
                <span className="text-[11px] text-slate-400">{executionMode === 'web' ? 'PC Admin 真实测试：浏览器登录、点击、填表、截图' : '接口测试：HTTP 请求 / 响应断言，不代表 UI 通过'}</span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-5 gap-3">
                <label className="text-xs text-slate-600">被测对象
                  <select value={targetID} onChange={(event) => onTargetChange(event.target.value)} disabled={busy}
                    className="mt-1 block w-full rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs focus:border-emerald-400 focus:outline-none disabled:bg-slate-50">
                    <option value="">选择真实对象</option>
                    {targets.map((target) => <option key={target.id} value={target.id}>{target.name} · #{target.id}</option>)}
                  </select>
                </label>
                <label className="text-xs text-slate-600">场景 ID
                  <input value={scenarioID} onChange={(event) => setScenarioID(event.target.value)} inputMode="numeric" disabled={busy}
                    className="mt-1 block w-full rounded-lg border border-slate-200 px-2.5 py-2 text-xs focus:border-emerald-400 focus:outline-none disabled:bg-slate-50" />
                </label>
                <label className="text-xs text-slate-600">环境
                  <input value={environment} onChange={(event) => setEnvironment(event.target.value)} disabled={busy}
                    className="mt-1 block w-full rounded-lg border border-slate-200 px-2.5 py-2 text-xs focus:border-emerald-400 focus:outline-none disabled:bg-slate-50" />
                </label>
                <label className="text-xs text-slate-600">环境运行版本
                  <input value={runtimeVersion} onChange={(event) => setRuntimeVersion(event.target.value)} placeholder="如 v1.0.0" disabled={busy}
                    className="mt-1 block w-full rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs focus:border-emerald-400 focus:outline-none disabled:bg-slate-50" />
                </label>
                <label className="text-xs text-slate-600">目标版本
                  <input value={targetVersion} onChange={(event) => setTargetVersion(event.target.value)} placeholder="如 v1.0.0" disabled={busy}
                    className="mt-1 block w-full rounded-lg border border-slate-200 px-2.5 py-2 text-xs focus:border-emerald-400 focus:outline-none disabled:bg-slate-50" />
                </label>
                <label className="text-xs text-slate-600">目标分支
                  <input value={targetBranch} onChange={(event) => setTargetBranch(event.target.value)} disabled={busy}
                    className="mt-1 block w-full rounded-lg border border-slate-200 px-2.5 py-2 text-xs focus:border-emerald-400 focus:outline-none disabled:bg-slate-50" />
                </label>
                <label className="text-xs text-slate-600 md:col-span-2">环境 Base URL（生成 API 用例的唯一目标）
                  <input value={environmentBaseURL} onChange={(event) => { setEnvironmentBaseURL(event.target.value); setRegisteredRuntimeKey(''); }} placeholder="https://qa.example.com" disabled={busy}
                    className="mt-1 block w-full rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs focus:border-emerald-400 focus:outline-none disabled:bg-slate-50" />
                </label>
              </div>
              <details className="mt-4 rounded-xl border border-emerald-200 bg-emerald-50/40">
                <summary className="cursor-pointer px-4 py-3 text-xs font-medium text-slate-700">环境与凭据 · 点击展开</summary>
                <div className="px-4 pb-4">
                <div className="flex flex-wrap items-start justify-between gap-2"><div><h3 className="text-sm font-semibold text-slate-700">执行环境需求清单</h3><p className="mt-1 text-[11px] leading-5 text-slate-500">系统根据当前服务的技术画像生成待确认项。AI 只说明需要什么及依据，不猜测真实地址、账号或数据。</p></div><span className="rounded-full bg-white px-2 py-1 text-[10px] font-medium text-emerald-700">{activeService ? `${activeService.name} · ${activeService.kind}` : '请选择目录服务以获得更精确建议'}</span></div>
                <div className="mt-3 grid gap-2 md:grid-cols-2">{environmentRequirements.map((requirement) => <div key={requirement.key} className="rounded-lg border border-emerald-100 bg-white p-3"><div className="flex items-center gap-2 text-xs font-medium text-slate-700">{requirement.label}{requirement.required && <span className="text-red-500">*</span>}{requirement.sensitive && <span className="rounded bg-amber-50 px-1.5 py-0.5 text-[10px] font-normal text-amber-700">仅引用</span>}</div><p className="mt-1 text-[11px] leading-5 text-slate-500">{requirement.reason}</p><p className="mt-1 text-[10px] text-slate-400">依据：{requirement.evidence}</p></div>)}</div>
                {requiresBrowserRunner && <div className="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] leading-5 text-amber-800"><b>UI 执行边界：</b>pc-admin 是 Web 应用。环境登记完成后由 BrowserRunner（Playwright）运行登录、页面交互和截图；HTTP 仅用于 API 用例，不能作为 UI 通过结果。</div>}
                <div className="mt-3 grid gap-3 md:grid-cols-3">
                  {environmentRequirements.some((item) => item.key === 'credential_ref') && <label className="text-xs text-slate-600">测试账号凭据引用（不填密码）<input value={credentialRef} onChange={(event) => { setCredentialRef(event.target.value); setRequirementsConfirmed(false); }} placeholder="如 vault://qa/pc-admin/admin" disabled={busy} className="mt-1 block w-full rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs focus:border-emerald-400 focus:outline-none" /></label>}
                  {environmentRequirements.some((item) => item.key === 'browser_profile') && <label className="text-xs text-slate-600">浏览器配置<input value={browserProfile} onChange={(event) => { setBrowserProfile(event.target.value); setRequirementsConfirmed(false); }} disabled={busy} className="mt-1 block w-full rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs focus:border-emerald-400 focus:outline-none" /></label>}
                  {environmentRequirements.some((item) => item.key === 'test_data_profile') && <label className="text-xs text-slate-600">测试数据上下文<input value={testDataProfile} onChange={(event) => { setTestDataProfile(event.target.value); setRequirementsConfirmed(false); }} placeholder="如 qa 租户 / 管理员角色 / 初始化数据集" disabled={busy} className="mt-1 block w-full rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs focus:border-emerald-400 focus:outline-none" /></label>}
                </div>
                <label className="mt-3 flex cursor-pointer items-start gap-2 text-[11px] text-slate-600"><input type="checkbox" checked={requirementsConfirmed} onChange={(event) => setRequirementsConfirmed(event.target.checked)} disabled={busy} className="mt-0.5 accent-emerald-600" /><span>我已确认以上真实环境信息；AI 推断仅作为建议。凭据字段填写的是密钥管理器引用，不包含密码、Token 或 Cookie。</span></label>
              </div></details>
              <div className="mt-3 rounded-lg border border-slate-200 p-3">
                <div className="mb-2 flex items-center justify-between text-[11px] text-slate-500"><span>{requestedCaseIDs.length ? `本批次用例 · 共 ${requestedCaseIDs.length} 条` : '本次用例 · 请明确选择执行范围'}</span><div className="flex items-center gap-3"><span>已选 {selectedCaseIDs.length} / {runnableCases.length}</span><button type="button" disabled={busy || casesLoading || Boolean(casesError) || runnableCases.length === 0} onClick={() => setSelectedCaseIDs(runnableCases.map((testCase) => testCase.id))} className="text-emerald-700 hover:underline disabled:opacity-50">全选当前项目</button></div></div>
                {casesLoading && <p className="text-xs text-slate-400">正在读取当前项目用例…</p>}
                {casesError && <p className="text-xs text-red-600">用例读取失败：{casesError}</p>}
                {!casesLoading && !casesError && runnableCases.length === 0 && <p className="text-xs text-slate-500">{executionMode === 'web' ? '当前没有 Web UI 用例；请先在用例生成页生成浏览器交互用例。API 用例不会作为 UI 通过结果。' : '当前没有 API 用例。'}</p>}
                <div className="max-h-32 space-y-1 overflow-auto">{runnableCases.map((testCase) => <label key={testCase.id} className="flex items-center gap-2 text-xs text-slate-600"><input type="checkbox" checked={selectedCaseIDs.includes(testCase.id)} onChange={() => setSelectedCaseIDs((current) => current.includes(testCase.id) ? current.filter((id) => id !== testCase.id) : [...current, testCase.id])} disabled={busy} className="accent-emerald-600" /><span className="font-mono">#{testCase.id}</span><span>{testCase.code} · {testCase.title}</span></label>)}</div>
              </div>
              <div className="mt-4 flex flex-wrap gap-2">
                <button type="button" onClick={() => void registerEnvironment()} disabled={busy}
                  className={'px-3 py-1.5 rounded-lg text-sm border ' + (registeredRuntimeKey ? 'border-emerald-300 bg-emerald-50 text-emerald-700' : 'border-slate-300 text-slate-700 hover:bg-slate-50')}>{registeredRuntimeKey ? '环境已登记' : '登记环境地址'}</button>
                <button type="button" onClick={submitRealRun} disabled={busy || casesLoading || Boolean(casesError) || selectedCaseIDs.length === 0 || runnableCases.length === 0}
                  className="px-3 py-1.5 rounded-lg text-sm bg-emerald-600 text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60">创建真实运行</button>
                <button type="button" onClick={() => run && invokeRunAction('pause')} disabled={!run || busy || run.state !== 'running'}
                  className="px-3 py-1.5 rounded-lg text-sm border border-slate-300 text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50">暂停</button>
                <button type="button" onClick={() => run && invokeRunAction('resume')} disabled={!run || busy || run.state !== 'paused'}
                  className="px-3 py-1.5 rounded-lg text-sm border border-slate-300 text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50">恢复</button>
                <button type="button" onClick={() => run && refreshRun(run.id).catch((error: unknown) => toast.error('刷新失败', { description: error instanceof Error ? error.message : '未知错误' }))} disabled={!run || busy}
                  className="px-3 py-1.5 rounded-lg text-sm border border-slate-300 text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50">刷新状态</button>
              </div>
              <div className="mt-2 text-[11px] text-slate-400">创建运行是本次执行的显式确认。随后由调度工作流自动完成版本校验、执行、证据收集和报告；页面只监听状态，不重复触发步骤。</div>
              {run && (
                <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600">
                  <div className="flex flex-wrap items-center gap-2"><span className="font-mono text-emerald-700">RUN #{run.id}</span><span className={'rounded-full px-2 py-0.5 text-[11px] font-medium ' + REAL_STATE_STYLE[run.state].className}>{REAL_STATE_STYLE[run.state].label}</span><span>目标 #{run.target_id} · {run.env} · {run.target_version}</span></div>
                  <div className="mt-1 text-[11px] text-slate-400">本次明确选择 {run.selected_cases.length} 个用例。</div>
                </div>
              )}
              {results.length > 0 && (
                <div className="mt-3 overflow-x-auto rounded-lg border border-slate-200">
                  <table className="w-full text-xs"><thead className="bg-slate-50 text-left text-slate-500"><tr><th className="px-3 py-2 font-medium">用例</th><th className="px-3 py-2 font-medium">版本</th><th className="px-3 py-2 font-medium">结果</th><th className="px-3 py-2 font-medium">尝试</th><th className="px-3 py-2 font-medium">说明</th></tr></thead>
                    <tbody className="divide-y divide-slate-100">{results.map((result) => <tr key={result.id}><td className="px-3 py-2 font-mono">#{result.case_id}</td><td className="px-3 py-2">v{result.case_version}</td><td className="px-3 py-2"><span className={'rounded-full px-2 py-0.5 ' + caseResultStyle(result.status)}>{result.status}</span></td><td className="px-3 py-2">{result.attempt_seq}</td><td className="px-3 py-2 text-slate-500">{result.result_text || '—'}</td></tr>)}</tbody>
                  </table>
                </div>
              )}
            </>
          ) : (
            <p className="text-sm text-slate-500">配置 <code className="rounded bg-slate-100 px-1.5 py-0.5">VITE_GP_API_BASE</code> 与数值型 <code className="rounded bg-slate-100 px-1.5 py-0.5">VITE_GP_TEAM_ID</code> 后，本区才会启用真实后端调用。当前下面的运行总览保留为原型展示，不会发起伪请求。</p>
          )}
        </div>
      </Card>

      {/* 本次执行总览 */}
      <div className={'card bg-white rounded-xl border border-slate-200 p-5 mb-5 ' + (client ? 'hidden' : '')}>
        <div className="flex items-center justify-between mb-3">
          <div>
            <span className="text-sm font-semibold text-slate-700">{EXEC_RUN.label}</span>
            <span className="ml-3 text-xs text-slate-500">{EXEC_RUN.branch}</span>
          </div>
          <span className="text-sm font-bold text-emerald-600">{EXEC_RUN.passed + EXEC_RUN.failed + EXEC_RUN.running + EXEC_RUN.blocked} / {EXEC_RUN.total}</span>
        </div>
        <div className="h-3 bg-slate-100 rounded-full overflow-hidden">
          <div className="h-full bg-gradient-to-r from-emerald-500 to-green-500 rounded-full" style={{ width: `${EXEC_RUN.progress}%` }} />
        </div>
        <div className="flex justify-between mt-3 text-xs">
          <div className="flex gap-5">
            <span className="text-emerald-600">✓ 通过 {stats.pass}</span>
            <span className="text-red-500">✕ 失败 {stats.fail}</span>
            <span className="text-slate-400">○ 排队 {stats.queued}</span>
            <span className="text-amber-500">◐ 执行中 {stats.running}</span>
            <span className="text-slate-400">⊘ 阻塞 {stats.blocked}</span>
          </div>
          <span className="text-slate-400">{EXEC_RUN.eta}</span>
        </div>
            </div>

      {/* 环境版本校验：执行前防线 */}
      <div className={'card bg-white rounded-xl border border-slate-200 p-5 mb-5 ' + (client ? 'hidden' : '')}>
        <div className="flex items-center justify-between mb-3 flex-wrap gap-2">
          <div className="flex items-center gap-2">
            <ShieldCheck className="w-4 h-4 text-emerald-500" />
            <span className="text-sm font-semibold text-slate-700">环境版本校验</span>
            <span className="text-[10px] text-slate-400">执行前防线 · 目标版本 vs 环境运行版本，不一致则阻断相关用例，防止「测了也白测」</span>
          </div>
          <button type="button" onClick={recheck}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] bg-emerald-600 text-white hover:bg-emerald-700">
            <RefreshCw className="w-3 h-3" />重新校验 / 刷新环境
          </button>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-2.5">
          {envChecks.map((v) => (
            <div key={v.asset}
              className={'flex items-center justify-between rounded-lg border px-3 py-2 ' + (v.matched ? 'border-emerald-200 bg-emerald-50/50' : 'border-red-200 bg-red-50/50')}>
              <div>
                <div className="text-[11px] font-medium text-slate-600">{v.asset}</div>
                <div className="text-[10px] text-slate-400 mt-0.5">目标 <span className="font-mono">{v.target}</span> · 环境 <span className={'font-mono ' + (v.matched ? 'text-emerald-600' : 'text-red-600')}>{v.runtime}</span></div>
              </div>
              <span className={'flex items-center gap-1 text-[10px] font-medium ' + (v.matched ? 'text-emerald-600' : 'text-red-600')}>
                {v.matched ? <ShieldCheck className="w-3.5 h-3.5" /> : <ShieldAlert className="w-3.5 h-3.5" />}{v.matched ? '匹配' : '不匹配'}
              </span>
            </div>
          ))}
        </div>
        {envChecks.some((v) => !v.matched) && (
          <div className="mt-3 flex items-start gap-2 text-[11px] text-red-600 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
            <ShieldAlert className="w-4 h-4 mt-0.5 shrink-0" />
            <span>svc-payment 环境运行 v2.4.0 与目标 v2.4.1 不一致：相关用例（支付回调幂等、退款金额计算等）将被<b>阻断执行</b>，避免测试结果失真。请刷新环境 / 重新校验后放行。</span>
          </div>
        )}
      </div>

      <div className={'grid grid-cols-3 gap-5 ' + (client ? 'hidden' : '')}>
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
            <h2 className="font-semibold text-slate-700 text-sm">执行任务清单 <span className="text-[10px] text-slate-400 font-normal">· 跨场景总览 · 每任务标注所属测试场景</span></h2>
            <ListFilter search={q} onSearch={setQ}
              selects={[{ key: 'status', label: '状态', options: ['通过', '失败', '执行中', '阻塞'], value: status, onChange: setStatus }]} />
          </div>
          <table className="w-full text-xs">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-slate-500">
                <th className="px-5 py-2.5 font-medium">任务</th>
                <th className="px-5 py-2.5 font-medium">所属场景</th>
                <th className="px-5 py-2.5 font-medium">状态</th>
                <th className="px-5 py-2.5 font-medium text-right">耗时</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {filtered.map((item) => {
                const s = STATUS_STYLE[item.status];
                const scen = TEST_SCENARIOS.find((x) => x.id === flowScenario(item.title, item.asset));
                const Icon = scen ? SCEN_ICON[scen.icon] ?? Cpu : null;
                return (
                  <tr key={item.id} className={item.status === '执行中' ? 'bg-emerald-50/40' : item.status === '失败' ? 'bg-red-50/40' : item.status === '阻塞' ? 'bg-amber-50/40' : ''}>
                    <td className="px-5 py-2.5">
                      <div className="flex items-center gap-2">
                        <span className={s.dot + ' w-2 h-2 rounded-full' + (item.status === '执行中' ? ' animate-pulse' : '')} />
                        <span className="font-mono text-emerald-700 w-24 flex-shrink-0">{item.id}</span>
                        <span className="text-slate-600 truncate">{item.title}</span>
                      </div>
                    </td>
                    <td className="px-5 py-2.5">
                      {scen ? (
                        <span className="inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded-full bg-slate-100 text-slate-500">
                          {Icon && <Icon className="w-3 h-3" />}{scen.name}
                        </span>
                      ) : <span className="text-slate-300">—</span>}
                    </td>
                    <td className="px-5 py-2.5"><span className={'text-[11px] px-2 py-0.5 rounded-full ' + s.badge}>{s.text}</span></td>
                    <td className="px-5 py-2.5 text-slate-400 text-right">{item.duration}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>

        <div className="space-y-5">
          <Card title="执行证据" className="p-5">
            {results.length === 0 ? (
              <p className="text-xs text-slate-500">暂无真实执行证据。创建并执行运行后，这里展示每个用例的哈希、日志和截图引用。</p>
            ) : (
              <div className="space-y-3">
                {results.map((result) => (
                  <div key={result.id} className="rounded-lg border border-slate-200 bg-slate-50 p-3 text-[11px]">
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-mono font-medium text-slate-700">#{result.case_id}</span>
                      <span className={'rounded-full px-2 py-0.5 ' + caseResultStyle(result.status)}>{result.status}</span>
                    </div>
                    <div className="mt-2 flex justify-between gap-3"><span className="text-slate-500">证据哈希</span><span className="truncate font-mono text-slate-600">{(result.evidence as { hash?: string } | undefined)?.hash ?? '—'}</span></div>
                    <div className="mt-1 flex justify-between gap-3"><span className="text-slate-500">日志</span><span className="text-slate-600">{(result.evidence as { logs?: unknown[] } | undefined)?.logs?.length ?? 0} 条</span></div>
                    <div className="mt-1 flex justify-between gap-3"><span className="text-slate-500">截图</span><span className="text-slate-600">{(result.evidence as { screenshots?: unknown[] } | undefined)?.screenshots?.length ?? 0} 张</span></div>
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>
      </div>
    </div>
  );
}

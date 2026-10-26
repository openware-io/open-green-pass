import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { PageHeader, Card } from '@/components/shared';
import { configuredGreenPassClient } from '@/api/runtime';
import { useObjectCatalog } from '@/context/objectCatalog';
import { ShieldCheck, ShieldAlert, RefreshCw, ArrowRight, Database, FlaskConical, Wrench } from 'lucide-react';

type GateData = Awaited<ReturnType<NonNullable<ReturnType<typeof configuredGreenPassClient>>['coverageGate']>>;
type GateMode = 'strict' | 'risk';

const RISK_MODULES = new Set(['auth', 'user', 'security', 'config', 'openPlatform', 'clientRelease', 'execution']);

function pct(value: number): string {
  return `${(value * 100).toFixed(1)}%`;
}

export default function GatePage() {
  const client = useMemo(() => configuredGreenPassClient(), []);
  const navigate = useNavigate();
  const catalog = useObjectCatalog();
  const [projectID, setProjectID] = useState('');
  const [mode, setMode] = useState<GateMode>('risk');
  const [data, setData] = useState<GateData>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (projectID || !catalog.projects.length) return;
    setProjectID(catalog.projectId || String(catalog.projects[0].id));
  }, [catalog.projectId, catalog.projects, projectID]);

  const load = async () => {
    if (!client) return;
    const id = Number(projectID);
    if (!Number.isSafeInteger(id) || id <= 0) { setError('请选择项目'); return; }
    setBusy(true); setError('');
    try { setData(await client.coverageGate(id, mode)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : '读取门禁失败'); }
    finally { setBusy(false); }
  };

  useEffect(() => {
    if (!client || !catalog.loaded || !projectID) return;
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, catalog.loaded, projectID, mode]);

  if (!client) {
    return <div className="p-6 text-sm text-slate-500">配置 GreenPass API 后显示真实质量门禁。</div>;
  }

  const modules = data?.modules ?? [];

  return (
    <div>
      <PageHeader
        title="质量门禁"
        desc="直接读取 API 接口覆盖账本与未关闭缺陷；m3-* 只计入 API 覆盖，不计入 PC Admin UI 覆盖。"
      />

      <Card title="门禁口径" className="mb-5 p-4">
        <div className="flex flex-wrap items-end gap-3">
          <label className="text-xs text-slate-600">项目
            <select value={projectID} onChange={(event) => setProjectID(event.target.value)} disabled={busy} className="mt-1 block min-w-56 rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs">
              <option value="">选择项目</option>
              {catalog.projects.map((project) => <option key={project.id} value={project.id}>{project.name} · #{project.id}</option>)}
            </select>
          </label>
          <div className="text-xs text-slate-600">口径
            <div className="mt-1 inline-flex rounded-lg border border-slate-200 bg-slate-50 p-0.5">
              <button type="button" onClick={() => setMode('risk')} className={'rounded-md px-3 py-1.5 text-[11px] ' + (mode === 'risk' ? 'bg-emerald-600 text-white' : 'text-slate-600')}>风险优先</button>
              <button type="button" onClick={() => setMode('strict')} className={'rounded-md px-3 py-1.5 text-[11px] ' + (mode === 'strict' ? 'bg-emerald-600 text-white' : 'text-slate-600')}>严格</button>
            </div>
          </div>
          <button type="button" onClick={() => void load()} disabled={busy} className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-700 hover:bg-slate-50 disabled:opacity-50"><RefreshCw className="h-3.5 w-3.5" />刷新</button>
          <span className="text-[11px] text-slate-400">{mode === 'risk' ? '18 模块 smoke + 高风险模块 90% + P0/P1=0' : 'design 100% + execution≥90% + P0/P1=0'}</span>
        </div>
      </Card>

      {error && <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">{error}</div>}

      {data && <>
        <div className={'mb-5 flex items-center justify-between rounded-xl border px-5 py-4 ' + (data.gate_passed ? 'border-emerald-200 bg-emerald-50' : 'border-red-200 bg-red-50')}>
          <div className="flex items-center gap-3">
            {data.gate_passed ? <ShieldCheck className="h-7 w-7 text-emerald-600" /> : <ShieldAlert className="h-7 w-7 text-red-600" />}
            <div>
              <div className={'text-lg font-semibold ' + (data.gate_passed ? 'text-emerald-800' : 'text-red-800')}>{data.gate_passed ? '门禁通过' : '门禁阻断'}</div>
              <div className="mt-0.5 text-[11px] text-slate-500">{mode === 'risk' ? '风险优先口径' : '严格口径'} · 项目 #{data.project_id} · {data.blockers.length} 个阻断项</div>
            </div>
          </div>
          <button type="button" onClick={() => navigate('/cases')} className="inline-flex items-center gap-1.5 rounded-lg bg-white px-3 py-2 text-xs text-slate-700 shadow-sm">查看用例库<ArrowRight className="h-3.5 w-3.5" /></button>
        </div>

        <div className="mb-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {[
            { label: '必需 Cell', value: data.total_required, icon: Database },
            { label: 'API 设计覆盖', value: `${data.design_covered} · ${pct(data.design_coverage)}`, icon: FlaskConical },
            { label: 'API 执行覆盖', value: `${data.executed} · ${pct(data.execution_coverage)}`, icon: Wrench },
            { label: '未关闭缺陷', value: `P0 ${data.open_p0} · P1 ${data.open_p1}`, icon: ShieldAlert },
          ].map((item) => <div key={item.label} className="rounded-xl border border-slate-200 bg-white p-4"><div className="flex items-center gap-2 text-[11px] text-slate-500"><item.icon className="h-3.5 w-3.5" />{item.label}</div><div className="mt-2 text-lg font-semibold text-slate-800">{item.value}</div></div>)}
        </div>

        {data.blockers.length > 0 && <Card title="阻断原因" className="mb-5 p-5"><ul className="space-y-1.5 text-xs text-slate-600">{data.blockers.map((blocker) => <li key={blocker} className="flex gap-2"><span className="text-red-500">•</span>{blocker}</li>)}</ul></Card>}

        <Card title="模块覆盖" className="p-0" extra={<span className="text-[11px] text-slate-400">来自 cov_cell 账本</span>}>
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead className="bg-slate-50 text-left text-slate-500"><tr><th className="px-4 py-2.5">模块</th><th className="px-4 py-2.5">必需</th><th className="px-4 py-2.5">已映射</th><th className="px-4 py-2.5">已通过</th><th className="px-4 py-2.5">API 设计覆盖</th><th className="px-4 py-2.5">API 执行覆盖</th><th className="px-4 py-2.5">风险口径</th></tr></thead>
              <tbody className="divide-y divide-slate-100">{modules.map((module) => <tr key={module.module}><td className="px-4 py-2 font-mono text-slate-700">{module.module}</td><td className="px-4 py-2">{module.required}</td><td className="px-4 py-2">{module.mapped}</td><td className="px-4 py-2 text-emerald-700">{module.passed}</td><td className="px-4 py-2">{pct(module.design_coverage)}</td><td className="px-4 py-2">{pct(module.execution_coverage)}</td><td className="px-4 py-2 text-[11px]">{RISK_MODULES.has(module.module) ? '高风险模块 · ≥90%' : '模块 smoke'}</td></tr>)}</tbody>
            </table>
          </div>
          {modules.length === 0 && <div className="p-6 text-center text-xs text-slate-400">该项目还没有覆盖账本数据。</div>}
        </Card>
      </>}
    </div>
  );
}

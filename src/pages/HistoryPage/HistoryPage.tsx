import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { PageHeader, Card } from '@/components/shared';
import { configuredGreenPassClient } from '@/api/runtime';
import { useObjectCatalog } from '@/context/objectCatalog';
import type { CaseResult, GateResult, Run } from '@/api/client';
import { Activity, ArrowRight, FileText, RefreshCw } from 'lucide-react';

const STATE_STYLE: Record<string, string> = {
  done: 'bg-emerald-50 text-emerald-700',
  failed: 'bg-red-50 text-red-700',
  running: 'bg-sky-50 text-sky-700',
  paused: 'bg-amber-50 text-amber-700',
};

const RESULT_STYLE: Record<string, string> = {
  pass: 'bg-emerald-50 text-emerald-700',
  fail: 'bg-red-50 text-red-700',
  blocked: 'bg-amber-50 text-amber-700',
  skipped: 'bg-slate-100 text-slate-500',
  retried: 'bg-violet-50 text-violet-700',
};

function duration(run: Run): string {
  if (!run.started_at || !run.ended_at) return '—';
  const ms = new Date(run.ended_at).getTime() - new Date(run.started_at).getTime();
  if (!Number.isFinite(ms) || ms < 0) return '—';
  return `${Math.round(ms / 1000)}s`;
}

export default function HistoryPage() {
  const client = useMemo(() => configuredGreenPassClient(), []);
  const navigate = useNavigate();
  const catalog = useObjectCatalog();
  const [projectID, setProjectID] = useState('');
  const [runs, setRuns] = useState<Run[]>([]);
  const [selectedID, setSelectedID] = useState<number>();
  const [results, setResults] = useState<CaseResult[]>([]);
  const [gates, setGates] = useState<GateResult[]>([]);
  const [loading, setLoading] = useState(Boolean(client));
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (projectID || !catalog.projects.length) return;
    setProjectID(catalog.projectId || String(catalog.projects[0].id));
  }, [catalog.projectId, catalog.projects, projectID]);

  const loadRuns = async () => {
    if (!client) return;
    const targetID = Number(projectID);
    if (!Number.isSafeInteger(targetID) || targetID <= 0) return;
    setLoading(true); setError('');
    try {
      const loaded = await client.listRuns({ target_id: targetID, limit: 50 });
      setRuns(loaded);
      setSelectedID((current) => current && loaded.some((run) => run.id === current) ? current : loaded[0]?.id);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '读取测试历史失败');
    } finally { setLoading(false); }
  };

  useEffect(() => { void loadRuns(); /* eslint-disable-next-line react-hooks/exhaustive-deps */ }, [client, projectID]);

  useEffect(() => {
    if (!client || !selectedID) { setResults([]); setGates([]); return; }
    let active = true;
    setDetailLoading(true);
    Promise.all([client.caseResults(selectedID), client.gateResults(selectedID)])
      .then(([nextResults, nextGates]) => { if (active) { setResults(nextResults); setGates(nextGates); } })
      .catch((cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : '读取运行明细失败'); })
      .finally(() => { if (active) setDetailLoading(false); });
    return () => { active = false; };
  }, [client, selectedID]);

  if (!client) {
    return <PageHeader title="测试历史" desc="配置 GreenPass API 后显示真实运行历史。" />;
  }

  const selected = runs.find((run) => run.id === selectedID);
  const pass = results.filter((result) => result.status === 'pass').length;
  const fail = results.filter((result) => result.status === 'fail').length;
  const blocked = results.filter((result) => result.status === 'blocked').length;
  const gatePassed = gates.some((gate) => gate.result === 'pass');

  return (
    <div>
      <PageHeader title="测试历史" desc="直接读取运行动账本、用例结果和门禁结论；不展示原型运行数据。">
'        <label className="text-[11px] text-slate-500">项目
          <select value={projectID} onChange={(event) => setProjectID(event.target.value)} disabled={loading} className="ml-1 rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-xs text-slate-700">
            <option value="">选择项目</option>{catalog.projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
          </select>
        </label>
'        <button type="button" onClick={() => void loadRuns()} disabled={loading} className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs text-slate-700"><RefreshCw className="h-3.5 w-3.5" />{loading ? '加载中…' : '刷新'}</button>
      </PageHeader>

      {error && <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">{error}</div>}

      <Card title="真实运行记录" className="mb-5 p-0" extra={<span className="text-[11px] text-slate-400">{runs.length} 条</span>}>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead className="bg-slate-50 text-left text-slate-500"><tr><th className="px-3 py-2.5">运行</th><th className="px-3 py-2.5">对象 / 场景</th><th className="px-3 py-2.5">分支 / 版本</th><th className="px-3 py-2.5">环境</th><th className="px-3 py-2.5">状态</th><th className="px-3 py-2.5">用例</th><th className="px-3 py-2.5">耗时</th></tr></thead>
            <tbody className="divide-y divide-slate-100">{runs.map((run) => <tr key={run.id} onClick={() => setSelectedID(run.id)} className={'cursor-pointer ' + (selectedID === run.id ? 'bg-emerald-50/50' : 'hover:bg-slate-50')}><td className="px-3 py-2.5 font-mono text-emerald-700">#{run.id}</td><td className="px-3 py-2.5">#{run.target_id} / 场景 #{run.scenario_id}</td><td className="px-3 py-2.5 font-mono">{run.target_branch || '—'} / {run.target_version}</td><td className="px-3 py-2.5">{run.env} · {run.env_version || '—'}</td><td className="px-3 py-2.5"><span className={'rounded-full px-2 py-0.5 text-[10px] ' + (STATE_STYLE[run.state] ?? 'bg-slate-100 text-slate-500')}>{run.state}</span></td><td className="px-3 py-2.5">{run.selected_cases.length}</td><td className="px-3 py-2.5">{duration(run)}</td></tr>)}</tbody>
          </table>
        </div>
        {!loading && runs.length === 0 && <div className="p-8 text-center text-xs text-slate-400">还没有真实运行记录。</div>}
      </Card>

      {selected && <Card title={`运行 #${selected.id} 明细`} className="p-0" extra={<button type="button" onClick={() => navigate('/reports')} className="inline-flex items-center gap-1 text-[11px] text-emerald-700">测试报告<ArrowRight className="h-3.5 w-3.5" /></button>}>
        <div className="grid grid-cols-2 gap-3 border-b border-slate-200 bg-slate-50 p-4 text-xs md:grid-cols-5">
          <div><div className="text-slate-400">通过</div><div className="mt-1 text-lg font-semibold text-emerald-600">{pass}</div></div>
          <div><div className="text-slate-400">失败</div><div className="mt-1 text-lg font-semibold text-red-600">{fail}</div></div>
          <div><div className="text-slate-400">阻塞</div><div className="mt-1 text-lg font-semibold text-amber-600">{blocked}</div></div>
          <div><div className="text-slate-400">门禁</div><div className="mt-1 font-semibold"><span className={gatePassed ? 'text-emerald-700' : 'text-red-700'}>{gates.length ? (gatePassed ? '通过' : '阻断') : '未评估'}</span></div></div>
          <div><div className="text-slate-400">结束时间</div><div className="mt-1 text-slate-600">{selected.ended_at ? new Date(selected.ended_at).toLocaleString('zh-CN', { hour12: false }) : '—'}</div></div>
        </div>
        <div className="flex items-center gap-2 px-4 py-2 text-[11px] text-slate-500"><Activity className="h-3.5 w-3.5" />{detailLoading ? '读取用例结果…' : `${results.length} 条用例结果`}</div>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead className="bg-slate-50 text-left text-slate-500"><tr><th className="px-3 py-2">用例</th><th className="px-3 py-2">版本</th><th className="px-3 py-2">结果</th><th className="px-3 py-2">尝试</th><th className="px-3 py-2">结果说明</th><th className="px-3 py-2">耗时</th></tr></thead>
            <tbody className="divide-y divide-slate-100">{results.map((result) => <tr key={result.id}><td className="px-3 py-2 font-mono">#{result.case_id}</td><td className="px-3 py-2">v{result.case_version}</td><td className="px-3 py-2"><span className={'rounded-full px-2 py-0.5 text-[10px] ' + (RESULT_STYLE[result.status] ?? 'bg-slate-100 text-slate-500')}>{result.status}</span></td><td className="px-3 py-2">{result.attempt_seq}</td><td className="max-w-[420px] truncate px-3 py-2 text-slate-600">{result.result_text || '—'}</td><td className="px-3 py-2">{result.started_at && result.ended_at ? `${Math.max(0, Math.round((new Date(result.ended_at).getTime() - new Date(result.started_at).getTime()) / 1000))}s` : '—'}</td></tr>)}</tbody>
          </table>
        </div>
        {!detailLoading && results.length === 0 && <div className="p-6 text-center text-xs text-slate-400">该运行还没有用例结果。</div>}
      </Card>}

      {!selected && !loading && <div className="rounded-xl border border-dashed border-slate-200 bg-white p-8 text-center text-xs text-slate-400"><FileText className="mx-auto mb-2 h-5 w-5" />选择一条运行查看明细。</div>}
    </div>
  );
}

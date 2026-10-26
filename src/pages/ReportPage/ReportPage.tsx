import { useEffect, useMemo, useState } from 'react';
import { PageHeader, Card, PrimaryButton } from '@/components/shared';
import { configuredGreenPassClient } from '@/api/runtime';
import { useObjectCatalog } from '@/context/objectCatalog';
import type { Report, Run } from '@/api/client';
import { Download, ExternalLink, FileText, RefreshCw } from 'lucide-react';

export default function ReportPage() {
  const client = useMemo(() => configuredGreenPassClient(), []);
  const catalog = useObjectCatalog();
  const [projectID, setProjectID] = useState('');
  const [runs, setRuns] = useState<Run[]>([]);
  const [runID, setRunID] = useState('');
  const [report, setReport] = useState<Report>();
  const [html, setHTML] = useState('');
  const [loading, setLoading] = useState(Boolean(client));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (projectID || !catalog.projects.length) return;
    setProjectID(catalog.projectId || String(catalog.projects[0].id));
  }, [catalog.projectId, catalog.projects, projectID]);

  useEffect(() => {
    if (!client) return;
    const targetID = Number(projectID);
    if (!Number.isSafeInteger(targetID) || targetID <= 0) { setRuns([]); setLoading(false); return; }
    let active = true;
    setLoading(true);
    client.listRuns({ target_id: targetID, limit: 50 })
      .then((loaded) => {
        if (!active) return;
        setRuns(loaded);
        setRunID((current) => current || String(loaded[0]?.id ?? ''));
      })
      .catch((cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : '读取运行失败'); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [client, projectID]);

  const generate = async () => {
    if (!client) return;
    const id = Number(runID);
    if (!Number.isSafeInteger(id) || id <= 0) { setError('请选择运行'); return; }
    setBusy(true); setError('');
    try {
      const next = await client.generateReport(id);
      setReport(next);
      setHTML(next.html || await client.reportHTML(next.id));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '生成报告失败');
    } finally { setBusy(false); }
  };

  const refreshHTML = async () => {
    if (!client || !report) return;
    setBusy(true); setError('');
    try { setHTML(await client.reportHTML(report.id)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : '读取 HTML 失败'); }
    finally { setBusy(false); }
  };

  const openPreview = () => {
    if (!html) return;
    const win = window.open('', '_blank', 'noopener,noreferrer');
    if (!win) { setError('浏览器阻止了预览窗口'); return; }
    win.document.write(html);
    win.document.close();
  };

  const download = () => {
    if (!html) return;
    const blob = new Blob([html], { type: 'text/html;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `greenpass-report-${report?.id ?? runID}.html`;
    link.click();
    URL.revokeObjectURL(url);
  };

  if (!client) return <PageHeader title="测试报告" desc="配置 GreenPass API 后生成真实报告。" />;

  return (
    <div>
      <PageHeader title="测试报告" desc="选择真实运行，由服务端生成报告；不使用原型报告数据。" />

      {error && <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">{error}</div>}

      <Card title="报告生成" className="mb-5 p-5">
        <div className="flex flex-wrap items-end gap-3">
          <label className="text-xs text-slate-600">项目
            <select value={projectID} onChange={(event) => { setProjectID(event.target.value); setReport(undefined); setHTML(''); }} disabled={loading || busy} className="mt-1 block min-w-44 rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs">
              <option value="">选择项目</option>{catalog.projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
            </select>
          </label>
          <label className="text-xs text-slate-600">真实运行
            <select value={runID} onChange={(event) => { setRunID(event.target.value); setReport(undefined); setHTML(''); }} disabled={loading || busy} className="mt-1 block min-w-72 rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs">
              <option value="">{loading ? '读取运行中…' : '选择运行'}</option>
              {runs.map((run) => <option key={run.id} value={run.id}>#{run.id} · 场景 #{run.scenario_id} · {run.state} · {run.target_version}</option>)}
            </select>
          </label>
          <PrimaryButton disabled={busy || !runID} onClick={() => void generate()}><span className="flex items-center gap-1.5"><FileText className="h-4 w-4" />{busy ? '生成中…' : '生成 HTML 报告'}</span></PrimaryButton>
          <button type="button" onClick={() => void refreshHTML()} disabled={!report || busy} className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-700 disabled:opacity-50"><RefreshCw className="h-3.5 w-3.5" />刷新 HTML</button>
          <button type="button" onClick={openPreview} disabled={!html} className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-700 disabled:opacity-50"><ExternalLink className="h-3.5 w-3.5" />新窗口预览</button>
          <button type="button" onClick={download} disabled={!html} className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-700 disabled:opacity-50"><Download className="h-3.5 w-3.5" />下载 HTML</button>
        </div>
        {runs.length === 0 && !loading && <p className="mt-3 text-xs text-slate-400">还没有真实运行，先去测试执行创建运行。</p>}
        {report && <div className="mt-4 flex flex-wrap items-center gap-3 rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 text-xs text-slate-600">
          <span className="font-mono text-emerald-700">REPORT #{report.id}</span>
          <span>{report.kind}</span>
          <span className={report.status === 'pass' ? 'text-emerald-700' : 'text-red-700'}>{report.status}</span>
          <span className="text-slate-400">运行 #{report.run_id} · 目标 #{report.target_id}</span>
          <span className="text-slate-400">{report.created_at ? new Date(report.created_at).toLocaleString('zh-CN', { hour12: false }) : ''}</span>
        </div>}
      </Card>

      <Card title="报告预览" className="p-0" extra={html ? <span className="text-[11px] text-slate-400">HTML · {html.length} bytes</span> : undefined}>
        {html ? <iframe title="GreenPass 测试报告" srcDoc={html} className="h-[720px] w-full rounded-b-xl border-0 bg-white" sandbox="" /> : <div className="p-10 text-center text-xs text-slate-400">选择运行并生成报告后显示真实 HTML。</div>}
      </Card>
    </div>
  );
}

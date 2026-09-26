import { useState } from 'react';
import { TEST_RUNS, RUN_SERVICE_REPORTS, EXEC_AUDIT_ROWS, CASE_COST_HISTORY, TEST_EVIDENCE, type ITestRun } from '@/data/mock';
import { PageHeader, Card } from '@/components/shared';
import { FileText, Download, CheckCircle2, Image, Video, File, FileJson } from 'lucide-react';

const RISK_BADGE: Record<string, string> = { '低': 'bg-emerald-50 text-emerald-600', '中': 'bg-amber-50 text-amber-600', '高': 'bg-red-50 text-red-600' };
const RESULT_BADGE: Record<string, string> = { '通过': 'bg-emerald-50 text-emerald-600', '失败': 'bg-red-50 text-red-600', '阻塞': 'bg-amber-50 text-amber-600' };
const EV_TYPE_ICON: Record<string, typeof Image> = { '截图': Image, '视频': Video, '日志': File, '请求响应': FileJson };
const EXPORT_FORMATS = [
  { k: 'html', label: 'HTML', ext: '.html' },
  { k: 'pdf', label: 'PDF', ext: '.pdf' },
  { k: 'docx', label: 'Word', ext: '.docx' },
  { k: 'md', label: 'Markdown', ext: '.md' },
];

function ExportMenu({ runId, onDone }: { runId: string; onDone: (f: string) => void }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="relative flex-shrink-0">
      <button type="button" onClick={() => setOpen((o) => !o)}
        className="px-3 py-1.5 text-xs bg-indigo-600 text-white rounded-lg flex items-center gap-1"><Download className="w-3.5 h-3.5" />导出报告</button>
      {open && (
        <div className="absolute right-0 top-9 z-20 w-44 rounded-lg border border-slate-200 bg-white shadow-lg overflow-hidden">
          {EXPORT_FORMATS.map((f) => (
            <button key={f.k} type="button"
              onClick={() => { onDone(`${runId}${f.ext}`); setOpen(false); }}
              className="w-full text-left px-3 py-2 text-xs text-slate-600 hover:bg-slate-50 flex items-center gap-2">
              <FileText className="w-3.5 h-3.5 text-slate-400" />{f.label}
              <span className="ml-auto text-[10px] text-slate-400">{f.ext}</span>
            </button>
          ))}
          <div className="px-3 py-1.5 text-[9px] text-slate-400 border-t border-slate-100">原型：模拟下载 · 接入后按模板生成</div>
        </div>
      )}
    </div>
  );
}

function ReportView({ run, onExport }: { run: ITestRun; onExport: (f: string) => void }) {
  const [service, setService] = useState<string | null>(null);
  const [caseId, setCaseId] = useState<string | null>(null);
  const runExport = (f: string) => onExport(f);
  if (caseId) return <CaseReport run={run} caseId={caseId} onExport={(f) => runExport(f)} onBack={() => setCaseId(null)} />;
  if (service) return <ServiceReport run={run} service={service} onExport={(f) => runExport(f)} onBack={() => setService(null)} onOpenCase={setCaseId} />;
  return <ProjectReport run={run} onExport={(f) => runExport(f)} onOpenService={setService} />;
}

function ProjectReport({ run, onExport, onOpenService }: { run: ITestRun; onExport: (f: string) => void; onOpenService: (s: string) => void }) {
  const passRate = Math.round((run.pass / run.total) * 1000) / 10;
  const highRisk = RUN_SERVICE_REPORTS.filter((s) => s.risk === '高');
  return (
    <div className="card bg-white rounded-xl border border-indigo-200 p-6 mt-5">
      <div className="flex items-start justify-between mb-5">
        <div>
          <div className="flex items-center gap-1.5 text-[10px] text-indigo-500 font-mono mb-1"><FileText className="w-3.5 h-3.5" />测试报告 · REP-{run.id.replace('RUN-', '')}</div>
          <h2 className="text-lg font-bold text-slate-800">工程测试报告 · {run.id}</h2>
          <p className="text-[11px] text-slate-500 mt-1">{run.ts} · 分支 {run.branch} · 触发 {run.trigger} · 工程 sys-payment-platform</p>
        </div>
        <ExportMenu runId={run.id} onDone={onExport} />
      </div>

      <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-5">
        <div className="p-3 rounded-lg bg-emerald-50 border border-emerald-200"><div className="text-[11px] text-emerald-600">通过率</div><div className="text-xl font-bold text-emerald-700">{passRate}%</div><div className="text-[10px] text-emerald-500">{run.pass}/{run.total}</div></div>
        <div className="p-3 rounded-lg bg-red-50 border border-red-200"><div className="text-[11px] text-red-600">失败 / 阻塞</div><div className="text-xl font-bold text-red-600">{run.fail}<span className="text-sm font-normal text-red-400"> / {run.block}</span></div><div className="text-[10px] text-red-400">阻断 10</div></div>
        <div className="p-3 rounded-lg bg-slate-50 border border-slate-200"><div className="text-[11px] text-slate-500">执行成本</div><div className="text-xl font-bold text-slate-700">¥{run.cost.toLocaleString()}</div><div className="text-[10px] text-slate-400">关联用例成本明细</div></div>
        <div className="p-3 rounded-lg bg-slate-50 border border-slate-200"><div className="text-[11px] text-slate-500">执行时长</div><div className="text-xl font-bold text-slate-700">{run.duration}</div><div className="text-[10px] text-slate-400">12 执行器并行</div></div>
        <div className="p-3 rounded-lg border bg-white border-slate-200"><div className="text-[11px] text-slate-500">门禁结论</div><div className={'text-xl font-bold ' + (run.gate === '通过' ? 'text-emerald-600' : 'text-red-600')}>{run.gate}</div><div className="text-[10px] text-slate-400">4 项规则判定</div></div>
      </div>

      <h3 className="font-semibold text-slate-700 text-sm mb-3">服务维度报告 <span className="text-[10px] text-slate-400 font-normal">（点击服务行下钻查看该服务报告）</span></h3>
      <div className="rounded-xl border border-slate-200 overflow-hidden mb-5">
        <table className="w-full text-xs">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-slate-500">
              <th className="px-4 py-2.5 font-medium">服务</th>
              <th className="px-4 py-2.5 font-medium">通过</th>
              <th className="px-4 py-2.5 font-medium">失败</th>
              <th className="px-4 py-2.5 font-medium">阻塞</th>
              <th className="px-4 py-2.5 font-medium">覆盖率</th>
              <th className="px-4 py-2.5 font-medium">成本</th>
              <th className="px-4 py-2.5 font-medium">风险</th>
              <th className="px-4 py-2.5 font-medium">结论</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {RUN_SERVICE_REPORTS.map((s) => (
              <tr key={s.service} onClick={() => onOpenService(s.service)} className="cursor-pointer hover:bg-indigo-50/40">
                <td className="px-4 py-2.5"><span className="font-mono text-indigo-600 font-medium">{s.service}</span><span className="text-[10px] text-slate-400 ml-2">{s.name}</span></td>
                <td className="px-4 py-2.5 text-emerald-600">{s.pass}</td>
                <td className="px-4 py-2.5 text-red-600">{s.fail}</td>
                <td className="px-4 py-2.5 text-amber-600">{s.block}</td>
                <td className="px-4 py-2.5 text-slate-600">{s.coverage}%</td>
                <td className="px-4 py-2.5 font-mono text-slate-600">¥{s.cost.toLocaleString()}</td>
                <td className="px-4 py-2.5"><span className={`px-2 py-0.5 rounded-full text-[10px] ${RISK_BADGE[s.risk]}`}>{s.risk}</span></td>
                <td className="px-4 py-2.5 text-slate-500 max-w-[180px]">{s.conclusion}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="grid grid-cols-3 gap-4 mb-5">
        <div className="p-4 rounded-xl bg-slate-50 border border-slate-200">
          <div className="text-[11px] text-slate-500 mb-1">成本汇总</div>
          <div className="text-lg font-bold text-slate-800">¥{run.cost.toLocaleString()}</div>
          <div className="text-[10px] text-slate-400 mt-1">总 Token {Math.round(run.cost / 0.04).toLocaleString()}K · 存量执行成本持平/递减</div>
        </div>
        <div className="p-4 rounded-xl bg-red-50 border border-red-200">
          <div className="text-[11px] text-red-600 mb-1">高风险服务</div>
          <div className="text-lg font-bold text-red-600">{highRisk.length} 个</div>
          <div className="text-[10px] text-red-500 mt-1">{highRisk.map((s) => s.service).join('、') || '—'}</div>
        </div>
        <div className="p-4 rounded-xl bg-emerald-50 border border-emerald-200">
          <div className="text-[11px] text-emerald-600 mb-1">治理结论</div>
          <div className={'text-lg font-bold ' + (run.gate === '通过' ? 'text-emerald-700' : 'text-red-600')}>{run.gate === '通过' ? '放行' : '阻断'}</div>
          <div className="text-[10px] text-emerald-500 mt-1">门禁判定 · 需解决 {run.fail} 项失败</div>
        </div>
      </div>

      <div className="rounded-xl border border-slate-200 p-4 mb-5">
        <h3 className="font-semibold text-slate-700 text-sm mb-2">结论与建议</h3>
        <ul className="space-y-1.5 text-[11px] text-slate-600 list-disc pl-4">
          <li>整体通过率 {passRate}%，门禁结论「{run.gate}」；svc-payment 存在 {highRisk.length} 项高风险（契约变更 + 断言弱化 + 变异分数不足），需修复后复测。</li>
          <li>契约门禁 CT-003 /v2/refund 破坏性变更阻断，2 个消费者契约测试待更新。</li>
          <li>存量用例回放成本持平/递减，符合治理降本逻辑；本次失败触发「新增分析」成本回升属预期。</li>
        </ul>
      </div>

      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-[10px] text-slate-400 border-t border-slate-100 pt-3">
        <span>报告编号 <span className="font-mono text-slate-600">REP-{run.id.replace('RUN-', '')}</span></span>
        <span>报告哈希 <span className="font-mono text-emerald-600">sha256:0a1f…d84b</span></span>
        <span>审计锚定 <span className="text-slate-600">2026-09-24 10:30:00</span></span>
        <span>可独立验证 <span className="text-emerald-600">✓ 哈希链完整</span></span>
      </div>
    </div>
  );
}

function ServiceReport({ run, service, onExport, onBack, onOpenCase }: {
  run: ITestRun; service: string; onExport: (f: string) => void;
  onBack: () => void; onOpenCase: (c: string) => void;
}) {
  const s = RUN_SERVICE_REPORTS.find((r) => r.service === service);
  const rows = EXEC_AUDIT_ROWS.filter((r) => r.asset === service);
  if (!s) return null;
  return (
    <div className="card bg-white rounded-xl border border-emerald-200 p-6 mt-5">
      <div className="flex items-start justify-between mb-5">
        <div>
          <button type="button" onClick={onBack} className="text-[10px] text-slate-400 hover:text-indigo-600 mb-1">← 返回工程报告</button>
          <div className="flex items-center gap-1.5 text-[10px] text-emerald-600 font-mono mb-1"><FileText className="w-3.5 h-3.5" />服务测试报告</div>
          <h2 className="text-lg font-bold text-slate-800">服务报告 · <span className="font-mono text-indigo-600">{s.service}</span> <span className="text-slate-400 font-normal text-sm">{s.name}</span></h2>
          <p className="text-[11px] text-slate-500 mt-1">所属运行 {run.id} · {run.ts} · 工程 sys-payment-platform</p>
        </div>
        <ExportMenu runId={`${run.id}-${service}`} onDone={onExport} />
      </div>

      <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-5">
        <div className="p-3 rounded-lg bg-emerald-50 border border-emerald-200"><div className="text-[11px] text-emerald-600">通过率</div><div className="text-xl font-bold text-emerald-700">{Math.round((s.pass / s.total) * 100)}%</div><div className="text-[10px] text-emerald-500">{s.pass}/{s.total}</div></div>
        <div className="p-3 rounded-lg bg-red-50 border border-red-200"><div className="text-[11px] text-red-600">失败 / 阻塞</div><div className="text-xl font-bold text-red-600">{s.fail}<span className="text-sm font-normal text-red-400"> / {s.block}</span></div><div className="text-[10px] text-red-400">需修复项</div></div>
        <div className="p-3 rounded-lg bg-slate-50 border border-slate-200"><div className="text-[11px] text-slate-500">覆盖率</div><div className="text-xl font-bold text-slate-700">{s.coverage}%</div><div className="text-[10px] text-slate-400">需求追溯覆盖</div></div>
        <div className="p-3 rounded-lg bg-slate-50 border border-slate-200"><div className="text-[11px] text-slate-500">执行成本</div><div className="text-xl font-bold text-slate-700">¥{s.cost.toLocaleString()}</div><div className="text-[10px] text-slate-400">关联用例成本</div></div>
        <div className="p-3 rounded-lg border bg-white border-slate-200"><div className="text-[11px] text-slate-500">风险等级</div><div className="text-xl font-bold"><span className={'px-2 py-0.5 rounded-full text-xs ' + RISK_BADGE[s.risk]}>{s.risk}</span></div><div className="text-[10px] text-slate-400 mt-1">{s.risks.length} 个风险点</div></div>
      </div>

      <h3 className="font-semibold text-slate-700 text-sm mb-3">用例执行明细 <span className="text-[10px] text-slate-400 font-normal">（点击用例行下钻查看该用例报告）</span></h3>
      <div className="rounded-xl border border-slate-200 overflow-hidden mb-5">
        <table className="w-full text-xs">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-slate-500">
              <th className="px-4 py-2.5 font-medium">用例 ID</th>
              <th className="px-4 py-2.5 font-medium">结果</th>
              <th className="px-4 py-2.5 font-medium">耗时</th>
              <th className="px-4 py-2.5 font-medium">模型</th>
              <th className="px-4 py-2.5 font-medium">成本</th>
              <th className="px-4 py-2.5 font-medium">时间</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {rows.map((r) => {
              const his = CASE_COST_HISTORY[r.caseId];
              const cost = his ? his[his.length - 1].cost : '—';
              return (
                <tr key={r.caseId + r.ts} onClick={() => onOpenCase(r.caseId)} className="cursor-pointer hover:bg-emerald-50/40">
                  <td className="px-4 py-2.5 font-mono text-indigo-600 font-medium">{r.caseId}</td>
                  <td className="px-4 py-2.5"><span className={`px-2 py-0.5 rounded-full text-[10px] ${RESULT_BADGE[r.result]}`}>{r.result}</span></td>
                  <td className="px-4 py-2.5 font-mono text-slate-500">{r.duration}</td>
                  <td className="px-4 py-2.5 text-slate-500">{r.model}</td>
                  <td className="px-4 py-2.5 font-mono text-slate-600">¥{typeof cost === 'number' ? cost.toFixed(1) : cost}</td>
                  <td className="px-4 py-2.5 text-slate-400">{r.ts}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {s.risks.length > 0 && (
        <div className="rounded-xl border border-red-200 bg-red-50/40 p-4 mb-5">
          <h3 className="font-semibold text-red-700 text-sm mb-2">风险点</h3>
          <div className="flex flex-wrap gap-1.5">{s.risks.map((r) => <span key={r} className="text-[10px] bg-red-100 text-red-700 px-2 py-0.5 rounded">{r}</span>)}</div>
        </div>
      )}
      <div className="rounded-xl border border-slate-200 p-4 mb-5">
        <h3 className="font-semibold text-slate-700 text-sm mb-1">结论</h3>
        <p className="text-[11px] text-slate-600">{s.conclusion}</p>
      </div>
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-[10px] text-slate-400 border-t border-slate-100 pt-3">
        <span>报告哈希 <span className="font-mono text-emerald-600">sha256:9c3e…a1f7</span></span>
        <span>审计锚定 <span className="text-slate-600">2026-09-24 10:30:00</span></span>
        <span>可独立验证 <span className="text-emerald-600">✓ 哈希链完整</span></span>
      </div>
    </div>
  );
}

function CaseReport({ run, caseId, onExport, onBack }: {
  run: ITestRun; caseId: string; onExport: (f: string) => void; onBack: () => void;
}) {
  const row = EXEC_AUDIT_ROWS.find((r) => r.caseId === caseId);
  const his = CASE_COST_HISTORY[caseId] ?? [];
  const last = his[his.length - 1];
  const evid = TEST_EVIDENCE[caseId] ?? [];
  if (!row) return null;
  const Icon = EV_TYPE_ICON[evid[0]?.type ?? '截图'] ?? Image;
  return (
    <div className="card bg-white rounded-xl border border-indigo-200 p-6 mt-5">
      <div className="flex items-start justify-between mb-5">
        <div>
          <button type="button" onClick={onBack} className="text-[10px] text-slate-400 hover:text-indigo-600 mb-1">← 返回 {row.asset} 服务报告</button>
          <div className="flex items-center gap-1.5 text-[10px] text-indigo-600 font-mono mb-1"><FileText className="w-3.5 h-3.5" />用例测试报告</div>
          <h2 className="text-lg font-bold text-slate-800">用例报告 · <span className="font-mono text-indigo-600">{caseId}</span></h2>
          <p className="text-[11px] text-slate-500 mt-1">{row.asset} · 运行 {run.id} · {run.ts} · {row.model}</p>
        </div>
        <ExportMenu runId={`${run.id}-${caseId}`} onDone={onExport} />
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-5">
        <div className="p-3 rounded-lg bg-slate-50 border border-slate-200"><div className="text-[11px] text-slate-500">执行结果</div><div className="mt-1"><span className={`px-2 py-0.5 rounded-full text-xs ${RESULT_BADGE[row.result]}`}>{row.result}</span></div><div className="text-[10px] text-slate-400 mt-1">本次运行</div></div>
        <div className="p-3 rounded-lg bg-slate-50 border border-slate-200"><div className="text-[11px] text-slate-500">耗时</div><div className="text-xl font-bold text-slate-700">{row.duration}</div><div className="text-[10px] text-slate-400">模型 {row.model}</div></div>
        <div className="p-3 rounded-lg bg-slate-50 border border-slate-200"><div className="text-[11px] text-slate-500">本次成本</div><div className="text-xl font-bold text-slate-700">¥{last?.cost ?? '—'}</div><div className="text-[10px] text-slate-400">{last?.run} · {last?.kind}</div></div>
        <div className="p-3 rounded-lg bg-slate-50 border border-slate-200"><div className="text-[11px] text-slate-500">证据数</div><div className="text-xl font-bold text-slate-700">{evid.length}</div><div className="text-[10px] text-slate-400">哈希锚定</div></div>
      </div>

      <div className="rounded-xl border border-slate-200 p-4 mb-5">
        <h3 className="font-semibold text-slate-700 text-sm mb-2">成本历史</h3>
        <div className="flex flex-wrap gap-2 text-[11px]">
          {his.map((p, i) => (
            <span key={i} className={'px-2 py-1 rounded-lg border ' + (i === his.length - 1 ? 'bg-indigo-50 border-indigo-200 text-indigo-700' : 'bg-white border-slate-200 text-slate-500')}>
              {p.run} <span className="font-mono">¥{p.cost}</span> · {p.result}
            </span>
          ))}
        </div>
        <p className="mt-2 text-[10px] text-slate-400">同一用例成本与历史记录对比 · 存量回放成本持平/递减，失败触发新增分析回升 —— 符合治理降本逻辑。</p>
      </div>

      <div className="rounded-xl border border-slate-200 p-4 mb-5">
        <h3 className="font-semibold text-slate-700 text-sm mb-3">测试证据</h3>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
          {evid.length === 0 ? <span className="text-[11px] text-slate-400">本用例无证据记录</span> : evid.map((ev, i) => (
            <div key={i} className="rounded-lg border border-slate-200 p-2.5">
              <div className="aspect-video bg-slate-100 rounded flex items-center justify-center text-slate-400 border border-slate-200 mb-1.5"><Icon className="w-6 h-6" /></div>
              <div className="flex items-center justify-between text-[10px]"><span className="font-medium text-slate-700">{ev.type}</span><span className="font-mono text-emerald-600">{ev.hash}</span></div>
              <div className="text-[10px] text-slate-500 mt-0.5">{ev.note}</div>
            </div>
          ))}
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-[10px] text-slate-400 border-t border-slate-100 pt-3">
        <span>报告哈希 <span className="font-mono text-emerald-600">sha256:b1a7…7c2e</span></span>
        <span>审计锚定 <span className="text-slate-600">2026-09-24 10:25:12</span></span>
        <span>可独立验证 <span className="text-emerald-600">✓ 哈希链完整</span></span>
      </div>
    </div>
  );
}

export default function ReportPage() {
  const [runId, setRunId] = useState('RUN-4821');
  const [toast, setToast] = useState('');
  const run = TEST_RUNS.find((r) => r.id === runId) as ITestRun;
  const onExport = (f: string) => { setToast(f); window.setTimeout(() => setToast(''), 2600); };
  return (
    <div>
      <PageHeader title="测试报告" desc="测试闭环最后一环 · 工程 / 服务 / 用例三级报告 · 可查看、可导出（HTML / PDF / Word / Markdown）">
        <span className="text-[11px] text-slate-400">按工程筛选：sys-payment-platform</span>
      </PageHeader>
      <Card title="选择运行 · 生成报告" extra={<span className="text-[11px] text-slate-400">点击运行生成对应测试报告（工程 / 服务 / 用例可逐级下钻）</span>}>
        <div className="flex flex-wrap gap-2">
          {TEST_RUNS.map((r) => (
            <button key={r.id} type="button" onClick={() => setRunId(r.id)}
              className={runId === r.id ? 'px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg' : 'px-3 py-1.5 text-xs bg-slate-100 text-slate-600 rounded-lg hover:bg-slate-200'}>
              <span className="font-mono">{r.id}</span>
              <span className="ml-2">{r.gate === '通过' ? '✓' : '✕'}{Math.round((r.pass / r.total) * 1000) / 10}%</span>
            </button>
          ))}
        </div>
        <p className="mt-3 text-[10px] text-slate-400">存量用例回放成本持平/递减；本次失败触发「新增分析」成本回升 —— 每次 CI 触发即产出一份可追溯、可独立验证的测试报告。</p>
      </Card>

      <ReportView run={run} onExport={onExport} />

      {toast && (
        <div className="fixed bottom-6 right-6 z-50 px-4 py-3 rounded-xl bg-slate-800 text-white text-xs shadow-xl flex items-center gap-2">
          <CheckCircle2 className="w-4 h-4 text-emerald-400" />已导出报告为 <span className="font-mono text-emerald-300">{toast}</span>（原型模拟下载）
        </div>
      )}
    </div>
  );
}
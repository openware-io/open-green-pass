import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { TEST_RUNS, RUN_SERVICE_REPORTS, EXEC_AUDIT_ROWS, CASE_COST_HISTORY, TEST_EVIDENCE, SCREENSHOT_POLICY, type ITestRun, type IScreenshotPolicy } from '@/data/mock';
import { PageHeader, Card, ListFilter } from '@/components/shared';
import { Activity, FileText, Image, FileJson, File, Video, ArrowUpRight, ArrowDownRight, Minus, ScanEye, Camera } from 'lucide-react';

const RESULT_BADGE: Record<string, string> = {
  '通过': 'bg-emerald-50 text-emerald-600',
  '失败': 'bg-red-50 text-red-600',
  '阻塞': 'bg-amber-50 text-amber-600',
};
const RISK_BADGE: Record<string, string> = { '低': 'bg-emerald-50 text-emerald-600', '中': 'bg-amber-50 text-amber-600', '高': 'bg-red-50 text-red-600' };
const EV_TYPE_ICON: Record<string, typeof Image> = { '截图': Image, '视频': Video, '日志': File, '请求响应': FileJson };

type TabKey = 'exec' | 'report' | 'evidence' | 'cost';
const TABS: { key: TabKey; label: string }[] = [
  { key: 'exec', label: '执行明细' },
  { key: 'report', label: '服务报告' },
  { key: 'evidence', label: '测试证据' },
  { key: 'cost', label: '用例成本' },
];

function RunTrend() {
  const W = 640, H = 150, PAD = 8;
  const rates = TEST_RUNS.map((r) => (r.pass / r.total) * 100);
  const max = Math.max(...rates);
  const min = Math.min(...rates);
  const x = (i: number) => PAD + (i * (W - PAD * 2)) / (TEST_RUNS.length - 1);
  const y = (v: number) => PAD + (1 - (v - min + 1) / (max - min + 2)) * (H - PAD * 2);
  const line = rates.map((v, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(' ');
  return (
    <div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-36">
        {[0, 1, 2].map((i) => <line key={i} x1="0" y1={PAD + (i * (H - PAD * 2)) / 2} x2={W} y2={PAD + (i * (H - PAD * 2)) / 2} stroke="#f1f5f9" strokeWidth="1" />)}
        <path d={line} fill="none" stroke="#10b981" strokeWidth="2" />
        {rates.map((v, i) => (
          <g key={i}>
            <circle cx={x(i)} cy={y(v)} r="3.5" fill="#10b981" stroke="white" strokeWidth="1.5" />
            <text x={x(i)} y={y(v) - 8} textAnchor="middle" fontSize="9" fill="#475569">{v.toFixed(1)}%</text>
            <text x={x(i)} y={H - 3} textAnchor="middle" fontSize="9" fill="#94a3b8">{TEST_RUNS[i].id.replace('RUN-', '')}</text>
          </g>
        ))}
      </svg>
      <p className="text-[10px] text-slate-400 mt-1">通过率 = 通过 / 总量 · RUN-4788 阻断点（trace 分支）· 本轮 RUN-4821 回落至 91.7%</p>
    </div>
  );
}

function CaseCostPanel({ caseId, cases, onSelect }: { caseId: string; cases: string[]; onSelect: (c: string) => void }) {
  const pts = CASE_COST_HISTORY[caseId] ?? [];
  const last = pts[pts.length - 1];
  const prev = pts[pts.length - 2];
  const delta = prev ? Math.round((((last?.cost ?? 0) - prev.cost) / prev.cost) * 1000) / 10 : null;
  const max = Math.max(...pts.map((p) => p.cost), 1);
  const W = 420, H = 130, PAD = 8;
  const x = (i: number) => PAD + (i * (W - PAD * 2)) / (pts.length - 1);
  const y = (v: number) => PAD + (1 - v / max) * (H - PAD * 2);
  return (
    <div>
      <div className="flex flex-wrap gap-2 mb-4">
        {cases.map((c) => (
          <button key={c} type="button" onClick={() => onSelect(c)}
            className={caseId === c ? 'px-2.5 py-1 text-[11px] bg-emerald-600 text-white rounded-lg' : 'px-2.5 py-1 text-[11px] bg-slate-100 text-slate-600 rounded-lg hover:bg-slate-200'}>{c}</button>
        ))}
      </div>
      <div className="grid grid-cols-4 gap-4 mb-4">
        <div className="card bg-white rounded-lg border border-slate-200 p-3">
          <div className="text-[11px] text-slate-500">本次成本</div>
          <div className="text-lg font-bold text-slate-800">¥{last?.cost}</div>
          <div className="text-[10px] text-slate-400">{last?.run} · {last?.kind}</div>
        </div>
        <div className="card bg-white rounded-lg border border-slate-200 p-3">
          <div className="text-[11px] text-slate-500">上次成本</div>
          <div className="text-lg font-bold text-slate-800">¥{prev?.cost ?? '—'}</div>
          <div className="text-[10px] text-slate-400">{prev?.run}</div>
        </div>
        <div className="card bg-white rounded-lg border border-slate-200 p-3">
          <div className="text-[11px] text-slate-500">环比</div>
          {delta === null ? <div className="text-lg font-bold text-slate-800">—</div> : (
            <div className={'text-lg font-bold flex items-center gap-1 ' + (delta > 0 ? 'text-amber-600' : delta < 0 ? 'text-emerald-600' : 'text-slate-700')}>
              {delta > 0 ? <ArrowUpRight className="w-4 h-4" /> : delta < 0 ? <ArrowDownRight className="w-4 h-4" /> : <Minus className="w-4 h-4" />}
              {Math.abs(delta)}%
            </div>
          )}
          <div className="text-[10px] text-slate-400">本次 vs 上次</div>
        </div>
        <div className="card bg-white rounded-lg border border-slate-200 p-3">
          <div className="text-[11px] text-slate-500">本次结果</div>
          <div><span className={`px-2 py-0.5 rounded-full text-[10px] ${RESULT_BADGE[last?.result ?? '']}`}>{last?.result}</span></div>
          <div className="text-[10px] text-slate-400 mt-1">{last?.kind}</div>
        </div>
      </div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-32">
        <path d={pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(p.cost).toFixed(1)}`).join(' ')} fill="none" stroke="#6366f1" strokeWidth="2" />
        {pts.map((p, i) => (
          <g key={i}>
            <circle cx={x(i)} cy={y(p.cost)} r="3.5" fill={p.result === '失败' ? '#ef4444' : '#10b981'} stroke="white" strokeWidth="1.5" />
            <text x={x(i)} y={y(p.cost) - 7} textAnchor="middle" fontSize="9" fill="#475569">¥{p.cost}</text>
            <text x={x(i)} y={H - 3} textAnchor="middle" fontSize="9" fill="#94a3b8">{p.run.replace('RUN-', '')}</text>
          </g>
        ))}
      </svg>
      <p className="text-[10px] text-slate-400 mt-1">历史成本对比：存量回放成本随治理持平或递减；本次失败触发「新增分析」成本回升 —— 符合治理降本逻辑</p>
    </div>
  );
}

export default function HistoryPage() {
  const [runId, setRunId] = useState<string>('RUN-4821');
  const [q, setQ] = useState('');
  const [gate, setGate] = useState('');
  const kw = q.trim().toLowerCase();
  const runs = TEST_RUNS.filter((r) => {
    if (gate && r.gate !== gate) return false;
    if (kw && !(r.id + r.branch + r.trigger + r.ts).toLowerCase().includes(kw)) return false;
    return true;
  });
  const [tab, setTab] = useState<TabKey>('exec');
  const [caseForEvid, setCaseForEvid] = useState('TC-2024-118');
  const [policies, setPolicies] = useState<IScreenshotPolicy[]>(SCREENSHOT_POLICY);
  const togglePolicy = (id: string) => setPolicies((prev) => prev.map((pl) => (pl.serviceId === id ? { ...pl, enabled: !pl.enabled, mode: pl.enabled ? 'off' : 'on-fail' } : pl)));
  const [caseForCostSel, setCaseForCostSel] = useState('TC-2024-118');

  const navigate = useNavigate();
  const run = TEST_RUNS.find((r) => r.id === runId) as ITestRun;
  const passRate = Math.round((run.pass / run.total) * 1000) / 10;
  // 运行要点从 mock 派生（避免硬编码与实际数据不一致）
  const totalExec = TEST_RUNS.reduce((a, r) => a + r.total, 0);
  const avgPass = Math.round((TEST_RUNS.reduce((a, r) => a + r.pass, 0) / totalExec) * 1000) / 10;
  const avgCost = Math.round(TEST_RUNS.reduce((a, r) => a + r.cost, 0) / TEST_RUNS.length);
  const lastGreen = TEST_RUNS.find((r) => r.gate === '通过')?.id ?? '—';
  const failRegressions = Object.entries(CASE_COST_HISTORY).filter(([, pts]) => pts[pts.length - 1].result === '失败').map(([id]) => id).join(' / ') || '无';
  const cases = Object.keys(CASE_COST_HISTORY);
  const evidCases = Object.keys(TEST_EVIDENCE);

  return (
    <div>
      <PageHeader title="测试历史" desc="每次 CI 触发 = 一次运行 · 时间维主键 · 报告 / 证据 / 成本联动">
        <span className="text-[11px] text-slate-400">按工程筛选：sys-payment-platform</span>
      </PageHeader>

      <div className="grid grid-cols-3 gap-5 mb-5">
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 p-5">
          <h3 className="font-semibold text-slate-700 text-sm mb-2 flex items-center gap-1.5"><Activity className="w-4 h-4 text-emerald-500" />运行通过率趋势（近 6 次）</h3>
          <RunTrend />
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <h3 className="font-semibold text-slate-700 text-sm mb-3 flex items-center gap-1.5"><ScanEye className="w-4 h-4 text-emerald-600" />运行要点</h3>
          <div className="space-y-2.5 text-[11px]">
            <div className="flex justify-between"><span className="text-slate-500">历史共执行</span><span className="text-slate-700 font-medium">{TEST_RUNS.length} 次 / {totalExec.toLocaleString()} 用例次</span></div>
            <div className="flex justify-between"><span className="text-slate-500">平均通过率</span><span className="text-emerald-600 font-medium">{avgPass}%</span></div>
            <div className="flex justify-between"><span className="text-slate-500">平均成本 / 次</span><span className="text-slate-700 font-medium">¥{avgCost.toLocaleString()}</span></div>
            <div className="flex justify-between"><span className="text-slate-500">上次全绿运行</span><span className="text-slate-700 font-medium">{lastGreen}</span></div>
            <div className="flex justify-between"><span className="text-slate-500">失败回归数</span><span className="text-amber-600 font-medium">{failRegressions}</span></div>
          </div>
        </div>
      </div>

      {/* 运行列表 */}
      <Card title="运行列表" extra={<span className="text-[11px] text-slate-400">点击行查看运行详情（报告 / 证据 / 成本）</span>}>
        <div className="px-5 pt-3 flex items-center justify-between">
          <ListFilter search={q} onSearch={setQ}
            selects={[{ key: 'gate', label: '门禁', options: ['通过', '阻断'], value: gate, onChange: setGate }]} />
          <span className="text-[11px] text-slate-400">共 {runs.length} / {TEST_RUNS.length} 条</span>
        </div>
        <table className="w-full text-xs">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-slate-500">
              <th className="px-4 py-2.5 font-medium">运行 ID</th>
              <th className="px-4 py-2.5 font-medium">时间</th>
              <th className="px-4 py-2.5 font-medium">分支</th>
              <th className="px-4 py-2.5 font-medium">触发</th>
              <th className="px-4 py-2.5 font-medium">通过率</th>
              <th className="px-4 py-2.5 font-medium">成本</th>
              <th className="px-4 py-2.5 font-medium">门禁</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {runs.map((r) => (
              <tr key={r.id} onClick={() => setRunId(r.id)}
                className={'cursor-pointer ' + (runId === r.id ? 'bg-emerald-50/50' : 'hover:bg-slate-50')}>
                <td className="px-4 py-2.5 font-mono text-emerald-700 font-medium">{r.id}</td>
                <td className="px-4 py-2.5 text-slate-500">{r.ts}</td>
                <td className="px-4 py-2.5 font-mono text-slate-600">{r.branch}</td>
                <td className="px-4 py-2.5 text-slate-500">{r.trigger}</td>
                <td className="px-4 py-2.5">
                  <div className="flex items-center gap-2">
                    <div className="w-16 h-1.5 bg-slate-100 rounded-full"><div className={'h-full rounded-full ' + (r.gate === '通过' ? 'bg-emerald-500' : 'bg-red-500')} style={{ width: `${(r.pass / r.total) * 100}%` }} /></div>
                    <span className={'text-[11px] font-medium ' + (r.gate === '通过' ? 'text-emerald-600' : 'text-red-600')}>{Math.round((r.pass / r.total) * 1000) / 10}%</span>
                  </div>
                </td>
                <td className="px-4 py-2.5 font-mono text-slate-600">¥{r.cost.toLocaleString()}</td>
                <td className="px-4 py-2.5"><span className={'px-2 py-0.5 rounded-full text-[10px] ' + (r.gate === '通过' ? 'bg-emerald-50 text-emerald-600' : 'bg-red-50 text-red-600')}>{r.gate}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>

      {/* 运行详情 */}
      <div className="card bg-white rounded-xl border border-slate-200 p-5 mt-5">
        <div className="flex items-center justify-between mb-1">
          <h3 className="font-semibold text-slate-700 text-sm">运行详情 · <span className="font-mono text-emerald-700">{run.id}</span> <span className="text-slate-400 font-normal">（{run.ts} · {run.branch}）</span></h3>
          <button type="button" onClick={() => navigate('/report')} className="px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg flex items-center gap-1"><FileText className="w-3.5 h-3.5" />查看工程报告</button>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-4">
          <div className="p-3 rounded-lg bg-emerald-50 border border-emerald-200"><div className="text-[11px] text-emerald-600">通过率</div><div className="text-lg font-bold text-emerald-700">{passRate}%</div><div className="text-[10px] text-emerald-500">{run.pass}/{run.total}</div></div>
          <div className="p-3 rounded-lg bg-red-50 border border-red-200"><div className="text-[11px] text-red-600">失败</div><div className="text-lg font-bold text-red-600">{run.fail}</div><div className="text-[10px] text-red-400">阻塞 {run.block}</div></div>
          <div className="p-3 rounded-lg bg-slate-50 border border-slate-200"><div className="text-[11px] text-slate-500">成本</div><div className="text-lg font-bold text-slate-700">¥{run.cost.toLocaleString()}</div><div className="text-[10px] text-slate-400">关联用例成本</div></div>
          <div className="p-3 rounded-lg bg-slate-50 border border-slate-200"><div className="text-[11px] text-slate-500">时长</div><div className="text-lg font-bold text-slate-700">{run.duration}</div><div className="text-[10px] text-slate-400">12 执行器</div></div>
          <div className="p-3 rounded-lg bg-slate-50 border border-slate-200"><div className="text-[11px] text-slate-500">门禁结论</div><div className={'text-lg font-bold ' + (run.gate === '通过' ? 'text-emerald-600' : 'text-red-600')}>{run.gate}</div><div className="text-[10px] text-slate-400">{run.trigger}</div></div>
        </div>

        {/* tabs */}
        <div className="flex gap-1 mb-4">
          {TABS.map((t) => (
            <button key={t.key} type="button" onClick={() => setTab(t.key)}
              className={tab === t.key ? 'px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg' : 'px-3 py-1.5 text-xs bg-slate-100 text-slate-600 rounded-lg hover:bg-slate-200'}>{t.label}</button>
          ))}
        </div>

        {/* 执行明细 */}
        {tab === 'exec' && (
          <div className="rounded-xl border border-slate-200 overflow-hidden">
            <table className="w-full text-xs">
              <thead className="bg-slate-50 border-b border-slate-200">
                <tr className="text-left text-slate-500">
                  <th className="px-4 py-2.5 font-medium">用例 ID</th>
                  <th className="px-4 py-2.5 font-medium">所属服务</th>
                  <th className="px-4 py-2.5 font-medium">结果</th>
                  <th className="px-4 py-2.5 font-medium">耗时</th>
                  <th className="px-4 py-2.5 font-medium">成本</th>
                  <th className="px-4 py-2.5 font-medium">时间</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {EXEC_AUDIT_ROWS.map((r) => {
                  const his = CASE_COST_HISTORY[r.caseId];
                  const cost = his ? his[his.length - 1].cost : '—';
                  return (
                    <tr key={r.caseId + r.ts} className="hover:bg-slate-50">
                      <td className="px-4 py-3 font-mono text-emerald-700 font-medium">{r.caseId}</td>
                      <td className="px-4 py-3 font-mono text-slate-500">{r.asset}</td>
                      <td className="px-4 py-3"><span className={`px-2 py-0.5 rounded-full text-[10px] ${RESULT_BADGE[r.result]}`}>{r.result}</span></td>
                      <td className="px-4 py-3 font-mono text-slate-500">{r.duration}</td>
                      <td className="px-4 py-3 font-mono text-slate-600">¥{typeof cost === 'number' ? cost.toFixed(1) : cost}</td>
                      <td className="px-4 py-3 text-slate-400">{r.ts}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            <p className="px-4 py-2.5 text-[10px] text-slate-400">成本列关联「用例成本历史」——点开下方「用例成本」tab 可对比同一用例的历史成本。</p>
          </div>
        )}

        {/* 服务报告 */}
        {tab === 'report' && (
          <div>
            <div className="mb-4 px-4 py-3 rounded-xl bg-emerald-50 border border-emerald-200">
              <div className="text-sm font-semibold text-emerald-800">工程报告摘要 · {run.id}</div>
              <div className="text-[11px] text-emerald-700 mt-1">整体通过率 {passRate}% · 门禁结论：<span className="font-semibold">{run.gate}</span> · 累计成本 ¥{run.cost.toLocaleString()} · 高风险服务 1（svc-payment）</div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {RUN_SERVICE_REPORTS.map((s) => (
                <div key={s.service} className={'rounded-xl border p-4 ' + (s.risk === '高' ? 'bg-red-50/40 border-red-200' : s.risk === '中' ? 'bg-amber-50/30 border-amber-200' : 'bg-white border-slate-200')}>
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2"><span className="font-mono text-emerald-700 font-medium text-sm">{s.service}</span><span className="text-[11px] text-slate-400">{s.name}</span></div>
                    <span className={`px-2 py-0.5 rounded-full text-[10px] ${RISK_BADGE[s.risk]}`}>风险 {s.risk}</span>
                  </div>
                  <div className="flex gap-4 text-xs text-slate-600 mb-2">
                    <span className="text-emerald-600">通过 {s.pass}</span>
                    <span className="text-red-600">失败 {s.fail}</span>
                    <span className="text-amber-600">阻塞 {s.block}</span>
                    <span className="text-slate-500">覆盖 {s.coverage}%</span>
                    <span className="font-mono text-slate-600">¥{s.cost.toLocaleString()}</span>
                  </div>
                  {s.risks.length > 0 && (
                    <div className="mb-1.5 flex flex-wrap gap-1">{s.risks.map((r) => <span key={r} className="text-[10px] bg-red-100 text-red-700 px-1.5 py-0.5 rounded">{r}</span>)}</div>
                  )}
                  <div className="text-[11px] text-slate-500">{s.conclusion}</div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* 测试证据 */}
        {tab === 'evidence' && (
          <div>
            {/* 截图证据规范 */}
            <div className="mb-4 p-4 rounded-xl bg-emerald-50 border border-emerald-200">
              <div className="flex items-center gap-1.5 mb-1">
                <Camera className="w-4 h-4 text-emerald-700" />
                <span className="text-sm font-semibold text-emerald-800">截图证据 · 普遍规范</span>
                <span className="ml-auto text-[10px] text-emerald-600">关键流程全程 + 失败现场自动截图</span>
              </div>
              <p className="text-[11px] text-emerald-700">截图作为测试证据的普遍规范：关键流程执行全程截图、断言失败自动捕获失败现场，默认开启；服务可单独关闭，以避免高并发 / 大流量服务的截图性能开销。</p>
              <div className="mt-3 rounded-lg border border-emerald-200 bg-white overflow-hidden">
                <table className="w-full text-[11px]">
                  <thead className="bg-emerald-50/60">
                    <tr className="text-left text-emerald-600">
                      <th className="px-3 py-1.5 font-medium">服务</th>
                      <th className="px-3 py-1.5 font-medium">截图模式</th>
                      <th className="px-3 py-1.5 font-medium">状态</th>
                      <th className="px-3 py-1.5 font-medium">说明</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {policies.map((pl) => (
                      <tr key={pl.serviceId}>
                        <td className="px-3 py-1.5 font-mono text-emerald-700 font-medium">{pl.serviceId}</td>
                        <td className="px-3 py-1.5 text-slate-600">{pl.mode === 'always' ? '全程截图' : pl.mode === 'on-fail' ? '仅失败时' : '关闭'}</td>
                        <td className="px-3 py-1.5">
                          <button type="button" onClick={() => togglePolicy(pl.serviceId)}
                            className={'px-2 py-0.5 rounded-full text-[10px] ' + (pl.enabled ? 'bg-emerald-50 text-emerald-600' : 'bg-slate-100 text-slate-400')}>{pl.enabled ? '启用' : '关闭'}</button>
                        </td>
                        <td className="px-3 py-1.5 text-slate-500">{pl.reason}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <p className="mt-2 text-[10px] text-emerald-500">服务级策略覆盖普遍规范 · 关闭可防截图导致的性能问题 · 证据仍按「用例→Run」组织并哈希锚定</p>
            </div>
            <div className="flex flex-wrap gap-2 mb-4">
              {evidCases.map((c) => (
                <button key={c} type="button" onClick={() => setCaseForEvid(c)}
                  className={caseForEvid === c ? 'px-2.5 py-1 text-[11px] bg-emerald-600 text-white rounded-lg' : 'px-2.5 py-1 text-[11px] bg-slate-100 text-slate-600 rounded-lg hover:bg-slate-200'}>{c}</button>
              ))}
            </div>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
              {(TEST_EVIDENCE[caseForEvid] ?? []).map((ev, i) => {
                const Icon = EV_TYPE_ICON[ev.type] ?? Image;
                return (
                  <div key={i} className="rounded-xl border border-slate-200 p-3">
                    <div className="aspect-video bg-slate-100 rounded-lg flex items-center justify-center text-slate-400 border border-slate-200 mb-2">
                      <Icon className="w-8 h-8" />
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-medium text-slate-700">{ev.type}</span>
                      <span className="font-mono text-[9px] text-slate-400">{ev.file}</span>
                    </div>
                    <div className="text-[10px] text-slate-500 mt-1">{ev.note}</div>
                    <div className="mt-1.5 flex items-center gap-2 text-[10px]">
                      <span className="font-mono text-emerald-600">{ev.hash}</span>
                      <span className="text-slate-400">{ev.ts}</span>
                    </div>
                    <div className="mt-1.5 text-[9px] text-emerald-500 flex items-center gap-1"><ScanEye className="w-3 h-3" />哈希锚定 · 可独立验证</div>
                  </div>
                );
              })}
            </div>
            <p className="mt-3 text-[10px] text-slate-400">测试证据按「用例 → 本次运行」组织，每条带 sha256 哈希并锚定审计链，与「一份链四视图」同源 —— 证据本身不可篡改、可独立验证。</p>
          </div>
        )}

        {/* 用例成本 */}
        {tab === 'cost' && (
          <div>
            <CaseCostPanel caseId={caseForCostSel} cases={cases} onSelect={setCaseForCostSel} />
          </div>
        )}
      </div>


    </div>
  );
}
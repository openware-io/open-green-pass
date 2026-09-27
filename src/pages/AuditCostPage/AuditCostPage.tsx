import { useState } from 'react';
import { COST_GEN_DETAIL, COST_EXEC_DETAIL, COST_SCENARIOS, COST_TOTAL, COST_TOTAL_TOKENS_IN, COST_TOTAL_TOKENS_OUT, COST_AICALLS, COST_MOM_CHANGE, COST_PER_CASE, COST_TREND, COST_EXEC_LEGACY_TOTAL, COST_EXEC_NEW_TOTAL, COST_ORGS, COST_BY_GROUP, COST_BY_SERVICE, COST_HEAT, SERVICE_ORG_MAP, AI_MODELS, CASE_COST_HISTORY } from '@/data/mock';
import { PageHeader, Card, ListFilter } from '@/components/shared';
import { Wallet, TrendingDown, Cpu, Layers, ArrowDownRight, ArrowUpRight, History } from 'lucide-react';

const SCEN_COLOR: Record<string, string> = {
  '用例生成': 'bg-emerald-500', '测试执行辅助': 'bg-teal-500', '质量判定': 'bg-indigo-500', '契约分析': 'bg-amber-500', '变异/篡改检测': 'bg-slate-400',
};

// 近14天趋势线 SVG（total 波动，exec 存量执行成本持平/递减）
function TrendChart() {
  const pts = COST_TREND;
  const W = 640, H = 170, PAD = 8;
  const maxT = Math.max(...pts.map((p) => p.total)) + 2;
  const maxE = Math.max(...pts.map((p) => p.exec)) + 1;
  const x = (i: number) => PAD + (i * (W - PAD * 2)) / (pts.length - 1);
  const yT = (v: number) => PAD + (1 - v / maxT) * (H - PAD * 2);
  const yE = (v: number) => PAD + (1 - v / maxE) * (H - PAD * 2);
  const line = (fn: (v: number) => number, get: (p: (typeof pts)[0]) => number) =>
    pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${fn(get(p)).toFixed(1)}`).join(' ');
  return (
    <div>
      <div className="flex gap-3 text-xs mb-3">
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-indigo-500" />总成本</span>
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-emerald-500" />存量用例执行成本</span>
        <span className="text-[10px] text-slate-400 ml-auto">存量执行成本持平或递减（治理降本）</span>
      </div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-40">
        <line x1="0" y1={yT(maxT / 2)} x2={W} y2={yT(maxT / 2)} stroke="#f1f5f9" strokeWidth="1" />
        <line x1="0" y1={H - PAD} x2={W} y2={H - PAD} stroke="#e2e8f0" strokeWidth="1" />
        <path d={line(yE, (p) => p.exec) + ' L' + (W - PAD) + ',' + (H - PAD) + ' L' + PAD + ',' + (H - PAD) + ' Z'} fill="#10b981" opacity="0.06" />
        <path d={line(yT, (p) => p.total)} fill="none" stroke="#6366f1" strokeWidth="2" />
        <path d={line(yE, (p) => p.exec)} fill="none" stroke="#10b981" strokeWidth="2" />
        <circle cx={x(pts.length - 1)} cy={yT(pts[pts.length - 1].total)} r="3" fill="#6366f1" />
        <circle cx={x(pts.length - 1)} cy={yE(pts[pts.length - 1].exec)} r="3" fill="#10b981" />
      </svg>
      <div className="flex justify-between text-[10px] text-slate-400 mt-1">
        {pts.map((p) => <span key={p.day}>{p.day}</span>)}
      </div>

    </div>
  );
}


type DimKey = 'org' | 'group' | 'service';
const DIMS: { key: DimKey; label: string }[] = [
  { key: 'org', label: '研发组' },
  { key: 'group', label: '工程 / 服务组' },
  { key: 'service', label: '服务' },
];

export default function AuditCostPage() {
  const [dim, setDim] = useState<DimKey>('org');
  const [genQ, setGenQ] = useState('');
  const [execQ, setExecQ] = useState('');
  const [execKind, setExecKind] = useState('');
  const gkw = genQ.trim().toLowerCase();
  const ekw = execQ.trim().toLowerCase();
  const genFiltered = COST_GEN_DETAIL.filter((d) => !gkw || (d.id + d.caseId + d.source + d.asset).toLowerCase().includes(gkw));
  const execFiltered = COST_EXEC_DETAIL.filter((d) => {
    if (execKind === '存量' && !d.isLegacy) return false;
    if (execKind === '新增' && d.isLegacy) return false;
    if (ekw && !(d.caseId + d.asset + d.id).toLowerCase().includes(ekw)) return false;
    return true;
  });
  const [costCase, setCostCase] = useState('TC-2024-118');
  const modelName = (id: string) => AI_MODELS.find((m) => m.id === id)?.name ?? id;
  const maxScen = Math.max(...COST_SCENARIOS.map((s) => s.amount));
  const fmtM = (v: number) => { const m = v / 1e6; return (Number.isInteger(m) ? m.toFixed(0) : m.toFixed(1)) + 'M'; };
  const genOrg = (d: (typeof COST_GEN_DETAIL)[0]) => SERVICE_ORG_MAP[d.asset] ?? '—';

  // 维度聚合数据
  const dimData = dim === 'org'
    ? COST_ORGS.map((o) => ({ name: o.name, amount: o.amount }))
    : dim === 'group'
      ? COST_BY_GROUP.map((g) => ({ name: g.name, amount: g.amount }))
      : COST_BY_SERVICE.map((s) => ({ name: `${s.service} · ${s.name}`, amount: s.amount }));
  const dimTotal = dimData.reduce((s, d) => s + d.amount, 0);
  const maxDim = Math.max(...dimData.map((d) => d.amount));
  const dimLabel = DIMS.find((d) => d.key === dim)?.label ?? '';

  return (
    <div>
      <PageHeader title="成本审计" desc="AI 场景成本 · 维度归因 · 治理降本">
        <span className="text-[11px] text-slate-400">统计周期：近 30 天 · 金额 = 单价表 × token 量（输入+输出）</span>
      </PageHeader>

      {/* 总览 KPI */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-5">
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">AI 总成本（¥）</span><Wallet className="w-4 h-4 text-emerald-500" /></div>
          <div className="text-2xl font-bold text-slate-800">{COST_TOTAL.toLocaleString()}</div>
          <div className="mt-1 flex items-center gap-1 text-[11px] text-emerald-600"><TrendingDown className="w-3 h-3" />环比 {Math.abs(COST_MOM_CHANGE)}%（治理降本）</div>
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">Token 总量</span><Layers className="w-4 h-4 text-indigo-500" /></div>
          <div className="text-xl font-bold text-slate-800">{(COST_TOTAL_TOKENS_IN / 1e6).toFixed(0)}M<span className="text-sm font-normal text-slate-400"> 入</span> / {(COST_TOTAL_TOKENS_OUT / 1e6).toFixed(0)}M<span className="text-sm font-normal text-slate-400"> 出</span></div>
          <div className="mt-1 text-[11px] text-slate-400">输入输出分别计费</div>
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">AI 调用次数</span><Cpu className="w-4 h-4 text-amber-500" /></div>
          <div className="text-2xl font-bold text-slate-800">{COST_AICALLS.toLocaleString()}</div>
          <div className="mt-1 text-[11px] text-slate-400">用例生成 · 判定 · 契约 · 执行</div>
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">平均每用例成本</span><ArrowDownRight className="w-4 h-4 text-emerald-500" /></div>
          <div className="text-2xl font-bold text-slate-800">¥{COST_PER_CASE}</div>
          <div className="mt-1 text-[11px] text-slate-400">存量用例成本趋稳</div>
        </div>
      </div>

      {/* 维度归因 */}
      <Card title={<span className="flex items-center gap-1.5"><ArrowUpRight className="w-4 h-4 text-emerald-500" />成本维度归因</span>}
        extra={<span className="text-[11px] text-slate-400">面包屑下钻 · 各维度合计 = 总成本 ¥{COST_TOTAL.toLocaleString()}</span>}>
        <div className="flex items-center gap-2 mb-4">
          {DIMS.map((d) => (
            <button key={d.key} type="button" onClick={() => setDim(d.key)}
              className={dim === d.key ? 'px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg' : 'px-3 py-1.5 text-xs bg-slate-100 text-slate-500 rounded-lg hover:bg-slate-200'}>
              {d.label}
            </button>
          ))}
          <span className="ml-2 text-xs text-slate-400">当前：<span className="font-medium text-emerald-600">{dimLabel}</span> / 总成本 ¥{dimTotal.toLocaleString()}</span>
        </div>

        <div className="grid grid-cols-3 gap-5">
          <div className="col-span-2">
            {/* 柱状图 */}
            <div className="h-48 flex items-end gap-3 px-2 border-b border-slate-200">
              {dimData.map((d) => (
                <div key={d.name} className="flex-1 flex flex-col items-center justify-end gap-1 group">
                  <span className="text-[10px] text-slate-500">¥{d.amount.toLocaleString()}</span>
                  <div className="w-full max-w-24 rounded-t-lg bg-gradient-to-t from-emerald-500 to-emerald-300 hover:to-emerald-200 transition-all" style={{ height: `${Math.max(6, (d.amount / maxDim) * 130)}px` }} title={`${d.name} ¥${d.amount}`} />
                </div>
              ))}
            </div>
            <div className="flex gap-3 px-2 pt-1.5">
              {dimData.map((d) => <div key={d.name} className="flex-1 text-[10px] text-slate-500 text-center truncate">{d.name}</div>)}
            </div>
          </div>
          <div className="rounded-xl border border-slate-200 overflow-hidden">
            <table className="w-full text-xs">
              <thead className="bg-slate-50 border-b border-slate-200">
                <tr className="text-left text-slate-500">
                  <th className="px-3 py-2 font-medium">{dimLabel}</th>
                  <th className="px-3 py-2 font-medium text-right">成本（¥）</th>
                  <th className="px-3 py-2 font-medium text-right">占比</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {dimData.map((d) => (
                  <tr key={d.name} className="hover:bg-slate-50">
                    <td className="px-3 py-2 text-slate-600">{d.name}</td>
                    <td className="px-3 py-2 text-right font-medium text-slate-700">¥{d.amount.toLocaleString()}</td>
                    <td className="px-3 py-2 text-right text-slate-500">{Math.round((d.amount / dimTotal) * 100)}%</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* 维度 × 场景 热力矩阵 */}
        <h3 className="font-semibold text-slate-700 text-sm mt-6 mb-3 flex items-center gap-1.5"><Layers className="w-4 h-4 text-indigo-500" />服务 × 场景 热力矩阵（¥）</h3>
        <div className="rounded-xl border border-slate-200 overflow-hidden">
          <table className="w-full text-xs">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-slate-500">
                <th className="px-3 py-2 font-medium">服务</th>
                <th className="px-3 py-2 font-medium">研发组</th>
                {(['生成', '执行', '判定', '契约', '变异'] as const).map((s) => <th key={s} className="px-3 py-2 font-medium text-right">{s}</th>)}
                <th className="px-3 py-2 font-medium text-right">合计</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {COST_HEAT.map((h) => {
                const row = h.cells;
                const rowSum = row.生成 + row.执行 + row.判定 + row.契约 + row.变异;
                return (
                  <tr key={h.serviceId} className="hover:bg-slate-50">
                    <td className="px-3 py-2 font-mono text-indigo-600 font-medium">{h.serviceId}</td>
                    <td className="px-3 py-2 text-slate-500">{h.org}</td>
                    {(['生成', '执行', '判定', '契约', '变异'] as const).map((k) => (
                      <td key={k} className="px-3 py-2 text-right">
                        <span className="inline-block px-2 py-0.5 rounded" style={{ background: `rgba(16,185,129,${0.12 + (row[k] / 600) * 0.5})`, color: '#065f46' }}>{row[k]}</span>
                      </td>
                    ))}
                    <td className="px-3 py-2 text-right font-semibold text-slate-700">¥{rowSum.toLocaleString()}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </Card>

      {/* 趋势 + 场景占比 */}
      <div className="grid grid-cols-3 gap-5 my-5">
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 p-5">
          <h3 className="font-semibold text-slate-700 text-sm mb-3">成本趋势（近 14 天，¥）</h3>
          <TrendChart />
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <h3 className="font-semibold text-slate-700 text-sm mb-3">场景成本占比</h3>
          <div className="space-y-3">
            {COST_SCENARIOS.map((s) => (
              <div key={s.scenario}>
                <div className="flex justify-between text-xs mb-1"><span className="text-slate-600">{s.scenario}</span><span className="text-slate-500">¥{s.amount.toLocaleString()} · {Math.round((s.amount / COST_TOTAL) * 100)}%</span></div>
                <div className="h-2 bg-slate-100 rounded-full"><div className={`h-full rounded-full ${SCEN_COLOR[s.scenario] ?? 'bg-slate-400'}`} style={{ width: `${(s.amount / maxScen) * 100}%` }} /></div>
                <div className="text-[10px] text-slate-400 mt-0.5">{s.note}</div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* 存量逻辑说明 */}
      <div className="mb-5 px-4 py-3 rounded-xl bg-emerald-50 border border-emerald-200 flex items-start gap-3">
        <TrendingDown className="w-4 h-4 text-emerald-600 flex-shrink-0 mt-0.5" />
        <div className="text-[11px] text-emerald-800 leading-relaxed">
          <span className="font-semibold">存量用例执行成本逻辑：</span>
          存量用例（已稳定）执行走纯回放 + 断言缓存，AI 不重复生成/重判，成本应<span className="font-semibold">持平或递减</span>；新增/变更用例才会引入新的 AI 分析成本。
          当前存量执行成本 ¥{COST_EXEC_LEGACY_TOTAL.toFixed(0)} 显著低于新增 ¥{COST_EXEC_NEW_TOTAL.toFixed(0)}；趋势图可见存量执行成本整体持平或递减，仅失败用例触发「新增分析」时小幅回升 —— 符合治理降本预期。
        </div>
      </div>

      {/* 明细：用例生成成本 */}
      <h3 className="font-semibold text-slate-700 text-sm mb-3 flex items-center gap-1.5"><ArrowUpRight className="w-4 h-4 text-emerald-500" />用例生成成本明细</h3>
      <div className="flex items-center justify-between mb-3">
        <ListFilter search={genQ} onSearch={setGenQ} />
        <span className="text-[11px] text-slate-400">共 {genFiltered.length} / {COST_GEN_DETAIL.length} 条</span>
      </div>
      <div className="rounded-xl border border-slate-200 overflow-hidden mb-5">
        <table className="w-full text-xs">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-slate-500">
              <th className="px-4 py-2.5 font-medium">生成批次</th>
              <th className="px-4 py-2.5 font-medium">用例 ID</th>
              <th className="px-4 py-2.5 font-medium">触发源</th>
              <th className="px-4 py-2.5 font-medium">归属研发组 / 服务</th>
              <th className="px-4 py-2.5 font-medium">模型</th>
              <th className="px-4 py-2.5 font-medium">Token 入/出</th>
              <th className="px-4 py-2.5 font-medium text-right">成本（¥）</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {genFiltered.map((d) => (
              <tr key={d.id} className="hover:bg-slate-50">
                <td className="px-4 py-3 font-mono text-slate-500">{d.id}</td>
                <td className="px-4 py-3 font-mono text-indigo-600 font-medium">{d.caseId}</td>
                <td className="px-4 py-3 text-slate-600">{d.source}</td>
                <td className="px-4 py-3"><span className="text-[10px] bg-emerald-50 text-emerald-600 px-1.5 py-0.5 rounded">{genOrg(d)}</span> <span className="text-[10px] text-slate-400 font-mono">{d.asset}</span></td>
                <td className="px-4 py-3 text-slate-600">{modelName(d.modelId)}</td>
                <td className="px-4 py-3 font-mono text-slate-400">{fmtM(d.tokensIn)} / {fmtM(d.tokensOut)}</td>
                <td className="px-4 py-3 text-right font-medium text-slate-700">¥{d.amount}</td>
              </tr>
            ))}
          </tbody>
          <tfoot className="bg-slate-50 border-t border-slate-200">
            <tr><td colSpan={6} className="px-4 py-2.5 text-[11px] text-slate-500 font-medium text-right">小计 · {genFiltered.length} 条</td><td className="px-4 py-2.5 text-right font-semibold text-emerald-600">¥{genFiltered.reduce((s, d) => s + d.amount, 0).toLocaleString()}</td></tr>
          </tfoot>
        </table>
      </div>

      {/* 明细：测试用例执行成本 */}
      <h3 className="font-semibold text-slate-700 text-sm mb-3 flex items-center gap-1.5"><ArrowDownRight className="w-4 h-4 text-emerald-500" />测试用例执行成本明细</h3>
      <div className="flex items-center justify-between mb-3">
        <ListFilter search={execQ} onSearch={setExecQ}
          selects={[{ key: 'kind', label: '类别', options: ['存量', '新增'], value: execKind, onChange: setExecKind }]} />
        <span className="text-[11px] text-slate-400">共 {execFiltered.length} / {COST_EXEC_DETAIL.length} 条</span>
      </div>
      <div className="rounded-xl border border-slate-200 overflow-hidden">
        <table className="w-full text-xs">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-slate-500">
              <th className="px-4 py-2.5 font-medium">用例 ID</th>
              <th className="px-4 py-2.5 font-medium">归属研发组 / 服务</th>
              <th className="px-4 py-2.5 font-medium">类别</th>
              <th className="px-4 py-2.5 font-medium">模型</th>
              <th className="px-4 py-2.5 font-medium">Token 入/出</th>
              <th className="px-4 py-2.5 font-medium text-right">成本（¥）</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {execFiltered.map((d) => (
              <tr key={d.id} className={'hover:bg-slate-50 ' + (d.isLegacy ? '' : 'bg-amber-50/30')}>
                <td className="px-4 py-3 font-mono text-indigo-600 font-medium">{d.caseId}</td>
                <td className="px-4 py-3"><span className="text-[10px] bg-emerald-50 text-emerald-600 px-1.5 py-0.5 rounded">{genOrg(d)}</span> <span className="text-[10px] text-slate-400 font-mono">{d.asset}</span></td>
                <td className="px-4 py-3"><span className={`px-2 py-0.5 rounded-full text-[10px] ${d.isLegacy ? 'bg-emerald-50 text-emerald-600' : 'bg-amber-50 text-amber-600'}`}>{d.isLegacy ? '存量回放' : '新增用例'}</span></td>
                <td className="px-4 py-3 text-slate-600">{modelName(d.modelId)}</td>
                <td className="px-4 py-3 font-mono text-slate-400">{fmtM(d.tokensIn)} / {fmtM(d.tokensOut)}</td>
                <td className="px-4 py-3 text-right font-medium text-slate-700">¥{d.amount}</td>
              </tr>
            ))}
          </tbody>
          <tfoot className="bg-slate-50 border-t border-slate-200">
            <tr><td colSpan={5} className="px-4 py-2.5 text-[11px] text-slate-500 font-medium text-right">小计 ¥{execFiltered.reduce((s, d) => s + d.amount, 0).toFixed(1)} · 存量 ¥{COST_EXEC_LEGACY_TOTAL.toFixed(1)} / 新增 ¥{COST_EXEC_NEW_TOTAL.toFixed(1)}</td><td className="px-4 py-2.5 text-right font-semibold text-emerald-600">¥{execFiltered.reduce((s, d) => s + d.amount, 0).toFixed(1)}</td></tr>
          </tfoot>
        </table>
      </div>
      {/* 用例成本历史对比 */}
      <Card title={<span className="flex items-center gap-1.5"><History className="w-4 h-4 text-emerald-500" />用例成本历史对比</span>}
        extra={<span className="text-[11px] text-slate-400">同一用例跨 Run 成本对比 · 存量回放持平 / 新增分析回升</span>}>
        <div className="flex flex-wrap gap-2 mb-4">
          {Object.keys(CASE_COST_HISTORY).map((c) => (
            <button key={c} type="button" onClick={() => setCostCase(c)}
              className={costCase === c ? 'px-2.5 py-1 text-[11px] bg-emerald-600 text-white rounded-lg' : 'px-2.5 py-1 text-[11px] bg-slate-100 text-slate-600 rounded-lg hover:bg-slate-200'}>{c}</button>
          ))}
        </div>
        {(() => {
          const pts = CASE_COST_HISTORY[costCase] ?? [];
          const last = pts[pts.length - 1];
          const prev = pts[pts.length - 2];
          const delta = prev ? Math.round((((last?.cost ?? 0) - prev.cost) / prev.cost) * 1000) / 10 : null;
          const max = Math.max(...pts.map((p) => p.cost), 1);
          const W = 520, H = 130, PAD = 8;
          const x = (i: number) => PAD + (i * (W - PAD * 2)) / (pts.length - 1);
          const y = (v: number) => PAD + (1 - v / max) * (H - PAD * 2);
          return (
            <div className="grid grid-cols-3 gap-5">
              <div className="space-y-3">
                {[
                  ['本次成本', `¥${last?.cost}`, `${last?.run} · ${last?.kind}`],
                  ['上次成本', `¥${prev?.cost ?? '—'}`, prev?.run ?? ''],
                  ['环比', delta === null ? '—' : `${delta > 0 ? '+' : ''}${delta}%`, delta === null ? '' : delta > 0 ? '本次触发新增分析' : delta < 0 ? '存量回放降本' : '持平'],
                ].map(([lab, val, sub]) => (
                  <div key={lab} className="rounded-lg border border-slate-200 p-3">
                    <div className="text-[11px] text-slate-500">{lab}</div>
                    <div className={'text-lg font-bold ' + (lab === '环比' && typeof delta === 'number' ? (delta > 0 ? 'text-amber-600' : delta < 0 ? 'text-emerald-600' : 'text-slate-700') : 'text-slate-800')}>{val}</div>
                    <div className="text-[10px] text-slate-400">{sub}</div>
                  </div>
                ))}
                <div className="rounded-lg border border-slate-200 p-3">
                  <div className="text-[11px] text-slate-500">本次结果</div>
                  <div><span className={'px-2 py-0.5 rounded-full text-[10px] ' + (last?.result === '通过' ? 'bg-emerald-50 text-emerald-600' : last?.result === '失败' ? 'bg-red-50 text-red-600' : 'bg-amber-50 text-amber-600')}>{last?.result}</span></div>
                  <div className="text-[10px] text-slate-400 mt-1">{last?.kind} · {last?.ts}</div>
                </div>
              </div>
              <div className="col-span-2">
                <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-40">
                  <path d={pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(p.cost).toFixed(1)}`).join(' ')} fill="none" stroke="#6366f1" strokeWidth="2" />
                  {pts.map((p, i) => (
                    <g key={i}>
                      <circle cx={x(i)} cy={y(p.cost)} r="3.5" fill={p.result === '失败' ? '#ef4444' : '#10b981'} stroke="white" strokeWidth="1.5" />
                      <text x={x(i)} y={y(p.cost) - 7} textAnchor="middle" fontSize="9" fill="#475569">¥{p.cost}</text>
                      <text x={x(i)} y={H - 3} textAnchor="middle" fontSize="9" fill="#94a3b8">{p.run.replace('RUN-', '')}</text>
                    </g>
                  ))}
                </svg>
                <p className="text-[10px] text-slate-400 mt-1">纵轴为该用例单次执行成本（¥）· 红点 = 失败执行 · 存量回放持平或递减，失败触发「新增分析」成本回升</p>
              </div>
            </div>
          );
        })()}
      </Card>    </div>
  );
}

import { AUDIT_LOGS, COST_GEN_DETAIL, COST_EXEC_DETAIL, COST_SCENARIOS, COST_TOTAL, COST_TOTAL_TOKENS_IN, COST_TOTAL_TOKENS_OUT, COST_AICALLS, COST_MOM_CHANGE, COST_PER_CASE, COST_TREND, COST_EXEC_LEGACY_TOTAL, COST_EXEC_NEW_TOTAL, AI_MODELS } from '@/data/mock';
import { PageHeader, GhostButton, Card } from '@/components/shared';
import { ShieldCheck, Wallet, TrendingDown, Cpu, Download, ArrowDownRight, ArrowUpRight, Layers } from 'lucide-react';

const TYPE_BADGE: Record<string, string> = {
  '门禁阻断': 'text-red-600 bg-red-50',
  '契约告警': 'text-red-600 bg-red-50',
  '篡改检测': 'text-amber-600 bg-amber-50',
  '种子生成': 'text-emerald-600 bg-emerald-50',
  '冲突检测': 'text-red-600 bg-red-50',
  '执行通过': 'text-emerald-600 bg-emerald-50',
};

const SCEN_COLOR: Record<string, string> = {
  '用例生成': 'bg-emerald-500',
  '测试执行辅助': 'bg-teal-500',
  '质量判定': 'bg-indigo-500',
  '契约分析': 'bg-amber-500',
  '变异/篡改检测': 'bg-slate-400',
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

export default function AuditPage() {
  const genByModel = (d: (typeof COST_GEN_DETAIL)[0]) => AI_MODELS.find((m) => m.id === d.modelId)?.name ?? d.modelId;
  const maxScen = Math.max(...COST_SCENARIOS.map((s) => s.amount));
  const fmtM = (v: number) => { const m = v / 1e6; return (Number.isInteger(m) ? m.toFixed(0) : m.toFixed(1)) + 'M'; };

  return (
    <div>
      <PageHeader title="审计日志" desc="哈希链 · 仅追加 · 可独立验证 · 跨资产审计 · 成本审计">
        <GhostButton><span className="flex items-center gap-1"><ShieldCheck className="w-3.5 h-3.5" />验证链完整性</span></GhostButton>
        <GhostButton><span className="flex items-center gap-1"><Download className="w-3.5 h-3.5" />导出审计报告</span></GhostButton>
      </PageHeader>

      <div className="card bg-white rounded-xl border border-slate-200 p-5 mb-5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className="w-10 h-10 rounded-full bg-emerald-100 flex items-center justify-center text-emerald-600 text-lg">✓</div>
            <div>
              <div className="text-sm font-semibold text-slate-700">哈希链完整性验证通过</div>
              <div className="text-xs text-slate-500 mt-0.5">共 12,847 条日志 · 最后锚定 2026-09-24 10:30:00</div>
            </div>
          </div>
          <div className="text-right">
            <div className="font-mono text-slate-500 text-[11px]">链头哈希</div>
            <div className="font-mono text-slate-700 font-medium">sha256:0a1f9c3e…d84b</div>
            <div className="font-mono text-emerald-600 text-[10px] mt-0.5">外部锚定: WORM bucket ✓</div>
          </div>
        </div>
      </div>

      {/* ==================== 成本审计 ==================== */}
      <Card title={<span className="flex items-center gap-1.5"><Wallet className="w-4 h-4 text-emerald-500" />成本审计 · AI 场景成本总览与明细</span>}
        extra={<span className="text-[11px] text-slate-400">统计周期：近 30 天 · 金额 = 单价表 × token 量（输入+输出）</span>}>
        {/* 总览 KPI */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-5">
          <div className="card bg-white rounded-xl border border-slate-200 p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs text-slate-500">AI 总成本（¥）</span><Wallet className="w-4 h-4 text-emerald-500" />
            </div>
            <div className="text-2xl font-bold text-slate-800">{COST_TOTAL.toLocaleString()}</div>
            <div className="mt-1 flex items-center gap-1 text-[11px] text-emerald-600"><TrendingDown className="w-3 h-3" />环比 {Math.abs(COST_MOM_CHANGE)}%（治理降本）</div>
          </div>
          <div className="card bg-white rounded-xl border border-slate-200 p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs text-slate-500">Token 总量</span><Layers className="w-4 h-4 text-indigo-500" />
            </div>
            <div className="text-xl font-bold text-slate-800">{(COST_TOTAL_TOKENS_IN / 1e6).toFixed(0)}M<span className="text-sm font-normal text-slate-400"> 入</span> / {(COST_TOTAL_TOKENS_OUT / 1e6).toFixed(0)}M<span className="text-sm font-normal text-slate-400"> 出</span></div>
            <div className="mt-1 text-[11px] text-slate-400">输入输出分别计费</div>
          </div>
          <div className="card bg-white rounded-xl border border-slate-200 p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs text-slate-500">AI 调用次数</span><Cpu className="w-4 h-4 text-amber-500" />
            </div>
            <div className="text-2xl font-bold text-slate-800">{COST_AICALLS.toLocaleString()}</div>
            <div className="mt-1 text-[11px] text-slate-400">用例生成 · 判定 · 契约 · 执行</div>
          </div>
          <div className="card bg-white rounded-xl border border-slate-200 p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs text-slate-500">平均每用例成本</span><ArrowDownRight className="w-4 h-4 text-emerald-500" />
            </div>
            <div className="text-2xl font-bold text-slate-800">¥{COST_PER_CASE}</div>
            <div className="mt-1 text-[11px] text-slate-400">存量用例成本趋稳</div>
          </div>
        </div>

        <div className="grid grid-cols-3 gap-5 mb-5">
          {/* 成本趋势 */}
          <div className="col-span-2 card bg-white rounded-xl border border-slate-200 p-5">
            <h3 className="font-semibold text-slate-700 text-sm mb-3">成本趋势（近 14 天，¥）</h3>
            <TrendChart />
          </div>
          {/* 场景成本占比 */}
          <div className="card bg-white rounded-xl border border-slate-200 p-5">
            <h3 className="font-semibold text-slate-700 text-sm mb-3">场景成本占比</h3>
            <div className="space-y-3">
              {COST_SCENARIOS.map((s) => (
                <div key={s.scenario}>
                  <div className="flex justify-between text-xs mb-1">
                    <span className="text-slate-600">{s.scenario}</span>
                    <span className="text-slate-500">¥{s.amount.toLocaleString()} · {Math.round((s.amount / COST_TOTAL) * 100)}%</span>
                  </div>
                  <div className="h-2 bg-slate-100 rounded-full">
                    <div className={`h-full rounded-full ${SCEN_COLOR[s.scenario] ?? 'bg-slate-400'}`} style={{ width: `${(s.amount / maxScen) * 100}%` }} />
                  </div>
                  <div className="text-[10px] text-slate-400 mt-0.5">{s.note}</div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* 业务逻辑说明：存量用例执行成本 */}
        <div className="mb-5 px-4 py-3 rounded-xl bg-emerald-50 border border-emerald-200 flex items-start gap-3">
          <TrendingDown className="w-4 h-4 text-emerald-600 flex-shrink-0 mt-0.5" />
          <div className="text-[11px] text-emerald-800 leading-relaxed">
            <span className="font-semibold">存量用例执行成本逻辑：</span>
            存量用例（已稳定）执行走纯回放 + 断言缓存，AI 不重复生成/重判，因此其成本应<span className="font-semibold">持平或递减</span>；
            新增/变更用例才会引入新的 AI 分析成本。当前存量执行成本 ¥{COST_EXEC_LEGACY_TOTAL.toFixed(0)} 显著低于新增 ¥{COST_EXEC_NEW_TOTAL.toFixed(0)}，且近 14 天存量执行成本从 ¥9.8 降至 ¥5.6 —— 符合治理降本预期。
          </div>
        </div>

        {/* 明细：用例生成成本 */}
        <h3 className="font-semibold text-slate-700 text-sm mb-3 flex items-center gap-1.5"><ArrowUpRight className="w-4 h-4 text-emerald-500" />用例生成成本明细</h3>
        <div className="rounded-xl border border-slate-200 overflow-hidden mb-5">
          <table className="w-full text-xs">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-slate-500">
                <th className="px-4 py-2.5 font-medium">生成批次</th>
                <th className="px-4 py-2.5 font-medium">用例 ID</th>
                <th className="px-4 py-2.5 font-medium">触发源</th>
                <th className="px-4 py-2.5 font-medium">模型</th>
                <th className="px-4 py-2.5 font-medium">Token 入/出</th>
                <th className="px-4 py-2.5 font-medium text-right">成本（¥）</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {COST_GEN_DETAIL.map((d) => (
                <tr key={d.id} className="hover:bg-slate-50">
                  <td className="px-4 py-3 font-mono text-slate-500">{d.id}</td>
                  <td className="px-4 py-3 font-mono text-indigo-600 font-medium">{d.caseId}</td>
                  <td className="px-4 py-3 text-slate-600">{d.source}</td>
                  <td className="px-4 py-3 text-slate-600">{genByModel(d)}</td>
                  <td className="px-4 py-3 font-mono text-slate-400">{fmtM(d.tokensIn)} / {fmtM(d.tokensOut)}</td>
                  <td className="px-4 py-3 text-right font-medium text-slate-700">¥{d.amount}</td>
                </tr>
              ))}
            </tbody>
            <tfoot className="bg-slate-50 border-t border-slate-200">
              <tr>
                <td colSpan={5} className="px-4 py-2.5 text-[11px] text-slate-500 font-medium text-right">小计 · {COST_GEN_DETAIL.length} 条</td>
                <td className="px-4 py-2.5 text-right font-semibold text-emerald-600">¥{COST_GEN_DETAIL.reduce((s, d) => s + d.amount, 0).toLocaleString()}</td>
              </tr>
            </tfoot>
          </table>
        </div>

        {/* 明细：测试用例执行成本 */}
        <h3 className="font-semibold text-slate-700 text-sm mb-3 flex items-center gap-1.5"><ArrowDownRight className="w-4 h-4 text-emerald-500" />测试用例执行成本明细</h3>
        <div className="rounded-xl border border-slate-200 overflow-hidden">
          <table className="w-full text-xs">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-slate-500">
                <th className="px-4 py-2.5 font-medium">用例 ID</th>
                <th className="px-4 py-2.5 font-medium">资产</th>
                <th className="px-4 py-2.5 font-medium">类别</th>
                <th className="px-4 py-2.5 font-medium">模型</th>
                <th className="px-4 py-2.5 font-medium">Token 入/出</th>
                <th className="px-4 py-2.5 font-medium text-right">成本（¥）</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {COST_EXEC_DETAIL.map((d) => (
                <tr key={d.id} className={'hover:bg-slate-50 ' + (d.isLegacy ? '' : 'bg-amber-50/30')}>
                  <td className="px-4 py-3 font-mono text-indigo-600 font-medium">{d.caseId}</td>
                  <td className="px-4 py-3 font-mono text-slate-600">{d.asset}</td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-0.5 rounded-full text-[10px] ${d.isLegacy ? 'bg-emerald-50 text-emerald-600' : 'bg-amber-50 text-amber-600'}`}>
                      {d.isLegacy ? '存量回放' : '新增用例'}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-slate-600">{genByModel(d)}</td>
                  <td className="px-4 py-3 font-mono text-slate-400">{fmtM(d.tokensIn)} / {fmtM(d.tokensOut)}</td>
                  <td className="px-4 py-3 text-right font-medium text-slate-700">¥{d.amount}</td>
                </tr>
              ))}
            </tbody>
            <tfoot className="bg-slate-50 border-t border-slate-200">
              <tr>
                <td colSpan={5} className="px-4 py-2.5 text-[11px] text-slate-500 font-medium text-right">
                  小计 ¥{COST_EXEC_DETAIL.reduce((s, d) => s + d.amount, 0).toFixed(1)} · 其中存量 ¥{COST_EXEC_LEGACY_TOTAL.toFixed(1)} / 新增 ¥{COST_EXEC_NEW_TOTAL.toFixed(1)}
                </td>
                <td className="px-4 py-2.5 text-right font-semibold text-emerald-600">¥{COST_EXEC_DETAIL.reduce((s, d) => s + d.amount, 0).toFixed(1)}</td>
              </tr>
            </tfoot>
          </table>
        </div>
      </Card>

      <div className="mt-5">
        <Card title="日志流"
          extra={
            <select className="border border-slate-300 rounded-lg px-2 py-1 outline-none text-xs">
              <option>全部资产</option>
              <option>svc-auth</option>
              <option>svc-payment</option>
            </select>
          }>
          <div className="divide-y divide-slate-100">
            {AUDIT_LOGS.map((log) => (
              <div key={log.seq} className="px-5 py-3.5 hover:bg-slate-50">
                <div className="flex items-center gap-4">
                  <span className="font-mono text-slate-400 w-12 text-xs">{log.seq}</span>
                  <span className="text-[11px] text-slate-400 w-36">{log.time}</span>
                  <span className={'text-xs font-medium px-2 py-0.5 rounded-full ' + TYPE_BADGE[log.type]}>{log.type}</span>
                  <span className="text-xs text-slate-700 flex-1">{log.message}</span>
                  <span className="font-mono text-slate-400 text-xs">{log.hash}</span>
                </div>
                <div className="ml-16 mt-2 flex gap-4 text-[11px] text-slate-500">
                  <span>actor: <span className="text-slate-600">{log.actor}</span></span>
                  <span>asset: <span className="text-slate-600 font-mono">{log.asset}</span></span>
                  <span>prev: <span className="font-mono text-slate-400">sha256:9c3e…</span></span>
                </div>
              </div>
            ))}
          </div>
        </Card>
      </div>
    </div>
  );
}

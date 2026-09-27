import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { TEST_RUNS, TEST_SCENARIOS, SCENARIO_GROUPS, type ITestRun } from '@/data/mock';
import { scenarioNav } from '@/context/scenarioNav';
import { PageHeader, Card } from '@/components/shared';
import { Cpu, Globe, Smartphone, Sparkles, Download, CheckCircle2, ShieldCheck, TrendingUp, CornerDownRight } from 'lucide-react';

const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles };
const EXPORT_FORMATS = [
  { f: '.html', l: 'HTML' }, { f: '.pdf', l: 'PDF' }, { f: '.docx', l: 'Word' }, { f: '.md', l: 'Markdown' },
];

export default function ReportPage() {
  const [runId, setRunId] = useState('RUN-4821');
  const navigate = useNavigate();
  const [toast, setToast] = useState('');
  const run = TEST_RUNS.find((r) => r.id === runId) as ITestRun;

  // ===== 跨场景汇总（从场景历史派生）=====
  const scen = TEST_SCENARIOS;
  const passRate = Math.round(scen.reduce((a, s) => a + s.history[s.history.length - 1].p, 0) / scen.length);
  const totalCost = scen.reduce((a, s) => a + s.history.reduce((x, h) => x + h.c, 0), 0);
  const highRisk = scen.filter((s) => s.history[s.history.length - 1].p < 90);
  const passCount = scen.filter((s) => s.history[s.history.length - 1].p >= 90).length;

  const onExport = (name: string) => { setToast(name); window.setTimeout(() => setToast(''), 2600); };

  return (
    <div>
      <PageHeader title="测试报告" desc="测试闭环最后一环 · 跨场景汇总合编 · 工程报告 = 多场景报告合编（可逐场景分块 / 整份合编导出）">
        <span className="text-[11px] text-slate-400">按工程筛选：sys-payment-platform · {run.gate === '通过' ? '门禁通过' : '门禁阻断'}</span>
      </PageHeader>

      <Card title="选择运行 · 生成报告" extra={<span className="text-[11px] text-slate-400">点击运行生成对应跨场景合编报告</span>} className="mb-5 p-5">
        <div className="flex flex-wrap gap-2">
          {TEST_RUNS.map((r) => (
            <button key={r.id} type="button" onClick={() => setRunId(r.id)}
              className={runId === r.id ? 'px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg' : 'px-3 py-1.5 text-xs bg-slate-100 text-slate-600 rounded-lg hover:bg-slate-200'}>
              <span className="font-mono">{r.id}</span>
              <span className="ml-2">{r.gate === '通过' ? '✓' : '✕'}{Math.round((r.pass / r.total) * 1000) / 10}%</span>
            </button>
          ))}
        </div>
        <p className="mt-3 text-[10px] text-slate-400">存量用例回放成本持平/递减；本次失败触发「新增分析」成本回升 —— 每次 CI 触发产出一份可追溯、可独立验证、跨场景合编的测试报告。</p>
      </Card>

      {/* 跨场景总览 */}
      <Card title="跨场景汇总总览" extra={<span className="text-[11px] text-slate-400">将全部测试场景报告汇总合编为工程级报告</span>} className="p-5 mb-5">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
          <div className="rounded-xl border border-slate-200 p-4">
            <div className="text-[11px] text-slate-400 flex items-center gap-1"><TrendingUp className="w-3 h-3" />汇总通过率</div>
            <div className="text-2xl font-bold text-slate-800">{passRate}%</div>
          </div>
          <div className="rounded-xl border border-slate-200 p-4">
            <div className="text-[11px] text-slate-400">覆盖场景</div>
            <div className="text-2xl font-bold text-slate-800">{scen.length}<span className="text-sm font-normal text-slate-400"> 个</span></div>
          </div>
          <div className="rounded-xl border border-slate-200 p-4">
            <div className="text-[11px] text-slate-400">汇总成本</div>
            <div className="text-2xl font-bold text-slate-800">¥{totalCost.toLocaleString()}</div>
          </div>
          <div className={'rounded-xl border p-4 ' + (highRisk.length ? 'border-red-200 bg-red-50/40' : 'border-emerald-200 bg-emerald-50/40')}>
            <div className="text-[11px] text-slate-500">高风险场景</div>
            <div className={'text-2xl font-bold ' + (highRisk.length ? 'text-red-500' : 'text-emerald-600')}>{highRisk.length} <span className="text-sm font-normal text-slate-400">/ {scen.length}</span></div>
          </div>
        </div>
        {highRisk.length > 0 && (
          <div className="text-xs text-slate-600 bg-amber-50 border border-amber-100 rounded-lg px-3 py-2 mb-3">
            存在 <span className="font-medium text-amber-700">{highRisk.map((h) => h.name).join('、')}</span> 场景通过率低于 90%，需重点核查后再放行。
          </div>
        )}
        <div className="text-xs text-slate-600 bg-emerald-50/50 border border-emerald-100 rounded-lg px-3 py-2 mb-3">
          <span className="font-medium text-emerald-700">合编结论：</span>{passCount}/{scen.length} 场景通过门禁，存量用例成本持平/递减、失败触发新增分析回升，整体测试质量受控。
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-[11px] text-slate-400">整份合编导出：</span>
          {EXPORT_FORMATS.map((f) => (
            <button key={f.f} type="button" onClick={() => onExport(`RUN-4821-合编${f.f}`)}
              className="flex items-center gap-1 px-2.5 py-1.5 text-xs bg-slate-100 text-slate-600 rounded-lg hover:bg-emerald-50 hover:text-emerald-600">
              <Download className="w-3 h-3" />{f.l} 合编报告
            </button>
          ))}
        </div>
      </Card>

      {/* 场景分块合编 */}
      <Card title="场景分块 · 汇总合编" extra={<span className="text-[11px] text-slate-400">工程报告按测试场景分块 · 每块可独立查看与导出</span>} className="p-5">
        <div className="space-y-5">
          {SCENARIO_GROUPS.map((g) => {
            const GIcon = SCEN_ICON[g.scenarios[0]?.icon ?? 'Cpu'];
            return (
              <div key={g.label}>
                <div className="flex items-center gap-2 mb-2">
                  <GIcon className="w-4 h-4 text-emerald-600" />
                  <span className="font-medium text-sm text-slate-700">{g.label}</span>
                  <span className="text-[11px] text-slate-400">· {g.resource}</span>
                </div>
                <div className="grid grid-cols-1 xl:grid-cols-3 gap-3">
                  {g.scenarios.map((s) => {
                    const SIcon = SCEN_ICON[s.icon];
                    const lp = s.history[s.history.length - 1].p;
                    const scost = s.history.reduce((a, h) => a + h.c, 0);
                    const blocked = lp < 90;
                    return (
                      <div key={s.id} className={'border rounded-xl p-4 ' + (blocked ? 'border-red-200' : 'border-slate-200')}>
                        <div className="flex items-center justify-between mb-2">
                          <div className="flex items-center gap-2">
                            <SIcon className="w-4 h-4 text-emerald-600" />
                            <span className="text-sm font-medium text-slate-800">{s.name}</span>
                            <span className="text-[10px] text-slate-400 font-mono">{s.id}</span>
                          </div>
                          <span className={'text-[10px] px-1.5 py-0.5 rounded ' + (blocked ? 'bg-red-50 text-red-600' : 'bg-emerald-50 text-emerald-600')}>{lp >= 90 ? '通过' : '阻断'}</span>
                        </div>
                        <div className="grid grid-cols-3 gap-2 text-center mb-3">
                          <div><div className="text-[10px] text-slate-400">通过率</div><div className={'text-sm font-bold ' + (blocked ? 'text-red-600' : 'text-emerald-600')}>{lp}%</div></div>
                          <div><div className="text-[10px] text-slate-400">执行</div><div className="text-sm font-bold text-slate-700">{s.history.length}次</div></div>
                          <div><div className="text-[10px] text-slate-400">成本</div><div className="text-sm font-bold text-slate-700">¥{scost.toLocaleString()}</div></div>
                        </div>
                        <div className="text-[10px] text-slate-500 bg-slate-50 rounded-lg px-2 py-1.5 mb-2 flex items-center gap-1">
                          <ShieldCheck className="w-3 h-3 text-emerald-500" />{s.gateRule}
                        </div>
                        <div className="flex items-center justify-between gap-2">
                          <div className="flex items-center gap-2">
                            <button type="button" onClick={() => onExport(`RUN-4821-${s.id}${EXPORT_FORMATS[1].f}`)}
                              className="flex items-center gap-1 px-2 py-1 text-[10px] bg-slate-100 text-slate-600 rounded-lg hover:bg-emerald-50 hover:text-emerald-600">
                              <Download className="w-3 h-3" />单独导出
                            </button>
                            <button type="button" onClick={() => { scenarioNav.go(s.id, 'report'); navigate('/scenarios'); }}
                              className="flex items-center gap-1 px-2 py-1 text-[10px] bg-slate-100 text-slate-600 rounded-lg hover:bg-emerald-50 hover:text-emerald-600">
                              <CornerDownRight className="w-3 h-3" />进入场景闭环
                            </button>
                          </div>
                          <span className="text-[10px] text-slate-400">近5次 {s.history.map((h) => `${h.p}%`).join(' → ')}</span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </div>
      </Card>

      {toast && (
        <div className="fixed bottom-6 right-6 z-50 px-4 py-3 rounded-xl bg-slate-800 text-white text-xs shadow-xl flex items-center gap-2">
          <CheckCircle2 className="w-4 h-4 text-emerald-400" />已导出报告为 <span className="font-mono text-emerald-300">{toast}</span>（原型模拟下载）
        </div>
      )}
    </div>
  );
}

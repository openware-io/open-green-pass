import { useState } from 'react';
import { TEST_SCENARIOS, SCENARIO_GROUPS, SCEN_STATUS, type ITestScenario, type ScenStatus } from '@/data/mock';
import { PageHeader, Card } from '@/components/shared';
import { Cpu, Globe, Smartphone, Sparkles, ListChecks, Play, ShieldCheck, History, FileText, CheckCircle2, Download, CircleCheckBig, ArrowLeft } from 'lucide-react';
import { toast } from 'sonner';

const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles };
const FORM_BADGE: Record<string, string> = { '服务/后端': 'bg-emerald-50 text-emerald-600', 'Web 前端': 'bg-sky-50 text-sky-600', '移动端': 'bg-purple-50 text-purple-600', 'AI 专项': 'bg-indigo-50 text-indigo-600' };
const RESULT_CLS: Record<string, string> = { '通过': 'text-emerald-600', '失败': 'text-red-600', '阻塞': 'text-amber-600' };
const STATUS_META: Record<ScenStatus, { label: string; cls: string; dot: string }> = {
  idle: { label: '空闲', cls: 'bg-slate-100 text-slate-500', dot: 'bg-slate-300' },
  running: { label: '执行中', cls: 'bg-emerald-50 text-emerald-600', dot: 'animate-pulse bg-emerald-500' },
  queued: { label: '排队中', cls: 'bg-amber-50 text-amber-600', dot: 'bg-amber-400' },
};

const TABS = [
  { key: 'cases', label: '场景用例', icon: ListChecks },
  { key: 'exec', label: '场景执行', icon: Play },
  { key: 'gate', label: '场景门禁', icon: ShieldCheck },
  { key: 'history', label: '执行历史', icon: History },
  { key: 'report', label: '场景报告', icon: FileText },
] as const;
type TabKey = (typeof TABS)[number]['key'];

export default function ScenarioCenterPage() {
  const [selId, setSelId] = useState('SCEN-01');
  const [tab, setTab] = useState<TabKey>('cases');
  const [focused, setFocused] = useState(false);
  const sel = TEST_SCENARIOS.find((s) => s.id === selId) as ITestScenario;
  const selStatus = SCEN_STATUS[sel.id] ?? 'idle';
  const SIcon = SCEN_ICON[sel.icon];

  // ===== 从单一 mock 源派生场景闭环数据（用例/执行/门禁/历史/报告）=====
  const selCases = sel.cases.map((cid, i) => ({
    id: cid, name: `${sel.name} · 用例${i + 1}`,
    result: (i % 3 === 0 && sel.id !== 'SCEN-05' && sel.id !== 'SCEN-03' ? '失败' : '通过') as '通过' | '失败',
    cost: 60 + i * 20,
  }));
  const execs = sel.history.map((h, i) => ({
    run: `RUN-482${i + 1}`, ts: h.d,
    result: h.p >= 95 ? '通过' : h.p >= 85 ? '失败' : '阻塞' as string,
    pass: h.p, cost: h.c, resource: sel.resource,
  }));
  const latest = sel.history[sel.history.length - 1];
  const totalCost = sel.history.reduce((a, b) => a + b.c, 0);
  const gateItems = [
    { item: '通过率', threshold: sel.id === 'SCEN-05' ? '≥100%' : sel.id === 'SCEN-08' ? '≥100%' : '≥90%', current: `${latest.p}%`, status: latest.p >= 90 ? '通过' : '阻断' },
    { item: '门禁阈值', threshold: sel.gateRule.split('且')[1]?.trim() ?? sel.gateRule, current: sel.gate, status: sel.gate === '通过' ? '通过' : '阻断' },
  ] as { item: string; threshold: string; current: string; status: '通过' | '阻断' }[];
  const report = {
    passRate: latest.p, execCount: sel.history.length, totalCost,
    risk: latest.p < 90 ? `近次通过率降至 ${latest.p}%，存在质量回落风险` : '当前处于质量稳定区间',
    conclusion: latest.p >= 90 ? `${sel.name} 场景通过门禁，可放行。存量用例成本持平/递减，符合预期。` : `${sel.name} 场景触发门禁阻断，需排查近次失败用例并新增分析。`,
  };

  const onExport = (scope: string) => toast.success(`已导出 ${sel.id}-${scope} 报告（原型模拟下载）`);

  return (
    <div>
      <PageHeader title="测试场景中心" desc="测试质量管控的核心 · 12 场景 × 完整闭环（用例 / 执行 / 门禁 / 历史 / 报告）">
        <div className="flex items-center gap-3 text-xs text-slate-500">
          <span className="flex items-center gap-1.5"><CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" />12 场景全部启用</span>
          <span className="flex items-center gap-1.5"><CircleCheckBig className="w-3.5 h-3.5 text-emerald-500" />每场景一条独立闭环</span>
        </div>
      </PageHeader>

      {/* 场景 × 闭环 矩阵 */}
      <Card title="测试场景 · 闭环矩阵" extra={<span className="text-[11px] text-slate-400">{focused ? '单卡片聚焦模式 · 点击「返回全部场景」回到矩阵' : '点击卡片进入单卡片聚焦 · 绿色脉冲圆点表示该场景正在执行'}</span>} className="p-5 mb-5">
        {focused ? (
          <div className="animate-gp-scale-in border-2 border-emerald-300 rounded-2xl p-6 bg-white shadow-xl">
            <button type="button" onClick={() => setFocused(false)}
              className="flex items-center gap-1 text-xs text-slate-400 hover:text-emerald-600 mb-4 transition-colors">
              <ArrowLeft className="w-3.5 h-3.5" />返回全部场景
            </button>
            <div className="flex items-center gap-4 mb-4">
              <span className="w-14 h-14 rounded-2xl bg-emerald-500 text-white flex items-center justify-center shadow-md"><SIcon className="w-7 h-7" /></span>
              <div>
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-xl font-bold text-slate-800">{sel.name}</span>
                  <span className="text-[10px] text-slate-400 font-mono">{sel.id}</span>
                  <span className={'text-[10px] px-1.5 py-0.5 rounded ' + FORM_BADGE[sel.form]}>{sel.form}</span>
                  {selStatus !== 'idle' && (
                    <span className={'flex items-center gap-1.5 text-[10px] px-2 py-0.5 rounded ' + STATUS_META[selStatus].cls}>
                      <span className={'w-1.5 h-1.5 rounded-full ' + STATUS_META[selStatus].dot} />{STATUS_META[selStatus].label}
                    </span>
                  )}
                </div>
                <p className="text-[11px] text-slate-500 mt-1">{sel.resource} · {sel.tool} · {sel.trigger}</p>
              </div>
            </div>
            <p className="text-sm text-slate-600 leading-relaxed mb-4">{sel.how}</p>
            <div className="flex flex-wrap gap-2 mb-4">
              <span className="px-2.5 py-1 rounded-lg bg-slate-100 text-slate-600 text-xs">{sel.resource}</span>
              <span className="px-2.5 py-1 rounded-lg bg-slate-100 text-slate-600 text-xs">{sel.tool}</span>
              <span className="px-2.5 py-1 rounded-lg bg-slate-100 text-slate-600 text-xs">{sel.trigger}</span>
              <span className="px-2.5 py-1 rounded-lg bg-emerald-50 text-emerald-600 text-xs">{sel.gate}</span>
            </div>
            <div className="flex items-center gap-2 bg-emerald-50/60 border border-emerald-100 rounded-xl px-3 py-2 text-xs text-slate-600 mb-4">
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-500" />门禁规则：<span className="text-emerald-700">{sel.gateRule}</span>
            </div>
            <div className="flex items-center gap-2 flex-wrap">
              <span className="text-[11px] text-slate-400 mr-1">聚焦本场景闭环：</span>
              {TABS.map((t) => (
                <button key={t.key} type="button" onClick={() => setTab(t.key)}
                  className={'flex items-center gap-1 px-2.5 py-1.5 text-xs rounded-lg transition-colors ' + (tab === t.key ? 'bg-emerald-600 text-white' : 'bg-slate-100 text-slate-600 hover:bg-emerald-50 hover:text-emerald-600')}>
                  <t.icon className="w-3 h-3" />{t.label}
                </button>
              ))}
            </div>
          </div>
        ) : (
          <div className="space-y-5">
            {SCENARIO_GROUPS.map((g) => {
              const GIcon = SCEN_ICON[g.scenarios[0]?.icon ?? 'Cpu'];
              return (
                <div key={g.label}>
                  <div className="flex items-center gap-2 mb-2.5">
                    <GIcon className="w-4 h-4 text-emerald-600" />
                    <span className="font-medium text-sm text-slate-700">{g.label}</span>
                    <span className="text-[11px] text-slate-400">· 依赖 {g.resource}</span>
                    <span className="text-[10px] text-slate-400 ml-auto">{g.scenarios.length} 场景</span>
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3">
                    {g.scenarios.map((s) => {
                      const SIcon = SCEN_ICON[s.icon];
                      const st = SCEN_STATUS[s.id] ?? 'idle';
                      const active = s.id === selId;
                      return (
                        <div key={s.id} onClick={() => { setSelId(s.id); setTab('cases'); setFocused(true); }}
                          className={'border rounded-xl p-4 cursor-pointer transition-all duration-200 hover:-translate-y-0.5 hover:shadow-md ' + (active ? 'border-emerald-400 ring-1 ring-emerald-200 bg-emerald-50/30' : 'border-slate-200 hover:border-emerald-300')}>
                          <div className="flex items-center justify-between mb-1.5">
                            <div className="flex items-center gap-2">
                              <span className={'w-7 h-7 rounded-lg flex items-center justify-center ' + (active ? 'bg-emerald-500 text-white' : 'bg-slate-100 text-slate-500')}>
                                <SIcon className="w-4 h-4" />
                              </span>
                              <div>
                                <div className="text-sm font-medium text-slate-800 flex items-center gap-1.5">{s.name}<span className="text-[10px] text-slate-400 font-mono">{s.id}</span></div>
                                <div className="text-[10px] text-slate-400">通过率 {s.history[s.history.length - 1].p}% · 近 5 次</div>
                              </div>
                            </div>
                            <div className="flex flex-col items-end gap-1">
                              {st !== 'idle' && (
                                <span className={'flex items-center gap-1.5 text-[10px] px-1.5 py-0.5 rounded ' + STATUS_META[st].cls}>
                                  <span className={'w-1.5 h-1.5 rounded-full ' + STATUS_META[st].dot} />{STATUS_META[st].label}
                                </span>
                              )}
                              <span className={'text-[10px] px-1.5 py-0.5 rounded ' + FORM_BADGE[s.form]}>{s.form}</span>
                            </div>
                          </div>
                          <p className="text-[11px] text-slate-500 leading-relaxed line-clamp-2">{s.how}</p>
                          <div className="mt-2.5 flex flex-wrap gap-1.5 text-[10px]">
                            {TABS.map((t) => (
                              <button key={t.key} type="button" onClick={(e) => { e.stopPropagation(); setSelId(s.id); setTab(t.key); setFocused(true); }}
                                className="flex items-center gap-1 px-1.5 py-0.5 rounded bg-slate-100 text-slate-600 hover:bg-emerald-50 hover:text-emerald-600">
                                <t.icon className="w-3 h-3" />{t.label.replace('场景', '')}
                              </button>
                            ))}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </Card>

      {/* 场景详情 · 五 tab 闭环 */}
      <Card title={<span className="flex items-center gap-2"><SIcon className="w-4 h-4 text-emerald-600" />{sel.name}<span className="text-[10px] text-slate-400 font-mono">{sel.id}</span><span className={'text-[10px] px-1.5 py-0.5 rounded ' + FORM_BADGE[sel.form]}>{sel.form}</span></span>}
        extra={<span className="text-[11px] text-slate-400">{sel.resource} · {sel.tool}</span>} className="p-5">
        <div className="flex items-center gap-1 border-b border-slate-200 pb-3 mb-4">
          {TABS.map((t) => (
            <button key={t.key} type="button" onClick={() => setTab(t.key)}
              className={'flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-t-lg border-b-2 ' + (tab === t.key ? 'text-emerald-600 border-emerald-500 font-medium' : 'text-slate-500 border-transparent hover:text-slate-700')}>
              <t.icon className="w-3.5 h-3.5" />{t.label}
            </button>
          ))}
        </div>

        {tab === 'cases' && (
          <div>
            <p className="text-[11px] text-slate-400 mb-3">{sel.how}</p>
            <table className="w-full text-xs">
              <thead><tr className="text-left text-slate-400 border-b border-slate-100"><th className="py-1.5 font-medium">用例</th><th className="py-1.5 font-medium">名称</th><th className="py-1.5 font-medium">结果</th><th className="py-1.5 font-medium">成本</th></tr></thead>
              <tbody>{selCases.map((c) => (
                <tr key={c.id} className="border-b border-slate-50">
                  <td className="py-2 font-mono text-slate-600">{c.id}</td>
                  <td className="py-2 text-slate-700">{c.name}</td>
                  <td className="py-2"><span className={RESULT_CLS[c.result]}>{c.result}</span></td>
                  <td className="py-2 text-slate-600">¥{c.cost}</td>
                </tr>))}</tbody>
            </table>
          </div>
        )}

        {tab === 'exec' && (
          <table className="w-full text-xs">
            <thead><tr className="text-left text-slate-400 border-b border-slate-100"><th className="py-1.5 font-medium">运行</th><th className="py-1.5 font-medium">日期</th><th className="py-1.5 font-medium">结果</th><th className="py-1.5 font-medium">通过率</th><th className="py-1.5 font-medium">资源</th><th className="py-1.5 font-medium">成本</th></tr></thead>
            <tbody>{execs.map((e) => (
              <tr key={e.run} className="border-b border-slate-50">
                <td className="py-2 font-mono text-slate-600">{e.run}</td>
                <td className="py-2 text-slate-600">{e.ts}</td>
                <td className="py-2"><span className={RESULT_CLS[e.result]}>{e.result}</span></td>
                <td className="py-2 text-slate-700">{e.pass}%</td>
                <td className="py-2 text-slate-600">{e.resource}</td>
                <td className="py-2 text-slate-600">¥{e.cost.toLocaleString()}</td>
              </tr>))}</tbody>
          </table>
        )}

        {tab === 'gate' && (
          <div>
            <div className="flex items-center gap-2 mb-3"><ShieldCheck className="w-4 h-4 text-emerald-500" /><span className="text-sm font-medium text-slate-700">门禁规则</span></div>
            <div className="mb-3 text-xs text-slate-600 bg-slate-50 rounded-lg px-3 py-2 border border-slate-100">{sel.gateRule}</div>
            <table className="w-full text-xs">
              <thead><tr className="text-left text-slate-400 border-b border-slate-100"><th className="py-1.5 font-medium">判定项</th><th className="py-1.5 font-medium">阈值</th><th className="py-1.5 font-medium">当前</th><th className="py-1.5 font-medium">状态</th></tr></thead>
              <tbody>{gateItems.map((g) => (
                <tr key={g.item} className="border-b border-slate-50">
                  <td className="py-2 text-slate-700">{g.item}</td>
                  <td className="py-2 text-slate-600">{g.threshold}</td>
                  <td className="py-2 text-slate-700">{g.current}</td>
                  <td className="py-2"><span className={'px-1.5 py-0.5 rounded text-[10px] ' + (g.status === '通过' ? 'bg-emerald-50 text-emerald-600' : 'bg-red-50 text-red-600')}>{g.status}</span></td>
                </tr>))}</tbody>
            </table>
          </div>
        )}

        {tab === 'history' && (
          <table className="w-full text-xs">
            <thead><tr className="text-left text-slate-400 border-b border-slate-100"><th className="py-1.5 font-medium">日期</th><th className="py-1.5 font-medium">通过率</th><th className="py-1.5 font-medium">成本</th><th className="py-1.5 font-medium">环比</th></tr></thead>
            <tbody>{sel.history.map((h, i) => {
              const prev = i > 0 ? sel.history[i - 1].c : h.c;
              const diff = h.c - prev;
              return (
                <tr key={h.d} className="border-b border-slate-50">
                  <td className="py-2 text-slate-600">{h.d}</td>
                  <td className="py-2"><span className={h.p >= 95 ? 'text-emerald-600' : h.p >= 90 ? 'text-amber-600' : 'text-red-600'}>{h.p}%</span></td>
                  <td className="py-2 text-slate-600">¥{h.c.toLocaleString()}</td>
                  <td className="py-2 text-slate-500">{diff === 0 ? '持平' : diff < 0 ? `↓ ¥${Math.abs(diff).toLocaleString()}` : `↑ ¥${diff.toLocaleString()}`}</td>
                </tr>
              );
            })}</tbody>
          </table>
        )}

        {tab === 'report' && (
          <div>
            <div className="grid grid-cols-4 gap-3 mb-4">
              {[{ l: '通过率', v: `${report.passRate}%` }, { l: '执行次数', v: report.execCount }, { l: '总成本', v: `¥${report.totalCost.toLocaleString()}` }, { l: '门禁状态', v: latest.p >= 90 ? '通过' : '阻断' }].map((k) => (
                <div key={k.l} className="rounded-xl border border-slate-200 p-3">
                  <div className="text-[10px] text-slate-400">{k.l}</div>
                  <div className={'text-lg font-bold ' + (k.v === '阻断' ? 'text-red-500' : k.v === '通过' ? 'text-emerald-600' : 'text-slate-800')}>{k.v}</div>
                </div>
              ))}
            </div>
            <div className="text-xs text-slate-600 bg-emerald-50/50 border border-emerald-100 rounded-lg px-3 py-2 mb-3"><span className="font-medium text-emerald-700">风险评估：</span>{report.risk}</div>
            <div className="text-xs text-slate-600 bg-slate-50 rounded-lg px-3 py-2 mb-3"><span className="font-medium text-slate-700">结论建议：</span>{report.conclusion}</div>
            <div className="flex gap-2">
              <button type="button" onClick={() => onExport('工程合编')} className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg hover:bg-emerald-700"><Download className="w-3 h-3" />导出报告（HTML / PDF / Word / MD）</button>
            </div>
          </div>
        )}
      </Card>
    </div>
  );
}

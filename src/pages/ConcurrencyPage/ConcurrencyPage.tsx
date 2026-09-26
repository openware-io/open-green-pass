import { CONCURRENCY_ROWS, CONFLICT_EVENTS, RESOURCE_POOLS, SCENARIO_GROUPS } from '@/data/mock';
import { PageHeader, Card } from '@/components/shared';
import { Cpu, Globe, Smartphone, Sparkles } from 'lucide-react';

const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles };
const FORM_BADGE: Record<string, string> = { '服务/后端': 'bg-emerald-50 text-emerald-600', 'Web 前端': 'bg-sky-50 text-sky-600', '移动端': 'bg-purple-50 text-purple-600', 'AI 专项': 'bg-indigo-50 text-indigo-600' };

const TYPE_BADGE: Record<string, string> = { '服务': 'bg-emerald-50 text-emerald-600', '端': 'bg-purple-50 text-purple-600' };
const LEVEL_BAR: Record<string, string> = { success: 'bg-emerald-500', warning: 'bg-amber-500', danger: 'bg-red-500' };
const LEVEL_DOT: Record<string, string> = { success: 'bg-emerald-400', warning: 'bg-amber-400', danger: 'bg-red-400' };
const CONFLICT: Record<string, string> = { danger: 'bg-red-50 border-red-100', warning: 'bg-amber-50 border-amber-100', info: 'bg-slate-50 border-slate-200' };
const CONFLICT_DOT: Record<string, string> = { danger: 'text-red-500', warning: 'text-amber-500', info: 'text-slate-400' };

export default function ConcurrencyPage() {
  return (
    <div>
      <PageHeader title="测试资源" desc="资源池 · 测试场景设计 · 调度隔离 · 冲突事件">
        <div className="flex items-center gap-3 text-xs">
          <span className="flex items-center gap-1.5"><span className="w-2 h-2 rounded-full bg-emerald-400"></span>健康</span>
          <span className="flex items-center gap-1.5"><span className="w-2 h-2 rounded-full bg-amber-400"></span>高负载</span>
          <span className="flex items-center gap-1.5"><span className="w-2 h-2 rounded-full bg-red-400"></span>饱和</span>
        </div>
      </PageHeader>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-5">
        {RESOURCE_POOLS.map((p) => (
          <div key={p.name} className="card bg-white rounded-xl border border-slate-200 p-4">
            <div className="flex items-center justify-between mb-3">
              <span className="text-xs font-medium text-slate-600">{p.name}</span>
              <span className={'w-2 h-2 rounded-full ' + LEVEL_DOT[p.level]} />
            </div>
            <div className="text-2xl font-bold text-slate-800">
              {p.used}<span className="text-sm font-normal text-slate-400"> / {p.total}</span>
            </div>
            <div className="mt-2 h-1.5 bg-slate-100 rounded-full">
              <div className={LEVEL_BAR[p.level] + ' h-full rounded-full'} style={{ width: `${(p.used / p.total) * 100}%` }} />
            </div>
            <div className="text-[11px] text-slate-400 mt-2">{p.note}</div>
          </div>
        ))}
      </div>

      <Card title="测试场景设计" extra={<span className="text-[11px] text-slate-400">平台覆盖测试场景 · 每个场景怎么测 · 依赖资源 / 工具链 / 触发 / 门禁</span>} className="p-5 mb-5">
        <div className="space-y-5">
          {SCENARIO_GROUPS.map((g) => {
            const GIcon = SCEN_ICON[g.scenarios[0]?.icon ?? 'Cpu'];
            return (
              <div key={g.label}>
                <div className="flex items-center gap-2 mb-3">
                  <GIcon className="w-4 h-4 text-emerald-600" />
                  <span className="font-medium text-sm text-slate-700">{g.label}</span>
                  <span className="text-[11px] text-slate-400">· 依赖 {g.resource}</span>
                  <span className="text-[10px] text-slate-400 ml-auto">{g.scenarios.length} 个场景</span>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3">
                  {g.scenarios.map((s) => {
                    const SIcon = SCEN_ICON[s.icon];
                    return (
                      <div key={s.id} className="border border-slate-200 rounded-xl p-4 hover:border-emerald-300 transition-colors">
                        <div className="flex items-center justify-between mb-2">
                          <div className="flex items-center gap-2">
                            <SIcon className="w-4 h-4 text-emerald-600" />
                            <span className="font-medium text-sm text-slate-800">{s.name}</span>
                            <span className="text-[10px] text-slate-400 font-mono">{s.id}</span>
                          </div>
                          <span className={'text-[10px] px-1.5 py-0.5 rounded ' + FORM_BADGE[s.form]}>{s.form}</span>
                        </div>
                        <p className="text-xs text-slate-500 leading-relaxed">{s.how}</p>
                        <div className="mt-3 flex flex-wrap gap-1.5 text-[10px]">
                          <span className="px-2 py-0.5 rounded-full bg-slate-100 text-slate-600">{s.resource}</span>
                          <span className="px-2 py-0.5 rounded-full bg-slate-100 text-slate-600">{s.tool}</span>
                          <span className="px-2 py-0.5 rounded-full bg-slate-100 text-slate-600">{s.trigger}</span>
                          <span className="px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-600">{s.gate}</span>
                        </div>
                        <details className="mt-3">
                          <summary className="text-[11px] text-emerald-600 cursor-pointer select-none">怎么测的步骤</summary>
                          <ol className="mt-2 pl-4 list-decimal text-[11px] text-slate-600 space-y-1">
                            {s.steps.map((st, i) => <li key={i}>{st}</li>)}
                          </ol>
                        </details>
                      </div>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </div>
      </Card>

      <div className="grid grid-cols-3 gap-5">
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-slate-200">
            <h2 className="font-semibold text-slate-700 text-sm">资产级并发视图</h2>
          </div>
          <table className="w-full text-xs">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-slate-500">
                <th className="px-5 py-2.5 font-medium">资产节点</th>
                <th className="px-5 py-2.5 font-medium">类型</th>
                <th className="px-5 py-2.5 font-medium">运行中</th>
                <th className="px-5 py-2.5 font-medium">排队</th>
                <th className="px-5 py-2.5 font-medium">配额</th>
                <th className="px-5 py-2.5 font-medium">使用率</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {CONCURRENCY_ROWS.map((r) => (
                <tr key={r.name} className="hover:bg-slate-50">
                  <td className="px-5 py-3 font-medium text-slate-700">{r.name}</td>
                  <td className="px-5 py-3"><span className={TYPE_BADGE[r.type] + ' text-[10px] px-1.5 py-0.5 rounded'}>{r.type}</span></td>
                  <td className="px-5 py-3 text-slate-600">{r.running}</td>
                  <td className="px-5 py-3 text-slate-600">{r.queued}</td>
                  <td className="px-5 py-3 text-slate-600">{r.quota}</td>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2">
                      <div className="w-16 h-1.5 bg-slate-100 rounded-full">
                        <div className={(r.usage >= 80 ? 'bg-red-500' : r.usage >= 65 ? 'bg-amber-500' : 'bg-emerald-500') + ' h-full rounded-full'}
                          style={{ width: r.usage + '%' }} />
                      </div>
                      <span className="text-slate-500">{r.usage}%</span>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <Card title="冲突与抢占事件" className="p-5">
          <div className="space-y-3">
            {CONFLICT_EVENTS.map((e, i) => (
              <div key={i} className={'p-3 rounded-lg border ' + CONFLICT[e.level]}>
                <div className="flex items-center gap-2 mb-1">
                  <span className={CONFLICT_DOT[e.level] + ' text-xs'}>●</span>
                  <span className="text-xs font-medium text-slate-700">{e.title}</span>
                </div>
                <div className="text-[11px] text-slate-600">{e.desc}</div>
                <div className="text-[10px] text-slate-400 mt-1">{e.meta}</div>
              </div>
            ))}
          </div>
        </Card>
      </div>
    </div>
  );
}

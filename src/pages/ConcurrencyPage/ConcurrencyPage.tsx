import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { CONCURRENCY_ROWS, CONFLICT_EVENTS, RESOURCE_POOLS, TEST_SCENARIOS } from '@/data/mock';
import { scenarioNav } from '@/context/scenarioNav';
import { PageHeader, Card } from '@/components/shared';
import { ChevronDown, ChevronRight, CornerDownRight, Cpu, Globe, Smartphone, Sparkles, Zap, ShieldCheck, Camera, History, Waypoints } from 'lucide-react';

const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles };
const FORM_BADGE: Record<string, string> = { '服务/后端': 'bg-emerald-50 text-emerald-600', 'Web 前端': 'bg-sky-50 text-sky-600', '移动端': 'bg-purple-50 text-purple-600', 'AI 专项': 'bg-indigo-50 text-indigo-600' };
const TRIGGER_BADGE: Record<string, string> = { 'CI 提交': 'bg-emerald-50 text-emerald-600', '定时': 'bg-sky-50 text-sky-600', '发布前': 'bg-indigo-50 text-indigo-600', '手动': 'bg-amber-50 text-amber-600' };

const poolRatio = (res: string): number => {
  const kw = res.includes('K8s') ? 'K8s' : res.includes('浏览器') ? '浏览器' : res.includes('真机') ? '真机' : '';
  if (kw) { const pp = RESOURCE_POOLS.find((x) => x.name.includes(kw)); if (pp) return Math.round((pp.used / pp.total) * 100); }
  return 40;
};

const TYPE_BADGE: Record<string, string> = { '服务': 'bg-emerald-50 text-emerald-600', '端': 'bg-purple-50 text-purple-600' };
const LEVEL_BAR: Record<string, string> = { success: 'bg-emerald-500', warning: 'bg-amber-500', danger: 'bg-red-500' };
const LEVEL_DOT: Record<string, string> = { success: 'bg-emerald-400', warning: 'bg-amber-400', danger: 'bg-red-400' };
const CONFLICT: Record<string, string> = { danger: 'bg-red-50 border-red-100', warning: 'bg-amber-50 border-amber-100', info: 'bg-slate-50 border-slate-200' };
const CONFLICT_DOT: Record<string, string> = { danger: 'text-red-500', warning: 'text-amber-500', info: 'text-slate-400' };

export default function ConcurrencyPage() {
  const navigate = useNavigate();
  const [expanded, setExpanded] = useState<string | null>(null);
  const toggle = (id: string) => setExpanded((cur) => (cur === id ? null : id));
  return (
    <div>
      <PageHeader title="测试资源" desc="测试执行资源底座 · 真机 / 浏览器 / 沙箱并发池 · 场景测试方法与实现方案 · 调度隔离与冲突事件">
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

      <Card title="测试场景 · 测试方法与实现方案" extra={<span className="text-[11px] text-slate-400">覆盖 12 场景：点击场景行展开完整测试方法（怎么测 / 怎么实现 / 依赖资源 / 门禁）</span>} className="p-5 mb-5">
        <div className="space-y-2">
          {TEST_SCENARIOS.map((s) => {
            const Icon = SCEN_ICON[s.icon] ?? Cpu;
            const ratio = poolRatio(s.resource);
            const bar = ratio >= 80 ? 'bg-red-500' : ratio >= 65 ? 'bg-amber-500' : 'bg-emerald-500';
            const open = expanded === s.id;
            return (
              <div key={s.id} className={'rounded-xl border transition-colors ' + (open ? 'border-emerald-300 bg-emerald-50/30' : 'border-slate-200')}>
                {/* 行头 */}
                <div role="button" onClick={() => toggle(s.id)}
                  className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-slate-50/70">
                  <span className="w-8 h-8 rounded-lg bg-emerald-500 text-white flex items-center justify-center flex-shrink-0"><Icon className="w-4 h-4" /></span>
                  <div className="w-16 flex-shrink-0"><span className="font-mono text-[10px] text-slate-400">{s.id}</span></div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-slate-800">{s.name}</div>
                    <div className="text-[11px] text-slate-500 truncate">{s.how}</div>
                  </div>
                  <span className={'text-[10px] px-1.5 py-0.5 rounded hidden md:inline ' + FORM_BADGE[s.form]}>{s.form}</span>
                  <div className="hidden lg:block w-40"><span className="text-[11px] text-slate-600">{s.resource}</span></div>
                  <div className="hidden xl:flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded-full bg-slate-100 text-slate-500 flex-shrink-0"><Zap className="w-3 h-3" />{s.tool}</div>
                  <div className="w-20 text-right flex-shrink-0">
                    <div className="flex items-center gap-1.5 justify-end">
                      <div className="w-12 h-1.5 bg-slate-100 rounded-full"><div className={bar + ' h-full rounded-full'} style={{ width: ratio + '%' }} /></div>
                      <span className="text-[10px] text-slate-500 w-7 text-right">{ratio}%</span>
                    </div>
                  </div>
                  {open ? <ChevronDown className="w-4 h-4 text-emerald-500 flex-shrink-0" /> : <ChevronRight className="w-4 h-4 text-slate-400 flex-shrink-0" />}
                </div>
                {/* 展开：测试方法详情 */}
                {open && (
                  <div className="px-4 pb-4 border-t border-emerald-100 pt-3">
                    <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
                      <div className="lg:col-span-2 space-y-3">
                        <div>
                          <div className="text-[11px] font-medium text-emerald-600 flex items-center gap-1 mb-1"><Waypoints className="w-3 h-3" />测试方法</div>
                          <p className="text-xs text-slate-700">{s.how}</p>
                        </div>
                        <div>
                          <div className="text-[11px] font-medium text-emerald-600 flex items-center gap-1 mb-1.5"><History className="w-3 h-3" />实施步骤</div>
                          <ol className="space-y-1">
                            {s.steps.map((st, i) => (
                              <li key={i} className="flex items-start gap-2 text-xs text-slate-600">
                                <span className="w-4 h-4 rounded-full bg-emerald-100 text-emerald-600 text-[10px] flex items-center justify-center flex-shrink-0 mt-0.5">{i + 1}</span>{st}
                              </li>
                            ))}
                          </ol>
                        </div>
                        <div>
                          <div className="text-[11px] font-medium text-emerald-600 flex items-center gap-1 mb-1"><ShieldCheck className="w-3 h-3" />实现路径</div>
                          <p className="text-xs text-slate-600">{s.impl}</p>
                        </div>
                      </div>
                      <div className="space-y-2 text-[11px]">
                        <div className="rounded-lg border border-slate-200 bg-white p-3">
                          <div className="text-slate-400 mb-1">依赖资源</div><div className="text-slate-700 font-medium">{s.resource}</div>
                        </div>
                        <div className="rounded-lg border border-slate-200 bg-white p-3">
                          <div className="text-slate-400 mb-1">工具链</div><div className="text-slate-700 font-medium">{s.tool}</div>
                        </div>
                        <div className="rounded-lg border border-slate-200 bg-white p-3">
                          <div className="text-slate-400 mb-1">触发时机</div><div><span className={'text-[10px] px-1.5 py-0.5 rounded ' + (TRIGGER_BADGE[s.trigger.split(' / ')[0]] ?? 'bg-slate-100 text-slate-600')}>{s.trigger}</span></div>
                        </div>
                        <div className="rounded-lg border border-slate-200 bg-white p-3">
                          <div className="text-slate-400 mb-1 flex items-center gap-1"><Camera className="w-3 h-3" />证据类型</div><div className="text-slate-700 font-medium">{s.evidence}</div>
                        </div>
                        <div className="rounded-lg border border-slate-200 bg-white p-3">
                          <div className="text-slate-400 mb-1">门禁</div><div className="text-emerald-700 font-medium">{s.gate}</div>
                        </div>
                        <button type="button"
                          onClick={() => { scenarioNav.go(s.id, 'cases'); navigate('/scenarios'); }}
                          className="w-full flex items-center justify-center gap-1 px-3 py-2 text-[11px] bg-emerald-600 text-white rounded-lg hover:bg-emerald-700">
                          <CornerDownRight className="w-3.5 h-3.5" />进入「{s.name}」测试中心闭环
                        </button>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
        <p className="mt-3 text-[10px] text-slate-400">端形态可依赖真机测试（移动端 E2E / 兼容 / 弱网），Web 依赖浏览器实例压测（Web E2E / 视觉 / 性能）——每个场景展开可见其完整测试方法与实现路径；右下按钮进入测试中心该场景完整闭环。</p>
      </Card>
      <div className="grid grid-cols-3 gap-5">
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-slate-200">
            <h2 className="font-semibold text-slate-700 text-sm">被测对象级并发视图</h2>
          </div>
          <table className="w-full text-xs">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-slate-500">
                <th className="px-5 py-2.5 font-medium">被测对象节点</th>
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
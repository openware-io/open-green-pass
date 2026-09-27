import { useNavigate } from 'react-router-dom';
import { CONCURRENCY_ROWS, CONFLICT_EVENTS, RESOURCE_POOLS, TEST_SCENARIOS } from '@/data/mock';
import { scenarioNav } from '@/context/scenarioNav';
import { PageHeader, Card } from '@/components/shared';

const FORM_BADGE: Record<string, string> = { '服务/后端': 'bg-emerald-50 text-emerald-600', 'Web 前端': 'bg-sky-50 text-sky-600', '移动端': 'bg-purple-50 text-purple-600', 'AI 专项': 'bg-indigo-50 text-indigo-600' };
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
  return (
    <div>
      <PageHeader title="测试资源" desc="资源池 · 场景资源映射 · 调度隔离 · 冲突事件">
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

      <Card title="场景 × 资源映射" extra={<span className="text-[11px] text-slate-400">每个测试场景依赖的资源与当前占用 · 点击场景行进入「测试中心」该场景闭环</span>} className="p-5 mb-5">
        <table className="w-full text-xs">
          <thead className="text-left text-slate-400 border-b border-slate-100">
            <tr><th className="py-2 font-medium">场景</th><th className="py-2 font-medium">形态</th><th className="py-2 font-medium">依赖资源</th><th className="py-2 font-medium">工具链</th><th className="py-2 font-medium">关联门禁</th><th className="py-2 font-medium">资源占用</th></tr>
          </thead>
          <tbody>
            {TEST_SCENARIOS.map((s) => {
              const ratio = poolRatio(s.resource);
              const bar = ratio >= 80 ? 'bg-red-500' : ratio >= 65 ? 'bg-amber-500' : 'bg-emerald-500';
              return (
                <tr key={s.id} onClick={() => { scenarioNav.go(s.id, "cases"); navigate("/scenarios"); }} className="border-b border-slate-50 hover:bg-emerald-50/40 cursor-pointer">
                  <td className="py-2"><span className="font-mono text-slate-400 mr-1">{s.id}</span><span className="text-slate-700 font-medium">{s.name}</span></td>
                  <td className="py-2"><span className={'text-[10px] px-1.5 py-0.5 rounded ' + FORM_BADGE[s.form]}>{s.form}</span></td>
                  <td className="py-2 text-slate-600">{s.resource}</td>
                  <td className="py-2 text-slate-600">{s.tool}</td>
                  <td className="py-2"><span className="text-emerald-600">{s.gate}</span></td>
                  <td className="py-2"><div className="flex items-center gap-2"><div className="w-16 h-1.5 bg-slate-100 rounded-full"><div className={bar + ' h-full rounded-full'} style={{ width: ratio + '%' }} /></div><span className="text-slate-500">{ratio}%</span></div></td>
                </tr>
              );
            })}
          </tbody>
        </table>
        <p className="mt-3 text-[10px] text-slate-400">点击任一场景行，进入「测试中心」该场景完整闭环（用例 / 执行 / 门禁 / 历史 / 报告）。</p>
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

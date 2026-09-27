import { useState } from 'react';
import { Link } from 'react-router-dom';
import { AUDIT_LOGS, COST_TOTAL, COST_MOM_CHANGE } from '@/data/mock';
import { PageHeader, GhostButton, Card, ListFilter } from '@/components/shared';
import { ShieldCheck, Wallet, Play, UserCog, ScrollText, ChevronRight, Layers } from 'lucide-react';

const TYPE_BADGE: Record<string, string> = {
  '门禁阻断': 'text-red-600 bg-red-50',
  '契约告警': 'text-red-600 bg-red-50',
  '篡改检测': 'text-amber-600 bg-amber-50',
  '种子生成': 'text-emerald-600 bg-emerald-50',
  '冲突检测': 'text-red-600 bg-red-50',
  '执行通过': 'text-emerald-600 bg-emerald-50',
};

const DOMAINS = [
  { to: '/audit-cost', icon: Wallet, color: 'text-emerald-600 bg-emerald-50 border-emerald-200', title: '成本审计', desc: '金额 · 维度 · 降本', kpi: `¥${COST_TOTAL.toLocaleString()}`, sub: `环比 ${Math.abs(COST_MOM_CHANGE)}% ↓` },
  { to: '/audit-exec', icon: Play, color: 'text-indigo-600 bg-indigo-50 border-indigo-200', title: '执行审计', desc: '用例 · 结果 · 耗时', kpi: '1,248 / 1,300', sub: '失败 34 · 阻塞 10' },
  { to: '/audit-op', icon: UserCog, color: 'text-amber-600 bg-amber-50 border-amber-200', title: '操作审计', desc: '谁 · 何时 · 做了什么', kpi: '128 条', sub: '近 24h · 高风险 3' },
  { to: '/audit', icon: ScrollText, color: 'text-slate-600 bg-slate-50 border-slate-200', title: '原始日志', desc: '哈希链 · 仅追加', kpi: '12,847 条', sub: '外部锚定 WORM ✓' },
];

export default function AuditPage() {
  const [q, setQ] = useState('');
  const [type, setType] = useState('');
  const kw = q.trim().toLowerCase();
  const filtered = AUDIT_LOGS.filter((log) => {
    if (type && log.type !== type) return false;
    if (kw && !(log.seq + log.message + log.actor + log.asset).toLowerCase().includes(kw)) return false;
    return true;
  });
  return (
    <div>
      <PageHeader title="审计总览" desc="哈希链 · 仅追加 · 可独立验证 · 一份链四视图">
        <GhostButton><span className="flex items-center gap-1"><ShieldCheck className="w-3.5 h-3.5" />验证链完整性</span></GhostButton>
        <GhostButton><span className="flex items-center gap-1"><ScrollText className="w-3.5 h-3.5" />导出审计报告</span></GhostButton>
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

      {/* 一份链四视图：设计原则 */}
      <div className="mb-5 px-4 py-3 rounded-xl bg-emerald-50 border border-emerald-200 flex items-start gap-3">
        <Layers className="w-4 h-4 text-emerald-600 flex-shrink-0 mt-0.5" />
        <div className="text-[11px] text-emerald-800 leading-relaxed">
          <span className="font-semibold">审计设计原则 · 一份链四视图：</span>
          底层为<span className="font-semibold">单一哈希链日志</span>（唯一不可变数据源，跨被测对象 / 跨域追溯靠它），上层按业务域拆成独立视图
          —— <span className="font-semibold">成本审计 / 执行审计 / 操作审计 / 审计总览</span>。既满足各自管理，又不破坏"仅追加、可独立验证"的审计根基。
        </div>
      </div>

      {/* 四域概览 */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        {DOMAINS.map((d) => {
          const Icon = d.icon;
          return (
            <Link key={d.title} to={d.to} className="card bg-white rounded-xl border border-slate-200 p-4 hover:shadow-md transition group">
              <div className="flex items-center justify-between mb-3">
                <span className={`w-9 h-9 rounded-lg ${d.color} flex items-center justify-center`}><Icon className="w-4 h-4" /></span>
                <ChevronRight className="w-4 h-4 text-slate-300 group-hover:text-emerald-500 transition" />
              </div>
              <div className="text-sm font-semibold text-slate-700">{d.title}</div>
              <div className="text-[11px] text-slate-400 mt-0.5">{d.desc}</div>
              <div className="text-xl font-bold text-slate-800 mt-2">{d.kpi}</div>
              <div className="text-[11px] text-slate-400 mt-0.5">{d.sub}</div>
            </Link>
          );
        })}
      </div>

      <Card title="原始日志流"
        extra={
          <select className="border border-slate-300 rounded-lg px-2 py-1 outline-none text-xs">
            <option>全部被测对象</option>
            <option>svc-auth</option>
            <option>svc-payment</option>
          </select>
        }>
        <div className="flex items-center justify-between px-5 pt-3">
          <ListFilter search={q} onSearch={setQ}
            selects={[{ key: 'type', label: '类型', options: ['门禁阻断', '契约告警', '篡改检测', '种子生成', '冲突检测', '执行通过'], value: type, onChange: setType }]} />
          <span className="text-[11px] text-slate-400">共 {filtered.length} / {AUDIT_LOGS.length} 条</span>
        </div>
        <div className="divide-y divide-slate-100">
          {filtered.map((log) => (
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
  );
}
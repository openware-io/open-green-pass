import { AUDIT_LOGS } from '@/data/mock';
import { PageHeader, GhostButton, Card } from '@/components/shared';

const TYPE_BADGE: Record<string, string> = {
  '门禁阻断': 'text-red-600 bg-red-50',
  '契约告警': 'text-red-600 bg-red-50',
  '篡改检测': 'text-amber-600 bg-amber-50',
  '种子生成': 'text-emerald-600 bg-emerald-50',
  '冲突检测': 'text-red-600 bg-red-50',
  '执行通过': 'text-emerald-600 bg-emerald-50',
};

export default function AuditPage() {
  return (
    <div>
      <PageHeader title="审计日志" desc="哈希链 · 仅追加 · 可独立验证 · 跨资产审计">
        <GhostButton>验证链完整性</GhostButton>
        <GhostButton>导出审计报告</GhostButton>
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
  );
}

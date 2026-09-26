import { OP_AUDIT_ROWS } from '@/data/mock';
import { PageHeader, Card } from '@/components/shared';
import { UserCog, ShieldAlert, Bot, Fingerprint } from 'lucide-react';

const RISK_BADGE: Record<string, string> = {
  '低': 'bg-emerald-50 text-emerald-600',
  '中': 'bg-amber-50 text-amber-600',
  '高': 'bg-red-50 text-red-600',
};

export default function AuditOpPage() {
  const total = OP_AUDIT_ROWS.length;
  const high = OP_AUDIT_ROWS.filter((r) => r.risk === '高').length;
  const med = OP_AUDIT_ROWS.filter((r) => r.risk === '中').length;
  const low = OP_AUDIT_ROWS.filter((r) => r.risk === '低').length;
  const bot = OP_AUDIT_ROWS.filter((r) => r.user === 'ai-agent-3').length;

  return (
    <div>
      <PageHeader title="操作审计" desc="谁 · 何时 · 做了什么 · 风险分级">
        <span className="text-[11px] text-slate-400">近 24h · 覆盖 AI Agent 与人工操作</span>
      </PageHeader>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-5">
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">操作总数</span><Fingerprint className="w-4 h-4 text-indigo-500" /></div>
          <div className="text-2xl font-bold text-slate-800">128 <span className="text-sm font-normal text-slate-400">/ 24h</span></div>
          <div className="mt-1 text-[11px] text-slate-400">全部写入哈希链</div>
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">高风险操作</span><ShieldAlert className="w-4 h-4 text-red-500" /></div>
          <div className="text-2xl font-bold text-red-600">3</div>
          <div className="mt-1 text-[11px] text-slate-400">契约变更 · 篡改检测</div>
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">AI Agent 操作</span><Bot className="w-4 h-4 text-emerald-500" /></div>
          <div className="text-2xl font-bold text-slate-800">{Math.round((bot / total) * 100)}<span className="text-sm font-normal text-slate-400">%</span></div>
          <div className="mt-1 text-[11px] text-slate-400">用例生成 · 判定 · 检测</div>
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">操作者</span><UserCog className="w-4 h-4 text-amber-500" /></div>
          <div className="text-xl font-bold text-slate-800">6 <span className="text-sm font-normal text-slate-400">人 / 系统</span></div>
          <div className="mt-1 text-[11px] text-slate-400">含 AI Agent · 系统</div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-5 mb-5">
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 p-5">
          <h3 className="font-semibold text-slate-700 text-sm mb-4">风险等级分布</h3>
          <div className="flex h-6 rounded-full overflow-hidden mb-3">
            <div className="bg-emerald-500" style={{ width: `${(low / total) * 100}%` }} />
            <div className="bg-amber-500" style={{ width: `${(med / total) * 100}%` }} />
            <div className="bg-red-500" style={{ width: `${(high / total) * 100}%` }} />
          </div>
          <div className="flex gap-4 text-xs text-slate-500">
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-emerald-500" />低 {Math.round((low / total) * 100)}%</span>
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-amber-500" />中 {Math.round((med / total) * 100)}%</span>
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-red-500" />高 {Math.round((high / total) * 100)}%</span>
          </div>
          <p className="mt-3 text-[11px] text-slate-400">高风险操作（契约破坏性变更、用例篡改）自动触发门禁阻断，已记录操作者、时间与哈希链锚点。</p>
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <h3 className="font-semibold text-slate-700 text-sm mb-3">操作类型占比</h3>
          <div className="space-y-3">
            {[['用例 / 执行相关', 42], ['质量门禁判定', 24], ['契约变更', 18], ['系统检测', 10], ['其他', 6]].map(([n, v]) => (
              <div key={n as string}>
                <div className="flex justify-between text-xs mb-1"><span className="text-slate-600">{n}</span><span className="text-slate-500">{v}%</span></div>
                <div className="h-1.5 bg-slate-100 rounded-full"><div className="h-full rounded-full bg-indigo-400" style={{ width: `${v}%` }} /></div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <Card title="操作审计明细" extra={<span className="text-[11px] text-slate-400">一条操作 = 一条哈希链日志（仅追加）</span>}>
        <table className="w-full text-xs">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-slate-500">
              <th className="px-4 py-2.5 font-medium">操作者</th>
              <th className="px-4 py-2.5 font-medium">角色</th>
              <th className="px-4 py-2.5 font-medium">操作</th>
              <th className="px-4 py-2.5 font-medium">对象</th>
              <th className="px-4 py-2.5 font-medium">结果</th>
              <th className="px-4 py-2.5 font-medium">风险</th>
              <th className="px-4 py-2.5 font-medium">时间</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {OP_AUDIT_ROWS.map((r) => (
              <tr key={r.user + r.action + r.ts} className="hover:bg-slate-50">
                <td className="px-4 py-3"><span className="font-medium text-slate-700">{r.user}</span> <span className="text-[10px] text-slate-400">{r.role}</span></td>
                <td className="px-4 py-3 text-slate-500">{r.role}</td>
                <td className="px-4 py-3 font-medium text-slate-700">{r.action}</td>
                <td className="px-4 py-3 font-mono text-indigo-600 text-[11px]">{r.target}</td>
                <td className="px-4 py-3 text-slate-600">{r.result}</td>
                <td className="px-4 py-3"><span className={`px-2 py-0.5 rounded-full text-[10px] ${RISK_BADGE[r.risk]}`}>{r.risk}</span></td>
                <td className="px-4 py-3 text-slate-400">{r.ts}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}
import { REQUIREMENTS } from '@/data/mock';
import { PageHeader, GhostButton, Card } from '@/components/shared';

const TRACE_ROWS = [
  { dot: 'bg-emerald-400', asset: 'svc-user', tp: 'TP-101-3', tc: 'TC-005', status: '✓ 通过', color: 'text-emerald-600' },
  { dot: 'bg-blue-400', asset: 'web-frontend', tp: 'TP-101-4', tc: 'TC-006, TC-007', status: '✓ 通过', color: 'text-emerald-600' },
  { dot: 'bg-purple-400', asset: 'mobile-ios', tp: 'TP-101-5', tc: 'TC-008', status: '⚠ 阻塞', color: 'text-amber-600' },
  { dot: 'bg-purple-400', asset: 'mobile-android', tp: 'TP-101-5', tc: 'TC-009', status: '✓ 通过', color: 'text-emerald-600' },
];

const TRACEABILITY_BADGE: Record<string, string> = {
  '完整': 'bg-emerald-50 text-emerald-600',
  '部分': 'bg-amber-50 text-amber-600',
  '缺口': 'bg-red-50 text-red-600',
};

export default function TracePage() {
  return (
    <div>
      <PageHeader title="需求与追溯矩阵" desc="跨服务追溯链 · 从需求到执行的完整链路">
        <GhostButton>检测覆盖缺口</GhostButton>
        <GhostButton>切换视图</GhostButton>
      </PageHeader>

      <Card title="跨服务追溯链 · REQ-101 用户登录与鉴权" className="p-5 mb-5"
        extra={
          <div className="flex gap-3 text-[10px]">
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded bg-amber-400"></span>系统</span>
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded bg-indigo-400"></span>服务组</span>
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded bg-emerald-400"></span>服务</span>
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded bg-blue-400"></span>端</span>
          </div>
        }>
        <div className="grid grid-cols-5 gap-0">
          <div className="px-3 py-2 bg-amber-50 border border-amber-200 rounded-lg text-center">
            <div className="text-[10px] text-amber-600 font-medium">系统</div>
            <div className="text-xs text-slate-700 mt-0.5">sys-payment-platform</div>
          </div>
          <div className="px-3 py-2 bg-indigo-50 border border-indigo-200 rounded-lg text-center">
            <div className="text-[10px] text-indigo-600 font-medium">服务组</div>
            <div className="text-xs text-slate-700 mt-0.5">身份服务组</div>
          </div>
          <div className="px-3 py-2 bg-emerald-50 border border-emerald-200 rounded-lg text-center">
            <div className="text-[10px] text-emerald-600 font-medium">服务</div>
            <div className="text-xs text-slate-700 mt-0.5">svc-auth</div>
            <div className="text-[10px] text-slate-500 mt-0.5">TP-101-1, TP-101-2</div>
          </div>
          <div className="px-3 py-2 bg-white border border-slate-200 rounded-lg text-center">
            <div className="text-[10px] text-slate-500">测试点</div>
            <div className="text-[10px] text-slate-700 mt-0.5">密码校验 / Token签发</div>
          </div>
          <div className="flex flex-col gap-1.5">
            {['TC-001 正常登录', 'TC-002 密码错误', 'TC-003 Token刷新', 'TC-004 Token过期'].map((tc) => (
              <div key={tc} className="px-2 py-1 bg-white border border-slate-200 rounded text-[10px] text-slate-600">{tc}</div>
            ))}
          </div>
        </div>
        <div className="mt-4 pt-4 border-t border-slate-100 space-y-2">
          {TRACE_ROWS.map((r, i) => (
            <div key={i} className="flex items-center gap-3 text-[11px]">
              <span className={r.dot + ' w-2 h-2 rounded'} />
              <span className="font-mono text-slate-500 w-32">{r.asset}</span>
              <span className="text-slate-300">→</span>
              <span className="font-mono text-slate-500 w-32">{r.tp}</span>
              <span className="text-slate-300">→</span>
              <span className="font-mono text-indigo-500">{r.tc}</span>
              <span className={'ml-auto ' + r.color}>{r.status}</span>
            </div>
          ))}
        </div>
      </Card>

      <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
        <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
          <h2 className="font-semibold text-slate-700 text-sm">需求追溯矩阵</h2>
          <div className="flex gap-2 text-[11px]">
            <span className="flex items-center gap-1"><span className="w-3 h-3 rounded bg-emerald-100 border border-emerald-300"></span>已覆盖</span>
            <span className="flex items-center gap-1"><span className="w-3 h-3 rounded bg-amber-100 border border-amber-300"></span>部分</span>
            <span className="flex items-center gap-1"><span className="w-3 h-3 rounded bg-red-100 border border-red-300"></span>缺口</span>
          </div>
        </div>
        <table className="w-full text-sm">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-xs text-slate-500">
              <th className="px-4 py-3 font-medium">需求 ID</th>
              <th className="px-4 py-3 font-medium">需求标题</th>
              <th className="px-4 py-3 font-medium">涉及资产</th>
              <th className="px-4 py-3 font-medium">测试点</th>
              <th className="px-4 py-3 font-medium">用例数</th>
              <th className="px-4 py-3 font-medium">执行状态</th>
              <th className="px-4 py-3 font-medium">追溯完整性</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {REQUIREMENTS.map((r) => (
              <tr key={r.id} className={r.traceability === '缺口' ? 'bg-red-50/40' : 'hover:bg-slate-50'}>
                <td className={'px-4 py-3 font-mono text-xs font-medium ' + (r.traceability === '缺口' ? 'text-red-600' : 'text-indigo-600')}>{r.id}</td>
                <td className="px-4 py-3 text-slate-700">{r.title}</td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-1">
                    {r.assets.map((a) => (
                      <span key={a} className="text-[10px] bg-slate-50 text-slate-600 px-1.5 py-0.5 rounded">{a}</span>
                    ))}
                  </div>
                </td>
                <td className={'px-4 py-3 ' + (r.traceability === '缺口' ? 'text-red-500 font-medium' : 'text-slate-600')}>{r.testPoints}</td>
                <td className={'px-4 py-3 ' + (r.traceability === '缺口' ? 'text-red-500 font-medium' : 'text-slate-600')}>{r.cases}</td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <div className="w-16 h-1.5 bg-slate-100 rounded-full">
                      <div className={r.executed === r.cases && r.cases > 0 ? 'bg-emerald-500 h-full rounded-full' : 'bg-amber-500 h-full rounded-full'}
                        style={{ width: r.cases > 0 ? `${(r.executed / r.cases) * 100}%` : '0%' }} />
                    </div>
                    <span className="text-xs text-slate-500">{r.executed}/{r.cases}</span>
                  </div>
                </td>
                <td className="px-4 py-3">
                  <span className={'text-[11px] px-2 py-0.5 rounded-full ' + TRACEABILITY_BADGE[r.traceability]}>{r.traceability}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

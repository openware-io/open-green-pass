import { useState } from 'react';
import { EXEC_AUDIT_ROWS, TEST_RUNS } from '@/data/mock';
import { PageHeader, Card, ListFilter } from '@/components/shared';
import { Play, XCircle, Ban, Timer } from 'lucide-react';

const RESULT_BADGE: Record<string, string> = {
  '通过': 'bg-emerald-50 text-emerald-600',
  '失败': 'bg-red-50 text-red-600',
  '阻塞': 'bg-amber-50 text-amber-600',
};

export default function AuditExecPage() {
  const run = TEST_RUNS.find((r) => r.id === 'RUN-4821');
  const done = run ? run.pass + run.fail + run.block : EXEC_AUDIT_ROWS.length;
  const [q, setQ] = useState('');
  const [result, setResult] = useState('');
  const kw = q.trim().toLowerCase();
  const filtered = EXEC_AUDIT_ROWS.filter((r) => {
    if (result && r.result !== result) return false;
    if (kw && !(r.caseId + r.asset + r.model).toLowerCase().includes(kw)) return false;
    return true;
  });
  const total = EXEC_AUDIT_ROWS.length;
  const pass = EXEC_AUDIT_ROWS.filter((r) => r.result === '通过').length;
  const fail = EXEC_AUDIT_ROWS.filter((r) => r.result === '失败').length;
  const block = EXEC_AUDIT_ROWS.filter((r) => r.result === '阻塞').length;
  const durations = EXEC_AUDIT_ROWS.filter((r) => r.duration !== '—').map((r) => parseFloat(r.duration));
  const avg = durations.length ? (durations.reduce((s, v) => s + v, 0) / durations.length).toFixed(1) : '0';

  return (
    <div>
      <PageHeader title="执行审计" desc="用例 · 结果 · 耗时 · 跨被测对象执行（本次 CI #4821）">
        <span className="text-[11px] text-slate-400">统计周期：近 14 天</span>
      </PageHeader>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-5">
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">本次执行</span><Play className="w-4 h-4 text-emerald-500" /></div>
          <div className="text-2xl font-bold text-slate-800">{done.toLocaleString()} <span className="text-sm font-normal text-slate-400">/ {(run?.total ?? 1300).toLocaleString()}</span></div>
          <div className="mt-1 text-[11px] text-slate-400">进度 96% · 预计剩余 4m</div>
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">失败用例</span><XCircle className="w-4 h-4 text-red-500" /></div>
          <div className="text-2xl font-bold text-red-600">{run?.fail ?? fail}</div>
          <div className="mt-1 text-[11px] text-slate-400">关联 12 条存量用例成本</div>
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">平均耗时</span><Timer className="w-4 h-4 text-emerald-600" /></div>
          <div className="text-2xl font-bold text-slate-800">{avg}<span className="text-sm font-normal text-slate-400">s</span></div>
          <div className="mt-1 text-[11px] text-slate-400">移动端耗时更高</div>
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-4">
          <div className="flex items-center justify-between mb-2"><span className="text-xs text-slate-500">执行器在线</span><Ban className="w-4 h-4 text-amber-500" /></div>
          <div className="text-2xl font-bold text-slate-800">12 <span className="text-sm font-normal text-slate-400">/ 16</span></div>
          <div className="mt-1 text-[11px] text-slate-400">K8s 沙箱高负载</div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-5 mb-5">
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 p-5">
          <h3 className="font-semibold text-slate-700 text-sm mb-4">执行结果占比</h3>
          <div className="flex h-6 rounded-full overflow-hidden mb-3">
            <div className="bg-emerald-500" style={{ width: `${(pass / total) * 100}%` }} />
            <div className="bg-red-500" style={{ width: `${(fail / total) * 100}%` }} />
            <div className="bg-amber-500" style={{ width: `${(block / total) * 100}%` }} />
          </div>
          <div className="flex gap-4 text-xs text-slate-500">
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-emerald-500" />通过 {Math.round((pass / total) * 100)}%</span>
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-red-500" />失败 {Math.round((fail / total) * 100)}%</span>
            <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-amber-500" />阻塞 {Math.round((block / total) * 100)}%</span>
          </div>
          <p className="mt-3 text-[11px] text-slate-400">失败主要集中于 svc-payment（TC-2024-095 幂等性、TC-2024-118 退款金额计算），与质量门禁阻断一致。</p>
        </div>
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <h3 className="font-semibold text-slate-700 text-sm mb-3">执行器负载（K8s 沙箱）</h3>
          <div className="space-y-3">
            {[['svc-auth', 60, '60%'], ['svc-payment', 80, '80%'], ['web-frontend', 55, '55%'], ['mobile-android', 94, '94%']].map(([n, w, l]) => (
              <div key={n as string}>
                <div className="flex justify-between text-xs mb-1"><span className="text-slate-600 font-mono">{n}</span><span className="text-slate-500">{l}</span></div>
                <div className="h-1.5 bg-slate-100 rounded-full"><div className={'h-full rounded-full ' + (Number(w) >= 85 ? 'bg-red-500' : Number(w) >= 70 ? 'bg-amber-500' : 'bg-emerald-500')} style={{ width: `${w}%` }} /></div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <Card title="执行审计明细" extra={<span className="text-[11px] text-slate-400">关联：用例生成成本 → 执行成本 → 门禁判定</span>}>
        <div className="flex items-center justify-between px-5 pt-3">
          <ListFilter search={q} onSearch={setQ} selects={[{ key: 'result', label: '结果', options: ['通过', '失败', '阻塞'], value: result, onChange: setResult }]} />
          <span className="text-[11px] text-slate-400">共 {filtered.length} / {EXEC_AUDIT_ROWS.length} 条</span>
        </div>
        <table className="w-full text-xs">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-slate-500">
              <th className="px-4 py-2.5 font-medium">用例 ID</th>
              <th className="px-4 py-2.5 font-medium">所属服务</th>
              <th className="px-4 py-2.5 font-medium">结果</th>
              <th className="px-4 py-2.5 font-medium">耗时</th>
              <th className="px-4 py-2.5 font-medium">执行模型</th>
              <th className="px-4 py-2.5 font-medium">时间</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {filtered.map((r) => (
              <tr key={r.caseId + r.ts} className="hover:bg-slate-50">
                <td className="px-4 py-3 font-mono text-emerald-700 font-medium">{r.caseId}</td>
                <td className="px-4 py-3"><span className="text-[10px] bg-slate-100 text-slate-600 px-1.5 py-0.5 rounded font-mono">{r.asset}</span></td>
                <td className="px-4 py-3"><span className={`px-2 py-0.5 rounded-full text-[10px] ${RESULT_BADGE[r.result]}`}>{r.result}</span></td>
                <td className="px-4 py-3 font-mono text-slate-500">{r.duration}</td>
                <td className="px-4 py-3 text-slate-600">{r.model}</td>
                <td className="px-4 py-3 text-slate-400">{r.ts}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}
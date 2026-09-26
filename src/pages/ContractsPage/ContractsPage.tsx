import { CONTRACTS } from '@/data/mock';
import { PageHeader, GhostButton, PrimaryButton, Card } from '@/components/shared';
import { AlertTriangle } from 'lucide-react';

const STATUS_BADGE: Record<string, string> = {
  pass: 'bg-emerald-50 text-emerald-600',
  pending: 'bg-amber-50 text-amber-600',
  fail: 'bg-red-50 text-red-600',
};
const STATUS_TEXT: Record<string, string> = { pass: '✓ 通过', pending: '⚠ 变更待验证', fail: '⚠ 变更待验证' };

export default function ContractsPage() {
  return (
    <div>
      <PageHeader title="契约测试" desc="服务间契约注册 · 变更影响分析 · 消费者验证">
        <GhostButton>扫描契约变更</GhostButton>
        <PrimaryButton>注册新契约</PrimaryButton>
      </PageHeader>

      <div className="card bg-red-50 border border-red-200 rounded-xl p-4 mb-5 flex items-start gap-3">
        <AlertTriangle className="text-red-500 w-5 h-5 mt-0.5 flex-shrink-0" />
        <div className="flex-1">
          <div className="text-sm font-semibold text-red-700">契约变更告警 · svc-payment</div>
          <div className="text-xs text-red-600 mt-1">/v2/refund 响应结构变更：新增 refund_id 字段，移除 status 字段。影响 3 个消费者，2 个契约测试待更新。</div>
          <div className="flex gap-4 mt-2 text-[11px] text-red-500">
            <span>检测时间：2026-09-26 10:28:00</span>
            <span>消费者：web-frontend, mobile-ios, svc-order</span>
          </div>
        </div>
        <button className="px-3 py-1.5 text-xs bg-red-600 text-white rounded-lg flex-shrink-0">查看影响</button>
      </div>

      <div className="grid grid-cols-3 gap-5">
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-slate-200">
            <h2 className="font-semibold text-slate-700 text-sm">契约注册表</h2>
          </div>
          <table className="w-full text-xs">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-slate-500">
                <th className="px-5 py-2.5 font-medium">契约 ID</th>
                <th className="px-5 py-2.5 font-medium">提供者</th>
                <th className="px-5 py-2.5 font-medium">接口</th>
                <th className="px-5 py-2.5 font-medium">消费者</th>
                <th className="px-5 py-2.5 font-medium">验证状态</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {CONTRACTS.map((c) => (
                <tr key={c.id} className={c.status === 'fail' ? 'bg-red-50/40' : 'hover:bg-slate-50'}>
                  <td className="px-5 py-3 font-mono text-indigo-600 font-medium">{c.id}</td>
                  <td className="px-5 py-3 text-emerald-600">{c.provider}</td>
                  <td className="px-5 py-3 font-mono text-slate-600">{c.endpoint}</td>
                  <td className="px-5 py-3 text-slate-500">{c.consumers}</td>
                  <td className="px-5 py-3">
                    <span className={STATUS_BADGE[c.status] + ' px-2 py-0.5 rounded-full text-[10px]'}>{STATUS_TEXT[c.status]}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="space-y-5">
          <Card title="变更影响分析" className="p-5">
            <div className="bg-slate-900 rounded-lg p-3 mb-3">
              <pre className="text-[10px] text-slate-300 font-mono leading-relaxed">CT-003: /v2/refund
provider: svc-payment
change: response schema
  + refund_id: string
  - status: string
consumers:
  - web-frontend (v2.3.1)
  - mobile-ios (v4.1.0)
  - svc-order (v1.8.2)</pre>
            </div>
            <div className="space-y-2 text-[11px]">
              <div className="flex justify-between"><span className="text-slate-500">影响消费者</span><span className="text-red-600 font-medium">3 个</span></div>
              <div className="flex justify-between"><span className="text-slate-500">破坏性变更</span><span className="text-red-600 font-medium">是（字段移除）</span></div>
              <div className="flex justify-between"><span className="text-slate-500">待更新契约测试</span><span className="text-amber-600 font-medium">2 个</span></div>
              <div className="flex justify-between"><span className="text-slate-500">门禁状态</span><span className="text-red-600 font-medium">阻断</span></div>
            </div>
          </Card>
          <Card title="契约门禁规则" className="p-5">
            <div className="bg-slate-900 rounded-lg p-3">
              <pre className="text-[10px] text-slate-300 font-mono leading-relaxed">contract_gate:
  provider: svc-payment
  rule: no-breaking-changes
  on_violation: block
  require:
    - consumer_tests_pass
    - changelog_updated
    - version_bumped</pre>
            </div>
            <div className="mt-3 text-[11px] text-red-600">当前状态：阻断中（CT-003）</div>
          </Card>
        </div>
      </div>
    </div>
  );
}

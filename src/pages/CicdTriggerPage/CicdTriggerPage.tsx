import { useState } from 'react';
import { toast } from 'sonner';
import { CICD_TRIGGERS } from '@/data/mock';
import { PageHeader, PrimaryButton, ListFilter } from '@/components/shared';
import { Zap, ShieldCheck, AlarmClock, RefreshCcw, Check, Ban } from 'lucide-react';

const GATE_BILL = [
  { icon: ShieldCheck, title: '判定阻断', desc: '门禁未通过 → 阻断流水线 / 阻止合并 / 阻止发布', value: '阻断规则 · 达标阈值 85 分' },
  { icon: RefreshCcw, title: '失败降级', desc: 'CI 超时 / 不可达 → 默认失败降级（fail-closed），避免自放水', value: 'fail-closed · 超时 10m' },
  { icon: AlarmClock, title: '回写目标', desc: '门禁结果回写 CI 状态 + 通知 IM / 邮件，附证据与报告链接', value: 'CI 状态 + IM + 邮件' },
  { icon: Ban, title: '安全护栏', desc: '签名校验 + 白名单 + 操作审计，防伪造 Webhook', value: 'HMAC 签名 · Token' },
];

export default function CicdTriggerPage() {
  const [q, setQ] = useState('');
  const [st, setSt] = useState('全部');
  const kw = q.trim().toLowerCase();
  const list = CICD_TRIGGERS.filter((t) =>
    (st === '全部' || (st === '已启用' ? t.enabled : !t.enabled)) &&
    (!kw || (t.id + t.asset + t.event + t.branch).toLowerCase().includes(kw)));

  return (
    <div>
      <PageHeader title="触发与回写" desc="配置 Pipeline 触发规则，以及质量门禁判定结果回写 CI 的阻断 / 放行策略">
        <PrimaryButton onClick={() => toast.success('新建触发规则（原型示意）', { description: '选择被测对象 + 测试场景 + 触发事件 + 环境 + 分支' })}>新建触发规则</PrimaryButton>
      </PageHeader>

      <div className="grid grid-cols-4 gap-4 mb-5">
        {GATE_BILL.map((g) => {
          const Icon = g.icon;
          return (
            <div key={g.title} className="card bg-white rounded-xl border border-slate-200 p-4">
              <div className="flex items-center gap-2 mb-2">
                <Icon className="w-4 h-4 text-emerald-500" />
                <span className="text-sm font-medium text-slate-700">{g.title}</span>
              </div>
              <div className="text-[10px] text-slate-400 leading-relaxed mb-2">{g.desc}</div>
              <span className="text-[10px] text-emerald-600 bg-emerald-50 px-2 py-0.5 rounded">{g.value}</span>
            </div>
          );
        })}
      </div>

      <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
        <div className="px-5 py-4 border-b border-slate-100 flex items-center justify-between">
          <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><Zap className="w-4 h-4 text-emerald-500" />触发规则</h2>
          <span className="text-[11px] text-slate-400">事件 → 自动发起对应场景测试 → 门禁判定 → 回写 CI</span>
        </div>
        <div className="flex items-center justify-between px-5 pt-3">
          <ListFilter search={q} onSearch={setQ}
            selects={[{ key: 'st', label: '状态', options: ['全部', '已启用', '已停用'], value: st, onChange: setSt }]} />
          <span className="text-[11px] text-slate-400">共 {list.length} / {CICD_TRIGGERS.length} 条</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-[10px] text-slate-400 border-b border-slate-100">
                <th className="px-5 py-2.5 font-medium">规则 ID</th>
                <th className="px-2 py-2.5 font-medium">被测对象</th>
                <th className="px-2 py-2.5 font-medium">测试场景</th>
                <th className="px-2 py-2.5 font-medium">触发事件</th>
                <th className="px-2 py-2.5 font-medium">环境 / 分支</th>
                <th className="px-2 py-2.5 font-medium">启用</th>
                <th className="px-2 py-2.5 font-medium">最近运行</th>
                <th className="px-2 py-2.5 font-medium">阻断数</th>
              </tr>
            </thead>
            <tbody>
              {list.map((t) => (
                <tr key={t.id} className="border-b border-slate-50 hover:bg-slate-50/60">
                  <td className="px-5 py-3 font-mono text-slate-600">{t.id}</td>
                  <td className="px-2 py-3 text-slate-700">{t.asset}</td>
                  <td className="px-2 py-3 text-slate-500">{t.scenarios}</td>
                  <td className="px-2 py-3 text-slate-500">{t.event}</td>
                  <td className="px-2 py-3 text-slate-500 font-mono">{t.branch}<span className="text-slate-300"> · </span>{t.env}</td>
                  <td className="px-2 py-3">
                    <button type="button"
                      onClick={() => toast.success(t.enabled ? '已停用（示意）' : '已启用（示意）', { description: t.id })}
                      className={'w-8 h-4.5 rounded-full relative transition ' + (t.enabled ? 'bg-emerald-500' : 'bg-slate-300')}>
                      <span className={'absolute top-0.5 w-3.5 h-3.5 rounded-full bg-white transition-all ' + (t.enabled ? 'left-4' : 'left-0.5')} />
                    </button>
                  </td>
                  <td className="px-2 py-3 text-slate-400">{t.lastRun}</td>
                  <td className="px-2 py-3">{t.gateBlock > 0 ? <span className="text-red-600 font-medium flex items-center gap-1"><Ban className="w-3 h-3" />{t.gateBlock}</span> : <span className="text-emerald-600 flex items-center gap-1"><Check className="w-3 h-3" />0</span>}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

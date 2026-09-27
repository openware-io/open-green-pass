import { useState } from 'react';
import { toast } from 'sonner';
import { Link } from 'react-router-dom';
import { CICD_CONNECTORS, CICD_RUNS } from '@/data/mock';
import { PageHeader, KpiCard, PrimaryButton, ListFilter } from '@/components/shared';
import { Workflow, Webhook, Terminal, GitMerge, Plug, Activity, Clock, XCircle, CheckCircle2, Loader, ChevronRight } from 'lucide-react';

const FORM_CARDS = [
  { icon: Workflow, title: 'Pipeline 触发', desc: 'Git 提交 / 合并请求 / 定时事件自动发起测试' },
  { icon: Webhook, title: 'Webhook 回调', desc: 'CI 工具经 HMAC 签名回调调用测试 API' },
  { icon: Terminal, title: 'CLI 接入', desc: '流水线脚本内 greenpass test 直接调用' },
  { icon: GitMerge, title: '门禁回写', desc: '质量门禁判定结果回调 CI，阻断 / 放行流水线' },
];

const RUN_STATUS: Record<string, { label: string; cls: string; icon: typeof CheckCircle2 }> = {
  success: { label: '通过', cls: 'text-emerald-600 bg-emerald-50', icon: CheckCircle2 },
  blocked: { label: '阻断', cls: 'text-red-600 bg-red-50', icon: XCircle },
  running: { label: '判定中', cls: 'text-amber-600 bg-amber-50', icon: Loader },
  failed: { label: '失败', cls: 'text-slate-500 bg-slate-100', icon: XCircle },
};

export default function CicdPage() {
  const [q, setQ] = useState('');
  const kw = q.trim().toLowerCase();
  const runs = CICD_RUNS.filter((r) => !kw || (r.id + r.asset + r.scenarios + r.commit).toLowerCase().includes(kw));

  return (
    <div>
      <PageHeader title="CI/CD 对接" desc="将 GreenPass 测试治理闭环嵌入 CI/CD 流水线 · Pipeline 触发 / Webhook 回调 / CLI 接入 / 门禁回写">
        <PrimaryButton onClick={() => toast.success('新建触发（原型示意）', { description: '前往「触发与回写」配置被测对象 + 场景 + 事件' })}>新建触发</PrimaryButton>
      </PageHeader>

      {/* 集成形态 4 卡 */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-5">
        {FORM_CARDS.map((f) => {
          const Icon = f.icon;
          return (
            <div key={f.title} className="card bg-white rounded-xl border border-slate-200 p-4 hover:border-emerald-300 transition">
              <div className="w-9 h-9 rounded-lg bg-emerald-500/10 text-emerald-600 flex items-center justify-center mb-3"><Icon className="w-4.5 h-4.5" /></div>
              <div className="text-sm font-medium text-slate-800">{f.title}</div>
              <div className="text-[10px] text-slate-400 mt-1 leading-relaxed">{f.desc}</div>
            </div>
          );
        })}
      </div>

      {/* 近 7 天触发统计 */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-5">
        <KpiCard label="近 7 天触发" value="227" color="primary" note="跨 11 条流水线触发" />
        <KpiCard label="门禁阻断" value="12" color="warning" note="阻断发布 / 合并 5 次" />
        <KpiCard label="平均判定" value="4.2m" color="primary" note="触发 → 门禁判定时长" />
        <KpiCard label="回写成功率" value="96.5%" color="success" note="门禁结果回调 CI 成功" />
      </div>

      <div className="grid grid-cols-3 gap-5">
        {/* 连接器概览 */}
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <div className="flex items-center justify-between mb-3">
            <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><Plug className="w-4 h-4 text-emerald-500" />CI 连接器</h2>
            <Link to="/cicd-connector" className="text-[10px] text-emerald-600 hover:underline flex items-center gap-0.5">管理<ChevronRight className="w-3 h-3" /></Link>
          </div>
          <div className="space-y-2.5">
            {CICD_CONNECTORS.map((c) => (
              <div key={c.id} className="flex items-center gap-2.5 border border-slate-100 rounded-lg px-3 py-2">
                <div className={'w-2 h-2 rounded-full flex-shrink-0 ' + (c.status === 'connected' ? 'bg-emerald-500' : 'bg-slate-300')} />
                <div className="flex-1 min-w-0">
                  <div className="text-xs font-medium text-slate-700 truncate">{c.name}</div>
                  <div className="text-[9px] text-slate-400">{c.mode}</div>
                </div>
                <span className="text-[10px] text-slate-500 flex-shrink-0">{c.projects} 工程</span>
              </div>
            ))}
          </div>
        </div>

        {/* 运行记录 */}
        <div className="card bg-white rounded-xl border border-slate-200 col-span-2 overflow-hidden">
          <div className="px-5 py-4 border-b border-slate-100 flex items-center justify-between">
            <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><Activity className="w-4 h-4 text-emerald-500" />CI 集成运行记录</h2>
            <span className="text-[11px] text-slate-400">commit → 测试 → 门禁 → 回写</span>
          </div>
          <div className="flex items-center justify-between px-5 pt-3">
            <ListFilter search={q} onSearch={setQ} />
            <span className="text-[11px] text-slate-400">共 {runs.length} / {CICD_RUNS.length} 条</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-[10px] text-slate-400 border-b border-slate-100">
                  <th className="px-5 py-2.5 font-medium">运行 ID</th>
                  <th className="px-2 py-2.5 font-medium">被测对象</th>
                  <th className="px-2 py-2.5 font-medium">Commit</th>
                  <th className="px-2 py-2.5 font-medium">分支 / 环境</th>
                  <th className="px-2 py-2.5 font-medium">状态</th>
                  <th className="px-2 py-2.5 font-medium">门禁</th>
                  <th className="px-2 py-2.5 font-medium">耗时</th>
                  <th className="px-2 py-2.5 font-medium">时间</th>
                </tr>
              </thead>
              <tbody>
                {runs.map((r) => {
                  const st = RUN_STATUS[r.status];
                  const Icon = st.icon;
                  return (
                    <tr key={r.id} className="border-b border-slate-50 hover:bg-slate-50/60">
                      <td className="px-5 py-3 font-mono text-slate-600">{r.id}</td>
                      <td className="px-2 py-3 text-slate-700">{r.asset}</td>
                      <td className="px-2 py-3 font-mono text-slate-500">{r.commit}</td>
                      <td className="px-2 py-3 text-slate-500">{r.branch}<span className="text-slate-300"> · </span>{r.env}</td>
                      <td className="px-2 py-3"><span className={'inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] ' + st.cls}><Icon className="w-3 h-3" />{st.label}</span></td>
                      <td className="px-2 py-3 text-slate-600">{r.gate}</td>
                      <td className="px-2 py-3 text-slate-500"><span className="inline-flex items-center gap-1"><Clock className="w-3 h-3 text-slate-400" />{r.duration}</span></td>
                      <td className="px-2 py-3 text-slate-400">{r.time}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}

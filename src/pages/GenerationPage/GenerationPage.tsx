import { UPSTREAM_ADAPTERS, GENERATION_STAGES, ADAPTER_SEED_TOTAL } from '@/data/mock';
import { PageHeader, GhostButton, PrimaryButton, Card } from '@/components/shared';
import { BrainCircuit, Sparkles } from 'lucide-react';

const STAGE_ICON: Record<string, React.ReactNode> = {
  source: '◉',
  adapter: '⚙',
  seed: '◈',
  quality: '✓',
  review: '👤',
  storage: '▤',
};
const STAGE_COLOR: Record<string, string> = {
  source: 'bg-indigo-50 border-indigo-200 text-indigo-600',
  adapter: 'bg-indigo-50 border-indigo-200 text-indigo-600',
  seed: 'bg-amber-50 border-amber-200 text-amber-600',
  quality: 'bg-emerald-50 border-emerald-200 text-emerald-600',
  review: 'bg-purple-50 border-purple-200 text-purple-600',
  storage: 'bg-emerald-50 border-emerald-200 text-emerald-600',
};

const QUALITY = [
  { label: '变异测试验证', val: '38 / 48', width: 79, color: 'bg-emerald-500', note: '10 条未通过，无法杀死代码变体' },
  { label: '断言强度检查', val: '42 / 48', width: 87, color: 'bg-emerald-500', note: '6 条存在弱断言嫌疑' },
  { label: '重复检测', val: '46 / 48', width: 96, color: 'bg-emerald-500', note: '2 条与已有用例语义重复' },
  { label: '追溯绑定', val: '48 / 48', width: 100, color: 'bg-emerald-500', note: '全部绑定上游源' },
];

// AI 决策痕迹：展示 AI 如何理解上游源并产出用例种子
const AI_TRACES = [
  { step: '解析', desc: 'AI 读取 OpenAPI /v2/refund 变更 diff，提取新增字段 refund_id、移除 status', out: '识别 1 处破坏性变更' },
  { step: '生成', desc: 'AI 依据契约 + 历史缺陷模式，生成 12 条用例种子覆盖正/反/边界路径', out: '12 条种子' },
  { step: '筛选', desc: 'AI 计算种子断言强度，剔除 6 条弱断言嫌疑，保留强断言', out: '保留 12 条' },
  { step: '绑定', desc: 'AI 为每条种子建立 REQ→测试点→用例 追溯链', out: '追溯完整' },
];

export default function GenerationPage() {
  return (
    <div>
      <PageHeader title="用例生成管道" desc="六类上游源 · AI 理解 · 质量验证 · 人工审核">
        <GhostButton>配置适配器</GhostButton>
        <PrimaryButton>手动触发同步</PrimaryButton>
      </PageHeader>

      <Card title="生成管道 · 当前批次 #GEN-2041" className="p-5 mb-5">
        <div className="flex items-center justify-between">
          {GENERATION_STAGES.map((s, i) => (
            <div key={s.label} className="flex flex-1">
              <div className="flex-1 text-center">
                <div className={'w-12 h-12 rounded-full border flex items-center justify-center mx-auto mb-2 text-lg ' + STAGE_COLOR[s.icon]}>
                  {STAGE_ICON[s.icon]}
                </div>
                <div className="text-xs font-medium text-slate-700">{s.label}</div>
                <div className="text-[10px] text-slate-400 mt-0.5">{s.meta}</div>
              </div>
              {i < GENERATION_STAGES.length - 1 && (
                <div className="flex justify-center items-center text-slate-300 px-2">→</div>
              )}
            </div>
          ))}
        </div>
      </Card>

      {/* AI 决策痕迹：把 AI 驱动从角落拉到主线 */}
      <div className="card bg-gradient-to-r from-emerald-600 to-teal-700 rounded-xl border border-transparent p-5 mb-5 text-white">
        <div className="flex items-center gap-2 mb-4">
          <BrainCircuit className="w-5 h-5" />
          <h2 className="font-semibold text-sm">AI 决策痕迹 · 本批 {ADAPTER_SEED_TOTAL} 条种子如何产生</h2>
          <span className="ml-auto text-[10px] bg-white/10 px-2 py-1 rounded flex items-center gap-1"><Sparkles className="w-3 h-3" />ai-agent-3</span>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {AI_TRACES.map((t, i) => (
            <div key={t.step} className="bg-white/10 rounded-lg p-3 relative">
              <div className="flex items-center gap-2 mb-1.5">
                <span className="w-5 h-5 rounded-full bg-white/20 flex items-center justify-center text-[10px] font-bold">{i + 1}</span>
                <span className="text-sm font-medium">{t.step}</span>
              </div>
              <div className="text-[10px] text-emerald-100 leading-relaxed">{t.desc}</div>
              <div className="mt-2 text-[10px] text-amber-200 font-medium flex items-center gap-1">
                <Sparkles className="w-3 h-3" />{t.out}
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-3 gap-5">
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-slate-200">
            <h2 className="font-semibold text-slate-700 text-sm">上游源适配器</h2>
          </div>
          <table className="w-full text-xs">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-slate-500">
                <th className="px-5 py-2.5 font-medium">上游源</th>
                <th className="px-5 py-2.5 font-medium">系统</th>
                <th className="px-5 py-2.5 font-medium">提取信息</th>
                <th className="px-5 py-2.5 font-medium">本次种子</th>
                <th className="px-5 py-2.5 font-medium">状态</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {UPSTREAM_ADAPTERS.map((a) => (
                <tr key={a.source} className={a.status === '验证中' ? 'bg-indigo-50/30 hover:bg-indigo-50' : 'hover:bg-slate-50'}>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2">
                      <span className="w-5 h-5 rounded bg-slate-100 text-slate-600 flex items-center justify-center text-[10px]">源</span>
                      <span className="text-slate-700 font-medium">{a.source}</span>
                    </div>
                  </td>
                  <td className="px-5 py-3 text-slate-500">{a.system}</td>
                  <td className="px-5 py-3 text-slate-500">{a.extract}</td>
                  <td className="px-5 py-3 text-slate-600">{a.seeds}</td>
                  <td className="px-5 py-3">
                    <span className={a.status === '验证中' ? 'bg-amber-50 text-amber-600 px-2 py-0.5 rounded-full text-[10px]' : 'bg-emerald-50 text-emerald-600 px-2 py-0.5 rounded-full text-[10px]'}>
                      {a.status}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <Card title="质量验证结果" className="p-5">
          <div className="space-y-3.5">
            {QUALITY.map((q) => (
              <div key={q.label}>
                <div className="flex justify-between text-xs mb-1.5">
                  <span className="text-slate-600">{q.label}</span>
                  <span className="text-slate-500">{q.val}</span>
                </div>
                <div className="h-2 bg-slate-100 rounded-full">
                  <div className={q.color + ' h-full rounded-full'} style={{ width: q.width + '%' }} />
                </div>
                <div className="text-[10px] text-slate-400 mt-1">{q.note}</div>
              </div>
            ))}
          </div>
        </Card>
      </div>
    </div>
  );
}

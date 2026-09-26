import { useState } from 'react';
import { toast } from 'sonner';
import { AI_MODELS, PROJECT_MODELS, type IAIModel } from '@/data/mock';
import { PageHeader, PrimaryButton, GhostButton } from '@/components/shared';
import { Sparkles, Boxes, Cpu, Star, Check } from 'lucide-react';
import { cn } from '@/lib/utils';

const MODEL_COLOR: Record<string, string> = {
  'doubao-1.5-pro': 'bg-orange-500',
  'deepseek-v3': 'bg-blue-500',
  'gpt-4o': 'bg-emerald-500',
  'claude-3.5': 'bg-amber-500',
  'self-llama3': 'bg-purple-500',
};

export default function ModelPage() {
  const [selectedModel, setSelectedModel] = useState<IAIModel>(AI_MODELS[0]);

  const handleBind = (projectId: string, modelId: string) => {
    const m = AI_MODELS.find((x) => x.id === modelId);
    const p = PROJECT_MODELS.find((x) => x.projectId === projectId);
    toast.success(`工程 ${p?.projectName} 已切换至「${m?.name}」`, { description: '原型示意：该工程后续的用例生成 / 质量判定将调用所选模型' });
  };

  const handleAddModel = () => {
    toast('接入模型', { description: '通过 API Key 或私有化部署接入新的 AI 模型到模型池（原型示意）' });
  };

  return (
    <div>
      <PageHeader title="AI 模型配置" desc="模型池管理 · 每个工程独立选择测试所用 AI 模型 · 团队级可用">
        <GhostButton onClick={handleAddModel}>接入模型</GhostButton>
        <PrimaryButton onClick={() => setSelectedModel(AI_MODELS[0])}><span className="flex items-center gap-1"><Sparkles className="w-4 h-4" />设为默认</span></PrimaryButton>
      </PageHeader>

      {/* 模型池 */}
      <div className="mb-5">
        <div className="flex items-center justify-between mb-3">
          <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><Cpu className="w-4 h-4 text-emerald-500" />模型池（团队可用）</h2>
          <span className="text-[11px] text-slate-400">{AI_MODELS.length} 个模型</span>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
          {AI_MODELS.map((m) => {
            const selected = selectedModel.id === m.id;
            return (
              <button key={m.id} type="button" onClick={() => setSelectedModel(m)}
                className={cn('card bg-white rounded-xl border p-4 text-left transition',
                  selected ? 'border-emerald-400 ring-2 ring-emerald-100' : 'border-slate-200 hover:border-emerald-300')}>
                <div className="flex items-center justify-between mb-2">
                  <div className={cn('w-7 h-7 rounded-lg flex items-center justify-center text-white text-[10px] font-bold', MODEL_COLOR[m.id])}>
                    {m.name.slice(0, 1)}
                  </div>
                  {m.isDefault && <span className="text-[9px] bg-emerald-50 text-emerald-600 px-1.5 py-0.5 rounded-full flex items-center gap-0.5"><Star className="w-2.5 h-2.5" />默认</span>}
                </div>
                <div className="text-sm font-semibold text-slate-800">{m.name}</div>
                <div className="text-[10px] text-slate-400">{m.vendor}</div>
                <div className="text-[10px] text-slate-500 mt-2">{m.capability}</div>
                <div className="mt-2 flex flex-wrap gap-1">
                  {m.tags.map((t) => <span key={t} className="text-[9px] bg-slate-100 text-slate-500 px-1.5 py-0.5 rounded">{t}</span>)}
                </div>
                <div className="mt-2 flex items-center justify-between text-[9px] text-slate-400">
                  <span>成本 {m.cost}</span><span>时延 {m.latency}</span>
                </div>
              </button>
            );
          })}
        </div>
      </div>

      {/* 当前所选模型详情 */}
      <div className="card bg-gradient-to-br from-emerald-600 to-teal-700 rounded-2xl border border-transparent p-5 mb-5 text-white">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-4">
            <div className={cn('w-11 h-11 rounded-xl flex items-center justify-center text-white text-base font-bold', MODEL_COLOR[selectedModel.id])}>
              {selectedModel.name.slice(0, 1)}
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="text-lg font-bold">{selectedModel.name}</span>
                {selectedModel.isDefault && <span className="text-[10px] bg-white/15 px-2 py-0.5 rounded-full">团队默认</span>}
              </div>
              <div className="text-[11px] text-emerald-100 mt-0.5">{selectedModel.vendor} · {selectedModel.capability}</div>
            </div>
          </div>
          <div className="flex items-center gap-5 text-center">
            <div><div className="text-sm font-bold">{selectedModel.cost}</div><div className="text-[10px] text-emerald-100">调用成本</div></div>
            <div><div className="text-sm font-bold">{selectedModel.latency}</div><div className="text-[10px] text-emerald-100">平均时延</div></div>
            <div><div className="text-sm font-bold">企业级</div><div className="text-[10px] text-emerald-100">合规审批</div></div>
          </div>
        </div>
      </div>

      {/* 每工程模型绑定 */}
      <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
        <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
          <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><Boxes className="w-4 h-4 text-emerald-500" />每工程模型绑定</h2>
          <span className="text-[11px] text-slate-400">切换后仅对当前工程生效 · 覆盖默认模型</span>
        </div>
        <table className="w-full text-sm">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-xs text-slate-500">
              <th className="px-5 py-2.5 font-medium">工程</th>
              <th className="px-5 py-2.5 font-medium">当前所选模型</th>
              <th className="px-5 py-2.5 font-medium">覆盖率</th>
              <th className="px-5 py-2.5 font-medium">切换模型</th>
              <th className="px-5 py-2.5 font-medium">生效</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 text-xs">
            {PROJECT_MODELS.map((p) => {
              const bound = AI_MODELS.find((m) => m.id === p.modelId);
              return (
                <tr key={p.projectId} className="hover:bg-slate-50">
                  <td className="px-5 py-3">
                    <div className="text-slate-700 font-medium flex items-center gap-2">
                      <span className={cn('w-2 h-2 rounded-full', bound && MODEL_COLOR[p.modelId])} />
                      {p.projectName}
                    </div>
                  </td>
                  <td className="px-5 py-3">
                    <span className="text-slate-600">{bound?.name}</span>
                    {bound?.isDefault && <span className="ml-2 text-[9px] bg-emerald-50 text-emerald-600 px-1.5 py-0.5 rounded-full">默认</span>}
                  </td>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2">
                      <div className="w-16 h-1.5 bg-slate-100 rounded-full overflow-hidden">
                        <div className={cn('h-full rounded-full', p.coverage >= 70 ? 'bg-emerald-500' : 'bg-amber-500')} style={{ width: `${p.coverage}%` }} />
                      </div>
                      <span className="text-slate-500">{p.coverage}%</span>
                    </div>
                  </td>
                  <td className="px-5 py-3">
                    <select defaultValue={p.modelId} onChange={(e) => handleBind(p.projectId, e.target.value)}
                      className="border border-slate-200 rounded-lg px-2 py-1 text-[11px] outline-none focus:border-emerald-300">
                      {AI_MODELS.map((m) => <option key={m.id} value={m.id}>{m.name}</option>)}
                    </select>
                  </td>
                  <td className="px-5 py-3"><span className="text-emerald-600"><Check className="w-3.5 h-3.5" /></span></td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

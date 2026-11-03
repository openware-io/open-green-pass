import { Layers } from 'lucide-react';
import { Card } from '@/components/shared';
import { SCOPE_LABEL, type GenerationScope } from './types';

const SCOPE_OPTIONS: GenerationScope[] = ['project', 'repo', 'service'];

const SCOPE_HINT: Record<GenerationScope, string> = {
  project: '项目全量：包含当前项目全部已确认服务，每个仓库单独选择分支或 Tag。',
  repo: '仓库全量：只生成选中仓库下的已确认服务，其他仓库不受影响。',
  service: '指定服务：自行勾选服务，适合局部复测；每个服务按所在仓库的分支或 Tag 生成。',
};

export type ScopeRepoOption = { id: number; url: string; defaultBranch: string; serviceCount: number };

export type ScopeConfigProps = {
  scope: GenerationScope;
  onScopeChange: (scope: GenerationScope) => void;
  repoOptions: ScopeRepoOption[];
  activeRepoId: number | null;
  onRepoChange: (repoId: number) => void;
  serviceCount: number;
  repoCount: number;
  confirmedCount: number;
  disabled?: boolean;
};

/** Step 1: the only place that edits the generation scope. */
export function ScopeConfig({ scope, onScopeChange, repoOptions, activeRepoId, onRepoChange, serviceCount, repoCount, confirmedCount, disabled }: ScopeConfigProps) {
  return (
    <Card
      title={<span className="inline-flex items-center gap-2"><Layers className="h-4 w-4 text-emerald-600" />范围与引用</span>}
      className="mb-5 p-5"
      extra={<span className="text-[11px] text-slate-400">唯一配置源 · 修改后需重新预检</span>}
    >
      <div role="tablist" aria-label="生成范围" className="inline-flex rounded-xl border border-slate-200 bg-slate-50 p-1">
        {SCOPE_OPTIONS.map((option) => (
          <button
            key={option}
            type="button"
            role="tab"
            aria-selected={scope === option}
            disabled={disabled}
            onClick={() => onScopeChange(option)}
            className={'rounded-lg px-3 py-2 text-xs font-semibold transition disabled:opacity-50 ' + (scope === option ? 'bg-white text-emerald-700 shadow-sm' : 'text-slate-500 hover:text-slate-700')}
          >
            {SCOPE_LABEL[option]}
          </button>
        ))}
      </div>

      {scope === 'repo' && (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <label htmlFor="generation-scope-repo" className="text-xs text-slate-500">仓库</label>
          <select
            id="generation-scope-repo"
            value={activeRepoId ?? ''}
            disabled={disabled || !repoOptions.length}
            onChange={(event) => onRepoChange(Number(event.target.value))}
            className="min-w-[280px] flex-1 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-700 outline-none focus:border-emerald-400 disabled:opacity-50"
          >
            {repoOptions.map((repo) => <option key={repo.id} value={repo.id}>{repo.url} · {repo.serviceCount} 个服务</option>)}
          </select>
        </div>
      )}

      <p className="mt-3 text-xs text-slate-500">{SCOPE_HINT[scope]}</p>

      <div className="mt-3 flex flex-wrap gap-2 text-[10px] text-slate-600">
        <span className="rounded bg-slate-50 px-2 py-1">范围：{SCOPE_LABEL[scope]}</span>
        <span className="rounded bg-slate-50 px-2 py-1">服务：{serviceCount} / 已确认 {confirmedCount}</span>
        <span className="rounded bg-slate-50 px-2 py-1">仓库：{repoCount}</span>
        <span className="rounded bg-slate-50 px-2 py-1">每服务一个生成批次</span>
      </div>

      <p className="mt-3 text-[11px] text-slate-400">通过内部 DSH SDK gateway 生成用例；模型由 DSH Profile 配置，不使用项目级模型绑定。</p>
    </Card>
  );
}

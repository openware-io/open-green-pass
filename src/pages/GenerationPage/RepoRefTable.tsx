import { RefreshCw } from 'lucide-react';
import type { RepositoryRef } from '@/api/client';
import type { CatalogRepository } from '@/context/objectCatalog';
import type { DraftRef } from './types';

export type RepoRefRow = {
  repo: CatalogRepository;
  serviceCount: number;
  refs: RepositoryRef[];
  selected: DraftRef | null;
  loading: boolean;
};

export type RepoRefTableProps = {
  rows: RepoRefRow[];
  disabled?: boolean;
  onChange: (repoId: number, ref: DraftRef) => void;
  onRefresh: (repoId: number) => void;
  onRefreshAll?: () => void;
  refreshingAll?: boolean;
};

/** The one and only branch/Tag selector for generation references. */
function CodeRefSelect({ refs, value, disabled, onChange, label }: {
  refs: RepositoryRef[];
  value: DraftRef | null;
  disabled?: boolean;
  onChange: (next: DraftRef) => void;
  label: string;
}) {
  const type = value?.type ?? 'branch';
  const options = refs.filter((ref) => ref.ref_type === type);
  return (
    <div className="flex min-w-0 gap-2">
      <select
        aria-label={`${label}类型`}
        value={type}
        disabled={disabled || !refs.length}
        onChange={(event) => {
          const nextType = event.target.value as 'branch' | 'tag';
          const first = refs.find((ref) => ref.ref_type === nextType);
          onChange({ type: nextType, name: first?.name ?? '' });
        }}
        className="rounded-md border border-slate-200 bg-white px-2 py-1.5 text-xs text-slate-700 disabled:opacity-50"
      >
        <option value="branch">分支</option>
        <option value="tag">Tag</option>
      </select>
      <select
        aria-label={label}
        value={value?.name ?? ''}
        disabled={disabled || !options.length}
        onChange={(event) => onChange({ type, name: event.target.value })}
        className="min-w-0 flex-1 rounded-md border border-slate-200 bg-white px-2 py-1.5 font-mono text-xs text-slate-700 disabled:opacity-50"
      >
        <option value="">{refs.length ? '请选择代码引用' : '尚未同步，请刷新'}</option>
        {options.map((ref) => <option key={`${ref.ref_type}:${ref.name}`} value={ref.name}>{ref.name}</option>)}
      </select>
    </div>
  );
}

export function RepoRefTable({ rows, disabled, onChange, onRefresh, onRefreshAll, refreshingAll }: RepoRefTableProps) {
  if (!rows.length) {
    return <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] text-amber-800">当前范围没有已关联仓库的已确认服务。</div>;
  }
  return (
    <section aria-label="代码引用" className="mt-4">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div>
          <div className="text-xs font-semibold text-slate-700">代码引用</div>
          <div className="mt-0.5 text-[10px] text-slate-400">每个仓库独立选择分支或 Tag；创建后服务端冻结对应提交 SHA。</div>
        </div>
        {onRefreshAll && (
          <button
            type="button"
            disabled={refreshingAll}
            onClick={onRefreshAll}
            className="inline-flex items-center gap-1 rounded border border-emerald-200 bg-white px-2 py-1 text-[10px] text-emerald-700 hover:bg-emerald-50 disabled:opacity-50"
          >
            <RefreshCw className={'h-3 w-3 ' + (refreshingAll ? 'animate-spin' : '')} />{refreshingAll ? '同步中…' : '刷新全部'}
          </button>
        )}
      </div>
      <div className="space-y-2">
        {rows.map(({ repo, serviceCount, refs, selected, loading }) => {
          const detail = refs.find((ref) => ref.ref_type === selected?.type && ref.name === selected.name);
          return (
            <div key={repo.id} className="flex flex-wrap items-center gap-3 rounded-lg border border-slate-200 bg-white px-3 py-2">
              <div className="min-w-0 flex-1">
                <div className="truncate text-xs font-medium text-slate-700" title={repo.url}>{repo.url}</div>
                <div className="mt-0.5 text-[10px] text-slate-400">
                  默认分支 {repo.default_branch} · {serviceCount} 个服务
                  {detail?.commit_sha && <span className="ml-2 font-mono text-slate-500">@{detail.commit_sha.slice(0, 12)}</span>}
                  {detail?.fetched_at && <span className="ml-2">引用同步于 {new Date(detail.fetched_at).toLocaleString('zh-CN', { hour12: false })}</span>}
                </div>
              </div>
              <div className="flex items-center gap-2">
                {refs.length
                  ? <CodeRefSelect label={`${repo.url} 代码引用`} refs={refs} value={selected} disabled={disabled || loading} onChange={(next) => onChange(repo.id, next)} />
                  : <span className="text-[10px] text-amber-700">{loading ? '正在同步…' : '尚未同步'}</span>}
                <button
                  type="button"
                  disabled={loading}
                  onClick={() => onRefresh(repo.id)}
                  className="inline-flex items-center gap-1 rounded border border-slate-200 bg-white px-2 py-1 text-[10px] text-slate-600 hover:bg-slate-50 disabled:opacity-50"
                >
                  <RefreshCw className={'h-3 w-3 ' + (loading ? 'animate-spin' : '')} />{loading ? '同步中…' : '刷新'}
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

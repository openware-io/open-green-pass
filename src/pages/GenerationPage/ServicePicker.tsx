import { useMemo, useState } from 'react';
import { Search } from 'lucide-react';
import type { CatalogRepository, CatalogService } from '@/context/objectCatalog';
import { sourceDirectoryLabel } from './types';

export type ServicePickerProps = {
  services: CatalogService[];
  selectedIds: number[];
  repos: CatalogRepository[];
  /** project/repo scope: the list is informative, selection is fixed by scope. */
  readOnly: boolean;
  disabled?: boolean;
  onToggle: (serviceId: number) => void;
  onSelectAll: () => void;
  onClear: () => void;
};

/** Single service list used by every scope; never renders a second selector. */
export function ServicePicker({ services, selectedIds, repos, readOnly, disabled, onToggle, onSelectAll, onClear }: ServicePickerProps) {
  const [search, setSearch] = useState('');
  const keyword = search.trim().toLowerCase();
  const visible = useMemo(() => services.filter((service) => {
    if (!keyword) return true;
    return `${service.name} ${service.source_path} ${service.kind}`.toLowerCase().includes(keyword);
  }), [keyword, services]);
  const selected = useMemo(() => new Set(readOnly ? services.map((service) => service.id) : selectedIds), [readOnly, selectedIds, services]);
  const repoById = useMemo(() => new Map(repos.map((repo) => [repo.id, repo])), [repos]);

  return (
    <section aria-label="服务选择" className="mt-4 rounded-xl border border-slate-200">
      <div className="flex flex-wrap items-center gap-3 border-b border-slate-100 px-4 py-3">
        <div className="min-w-[220px] flex-1">
          <div className="text-sm font-semibold text-slate-700">{readOnly ? '范围内服务' : '选择服务'}</div>
          <div className="mt-0.5 text-[11px] text-slate-400">
            {readOnly ? '范围模式已固定服务集合；切换到“指定服务”可单独勾选。' : '仅列出技术画像与生成开关均已确认的服务。'}
          </div>
        </div>
        <span className="rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700">已选 {selected.size} / {services.length}</span>
        {!readOnly && (
          <>
            <button type="button" disabled={disabled || !services.length} onClick={onSelectAll} className="text-xs text-emerald-700 hover:underline disabled:opacity-50">全选</button>
            <button type="button" disabled={disabled || !selected.size} onClick={onClear} className="text-xs text-slate-500 hover:underline disabled:opacity-50">清空</button>
          </>
        )}
        <div className="relative w-full sm:w-56">
          <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="搜索服务或源码目录"
            aria-label="搜索服务或源码目录"
            className="w-full rounded-lg border border-slate-200 py-2 pl-8 pr-3 text-xs outline-none focus:border-emerald-400"
          />
        </div>
      </div>
      <div className="max-h-[330px] divide-y divide-slate-100 overflow-y-auto">
        {visible.length === 0
          ? <div className="p-8 text-center text-xs text-slate-500">暂无符合条件的已确认服务</div>
          : visible.map((service) => {
            const checked = selected.has(service.id);
            const repo = repoById.get(service.repo_id);
            return (
              <label key={service.id} className={'flex items-start gap-3 px-4 py-3 transition ' + (readOnly ? 'cursor-default' : 'cursor-pointer') + (checked ? ' bg-emerald-50/60' : ' hover:bg-slate-50')}>
                <input
                  type="checkbox"
                  checked={checked}
                  disabled={readOnly || disabled}
                  onChange={() => onToggle(service.id)}
                  className="mt-1 h-4 w-4 accent-emerald-600"
                  aria-label={`选择服务 ${service.name}`}
                />
                <span className="min-w-0 flex-1">
                  <span className="flex flex-wrap items-center gap-2 text-xs font-semibold text-slate-700">
                    <span>{service.name}</span>
                    <span className="rounded bg-emerald-50 px-1.5 py-0.5 text-[10px] font-medium text-emerald-700">生成测试用例：是</span>
                    <span className="text-[10px] font-normal text-slate-400">{service.kind}</span>
                  </span>
                  <span className="mt-1 block truncate text-[11px] text-slate-500">{sourceDirectoryLabel(service.source_path)}</span>
                  <span className="mt-1 block text-[10px] text-slate-400">
                    语言：{service.language || '未登记'} · 框架：{service.framework || '未登记'} · 工具：{service.test_tool || '未登记'}
                  </span>
                </span>
                <span className="hidden min-w-[180px] text-right text-[10px] text-slate-400 sm:block">
                  {repo?.url ?? '未关联仓库'}<br /><b className="font-normal text-slate-500">{repo?.default_branch ?? '未配置分支'}</b>
                </span>
              </label>
            );
          })}
      </div>
    </section>
  );
}

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { AlertTriangle, ArrowLeft, ArrowRight, ChevronRight, History, Loader2, RefreshCw, Target } from 'lucide-react';
import {
  GreenPassApiError,
  type GenerationBatch,
  type GenerationRunSummary,
} from '@/api/client';
import { configuredGreenPassClient } from '@/api/runtime';
import { loginStore } from '@/context/login';
import { useObjectCatalog } from '@/context/objectCatalog';
import { Card, PageHeader } from '@/components/shared';
import { GenerationErrorBanner, GenerationStateCard, type GenerationErrorState } from '@/pages/GenerationPage/StatusPanels';
import {
  BATCH_STAGE_LABEL,
  errorCodeOf,
  formatCost,
  formatDateTime,
  formatTokens,
  isActiveRunState,
  RUN_STATE_LABEL,
  SCOPE_LABEL,
} from '@/pages/GenerationPage/types';
import {
  filterRunPageByState,
  hasNextPage,
  HISTORY_PAGE_SIZE,
  mergeRunPages,
  parseHistoryStateFilter,
  type HistoryStateFilter,
} from './historyPagination';

const STATE_FILTERS: Array<{ value: HistoryStateFilter; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'queued', label: '排队中' },
  { value: 'running', label: '生成中' },
  { value: 'waiting_review', label: '等待审核' },
  { value: 'partial_failed', label: '部分失败' },
  { value: 'succeeded', label: '已完成' },
  { value: 'failed', label: '失败' },
  { value: 'cancelled', label: '已取消' },
];

function runBadgeClass(state: GenerationRunSummary['state']): string {
  if (state === 'succeeded') return 'bg-emerald-50 text-emerald-700';
  if (state === 'failed') return 'bg-red-50 text-red-600';
  if (state === 'partial_failed' || state === 'waiting_review') return 'bg-amber-50 text-amber-700';
  if (state === 'cancelled') return 'bg-slate-200 text-slate-500';
  return 'bg-blue-50 text-blue-700';
}

function batchBadgeClass(state: GenerationBatch['state']): string {
  if (state === 'approved') return 'bg-emerald-50 text-emerald-700';
  if (state === 'failed' || state === 'rejected' || state === 'rolled_back') return 'bg-red-50 text-red-600';
  if (state === 'review') return 'bg-amber-50 text-amber-700';
  return 'bg-slate-100 text-slate-600';
}

/**
 * Dedicated history route: historical runs/batches are kept out of the new
 * generation flow.
 *
 * Runs use the backend cursor pagination (`GET /generation/runs?limit&cursor`
 * + optional server-side `state`); the UI appends pages via “加载更多” instead
 * of fetching everything and filtering in the browser. The selected state
 * filter is stored in the URL (`#/generation/history?state=failed`) so it is
 * deep-linkable. Batches still use the legacy non-paginated list endpoint.
 */
export default function GenerationHistoryPage() {
  const navigate = useNavigate();
  const client = useMemo(() => configuredGreenPassClient(), []);
  const catalog = useObjectCatalog();
  const projectId = Number(catalog.projectId) || 0;
  const projectName = catalog.projects.find((project) => String(project.id) === catalog.projectId)?.name ?? catalog.projectId;
  const [searchParams, setSearchParams] = useSearchParams();
  const runStateFilter = parseHistoryStateFilter(searchParams.get('state'));

  const [runs, setRuns] = useState<GenerationRunSummary[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [legacyPage, setLegacyPage] = useState(false);
  const [runsLoading, setRunsLoading] = useState(false);
  const [runsLoadingMore, setRunsLoadingMore] = useState(false);
  const [runsError, setRunsError] = useState<GenerationErrorState | null>(null);
  const [batches, setBatches] = useState<GenerationBatch[]>([]);
  const [batchesError, setBatchesError] = useState<GenerationErrorState | null>(null);
  const [batchStateFilter, setBatchStateFilter] = useState<string>('all');
  const runRequestSeq = useRef(0);

  const handleError = useCallback(async (cause: unknown, fallback: string): Promise<GenerationErrorState> => {
    if (cause instanceof GreenPassApiError) {
      const code = errorCodeOf(cause);
      if (cause.status === 401) {
        await loginStore.restore();
        if (!loginStore.get()) {
          navigate('/login', { replace: true });
          return { kind: 'auth', status: 401, message: '登录已过期，请重新登录。' };
        }
        return { kind: 'auth', status: 401, message: '登录已过期，请重新登录后刷新。' };
      }
      if (cause.status === 403) return { kind: 'permission', status: 403, code, message: cause.message || '当前账号没有查看生成历史的权限。' };
      return { kind: 'generic', status: cause.status, code, message: cause.message || fallback };
    }
    return { kind: 'generic', message: cause instanceof Error ? cause.message : fallback };
  }, [navigate]);

  const loadRunPage = useCallback(async (cursor: string | null, append: boolean) => {
    if (!client || !projectId) {
      setRuns([]);
      setNextCursor(null);
      setRunsLoading(false);
      setRunsLoadingMore(false);
      return;
    }
    const requestId = ++runRequestSeq.current;
    if (append) setRunsLoadingMore(true);
    else {
      setRunsLoading(true);
      setRuns([]);
      setNextCursor(null);
    }
    setRunsError(null);
    try {
      const page = await client.listGenerationRunPage({
        projectId,
        limit: HISTORY_PAGE_SIZE,
        cursor: append ? cursor : null,
        // The backend filters by state; the browser never screens a full list.
        state: runStateFilter === 'all' ? undefined : runStateFilter,
      });
      if (requestId !== runRequestSeq.current) return;
      // Transitional only: a pre-task-22 backend returns a bare array, so the
      // selected state filter can only be applied to that single page.
      const pageRuns = page.legacy ? filterRunPageByState(page.runs, runStateFilter) : page.runs;
      setRuns((current) => (append ? mergeRunPages(current, pageRuns) : pageRuns));
      setNextCursor(page.nextCursor);
      setLegacyPage(page.legacy);
    } catch (cause) {
      if (requestId !== runRequestSeq.current) return;
      setRunsError(await handleError(cause, '读取生成历史失败'));
      if (!append) {
        setRuns([]);
        setNextCursor(null);
      }
    } finally {
      if (requestId === runRequestSeq.current) {
        setRunsLoading(false);
        setRunsLoadingMore(false);
      }
    }
  }, [client, handleError, projectId, runStateFilter]);

  const loadBatches = useCallback(async () => {
    if (!client || !projectId) {
      setBatches([]);
      return;
    }
    try {
      const list = await client.listGenerationBatches(projectId);
      setBatches(list);
      setBatchesError(null);
    } catch (cause) {
      setBatchesError(await handleError(cause, '读取生成批次失败'));
    }
  }, [client, handleError, projectId]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-shot page load on mount/filter/project change (no polling)
    void loadRunPage(null, false);
  }, [loadRunPage]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-shot batch list load (legacy non-paginated endpoint)
    void loadBatches();
  }, [loadBatches]);

  const error = runsError ?? batchesError;
  const refresh = useCallback(() => {
    void loadRunPage(null, false);
    void loadBatches();
  }, [loadBatches, loadRunPage]);

  const changeStateFilter = useCallback((value: HistoryStateFilter) => {
    const next = new URLSearchParams(searchParams);
    if (value === 'all') next.delete('state');
    else next.set('state', value);
    setSearchParams(next, { replace: true });
  }, [searchParams, setSearchParams]);

  const visibleBatches = useMemo(() => [...batches]
    .filter((batch) => batchStateFilter === 'all' || batch.state === batchStateFilter)
    .sort((a, b) => Date.parse(b.updated_at) - Date.parse(a.updated_at)), [batches, batchStateFilter]);
  const serviceNameById = useMemo(() => new Map(catalog.services.map((service) => [service.id, service.name])), [catalog.services]);
  const serviceNameForBatch = useCallback((batch: GenerationBatch): string => {
    if (batch.service_id && serviceNameById.get(batch.service_id)) return serviceNameById.get(batch.service_id) as string;
    return batch.service_id ? `服务 #${batch.service_id}` : `仓库 #${batch.repo_id}`;
  }, [serviceNameById]);

  const activeCount = runs.filter((run) => isActiveRunState(run.state)).length;
  const waitingReviewCount = runs.filter((run) => run.state === 'waiting_review').length;
  const reviewBacklog = runs.reduce((total, run) => total + (run.review_units ?? 0), 0);
  // Copy depends on the actual backend mode: only a legacy bare-array response
  // should ever mention the legacy list interface. A cursor backend must not
  // show legacy wording anywhere (header/stats/footer).
  const pagingLabel = legacyPage ? '旧接口单页' : 'cursor 分页';
  const showEmptyState = !runsLoading && !error && runs.length === 0 && batches.length === 0 && runStateFilter === 'all';

  const openRun = (run: GenerationRunSummary) => {
    const params = new URLSearchParams({
      project_id: String(projectId),
      scope: run.scope_type,
      scope_id: String(run.scope_id),
      run_id: String(run.id),
    });
    navigate(`/generation?${params}`);
  };

  const rerun = (run: GenerationRunSummary) => {
    navigate(`/generation?project_id=${projectId}&rerun_from=${run.id}`);
  };

  if (!client) {
    return (
      <div>
        <PageHeader title="生成历史" desc="历史生成任务与批次" />
        <GenerationStateCard
          kind="unavailable"
          title="尚未配置真实 API"
          description="历史生成记录来自后端；配置 VITE_GP_API_BASE 与 VITE_GP_TEAM_ID 后可用。"
          action="返回生成用例"
          onAction={() => navigate('/generation')}
        />
      </div>
    );
  }

  return (
    <div className="pb-6">
      <div className="mb-5 flex flex-wrap items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 py-2.5 text-xs shadow-sm">
        <span className="font-semibold text-slate-700">{projectName || '当前项目'}</span>
        <span className="text-slate-300">/</span>
        <button type="button" onClick={() => navigate('/generation')} className="text-slate-500 hover:text-emerald-600">生成用例</button>
        <ChevronRight className="h-3.5 w-3.5 text-slate-300" />
        <span className="rounded-md bg-emerald-600 px-2.5 py-1 font-medium text-white">生成历史</span>
        <span className="ml-auto inline-flex items-center gap-2">
          <button type="button" onClick={() => navigate('/target')} className="inline-flex items-center gap-1 text-slate-500 hover:text-emerald-600"><Target className="h-3.5 w-3.5" />管理被测对象</button>
        </span>
      </div>

      <PageHeader
        title="生成历史"
        desc={`历史任务与批次独立于当前生成流程 · 已加载 ${runs.length} 个任务 / ${batches.length} 个批次 · ${pagingLabel}`}
      >
        <button type="button" onClick={() => navigate('/generation')} className="inline-flex items-center gap-1 rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm hover:bg-slate-50">
          <ArrowLeft className="h-3.5 w-3.5" />返回生成用例
        </button>
        <button type="button" onClick={refresh} disabled={runsLoading} className="inline-flex items-center gap-1 rounded-lg border border-emerald-200 bg-white px-3 py-1.5 text-sm text-emerald-700 hover:bg-emerald-50 disabled:opacity-50">
          <RefreshCw className={'h-3.5 w-3.5 ' + (runsLoading ? 'animate-spin' : '')} />刷新
        </button>
      </PageHeader>

      {error && <GenerationErrorBanner error={error} onRetry={refresh} />}

      {showEmptyState && (
        <GenerationStateCard
          kind="empty"
          title="还没有生成历史"
          description="创建生成任务后，历史任务与服务批次会显示在这里；当前任务仍在新生成流程的运行控制台中。"
          action="去生成用例"
          onAction={() => navigate('/generation')}
        />
      )}

      {runsLoading && runs.length === 0 && (
        <div className="mb-5 space-y-3 rounded-xl border border-slate-200 bg-white p-5" aria-busy="true" aria-label="正在读取生成历史">
          <div className="h-4 w-40 animate-pulse rounded bg-slate-100" />
          <div className="h-24 animate-pulse rounded-lg bg-slate-50" />
        </div>
      )}

      <div className="mb-5 grid gap-3 sm:grid-cols-3">
        <div className="rounded-xl border border-slate-200 bg-white px-4 py-3">
          <div className="text-[10px] text-slate-500">已加载任务</div>
          <div className="mt-1 text-2xl font-semibold text-slate-800">{runs.length}</div>
          <div className="text-[10px] text-slate-400">
            当前筛选 · 等待审核 {waitingReviewCount} · 进行中 {activeCount}
          </div>
          <div className="text-[10px] text-slate-400">{hasNextPage(nextCursor) ? '还有更多（加载更多）' : '已到末页'}</div>
        </div>
        <div className="rounded-xl border border-slate-200 bg-white px-4 py-3">
          <div className="text-[10px] text-slate-500">服务批次</div>
          <div className="mt-1 text-2xl font-semibold text-slate-800">{batches.length}</div>
          <div className="text-[10px] text-slate-400">含历史生成与待审核批次</div>
        </div>
        <div className="rounded-xl border border-slate-200 bg-white px-4 py-3">
          <div className="text-[10px] text-slate-500">审核积压（已加载）</div>
          <div className="mt-1 text-2xl font-semibold text-amber-600">{reviewBacklog}</div>
          <div className="text-[10px] text-slate-400">
            {waitingReviewCount > 0
              ? `${waitingReviewCount} 个任务生成已完成（100%），等待人工审核`
              : '待审核服务数（当前已加载任务范围内）'}
          </div>
        </div>
      </div>

      <Card
        title={<span className="inline-flex items-center gap-2"><History className="h-4 w-4 text-emerald-600" />生成任务</span>}
        className="mb-5 p-0 overflow-hidden"
        extra={
          <div className="flex flex-wrap items-center gap-3">
            <span className="text-[11px] text-slate-400">{legacyPage ? '单页列表' : `每页 ${HISTORY_PAGE_SIZE} 条 · cursor 分页`}</span>
            <label className="flex items-center gap-2 text-[11px] text-slate-500">
              状态
              <select
                value={runStateFilter}
                onChange={(event) => changeStateFilter(event.target.value as HistoryStateFilter)}
                className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-[11px] text-slate-600"
              >
                {STATE_FILTERS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select>
            </label>
          </div>
        }
      >
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <caption className="sr-only">历史生成任务列表（{pagingLabel}）</caption>
            <thead className="bg-slate-50 text-left text-slate-500">
              <tr>
                <th scope="col" className="px-4 py-2.5 font-medium">任务</th>
                <th scope="col" className="px-4 py-2.5 font-medium">范围</th>
                <th scope="col" className="px-4 py-2.5 font-medium">状态</th>
                <th scope="col" className="px-4 py-2.5 font-medium">服务</th>
                <th scope="col" className="px-4 py-2.5 font-medium">tokens / 成本</th>
                <th scope="col" className="px-4 py-2.5 font-medium">创建时间</th>
                <th scope="col" className="px-4 py-2.5 font-medium text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {runs.map((run) => (
                <tr key={run.id} className="hover:bg-slate-50">
                  <td className="px-4 py-2.5 font-mono text-emerald-700">#{run.id}</td>
                  <td className="px-4 py-2.5 text-slate-600">{SCOPE_LABEL[run.scope_type]}</td>
                  <td className="px-4 py-2.5"><span className={'rounded-full px-2 py-0.5 text-[10px] font-medium ' + runBadgeClass(run.state)}>{RUN_STATE_LABEL[run.state] ?? run.state}</span></td>
                  <td className="px-4 py-2.5 text-slate-600">
                    {run.state === 'waiting_review'
                      ? (
                        <span title="生成阶段已完成（100%），等待人工审核" className="inline-flex items-center gap-1 text-amber-700">
                          生成完成 · 待审核 {(run.review_units ?? 0) || run.total_units}
                        </span>
                      )
                      : (
                        <>
                          {run.completed_units}/{run.total_units}
                          {run.failed_units > 0 && <span className="ml-1 text-red-600">失败 {run.failed_units}</span>}
                          {(run.review_units ?? 0) > 0 && <span className="ml-1 text-amber-700">待审核 {run.review_units}</span>}
                        </>
                      )}
                  </td>
                  <td className="px-4 py-2.5 text-slate-600">{formatTokens(run)} · {formatCost(run.cost_amount)}</td>
                  <td className="px-4 py-2.5 text-slate-500">{formatDateTime(run.created_at)}</td>
                  <td className="px-4 py-2.5 text-right">
                    <button type="button" onClick={() => openRun(run)} className="inline-flex items-center gap-1 text-emerald-700 hover:underline">
                      控制台<ArrowRight className="h-3 w-3" />
                    </button>
                    <span className="mx-2 text-slate-200">|</span>
                    <button type="button" onClick={() => rerun(run)} className="text-slate-600 hover:underline">再次生成</button>
                  </td>
                </tr>
              ))}
              {runs.length === 0 && !runsLoading && (
                <tr><td colSpan={7} className="px-4 py-8 text-center text-slate-500">当前筛选条件下没有生成任务。</td></tr>
              )}
            </tbody>
          </table>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-2 border-t border-slate-100 px-4 py-3 text-[11px] text-slate-500">
          <span>
            已加载 {runs.length} 个任务
            {legacyPage && ' · 当前后端为旧列表接口（无 cursor），状态筛选仅作用于本页；后端升级后自动切换为服务端分页'}
          </span>
          {hasNextPage(nextCursor) ? (
            <button
              type="button"
              onClick={() => { if (nextCursor) void loadRunPage(nextCursor, true); }}
              disabled={runsLoadingMore}
              className="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 bg-white px-3 py-1.5 text-[11px] font-medium text-emerald-700 hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {runsLoadingMore ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
              {runsLoadingMore ? '加载中…' : '加载更多'}
            </button>
          ) : runs.length > 0 ? (
            <span className="text-slate-400">已加载全部</span>
          ) : null}
        </div>
      </Card>

      <Card
        title="服务批次"
        className="p-0 overflow-hidden"
        extra={
          <label className="flex items-center gap-2 text-[11px] text-slate-500">
            阶段
            <select
              value={batchStateFilter}
              onChange={(event) => setBatchStateFilter(event.target.value)}
              className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-[11px] text-slate-600"
            >
              <option value="all">全部</option>
              {Object.entries(BATCH_STAGE_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
          </label>
        }
      >
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <caption className="sr-only">历史生成批次列表</caption>
            <thead className="bg-slate-50 text-left text-slate-500">
              <tr>
                <th scope="col" className="px-4 py-2.5 font-medium">批次</th>
                <th scope="col" className="px-4 py-2.5 font-medium">服务</th>
                <th scope="col" className="px-4 py-2.5 font-medium">阶段</th>
                <th scope="col" className="px-4 py-2.5 font-medium">代码引用</th>
                <th scope="col" className="px-4 py-2.5 font-medium">冻结 SHA</th>
                <th scope="col" className="px-4 py-2.5 font-medium">基线</th>
                <th scope="col" className="px-4 py-2.5 font-medium">更新时间</th>
                <th scope="col" className="px-4 py-2.5 font-medium text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {visibleBatches.map((batch) => (
                <tr key={batch.id} className="hover:bg-slate-50">
                  <td className="px-4 py-2.5 font-mono text-slate-600">#{batch.id}</td>
                  <td className="px-4 py-2.5 text-slate-700">{serviceNameForBatch(batch)}</td>
                  <td className="px-4 py-2.5"><span className={'rounded-full px-2 py-0.5 text-[10px] font-medium ' + batchBadgeClass(batch.state)}>{BATCH_STAGE_LABEL[batch.state] ?? batch.state}</span></td>
                  <td className="px-4 py-2.5 font-mono text-[11px] text-slate-600">{batch.ref_type === 'tag' ? 'Tag' : '分支'} {batch.branch || '—'}</td>
                  <td className="px-4 py-2.5 font-mono text-[11px] text-slate-500">{batch.head_sha ? batch.head_sha.slice(0, 12) : '—'}</td>
                  <td className="px-4 py-2.5 text-slate-500">{batch.base_batch_id ? `#${batch.base_batch_id}` : '首次生成'}</td>
                  <td className="px-4 py-2.5 text-slate-500">{formatDateTime(batch.updated_at)}</td>
                  <td className="px-4 py-2.5 text-right">
                    {batch.state === 'approved'
                      ? <button type="button" onClick={() => navigate(`/cases?target_id=${batch.target_id}`)} className="text-emerald-700 hover:underline">查看用例</button>
                      : <button type="button" onClick={() => navigate(`/generation?project_id=${projectId}&scope=service&scope_id=${batch.service_id ?? 0}`)} className="text-slate-600 hover:underline">查看生成配置</button>}
                  </td>
                </tr>
              ))}
              {visibleBatches.length === 0 && (
                <tr><td colSpan={8} className="px-4 py-8 text-center text-slate-500">当前筛选条件下没有生成批次。</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </Card>

      <p className="mt-4 flex items-center gap-1 text-[11px] text-slate-400">
        <AlertTriangle className="h-3 w-3" />历史记录只读；“再次生成”只预填草稿，必须重新预检并确认后才能创建。筛选状态写入 URL（`?state=`），可直接分享/刷新恢复。
      </p>
    </div>
  );
}

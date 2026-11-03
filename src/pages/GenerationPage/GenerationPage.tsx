import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { ArrowRight, ChevronRight, History, RefreshCw, Target } from 'lucide-react';
import {
  GreenPassApiError,
  type GenerationPlanRequest,
  type GenerationPlanRequestUnit,
  type GenerationRunDetail,
  type RepositoryRef,
} from '@/api/client';
import { configuredGreenPassClient } from '@/api/runtime';
import { loginStore } from '@/context/login';
import { useObjectCatalog, type CatalogRepository } from '@/context/objectCatalog';
import { Card, GhostButton, PageHeader } from '@/components/shared';
import { ConfirmPlanDrawer } from './ConfirmPlanDrawer';
import { PlanPreview } from './PlanPreview';
import { RepoRefTable, type RepoRefRow } from './RepoRefTable';
import { RunConsole } from './RunConsole';
import { ScopeConfig, type ScopeRepoOption } from './ScopeConfig';
import { ServicePicker } from './ServicePicker';
import { GenerationErrorBanner, GenerationStateCard, type GenerationErrorState } from './StatusPanels';
import { useGenerationDraft } from './hooks/useGenerationDraft';
import { useGenerationPlan } from './hooks/useGenerationPlan';
import { useGenerationRunEvents } from './hooks/useGenerationRunEvents';
import {
  errorCodeOf,
  normalizeRunDetail,
  planRequestSignature,
  RUN_STATE_LABEL,
  type DraftRef,
  type GenerationScope,
} from './types';

type CreateError = { message: string; code?: string };

function createIdempotencyKey(projectId: number): string {
  const suffix = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  return `ui-run-${projectId}-${suffix}`;
}

export default function GenerationPage() {
  const navigate = useNavigate();
  const client = useMemo(() => configuredGreenPassClient(), []);
  const catalog = useObjectCatalog();
  const confirmedServices = useMemo(
    () => catalog.services.filter((service) => service.generation_status === 'confirmed' && service.profile_status === 'confirmed'),
    [catalog.services],
  );
  const projectIds = useMemo(() => catalog.projects.map((project) => String(project.id)), [catalog.projects]);
  const projectName = useMemo(
    () => catalog.projects.find((project) => String(project.id) === catalog.projectId)?.name ?? catalog.projectId,
    [catalog.projects, catalog.projectId],
  );

  const [pageError, setPageError] = useState<GenerationErrorState | null>(null);
  const [refsByRepo, setRefsByRepo] = useState<Record<number, RepositoryRef[]>>({});
  const [refsLoading, setRefsLoading] = useState<Record<number, boolean>>({});
  const loadedRepoRefs = useRef<Set<number>>(new Set());
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [createBusy, setCreateBusy] = useState(false);
  const [createError, setCreateError] = useState<CreateError | null>(null);
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const [runBusy, setRunBusy] = useState(false);
  const [refreshingAll, setRefreshingAll] = useState(false);

  /* --------------------------- central API errors --------------------------- */
  const handleApiError = useCallback(async (cause: unknown, fallback: string) => {
    if (cause instanceof GreenPassApiError) {
      const code = errorCodeOf(cause);
      if (cause.status === 401) {
        await loginStore.restore();
        if (!loginStore.get()) {
          navigate('/login', { replace: true });
          return;
        }
        setPageError({ kind: 'auth', status: 401, code, message: '登录已过期，请重新登录后重试。' });
        return;
      }
      if (cause.status === 403) {
        setPageError({
          kind: 'permission',
          status: 403,
          code,
          message: cause.message || '当前账号没有执行生成操作的权限（需要项目编辑权限）。',
        });
        return;
      }
      if (code === 'plan_stale' || code === 'idempotency_conflict') {
        setPageError({ kind: 'conflict', status: cause.status, code, message: cause.message || fallback });
        return;
      }
      setPageError({ kind: 'generic', status: cause.status, code, message: cause.message || fallback });
      return;
    }
    setPageError({ kind: 'generic', message: cause instanceof Error ? cause.message : fallback });
  }, [navigate]);

  const draft = useGenerationDraft({
    projectId: catalog.projectId,
    projectsLoaded: catalog.loaded,
    catalogLoading: catalog.loading,
    projectIds,
    services: confirmedServices,
    repos: catalog.repos,
    activeServiceId: catalog.serviceId,
    selectProject: catalog.selectProject,
  });

  /* ------------------------------ plan request ------------------------------ */
  const planUnits = useMemo<GenerationPlanRequestUnit[]>(() => draft.entries.flatMap(({ service, repo, ref }) => {
    if (!repo || !ref?.name) return [];
    return [{
      service_id: service.id,
      repo_id: repo.id,
      source_path: service.source_path,
      ref_type: ref.type,
      ref_name: ref.name,
    }];
  }), [draft.entries]);
  const scopeId = draft.draft.scope === 'project'
    ? draft.draft.projectId
    : draft.draft.scope === 'repo'
      ? draft.draft.repoId ?? 0
      : draft.draft.serviceIds[0] ?? 0;
  const planRequest = useMemo<GenerationPlanRequest | null>(() => (
    planUnits.length && draft.draft.projectId
      ? { project_id: draft.draft.projectId, scope_type: draft.draft.scope, scope_id: scopeId, units: planUnits }
      : null
  ), [draft.draft.projectId, draft.draft.scope, planUnits, scopeId]);
  const planSignature = useMemo(
    () => planRequestSignature(planUnits, draft.draft.projectId, draft.draft.scope, scopeId),
    [planUnits, draft.draft.projectId, draft.draft.scope, scopeId],
  );

  const planController = useGenerationPlan({
    client,
    request: planRequest,
    signature: planSignature,
    onApiError: (cause) => { void handleApiError(cause, '预检生成计划失败'); },
  });
  const events = useGenerationRunEvents(draft.runId, {
    client,
    onApiError: (cause) => { void handleApiError(cause, '读取生成任务失败'); },
  });
  const run = events.run;

  /* ------------------------------ repo refs ------------------------------ */
  const projectIdForRefs = draft.draft.projectId;

  const applyDefaultRefs = useCallback((repos: CatalogRepository[], refsSnapshot: Record<number, RepositoryRef[]>) => {
    const defaults: Record<number, DraftRef> = {};
    for (const repo of repos) {
      const refs = refsSnapshot[repo.id];
      if (!refs?.length) continue;
      const existing = draft.draft.refs[repo.id];
      if (existing && refs.some((ref) => ref.ref_type === existing.type && ref.name === existing.name)) continue;
      const preferred = refs.find((ref) => ref.ref_type === 'branch' && ref.name === repo.default_branch) ?? refs[0];
      if (preferred) defaults[repo.id] = { type: preferred.ref_type, name: preferred.name };
    }
    if (Object.keys(defaults).length) draft.actions.setRefs(defaults);
  }, [draft.draft.refs, draft.actions]);

  const loadRepoRefs = useCallback(async (repoId: number, force: boolean) => {
    if (!client || !projectIdForRefs || !draft.catalogProjectMatches) return [] as RepositoryRef[];
    setRefsLoading((current) => ({ ...current, [repoId]: true }));
    try {
      let refs = await client.listRepositoryRefs(projectIdForRefs, repoId);
      if (force || !refs.length) refs = await client.refreshRepositoryRefs(projectIdForRefs, repoId);
      setRefsByRepo((current) => ({ ...current, [repoId]: refs }));
      return refs;
    } catch (cause) {
      await handleApiError(cause, '读取代码引用失败');
      return [] as RepositoryRef[];
    } finally {
      setRefsLoading((current) => ({ ...current, [repoId]: false }));
    }
  }, [client, draft.catalogProjectMatches, handleApiError, projectIdForRefs]);

  useEffect(() => {
    if (!client || !projectIdForRefs || !draft.catalogProjectMatches) return;
    for (const repo of draft.scopeRepos) {
      if (refsByRepo[repo.id] !== undefined || refsLoading[repo.id] || loadedRepoRefs.current.has(repo.id)) continue;
      loadedRepoRefs.current.add(repo.id);
      void loadRepoRefs(repo.id, false);
    }
  }, [client, draft.catalogProjectMatches, draft.scopeRepos, loadRepoRefs, projectIdForRefs, refsByRepo, refsLoading]);

  useEffect(() => {
    if (!draft.catalogProjectMatches) return;
    applyDefaultRefs(draft.scopeRepos, refsByRepo);
  }, [applyDefaultRefs, draft.catalogProjectMatches, draft.scopeRepos, refsByRepo]);

  const refreshRepoRefs = useCallback(async (repoId: number) => {
    setPageError(null);
    const refs = await loadRepoRefs(repoId, true);
    if (refs.length) {
      loadedRepoRefs.current.add(repoId);
      toast.success('代码引用已刷新', {
        description: `已同步 ${refs.filter((ref) => ref.ref_type === 'branch').length} 个分支和 ${refs.filter((ref) => ref.ref_type === 'tag').length} 个 Tag`,
      });
    }
  }, [loadRepoRefs]);

  const refreshAllRepoRefs = useCallback(async () => {
    if (!draft.scopeRepos.length) return;
    setRefreshingAll(true);
    setPageError(null);
    try {
      for (const repo of draft.scopeRepos) {
        const refs = await loadRepoRefs(repo.id, true);
        if (refs.length) loadedRepoRefs.current.add(repo.id);
      }
      toast.success('代码引用已刷新', { description: `已同步 ${draft.scopeRepos.length} 个仓库的分支和 Tag` });
    } finally {
      setRefreshingAll(false);
    }
  }, [draft.scopeRepos, loadRepoRefs]);

  /* --------------------------- derived view models --------------------------- */
  const repoOptions = useMemo<ScopeRepoOption[]>(() => {
    const counts = new Map<number, number>();
    for (const service of confirmedServices) counts.set(service.repo_id, (counts.get(service.repo_id) ?? 0) + 1);
    return catalog.repos
      .filter((repo) => counts.has(repo.id))
      .map((repo) => ({ id: repo.id, url: repo.url, defaultBranch: repo.default_branch, serviceCount: counts.get(repo.id) ?? 0 }))
      .sort((a, b) => b.serviceCount - a.serviceCount || a.url.localeCompare(b.url));
  }, [catalog.repos, confirmedServices]);

  const repoRefRows = useMemo<RepoRefRow[]>(() => draft.scopeRepos.map((repo) => ({
    repo,
    serviceCount: draft.scopeServices.filter((service) => service.repo_id === repo.id).length,
    refs: refsByRepo[repo.id] ?? [],
    selected: draft.draft.refs[repo.id] ?? null,
    loading: Boolean(refsLoading[repo.id]),
  })), [draft.draft.refs, draft.scopeRepos, draft.scopeServices, refsByRepo, refsLoading]);

  const serviceNameById = useMemo(
    () => new Map(catalog.services.map((service) => [service.id, service.name])),
    [catalog.services],
  );

  const previewDisabledReason = !client
    ? '真实 API 未配置，无法预检生成计划。'
    : !planUnits.length
      ? '请选择至少一个服务，并等待其仓库的分支或 Tag 就绪。'
      : !draft.refsReady
        ? '请先为每个仓库选择分支或 Tag。'
        : undefined;

  /* ------------------------------ interactions ------------------------------ */
  const runPreview = useCallback(async () => {
    setPageError(null);
    setCreateError(null);
    const next = await planController.preview();
    if (next) {
      setIdempotencyKey(createIdempotencyKey(next.plan_id));
      setConfirmOpen(false);
    }
  }, [planController]);

  const openConfirm = useCallback(() => {
    if (!planController.plan || planController.stale || planController.plan.invalid.length > 0) return;
    setCreateError(null);
    setIdempotencyKey((current) => current || createIdempotencyKey(planController.plan?.plan_id ?? draft.draft.projectId));
    setConfirmOpen(true);
  }, [draft.draft.projectId, planController.plan, planController.stale]);

  const createRun = useCallback(async () => {
    if (!client || !planRequest || !planController.plan) return;
    const plan = planController.plan;
    const key = idempotencyKey || createIdempotencyKey(plan.plan_id);
    if (!idempotencyKey) setIdempotencyKey(key);
    setCreateBusy(true);
    setCreateError(null);
    setPageError(null);
    try {
      const created = await client.createGenerationRun({
        project_id: planRequest.project_id,
        scope_type: planRequest.scope_type,
        scope_id: planRequest.scope_id ?? 0,
        idempotency_key: key,
        plan_id: plan.plan_id,
        plan_hash: plan.plan_hash,
      });
      const normalized = normalizeRunDetail(created);
      events.setRun(normalized);
      draft.setRunId(normalized.id);
      planController.clear();
      setConfirmOpen(false);
      toast.success(`已创建生成任务 #${normalized.id}`, {
        description: `${normalized.total_units} 个服务批次已展开，运行控制台通过实时事件更新。`,
      });
    } catch (cause) {
      if (cause instanceof GreenPassApiError && (cause.status === 401 || cause.status === 403)) {
        await handleApiError(cause, '创建生成任务失败');
        return;
      }
      setCreateError({
        message: cause instanceof Error ? cause.message : '创建生成任务失败',
        code: errorCodeOf(cause),
      });
    } finally {
      setCreateBusy(false);
    }
  }, [client, draft, events, handleApiError, idempotencyKey, planController, planRequest]);

  const cancelRun = useCallback(async () => {
    if (!client || !run) return;
    setRunBusy(true);
    setPageError(null);
    try {
      const next = await client.cancelGenerationRun(run.id);
      const cancelled = normalizeRunDetail(next);
      events.setRun(cancelled);
      if (cancelled.state === 'cancelled') {
        toast.success('已取消生成任务', { description: '所属服务批次与临时数据已清理。' });
      } else {
        toast('任务已结束，无需取消', { description: `当前状态：${RUN_STATE_LABEL[cancelled.state] ?? cancelled.state}` });
      }
    } catch (cause) {
      await handleApiError(cause, '取消生成任务失败');
    } finally {
      setRunBusy(false);
    }
  }, [client, events, handleApiError, run]);

  const reviewRun = useCallback(async (decision: 'approve' | 'reject') => {
    if (!client || !run) return;
    setRunBusy(true);
    setPageError(null);
    try {
      const next = await client.reviewGenerationRun(run.id, {
        decision,
        request_id: `ui-run-review-${run.id}-${decision}-${Date.now()}`,
      });
      const reviewed = normalizeRunDetail(next);
      events.setRun(reviewed);
      if (decision === 'approve') {
        if (reviewed.state === 'succeeded') toast.success('本次任务待审核服务已全部通过并入库');
        else if (reviewed.failed_units > 0) toast.warning('部分服务审核/入库失败', { description: '请查看服务卡片失败原因；失败批次未写入用例。' });
        else toast.success('审核已提交', { description: '任务状态将通过实时事件更新。' });
      } else if (reviewed.state === 'cancelled') {
        toast.success('本次任务待审核服务已全部拒绝');
      } else {
        toast.success('已拒绝本次任务待审核服务', { description: '拒绝不会改变现有正式用例。' });
      }
    } catch (cause) {
      await handleApiError(cause, decision === 'approve' ? '批量审核失败' : '批量拒绝失败');
    } finally {
      setRunBusy(false);
    }
  }, [client, events, handleApiError, run]);

  const applyRunToDraft = useCallback((source: GenerationRunDetail) => {
    const projectId = source.project_id ?? draft.draft.projectId;
    if (!projectId) return;
    const refs: Record<number, DraftRef> = {};
    for (const unit of source.units) {
      // Legacy runs may omit the frozen ref metadata; those entries stay empty
      // so the user must pick a ref before the mandatory preflight.
      if (unit.ref_name && (unit.ref_type === 'branch' || unit.ref_type === 'tag')) {
        refs[unit.repo_id] = { type: unit.ref_type, name: unit.ref_name };
      }
    }
    draft.actions.hydrate({
      projectId,
      scope: 'service',
      repoId: null,
      serviceIds: source.units.map((unit) => unit.service_id),
      refs,
    });
    // Clear ref cache so the selectors re-load authoritative branches/Tags for
    // the prefilled repos before the next preflight.
    setRefsByRepo({});
    loadedRepoRefs.current = new Set();
    setPageError(null);
    planController.clear();
    setCreateError(null);
  }, [draft.actions, draft.draft.projectId, planController]);

  const regenerate = useCallback(() => {
    if (!run) return;
    applyRunToDraft(run);
    draft.setRunId(null);
    toast.success('已回到配置草稿', { description: '请重新预检生成计划并确认创建；不会直接复用上次参数。' });
  }, [applyRunToDraft, draft, run]);

  /* --------------------------- deep links / reset --------------------------- */
  const rerunConsumed = useRef(false);
  useEffect(() => {
    if (!client || !draft.rerunFrom || rerunConsumed.current) return;
    rerunConsumed.current = true;
    void (async () => {
      try {
        const source = normalizeRunDetail(await client.getGenerationRun(draft.rerunFrom as number));
        applyRunToDraft(source);
        draft.setRunId(null);
        toast.success('已按历史任务预填草稿', { description: '请重新预检生成计划并确认创建。' });
      } catch (cause) {
        await handleApiError(cause, '读取历史任务失败');
      }
    })();
  }, [applyRunToDraft, client, draft, handleApiError]);

  const lastProject = useRef(draft.draft.projectId);
  useEffect(() => {
    if (lastProject.current === draft.draft.projectId) return;
    lastProject.current = draft.draft.projectId;
    planController.clear();
    setConfirmOpen(false);
    setCreateError(null);
    setPageError(null);
  }, [draft.draft.projectId, planController]);

  /* --------------------------------- render --------------------------------- */
  const permissionBlocked = pageError?.kind === 'permission' || pageError?.kind === 'auth';
  const noClient = !client;
  // While the catalog is loading, or while it still belongs to another project
  // than the URL-restored draft, do not render catalog-derived selectors.
  const catalogProjectMismatch = Boolean(catalog.projectId) && draft.draft.projectId > 0 && !draft.catalogProjectMatches;
  const catalogBusy = Boolean(client) && (catalog.loading || catalogProjectMismatch);
  const hasProject = Boolean(catalog.projectId);
  const runId = draft.runId;
  const showRunConsole = Boolean(runId);

  return (
    <div className="pb-6">
      <div className="mb-5 flex flex-wrap items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 py-2.5 text-xs shadow-sm">
        <span className="font-semibold text-slate-700">{client ? projectName : '未配置真实 API'}</span>
        <span className="text-slate-300">/</span>
        <button type="button" onClick={() => navigate('/target')} className="inline-flex items-center gap-1 text-slate-500 hover:text-emerald-600"><Target className="h-3 w-3" />定义对象</button>
        <ChevronRight className="h-3.5 w-3.5 text-slate-300" />
        <span className="rounded-md bg-emerald-600 px-2.5 py-1 font-medium text-white">生成用例</span>
        <ChevronRight className="h-3.5 w-3.5 text-slate-300" />
        <button type="button" onClick={() => navigate('/trace')} className="text-slate-500 hover:text-emerald-600">追溯覆盖</button>
        <span className="ml-auto inline-flex items-center gap-2">
          <button type="button" onClick={() => navigate('/generation/history')} className="inline-flex items-center gap-1 text-slate-500 hover:text-emerald-600"><History className="h-3.5 w-3.5" />历史任务</button>
        </span>
      </div>

      <PageHeader
        title="生成用例"
        desc={client
          ? `单一配置 → 服务端预检 → 确认创建 → 实时运行控制台 · 当前项目 ${confirmedServices.length} 个已确认服务`
          : '配置真实 API 后使用生成控制台'}
      >
        <GhostButton onClick={() => navigate('/generation/history')}>
          <span className="inline-flex items-center gap-1"><History className="h-3.5 w-3.5" />生成历史</span>
        </GhostButton>
        <GhostButton onClick={() => navigate('/target')}>管理被测对象</GhostButton>
      </PageHeader>

      {pageError && <GenerationErrorBanner error={pageError} onDismiss={() => setPageError(null)} onRetry={() => setPageError(null)} />}
      {!pageError && catalog.error && (
        <GenerationErrorBanner error={{ kind: 'generic', message: catalog.error }} onRetry={() => void catalog.refresh()} />
      )}

      {noClient && (
        <GenerationStateCard
          kind="unavailable"
          title="尚未配置真实 API"
          description="生成用例需要连接 GreenPass 后端（VITE_GP_API_BASE + VITE_GP_TEAM_ID）。配置后本页提供预检、确认创建、实时运行控制台与审核。"
          action="管理被测对象"
          onAction={() => navigate('/target')}
        />
      )}

      {!noClient && permissionBlocked && (
        <GenerationStateCard
          kind={pageError?.kind === 'auth' ? 'auth' : 'permission'}
          title={pageError?.kind === 'auth' ? '登录状态已失效' : '没有生成操作权限'}
          description={pageError?.kind === 'auth'
            ? '请重新登录后刷新本页。'
            : '当前账号没有执行生成操作的权限（需要项目编辑权限）。如需权限，请联系团队 owner/admin。'}
          action={pageError?.kind === 'auth' ? '前往登录' : '返回被测对象'}
          onAction={() => navigate(pageError?.kind === 'auth' ? '/login' : '/target')}
        />
      )}

      {!noClient && !permissionBlocked && catalogBusy && (
        <div className="mb-5 space-y-3 rounded-xl border border-slate-200 bg-white p-5" aria-busy="true" aria-label="正在读取项目目录">
          <div className="h-4 w-40 animate-pulse rounded bg-slate-100" />
          <div className="h-24 animate-pulse rounded-lg bg-slate-50" />
          <div className="h-10 w-56 animate-pulse rounded-lg bg-slate-100" />
        </div>
      )}

      {!noClient && !permissionBlocked && !catalogBusy && !hasProject && (
        <GenerationStateCard
          kind="empty"
          title="还没有被测对象"
          description="生成用例以被测对象为轴心。请先在「被测对象」页创建或选择项目，并关联仓库与服务。"
          action="去定义被测对象"
          onAction={() => navigate('/target')}
        />
      )}

      {!noClient && !permissionBlocked && !catalogBusy && hasProject && confirmedServices.length === 0 && !showRunConsole && (
        <GenerationStateCard
          kind="empty"
          title="没有可生成的服务"
          description="只有技术画像与生成开关均已确认的服务才能生成用例。请前往「被测对象」页完成服务画像确认。"
          action="去被测对象页配置"
          onAction={() => navigate('/target')}
        />
      )}

      {!noClient && !permissionBlocked && !catalogBusy && hasProject && showRunConsole && (
        <RunConsole
          client={client as NonNullable<typeof client>}
          run={run}
          scopeFallback={draft.draft.scope}
          connection={events.connection}
          streamError={events.error}
          busy={runBusy}
          onRefresh={() => { void events.refresh(); }}
          onCancel={() => { void cancelRun(); }}
          onReview={(decision) => { void reviewRun(decision); }}
          onRegenerate={regenerate}
          onOpenHistory={() => navigate('/generation/history')}
        />
      )}

      {!noClient && !permissionBlocked && !catalogBusy && hasProject && confirmedServices.length > 0 && !showRunConsole && (
        <>
          <ScopeConfig
            scope={draft.draft.scope}
            onScopeChange={(scope: GenerationScope) => {
              draft.actions.setScope(scope);
              planController.clear();
              setConfirmOpen(false);
            }}
            repoOptions={repoOptions}
            activeRepoId={draft.draft.repoId}
            onRepoChange={(repoId) => { draft.actions.setRepo(repoId); }}
            serviceCount={draft.scopeServices.length}
            repoCount={draft.scopeRepos.length}
            confirmedCount={confirmedServices.length}
          />

          <Card
            title="服务与代码引用"
            className="mb-5 p-5"
            extra={
              <span className="inline-flex items-center gap-1 text-[11px] text-slate-400">
                <RefreshCw className="h-3 w-3" />引用在创建后冻结为提交 SHA
              </span>
            }
          >
            <ServicePicker
              services={draft.draft.scope === 'service' ? confirmedServices : draft.scopeServices}
              selectedIds={draft.draft.serviceIds}
              repos={catalog.repos}
              readOnly={draft.draft.scope !== 'service'}
              onToggle={(serviceId) => { draft.actions.toggleService(serviceId); }}
              onSelectAll={() => draft.actions.setServiceIds(confirmedServices.map((service) => service.id))}
              onClear={() => draft.actions.setServiceIds([])}
            />
            <RepoRefTable
              rows={repoRefRows}
              onChange={(repoId, ref) => draft.actions.setRef(repoId, ref)}
              onRefresh={(repoId) => { void refreshRepoRefs(repoId); }}
              onRefreshAll={() => { void refreshAllRepoRefs(); }}
              refreshingAll={refreshingAll}
            />
            {!draft.refsReady && (
              <p className="mt-3 text-[11px] text-amber-700" role="status">还有 {draft.missingRefRepoIds.length} 个仓库的代码引用未就绪，请选择分支或 Tag 后再预检。</p>
            )}
          </Card>

          <PlanPreview
            plan={planController.plan}
            status={planController.status}
            error={planController.error}
            errorCode={planController.errorCode}
            errorStatus={planController.errorStatus}
            stale={planController.stale}
            canPreview={Boolean(client) && planUnits.length > 0 && draft.refsReady}
            previewDisabledReason={previewDisabledReason}
            onPreview={() => { void runPreview(); }}
            onConfirm={openConfirm}
            serviceNameById={serviceNameById}
          />

          <p className="flex items-center gap-1 text-[11px] text-slate-400">
            创建后进入实时运行控制台；历史任务与当前任务分离，可在
            <button type="button" onClick={() => navigate('/generation/history')} className="inline-flex items-center gap-0.5 text-emerald-600 hover:underline">生成历史<ArrowRight className="h-3 w-3" /></button>
            查看。
          </p>
        </>
      )}

      <ConfirmPlanDrawer
        open={confirmOpen}
        plan={planController.plan}
        scope={planRequest?.scope_type ?? draft.draft.scope}
        serviceNameById={serviceNameById}
        busy={createBusy}
        error={createError?.message ?? ''}
        errorCode={createError?.code}
        onOpenChange={(open) => { if (!open) setConfirmOpen(false); }}
        onConfirm={() => { void createRun(); }}
        onRepreview={() => {
          setConfirmOpen(false);
          setCreateError(null);
          setIdempotencyKey('');
          void runPreview();
        }}
      />

      <div className="sr-only" aria-live="polite">
        {run ? `生成任务 ${run.id} 状态：${run.state}，进度 ${run.progress_percent ?? 0}%。` : ''}
      </div>
    </div>
  );
}

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from 'react';
import { useLocation, useSearchParams } from 'react-router-dom';
import type { CatalogRepository, CatalogService } from '@/context/objectCatalog';
import { draftMatchesCatalogData, shouldValidateDraftProject } from '../draftSync';
import {
  EMPTY_DRAFT,
  generationConsoleReducer,
  draftToSearchParams,
  parseGenerationSearch,
  type DraftRef,
  type GenerationConsoleState,
  type GenerationPlanDraft,
  type GenerationScope,
} from '../types';

export type DraftEntry = {
  service: CatalogService;
  repo: CatalogRepository | null;
  ref: DraftRef | null;
};

export type DraftController = {
  draft: GenerationPlanDraft;
  runId: number | null;
  setRunId: (runId: number | null) => void;
  scopeServices: CatalogService[];
  scopeRepos: CatalogRepository[];
  entries: DraftEntry[];
  /** Repos whose ref selection still needs to be loaded/chosen. */
  refsReady: boolean;
  missingRefRepoIds: number[];
  formSignature: string;
  /** True once catalog repos/services belong to the draft's project. */
  catalogProjectMatches: boolean;
  /** One-shot `rerun_from` deep link value captured before the URL is normalized. */
  rerunFrom: number | null;
  actions: {
    setScope: (scope: GenerationScope) => void;
    setRepo: (repoId: number | null) => void;
    toggleService: (serviceId: number) => void;
    setServiceIds: (serviceIds: number[]) => void;
    setRef: (repoId: number, ref: DraftRef) => void;
    setRefs: (refs: Record<number, DraftRef>) => void;
    hydrate: (draft: GenerationPlanDraft, runId?: number | null) => void;
  };
};

/**
 * Owns the single generation draft + current run id. The URL query is the
 * shareable/restorable projection of this state and also the source for
 * browser back/forward navigation.
 */
export function useGenerationDraft(options: {
  projectId: string;
  projectsLoaded: boolean;
  catalogLoading: boolean;
  projectIds: string[];
  services: CatalogService[];
  repos: CatalogRepository[];
  activeServiceId: number | null;
  selectProject: (projectId: string) => void;
}): DraftController {
  const { projectId, projectsLoaded, catalogLoading, projectIds, services, repos, activeServiceId, selectProject } = options;
  const [searchParams, setSearchParamsApi] = useSearchParams();
  const location = useLocation();
  const [state, dispatch] = useReducer(
    generationConsoleReducer,
    searchParams,
    (initial: URLSearchParams): GenerationConsoleState => parseGenerationSearch(initial, Number(projectId) || 0).state,
  );
  const [rerunFrom] = useState(() => parseGenerationSearch(searchParams, Number(projectId) || 0).rerunFrom);

  /* ----------------------------- project sync ----------------------------- */
  const initialProjectSync = useRef(false);
  const pendingProjectSync = useRef<number | null>(null);
  useEffect(() => {
    if (!projectsLoaded || initialProjectSync.current) return;
    initialProjectSync.current = true;
    const urlProject = Number(parseGenerationSearch(searchParams, 0).state.draft.projectId);
    if (urlProject && urlProject !== Number(projectId) && projectIds.includes(String(urlProject))) {
      pendingProjectSync.current = urlProject;
      selectProject(String(urlProject));
    }
  }, [projectIds, projectId, projectsLoaded, searchParams, selectProject]);

  useEffect(() => {
    if (!projectsLoaded) return;
    const catalogProject = Number(projectId) || 0;
    if (pendingProjectSync.current !== null) {
      // The URL project was requested; keep the restored draft until the header
      // context catches up instead of resetting it to the stale project.
      if (catalogProject === pendingProjectSync.current) pendingProjectSync.current = null;
      return;
    }
    if (!state.draft.projectId) {
      if (catalogProject) dispatch({ type: 'hydrate', draft: { ...EMPTY_DRAFT, projectId: catalogProject } });
      return;
    }
    if (catalogProject && catalogProject !== state.draft.projectId) {
      dispatch({ type: 'hydrate', draft: { ...EMPTY_DRAFT, projectId: catalogProject } });
    }
  }, [projectId, projectsLoaded, state.draft.projectId]);

  /* ----------------------- catalog validation / defaults ----------------------- */
  const catalogProjectId = Number(projectId) || 0;
  const catalogProjectMatches = draftMatchesCatalogData(state.draft.projectId, catalogProjectId, catalogLoading);
  useEffect(() => {
    if (!shouldValidateDraftProject({
      projectsLoaded,
      catalogLoading,
      draftProjectId: state.draft.projectId,
      catalogProjectId,
      pendingProjectSync: pendingProjectSync.current,
    })) return;
    const serviceIds = new Set(services.map((service) => service.id));
    if (state.draft.scope === 'service' && state.draft.serviceIds.some((id) => !serviceIds.has(id))) {
      dispatch({ type: 'setServiceIds', serviceIds: state.draft.serviceIds.filter((id) => serviceIds.has(id)) });
    }
    if (state.draft.scope === 'repo') {
      // Only repositories that actually contain confirmed services are valid
      // targets; never default to a repository with nothing to generate.
      const candidateRepos = repos.filter((repo) => services.some((service) => service.repo_id === repo.id));
      if (!state.draft.repoId && candidateRepos.length) dispatch({ type: 'setRepo', repoId: candidateRepos[0].id });
      else if (state.draft.repoId && !candidateRepos.some((repo) => repo.id === state.draft.repoId)) {
        dispatch({ type: 'setRepo', repoId: candidateRepos[0]?.id ?? null });
      }
    }
  }, [projectsLoaded, catalogLoading, catalogProjectId, services, repos, state.draft.scope, state.draft.serviceIds, state.draft.repoId, state.draft.projectId]);

  /* ------------------------------- selectors ------------------------------- */
  const scopeServices = useMemo(() => {
    if (state.draft.scope === 'project') return services;
    if (state.draft.scope === 'repo') return services.filter((service) => service.repo_id === state.draft.repoId);
    return services.filter((service) => state.draft.serviceIds.includes(service.id));
  }, [services, state.draft.scope, state.draft.repoId, state.draft.serviceIds]);

  const scopeRepos = useMemo(() => {
    const ids = new Set(scopeServices.map((service) => service.repo_id));
    return repos.filter((repo) => ids.has(repo.id));
  }, [repos, scopeServices]);

  const entries = useMemo<DraftEntry[]>(() => scopeServices.map((service) => ({
    service,
    repo: repos.find((repo) => repo.id === service.repo_id) ?? null,
    ref: state.draft.refs[service.repo_id] ?? null,
  })), [scopeServices, repos, state.draft.refs]);

  const missingRefRepoIds = useMemo(() => {
    const missing = new Set<number>();
    for (const entry of entries) {
      if (!entry.repo) missing.add(-entry.service.repo_id);
      else if (!entry.ref?.name) missing.add(entry.repo.id);
    }
    return [...missing];
  }, [entries]);

  const formSignature = useMemo(() => JSON.stringify({
    projectId: state.draft.projectId,
    scope: state.draft.scope,
    repoId: state.draft.repoId,
    serviceIds: state.draft.serviceIds,
    repos: scopeRepos.map((repo) => [repo.id, state.draft.refs[repo.id]?.type ?? '', state.draft.refs[repo.id]?.name ?? '']),
  }), [state.draft, scopeRepos]);

  /* ------------------------------ URL mirroring ------------------------------ */
  const pendingWrite = useRef<string | null>(null);
  const lastApplied = useRef<string>('');
  const prevScope = useRef(state.draft.scope);
  const prevRunId = useRef(state.runId);
  const scopeRepoIds = useMemo(() => scopeRepos.map((repo) => repo.id), [scopeRepos]);

  useEffect(() => {
    const current = location.search.replace(/^\?/, '');
    // A URL change that we did not write ourselves means browser back/forward
    // (or an external navigation). Let the URL->state effect below hydrate it;
    // never overwrite it with the stale in-memory draft.
    const external = pendingWrite.current === null && lastApplied.current !== '' && current !== lastApplied.current;
    if (external) return;
    const next = draftToSearchParams(state, scopeRepoIds).toString();
    if (next === current) {
      pendingWrite.current = null;
      lastApplied.current = current;
      prevScope.current = state.draft.scope;
      prevRunId.current = state.runId;
      return;
    }
    const push = prevScope.current !== state.draft.scope || prevRunId.current !== state.runId;
    prevScope.current = state.draft.scope;
    prevRunId.current = state.runId;
    pendingWrite.current = next;
    lastApplied.current = next;
    setSearchParamsApi(new URLSearchParams(next), { replace: !push });
  }, [state, scopeRepoIds, location.search, setSearchParamsApi]);

  useEffect(() => {
    const current = location.search.replace(/^\?/, '');
    if (pendingWrite.current !== null) {
      // Our own write has not reached the router yet; ignore the stale URL.
      if (current !== pendingWrite.current) return;
      pendingWrite.current = null;
      lastApplied.current = current;
      return;
    }
    if (current === lastApplied.current) return;
    lastApplied.current = current;
    const parsed = parseGenerationSearch(new URLSearchParams(current), Number(projectId) || 0);
    dispatch({ type: 'hydrate', draft: parsed.state.draft, runId: parsed.state.runId });
  }, [location.search, projectId]);

  /* ------------------------------- actions ------------------------------- */
  const actions = useMemo(() => ({
    setScope(scope: GenerationScope) {
      dispatch({ type: 'setScope', scope });
      // Keep the flow usable: entering service scope preselects the globally
      // active service if nothing is selected yet.
      if (scope === 'service' && !state.draft.serviceIds.length && activeServiceId && services.some((service) => service.id === activeServiceId)) {
        dispatch({ type: 'setServiceIds', serviceIds: [activeServiceId] });
      }
    },
    setRepo(repoId: number | null) { dispatch({ type: 'setRepo', repoId }); },
    toggleService(serviceId: number) { dispatch({ type: 'toggleService', serviceId }); },
    setServiceIds(serviceIds: number[]) { dispatch({ type: 'setServiceIds', serviceIds }); },
    setRef(repoId: number, ref: DraftRef) { dispatch({ type: 'setRef', repoId, ref }); },
    setRefs(refs: Record<number, DraftRef>) { dispatch({ type: 'setRefs', refs }); },
    hydrate(draft: GenerationPlanDraft, runId?: number | null) { dispatch({ type: 'hydrate', draft, runId }); },
  }), [activeServiceId, services, state.draft.serviceIds]);

  const setRunId = useCallback((runId: number | null) => { dispatch({ type: 'setRunId', runId }); }, []);

  return {
    draft: state.draft,
    runId: state.runId,
    setRunId,
    scopeServices,
    scopeRepos,
    entries,
    refsReady: missingRefRepoIds.length === 0,
    missingRefRepoIds,
    formSignature,
    catalogProjectMatches,
    rerunFrom,
    actions,
  };
}

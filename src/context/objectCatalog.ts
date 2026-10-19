import { useEffect, useSyncExternalStore } from 'react';
import type { GreenPassClient, Target } from '@/api/client';
import { configuredGreenPassClient } from '@/api/runtime';
import { useCurrentUser } from '@/context/login';

export type CatalogRepository = Awaited<ReturnType<GreenPassClient['listProjectRepositories']>>[number];
export type CatalogService = Awaited<ReturnType<GreenPassClient['listProjectServices']>>[number];

interface ObjectCatalogState {
  enabled: boolean;
  loaded: boolean;
  loading: boolean;
  error: string;
  projects: Target[];
  projectId: string;
  repoId: number | null;
  serviceId: number | null;
  repos: CatalogRepository[];
  services: CatalogService[];
}

const client = configuredGreenPassClient();
function remembered(key: string): string {
  try { return localStorage.getItem(key) ?? ''; } catch { return ''; }
}
let snapshot: ObjectCatalogState = {
  enabled: Boolean(client), loaded: false, loading: Boolean(client), error: '', projects: [],
  projectId: remembered('greenpass_project_id'), repoId: null,
  serviceId: Number(remembered('greenpass_active_service_id')) || null, repos: [], services: [],
};
const listeners = new Set<() => void>();
let requestVersion = 0;
let started = false;
let accountId = '';

function publish(change: Partial<ObjectCatalogState>) {
  snapshot = { ...snapshot, ...change };
  listeners.forEach((listener) => listener());
}

function persistSelection() {
  const service = snapshot.services.find((item) => item.id === snapshot.serviceId);
  try {
    localStorage.setItem('greenpass_project_id', snapshot.projectId);
    if (service) {
      localStorage.setItem('greenpass_active_service_id', String(service.id));
      localStorage.setItem('greenpass_active_service_name', service.name);
    } else {
      localStorage.removeItem('greenpass_active_service_id');
      localStorage.removeItem('greenpass_active_service_name');
    }
  } catch { /* The in-memory selection still works when storage is unavailable. */ }
}

async function loadCatalog(reloadProjects: boolean) {
  if (!client) return snapshot;
  const version = ++requestVersion;
  publish({ loading: true, error: '' });
  try {
    const projects = reloadProjects ? (await client.listTargets() ?? []).filter((target) => target.type === 'project') : snapshot.projects;
    if (version !== requestVersion) return snapshot;
    const projectId = projects.some((project) => String(project.id) === snapshot.projectId)
      ? snapshot.projectId : String(projects[0]?.id ?? '');
    const changedProject = projectId !== snapshot.projectId;
    publish({ projects, projectId, ...(changedProject ? { repos: [], services: [], repoId: null, serviceId: null } : {}) });
    const [repos, services] = projectId
      ? await Promise.all([client.listProjectRepositories(Number(projectId)), client.listProjectServices(Number(projectId))])
      : [[], []];
    if (version !== requestVersion) return snapshot;
    const service = (services ?? []).find((item) => item.id === snapshot.serviceId);
    const repoId = service?.repo_id ?? ((repos ?? []).some((repo) => repo.id === snapshot.repoId) ? snapshot.repoId : null);
    publish({ repos: repos ?? [], services: services ?? [], serviceId: service?.id ?? null, repoId, loaded: true, loading: false });
    persistSelection();
  } catch (cause) {
    if (version === requestVersion) publish({ loading: false, loaded: true, error: cause instanceof Error ? cause.message : '读取被测对象失败，请重试' });
  }
  return snapshot;
}

function selectProject(projectId: string) {
  if (!snapshot.projects.some((project) => String(project.id) === projectId)) return;
  if (projectId === snapshot.projectId) {
    publish({ repoId: null, serviceId: null });
    persistSelection();
    return;
  }
  publish({ projectId, repoId: null, serviceId: null, repos: [], services: [] });
  persistSelection();
  void loadCatalog(false);
}

function selectService(serviceId: number | null) {
  const service = snapshot.services.find((item) => item.id === serviceId);
  if (serviceId !== null && !service) return;
  publish({ serviceId: service?.id ?? null, repoId: service?.repo_id ?? null });
  persistSelection();
}

function selectRepository(repoId: number | null) {
  if (repoId !== null && !snapshot.repos.some((repo) => repo.id === repoId)) return;
  publish({ repoId, serviceId: null });
  persistSelection();
}

const subscribe = (listener: () => void) => { listeners.add(listener); return () => { listeners.delete(listener); }; };
const getSnapshot = () => snapshot;
const refresh = () => loadCatalog(true);

/** One reactive selection shared by the header, object management, and generation. */
export function useObjectCatalog() {
  const state = useSyncExternalStore(subscribe, getSnapshot);
  const user = useCurrentUser();
  const userId = user?.id ?? '';
  useEffect(() => {
    if (!client || !userId || (started && accountId === userId)) return;
    if (accountId && accountId !== userId) {
      publish({ projects: [], repos: [], services: [], projectId: '', repoId: null, serviceId: null, loaded: false });
      persistSelection();
    }
    accountId = userId;
    started = true;
    void refresh();
  }, [userId]);
  return { ...state, refresh, selectProject, selectService, selectRepository };
}

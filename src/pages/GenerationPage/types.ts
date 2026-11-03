import type {
  GenerationPlanRequestUnit,
  GenerationRefType,
  GenerationRunDetail,
  GenerationRunState,
  GenerationRunSummary,
  GenerationRunUnitDetail,
  GenerationRunUnitState,
} from '@/api/client';
import { GreenPassApiError } from '@/api/client';

/** Three scope modes exposed by the generation console. */
export type GenerationScope = 'project' | 'repo' | 'service';
export type DraftRef = { type: GenerationRefType; name: string };

/**
 * Single source of truth for "what will be generated".
 * Services/refs are never duplicated in separate state; entries are derived
 * from this draft plus the object catalog.
 */
export type GenerationPlanDraft = {
  projectId: number;
  scope: GenerationScope;
  repoId: number | null;
  serviceIds: number[];
  refs: Record<number, DraftRef>;
};

export const EMPTY_DRAFT: GenerationPlanDraft = {
  projectId: 0,
  scope: 'project',
  repoId: null,
  serviceIds: [],
  refs: {},
};

export type GenerationConsoleState = {
  draft: GenerationPlanDraft;
  runId: number | null;
};

export type GenerationConsoleAction =
  | { type: 'hydrate'; draft: GenerationPlanDraft; runId?: number | null }
  | { type: 'setProject'; projectId: number }
  | { type: 'setScope'; scope: GenerationScope }
  | { type: 'setRepo'; repoId: number | null }
  | { type: 'toggleService'; serviceId: number }
  | { type: 'setServiceIds'; serviceIds: number[] }
  | { type: 'setRef'; repoId: number; ref: DraftRef }
  | { type: 'setRefs'; refs: Record<number, DraftRef> }
  | { type: 'setRunId'; runId: number | null };

export function generationConsoleReducer(state: GenerationConsoleState, action: GenerationConsoleAction): GenerationConsoleState {
  switch (action.type) {
    case 'hydrate': {
      const draft = normalizeDraft(action.draft);
      // A different project can never keep the previous run console.
      const runId = action.runId !== undefined
        ? action.runId
        : draft.projectId === state.draft.projectId ? state.runId : null;
      return { draft, runId };
    }
    case 'setProject': {
      if (action.projectId === state.draft.projectId) return state;
      return { draft: { ...EMPTY_DRAFT, projectId: action.projectId }, runId: null };
    }
    case 'setScope':
      if (action.scope === state.draft.scope) return state;
      return { ...state, draft: { ...state.draft, scope: action.scope } };
    case 'setRepo':
      return { ...state, draft: { ...state.draft, repoId: action.repoId } };
    case 'toggleService': {
      const exists = state.draft.serviceIds.includes(action.serviceId);
      const serviceIds = exists
        ? state.draft.serviceIds.filter((id) => id !== action.serviceId)
        : [...state.draft.serviceIds, action.serviceId];
      return { ...state, draft: { ...state.draft, serviceIds: [...serviceIds].sort((a, b) => a - b) } };
    }
    case 'setServiceIds': {
      const serviceIds = [...new Set(action.serviceIds)].sort((a, b) => a - b);
      return { ...state, draft: { ...state.draft, serviceIds } };
    }
    case 'setRef':
      return { ...state, draft: { ...state.draft, refs: { ...state.draft.refs, [action.repoId]: action.ref } } };
    case 'setRefs':
      return { ...state, draft: { ...state.draft, refs: { ...state.draft.refs, ...action.refs } } };
    case 'setRunId':
      return state.runId === action.runId ? state : { ...state, runId: action.runId };
    default:
      return state;
  }
}

function normalizeDraft(draft: GenerationPlanDraft): GenerationPlanDraft {
  return {
    projectId: draft.projectId,
    scope: draft.scope === 'repo' || draft.scope === 'service' ? draft.scope : 'project',
    repoId: typeof draft.repoId === 'number' ? draft.repoId : null,
    serviceIds: [...new Set(draft.serviceIds)].filter((id) => Number.isFinite(id) && id > 0).sort((a, b) => a - b),
    refs: Object.fromEntries(Object.entries(draft.refs ?? {}).flatMap(([repoId, ref]) => {
      const id = Number(repoId);
      return Number.isFinite(id) && ref && (ref.type === 'branch' || ref.type === 'tag') && typeof ref.name === 'string' && ref.name
        ? [[id, { type: ref.type, name: ref.name }] as const]
        : [];
    })),
  };
}

/* ------------------------------------------------------------------ *
 * URL query (de)serialization: project_id / scope / scope_id /
 * service_ids / refs / run_id. URLs are shareable and survive reloads.
 * ------------------------------------------------------------------ */

export function draftToSearchParams(state: GenerationConsoleState, scopeRepoIds: number[]): URLSearchParams {
  const params = new URLSearchParams();
  const { draft, runId } = state;
  if (draft.projectId) params.set('project_id', String(draft.projectId));
  params.set('scope', draft.scope);
  const scopeId = draft.scope === 'project'
    ? draft.projectId
    : draft.scope === 'repo'
      ? draft.repoId ?? 0
      : draft.serviceIds[0] ?? 0;
  if (scopeId) params.set('scope_id', String(scopeId));
  if (draft.scope === 'service' && draft.serviceIds.length) params.set('service_ids', draft.serviceIds.join(','));
  const refParts = scopeRepoIds.flatMap((repoId) => {
    const ref = draft.refs[repoId];
    return ref && ref.name ? [`${repoId}:${ref.type}:${encodeURIComponent(ref.name)}`] : [];
  });
  if (refParts.length) params.set('refs', refParts.join(','));
  if (runId) params.set('run_id', String(runId));
  return params;
}

export type ParsedGenerationSearch = {
  state: GenerationConsoleState;
  /** One-shot deep link: prefill a new draft from a historical run, then require plan+confirm. */
  rerunFrom: number | null;
};

export function parseGenerationSearch(search: URLSearchParams, fallbackProjectId: number): ParsedGenerationSearch {
  const projectId = positiveInt(search.get('project_id')) ?? fallbackProjectId;
  const rawScope = search.get('scope');
  const scope: GenerationScope = rawScope === 'repo' || rawScope === 'service' ? rawScope : 'project';
  const runId = positiveInt(search.get('run_id'));
  const scopeId = positiveInt(search.get('scope_id'));
  const serviceIds = (search.get('service_ids') ?? '')
    .split(',')
    .flatMap((part) => {
      const id = positiveInt(part);
      return id ? [id] : [];
    });
  if (scope === 'service' && !serviceIds.length && scopeId) serviceIds.push(scopeId);
  const refs: Record<number, DraftRef> = {};
  for (const part of (search.get('refs') ?? '').split(',')) {
    const [repoRaw, typeRaw, ...nameParts] = part.split(':');
    const repoId = positiveInt(repoRaw);
    const name = nameParts.join(':');
    if (!repoId || (typeRaw !== 'branch' && typeRaw !== 'tag') || !name) continue;
    try {
      refs[repoId] = { type: typeRaw, name: decodeURIComponent(name) };
    } catch {
      refs[repoId] = { type: typeRaw, name };
    }
  }
  const draft: GenerationPlanDraft = normalizeDraft({
    projectId,
    scope,
    repoId: scope === 'repo' ? scopeId ?? null : null,
    serviceIds,
    refs,
  });
  return { state: { draft, runId }, rerunFrom: positiveInt(search.get('rerun_from')) };
}

function positiveInt(value: string | null): number | undefined {
  if (!value) return undefined;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined;
}

/* ------------------------------------------------------------------ *
 * Presentation metadata and derived selectors.
 * ------------------------------------------------------------------ */

export const SCOPE_LABEL: Record<GenerationScope, string> = {
  project: '项目全量',
  repo: '仓库全量',
  service: '指定服务',
};

export const RUN_STATE_LABEL: Record<GenerationRunState, string> = {
  queued: '排队中',
  running: '生成中',
  waiting_review: '等待审核',
  partial_failed: '部分失败',
  succeeded: '已完成',
  failed: '失败',
  cancelled: '已取消',
};

export const UNIT_STATE_LABEL: Record<GenerationRunUnitState, string> = {
  queued: '排队中',
  running: '生成中',
  waiting_review: '等待审核',
  succeeded: '已入库',
  failed: '失败',
  rejected: '已拒绝',
  cancelled: '已取消',
};

export const BATCH_STAGE_LABEL: Record<string, string> = {
  enqueued: '待启动',
  fetching: '冻结源码',
  parsing: '解析归一化',
  generating: '生成候选',
  quality_check: '质量检查',
  review: '人工审核',
  committing: '写入用例库',
  approved: '已完成',
  rejected: '已拒绝',
  failed: '失败',
  rolled_back: '已回退',
  cancelled: '已取消',
};

export const BATCH_STAGE_ORDER = ['enqueued', 'fetching', 'parsing', 'generating', 'quality_check', 'review', 'committing', 'approved'] as const;

export const RUN_TERMINAL_STATES: GenerationRunState[] = ['succeeded', 'partial_failed', 'failed', 'cancelled'];

export function isTerminalRunState(state: GenerationRunState | undefined | null): boolean {
  return Boolean(state && RUN_TERMINAL_STATES.includes(state));
}

export function isActiveRunState(state: GenerationRunState | undefined | null): boolean {
  return state === 'queued' || state === 'running' || state === 'waiting_review';
}

export function normalizeRunDetail(input: GenerationRunSummary | GenerationRunDetail): GenerationRunDetail {
  const units: GenerationRunUnitDetail[] = (input.units ?? []).map((unit) => ({
    ...unit,
    state: unit.state ?? 'queued',
  }));
  const total = typeof input.total_units === 'number' ? input.total_units : units.length;
  const completed = typeof input.completed_units === 'number'
    ? input.completed_units
    : units.filter((unit) => unit.state === 'succeeded').length;
  const failed = typeof input.failed_units === 'number'
    ? input.failed_units
    : units.filter((unit) => unit.state === 'failed' || unit.state === 'cancelled').length;
  const review = typeof input.review_units === 'number'
    ? input.review_units
    : units.filter((unit) => unit.state === 'waiting_review').length;
  const progress = typeof input.progress_percent === 'number'
    ? input.progress_percent
    : total > 0 ? Math.round(((completed + failed) / total) * 100) : 0;
  return {
    ...input,
    units,
    total_units: total,
    completed_units: completed,
    failed_units: failed,
    review_units: review,
    progress_percent: Math.max(0, Math.min(100, progress)),
  };
}

/** Stable signature of the units that will be sent to the preflight API. */
export function planRequestSignature(units: GenerationPlanRequestUnit[], projectId: number, scope: GenerationScope, scopeId: number): string {
  const canonical = [...units]
    .map((unit) => ({ service_id: unit.service_id, repo_id: unit.repo_id, source_path: unit.source_path ?? '', ref_type: unit.ref_type, ref_name: unit.ref_name }))
    .sort((a, b) => a.service_id - b.service_id || a.repo_id - b.repo_id);
  return JSON.stringify({ projectId, scope, scopeId, units: canonical });
}

export function formatDateTime(value?: string): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', { hour12: false });
}

export function formatTokens(run: Pick<GenerationRunSummary, 'tokens_in' | 'tokens_out'>): string {
  const total = (run.tokens_in ?? 0) + (run.tokens_out ?? 0);
  return total > 0 ? total.toLocaleString('zh-CN') : '—';
}

export { formatCost } from './planPresentation';

export function formatDuration(seconds?: number): string {
  if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds <= 0) return '未知';
  if (seconds < 60) return `${Math.round(seconds)} 秒`;
  const minutes = Math.round(seconds / 60);
  return minutes < 60 ? `${minutes} 分钟` : `${(minutes / 60).toFixed(1)} 小时`;
}

export function formatCandidateRange(min?: number, max?: number): string {
  if (typeof min !== 'number' || typeof max !== 'number') return '未知';
  return min === max ? `${min} 条` : `${min}–${max} 条`;
}

/** A repository root is stored as "." for the source reader; never show that. */
export function sourceDirectoryLabel(sourcePath?: string): string {
  return !sourcePath || sourcePath === '.' ? '仓库根目录' : sourcePath;
}

/** Machine-readable API error code (server envelope or legacy message text). */
export function errorCodeOf(cause: unknown): string | undefined {
  if (cause instanceof GreenPassApiError) {
    if (cause.code) return cause.code;
    const text = cause.message ?? '';
    const known = ['plan_stale', 'idempotency_conflict', 'ref_not_found', 'profile_not_confirmed', 'repo_unreadable', 'scm_not_verified', 'duplicate_units'];
    return known.find((code) => text.includes(code));
  }
  return undefined;
}

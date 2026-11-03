/**
 * Pure helpers for the cursor-paginated generation history list.
 * Kept dependency-free so they can be unit-tested with the Node test runner
 * (the repo has no vitest installed).
 */
export const HISTORY_PAGE_SIZE = 20;

export const HISTORY_RUN_STATES = [
  'queued',
  'running',
  'waiting_review',
  'partial_failed',
  'succeeded',
  'failed',
  'cancelled',
] as const;

export type HistoryRunStateValue = typeof HISTORY_RUN_STATES[number];
export type HistoryStateFilter = 'all' | HistoryRunStateValue;

export function parseHistoryStateFilter(value: string | null | undefined): HistoryStateFilter {
  if (!value) return 'all';
  return (HISTORY_RUN_STATES as readonly string[]).includes(value) ? (value as HistoryRunStateValue) : 'all';
}

/** Appends a page, de-duplicating by id while preserving server (id DESC) order. */
export function mergeRunPages<T extends { id: number }>(existing: T[], incoming: T[]): T[] {
  const seen = new Set<number>();
  const merged: T[] = [];
  for (const run of [...existing, ...incoming]) {
    if (seen.has(run.id)) continue;
    seen.add(run.id);
    merged.push(run);
  }
  return merged;
}

/** Transitional fallback for a backend that still returns the legacy bare array. */
export function filterRunPageByState<T extends { state: string }>(runs: T[], filter: HistoryStateFilter): T[] {
  if (filter === 'all') return runs;
  return runs.filter((run) => run.state === filter);
}

export function hasNextPage(nextCursor: string | null | undefined): boolean {
  return typeof nextCursor === 'string' && nextCursor.length > 0;
}

/**
 * Progress presentation for the run console.
 *
 * Generation progress and review are separate phases. The backend reports
 * `progress_percent = (completed + failed) / total`, so the UI derives two
 * special presentations:
 * - `cancelled`: finished units include cancelled units, shown as "已结束";
 * - `waiting_review`: generation work is complete (100%) and the caption asks
 *   for review instead of claiming the run is finished.
 * Every other state uses the server value. Caption and bar always share the
 * same percent.
 */
export type RunProgressUnitLike = { state?: string };

export type RunProgressRunLike = {
  state?: string;
  total_units?: number;
  completed_units?: number;
  failed_units?: number;
  review_units?: number;
  progress_percent?: number;
  units?: RunProgressUnitLike[];
};

export type RunProgressView = {
  state: string;
  /** Value rendered on the progress bar and in the caption. */
  percent: number;
  finishedUnits: number;
  waitingUnits: number;
  totalUnits: number;
  cancelledUnits: number;
  /** True when the percent is derived locally instead of taken from the server. */
  derived: boolean;
};

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, Math.round(value)));
}

export function runProgressView(run: RunProgressRunLike): RunProgressView {
  const units = run.units ?? [];
  const totalUnits = typeof run.total_units === 'number' ? run.total_units : units.length;
  const completedUnits = typeof run.completed_units === 'number'
    ? run.completed_units
    : units.filter((unit) => unit.state === 'succeeded').length;
  const failedUnits = typeof run.failed_units === 'number'
    ? run.failed_units
    : units.filter((unit) => unit.state === 'failed').length;
  const cancelledUnits = units.filter((unit) => unit.state === 'cancelled').length;
  const waitingUnits = typeof run.review_units === 'number'
    ? run.review_units
    : units.filter((unit) => unit.state === 'waiting_review').length;
  const finishedUnits = completedUnits + failedUnits + cancelledUnits;
  const finishedPercent = totalUnits > 0 ? (finishedUnits / totalUnits) * 100 : 0;
  const serverPercent = typeof run.progress_percent === 'number' ? run.progress_percent : finishedPercent;
  const waitingReview = run.state === 'waiting_review';
  const derived = run.state === 'cancelled' || waitingReview;
  return {
    state: run.state ?? '',
    percent: clampPercent(waitingReview ? 100 : derived ? finishedPercent : serverPercent),
    finishedUnits,
    waitingUnits,
    totalUnits,
    cancelledUnits,
    derived,
  };
}

/** Caption shared with the bar, e.g. "100% · 1 / 1 个服务已结束". */
export function runProgressCaption(view: RunProgressView): string {
  if (view.state === 'waiting_review') {
    return `${view.percent}% · ${view.waitingUnits} / ${view.totalUnits} 个服务待审核`;
  }
  return `${view.percent}% · ${view.finishedUnits} / ${view.totalUnits} 个服务已结束`;
}

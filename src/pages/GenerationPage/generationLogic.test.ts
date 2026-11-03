import assert from 'node:assert/strict';
import test from 'node:test';
import { capturePlanSignature, isPlanResponseStale } from './planSignature.ts';
import { draftMatchesCatalogData, shouldValidateDraftProject } from './draftSync.ts';
import { hasPriceMissingRisk, planCostView } from './planPresentation.ts';
import { runProgressCaption, runProgressView } from './runProgress.ts';

/* P1-1: preflight response must be attributed to the signature captured when
 * the request started, even when the draft changes while it is in flight. */
test('P1-1 plan response keeps the request-start signature after a mid-flight draft edit', async () => {
  let draftSignature = 'draft-A';
  let resolveResponse: (value: { plan_id: number }) => void = () => {};
  const responsePromise = new Promise<{ plan_id: number }>((resolve) => { resolveResponse = resolve; });

  // preview() starts here.
  const requestedSignature = capturePlanSignature(draftSignature);
  const previewPromise = responsePromise.then((plan) => ({ plan, plannedSignature: requestedSignature }));

  // The user edits the draft (services/refs) before the response arrives.
  draftSignature = 'draft-B';
  resolveResponse({ plan_id: 1 });

  const result = await previewPromise;
  assert.equal(result.plannedSignature, 'draft-A');
  assert.equal(isPlanResponseStale(result.plannedSignature, draftSignature), true);
});

test('P1-1 regression shape: adopting the latest signature would hide staleness', () => {
  let draftSignature = 'draft-A';
  const requestStart = capturePlanSignature(draftSignature);
  draftSignature = 'draft-B';
  const buggyPlannedSignature = draftSignature; // pre-fix behavior
  assert.equal(isPlanResponseStale(buggyPlannedSignature, draftSignature), false);
  assert.equal(isPlanResponseStale(requestStart, draftSignature), true);
});

test('P1-1 unchanged draft is not stale', () => {
  const requested = capturePlanSignature('draft-A');
  assert.equal(isPlanResponseStale(requested, 'draft-A'), false);
});

/* P1-9: cross-project URL restore must not validate/default the draft against
 * a stale catalog project. */
test('P1-9 catalog validation waits for the URL project catalog', () => {
  const base = { projectsLoaded: true, catalogLoading: false, draftProjectId: 2, catalogProjectId: 1 };
  assert.equal(shouldValidateDraftProject({ ...base, pendingProjectSync: 2 }), false, 'pending project sync must skip validation');
  assert.equal(shouldValidateDraftProject({ ...base, pendingProjectSync: null, catalogLoading: true }), false, 'loading catalog must skip validation');
  assert.equal(shouldValidateDraftProject({ ...base, pendingProjectSync: null }), false, 'project mismatch must skip validation');
  assert.equal(shouldValidateDraftProject({ ...base, draftProjectId: 1, pendingProjectSync: null }), true, 'matching project after load validates');
});

test('P1-9 refs loading only uses catalog data from the draft project', () => {
  assert.equal(draftMatchesCatalogData(2, 1, false), false);
  assert.equal(draftMatchesCatalogData(2, 1, true), false);
  assert.equal(draftMatchesCatalogData(1, 1, true), false, 'loading catalog still blocks refs');
  assert.equal(draftMatchesCatalogData(1, 1, false), true);
  assert.equal(draftMatchesCatalogData(0, 0, false), false);
});

/* P1-10: price_missing must never render as ¥0.00. */
test('P1-10 price_missing cost shows unknown instead of ¥0.00', () => {
  const plan = { estimate: { unknown: false, cost_amount: 0, currency: 'CNY' }, risks: [{ code: 'price_missing', severity: 'warning' }] };
  assert.equal(hasPriceMissingRisk(plan), true);
  const view = planCostView(plan);
  assert.equal(view.known, false);
  assert.match(view.text, /未知/);
  assert.doesNotMatch(view.text, /¥0\.00/);
  assert.match(view.hint, /单价/);
});

test('P1-10 unknown estimate and normal estimate render correctly', () => {
  assert.equal(planCostView({ estimate: { unknown: true, cost_amount: 0 } }).text, '未知');
  assert.equal(planCostView({ estimate: { unknown: false, cost_amount: 0.05, currency: 'CNY' }, risks: [] }).text, '¥0.05');
  assert.equal(planCostView({ estimate: {}, risks: [] }).text, '未知');
});

/* R3/P2-15: cancelled run progress uses finished units; bar and caption agree. */
test('R3 cancelled run progress is derived from finished units (not 0%)', () => {
  const view = runProgressView({ state: 'cancelled', total_units: 1, completed_units: 0, failed_units: 0, progress_percent: 0, units: [{ state: 'cancelled' }] });
  assert.equal(view.percent, 100);
  assert.equal(view.finishedUnits, 1);
  assert.equal(view.totalUnits, 1);
  assert.equal(view.derived, true);
  assert.equal(runProgressCaption(view), '100% · 1 / 1 个服务已结束');
});

test('R3 non-cancelled states keep the server progress_percent', () => {
  const running = runProgressView({ state: 'running', total_units: 2, completed_units: 1, failed_units: 0, progress_percent: 50, units: [{ state: 'succeeded' }, { state: 'running' }] });
  assert.equal(running.percent, 50);
  assert.equal(running.finishedUnits, 1);
  assert.equal(running.derived, false);
  assert.equal(runProgressCaption(running), '50% · 1 / 2 个服务已结束');
  const waiting = runProgressView({ state: 'waiting_review', total_units: 1, completed_units: 0, failed_units: 0, review_units: 1, progress_percent: 0, units: [{ state: 'waiting_review' }] });
  assert.equal(waiting.percent, 100, 'generation is complete while review is pending');
  assert.equal(waiting.finishedUnits, 0);
  assert.equal(waiting.waitingUnits, 1);
  assert.equal(runProgressCaption(waiting), '100% · 1 / 1 个服务待审核');
});

test('R3 cancelled mixed with failed units and missing server progress', () => {
  const mixed = runProgressView({ state: 'cancelled', total_units: 4, completed_units: 1, failed_units: 1, progress_percent: 50, units: [{ state: 'succeeded' }, { state: 'failed' }, { state: 'cancelled' }, { state: 'cancelled' }] });
  assert.equal(mixed.percent, 100);
  assert.equal(mixed.finishedUnits, 4);
  const fallback = runProgressView({ state: 'succeeded', total_units: 2, units: [{ state: 'succeeded' }, { state: 'failed' }] });
  assert.equal(fallback.percent, 100);
  assert.equal(runProgressCaption(fallback), '100% · 2 / 2 个服务已结束');
});

/* R4: waiting_review keeps the 100% product semantics (generation complete, review pending). */
test('R4 waiting_review keeps 100% product semantics with review caption', () => {
  const view = runProgressView({ state: 'waiting_review', total_units: 2, completed_units: 0, failed_units: 0, review_units: 2, progress_percent: 0, units: [{ state: 'waiting_review' }, { state: 'waiting_review' }] });
  assert.equal(view.percent, 100);
  assert.equal(view.derived, true);
  assert.equal(runProgressCaption(view), '100% · 2 / 2 个服务待审核');
});

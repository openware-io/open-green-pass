import { http, HttpResponse } from 'msw';
import type { CaseResult, CaseVersion, CostLineItem, CostOverview, GateResult, Run, Target, TestCase } from '@/api/client';

const teamID = 100;
let nextID = 9000;
const targets: Target[] = [
  { id: 1001, team_id: teamID, type: 'project', name: 'im-saas', kind: 'project', status: 'active', parent_id: null },
  { id: 1003, team_id: teamID, type: 'service', name: 'im-saas-gateway', kind: 'api_service', status: 'active', parent_id: 1001 },
];
const cases: TestCase[] = [
  { id: 4001, team_id: teamID, target_id: 1003, code: 'im-saas-gw-001', title: '健康检查接口', kind: 'api', current_version: 1, status: 'active' },
  { id: 4002, team_id: teamID, target_id: 1003, code: 'im-saas-gw-002', title: '登录接口', kind: 'api', current_version: 1, status: 'active' },
];
const histories = new Map<number, CaseVersion[]>(cases.map((item) => [item.id, [{ id: item.id * 10, team_id: teamID, case_id: item.id, version: 1, change_type: 'added', created_by: 1, created_at: new Date().toISOString(), script: { method: 'GET', path: '/healthz' } }]]));
const runs = new Map<number, Run>();
const results = new Map<number, CaseResult[]>();
const gateResults = new Map<number, GateResult[]>();

const json = (data: unknown, status = 200) => HttpResponse.json(data, { status });
const parseID = (value: string | undefined) => Number(value);
const runResponse = (run: Run): Run => ({ ...run });

export const handlers = [
  http.get('/targets', () => json(targets)),
  http.post('/targets', async ({ request }) => {
    const body = await request.json() as { name: string; type: Target['type']; kind?: string; parent_id?: number };
    const target: Target = { id: ++nextID, team_id: teamID, type: body.type, name: body.name, kind: body.kind ?? 'other', status: 'active', parent_id: body.parent_id ?? null };
    targets.push(target);
    return json(target, 201);
  }),
  http.post('/targets/:id/repo', async ({ params, request }) => {
    const target = targets.find((item) => item.id === parseID(params.id as string));
    if (!target) return json({ message: 'target not found' }, 404);
    const body = await request.json() as { url: string };
    void body;
    return json({ status: 'ok' });
  }),
  http.get('/cases', ({ request }) => {
    const query = new URL(request.url).searchParams;
    const targetID = query.get('target_id');
    return json(cases.filter((item) => !targetID || item.target_id === Number(targetID)));
  }),
  http.post('/cases', async ({ request }) => {
    const body = await request.json() as { target_id: number; code: string; title: string; kind?: string; script?: unknown };
    const item: TestCase = { id: ++nextID, team_id: teamID, target_id: body.target_id, code: body.code, title: body.title, kind: body.kind ?? 'api', current_version: 1, status: 'active' };
    cases.push(item);
    histories.set(item.id, [{ id: ++nextID, team_id: teamID, case_id: item.id, version: 1, change_type: 'added', created_by: 1, created_at: new Date().toISOString(), script: body.script }]);
    return json(item, 201);
  }),
  http.post('/cases/:id/versions', async ({ params, request }) => {
    const item = cases.find((candidate) => candidate.id === parseID(params.id as string));
    if (!item) return json({ message: 'case not found' }, 404);
    const body = await request.json() as { script: unknown };
    item.current_version += 1;
    histories.get(item.id)?.push({ id: ++nextID, team_id: teamID, case_id: item.id, version: item.current_version, change_type: 'updated', created_by: 1, created_at: new Date().toISOString(), script: body.script });
    return json(item);
  }),
  http.post('/cases/:id/rollback', async ({ params, request }) => {
    const item = cases.find((candidate) => candidate.id === parseID(params.id as string));
    if (!item) return json({ message: 'case not found' }, 404);
    const body = await request.json() as { version: number };
    const history = histories.get(item.id) ?? [];
    if (!history.some((version) => version.version === body.version)) return json({ message: 'version not found' }, 400);
    item.current_version = body.version;
    return json(item);
  }),
  http.get('/cases/:id/history', ({ params }) => json(histories.get(parseID(params.id as string)) ?? [])),
  http.delete('/cases/:id', ({ params }) => {
    const item = cases.find((candidate) => candidate.id === parseID(params.id as string));
    if (!item) return json({ message: 'case not found' }, 404);
    item.status = 'deleted';
    return json({ status: 'ok' });
  }),
  http.post('/runs', async ({ request }) => {
    const body = await request.json() as { target_id: number; scenario_id: number; env?: string; target_version: string; target_branch?: string; selected_case_ids?: number[] };
    const selected = body.selected_case_ids?.length ? body.selected_case_ids : cases.filter((item) => item.target_id === body.target_id && item.status === 'active').map((item) => item.id);
    const run: Run = { id: ++nextID, team_id: teamID, scenario_id: body.scenario_id, target_id: body.target_id, env: body.env ?? 'test', target_version: body.target_version, target_branch: body.target_branch ?? 'main', env_version: '', env_check_id: null, run_mode: 'manual', state: 'queued', selected_cases: selected, started_at: null, ended_at: null };
    runs.set(run.id, run);
    return json(run, 201);
  }),
  http.get('/runs', ({ request }) => {
    const query = new URL(request.url).searchParams;
    const targetID = query.get('target_id');
    const state = query.get('state');
    return json([...runs.values()].filter((run) => (!targetID || run.target_id === Number(targetID)) && (!state || run.state === state)));
  }),
  http.get('/runs/:id', ({ params }) => { const run = runs.get(parseID(params.id as string)); return run ? json(runResponse(run)) : json({ message: 'run not found' }, 404); }),
  http.post('/runs/:id/version-check', ({ params }) => { const run = runs.get(parseID(params.id as string)); if (!run) return json({ message: 'run not found' }, 404); run.state = 'scheduled'; run.env_version = run.target_version; return json(run); }),
  http.post('/runs/:id/execute', ({ params }) => {
    const run = runs.get(parseID(params.id as string));
    if (!run) return json({ message: 'run not found' }, 404);
    run.state = 'done'; run.started_at = new Date().toISOString(); run.ended_at = new Date().toISOString();
    const runResults = run.selected_cases.map((caseID) => ({ id: ++nextID, team_id: teamID, run_id: run.id, case_id: caseID, case_version: cases.find((item) => item.id === caseID)?.current_version ?? 1, status: 'pass' as const, result_text: 'mock pass', attempt_seq: 1, started_at: run.started_at!, ended_at: run.ended_at }));
    results.set(run.id, runResults);
    return json({ run, results: runResults });
  }),
  http.get('/runs/:id/case-results', ({ params }) => json(results.get(parseID(params.id as string)) ?? [])),
  http.post('/runs/:id/pause', ({ params }) => { const run = runs.get(parseID(params.id as string)); if (!run) return json({ message: 'run not found' }, 404); run.state = 'paused'; return json(run); }),
  http.post('/runs/:id/resume', ({ params }) => { const run = runs.get(parseID(params.id as string)); if (!run) return json({ message: 'run not found' }, 404); run.state = 'running'; return json(run); }),
  http.get('/runs/:id/events', ({ params }) => {
    const run = runs.get(parseID(params.id as string));
    if (!run) return json({ message: 'run not found' }, 404);
    const payload = `id: 1\nevent: run\ndata: ${JSON.stringify(run)}\n\n`;
    return new HttpResponse(new Blob([payload]), { headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' } });
  }),
  http.post('/gates/evaluate', async ({ request }) => { const body = await request.json() as { run_id: number }; const result: GateResult = { id: ++nextID, team_id: teamID, run_id: body.run_id, rule_id: 1, result: 'pass', detail: { mock: true }, decided_at: new Date().toISOString() }; gateResults.set(body.run_id, [result]); return json(result); }),
  http.get('/gates/results', ({ request }) => json(gateResults.get(Number(new URL(request.url).searchParams.get('run_id'))) ?? [])),
  http.get('/cost/overview', () => { const overview: CostOverview = { total_amount: 0, by_category: {}, by_point: {} }; return json(overview); }),
  http.get('/cost/items', () => json([] as CostLineItem[])),
  http.get('/cost/compare/:caseId', () => json([] as CostLineItem[])),
  http.post('/reports/generate', async ({ request }) => { const body = await request.json() as { run_id: number; kind?: string }; return json({ id: ++nextID, team_id: teamID, run_id: body.run_id, target_id: 1003, kind: body.kind ?? 'service', title: 'Mock report', version: 'mock', branch: 'main', scenario: 'api', status: 'done', summary: {}, html: '<html><body>Mock report</body></html>', created_at: new Date().toISOString() }); }),
  http.get('/reports/:id/export', () => new HttpResponse('<html><body>Mock report</body></html>', { headers: { 'Content-Type': 'text/html' } })),
];

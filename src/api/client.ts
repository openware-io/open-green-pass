import type { components, operations, paths } from './generated';

export type Target = components['schemas']['Target'];
export type TestCase = components['schemas']['TestCase'];
export type Run = components['schemas']['Run'];
export type CaseResult = components['schemas']['CaseResult'];
export type Report = components['schemas']['Report'];
export type ReportBundle = components['schemas']['ReportBundle'];

export type CreateTargetInput = paths['/targets']['post']['requestBody']['content']['application/json'];
export type CreateCaseInput = paths['/cases']['post']['requestBody']['content']['application/json'];
export type CreateCaseVersionInput = paths['/cases/{id}/versions']['post']['requestBody']['content']['application/json'];
export type CreateRunInput = paths['/runs']['post']['requestBody']['content']['application/json'];
export type CheckVersionInput = paths['/targets/{id}/version-check']['post']['requestBody']['content']['application/json'];

export class GreenPassApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
    this.name = 'GreenPassApiError';
  }
}

export interface GreenPassClientOptions {
  baseUrl?: string;
  teamId: number;
  fetcher?: typeof fetch;
}

type JsonResponse<T> = Promise<T>;

/** Typed HTTP client whose request and response shapes come exclusively from api/openapi.yaml. */
export class GreenPassClient {
  private readonly baseUrl: string;
  private readonly teamId: number;
  private readonly fetcher: typeof fetch;

  constructor(options: GreenPassClientOptions) {
    this.baseUrl = (options.baseUrl ?? import.meta.env.VITE_GP_API_BASE ?? '').replace(/\/$/, '');
    this.teamId = options.teamId;
    this.fetcher = options.fetcher ?? fetch;
  }

  private async request<T>(path: string, init: RequestInit = {}): JsonResponse<T> {
    const headers = new Headers(init.headers);
    headers.set('Accept', 'application/json');
    headers.set('x-gp-team-id', String(this.teamId));
    if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
    const response = await this.fetcher(`${this.baseUrl}${path}`, { ...init, headers });
    if (!response.ok) {
      let message = response.statusText;
      try {
        const body = await response.json() as { message?: string; error?: string };
        message = body.message ?? body.error ?? message;
      } catch {
        // Keep the HTTP status text when the server did not return JSON.
      }
      throw new GreenPassApiError(response.status, message);
    }
    if (response.status === 204) return undefined as T;
    return response.json() as Promise<T>;
  }

  private async requestText(path: string, init: RequestInit = {}): Promise<string> {
    const headers = new Headers(init.headers);
    headers.set('Accept', 'text/html');
    headers.set('x-gp-team-id', String(this.teamId));
    const response = await this.fetcher(`${this.baseUrl}${path}`, { ...init, headers });
    if (!response.ok) throw new GreenPassApiError(response.status, response.statusText);
    return response.text();
  }

  health(): JsonResponse<{ status: string }> { return this.request('/healthz'); }
  listTargets(): JsonResponse<Target[]> { return this.request('/targets'); }
  createTarget(input: CreateTargetInput): JsonResponse<Target> {
    return this.request('/targets', { method: 'POST', body: JSON.stringify(input) });
  }
  attachRepo(targetId: number, input: paths['/targets/{id}/repo']['post']['requestBody']['content']['application/json']) {
    return this.request<{ status: string }>(`/targets/${targetId}/repo`, { method: 'POST', body: JSON.stringify(input) });
  }
  listCases(params: NonNullable<paths['/cases']['get']['parameters']['query']> = {}): JsonResponse<TestCase[]> {
    const query = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => { if (value !== undefined) query.set(key, String(value)); });
    return this.request(`/cases${query.size ? `?${query}` : ''}`);
  }
  createCase(input: CreateCaseInput): JsonResponse<TestCase> {
    return this.request('/cases', { method: 'POST', body: JSON.stringify(input) });
  }
  createCaseVersion(caseId: number, input: CreateCaseVersionInput): JsonResponse<TestCase> {
    return this.request(`/cases/${caseId}/versions`, { method: 'POST', body: JSON.stringify(input) });
  }
  caseHistory(caseId: number): JsonResponse<unknown[]> { return this.request(`/cases/${caseId}/history`); }
  deleteCase(caseId: number): JsonResponse<{ status: string }> {
    return this.request(`/cases/${caseId}`, { method: 'DELETE' });
  }
  registerRuntime(targetId: number, input: paths['/targets/{id}/env/runtime']['post']['requestBody']['content']['application/json']) {
    return this.request<Record<string, unknown>>(`/targets/${targetId}/env/runtime`, { method: 'POST', body: JSON.stringify(input) });
  }
  checkVersion(targetId: number, input: CheckVersionInput): JsonResponse<operations['checkVersion']['responses'][200]['content']['application/json']> {
    return this.request(`/targets/${targetId}/version-check`, { method: 'POST', body: JSON.stringify(input) });
  }
  createRun(input: CreateRunInput): JsonResponse<Run> { return this.request('/runs', { method: 'POST', body: JSON.stringify(input) }); }
  getRun(runId: number): JsonResponse<Run> { return this.request(`/runs/${runId}`); }
  startVersionCheck(runId: number): JsonResponse<Run> { return this.request(`/runs/${runId}/version-check`, { method: 'POST' }); }
  executeRun(runId: number): JsonResponse<operations['executeRun']['responses'][200]['content']['application/json']> {
    return this.request(`/runs/${runId}/execute`, { method: 'POST' });
  }
  pauseRun(runId: number): JsonResponse<Run> { return this.request(`/runs/${runId}/pause`, { method: 'POST' }); }
  resumeRun(runId: number): JsonResponse<Run> { return this.request(`/runs/${runId}/resume`, { method: 'POST' }); }
  caseResults(runId: number): JsonResponse<CaseResult[]> { return this.request(`/runs/${runId}/case-results`); }
  gateResults(runId: number): JsonResponse<unknown[]> { return this.request(`/gates/results?run_id=${runId}`); }
  costOverview(): JsonResponse<Record<string, unknown>> { return this.request('/cost/overview'); }
  generateReport(runId: number, kind = 'service'): JsonResponse<Report> {
    return this.request('/reports/generate', { method: 'POST', body: JSON.stringify({ run_id: runId, kind }) });
  }
  reportHTML(reportId: number): Promise<string> {
    return this.requestText(`/reports/${reportId}/export?fmt=html`);
  }
  bundleReports(reportIds: number[], format: 'html' | 'markdown' = 'html'): JsonResponse<ReportBundle> {
    return this.request('/reports/bundle', { method: 'POST', body: JSON.stringify({ report_ids: reportIds, format }) });
  }
}

export function createGreenPassClient(teamId: number, baseUrl?: string): GreenPassClient {
  return new GreenPassClient({ teamId, baseUrl });
}

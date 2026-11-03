import { AlertTriangle, CheckCircle2, CircleAlert, Loader2, Play, RefreshCw, ShieldCheck } from 'lucide-react';
import type { GenerationPlan, GenerationPlanUnit } from '@/api/client';
import { Card } from '@/components/shared';
import {
  formatCandidateRange,
  formatDuration,
  formatTokens,
} from './types';
import { planCostView } from './planPresentation';
import type { PlanStatus } from './hooks/useGenerationPlan';

export type PlanPreviewProps = {
  plan: GenerationPlan | null;
  status: PlanStatus;
  error: string;
  errorCode?: string;
  errorStatus?: number;
  stale: boolean;
  canPreview: boolean;
  previewDisabledReason?: string;
  onPreview: () => void;
  onConfirm: () => void;
  serviceNameById: Map<number, string>;
};

function severityClass(severity: string): string {
  if (severity === 'error') return 'bg-red-50 text-red-700 border-red-200';
  if (severity === 'warning') return 'bg-amber-50 text-amber-700 border-amber-200';
  return 'bg-slate-50 text-slate-600 border-slate-200';
}

function unitLabel(unit: GenerationPlanUnit, serviceNameById: Map<number, string>): string {
  return serviceNameById.get(unit.service_id) ?? `服务 #${unit.service_id}`;
}

/** Step 2: read-only rendering of the server-side preflight plan. */
export function PlanPreview({ plan, status, error, errorCode, errorStatus, stale, canPreview, previewDisabledReason, onPreview, onConfirm, serviceNameById }: PlanPreviewProps) {
  const loading = status === 'loading';
  const invalidCount = plan?.invalid.length ?? 0;
  const validUnits = plan?.units.filter((unit) => unit.valid) ?? [];
  const costView = plan ? planCostView(plan) : null;
  const creatable = Boolean(plan && !stale && invalidCount === 0 && validUnits.length > 0);
  const confirmBlockedReason = !plan
    ? '请先预检生成计划'
    : stale
      ? '配置已变更，请重新预检'
      : invalidCount > 0
        ? '存在无效服务，修复后重新预检'
        : validUnits.length === 0
          ? '没有可生成的服务'
          : '';

  return (
    <Card
      title={<span className="inline-flex items-center gap-2"><ShieldCheck className="h-4 w-4 text-emerald-600" />生成计划</span>}
      className="mb-5 p-5"
      extra={
        <button
          type="button"
          onClick={onPreview}
          disabled={loading || !canPreview}
          aria-busy={loading}
          className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          {loading ? '预检中…' : plan ? '重新预检' : '预检生成计划'}
        </button>
      }
    >
      <div aria-live="polite" className="min-h-[1px]">
        {!canPreview && previewDisabledReason && status !== 'loading' && (
          <p className="text-xs text-slate-500">{previewDisabledReason}</p>
        )}

        {status === 'idle' && canPreview && (
          <p className="text-xs text-slate-500">
            预检会在服务端校验项目范围、服务画像、仓库读取连接、代码引用与基线，并返回预计批次数、候选区间、tokens、成本与时长。预检不会创建任务或消耗 tokens。
          </p>
        )}

        {status === 'error' && (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">
            <div className="flex items-start gap-2">
              <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" />
              <div className="min-w-0 flex-1">
                <div className="font-medium">预检失败{errorStatus ? `（HTTP ${errorStatus}）` : ''}</div>
                <div className="mt-1 break-words">{error || '请稍后重试'}</div>
                {errorCode && <div className="mt-1 font-mono text-[10px] text-red-500">code: {errorCode}</div>}
              </div>
            </div>
          </div>
        )}

        {plan && (
          <div className="space-y-4">
            {stale && (
              <div role="status" className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
                配置已变更，当前计划不再可用。请重新预检后再确认创建。
              </div>
            )}

            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <div className="rounded-xl border border-emerald-100 bg-emerald-50/60 px-4 py-3">
                <div className="text-[10px] text-emerald-700">可生成服务</div>
                <div className="mt-1 text-2xl font-semibold text-slate-800">{validUnits.length}</div>
                <div className="text-[10px] text-slate-500">{invalidCount > 0 ? `${invalidCount} 个无效` : '全部有效'}</div>
              </div>
              <div className="rounded-xl border border-slate-100 bg-slate-50 px-4 py-3">
                <div className="text-[10px] text-slate-500">生成批次</div>
                <div className="mt-1 text-2xl font-semibold text-slate-800">{plan.summary.batches}</div>
                <div className="text-[10px] text-slate-500">候选 {formatCandidateRange(plan.summary.candidates_min, plan.summary.candidates_max)}</div>
              </div>
              <div className="rounded-xl border border-slate-100 bg-slate-50 px-4 py-3">
                <div className="text-[10px] text-slate-500">预计 tokens</div>
                <div className="mt-1 text-2xl font-semibold text-slate-800">{plan.estimate.unknown ? '未知' : formatTokens(plan.estimate)}</div>
                <div className="text-[10px] text-slate-500">输入 / 输出（按历史批次估算）</div>
              </div>
              <div className="rounded-xl border border-slate-100 bg-slate-50 px-4 py-3">
                <div className="text-[10px] text-slate-500">预计成本 · 时长</div>
                <div className="mt-1 text-2xl font-semibold text-slate-800">{costView?.text ?? '未知'}</div>
                <div className="text-[10px] text-slate-500">
                  {[costView?.hint, plan.estimate.unknown ? '时长未知' : formatDuration(plan.estimate.duration_seconds)].filter(Boolean).join(' · ')}
                </div>
              </div>
            </div>

            {invalidCount > 0 && (
              <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3">
                <div className="flex items-center gap-2 text-xs font-medium text-red-700"><AlertTriangle className="h-4 w-4" />无效服务（{invalidCount}）</div>
                <ul className="mt-2 space-y-1 text-[11px] text-red-700">
                  {plan.invalid.map((item, index) => (
                    <li key={`${item.service_id}-${item.code}-${index}`}>
                      <span className="font-medium">{serviceNameById.get(item.service_id) ?? `服务 #${item.service_id}`}</span>
                      <span className="ml-2 font-mono text-[10px] text-red-500">{item.code}</span>
                      <span className="ml-2">{item.message}</span>
                    </li>
                  ))}
                </ul>
                <p className="mt-2 text-[10px] text-red-500">修复后请重新预检；存在无效服务时不能创建任务。</p>
              </div>
            )}

            {plan.risks.length > 0 && (
              <ul className="space-y-1.5">
                {plan.risks.map((risk, index) => (
                  <li key={`${risk.code}-${index}`} className={'flex items-start gap-2 rounded-lg border px-3 py-2 text-[11px] ' + severityClass(risk.severity)}>
                    <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                    <span><span className="font-mono text-[10px] opacity-80">{risk.code}</span> · {risk.message}</span>
                  </li>
                ))}
              </ul>
            )}

            <div className="overflow-x-auto rounded-lg border border-slate-200">
              <table className="w-full text-xs">
                <caption className="sr-only">预检计划明细</caption>
                <thead className="bg-slate-50 text-left text-slate-500">
                  <tr>
                    <th scope="col" className="px-3 py-2 font-medium">服务</th>
                    <th scope="col" className="px-3 py-2 font-medium">仓库</th>
                    <th scope="col" className="px-3 py-2 font-medium">代码引用</th>
                    <th scope="col" className="px-3 py-2 font-medium">冻结 SHA</th>
                    <th scope="col" className="px-3 py-2 font-medium">状态</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {plan.units.map((unit) => (
                    <tr key={`${unit.service_id}-${unit.repo_id}`} className={unit.valid ? '' : 'bg-red-50/40'}>
                      <td className="px-3 py-2 font-medium text-slate-700">{unitLabel(unit, serviceNameById)}</td>
                      <td className="max-w-[220px] truncate px-3 py-2 text-slate-500" title={unit.repo_url ?? ''}>{unit.repo_url ?? `仓库 #${unit.repo_id}`}</td>
                      <td className="px-3 py-2 font-mono text-[11px] text-slate-600">{unit.ref_type === 'tag' ? 'Tag' : '分支'} {unit.ref_name}</td>
                      <td className="px-3 py-2 font-mono text-[11px] text-slate-500">{unit.commit_sha ? unit.commit_sha.slice(0, 12) : '创建时冻结'}</td>
                      <td className="px-3 py-2">
                        {unit.valid
                          ? <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] text-emerald-700"><CheckCircle2 className="h-3 w-3" />可生成</span>
                          : <span className="inline-flex items-center gap-1 rounded-full bg-red-50 px-2 py-0.5 text-[10px] text-red-600"><CircleAlert className="h-3 w-3" />{unit.error || '不可生成'}</span>}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-slate-200 bg-slate-50 px-4 py-3">
              <p className="text-[11px] text-slate-500">
                确认后创建 1 个生成任务 / {plan.summary.batches} 个服务批次，立即开始消耗 tokens；生成完成需人工审核后才会入库。
              </p>
              <div className="flex items-center gap-2">
                {confirmBlockedReason && <span className="text-[10px] text-amber-700">{confirmBlockedReason}</span>}
                <button
                  type="button"
                  onClick={onConfirm}
                  disabled={!creatable}
                  className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-xs font-medium text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Play className="h-3.5 w-3.5" />确认创建
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </Card>
  );
}

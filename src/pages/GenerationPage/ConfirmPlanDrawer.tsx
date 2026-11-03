import { AlertTriangle, Loader2, Play, RefreshCw } from 'lucide-react';
import type { GenerationPlan } from '@/api/client';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogTitle } from '@/components/ui/dialog';
import {
  formatCandidateRange,
  formatDateTime,
  formatDuration,
  formatTokens,
  SCOPE_LABEL,
  type GenerationScope,
} from './types';
import { planCostView } from './planPresentation';

export type ConfirmPlanDrawerProps = {
  open: boolean;
  plan: GenerationPlan | null;
  scope: GenerationScope;
  serviceNameById: Map<number, string>;
  busy: boolean;
  error: string;
  errorCode?: string;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  onRepreview: () => void;
};

/**
 * Step 3: modal confirmation (Radix Dialog: focus trap, Esc to close, focus
 * restored to the trigger). Read-only: no selector is duplicated here.
 */
export function ConfirmPlanDrawer({ open, plan, scope, serviceNameById, busy, error, errorCode, onOpenChange, onConfirm, onRepreview }: ConfirmPlanDrawerProps) {
  const planStale = errorCode === 'plan_stale';
  const conflict = errorCode === 'idempotency_conflict';
  const costView = planCostView(plan ?? {});
  return (
    <Dialog open={open} onOpenChange={(next) => { if (!busy) onOpenChange(next); }}>
      <DialogContent
        className="fixed right-0 left-auto top-0 h-full max-h-[100dvh] w-full max-w-2xl translate-x-0 translate-y-0 gap-0 overflow-y-auto rounded-none border-l p-0"
        showCloseButton={!busy}
      >
        <div className="sticky top-0 z-10 border-b border-slate-200 bg-white px-6 py-4">
          <DialogTitle className="text-base font-semibold text-slate-800">确认创建生成任务</DialogTitle>
          <DialogDescription className="mt-1 text-xs text-slate-500">
            计划 #{plan?.plan_id ?? '—'} · 范围 {SCOPE_LABEL[scope]} · 确认后立即创建并开始消耗 tokens
            {plan?.plan_hash && <span className="ml-2 font-mono text-[10px] text-slate-400" title={plan.plan_hash}>hash {plan.plan_hash.slice(0, 20)}…</span>}
          </DialogDescription>
        </div>

        <div className="space-y-4 px-6 py-4">
          {!plan && <div className="text-xs text-slate-500">计划已失效，请关闭后重新预检。</div>}
          {plan && (
            <>
              <dl className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
                  <dt className="text-[10px] text-slate-500">服务 / 批次</dt>
                  <dd className="mt-0.5 text-sm font-semibold text-slate-800">{plan.summary.services} / {plan.summary.batches}</dd>
                </div>
                <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
                  <dt className="text-[10px] text-slate-500">候选区间</dt>
                  <dd className="mt-0.5 text-sm font-semibold text-slate-800">{formatCandidateRange(plan.summary.candidates_min, plan.summary.candidates_max)}</dd>
                </div>
                <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
                  <dt className="text-[10px] text-slate-500">tokens（入/出）</dt>
                  <dd className="mt-0.5 text-sm font-semibold text-slate-800">{plan.estimate.unknown ? '未知' : formatTokens(plan.estimate)}</dd>
                </div>
                <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
                  <dt className="text-[10px] text-slate-500">预计成本</dt>
                  <dd className="mt-0.5 text-sm font-semibold text-slate-800">{costView.text}</dd>
                  {costView.hint && <dd className="text-[10px] text-amber-700">{costView.hint}</dd>}
                </div>
                <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
                  <dt className="text-[10px] text-slate-500">预计时长</dt>
                  <dd className="mt-0.5 text-sm font-semibold text-slate-800">{plan.estimate.unknown ? '未知' : formatDuration(plan.estimate.duration_seconds)}</dd>
                </div>
                <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
                  <dt className="text-[10px] text-slate-500">计划有效期</dt>
                  <dd className="mt-0.5 text-sm font-semibold text-slate-800">{formatDateTime(plan.expires_at)}</dd>
                </div>
              </dl>

              <div className="overflow-x-auto rounded-lg border border-slate-200">
                <table className="w-full text-xs">
                  <caption className="sr-only">将要创建的服务批次</caption>
                  <thead className="bg-slate-50 text-left text-slate-500">
                    <tr>
                      <th scope="col" className="px-3 py-2 font-medium">服务</th>
                      <th scope="col" className="px-3 py-2 font-medium">仓库</th>
                      <th scope="col" className="px-3 py-2 font-medium">代码引用</th>
                      <th scope="col" className="px-3 py-2 font-medium">冻结 SHA</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {plan.units.map((unit) => (
                      <tr key={`${unit.service_id}-${unit.repo_id}`}>
                        <td className="px-3 py-2 font-medium text-slate-700">{serviceNameById.get(unit.service_id) ?? `服务 #${unit.service_id}`}</td>
                        <td className="max-w-[220px] truncate px-3 py-2 text-slate-500" title={unit.repo_url ?? ''}>{unit.repo_url ?? `仓库 #${unit.repo_id}`}</td>
                        <td className="px-3 py-2 font-mono text-[11px] text-slate-600">{unit.ref_type === 'tag' ? 'Tag' : '分支'} {unit.ref_name}</td>
                        <td className="px-3 py-2 font-mono text-[11px] text-slate-500">{unit.commit_sha ? unit.commit_sha.slice(0, 12) : '创建时冻结'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {plan.risks.length > 0 && (
                <ul className="space-y-1.5">
                  {plan.risks.map((risk, index) => (
                    <li key={`${risk.code}-${index}`} className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] text-amber-800">
                      <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                      <span><span className="font-mono text-[10px]">{risk.code}</span> · {risk.message}</span>
                    </li>
                  ))}
                </ul>
              )}

              <p className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] text-amber-800">
                确认后将创建 1 个生成任务 / {plan.summary.batches} 个服务批次并立即开始；生成完成后必须人工审核才会写入用例库。
              </p>
            </>
          )}

          {error && (
            <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
              <div className="font-medium">
                {planStale ? '计划已过期或配置已变化（plan_stale）' : conflict ? '幂等冲突（idempotency_conflict）' : '创建失败'}
              </div>
              <div className="mt-1 break-words">{error}</div>
              {(planStale || conflict) && (
                <button type="button" disabled={busy} onClick={onRepreview} className="mt-2 inline-flex items-center gap-1 font-medium text-red-700 underline disabled:opacity-50">
                  <RefreshCw className="h-3 w-3" />重新预检生成计划
                </button>
              )}
            </div>
          )}
        </div>

        <DialogFooter className="sticky bottom-0 border-t border-slate-200 bg-white px-6 py-4 sm:justify-end">
          <button
            type="button"
            disabled={busy}
            onClick={() => onOpenChange(false)}
            className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-xs text-slate-600 hover:bg-slate-50 disabled:opacity-50"
          >
            取消
          </button>
          <button
            type="button"
            disabled={busy || !plan || planStale || conflict}
            aria-busy={busy}
            onClick={onConfirm}
            className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-xs font-medium text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
            {busy ? '正在创建…' : '确认创建并开始'}
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

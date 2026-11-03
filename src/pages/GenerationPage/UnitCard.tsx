import { CheckCircle2, CircleAlert, CircleX, Clock, GitBranch, Loader2, Server } from 'lucide-react';
import type { GenerationRunUnitDetail } from '@/api/client';
import { BATCH_STAGE_LABEL, BATCH_STAGE_ORDER, UNIT_STATE_LABEL } from './types';

function unitBadgeClass(state: GenerationRunUnitDetail['state']): string {
  if (state === 'succeeded') return 'bg-emerald-50 text-emerald-700';
  if (state === 'failed') return 'bg-red-50 text-red-600';
  if (state === 'rejected') return 'bg-red-50 text-red-700 ring-1 ring-inset ring-red-200';
  if (state === 'waiting_review') return 'bg-amber-50 text-amber-700';
  if (state === 'cancelled') return 'bg-slate-200 text-slate-500';
  if (state === 'running') return 'bg-blue-50 text-blue-700';
  return 'bg-slate-100 text-slate-500';
}

function unitIcon(state: GenerationRunUnitDetail['state']) {
  if (state === 'succeeded') return <CheckCircle2 className="h-3.5 w-3.5" />;
  if (state === 'failed') return <CircleAlert className="h-3.5 w-3.5" />;
  if (state === 'rejected') return <CircleX className="h-3.5 w-3.5" />;
  if (state === 'waiting_review') return <Clock className="h-3.5 w-3.5" />;
  if (state === 'running') return <Loader2 className="h-3.5 w-3.5 animate-spin" />;
  return <Clock className="h-3.5 w-3.5" />;
}

/** One service card inside the run console. */
export function UnitCard({ unit }: { unit: GenerationRunUnitDetail }) {
  // task-20 adds the backend-projected `stage`; keep batch_state as fallback for
  // stores/snapshots produced by older builds (e.g. 0.1.221).
  const stage = unit.stage ?? unit.batch_state;
  const stageIndex = stage ? BATCH_STAGE_ORDER.indexOf(stage as typeof BATCH_STAGE_ORDER[number]) : -1;
  const stageLabel = stage ? (BATCH_STAGE_LABEL[stage] ?? stage) : '等待批次创建';
  const failed = unit.state === 'failed';
  const rejected = unit.state === 'rejected';
  return (
    <article className="rounded-xl border border-slate-200 bg-white px-4 py-3" aria-label={`服务 ${unit.service_name ?? unit.service_id}`}>
      <div className="flex flex-wrap items-center gap-2">
        <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-700">
          <Server className="h-3.5 w-3.5 text-slate-400" />{unit.service_name ?? `服务 #${unit.service_id}`}
        </span>
        <span className={'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium ' + unitBadgeClass(unit.state)}>
          {unitIcon(unit.state)}{UNIT_STATE_LABEL[unit.state] ?? unit.state}
        </span>
        <span className="rounded bg-slate-100 px-2 py-0.5 text-[10px] text-slate-600">{stageLabel}</span>
        {unit.review_required && unit.state !== 'succeeded' && unit.state !== 'cancelled' && (
          <span className="rounded bg-amber-50 px-2 py-0.5 text-[10px] text-amber-700">生成后需人工审核</span>
        )}
        {unit.state === 'waiting_review' && <span className="text-[10px] font-medium text-amber-700">等待人工审核</span>}
        {rejected && <span className="text-[10px] font-medium text-red-600">审核拒绝 · 未写入用例库</span>}
        <span className="ml-auto font-mono text-[10px] text-slate-400">{unit.batch_id ? `批次 #${unit.batch_id}` : '批次待创建'}</span>
      </div>

      <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[10px] text-slate-500">
        <span className="inline-flex min-w-0 items-center gap-1">
          <GitBranch className="h-3 w-3 shrink-0 text-slate-400" />
          <span className="truncate" title={unit.repo_url ?? ''}>{unit.repo_url ?? `仓库 #${unit.repo_id}`}</span>
        </span>
        <span className="font-mono">{unit.ref_type === 'tag' ? 'Tag' : '分支'} {unit.ref_name}</span>
        <span className="font-mono">SHA {unit.commit_sha ? unit.commit_sha.slice(0, 12) : '冻结中'}</span>
      </div>

      {stageIndex >= 0 && (
        <ol className="mt-2 flex flex-wrap items-center gap-1" aria-label="生成阶段">
          {BATCH_STAGE_ORDER.map((item, index) => {
            const done = index < stageIndex || stage === 'approved';
            const active = item === stage;
            return (
              <li key={item} className="flex items-center gap-1">
                {index > 0 && <span className="text-slate-200">→</span>}
                <span className={'rounded px-1.5 py-0.5 text-[9px] ' + (active ? 'bg-emerald-600 font-medium text-white' : done ? 'bg-emerald-50 text-emerald-600' : 'bg-slate-50 text-slate-300')}>
                  {BATCH_STAGE_LABEL[item]}
                </span>
              </li>
            );
          })}
        </ol>
      )}

      {unit.error && (
        <p className="mt-2 rounded-md border border-red-200 bg-red-50 px-2 py-1 text-[10px] text-red-700" role="alert">
          {failed ? '生成失败：' : rejected ? '已拒绝：' : unit.state === 'cancelled' ? '已取消：' : ''}{unit.error}
        </p>
      )}
    </article>
  );
}

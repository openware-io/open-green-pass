import { useState } from 'react';
import { AlertTriangle, ArrowRight, Ban, CheckCircle2, History, Loader2, Radio, RefreshCw, ShieldCheck, Timer, WifiOff, XCircle } from 'lucide-react';
import type { GenerationRunDetail, GreenPassClient } from '@/api/client';
import { Card } from '@/components/shared';
import { BatchReviewPanel } from './BatchReviewPanel';
import { UnitCard } from './UnitCard';
import type { RunConnectionState } from './hooks/useGenerationRunEvents';
import { runProgressCaption, runProgressView } from './runProgress';
import { formatCost, formatDateTime, formatTokens, isActiveRunState, isTerminalRunState, RUN_STATE_LABEL, SCOPE_LABEL } from './types';

export type RunConsoleProps = {
  client: GreenPassClient;
  run: GenerationRunDetail | null;
  /** Fallback scope when the detail payload omits it (frozen contract detail). */
  scopeFallback: GenerationRunDetail['scope_type'];
  connection: RunConnectionState;
  streamError: string;
  busy: boolean;
  onRefresh: () => void;
  onCancel: () => void;
  onReview: (decision: 'approve' | 'reject') => void;
  onRegenerate: () => void;
  onOpenHistory: () => void;
};

const CONNECTION_LABEL: Record<RunConnectionState, string> = {
  idle: '未连接',
  connecting: '正在连接实时事件…',
  live: '实时事件已连接',
  reconnecting: '实时连接中断，正在重连…',
  fallback: '实时事件不可用 · 已加载一次快照（不轮询）',
  closed: '任务已结束，事件流已关闭',
};

function runBadgeClass(state: GenerationRunDetail['state']): string {
  if (state === 'succeeded') return 'bg-emerald-100 text-emerald-700';
  if (state === 'failed') return 'bg-red-100 text-red-700';
  if (state === 'partial_failed') return 'bg-amber-100 text-amber-700';
  if (state === 'waiting_review') return 'bg-amber-100 text-amber-700';
  if (state === 'cancelled') return 'bg-slate-200 text-slate-500';
  return 'bg-blue-50 text-blue-700';
}

/** Step 4: the live run console (SSE driven, never polled). */
export function RunConsole({ client, run, scopeFallback, connection, streamError, busy, onRefresh, onCancel, onReview, onRegenerate, onOpenHistory }: RunConsoleProps) {
  const [confirmCancel, setConfirmCancel] = useState(false);
  if (!run) {
    return (
      <Card title="生成任务" className="mb-5 p-6">
        <div className="flex items-center gap-2 text-xs text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />正在读取生成任务详情…</div>
        {streamError && (
          <div role="alert" className="mt-3 flex flex-wrap items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] text-amber-800">
            <AlertTriangle className="h-3.5 w-3.5" />{streamError}
            <button type="button" onClick={onRefresh} className="inline-flex items-center gap-1 underline"><RefreshCw className="h-3 w-3" />重试读取</button>
          </div>
        )}
      </Card>
    );
  }
  const active = isActiveRunState(run.state);
  const terminal = isTerminalRunState(run.state);
  const scopeType = run.scope_type ?? scopeFallback;
  const progressView = runProgressView(run);
  const progress = progressView.percent;
  const progressCaption = runProgressCaption(progressView);
  const connectionTone = connection === 'live'
    ? 'text-emerald-700 bg-emerald-50'
    : connection === 'fallback' || connection === 'reconnecting'
      ? 'text-amber-700 bg-amber-50'
      : 'text-slate-500 bg-slate-100';

  return (
    <div className="space-y-4">
      <Card
        title={<span className="inline-flex items-center gap-2">运行控制台 · 任务 <span className="font-mono">#{run.id}</span></span>}
        className="p-5"
        extra={
          <div className="flex flex-wrap items-center gap-2">
            <span className={'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium ' + connectionTone} aria-live="polite">
              {connection === 'live' ? <Radio className="h-3 w-3" /> : connection === 'fallback' ? <WifiOff className="h-3 w-3" /> : <Timer className="h-3 w-3" />}
              {CONNECTION_LABEL[connection]}
            </span>
            <button type="button" onClick={onRefresh} className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-[10px] text-slate-600 hover:bg-slate-50">
              <RefreshCw className="h-3 w-3" />刷新快照
            </button>
            <button type="button" onClick={onOpenHistory} className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-[10px] text-slate-600 hover:bg-slate-50">
              <History className="h-3 w-3" />历史任务
            </button>
            {active && (confirmCancel ? (
              <>
                <span className="text-[10px] text-red-600">确认取消整个任务？</span>
                <button type="button" disabled={busy} onClick={() => { setConfirmCancel(false); onCancel(); }} className="inline-flex items-center gap-1 rounded-lg border border-red-300 bg-white px-2.5 py-1.5 text-[10px] font-medium text-red-600 hover:bg-red-50 disabled:opacity-50"><Ban className="h-3 w-3" />确认取消</button>
                <button type="button" disabled={busy} onClick={() => setConfirmCancel(false)} className="rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-[10px] text-slate-500 hover:bg-slate-50">返回</button>
              </>
            ) : (
              <button type="button" disabled={busy} onClick={() => setConfirmCancel(true)} className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-2.5 py-1.5 text-[10px] text-red-600 hover:bg-red-50 disabled:opacity-50"><Ban className="h-3 w-3" />取消任务</button>
            ))}
            {terminal && (
              <button type="button" onClick={onRegenerate} className="inline-flex items-center gap-1 rounded-lg border border-emerald-200 bg-white px-2.5 py-1.5 text-[10px] text-emerald-700 hover:bg-emerald-50">
                <RefreshCw className="h-3 w-3" />再次生成
              </button>
            )}
          </div>
        }
      >
        <div className="flex flex-wrap items-center gap-3 text-xs">
          <span className={'rounded-full px-2.5 py-1 font-medium ' + runBadgeClass(run.state)}>{RUN_STATE_LABEL[run.state] ?? run.state}</span>
          <span className="text-slate-500">{SCOPE_LABEL[scopeType]} · {run.total_units} 个服务</span>
          {typeof run.plan_id === 'number' && <span className="font-mono text-[10px] text-slate-400">计划 #{run.plan_id}</span>}
          <span className="text-emerald-700">已入库 {run.completed_units}</span>
          {run.failed_units > 0 && <span className="text-red-600">失败 {run.failed_units}</span>}
          {typeof run.review_units === 'number' && run.review_units > 0 && <span className="text-amber-700">待审核 {run.review_units}</span>}
          <span className="text-slate-400">tokens {formatTokens(run)}</span>
          <span className="text-slate-400">成本 {formatCost(run.cost_amount)}</span>
          <span className="ml-auto text-[10px] text-slate-400">创建于 {formatDateTime(run.created_at)} · 更新于 {formatDateTime(run.updated_at)}</span>
        </div>

        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-slate-100" role="progressbar" aria-valuenow={progress} aria-valuemin={0} aria-valuemax={100} aria-label="任务总进度">
          <div className={'h-full rounded-full transition-all ' + (run.state === 'failed' ? 'bg-red-500' : run.state === 'waiting_review' ? 'bg-amber-500' : 'bg-emerald-500')} style={{ width: `${progress}%` }} />
        </div>
        <div className="mt-1 flex flex-wrap items-center justify-between gap-2 text-[10px] text-slate-500">
          <span>{progressCaption}</span>
          {run.state === 'waiting_review' && <span className="text-amber-700">有服务等待人工审核，审核完成后任务才会结束</span>}
          {run.state === 'partial_failed' && <span className="text-amber-700">部分服务失败，请查看失败原因后重新生成</span>}
          {run.state === 'cancelled' && <span>任务已取消，候选与源码快照已清理。</span>}
          {run.state === 'succeeded' && <span className="inline-flex items-center gap-1 text-emerald-700"><CheckCircle2 className="h-3 w-3" />全部服务已入库</span>}
          {run.state === 'failed' && <span className="inline-flex items-center gap-1 text-red-600"><XCircle className="h-3 w-3" />任务失败</span>}
        </div>

        {streamError && (
          <div role="status" className="mt-3 flex flex-wrap items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] text-amber-800">
            <AlertTriangle className="h-3.5 w-3.5" />{streamError}
          </div>
        )}
      </Card>

      <Card title="服务进度" className="p-4">
        {run.units.length === 0
          ? <div className="px-2 py-6 text-center text-xs text-slate-500">该任务尚未展开服务批次。</div>
          : <div className="grid gap-3 lg:grid-cols-2">{run.units.map((unit) => <UnitCard key={unit.id} unit={unit} />)}</div>}
      </Card>

      <BatchReviewPanel client={client} run={run} busy={busy} onReview={onReview} />

      {terminal && (
        <Card title="后续操作" className="p-4">
          <div className="flex flex-wrap items-center gap-3 text-xs text-slate-600">
            <ShieldCheck className="h-4 w-4 text-emerald-600" />
            <span>需要再次生成时，必须回到配置草稿并重新预检确认，不会直接复用上次创建参数。</span>
            <button type="button" onClick={onRegenerate} className="ml-auto inline-flex items-center gap-1 rounded-lg bg-emerald-600 px-3 py-1.5 text-[11px] font-medium text-white hover:bg-emerald-700">
              回到草稿并再次生成<ArrowRight className="h-3 w-3" />
            </button>
          </div>
        </Card>
      )}
    </div>
  );
}

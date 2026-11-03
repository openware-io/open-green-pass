import { useCallback, useState } from 'react';
import { AlertTriangle, Check, ChevronDown, Loader2 } from 'lucide-react';
import type { GreenPassClient, GenerationRunDetail } from '@/api/client';
import { UNIT_STATE_LABEL } from './types';

type GenerationSeed = {
  id: number;
  batch_id: number;
  case_id?: number | null;
  source_locator: string;
  normalized_spec: Record<string, unknown>;
  quality_state: string;
  approval_state: string;
  dedupe_hash?: string;
};

type SeedLoadState = { loading: boolean; error: string; items: GenerationSeed[] };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function SeedList({ seeds }: { seeds: GenerationSeed[] }) {
  if (!seeds.length) return <p className="px-3 py-2 text-[11px] text-slate-500">该批次暂无可展示的候选用例。</p>;
  return (
    <div className="max-h-72 divide-y divide-slate-100 overflow-y-auto">
      {seeds.map((seed) => {
        const spec = seed.normalized_spec ?? {};
        const request = isRecord(spec.request) ? spec.request : {};
        const assertions = Array.isArray(spec.assertions) ? spec.assertions.filter(isRecord) : [];
        return (
          <div key={seed.id} className="px-3 py-2 text-[11px]">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-mono text-emerald-700">#{seed.id}</span>
              <span className="min-w-0 flex-1 truncate font-medium text-slate-700">{stringValue(spec.title) || '未命名候选'}</span>
              <span className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-[10px] text-slate-600">{stringValue(request.method) || '—'} {stringValue(request.path) || '—'}</span>
              <span className={'rounded px-1.5 py-0.5 text-[10px] ' + (seed.quality_state === 'passed' ? 'bg-emerald-50 text-emerald-700' : seed.quality_state === 'failed' ? 'bg-red-50 text-red-600' : 'bg-amber-50 text-amber-700')}>
                质量 {seed.quality_state}
              </span>
            </div>
            <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-[10px] text-slate-500">
              <span>断言 {assertions.length} 条</span>
              <span className="truncate">源码证据：{stringValue(spec.source_locator) || seed.source_locator}</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

export type BatchReviewPanelProps = {
  client: GreenPassClient;
  run: GenerationRunDetail;
  busy: boolean;
  onReview: (decision: 'approve' | 'reject') => void;
};

/**
 * Aggregated review for the whole run: "全部通过并入库" / "全部拒绝".
 * Candidate seeds are read-only inspection data, loaded once on demand.
 */
export function BatchReviewPanel({ client, run, busy, onReview }: BatchReviewPanelProps) {
  const units = (run.units ?? []).filter((unit) => unit.state === 'waiting_review' || (unit.batch_state === 'review' && unit.state !== 'succeeded'));
  const [expanded, setExpanded] = useState<number | null>(null);
  const [seeds, setSeeds] = useState<Record<number, SeedLoadState>>({});
  const [confirmReject, setConfirmReject] = useState(false);

  const loadSeeds = useCallback(async (batchId: number) => {
    setSeeds((current) => {
      if (current[batchId]?.loading || current[batchId]?.items?.length) return current;
      return { ...current, [batchId]: { loading: true, error: '', items: [] } };
    });
    try {
      const items = await client.listGenerationSeeds(batchId);
      setSeeds((current) => ({ ...current, [batchId]: { loading: false, error: '', items } }));
    } catch (cause) {
      setSeeds((current) => ({ ...current, [batchId]: { loading: false, error: cause instanceof Error ? cause.message : '读取候选用例失败', items: [] } }));
    }
  }, [client]);

  const toggle = (batchId: number) => {
    setExpanded((current) => (current === batchId ? null : batchId));
    if (expanded !== batchId) void loadSeeds(batchId);
  };

  if (!units.length) return null;

  return (
    <section aria-label="等待审核的服务批次" className="rounded-xl border border-amber-200 bg-amber-50/60 px-4 py-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-amber-800">
          <AlertTriangle className="h-4 w-4" />{units.length} 个服务等待审核
        </span>
        <span className="text-[11px] text-amber-700">审核决定会作用于本次任务全部待审核服务；建议先逐项核对候选请求与断言。</span>
        <span className="ml-auto flex flex-wrap items-center gap-2">
          {confirmReject ? (
            <>
              <span className="text-[10px] text-amber-800">确认全部拒绝？</span>
              <button type="button" disabled={busy} onClick={() => { setConfirmReject(false); onReview('reject'); }} className="rounded-lg border border-red-300 bg-white px-3 py-1.5 text-[11px] font-medium text-red-600 hover:bg-red-50 disabled:opacity-50">确认拒绝</button>
              <button type="button" disabled={busy} onClick={() => setConfirmReject(false)} className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-[11px] text-slate-500 hover:bg-slate-50">取消</button>
            </>
          ) : (
            <button type="button" disabled={busy} onClick={() => setConfirmReject(true)} className="rounded-lg border border-red-200 bg-white px-3 py-1.5 text-[11px] text-red-600 hover:bg-red-50 disabled:opacity-50">全部拒绝</button>
          )}
          <button type="button" disabled={busy} onClick={() => onReview('approve')} className="inline-flex items-center gap-1 rounded-lg bg-emerald-600 px-3 py-1.5 text-[11px] font-medium text-white hover:bg-emerald-700 disabled:opacity-50">
            {busy ? <Loader2 className="h-3 w-3 animate-spin" /> : <Check className="h-3 w-3" />}全部通过并入库
          </button>
        </span>
      </div>

      <ul className="mt-3 space-y-2">
        {units.map((unit) => {
          const batchId = unit.batch_id ?? null;
          const open = batchId !== null && expanded === batchId;
          const state = batchId !== null ? seeds[batchId] : undefined;
          return (
            <li key={unit.id} className="rounded-lg border border-amber-100 bg-white">
              <div className="flex flex-wrap items-center gap-2 px-3 py-2 text-[11px]">
                <span className="font-medium text-slate-700">{unit.service_name ?? `服务 #${unit.service_id}`}</span>
                <span className="rounded bg-amber-50 px-1.5 py-0.5 text-[10px] text-amber-700">{UNIT_STATE_LABEL[unit.state] ?? unit.state}</span>
                <span className="font-mono text-[10px] text-slate-500">{unit.ref_type === 'tag' ? 'Tag' : '分支'} {unit.ref_name}</span>
                {batchId !== null
                  ? (
                    <button type="button" onClick={() => toggle(batchId)} aria-expanded={open} className="ml-auto inline-flex items-center gap-1 text-[10px] text-emerald-700 hover:underline">
                      <ChevronDown className={'h-3 w-3 transition-transform ' + (open ? 'rotate-180' : '')} />{open ? '收起候选' : '查看候选'}
                    </button>
                  )
                  : <span className="ml-auto text-[10px] text-slate-400">批次待创建</span>}
                {batchId !== null && <span className="font-mono text-[10px] text-slate-400">批次 #{batchId}</span>}
              </div>
              {open && batchId !== null && (
                <div className="border-t border-slate-100">
                  {state?.loading && <p className="px-3 py-2 text-[11px] text-slate-500">正在读取候选用例…</p>}
                  {state?.error && <p role="alert" className="px-3 py-2 text-[11px] text-red-600">候选用例读取失败：{state.error}</p>}
                  {state && !state.loading && !state.error && <SeedList seeds={state.items} />}
                </div>
              )}
            </li>
          );
        })}
      </ul>
      <p className="mt-2 text-[10px] text-amber-700">通过后本次任务待审核服务会同步到用例库；拒绝不会改变现有正式用例。</p>
    </section>
  );
}

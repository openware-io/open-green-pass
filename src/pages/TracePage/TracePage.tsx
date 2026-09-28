import { useState, type ReactNode } from 'react';
import { useOutletContext } from 'react-router-dom';
import { REQUIREMENTS, TRACE_CHAINS, type IAssetNode } from '@/data/mock';
import { PageHeader, GhostButton, ListFilter as FilterBar } from '@/components/shared';
import { ScanSearch, ShieldCheck, ShieldAlert, ShieldX, ChevronRight, CheckCircle2, CircleX, CircleAlert, Camera, GitBranch, Info } from 'lucide-react';

// 覆盖状态徽章：业务语义 = 该需求是否有用例覆盖
const COVER_BADGE: Record<string, { label: string; cls: string }> = {
  '完整': { label: '已覆盖', cls: 'bg-emerald-50 text-emerald-700 border-emerald-200' },
  '部分': { label: '部分覆盖', cls: 'bg-amber-50 text-amber-700 border-amber-200' },
  '缺口': { label: '覆盖缺口', cls: 'bg-red-50 text-red-700 border-red-200' },
};

// 用例执行结果
const RESULT_ICON: Record<string, { el: ReactNode; cls: string }> = {
  '通过': { el: <CheckCircle2 className="w-3.5 h-3.5" />, cls: 'text-emerald-600' },
  '失败': { el: <CircleX className="w-3.5 h-3.5" />, cls: 'text-red-600' },
  '阻塞': { el: <CircleAlert className="w-3.5 h-3.5" />, cls: 'text-red-600' },
};

// 生成批次（AI 生成产出的归属，弱化为筛选）
const GEN_BATCH: Record<string, string> = { 'REQ-101': '#GEN-2041', 'REQ-102': '#GEN-2041', 'REQ-103': '#GEN-2038', 'REQ-104': '#GEN-2041' };
const GEN_BATCHES = ['全部', '#GEN-2041', '#GEN-2038'];

const serviceToReq = (svc: string): string => {
  if (svc.startsWith('web-') || svc.startsWith('mobile-')) return 'REQ-101';
  return (Object.values(TRACE_CHAINS).find((c) => c.service === svc)?.reqId
    ?? REQUIREMENTS.find((r) => r.assets.includes(svc))?.id
    ?? 'REQ-101');
};

// 覆盖状态排序：缺口最优先
const ORDER: Record<string, number> = { '缺口': 0, '部分': 1, '完整': 2 };

export default function TracePage() {
  const { selectedAsset } = useOutletContext<{ selectedAsset: IAssetNode }>();
  const [activeReq, setActiveReq] = useState('REQ-103'); // 默认选中「部分覆盖」需求，让用户第一眼看到业务问题
  const [gapOnly, setGapOnly] = useState(false);
  const [batch, setBatch] = useState('全部');
  const [q, setQ] = useState('');

  // 被测对象树点击联动（render 期间派生 state）
  const [prevAssetId, setPrevAssetId] = useState(selectedAsset.id);
  if (prevAssetId !== selectedAsset.id) {
    setPrevAssetId(selectedAsset.id);
    const id = selectedAsset.id;
    let svc: string | null = null;
    if (id.startsWith('svc-')) svc = id.split('-').slice(0, 2).join('-');
    else if (id.startsWith('web-') || id.startsWith('mobile-')) svc = id;
    if (svc) { const req = serviceToReq(svc); setActiveReq(req); }
  }

  const kw = q.trim().toLowerCase();
  const rows = REQUIREMENTS
    .filter((r) => batch === '全部' || GEN_BATCH[r.id] === batch)
    .filter((r) => !kw || (r.id + r.title).toLowerCase().includes(kw))
    .filter((r) => !gapOnly || r.traceability !== '完整')
    .sort((a, b) => (ORDER[a.traceability] ?? 9) - (ORDER[b.traceability] ?? 9));

  const covered = REQUIREMENTS.filter((r) => r.traceability === '完整').length;
  const partial = REQUIREMENTS.filter((r) => r.traceability === '部分').length;
  const gap = REQUIREMENTS.filter((r) => r.traceability === '缺口').length;

  const activeMeta = REQUIREMENTS.find((r) => r.id === activeReq);
  const activeChain = TRACE_CHAINS[activeReq];

  return (
    <div>
      <PageHeader title="追溯覆盖" desc="覆盖体检 · 每个需求都被测试覆盖了吗？需求 → 测试点 → 用例 可溯源">
        <GhostButton onClick={() => setGapOnly((g) => !g)}>
          <ScanSearch className="w-4 h-4 mr-1.5" />{gapOnly ? '显示全部' : '只看缺口'}
        </GhostButton>
        <span className="inline-flex items-center gap-1.5 text-[11px] text-slate-500 border border-slate-200 rounded-lg bg-white pl-2.5 pr-1 py-1">
          <ScanSearch className="w-3.5 h-3.5 text-emerald-600" />生成批次
          <select value={batch} onChange={(e) => setBatch(e.target.value)}
            className="bg-transparent text-[11px] font-medium text-slate-600 outline-none cursor-pointer py-0.5">
            {GEN_BATCHES.map((b) => <option key={b} value={b}>{b}</option>)}
          </select>
        </span>
      </PageHeader>

      {/* 一页一焦点 · 体检结论（总览疏） */}
      <div className="mb-6 grid grid-cols-3 gap-5">
        <div className="rounded-xl border border-emerald-200 bg-emerald-50/60 p-5">
          <div className="flex items-center gap-2 text-emerald-700"><ShieldCheck className="w-4 h-4" /><span className="text-sm font-semibold">已覆盖</span></div>
          <div className="mt-2 text-3xl font-bold text-emerald-700">{covered}</div>
          <div className="mt-1 text-[11px] text-emerald-600/80">需求全部有用例覆盖</div>
        </div>
        <div className="rounded-xl border border-amber-200 bg-amber-50/60 p-5">
          <div className="flex items-center gap-2 text-amber-700"><ShieldAlert className="w-4 h-4" /><span className="text-sm font-semibold">部分覆盖</span></div>
          <div className="mt-2 text-3xl font-bold text-amber-700">{partial}</div>
          <div className="mt-1 text-[11px] text-amber-600/80">有用例但未测全，建议补</div>
        </div>
        <div className="rounded-xl border border-red-200 bg-red-50/60 p-5">
          <div className="flex items-center gap-2 text-red-700"><ShieldX className="w-4 h-4" /><span className="text-sm font-semibold">覆盖缺口</span></div>
          <div className="mt-2 text-3xl font-bold text-red-700">{gap}</div>
          <div className="mt-1 text-[11px] text-red-600/80">无用例覆盖，上线盲区优先处理</div>
        </div>
      </div>

      {/* 一句话定位（中性说明） */}
      <div className="mb-6 flex items-start gap-2 text-[11px] text-slate-500 bg-slate-50 border border-slate-200 rounded-xl px-4 py-2.5">
        <Info className="w-4 h-4 text-slate-400 mt-0.5 shrink-0" />
        <span>每个<b>需求</b>必须有测试用例覆盖，否则上线即盲区。清单按覆盖状态排序（缺口置顶）；点选某条需求，下方查看它的 <b>需求 → 测试点 → 用例</b> 覆盖详情。</span>
      </div>

      {/* 需求覆盖清单（适中密度，缺口优先） */}
      <div className="rounded-xl border border-slate-200 bg-white overflow-hidden">
        <div className="px-5 py-4 border-b border-slate-200 flex items-center justify-between flex-wrap gap-2">
          <h2 className="font-semibold text-slate-700 text-sm">需求覆盖清单</h2>
          <div className="flex items-center gap-3">
            <FilterBar search={q} onSearch={setQ} />
            <span className="text-[11px] text-slate-400">共 {rows.length} / {REQUIREMENTS.length} 条需求</span>
          </div>
        </div>
        <div className="divide-y divide-slate-100">
          {rows.map((r) => {
            const b = COVER_BADGE[r.traceability];
            const sel = activeReq === r.id;
            return (
              <button key={r.id} type="button" onClick={() => setActiveReq(r.id)}
                className={'w-full text-left px-5 py-4 flex items-center gap-4 transition ' + (sel ? 'bg-emerald-50/60' : 'hover:bg-slate-50')}>
                <span className="font-mono text-xs font-medium text-slate-700 w-16 shrink-0">{r.id}</span>
                <span className={'flex-1 min-w-0 truncate text-xs ' + (r.traceability === '缺口' ? 'text-red-700 font-medium' : 'text-slate-700')}>{r.title}</span>
                <span className={'text-[11px] px-2.5 py-1 rounded-full border shrink-0 ' + b.cls}>{b.label}</span>
                <span className="flex items-center gap-1.5 text-[11px] text-slate-500 shrink-0">
                  <span className="w-16 h-1.5 bg-slate-100 rounded-full overflow-hidden"><span className={r.executed >= r.cases && r.cases > 0 ? 'bg-emerald-500' : 'bg-amber-500'} style={{ display: 'block', height: '100%', width: r.cases > 0 ? `${(r.executed / r.cases) * 100}%` : '0%' }} /></span>
                  <span className="font-mono">{r.executed}/{r.cases}</span>
                </span>
                <span className="flex flex-wrap gap-1 w-44 shrink-0 justify-end">
                  {r.assets.map((a) => <span key={a} className="text-[10px] bg-slate-50 text-slate-500 px-1.5 py-0.5 rounded">{a}</span>)}
                </span>
                <ChevronRight className={'w-4 h-4 shrink-0 ' + (sel ? 'text-emerald-600' : 'text-slate-300')} />
              </button>
            );
          })}
        </div>
        {gapOnly && (
          <div className="px-5 py-3 text-[11px] text-amber-600 bg-amber-50/40 border-t border-amber-200">已筛选「只看缺口」：仅显示未完全覆盖的需求（缺口 + 部分）。</div>
        )}
      </div>

      {/* 选中需求覆盖详情（详情疏朗：需求 → 测试点 → 用例） */}
      {activeMeta && activeChain && (
        <div className="mt-6 rounded-xl border border-emerald-200 bg-white p-6">
          <div className="flex items-center justify-between flex-wrap gap-2">
            <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-2">
              <GitBranch className="w-4 h-4 text-emerald-600" />{activeMeta.id} · {activeMeta.title}
            </h2>
            <span className={'text-[11px] px-2.5 py-1 rounded-full border ' + COVER_BADGE[activeMeta.traceability].cls}>{COVER_BADGE[activeMeta.traceability].label}</span>
          </div>

          <div className="mt-4 grid grid-cols-3 gap-4">
            <div className="rounded-lg bg-slate-50 border border-slate-200 px-3 py-2.5">
              <div className="text-[10px] text-slate-400 mb-1">测试点</div>
              <div className="text-xs text-slate-700 font-medium">{activeChain.testPoint || '—'}</div>
            </div>
            <div className="rounded-lg bg-slate-50 border border-slate-200 px-3 py-2.5">
              <div className="text-[10px] text-slate-400 mb-1">覆盖用例</div>
              <div className="text-xs text-slate-700 font-medium">{activeChain.tcs.length} 条</div>
            </div>
            <div className="rounded-lg bg-slate-50 border border-slate-200 px-3 py-2.5">
              <div className="text-[10px] text-slate-400 mb-1">覆盖状态</div>
              <div className={'text-xs font-medium ' + (activeMeta.traceability === '缺口' ? 'text-red-600' : activeMeta.traceability === '部分' ? 'text-amber-600' : 'text-emerald-600')}>{COVER_BADGE[activeMeta.traceability].label}</div>
            </div>
          </div>

          {/* 需求 → 测试点 → 用例 覆盖路径 */}
          <div className="mt-5 pt-5 border-t border-slate-100">
            <div className="text-xs font-medium text-slate-600 mb-3 flex items-center gap-1.5"><GitBranch className="w-3.5 h-3.5 text-emerald-500" />需求 → 测试点 → 用例</div>
            {activeChain.tcs.length === 0 ? (
              <div className="rounded-lg border border-red-200 bg-red-50/40 px-4 py-3 text-[11px] text-red-600">覆盖缺口 · 该需求无任何测试用例，需补建用例并绑定上游源。</div>
            ) : (
              <div className="grid grid-cols-2 gap-3">
                {activeChain.tcs.map((tc) => {
                  const ri = RESULT_ICON[tc.result] ?? RESULT_ICON['通过'];
                  return (
                    <div key={tc.id} className="flex items-center gap-3 rounded-lg border border-slate-200 bg-white px-3.5 py-2.5">
                      <span className="font-mono text-[11px] text-slate-500 w-16 shrink-0">{tc.id}</span>
                      <span className="flex-1 text-xs text-slate-700 truncate">{tc.name}</span>
                      <span className={'inline-flex items-center gap-1 text-[11px] shrink-0 ' + ri.cls}>{ri.el}{tc.result}</span>
                      <span className="inline-flex items-center gap-1 text-[10px] text-slate-400 shrink-0"><Camera className="w-3 h-3" />{tc.evidence} 证据</span>
                    </div>
                  );
                })}
              </div>
            )}
            {activeMeta.executed < activeMeta.cases && (
              <div className="mt-3 text-[11px] text-amber-600 flex items-center gap-1.5">
                <ShieldAlert className="w-3.5 h-3.5" />已执行 {activeMeta.executed}/{activeMeta.cases} 用例，剩余未执行，覆盖不完整。
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
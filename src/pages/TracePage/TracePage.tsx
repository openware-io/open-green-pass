import { useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { useAssetLevel, LEVEL_LABEL } from '@/context';
import { REQUIREMENTS, TRACE_CHAINS, type IAssetNode } from '@/data/mock';
import { PageHeader, GhostButton, Card, ListFilter as FilterBar } from '@/components/shared';
import { ScanSearch, ListFilter, CircleCheckBig, CircleAlert, CircleX, Camera, Wallet, Gauge, GitBranch } from 'lucide-react';

const TRACEABILITY_BADGE: Record<string, string> = {
  '完整': 'bg-emerald-50 text-emerald-600',
  '部分': 'bg-amber-50 text-amber-600',
  '缺口': 'bg-red-50 text-red-600',
};

const RESULT_BADGE: Record<string, { cls: string; icon: 'pass' | 'fail' | 'block' }> = {
  '通过': { cls: 'bg-emerald-50 text-emerald-600', icon: 'pass' },
  '失败': { cls: 'bg-red-50 text-red-600', icon: 'fail' },
  '阻塞': { cls: 'bg-amber-50 text-amber-600', icon: 'block' },
};

function ResultIcon({ kind }: { kind: string }) {
  if (kind === 'pass') return <CircleCheckBig className="w-3.5 h-3.5" />;
  if (kind === 'fail') return <CircleX className="w-3.5 h-3.5" />;
  return <CircleAlert className="w-3.5 h-3.5" />;
}

const COLS: Record<string, number> = { system: 2, group: 3, service: 4, module: 5 };

// 需求追溯 = 生成产出的消费视图：每需求所属的 AI 生成批次（呼应上游源与生成的追溯绑定）
const GEN_BATCH: Record<string, string> = { 'REQ-101': '#GEN-2041', 'REQ-102': '#GEN-2041', 'REQ-103': '#GEN-2038', 'REQ-104': '#GEN-2041' };
const GEN_BATCHES = ['全部', '#GEN-2041', '#GEN-2038'];

const serviceToReq = (svc: string): string => {
  if (svc.startsWith('web-') || svc.startsWith('mobile-')) return 'REQ-101';
  return (Object.values(TRACE_CHAINS).find((c) => c.service === svc)?.reqId
    ?? REQUIREMENTS.find((r) => r.assets.includes(svc))?.id
    ?? 'REQ-101');
};

export default function TracePage() {
  const { level } = useAssetLevel();
  const { selectedAsset } = useOutletContext<{ selectedAsset: IAssetNode }>();
  const [activeReq, setActiveReq] = useState('REQ-101');
  const [activeTcId, setActiveTcId] = useState<string | null>('TC-001');

  // 资产树点击联动（render 期间派生 state）：选中服务变化时切换到其相关需求链
  const [prevAssetId, setPrevAssetId] = useState(selectedAsset.id);
  if (prevAssetId !== selectedAsset.id) {
    setPrevAssetId(selectedAsset.id);
    const id = selectedAsset.id;
    let svc: string | null = null;
    if (id.startsWith('svc-')) svc = id.split('-').slice(0, 2).join('-');
    else if (id.startsWith('web-') || id.startsWith('mobile-')) svc = id;
    if (svc) {
      const req = serviceToReq(svc);
      setActiveReq(req);
      setActiveTcId(TRACE_CHAINS[req]?.tcs[0]?.id ?? null);
    }
  }
  const [gapOnly, setGapOnly] = useState(false);
  const [view, setView] = useState<'chain' | 'matrix'>('chain');
  const [batch, setBatch] = useState('全部');

  const chain = TRACE_CHAINS[activeReq];
  const activeTc = activeTcId ? chain.tcs.find((t) => t.id === activeTcId) ?? null : null;

  const [q, setQ] = useState('');
  const kw = q.trim().toLowerCase();
  const rows = (gapOnly ? REQUIREMENTS.filter((r) => r.traceability !== '完整') : REQUIREMENTS)
    .filter((r) => batch === '全部' || GEN_BATCH[r.id] === batch)
    .filter((r) => !kw || (r.id + r.title).toLowerCase().includes(kw));

  return (
    <div>
      <PageHeader title="需求与追溯矩阵" desc="AI 生成产出的追溯矩阵 · 被测对象 × 需求 → 测试点 → 用例 · 按被测对象 / 生成批次过滤">
        <GhostButton onClick={() => setGapOnly((g) => !g)}>
          <ScanSearch className="w-4 h-4 mr-1.5" />{gapOnly ? '显示全部' : '检测覆盖缺口'}
        </GhostButton>
        <GhostButton onClick={() => setView((v) => (v === 'chain' ? 'matrix' : 'chain'))}>
          <ListFilter className="w-4 h-4 mr-1.5" />{view === 'chain' ? '矩阵视图' : '链路视图'}
        </GhostButton>
        <GhostButton onClick={() => setBatch((b) => GEN_BATCHES[(GEN_BATCHES.indexOf(b) + 1) % GEN_BATCHES.length])}>
          <ScanSearch className="w-4 h-4 mr-1.5" />生成批次 · {batch}
        </GhostButton>
      </PageHeader>

      {/* 生成产出追溯说明：追溯链由上游源与生成的 AI 生成种子自动建立 */}
      <div className="mb-5 flex items-start gap-2 text-[11px] text-slate-500 bg-emerald-50/60 border border-emerald-200 rounded-xl px-4 py-2.5">
        <CircleCheckBig className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
        <span>本页是<b>生成产出的追溯矩阵</b>：AI 在上游源与生成页为每条用例种子建立 <b>REQ → 测试点 → 用例</b> 追溯链，此处即该产出的消费视图；可<b>按被测对象</b>（左侧树联动 / 页首层次筛选）与<b>按生成批次</b>（#GEN-2041 / #GEN-2038）过滤，追踪每批 AI 生成的用例覆盖与缺口。</span>
      </div>

      {view === 'chain' && (
        <Card title={<span className="flex items-center gap-1.5"><GitBranch className="w-4 h-4 text-emerald-600" />跨服务追溯链 · {activeReq} {chain.title}<span className="ml-2 text-[10px] bg-emerald-50 text-emerald-600 px-2 py-0.5 rounded-full">当前层级 · {LEVEL_LABEL[level]}</span></span>}
          className="p-5 mb-5"
          extra={
            <div className="flex gap-3 text-[10px]">
              <span className="flex items-center gap-1"><span className="w-2 h-2 rounded bg-amber-400"></span>系统</span>
              <span className="flex items-center gap-1"><span className="w-2 h-2 rounded bg-indigo-400"></span>服务组</span>
              <span className="flex items-center gap-1"><span className="w-2 h-2 rounded bg-emerald-400"></span>服务</span>
              <span className="flex items-center gap-1"><span className="w-2 h-2 rounded bg-blue-400"></span>端</span>
            </div>
          }>
          <div className="grid gap-0" style={{ gridTemplateColumns: `repeat(${COLS[level]}, minmax(0, 1fr))` }}>
            <div className="px-3 py-2 bg-amber-50 border border-amber-200 rounded-lg text-center">
              <div className="text-[10px] text-amber-600 font-medium">系统</div>
              <div className="text-xs text-slate-700 mt-0.5">{chain.system}</div>
            </div>
            {level !== 'system' && (
              <div className="px-3 py-2 bg-indigo-50 border border-indigo-200 rounded-lg text-center">
                <div className="text-[10px] text-indigo-600 font-medium">服务组</div>
                <div className="text-xs text-slate-700 mt-0.5">{chain.group}</div>
              </div>
            )}
            {(level === 'service' || level === 'module') && (
              <div className="px-3 py-2 bg-emerald-50 border border-emerald-200 rounded-lg text-center">
                <div className="text-[10px] text-emerald-600 font-medium">服务</div>
                <div className="text-xs text-slate-700 mt-0.5">{chain.service}</div>
                <div className="text-[10px] text-slate-500 mt-0.5">{chain.serviceTp}</div>
              </div>
            )}
            {level === 'module' && (
              <div className="px-3 py-2 bg-white border border-slate-200 rounded-lg text-center">
                <div className="text-[10px] text-slate-500">测试点</div>
                <div className="text-[10px] text-slate-700 mt-0.5">{chain.testPoint}</div>
              </div>
            )}
            <div className="flex flex-col gap-1.5">
              {chain.tcs.length === 0 ? (
                <div className="px-2 py-2 bg-red-50 border border-red-200 rounded text-[10px] text-red-600 text-center">覆盖缺口 · 无用例</div>
              ) : chain.tcs.map((tc) => (
                <button key={tc.id} type="button" onClick={() => setActiveTcId(tc.id)}
                  className={'px-2 py-1 rounded text-[10px] text-left transition ' + (activeTcId === tc.id ? 'bg-indigo-600 text-white' : 'bg-white border border-slate-200 text-slate-600 hover:border-indigo-300')}>
                  <span className="font-mono">{tc.id}</span> {tc.name}
                </button>
              ))}
            </div>
          </div>

          {/* 选中用例详情 */}
          {activeTc && (
            <div className="mt-3 p-3 rounded-lg border border-indigo-200 bg-indigo-50/50 grid grid-cols-4 gap-3">
              <div className="flex items-center gap-2">
                <ResultIcon kind={RESULT_BADGE[activeTc.result]?.icon ?? 'pass'} />
                <div>
                  <div className="text-[10px] text-slate-500">执行结果</div>
                  <div className={'text-xs font-semibold ' + (activeTc.result === '失败' ? 'text-red-600' : activeTc.result === '阻塞' ? 'text-amber-600' : 'text-emerald-600')}>{activeTc.result}</div>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Camera className="w-3.5 h-3.5 text-indigo-400" />
                <div>
                  <div className="text-[10px] text-slate-500">证据数</div>
                  <div className="text-xs font-semibold text-slate-700">{activeTc.evidence} 份</div>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Wallet className="w-3.5 h-3.5 text-indigo-400" />
                <div>
                  <div className="text-[10px] text-slate-500">执行成本</div>
                  <div className="text-xs font-semibold text-slate-700">¥{activeTc.cost}</div>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Gauge className="w-3.5 h-3.5 text-indigo-400" />
                <div>
                  <div className="text-[10px] text-slate-500">断言强度 / 变异</div>
                  <div className="text-xs font-semibold text-slate-700">{activeTc.assertion} · {activeTc.mutation}%</div>
                </div>
              </div>
            </div>
          )}

          <div className="mt-4 pt-4 border-t border-slate-100 space-y-2">
            {chain.links.length === 0 ? (
              <div className="text-[11px] text-red-500">该需求无跨服务用例绑定，追溯链缺失——建议按测试点补建用例并绑定上游源。</div>
            ) : chain.links.map((r, i) => (
              <div key={i} className="flex items-center gap-3 text-[11px]">
                <span className={r.dot + ' w-2 h-2 rounded'} />
                <span className="font-mono text-slate-500 w-32">{r.asset}</span>
                <span className="text-slate-300">→</span>
                <span className="font-mono text-slate-500 w-32">{r.tp}</span>
                <span className="text-slate-300">→</span>
                <span className="font-mono text-indigo-500">{r.tc}</span>
                <span className={'ml-auto ' + r.color}>{r.status}</span>
              </div>
            ))}
          </div>
        </Card>
      )}

      <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
        <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
          <h2 className="font-semibold text-slate-700 text-sm">需求追溯矩阵</h2>
          <div className="flex gap-2 text-[11px]">
            <span className="flex items-center gap-1"><span className="w-3 h-3 rounded bg-emerald-100 border border-emerald-300"></span>已覆盖</span>
            <span className="flex items-center gap-1"><span className="w-3 h-3 rounded bg-amber-100 border border-amber-300"></span>部分</span>
            <span className="flex items-center gap-1"><span className="w-3 h-3 rounded bg-red-100 border border-red-300"></span>缺口</span>
          </div>
        </div>
        <div className="flex items-center justify-between px-5 pt-3">
          <FilterBar search={q} onSearch={setQ} />
          <span className="text-[11px] text-slate-400">共 {rows.length} / {REQUIREMENTS.length} 条需求</span>
        </div>
        <table className="w-full text-sm">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-xs text-slate-500">
              <th className="px-4 py-3 font-medium">需求 ID</th>
              <th className="px-4 py-3 font-medium">需求标题</th>
              <th className="px-4 py-3 font-medium">生成批次</th>
              <th className="px-4 py-3 font-medium">涉及被测对象</th>
              <th className="px-4 py-3 font-medium">测试点</th>
              <th className="px-4 py-3 font-medium">用例数</th>
              <th className="px-4 py-3 font-medium">执行状态</th>
              <th className="px-4 py-3 font-medium">追溯完整性</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {rows.map((r) => (
              <tr key={r.id}
                onClick={() => { setActiveReq(r.id); setActiveTcId(TRACE_CHAINS[r.id]?.tcs[0]?.id ?? null); }}
                className={(activeReq === r.id ? 'bg-indigo-50/60 ' : '') + (r.traceability === '缺口' ? 'bg-red-50/40 ' : 'hover:bg-slate-50 ') + 'cursor-pointer'}>
                <td className={'px-4 py-3 font-mono text-xs font-medium ' + (r.traceability === '缺口' ? 'text-red-600' : 'text-indigo-600')}>{r.id}</td>
                <td className="px-4 py-3 text-slate-700">{r.title}</td>
                <td className="px-4 py-3">
                  <span className="font-mono text-[10px] bg-slate-50 border border-slate-200 text-slate-500 px-1.5 py-0.5 rounded">{GEN_BATCH[r.id]}</span>
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-1">
                    {r.assets.map((a) => (
                      <span key={a} className="text-[10px] bg-slate-50 text-slate-600 px-1.5 py-0.5 rounded">{a}</span>
                    ))}
                  </div>
                </td>
                <td className={'px-4 py-3 ' + (r.traceability === '缺口' ? 'text-red-500 font-medium' : 'text-slate-600')}>{r.testPoints}</td>
                <td className={'px-4 py-3 ' + (r.traceability === '缺口' ? 'text-red-500 font-medium' : 'text-slate-600')}>{r.cases}</td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <div className="w-16 h-1.5 bg-slate-100 rounded-full">
                      <div className={r.executed === r.cases && r.cases > 0 ? 'bg-emerald-500 h-full rounded-full' : 'bg-amber-500 h-full rounded-full'}
                        style={{ width: r.cases > 0 ? `${(r.executed / r.cases) * 100}%` : '0%' }} />
                    </div>
                    <span className="text-xs text-slate-500">{r.executed}/{r.cases}</span>
                  </div>
                </td>
                <td className="px-4 py-3">
                  <span className={'text-[11px] px-2 py-0.5 rounded-full ' + TRACEABILITY_BADGE[r.traceability]}>{r.traceability}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {gapOnly && (
          <div className="px-5 py-3 text-[11px] text-amber-600 bg-amber-50/40 border-t border-amber-200">已筛选为「检测覆盖缺口」视图：仅显示未完全覆盖的需求（缺口 + 部分）。</div>
        )}
      </div>
    </div>
  );
}

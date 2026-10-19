import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { AUDIT_LOGS, COST_TOTAL, COST_MOM_CHANGE } from '@/data/mock';
import { PageHeader, GhostButton, Card, ListFilter } from '@/components/shared';
import { toast } from 'sonner';
import { ShieldCheck, Wallet, Play, UserCog, ScrollText, ChevronRight, Layers, Circle, Pause, Play as PlayIcon, RefreshCw } from 'lucide-react';
import { configuredGreenPassClient } from '@/api/runtime';
import type { AuditEvent } from '@/api/client';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';

const TYPE_BADGE: Record<string, string> = {
  '门禁阻断': 'text-red-600 bg-red-50',
  '契约告警': 'text-red-600 bg-red-50',
  '篡改检测': 'text-amber-600 bg-amber-50',
  '种子生成': 'text-emerald-600 bg-emerald-50',
  '冲突检测': 'text-red-600 bg-red-50',
  '执行通过': 'text-emerald-600 bg-emerald-50',
};

const DOMAINS = [
  { to: '/audit-cost', icon: Wallet, color: 'text-emerald-600 bg-emerald-50 border-emerald-200', title: '成本审计', desc: '金额 · 维度 · 降本', kpi: `¥${COST_TOTAL.toLocaleString()}`, sub: `环比 ${Math.abs(COST_MOM_CHANGE)}% ↓` },
  { to: '/audit-exec', icon: Play, color: 'text-emerald-700 bg-emerald-50 border-emerald-200', title: '执行审计', desc: '用例 · 结果 · 耗时', kpi: '1,248 / 1,300', sub: '失败 34 · 阻塞 10' },
  { to: '/audit-op', icon: UserCog, color: 'text-amber-600 bg-amber-50 border-amber-200', title: '操作审计', desc: '谁 · 何时 · 做了什么', kpi: '128 条', sub: '近 24h · 高风险 3' },
  { to: '/audit', icon: ScrollText, color: 'text-slate-600 bg-slate-50 border-slate-200', title: '原始日志', desc: '哈希链 · 仅追加', kpi: '12,847 条', sub: '外部锚定 WORM ✓' },
];

function Detail({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return <div className="min-w-0"><p className="mb-1 text-[11px] font-medium text-slate-500">{label}</p><p className={mono ? 'break-all font-mono text-[11px] leading-5 text-slate-700' : 'break-words text-xs leading-5 text-slate-700'}>{value}</p></div>;
}

export default function AuditPage() {
  const client = useMemo(() => configuredGreenPassClient(), []);
  const [realEvents, setRealEvents] = useState<AuditEvent[]>([]);
  const [loadError, setLoadError] = useState('');
  const [live, setLive] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [selectedEvent, setSelectedEvent] = useState<AuditEvent | null>(null);
  useEffect(() => {
    if (!client) return;
    let cancelled = false;
    const load = async (manual = false) => {
      if (manual) setRefreshing(true);
      try {
        const events = await client.auditEvents();
        if (!cancelled) { setRealEvents(events); setLoadError(''); }
      } catch (error) {
        if (!cancelled) setLoadError(error instanceof Error ? error.message : '读取审计链失败');
      } finally {
        if (!cancelled) setRefreshing(false);
      }
    };
    void load();
    const timer = live ? window.setInterval(() => void load(), 12_000) : undefined;
    return () => { cancelled = true; if (timer) window.clearInterval(timer); };
  }, [client, live]);
  const [q, setQ] = useState('');
  const [type, setType] = useState('');
  const [assetFilter, setAssetFilter] = useState('');
  const kw = q.trim().toLowerCase();
  const filtered = AUDIT_LOGS.filter((log) => {
    if (type && log.type !== type) return false;
    if (assetFilter && !log.asset.includes(assetFilter)) return false;
    if (kw && !(log.seq + log.message + log.actor + log.asset).toLowerCase().includes(kw)) return false;
    return true;
  });
  return (
    <div>
      <PageHeader title="审计总览" desc="哈希链 · 仅追加 · 可独立验证 · 一份链四视图">
        <GhostButton onClick={() => toast.success('链完整性验证通过', { description: '12,847 条日志哈希链逐块校验一致，WORM 锚定正常（原型示意）' })}><span className="flex items-center gap-1"><ShieldCheck className="w-3.5 h-3.5" />验证链完整性</span></GhostButton>
        <GhostButton onClick={() => toast('审计报告导出', { description: '已生成哈希链审计快照（HTML / PDF / Word，原型模拟下载）' })}><span className="flex items-center gap-1"><ScrollText className="w-3.5 h-3.5" />导出审计报告</span></GhostButton>
      </PageHeader>

      {client && <section className="mb-5 overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
        <div className="flex flex-col gap-3 border-b border-slate-100 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-3">
            <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600"><ShieldCheck className="h-4 w-4" /></span>
            <div><h2 className="text-sm font-semibold text-slate-800">可信审计链</h2><p className="mt-0.5 text-[11px] text-slate-400">AI 与人工操作统一入链 · 点击事件查看完整证据</p></div>
          </div>
          <div className="flex items-center gap-2 text-[11px]">
            <span className={live ? 'flex items-center gap-1.5 text-emerald-700' : 'flex items-center gap-1.5 text-slate-400'}><Circle className={live ? 'h-2 w-2 fill-emerald-500 text-emerald-500' : 'h-2 w-2 fill-slate-300 text-slate-300'} />{live ? '实时更新' : '已暂停'}</span>
            <button type="button" onClick={() => setLive((value) => !value)} className="rounded-md border border-slate-200 px-2.5 py-1.5 text-slate-600 hover:bg-slate-50" title={live ? '暂停实时更新' : '恢复实时更新'}>{live ? <Pause className="h-3.5 w-3.5" /> : <PlayIcon className="h-3.5 w-3.5" />}</button>
            <button type="button" onClick={() => { setRefreshing(true); void client.auditEvents().then((events) => { setRealEvents(events); setLoadError(''); }).catch((error) => setLoadError(error instanceof Error ? error.message : '读取审计链失败')).finally(() => setRefreshing(false)); }} className="rounded-md border border-slate-200 p-1.5 text-slate-600 hover:bg-slate-50" title="立即刷新"><RefreshCw className={refreshing ? 'h-3.5 w-3.5 animate-spin' : 'h-3.5 w-3.5'} /></button>
          </div>
        </div>
        {loadError && <div role="alert" className="m-5 rounded-lg bg-red-50 p-3 text-xs text-red-700">审计数据读取失败：{loadError}</div>}
        {!loadError && (realEvents.length ? <div className="max-h-[348px] overflow-y-auto overscroll-contain">
          <div className="sticky top-0 z-10 grid grid-cols-[116px_minmax(160px,1fr)_minmax(120px,0.8fr)_70px] gap-3 border-b border-slate-100 bg-slate-50 px-5 py-2 text-[10px] font-medium uppercase tracking-wide text-slate-400"><span>发生时间</span><span>操作</span><span>对象</span><span className="text-right">详情</span></div>
          <div className="divide-y divide-slate-100">{realEvents.map((event) => <button type="button" key={event.id} onClick={() => setSelectedEvent(event)} className="grid w-full grid-cols-[116px_minmax(160px,1fr)_minmax(120px,0.8fr)_70px] items-center gap-3 px-5 py-3 text-left transition hover:bg-emerald-50/50 focus:bg-emerald-50/50 focus:outline-none">
            <time className="text-[11px] tabular-nums text-slate-400">{new Date(event.ts).toLocaleString('zh-CN', { hour12: false, month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })}</time>
            <span className="min-w-0"><span className="inline-flex max-w-full truncate rounded-md bg-emerald-50 px-2 py-1 font-mono text-[11px] font-medium text-emerald-700">{event.op}</span></span>
            <span className="truncate text-xs text-slate-600">{event.asset} <span className="font-mono text-slate-400">#{event.asset_id}</span></span>
            <span className="text-right text-[11px] font-medium text-emerald-700">查看 <ChevronRight className="inline h-3.5 w-3.5" /></span>
          </button>)}</div>
        </div> : <p className="px-5 py-10 text-center text-xs text-slate-400">尚无可信审计事件；AI 对话、生成批次和人工审核发生后将显示在这里。</p>)}
        {!loadError && realEvents.length > 0 && <div className="flex items-center justify-between border-t border-slate-100 bg-slate-50/60 px-5 py-2.5 text-[11px] text-slate-400"><span>当前窗口显示最近 {realEvents.length} 条事件</span><span>每 12 秒更新一次</span></div>}
      </section>}

      <Dialog open={Boolean(selectedEvent)} onOpenChange={(open) => { if (!open) setSelectedEvent(null); }}>
        <DialogContent className="max-w-2xl border-slate-200 bg-white p-0">
          {selectedEvent && <><DialogHeader className="border-b border-slate-100 px-6 py-5 pr-12"><DialogTitle className="text-base text-slate-800">审计事件详情</DialogTitle><DialogDescription className="mt-1 font-mono text-xs text-emerald-700">{selectedEvent.op}</DialogDescription></DialogHeader>
            <div className="max-h-[65vh] space-y-5 overflow-y-auto px-6 py-5 text-xs">
              <div className="grid grid-cols-2 gap-x-6 gap-y-4"><Detail label="事件 ID" value={String(selectedEvent.id)} mono /><Detail label="发生时间" value={new Date(selectedEvent.ts).toLocaleString('zh-CN', { hour12: false })} /><Detail label="操作者" value={selectedEvent.actor ? `用户 #${selectedEvent.actor}` : '系统'} /><Detail label="审计对象" value={`${selectedEvent.asset} #${selectedEvent.asset_id}`} mono /></div>
              <Detail label="事件哈希" value={selectedEvent.hash} mono /><Detail label="前序哈希" value={selectedEvent.prev_hash || '链起始事件'} mono />
              <div><p className="mb-2 text-[11px] font-medium text-slate-500">事件载荷</p><pre className="overflow-x-auto rounded-lg bg-slate-950 p-4 font-mono text-[11px] leading-5 text-slate-100">{JSON.stringify(selectedEvent.payload, null, 2)}</pre></div>
            </div></>}
        </DialogContent>
      </Dialog>

      <div className="card bg-white rounded-xl border border-slate-200 p-5 mb-5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className="w-10 h-10 rounded-full bg-emerald-100 flex items-center justify-center text-emerald-600 text-lg">✓</div>
            <div>
              <div className="text-sm font-semibold text-slate-700">哈希链完整性验证通过</div>
              <div className="text-xs text-slate-500 mt-0.5">共 12,847 条日志 · 最后锚定 2026-09-24 10:30:00</div>
            </div>
          </div>
          <div className="text-right">
            <div className="font-mono text-slate-500 text-[11px]">链头哈希</div>
            <div className="font-mono text-slate-700 font-medium">sha256:0a1f9c3e…d84b</div>
            <div className="font-mono text-emerald-600 text-[10px] mt-0.5">外部锚定: WORM bucket ✓</div>
          </div>
        </div>
      </div>

      {/* 一份链四视图：设计原则 */}
      <div className="mb-5 px-4 py-3 rounded-xl bg-emerald-50 border border-emerald-200 flex items-start gap-3">
        <Layers className="w-4 h-4 text-emerald-600 flex-shrink-0 mt-0.5" />
        <div className="text-[11px] text-emerald-800 leading-relaxed">
          <span className="font-semibold">审计设计原则 · 一份链四视图：</span>
          底层为<span className="font-semibold">单一哈希链日志</span>（唯一不可变数据源，跨被测对象 / 跨域追溯靠它），上层按业务域拆成独立视图
          —— <span className="font-semibold">成本审计 / 执行审计 / 操作审计 / 审计总览</span>。既满足各自管理，又不破坏"仅追加、可独立验证"的审计根基。
        </div>
      </div>

      {/* 四域概览 */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        {DOMAINS.map((d) => {
          const Icon = d.icon;
          return (
            <Link key={d.title} to={d.to} className="card bg-white rounded-xl border border-slate-200 p-4 hover:shadow-md transition group">
              <div className="flex items-center justify-between mb-3">
                <span className={`w-9 h-9 rounded-lg ${d.color} flex items-center justify-center`}><Icon className="w-4 h-4" /></span>
                <ChevronRight className="w-4 h-4 text-slate-300 group-hover:text-emerald-500 transition" />
              </div>
              <div className="text-sm font-semibold text-slate-700">{d.title}</div>
              <div className="text-[11px] text-slate-400 mt-0.5">{d.desc}</div>
              <div className="text-xl font-bold text-slate-800 mt-2">{d.kpi}</div>
              <div className="text-[11px] text-slate-400 mt-0.5">{d.sub}</div>
            </Link>
          );
        })}
      </div>

      <Card title="原始日志流"
        extra={
          <select value={assetFilter} onChange={(e) => setAssetFilter(e.target.value)}
            className="border border-slate-300 rounded-lg px-2 py-1 outline-none text-xs">
            <option value="">全部被测对象</option>
            <option value="svc-auth">svc-auth</option>
            <option value="svc-payment">svc-payment</option>
          </select>
        }>
        <div className="flex items-center justify-between px-5 pt-3">
          <ListFilter search={q} onSearch={setQ}
            selects={[{ key: 'type', label: '类型', options: ['门禁阻断', '契约告警', '篡改检测', '种子生成', '冲突检测', '执行通过'], value: type, onChange: setType }]} />
          <span className="text-[11px] text-slate-400">共 {filtered.length} / {AUDIT_LOGS.length} 条</span>
        </div>
        <div className="divide-y divide-slate-100">
          {filtered.map((log) => (
            <div key={log.seq} className="px-5 py-3.5 hover:bg-slate-50">
              <div className="flex items-center gap-4">
                <span className="font-mono text-slate-400 w-12 text-xs">{log.seq}</span>
                <span className="text-[11px] text-slate-400 w-36">{log.time}</span>
                <span className={'text-xs font-medium px-2 py-0.5 rounded-full ' + TYPE_BADGE[log.type]}>{log.type}</span>
                <span className="text-xs text-slate-700 flex-1">{log.message}</span>
                <span className="font-mono text-slate-400 text-xs">{log.hash}</span>
              </div>
              <div className="ml-16 mt-2 flex gap-4 text-[11px] text-slate-500">
                <span>actor: <span className="text-slate-600">{log.actor}</span></span>
                <span>asset: <span className="text-slate-600 font-mono">{log.asset}</span></span>
                <span>prev: <span className="font-mono text-slate-400">sha256:9c3e…</span></span>
              </div>
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}

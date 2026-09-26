import { FLOW_ITEMS, EXEC_RUN } from '@/data/mock';
import { PageHeader, Card } from '@/components/shared';

const STATUS_STYLE: Record<string, { dot: string; badge: string; text: string }> = {
  '通过': { dot: 'bg-emerald-500', badge: 'text-emerald-600 bg-emerald-50', text: '通过' },
  '失败': { dot: 'bg-red-500', badge: 'text-red-600 bg-red-50', text: '失败' },
  '执行中': { dot: 'bg-indigo-500', badge: 'text-indigo-500 bg-indigo-100', text: '执行中' },
  '阻塞': { dot: 'bg-amber-500', badge: 'text-amber-600 bg-amber-50', text: '阻塞' },
};

export default function ExecPage() {
  const stats = {
    pass: EXEC_RUN.passed,
    fail: EXEC_RUN.failed,
    queued: EXEC_RUN.queued,
    running: EXEC_RUN.running,
    blocked: EXEC_RUN.blocked,
  };

  return (
    <div>
      <PageHeader title="测试执行" desc="跨资产执行 · 实时进度 · 证据捕获 · 不放水不跳过">
        <div className="flex items-center gap-3">
          <span className="flex items-center gap-2 text-xs text-slate-500">
            <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span> 12 个执行器在线
          </span>
          <button className="px-3 py-1.5 text-sm border border-slate-300 rounded-lg hover:bg-slate-50 bg-white">暂停全部</button>
        </div>
      </PageHeader>

      <div className="card bg-white rounded-xl border border-slate-200 p-5 mb-5">
        <div className="flex items-center justify-between mb-3">
          <div>
            <span className="text-sm font-semibold text-slate-700">{EXEC_RUN.label}</span>
            <span className="ml-3 text-xs text-slate-500">{EXEC_RUN.branch}</span>
          </div>
          <span className="text-sm font-bold text-emerald-600">{EXEC_RUN.passed + EXEC_RUN.failed + EXEC_RUN.running + EXEC_RUN.blocked} / {EXEC_RUN.total}</span>
        </div>
        <div className="h-3 bg-slate-100 rounded-full overflow-hidden">
          <div className="h-full bg-gradient-to-r from-emerald-500 to-green-500 rounded-full" style={{ width: `${EXEC_RUN.progress}%` }} />
        </div>
        <div className="flex justify-between mt-3 text-xs">
          <div className="flex gap-5">
            <span className="text-emerald-600">✓ 通过 {stats.pass}</span>
            <span className="text-red-500">✕ 失败 {stats.fail}</span>
            <span className="text-slate-400">○ 排队 {stats.queued}</span>
            <span className="text-amber-500">◐ 执行中 {stats.running}</span>
            <span className="text-slate-400">⊘ 阻塞 {stats.blocked}</span>
          </div>
          <span className="text-slate-400">{EXEC_RUN.eta}</span>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-5">
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
            <h2 className="font-semibold text-slate-700 text-sm">实时执行流</h2>
            <div className="flex gap-2 text-xs">
              <button className="px-2 py-1 bg-slate-100 rounded text-slate-600">全部</button>
              <button className="px-2 py-1 rounded text-slate-400">失败</button>
            </div>
          </div>
          <div className="divide-y divide-slate-100 max-h-[480px] overflow-y-auto">
            {FLOW_ITEMS.map((item) => {
              const s = STATUS_STYLE[item.status];
              return (
                <div key={item.id} className={'px-5 py-3 flex items-center gap-3 ' + (item.status === '执行中' ? 'bg-indigo-50/40' : item.status === '失败' ? 'bg-red-50/40' : item.status === '阻塞' ? 'bg-amber-50/40' : '')}>
                  <span className={s.dot + ' w-2 h-2 rounded-full flex-shrink-0' + (item.status === '执行中' ? ' animate-pulse' : '')} />
                  <span className="font-mono text-xs text-indigo-600 w-28 flex-shrink-0">{item.id}</span>
                  <span className="text-xs text-slate-600 flex-1 truncate">{item.title}</span>
                  <span className={'text-[11px] px-2 py-0.5 rounded-full ' + s.badge}>{s.text}</span>
                  <span className="text-[11px] text-slate-400 w-12 text-right">{item.duration}</span>
                </div>
              );
            })}
          </div>
        </div>

        <div className="space-y-5">
          <Card title="证据捕获" className="p-5">
            <div className="grid grid-cols-2 gap-3 mb-4">
              {['截图 01', '截图 02', '视频', '日志'].map((e) => (
                <div key={e} className="aspect-video bg-slate-100 rounded-lg flex items-center justify-center text-slate-400 text-xs border border-slate-200">{e}</div>
              ))}
            </div>
            <div className="space-y-2 text-[11px]">
              <div className="flex justify-between"><span className="text-slate-500">证据哈希</span><span className="font-mono text-slate-600">sha256:a3f8…c21d</span></div>
              <div className="flex justify-between"><span className="text-slate-500">存储位置</span><span className="text-slate-600">minio://evidence/</span></div>
              <div className="flex justify-between"><span className="text-slate-500">写入状态</span><span className="text-emerald-600 font-medium">✓ 已锚定</span></div>
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}

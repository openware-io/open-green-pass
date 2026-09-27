import { useState } from 'react';
import { FLOW_ITEMS, EXEC_RUN, TEST_SCENARIOS } from '@/data/mock';
import { PageHeader, Card, ListFilter } from '@/components/shared';
import { Cpu, Globe, Smartphone, Sparkles, Play, Pause } from 'lucide-react';
import { toast } from 'sonner';

const STATUS_STYLE: Record<string, { dot: string; badge: string; text: string }> = {
  '通过': { dot: 'bg-emerald-500', badge: 'text-emerald-600 bg-emerald-50', text: '通过' },
  '失败': { dot: 'bg-red-500', badge: 'text-red-600 bg-red-50', text: '失败' },
  '执行中': { dot: 'bg-indigo-500', badge: 'text-indigo-500 bg-indigo-100', text: '执行中' },
  '阻塞': { dot: 'bg-amber-500', badge: 'text-amber-600 bg-amber-50', text: '阻塞' },
};
const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles };

/** 执行任务 → 所属测试场景（清单归属标注，按标题/被测对象关键词映射） */
function flowScenario(title: string, asset: string): string {
  if (title.includes('E2E') || asset.includes('web') || asset.includes('frontend')) return 'SCEN-08';
  if (title.includes('压力测试') || title.includes('并发')) return 'SCEN-05';
  if (title.includes('真机') || asset.includes('mobile')) return 'SCEN-09';
  return 'SCEN-02';
}

export default function ExecPage() {
  const [paused, setPaused] = useState(false);
  const [q, setQ] = useState('');
  const [status, setStatus] = useState('');
  const kw = q.trim().toLowerCase();
  const filtered = FLOW_ITEMS.filter((it) => {
    if (status && it.status !== status) return false;
    if (kw && !(it.id + it.title + it.asset).toLowerCase().includes(kw)) return false;
    return true;
  });
  const stats = {
    pass: EXEC_RUN.passed,
    fail: EXEC_RUN.failed,
    queued: EXEC_RUN.queued,
    running: EXEC_RUN.running,
    blocked: EXEC_RUN.blocked,
  };

  const togglePause = () => {
    setPaused(!paused);
    toast.success(paused ? '已恢复全部执行器（原型模拟）' : '已暂停全部执行器（原型模拟）');
  };

  return (
    <div>
      <PageHeader title="测试执行" desc="执行总览清单 · 跨被测对象执行 · 实时进度 · 证据捕获 · 不放水不跳过">
        <div className="flex items-center gap-3">
          <span className="flex items-center gap-2 text-xs text-slate-500">
            <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span> 12 个执行器在线
          </span>
          <button onClick={togglePause}
            className={'px-3 py-1.5 text-sm border rounded-lg flex items-center gap-1.5 ' + (paused ? 'bg-emerald-600 text-white border-emerald-600' : 'border-slate-300 hover:bg-slate-50 bg-white text-slate-600')}>
            {paused ? <Play className="w-3.5 h-3.5" /> : <Pause className="w-3.5 h-3.5" />}{paused ? '恢复全部' : '暂停全部'}
          </button>
        </div>
      </PageHeader>

      {/* 本次执行总览 */}
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
            <h2 className="font-semibold text-slate-700 text-sm">执行任务清单 <span className="text-[10px] text-slate-400 font-normal">· 跨场景总览 · 每任务标注所属测试场景</span></h2>
            <ListFilter search={q} onSearch={setQ}
              selects={[{ key: 'status', label: '状态', options: ['通过', '失败', '执行中', '阻塞'], value: status, onChange: setStatus }]} />
          </div>
          <table className="w-full text-xs">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-slate-500">
                <th className="px-5 py-2.5 font-medium">任务</th>
                <th className="px-5 py-2.5 font-medium">所属场景</th>
                <th className="px-5 py-2.5 font-medium">状态</th>
                <th className="px-5 py-2.5 font-medium text-right">耗时</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {filtered.map((item) => {
                const s = STATUS_STYLE[item.status];
                const scen = TEST_SCENARIOS.find((x) => x.id === flowScenario(item.title, item.asset));
                const Icon = scen ? SCEN_ICON[scen.icon] ?? Cpu : null;
                return (
                  <tr key={item.id} className={item.status === '执行中' ? 'bg-indigo-50/40' : item.status === '失败' ? 'bg-red-50/40' : item.status === '阻塞' ? 'bg-amber-50/40' : ''}>
                    <td className="px-5 py-2.5">
                      <div className="flex items-center gap-2">
                        <span className={s.dot + ' w-2 h-2 rounded-full' + (item.status === '执行中' ? ' animate-pulse' : '')} />
                        <span className="font-mono text-indigo-600 w-24 flex-shrink-0">{item.id}</span>
                        <span className="text-slate-600 truncate">{item.title}</span>
                      </div>
                    </td>
                    <td className="px-5 py-2.5">
                      {scen ? (
                        <span className="inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded-full bg-slate-100 text-slate-500">
                          {Icon && <Icon className="w-3 h-3" />}{scen.name}
                        </span>
                      ) : <span className="text-slate-300">—</span>}
                    </td>
                    <td className="px-5 py-2.5"><span className={'text-[11px] px-2 py-0.5 rounded-full ' + s.badge}>{s.text}</span></td>
                    <td className="px-5 py-2.5 text-slate-400 text-right">{item.duration}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
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
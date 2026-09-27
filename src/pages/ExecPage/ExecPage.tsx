import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { FLOW_ITEMS, EXEC_RUN, TEST_SCENARIOS } from '@/data/mock';
import { scenarioNav } from '@/context/scenarioNav';
import { PageHeader, Card } from '@/components/shared';
import { Cpu, Globe, Smartphone, Sparkles, Play, Pause, CornerDownRight } from 'lucide-react';
import { toast } from 'sonner';

const STATUS_STYLE: Record<string, { dot: string; badge: string; text: string }> = {
  '通过': { dot: 'bg-emerald-500', badge: 'text-emerald-600 bg-emerald-50', text: '通过' },
  '失败': { dot: 'bg-red-500', badge: 'text-red-600 bg-red-50', text: '失败' },
  '执行中': { dot: 'bg-indigo-500', badge: 'text-indigo-500 bg-indigo-100', text: '执行中' },
  '阻塞': { dot: 'bg-amber-500', badge: 'text-amber-600 bg-amber-50', text: '阻塞' },
};
const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles };

/** 执行流任务 → 测试场景（按标题/资产关键词映射） */
function flowScenario(title: string, asset: string): string {
  if (title.includes('E2E') || asset.includes('web') || asset.includes('frontend')) return 'SCEN-08';
  if (title.includes('压力测试') || title.includes('并发')) return 'SCEN-05';
  if (title.includes('真机') || asset.includes('mobile')) return 'SCEN-09';
  return 'SCEN-02';
}

export default function ExecPage() {
  const navigate = useNavigate();
  const [paused, setPaused] = useState(false);
  const [scenId, setScenId] = useState(TEST_SCENARIOS[0].id);
  const selScen = TEST_SCENARIOS.find((s) => s.id === scenId) as (typeof TEST_SCENARIOS)[number];

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
      <PageHeader title="测试执行" desc="跨资产执行 · 实时进度 · 证据捕获 · 不放水不跳过">
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

      {/* 跨场景发起执行：全局入口 → 下钻测试中心对应场景执行闭环 */}
      <Card title="跨场景发起执行" extra={<span className="text-[11px] text-slate-400">从全局进入测试中心 · 在场景执行闭环中配置范围并启动</span>} className="p-5 mb-5">
        <div className="flex items-center gap-3 flex-wrap">
          <select value={scenId} onChange={(e) => setScenId(e.target.value)}
            className="text-xs border border-slate-200 rounded-lg px-2 py-1.5 bg-white text-slate-600">
            {TEST_SCENARIOS.map((s) => <option key={s.id} value={s.id}>{s.name} · {s.id}</option>)}
          </select>
          <button type="button"
            onClick={() => { scenarioNav.go(scenId, 'exec'); navigate('/scenarios'); }}
            className="flex items-center gap-1.5 px-4 py-2 text-sm bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 shadow-sm">
            <CornerDownRight className="w-4 h-4" />进入「{selScen.name}」执行闭环
          </button>
          <span className="text-[11px] text-slate-400">默认全部执行 · 可在场景执行中按用例树范围人工选择当次用例</span>
        </div>
      </Card>

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
            <h2 className="font-semibold text-slate-700 text-sm">实时执行流 <span className="text-[10px] text-slate-400 font-normal">· 点击场景标签下钻测试中心</span></h2>
            <div className="flex gap-2 text-xs">
              <button className="px-2 py-1 bg-slate-100 rounded text-slate-600">全部</button>
              <button className="px-2 py-1 rounded text-slate-400">失败</button>
            </div>
          </div>
          <div className="divide-y divide-slate-100 max-h-[480px] overflow-y-auto">
            {FLOW_ITEMS.map((item) => {
              const s = STATUS_STYLE[item.status];
              const scen = TEST_SCENARIOS.find((x) => x.id === flowScenario(item.title, item.asset));
              const Icon = scen ? SCEN_ICON[scen.icon] ?? Cpu : null;
              return (
                <div key={item.id} className={'px-5 py-3 flex items-center gap-3 ' + (item.status === '执行中' ? 'bg-indigo-50/40' : item.status === '失败' ? 'bg-red-50/40' : item.status === '阻塞' ? 'bg-amber-50/40' : '')}>
                  <span className={s.dot + ' w-2 h-2 rounded-full flex-shrink-0' + (item.status === '执行中' ? ' animate-pulse' : '')} />
                  <span className="font-mono text-xs text-indigo-600 w-28 flex-shrink-0">{item.id}</span>
                  <span className="text-xs text-slate-600 flex-1 truncate">{item.title}</span>
                  {scen && (
                    <button type="button" onClick={() => { scenarioNav.go(scen.id, 'exec'); navigate('/scenarios'); }}
                      className="flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded-full bg-slate-100 text-slate-500 hover:bg-emerald-50 hover:text-emerald-600 flex-shrink-0">
                      {Icon && <Icon className="w-3 h-3" />}{scen.name}
                    </button>
                  )}
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

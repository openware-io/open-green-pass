import { useState } from 'react';
import { UPSTREAM_ADAPTERS, GENERATION_STAGES, ADAPTER_SEED_TOTAL } from '@/data/mock';
import { PageHeader, GhostButton, PrimaryButton, Card, ListFilter } from '@/components/shared';
import { toast } from 'sonner';
import { BrainCircuit, Sparkles, Inbox, Plug, ShieldCheck, UserCheck, Database, GitBranch, BookOpen, Wand2, PlusCircle, CloudDownload, ArrowLeft } from 'lucide-react';

const STAGE_ICON: Record<string, typeof BrainCircuit> = {
  source: Inbox,
  adapter: Plug,
  seed: Sparkles,
  quality: ShieldCheck,
  review: UserCheck,
  storage: Database,
};
const STAGE_COLOR: Record<string, string> = {
  source: 'bg-indigo-50 border-indigo-200 text-indigo-600',
  adapter: 'bg-indigo-50 border-indigo-200 text-indigo-600',
  seed: 'bg-amber-50 border-amber-200 text-amber-600',
  quality: 'bg-emerald-50 border-emerald-200 text-emerald-600',
  review: 'bg-purple-50 border-purple-200 text-purple-600',
  storage: 'bg-emerald-50 border-emerald-200 text-emerald-600',
};

const QUALITY = [
  { label: '变异测试验证', val: '38 / 48', width: 79, color: 'bg-emerald-500', note: '10 条未通过，无法杀死代码变体' },
  { label: '断言强度检查', val: '42 / 48', width: 87, color: 'bg-emerald-500', note: '6 条存在弱断言嫌疑' },
  { label: '重复检测', val: '46 / 48', width: 96, color: 'bg-emerald-500', note: '2 条与已有用例语义重复' },
  { label: '追溯绑定', val: '48 / 48', width: 100, color: 'bg-emerald-500', note: '全部绑定上游源' },
];

// AI 决策痕迹：展示 AI 如何理解上游源并产出用例种子
const AI_TRACES = [
  { step: '解析', desc: 'AI 读取 OpenAPI /v2/refund 变更 diff，提取新增字段 refund_id、移除 status', out: '识别 1 处破坏性变更' },
  { step: '生成', desc: 'AI 依据契约 + 历史缺陷模式，生成 12 条用例种子覆盖正/反/边界路径', out: '12 条种子' },
  { step: '筛选', desc: 'AI 计算种子断言强度，剔除 6 条弱断言嫌疑，保留强断言', out: '保留 12 条' },
  { step: '绑定', desc: 'AI 为每条种子建立 REQ→测试点→用例 追溯链', out: '追溯完整' },
];

// ============ 被测仓库：AI 生成用例的输入（仓库 + 版本） ============
// 研发团队管理者在此添加代码仓库并获取仓库代码与版本；
// 每次更新测试在此选择目标版本生成用例；未来 CI/CD 联动后版本可由上游流水线推送并触发联动流程（后续迭代实现）。
interface IRepo { name: string; source: string; branch: string; branches: string[]; versions: string[]; synced: string; owner: string }
const REPOS: IRepo[] = [
  { name: 'svc-payment', source: 'GitLab', branch: 'main', branches: ['main', 'release', 'feature-refund-v2'], versions: ['v2.4.1', 'v2.4.0'], synced: '2026-09-27', owner: '张立' },
  { name: 'svc-auth', source: 'GitLab', branch: 'main', branches: ['main', 'develop'], versions: ['v2.3.0'], synced: '2026-09-25', owner: '张立' },
  { name: 'svc-user', source: 'GitHub', branch: 'main', branches: ['main'], versions: ['v2.1.2'], synced: '2026-09-22', owner: '张立' },
  { name: 'web-frontend', source: 'GitLab', branch: 'release', branches: ['release', 'main'], versions: ['v2.4.0'], synced: '2026-09-26', owner: '李伟' },
  { name: 'mobile-ios', source: 'GitHub', branch: 'main', branches: ['main', 'release'], versions: ['v2.4.0'], synced: '2026-09-24', owner: '李伟' },
  { name: 'svc-order', source: 'GitLab', branch: 'main', branches: ['main', 'develop'], versions: ['v2.0.5'], synced: '2026-09-20', owner: '张立' },
];

export default function GenerationPage() {
  const [q, setQ] = useState('');
  const kw = q.trim().toLowerCase();
  const upFiltered = UPSTREAM_ADAPTERS.filter((a) => !kw || (a.source + a.status).toLowerCase().includes(kw));

  // AI 生成用例面板：输入 = 仓库 + 版本
  const [genPanel, setGenPanel] = useState(false);
  const [genRepo, setGenRepo] = useState('svc-payment');
  const [genBranch, setGenBranch] = useState('main');
  const [genVer, setGenVer] = useState('v2.4.1');

  return (
    <div>
      <PageHeader title="上游源与生成" desc="被测仓库 · AI 理解上游源 · 生成用例 · 质量验证 · 人工审核">
        <GhostButton onClick={() => toast('配置适配器', { description: '管理六类上游源的同步规则与解析策略（原型示意）' })}>配置适配器</GhostButton>
        <PrimaryButton onClick={() => toast.success('已发起手动同步（原型模拟）', { description: '将拉取各上游源最新变更并重跑用例生成管道' })}>手动触发同步</PrimaryButton>
      </PageHeader>

      {/* 被测仓库：生成用例输入（仓库 + 版本） */}
      <Card title="被测仓库 · 生成用例输入" className="p-5 mb-5">
        <div className="flex items-start justify-between mb-4 gap-3 flex-wrap">
          <p className="text-[11px] text-slate-500 leading-relaxed max-w-xl">
            研发团队管理者在此<b>添加代码仓库</b>并获取<b>仓库代码与版本</b>。每次需要更新测试时，在此选择<b>目标版本</b>生成用例；
            版本将用于执行前环境版本校验。<span className="text-slate-400">未来接入 CI/CD 后，版本可由上游流水线推送并自动触发联动流程（后续迭代实现）。</span>
          </p>
          <GhostButton onClick={() => toast('添加仓库', { description: '研发团队管理者添加被测代码仓库，获取代码与版本（原型示意）' })}>
            <PlusCircle className="w-3.5 h-3.5" />添加仓库
          </GhostButton>
        </div>
        <table className="w-full text-xs">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-slate-500">
              <th className="px-4 py-2.5 font-medium">仓库</th>
              <th className="px-4 py-2.5 font-medium">代码源</th>
              <th className="px-4 py-2.5 font-medium">分支</th>
              <th className="px-4 py-2.5 font-medium">可用版本</th>
              <th className="px-4 py-2.5 font-medium">最近同步</th>
              <th className="px-4 py-2.5 font-medium">负责人</th>
              <th className="px-4 py-2.5 font-medium text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {REPOS.map((r) => (
              <tr key={r.name} className="hover:bg-slate-50">
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <span className="flex items-center justify-center w-5 h-5 rounded bg-emerald-50 text-emerald-600"><GitBranch className="w-3.5 h-3.5" /></span>
                    <span className="font-medium text-slate-700">{r.name}</span>
                  </div>
                </td>
                <td className="px-4 py-3 text-slate-500">{r.source}</td>
                <td className="px-4 py-3 text-slate-500">{r.branch}</td>
                <td className="px-4 py-3"><span className="font-mono text-indigo-600">{r.versions[0]}</span><span className="text-slate-400 ml-1 text-[10px]">+{r.versions.length - 1}</span></td>
                <td className="px-4 py-3 text-slate-500">{r.synced}</td>
                <td className="px-4 py-3 text-slate-500">{r.owner}</td>
                <td className="px-4 py-3 text-right">
                  <button type="button" onClick={() => { setGenRepo(r.name); setGenVer(r.versions[0]); setGenPanel(true); }}
                    className="px-2.5 py-1 rounded-md text-[11px] bg-emerald-600 text-white hover:bg-emerald-700">生成用例</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>

      <Card title="生成管道 · 当前批次 #GEN-2041" className="p-5 mb-5">
        <div className="flex items-center justify-between">
          {GENERATION_STAGES.map((s, i) => (
            <div key={s.label} className="flex flex-1">
              <div className="flex-1 text-center">
                <div className={'w-12 h-12 rounded-full border flex items-center justify-center mx-auto mb-2 ' + STAGE_COLOR[s.icon]}>
                  {(() => { const I = STAGE_ICON[s.icon]; return I ? <I className="w-5 h-5" /> : null; })()}
                </div>
                <div className="text-xs font-medium text-slate-700">{s.label}</div>
                <div className="text-[10px] text-slate-400 mt-0.5">{s.meta}</div>
              </div>
              {i < GENERATION_STAGES.length - 1 && (
                <div className="flex justify-center items-center text-slate-300 px-2">→</div>
              )}
            </div>
          ))}
        </div>
      </Card>

      {/* AI 决策痕迹：把 AI 驱动从角落拉到主线 */}
      <div className="card bg-gradient-to-r from-emerald-600 to-teal-700 rounded-xl border border-transparent p-5 mb-5 text-white">
        <div className="flex items-center gap-2 mb-4">
          <BrainCircuit className="w-5 h-5" />
          <h2 className="font-semibold text-sm">AI 决策痕迹 · 本批 {ADAPTER_SEED_TOTAL} 条种子如何产生</h2>
          <span className="ml-auto text-[10px] bg-white/10 px-2 py-1 rounded flex items-center gap-1"><Sparkles className="w-3 h-3" />AI 驱动 · 自动</span>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {AI_TRACES.map((t, i) => (
            <div key={t.step} className="bg-white/10 rounded-lg p-3 relative">
              <div className="flex items-center gap-2 mb-1.5">
                <span className="w-5 h-5 rounded-full bg-white/20 flex items-center justify-center text-[10px] font-bold">{i + 1}</span>
                <span className="text-sm font-medium">{t.step}</span>
              </div>
              <div className="text-[10px] text-emerald-100 leading-relaxed">{t.desc}</div>
              <div className="mt-2 text-[10px] text-amber-200 font-medium flex items-center gap-1">
                <Sparkles className="w-3 h-3" />{t.out}
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-3 gap-5">
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-slate-200">
            <h2 className="font-semibold text-slate-700 text-sm">上游源适配器</h2>
          </div>
          <div className="flex items-center justify-between px-5 pt-3">
            <ListFilter search={q} onSearch={setQ} />
            <span className="text-[11px] text-slate-400">共 {upFiltered.length} / {UPSTREAM_ADAPTERS.length} 个上游源</span>
          </div>
          <table className="w-full text-xs">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-slate-500">
                <th className="px-5 py-2.5 font-medium">上游源</th>
                <th className="px-5 py-2.5 font-medium">系统</th>
                <th className="px-5 py-2.5 font-medium">提取信息</th>
                <th className="px-5 py-2.5 font-medium">本次种子</th>
                <th className="px-5 py-2.5 font-medium">状态</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {upFiltered.map((a) => (
                <tr key={a.source} className={a.status === '验证中' ? 'bg-indigo-50/30 hover:bg-indigo-50' : 'hover:bg-slate-50'}>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2">
                      <span className="w-5 h-5 rounded bg-slate-100 text-slate-600 flex items-center justify-center text-[10px]">源</span>
                      <span className="text-slate-700 font-medium">{a.source}</span>
                    </div>
                  </td>
                  <td className="px-5 py-3 text-slate-500">{a.system}</td>
                  <td className="px-5 py-3 text-slate-500">{a.extract}</td>
                  <td className="px-5 py-3 text-slate-600">{a.seeds}</td>
                  <td className="px-5 py-3">
                    <span className={a.status === '验证中' ? 'bg-amber-50 text-amber-600 px-2 py-0.5 rounded-full text-[10px]' : 'bg-emerald-50 text-emerald-600 px-2 py-0.5 rounded-full text-[10px]'}>
                      {a.status}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <Card title="质量验证结果" className="p-5">
          <div className="space-y-3.5">
            {QUALITY.map((q) => (
              <div key={q.label}>
                <div className="flex justify-between text-xs mb-1.5">
                  <span className="text-slate-600">{q.label}</span>
                  <span className="text-slate-500">{q.val}</span>
                </div>
                <div className="h-2 bg-slate-100 rounded-full">
                  <div className={q.color + ' h-full rounded-full'} style={{ width: q.width + '%' }} />
                </div>
                <div className="text-[10px] text-slate-400 mt-1">{q.note}</div>
              </div>
            ))}
          </div>
        </Card>
      </div>

      {/* AI 生成用例面板：输入 = 仓库 + 版本 */}
      {genPanel && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-6" onClick={() => setGenPanel(false)}>
          <div className="bg-white rounded-xl shadow-xl w-full max-w-lg" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between px-5 py-4 border-b border-slate-100">
              <div className="flex items-center gap-2">
                <span className="flex items-center justify-center w-8 h-8 rounded-lg bg-emerald-50 text-emerald-600"><Wand2 className="w-4 h-4" /></span>
                <div>
                  <div className="text-sm font-semibold text-slate-800">AI 生成用例</div>
                  <div className="text-[10px] text-slate-400">输入 = 代码仓库 + 版本 · AI 读取仓库代码与文档生成用例规格</div>
                </div>
              </div>
              <button type="button" onClick={() => setGenPanel(false)} className="flex items-center gap-1 text-xs text-slate-400 hover:text-slate-600"><ArrowLeft className="w-4 h-4" />关闭</button>
            </div>
            <div className="px-5 py-4 space-y-4">
              <div>
                <div className="flex items-center gap-1.5 text-xs font-medium text-slate-500 mb-1.5"><GitBranch className="w-3.5 h-3.5" />被测仓库</div>
                <select value={genRepo} onChange={(e) => { const r = e.target.value; const repo = REPOS.find((x) => x.name === r); setGenRepo(r); setGenBranch(repo?.branches?.[0] ?? 'main'); setGenVer((repo?.versions ?? ['v2.4.1'])[0]); }}
                  className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white text-slate-700 focus:outline-none focus:border-emerald-400">
                  {REPOS.map((r) => <option key={r.name} value={r.name}>{r.name}</option>)}
                </select>
              </div>
              <div>
                <div className="flex items-center gap-1.5 text-xs font-medium text-slate-500 mb-1.5"><GitBranch className="w-3.5 h-3.5" />分支（测试环境可能运行在指定分支）</div>
                <select value={genBranch} onChange={(e) => setGenBranch(e.target.value)}
                  className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white text-slate-700 focus:outline-none focus:border-emerald-400">
                  {(REPOS.find((r) => r.name === genRepo)?.branches ?? ['main']).map((b) => <option key={b} value={b}>{b}</option>)}
                </select>
              </div>
              <div>
                <div className="flex items-center gap-1.5 text-xs font-medium text-slate-500 mb-1.5"><CloudDownload className="w-3.5 h-3.5" />目标版本（用于执行前环境版本校验）</div>
                <select value={genVer} onChange={(e) => setGenVer(e.target.value)}
                  className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white text-slate-700 focus:outline-none focus:border-emerald-400">
                  {(REPOS.find((r) => r.name === genRepo)?.versions ?? []).map((v) => <option key={v} value={v}>{v}</option>)}
                </select>
                <div className="text-[10px] text-slate-300 mt-1">生成时锁定分支与版本，执行前校验测试环境运行的分支/版本与之一致，防止「测了也白测」</div>
              </div>
              <div className="flex items-start gap-2 text-[11px] text-slate-500 bg-slate-50 border border-slate-200 rounded-lg px-3 py-2">
                <BookOpen className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
                <span>AI 将读取 <b>{genRepo}@{genBranch}@{genVer}</b> 的仓库代码（接口 / 业务逻辑）与文档（README / 接口契约 / 需求）→ 生成用例规格 → 翻译为对应场景的执行载体 → 进入「待审核」。分支/版本将用于执行前环境校验。</span>
              </div>
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setGenPanel(false)} className="px-3 py-1.5 text-xs border border-slate-200 rounded-lg text-slate-500 hover:bg-slate-50">取消</button>
                <button type="button" onClick={() => { setGenPanel(false); toast('用例已生成', { description: `AI 依据 ${genRepo}@${genVer} 的代码与文档生成一批用例，进入待审核（原型 mock）` }); }}
                  className="px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg hover:bg-emerald-700">生成用例</button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

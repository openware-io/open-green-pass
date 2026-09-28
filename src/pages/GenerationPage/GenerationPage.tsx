import { useState } from 'react';
import { UPSTREAM_ADAPTERS } from '@/data/mock';
import { PageHeader, GhostButton, PrimaryButton, Card, ListFilter } from '@/components/shared';
import { toast } from 'sonner';
import { Sparkles, Plug, ShieldCheck, UserCheck, Database, GitBranch, BookOpen, PlusCircle, CloudDownload, ChevronLeft, ChevronRight, Check } from 'lucide-react';

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

// ============ 被测仓库：AI 生成用例的输入（仓库 + 分支 + 版本） ============
interface IRepo { name: string; source: string; branch: string; branches: string[]; versions: string[]; synced: string; owner: string }
const REPOS: IRepo[] = [
  { name: 'svc-payment', source: 'GitLab', branch: 'main', branches: ['main', 'release', 'feature-refund-v2'], versions: ['v2.4.1', 'v2.4.0'], synced: '2026-09-27', owner: '张立' },
  { name: 'svc-auth', source: 'GitLab', branch: 'main', branches: ['main', 'develop'], versions: ['v2.3.0'], synced: '2026-09-25', owner: '张立' },
  { name: 'svc-user', source: 'GitHub', branch: 'main', branches: ['main'], versions: ['v2.1.2'], synced: '2026-09-22', owner: '张立' },
  { name: 'web-frontend', source: 'GitLab', branch: 'release', branches: ['release', 'main'], versions: ['v2.4.0'], synced: '2026-09-26', owner: '李伟' },
  { name: 'mobile-ios', source: 'GitHub', branch: 'main', branches: ['main', 'release'], versions: ['v2.4.0'], synced: '2026-09-24', owner: '李伟' },
  { name: 'svc-order', source: 'GitLab', branch: 'main', branches: ['main', 'develop'], versions: ['v2.0.5'], synced: '2026-09-20', owner: '张立' },
];

// 生成流程向导步骤
const FLOW_STEPS = [
  { key: 'repo', label: '关联仓库', icon: GitBranch, meta: '被测仓库 · 分支 · 版本' },
  { key: 'adapter', label: '适配解析', icon: Plug, meta: '六类上游源 → 统一格式' },
  { key: 'seed', label: 'AI 生成种子', icon: Sparkles, meta: '读取代码与文档生成' },
  { key: 'quality', label: '质量验证', icon: ShieldCheck, meta: '变异 + 断言 + 重复' },
  { key: 'review', label: '人工审核', icon: UserCheck, meta: '逐条审核用例' },
  { key: 'storage', label: '用例入库', icon: Database, meta: '确认入库 · 批次记录' },
];
// 人工审核示例清单
const AUDIT_ROWS = [
  { id: 'SD-2401', scene: '退款金额含优惠券 · 正路径', assertion: '强', from: 'REQ-104 + refund-v2' },
  { id: 'SD-2402', scene: '退款金额含优惠券 · 金额边界', assertion: '强', from: 'REQ-104 + OpenAPI' },
  { id: 'SD-2403', scene: '支付超时自动撤销 · 正常流程', assertion: '强', from: 'REQ-104 + 状态机' },
  { id: 'SD-2404', scene: '支付超时自动撤销 · 幂等重试', assertion: '中', from: 'REQ-104 + 生产追踪' },
];

// 六类上游源接入状态（关联仓库步骤展示）
const SOURCE_CHIPS = [
  { label: '需求文档', from: 'Jira', ok: true },
  { label: '设计文档', from: 'Confluence', ok: true },
  { label: 'API 契约', from: 'OpenAPI/Proto', ok: true },
  { label: '代码变更', from: 'Git PR', ok: true },
  { label: '生产追踪', from: 'APM Trace', ok: true },
  { label: '历史缺陷', from: '缺陷库', ok: true },
];

export default function GenerationPage() {
  const [q, setQ] = useState('');
  const kw = q.trim().toLowerCase();
  const upFiltered = UPSTREAM_ADAPTERS.filter((a) => !kw || (a.source + a.status).toLowerCase().includes(kw));

  // 生成流程向导状态
  const [step, setStep] = useState(1);
  const [genRepo, setGenRepo] = useState('svc-payment');
  const [genBranch, setGenBranch] = useState('main');
  const [genVer, setGenVer] = useState('v2.4.1');
  const [stored, setStored] = useState(false);
  const [audit, setAudit] = useState<Record<string, string>>({});

  const cur = FLOW_STEPS[step - 1];

  const pickRepo = (r: IRepo) => { setGenRepo(r.name); setGenBranch(r.branches[0]); setGenVer(r.versions[0]); setStep(1); toast.success('已关联仓库', { description: `${r.name} 已选择为生成输入，进入「关联仓库」步骤` }); };

  return (
    <div>
      <PageHeader title="上游源与生成" desc="用例生成流程向导 · 关联仓库 → AI 生成 → 质量验证 → 审核入库">
        <GhostButton onClick={() => toast('配置适配器', { description: '管理六类上游源的同步规则与解析策略（原型示意）' })}>配置适配器</GhostButton>
        <PrimaryButton onClick={() => setStep(1)}>开始生成流程</PrimaryButton>
      </PageHeader>

      {/* 被测仓库：仓库管理（流程的第一步 · 关联仓库的输入源） */}
      <Card title="被测仓库 · 仓库管理" className="p-5 mb-5">
        <div className="flex items-start justify-between mb-4 gap-3 flex-wrap">
          <p className="text-[11px] text-slate-500 leading-relaxed max-w-xl">
            研发团队管理者在此<b>添加代码仓库</b>并获取<b>仓库代码与版本</b>。关联仓库是生成流程的第一步——点某仓库「发起生成」即进入下方流程向导并预选该仓库；用错分支/版本可在用例管理回退。
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
              <tr key={r.name} className={'hover:bg-slate-50 ' + (r.name === genRepo ? 'bg-emerald-50/40' : '')}>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <span className="flex items-center justify-center w-5 h-5 rounded bg-emerald-50 text-emerald-600"><GitBranch className="w-3.5 h-3.5" /></span>
                    <span className="font-medium text-slate-700">{r.name}</span>
                    {r.name === genRepo && <span className="text-[10px] text-emerald-600">已关联</span>}
                  </div>
                </td>
                <td className="px-4 py-3 text-slate-500">{r.source}</td>
                <td className="px-4 py-3 text-slate-500">{r.branch}</td>
                <td className="px-4 py-3"><span className="font-mono text-indigo-600">{r.versions[0]}</span><span className="text-slate-400 ml-1 text-[10px]">+{r.versions.length - 1}</span></td>
                <td className="px-4 py-3 text-slate-500">{r.synced}</td>
                <td className="px-4 py-3 text-slate-500">{r.owner}</td>
                <td className="px-4 py-3 text-right">
                  <button type="button" onClick={() => pickRepo(r)}
                    className="px-2.5 py-1 rounded-md text-[11px] bg-emerald-600 text-white hover:bg-emerald-700">发起生成</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>

      {/* 用例生成流程向导（六步可交互） */}
      <Card title="用例生成流程 · 批次 #GEN-2041" className="p-5 mb-5">
        {/* 步骤导航 */}
        <div className="flex items-center mb-5">
          {FLOW_STEPS.map((s, i) => {
            const n = i + 1;
            const done = stored || n < step;
            const active = n === step;
            return (
              <div key={s.key} className="flex flex-1 items-center">
                <button type="button" onClick={() => { if (done || active) return; setStep(n); }}
                  className={'flex-1 text-center rounded-lg px-2 py-2 border transition ' + (active ? 'bg-emerald-600 border-emerald-600 text-white' : done ? 'bg-emerald-50 border-emerald-200 text-emerald-700 cursor-pointer hover:border-emerald-400' : 'bg-slate-50 border-slate-200 text-slate-400')}>
                  <div className="flex items-center justify-center gap-1.5">
                    <span className={'w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-bold ' + (active ? 'bg-white/20' : done ? 'bg-emerald-100' : 'bg-slate-100')}>
                      {done && !active ? <Check className="w-3 h-3" /> : n}
                    </span>
                    <span className="text-xs font-medium">{s.label}</span>
                  </div>
                  <div className={'text-[9px] mt-1 ' + (active ? 'text-emerald-100' : done ? 'text-emerald-500' : 'text-slate-300')}>{s.meta}</div>
                </button>
                {n < FLOW_STEPS.length && <div className="w-6 flex items-center text-slate-300 justify-center"><ChevronRight className="w-4 h-4" /></div>}
              </div>
            );
          })}
        </div>

        {/* 当前步骤内容 */}
        <div className="border border-slate-100 rounded-lg bg-slate-50/40 p-4 min-h-[180px]">
          {cur.key === 'repo' && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-50 text-emerald-600"><GitBranch className="w-4 h-4" /></span>
                关联被测仓库（生成流程第一步 · 仓库关联）
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <div className="text-[11px] text-slate-500 mb-1">被测仓库</div>
                  <select value={genRepo} onChange={(e) => { const r = e.target.value; const repo = REPOS.find((x) => x.name === r); setGenRepo(r); setGenBranch(repo?.branches?.[0] ?? 'main'); setGenVer((repo?.versions ?? ['v2.4.1'])[0]); }}
                    className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white text-slate-700 focus:outline-none focus:border-emerald-400">
                    {REPOS.map((r) => <option key={r.name} value={r.name}>{r.name}</option>)}
                  </select>
                </div>
                <div>
                  <div className="text-[11px] text-slate-500 mb-1">分支</div>
                  <select value={genBranch} onChange={(e) => setGenBranch(e.target.value)}
                    className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white text-slate-700 focus:outline-none focus:border-emerald-400">
                    {(REPOS.find((r) => r.name === genRepo)?.branches ?? ['main']).map((b) => <option key={b} value={b}>{b}</option>)}
                  </select>
                </div>
                <div>
                  <div className="text-[11px] text-slate-500 mb-1">目标版本</div>
                  <select value={genVer} onChange={(e) => setGenVer(e.target.value)}
                    className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white text-slate-700 focus:outline-none focus:border-emerald-400">
                    {(REPOS.find((r) => r.name === genRepo)?.versions ?? []).map((v) => <option key={v} value={v}>{v}</option>)}
                  </select>
                </div>
              </div>
              <div className="flex items-start gap-2 text-[11px] text-slate-500 bg-white border border-slate-200 rounded-lg px-3 py-2">
                <CloudDownload className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
                <span>关联仓库后获取<b>仓库代码与版本</b>（{genRepo}@{genBranch}@{genVer}），作为 AI 生成用例的输入；分支/版本将用于执行前环境版本校验。下方为可关联的六类上游源接入状态。</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {SOURCE_CHIPS.map((sc) => (
                  <span key={sc.label} className="flex items-center gap-1 text-[10px] bg-white border border-slate-200 px-2 py-1 rounded-md">
                    <Check className="w-3 h-3 text-emerald-500" /><span className="font-medium text-slate-600">{sc.label}</span>
                    <span className="text-slate-400">{sc.from}</span>
                  </span>
                ))}
              </div>
            </div>
          )}

          {cur.key === 'adapter' && (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-indigo-50 text-indigo-600"><Plug className="w-4 h-4" /></span>
                适配解析 · 六类上游源 → 统一格式
              </div>
              <div className="grid grid-cols-2 md:grid-cols-3 gap-2.5">
                {UPSTREAM_ADAPTERS.map((a) => (
                  <div key={a.source} className="bg-white border border-slate-200 rounded-lg px-3 py-2">
                    <div className="text-[11px] font-medium text-slate-600">{a.source}</div>
                    <div className="text-[10px] text-slate-400 mt-0.5">{a.extract}</div>
                    <div className="text-[10px] text-emerald-600 mt-1">统一为结构化规格</div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {cur.key === 'seed' && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-amber-50 text-amber-600"><Sparkles className="w-4 h-4" /></span>
                AI 生成种子 · 读取 {genRepo}@{genBranch}@{genVer} 代码与文档
              </div>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                {AI_TRACES.map((t, i) => (
                  <div key={t.step} className="bg-amber-50/50 border border-amber-100 rounded-lg p-3">
                    <div className="flex items-center gap-2 mb-1.5">
                      <span className="w-5 h-5 rounded-full bg-amber-500 text-white flex items-center justify-center text-[10px] font-bold">{i + 1}</span>
                      <span className="text-xs font-medium text-slate-700">{t.step}</span>
                    </div>
                    <div className="text-[10px] text-slate-500 leading-relaxed">{t.desc}</div>
                    <div className="mt-1.5 text-[10px] text-amber-600 font-medium flex items-center gap-1"><Sparkles className="w-3 h-3" />{t.out}</div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {cur.key === 'quality' && (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-50 text-emerald-600"><ShieldCheck className="w-4 h-4" /></span>
                质量验证 · 变异 + 断言 + 重复 + 追溯
              </div>
              <div className="grid grid-cols-2 gap-3">
                {QUALITY.map((qv) => (
                  <div key={qv.label} className="bg-white border border-slate-200 rounded-lg px-3 py-2">
                    <div className="flex justify-between text-xs mb-1.5">
                      <span className="text-slate-600">{qv.label}</span>
                      <span className="text-slate-500">{qv.val}</span>
                    </div>
                    <div className="h-2 bg-slate-100 rounded-full">
                      <div className={qv.color + ' h-full rounded-full'} style={{ width: qv.width + '%' }} />
                    </div>
                    <div className="text-[10px] text-slate-400 mt-1">{qv.note}</div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {cur.key === 'review' && (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-purple-50 text-purple-600"><UserCheck className="w-4 h-4" /></span>
                人工审核 · 逐条确认用例
              </div>
              <table className="w-full text-xs bg-white border border-slate-200 rounded-lg overflow-hidden">
                <thead className="bg-slate-50">
                  <tr className="text-left text-slate-500">
                    <th className="px-3 py-2 font-medium">种子</th>
                    <th className="px-3 py-2 font-medium">覆盖场景</th>
                    <th className="px-3 py-2 font-medium">断言</th>
                    <th className="px-3 py-2 font-medium">来源</th>
                    <th className="px-3 py-2 font-medium text-right">审核</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {AUDIT_ROWS.map((ar) => (
                    <tr key={ar.id}>
                      <td className="px-3 py-2 font-mono text-indigo-600">{ar.id}</td>
                      <td className="px-3 py-2 text-slate-600">{ar.scene}</td>
                      <td className="px-3 py-2 text-slate-500">{ar.assertion}</td>
                      <td className="px-3 py-2 text-slate-400">{ar.from}</td>
                      <td className="px-3 py-2 text-right">
                        {audit[ar.id] === '通过' ? (
                          <span className="text-emerald-600 text-[11px] flex items-center justify-end gap-1"><Check className="w-3 h-3" />已通过</span>
                        ) : audit[ar.id] === '剔除' ? (
                          <span className="text-red-500 text-[11px]">已剔除</span>
                        ) : (
                          <span className="flex justify-end gap-1.5">
                            <button type="button" onClick={() => setAudit({ ...audit, [ar.id]: '通过' })}
                              className="text-[10px] px-1.5 py-0.5 rounded border border-emerald-200 text-emerald-600 hover:bg-emerald-50">通过</button>
                            <button type="button" onClick={() => setAudit({ ...audit, [ar.id]: '剔除' })}
                              className="text-[10px] px-1.5 py-0.5 rounded border border-slate-200 text-slate-400 hover:bg-red-50 hover:text-red-500">剔除</button>
                          </span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {cur.key === 'storage' && (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-50 text-emerald-600"><Database className="w-4 h-4" /></span>
                用例入库 · 确认本批用例进入用例库
              </div>
              <div className="flex items-start gap-2 text-[11px] text-slate-500 bg-white border border-slate-200 rounded-lg px-3 py-2">
                <BookOpen className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
                <span>本批经审核通过 <b>{Object.values(audit).filter((v) => v === '通过').length}</b> 条种子将生成版本化用例并入库（{genRepo}@{genBranch}@{genVer}）；生成批次 #GEN-2041 将被记录，若用错分支/版本可到用例管理页回退到本次生成之前。</span>
              </div>
            </div>
          )}
        </div>

        {/* 步骤操作 */}
        <div className="mt-4 flex items-center justify-between">
          <div className="text-[11px] text-slate-400">
            {stored ? '本批次已入库并记录，可到用例管理回退' : `步骤 ${step} / ${FLOW_STEPS.length} · ${cur.label}`}
          </div>
          <div className="flex gap-2">
            {step > 1 && (
              <button type="button" onClick={() => setStep(step - 1)}
                className="flex items-center gap-1 px-3 py-1.5 text-xs border border-slate-200 rounded-lg text-slate-500 hover:bg-slate-50">
                <ChevronLeft className="w-3.5 h-3.5" />上一步
              </button>
            )}
            {step < FLOW_STEPS.length ? (
              <button type="button" onClick={() => setStep(step + 1)}
                className="flex items-center gap-1 px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg hover:bg-emerald-700">
                下一步 · {FLOW_STEPS[step]?.label}<ChevronRight className="w-3.5 h-3.5" />
              </button>
            ) : (
              <button type="button" onClick={() => { setStored(true); toast.success('用例已入库', { description: `批次 #GEN-2041 · ${genRepo}@${genBranch}@${genVer} 的用例已入库，可到用例管理查看/回退` }); }}
                className="flex items-center gap-1 px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg hover:bg-emerald-700">
                <Check className="w-3.5 h-3.5" />确认入库
              </button>
            )}
          </div>
        </div>
      </Card>

      {/* 上游源适配器明细 */}
      <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
        <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
          <h2 className="font-semibold text-slate-700 text-sm">上游源适配器 · 明细</h2>
          <span className="text-[11px] text-slate-400">共 {upFiltered.length} / {UPSTREAM_ADAPTERS.length} 个上游源</span>
        </div>
        <div className="flex items-center justify-between px-5 pt-3">
          <ListFilter search={q} onSearch={setQ} />
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
    </div>
  );
}

import { useState } from 'react';
import { Link, useNavigate, useOutletContext } from 'react-router-dom';
import { toast } from 'sonner';
import { ASSET_TREE, METRICS, GEN_TRACE, ASSET_RISKS, AI_MODELS, PROJECT_MODELS, TEST_SCENARIOS, TESTED_REPOS, assetToProfile, nodeRepos, type IAsset, type IAssetNode, type ITestedRepo } from '@/data/mock';
import { scenarioNav } from '@/context/scenarioNav';
import { KpiCard, PageHeader, PrimaryButton, GhostButton, ListFilter } from '@/components/shared';
import { Brain, ShieldAlert, ChevronRight, ChevronDown, Sparkles, GitBranch, CircleDot, FileSearch, History, CheckCircle2, AlertTriangle, XCircle, Layers, Cpu, Globe, Smartphone, X, Plus, ExternalLink, Check, Plug, Files, FileText, Wallet } from 'lucide-react';

const ADD_STEPS = ['基本信息', '代码源接入', '上游源', '确认'];

// 围绕当前被测对象的测试治理环节状态条：被测对象为轴心，每环节只显示 状态点+数字（交互本身表达，不铺文字）
const STAGE_FLOW = [
  { path: '/generation', label: '用例生成', icon: Sparkles, status: 'done', value: '12 用例' },
  { path: '/trace', label: '需求追溯', icon: GitBranch, status: 'done', value: '完整' },
  { path: '/cases', label: '用例管理', icon: Files, status: 'done', value: '32' },
  { path: '/exec', label: '测试执行', icon: CircleDot, status: 'partial', value: '5/12' },
  { path: '/gate', label: '质量门禁', icon: ShieldAlert, status: 'block', value: '3 阻断' },
  { path: '/history', label: '测试历史', icon: History, status: 'done', value: '18 次' },
  { path: '/report', label: '测试报告', icon: FileText, status: 'todo', value: '待生成' },
  { path: '/audit-cost', label: '成本审计', icon: Wallet, status: 'done', value: '¥2.4k' },
];
// 环节状态点：绿实心=完成、琥珀=待处理、红=阻断、灰=待办
const STAGE_DOT: Record<string, { cls: string; txt: string }> = {
  done: { cls: 'bg-emerald-500', txt: 'text-emerald-600' },
  partial: { cls: 'bg-amber-500', txt: 'text-amber-600' },
  block: { cls: 'bg-red-500', txt: 'text-red-600' },
  todo: { cls: 'bg-slate-300', txt: 'text-slate-400' },
};

const DIM_BAR: Record<string, string> = {
  '需求覆盖率': 'bg-emerald-500',
  '门禁通过': 'bg-amber-500',
  '契约健康': 'bg-emerald-500',
  '用例有效': 'bg-emerald-500',
  '追溯完整': 'bg-amber-500',
};
const GEN_ICON: Record<string, typeof History> = { '规格解析': FileSearch, '架构生成': Layers, '代码生成': Sparkles, '自检回放': CheckCircle2, '提交验收': ShieldAlert };
const TYPE_LABEL: Record<string, string> = { service: '服务', 'service-group': '服务组', app: '应用', end: '端', project: '工程', module: '模块' };
const TYPE_COLOR: Record<string, string> = {
  project: 'bg-indigo-500', 'service-group': 'bg-sky-500', service: 'bg-emerald-500',
  module: 'bg-teal-500', app: 'bg-purple-500', end: 'bg-amber-500',
};
const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles };
const ASSET_SCEN: Record<string, string[]> = {
  auth: ['SCEN-01', 'SCEN-02', 'SCEN-04'],
  payment: ['SCEN-02', 'SCEN-03', 'SCEN-05'],
  order: ['SCEN-02', 'SCEN-03'],
  user: ['SCEN-01', 'SCEN-02'],
  web: ['SCEN-06', 'SCEN-07', 'SCEN-08'],
  mobile: ['SCEN-09', 'SCEN-10', 'SCEN-11'],
  metric: ['SCEN-01', 'SCEN-02', 'SCEN-04'],
  alert: ['SCEN-01', 'SCEN-02'],
};
function assetScenarios(asset: IAsset) {
  const hits = Object.entries(ASSET_SCEN).filter(([k]) => asset.name.includes(k)).flatMap(([, v]) => v);
  const pool = hits.length ? hits : (asset.type === 'app' || asset.type === 'end' ? ['SCEN-06', 'SCEN-09'] : ['SCEN-02']);
  return [...new Set(pool)].map((id) => TEST_SCENARIOS.find((s) => s.id === id)).filter((s): s is NonNullable<typeof s> => !!s);
}

// 子树是否包含目标节点（用于默认展开当前链路）
function containsNode(n: IAssetNode, id: string): boolean {
  if (n.id === id) return true;
  return (n.children ?? []).some((c) => containsNode(c, id));
}

// 被测对象树节点（受控展开：默认折叠非当前链路，点击节点联动画像与全局上下文）
function TreeNode({ node, activeId, onSelect, depth, openMap, onToggle }: {
  node: IAssetNode; activeId: string; onSelect: (n: IAssetNode) => void; depth: number;
  openMap: Record<string, boolean>; onToggle: (n: IAssetNode) => void;
}) {
  const hasChildren = !!node.children && node.children.length > 0;
  const active = node.id === activeId;
  const open = openMap[node.id] ?? (hasChildren && containsNode(node, activeId));
  return (
    <div>
      <div
        role="button"
        onClick={() => { if (hasChildren) onToggle(node); onSelect(node); }}
        className={'flex items-center gap-2 rounded-lg py-2 cursor-pointer select-none ' + (active ? 'bg-emerald-50 text-emerald-700 font-medium' : 'text-slate-600 hover:bg-slate-50')}
        style={{ paddingLeft: depth * 18 + 10 }}
      >
        {hasChildren
          ? (open ? <ChevronDown className="w-3.5 h-3.5 text-slate-400 flex-shrink-0" /> : <ChevronRight className="w-3.5 h-3.5 text-slate-400 flex-shrink-0" />)
          : <span className="w-3.5 flex-shrink-0" />}
        <span className={'w-2 h-2 rounded-sm flex-shrink-0 ' + (TYPE_COLOR[node.type] ?? 'bg-slate-300')} />
        <span className="text-xs truncate">{node.name}</span>
        {!hasChildren && node.repo && (
          <span className="ml-1 inline-flex items-center gap-0.5 text-[9px] text-emerald-600 flex-shrink-0"><GitBranch className="w-3 h-3" />{node.repo}</span>
        )}
        {!hasChildren && !node.repo && (
          <span className="ml-1 text-[9px] text-slate-300 flex-shrink-0">未接入源码</span>
        )}
        {hasChildren && nodeRepos(node).length > 0 && (
          <span className="ml-1 text-[9px] text-slate-400 flex-shrink-0">{nodeRepos(node).length} 仓</span>
        )}
        <span className={'ml-auto text-[9px] font-normal flex-shrink-0 ' + (node.coverage >= 85 ? 'text-emerald-500' : 'text-amber-500')}>{node.coverage}%</span>
      </div>
      {hasChildren && open && node.children!.map((c) => (
        <TreeNode key={c.id} node={c} activeId={activeId} onSelect={onSelect} depth={depth + 1} openMap={openMap} onToggle={onToggle} />
      ))}
    </div>
  );
}

export default function TargetPage() {
  const navigate = useNavigate();
  const { selectedAsset, setSelectedAsset } = useOutletContext<{ selectedAsset: IAssetNode; setSelectedAsset: (n: IAssetNode) => void }>();
  const profile = assetToProfile(selectedAsset);
  const risks = profile.gateRules.filter((r) => r.status === 'block');
  const [openMap, setOpenMap] = useState<Record<string, boolean>>({});
  const [q, setQ] = useState('');
  const kw = q.trim().toLowerCase();
  const riskFiltered = ASSET_RISKS.filter((a) => !kw || (a.name + a.type).toLowerCase().includes(kw));
  const riskAssets = ASSET_RISKS.filter((a) => a.gate === '阻断');
  const modelBound = PROJECT_MODELS.find((p) => p.projectId === profile.name);
  const model = modelBound && AI_MODELS.find((m) => m.id === modelBound.modelId);

  // 被测对象页 = 被测仓库与版本管理的权威来源（仓库列表可增删）
  const [repos, setRepos] = useState<ITestedRepo[]>(TESTED_REPOS);
  // 添加仓库分步录入弹窗
  const [showAdd, setShowAdd] = useState(false);
  const [addStep, setAddStep] = useState(0);
  const [addForm, setAddForm] = useState({ name: '', type: 'GitLab', url: '', node: '服务', owner: '', ver: '', auth: 'Access Token', token: '' });
  const [addBranches, setAddBranches] = useState<string[]>([]);
  const [addBrInput, setAddBrInput] = useState('');
  const [connected, setConnected] = useState(false);
  const [addSrc, setAddSrc] = useState<string[]>([]);
  const urlPrefix = addForm.type === 'GitHub' ? 'https://github.com/org/' : addForm.type === 'Gitee' ? 'https://gitee.com/' : 'https://gitlab.example.com/';
  const srcOptions = ['需求文档', '设计文档', 'API 契约', '代码变更', '生产追踪', '缺陷报告'];
  const canNext = addStep === 0 ? addForm.name.trim() !== '' && addForm.url.trim() !== ''
    : addStep === 1 ? connected
    : addStep === 2 ? addSrc.length > 0
    : true;
  const goGenerate = (r: ITestedRepo) => { try { localStorage.setItem('greenpass_pick_repo', r.name); } catch { /* ignore */ } navigate('/generation'); };
  const submitAdd = () => {
    const nr: ITestedRepo = {
      name: addForm.name.trim(), source: addForm.type, branch: addBranches[0] ?? 'main',
      branches: addBranches.length ? addBranches : ['main'], versions: [addForm.ver.trim() || 'v1.0.0'],
      synced: '2026-09-28', owner: addForm.owner.trim() || '张立', src: addSrc,
    };
    setRepos((p) => [...p, nr]);
    setShowAdd(false); setAddStep(0); setConnected(false); setAddBranches([]); setAddSrc([]);
    setAddForm({ name: '', type: 'GitLab', url: '', node: '服务', owner: '', ver: '', auth: 'Access Token', token: '' });
    toast.success('仓库已添加', { description: `${nr.name} 已纳入被测对象库，可到「生成用例」对其发起用例生成` });
  };

  return (
    <div>
      <PageHeader title="被测对象" desc="被测对象树 + 画像一体 · 点击左侧树节点联动右侧画像与全局上下文">
        <PrimaryButton><Link to="/gate" className="flex items-center gap-1">进入质量门禁<ChevronRight className="w-4 h-4" /></Link></PrimaryButton>
      </PageHeader>

      <div className="grid grid-cols-[360px_1fr] gap-6 items-start">
        {/* ===== 左侧：被测对象树（一体化导航） ===== */}
        <aside className="space-y-5">
          <div className="card bg-white rounded-xl border border-slate-200 p-4">
            <h2 className="font-semibold text-slate-700 text-sm mb-1 flex items-center gap-1.5"><Layers className="w-4 h-4 text-emerald-500" />被测对象树</h2>
            <p className="text-[10px] text-slate-400 mb-3">四级层级：工程 → 服务组/服务 → 模块 → 应用/端 · 点击节点联动画像</p>
            <TreeNode node={ASSET_TREE} activeId={selectedAsset.id} onSelect={setSelectedAsset} depth={0}
              openMap={openMap} onToggle={(n) => setOpenMap((p) => ({ ...p, [n.id]: !(p[n.id] ?? (n.children ? containsNode(n, selectedAsset.id) : false)) }))} />
          </div>
          <div>
            <h2 className="font-semibold text-slate-700 text-sm mb-2">被测对象库全局指标</h2>
            <span className="text-[10px] text-slate-400 mb-2 block">库级汇总 · 右侧画像为当前所选节点</span>
            <div className="grid grid-cols-2 gap-2.5">
              {METRICS.map((m) => (
                <KpiCard key={m.label} label={m.label} value={`${m.value}%`} color={m.color} note={m.note} />
              ))}
            </div>
          </div>
        </aside>

        {/* ===== 右侧：当前节点画像（随树联动） ===== */}
        <div className="min-w-0 space-y-5">
          {/* 被测对象主卡：随所选节点联动 */}
          <div className="card bg-gradient-to-br from-emerald-600 to-teal-700 rounded-2xl border border-transparent p-6 text-white">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div className="flex items-center gap-4">
                <div className="w-12 h-12 rounded-xl bg-white/10 flex items-center justify-center">
                  <Brain className="w-6 h-6" />
                </div>
                <div>
                  <div className="text-xs text-emerald-100 flex items-center gap-2">
                    被测对象 · {TYPE_LABEL[selectedAsset.type] ?? selectedAsset.type}
                    <span className="text-[9px] bg-white/15 px-1.5 py-0.5 rounded-full">覆盖率 {selectedAsset.coverage}%</span>
                  </div>
                  <div className="text-xl font-bold">{profile.name}</div>
                  <div className="text-[11px] text-emerald-100 mt-1 flex items-center gap-1">
                    <Sparkles className="w-3 h-3" />门禁通过率 {profile.passRate}% · {profile.riskCount > 0 ? `${profile.riskCount} 项风险待处理` : '当前无未决风险'}
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-6">
                <div className="text-center">
                  <div className="text-3xl font-bold">{profile.qualityScore}</div>
                  <div className="text-[11px] text-emerald-100">质量评分</div>
                </div>
                <div className="text-center">
                  <div className="text-3xl font-bold text-amber-300">{profile.riskCount}</div>
                  <div className="text-[11px] text-emerald-100">未决风险</div>
                </div>
              </div>
            </div>
            <div className="mt-5">
              <div className="flex justify-between text-[11px] text-emerald-100 mb-1.5">
                <span>门禁通过率</span><span>{profile.passRate}%</span>
              </div>
              <div className="h-2 bg-white/15 rounded-full overflow-hidden">
                <div className="h-full bg-emerald-300 rounded-full" style={{ width: `${profile.passRate}%` }} />
              </div>
            </div>
            {model && (
              <div className="mt-3 pt-3 border-t border-white/15 flex items-center justify-between text-[11px] text-emerald-100">
                <span className="flex items-center gap-1.5"><Cpu className="w-3.5 h-3.5" />测试所用 AI 模型</span>
                <span className="flex items-center gap-2">
                  <span className="font-medium">{model.name}</span>
                  <span className="text-[9px] bg-white/15 px-2 py-0.5 rounded-full">{model.vendor}</span>
                </span>
              </div>
            )}
            {/* 来源溯源：被测对象 → 源码仓库 → 分支@版本（实现「来源可溯源」） */}
            {nodeRepos(selectedAsset).length > 0 ? (
              <div className="mt-3 pt-3 border-t border-white/15 text-[11px] text-emerald-100">
                <div className="flex items-center gap-1.5 mb-1.5"><GitBranch className="w-3.5 h-3.5" />来源溯源 · 源码仓库</div>
                <div className="flex flex-wrap gap-1.5">
                  {nodeRepos(selectedAsset).map((repo) => {
                    const rp = repos.find((x) => x.name === repo);
                    return (
                      <span key={repo} className="inline-flex items-center gap-1 bg-white/10 rounded-full px-2 py-0.5 font-mono">
                        {repo}<span className="text-emerald-200">{rp ? `${rp.branch}@${rp.versions[0]}` : '未同步'}</span>
                      </span>
                    );
                  })}
                </div>
              </div>
            ) : (
              <div className="mt-3 pt-3 border-t border-white/15 text-[11px] text-amber-200 flex items-center gap-1.5">
                <GitBranch className="w-3.5 h-3.5" />来源未接入源码仓库 —— 需绑定仓库方可溯源
              </div>
            )}
          </div>

          {/* 被测对象 = 轴心：环节状态条（一页一焦点，状态可见，不铺文字） */}
          <div className="card bg-white rounded-xl border border-slate-200 p-5">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><Layers className="w-4 h-4 text-emerald-500" />围绕 {profile.name} 的测试治理环节</h2>
              <span className="text-[11px] text-slate-400">状态点：绿=完成 · 琥珀=待处理 · 红=阻断 · 灰=待办 · 点击进入</span>
            </div>
            <div className="grid grid-cols-4 gap-2.5">
              {STAGE_FLOW.map((s) => {
                const Icon = s.icon;
                const dot = STAGE_DOT[s.status];
                const core = s.label === '用例生成' || s.label === '质量门禁';
                return (
                  <Link key={s.path} to={s.path}
                    className={'flex items-center gap-2.5 rounded-xl border px-3 py-2.5 transition hover:-translate-y-0.5 ' + (core ? 'bg-emerald-50/60 border-emerald-200 hover:border-emerald-300 hover:shadow-sm' : 'bg-white border-slate-200 hover:border-emerald-300 hover:shadow-sm')}>
                    <span className={'w-8 h-8 rounded-lg flex items-center justify-center shrink-0 ' + (core ? 'bg-emerald-500 text-white' : 'bg-emerald-50 text-emerald-600')}><Icon className="w-4 h-4" /></span>
                    <div className="min-w-0">
                      <div className="text-xs font-medium text-slate-700 flex items-center gap-1.5">{s.label}{core && <span className="text-[9px] text-emerald-500">核心</span>}</div>
                      <div className="flex items-center gap-1.5 mt-0.5">
                        <span className={'w-2 h-2 rounded-full flex-shrink-0 ' + dot.cls} />
                        <span className={'text-[11px] font-medium ' + dot.txt}>{s.value}</span>
                      </div>
                    </div>
                  </Link>
                );
              })}
            </div>
          </div>

          {/* 被测对象页 = 被测对象管理总入口：仓库与版本管理（权威来源） */}
          <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
            <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between flex-wrap gap-2">
              <div className="flex items-center gap-2.5">
                <h2 className="font-semibold text-slate-700 text-sm">仓库与版本管理</h2>
                <span className="text-[11px] bg-emerald-50 text-emerald-700 px-2 py-0.5 rounded-full font-medium">{repos.length} 个被测仓库</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-[10px] text-slate-400 hidden md:inline">被测对象库统一管理 · 选择「发起生成」跳转生成用例</span>
                <GhostButton onClick={() => setShowAdd(true)}>
                  <Plus className="w-3.5 h-3.5 mr-1" />添加仓库
                </GhostButton>
              </div>
            </div>
            <div className="px-5 py-2 bg-slate-50/50 border-b border-slate-100 flex items-center gap-2 text-[11px] text-slate-500">
              <ExternalLink className="w-3.5 h-3.5 text-emerald-500 shrink-0" />
              <span>仓库是<b>被测对象的资产底座</b>：此处管理仓库/分支/版本/上游源接入（树节点选中的服务将在下方高亮对应仓库）；用例生成在生成用例页从本库选择对象。</span>
            </div>
            <table className="w-full text-xs">
              <thead className="bg-slate-50 border-b border-slate-200">
                <tr className="text-left text-slate-500">
                  <th className="px-5 py-2.5 font-medium">被测仓库</th>
                  <th className="px-5 py-2.5 font-medium">代码源</th>
                  <th className="px-5 py-2.5 font-medium">分支</th>
                  <th className="px-5 py-2.5 font-medium">可用版本</th>
                  <th className="px-5 py-2.5 font-medium">接入上游源</th>
                  <th className="px-5 py-2.5 font-medium">最近同步</th>
                  <th className="px-5 py-2.5 font-medium">负责人</th>
                  <th className="px-5 py-2.5 font-medium text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {repos.map((r) => {
                  const linked = nodeRepos(selectedAsset).includes(r.name);
                  return (
                    <tr key={r.name} className={(linked ? 'bg-emerald-50/60' : 'hover:bg-slate-50')}>
                      <td className="px-5 py-3">
                        <div className="flex items-center gap-2">
                          <span className="flex items-center justify-center w-5 h-5 rounded bg-emerald-50 text-emerald-600"><GitBranch className="w-3.5 h-3.5" /></span>
                          <span className="font-medium text-slate-700">{r.name}</span>
                          {linked && <span className="text-[10px] text-emerald-600">来源仓库 · 树联动</span>}
                        </div>
                      </td>
                      <td className="px-5 py-3 text-slate-500">{r.source}</td>
                      <td className="px-5 py-3 text-slate-500">{r.branch}</td>
                      <td className="px-5 py-3"><span className="font-mono text-emerald-700">{r.versions[0]}</span><span className="text-slate-400 ml-1 text-[10px]">+{r.versions.length - 1}</span></td>
                      <td className="px-5 py-3"><span className="text-emerald-600">{r.src.length}/6 类</span></td>
                      <td className="px-5 py-3 text-slate-500">{r.synced}</td>
                      <td className="px-5 py-3 text-slate-500">{r.owner}</td>
                      <td className="px-5 py-3 text-right">
                        <button type="button" onClick={() => goGenerate(r)}
                          className="px-2.5 py-1 rounded-md text-[11px] bg-emerald-600 text-white hover:bg-emerald-700">发起生成</button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          {/* 该节点涉及的测试场景：打通 画像 ↔ 测试中心 */}
          <div className="card bg-white rounded-xl border border-slate-200 p-5">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><Layers className="w-4 h-4 text-emerald-500" />{profile.name} · 测试场景覆盖</h2>
              <span className="text-[11px] text-slate-400">关联的测试中心场景 · 点击进入对应场景闭环</span>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-3 2xl:grid-cols-4 gap-3">
              {assetScenarios(selectedAsset).map((s) => {
                const Icon = SCEN_ICON[s.icon] ?? Cpu;
                const lp = s.history[s.history.length - 1].p;
                return (
                  <button key={s.id} type="button"
                    onClick={() => { scenarioNav.go(s.id, 'cases'); navigate('/scenarios'); }}
                    className="flex items-center gap-3 border rounded-xl p-3 text-left transition-all hover:shadow-sm hover:border-emerald-300 hover:-translate-y-0.5">
                    <span className="w-9 h-9 rounded-lg bg-emerald-500 text-white flex items-center justify-center flex-shrink-0"><Icon className="w-5 h-5" /></span>
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium text-slate-800 flex items-center gap-1.5">{s.name}<span className="text-[10px] text-slate-400 font-mono">{s.id}</span></div>
                      <div className="text-[10px] text-slate-400">通过率 <span className={lp >= 90 ? 'text-emerald-600 font-medium' : 'text-amber-600 font-medium'}>{lp}%</span> · {s.form}</div>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>

          {/* AI 生成过程还原 */}
          <div className="card bg-white rounded-xl border border-slate-200 p-5">
            <div className="flex items-center justify-between mb-5">
              <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><History className="w-4 h-4 text-emerald-500" />AI 生成过程还原</h2>
              <span className="text-[11px] text-slate-400">从需求到提交验收 · 本对象由 AI 全流程产出</span>
            </div>
            <div className="flex flex-col md:flex-row gap-4">
              {GEN_TRACE.map((g) => {
                const Icon = GEN_ICON[g.step] ?? History;
                const done = g.status === 'done';
                const active = g.status === 'active';
                return (
                  <div key={g.seq} className="flex md:flex-1 items-start md:flex-col gap-3 relative">
                    <div className={`w-4 h-4 rounded-full flex-shrink-0 mt-1 md:mt-0 ${active ? 'bg-emerald-500 ring-4 ring-emerald-100' : done ? 'bg-emerald-400' : 'bg-slate-300'}`} />
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <Icon className="w-4 h-4 text-emerald-500" />
                        <span className="text-sm font-medium text-slate-700">{g.step}</span>
                        {active && <span className="text-[9px] bg-emerald-100 text-emerald-700 px-1.5 py-0.5 rounded-full">当前</span>}
                      </div>
                      <div className="text-[10px] text-slate-400 mt-0.5 font-mono">{g.actor} · {g.time}</div>
                      <div className="text-[11px] text-slate-600 mt-1 leading-relaxed">{g.desc}</div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* 质量评分构成 + 未决风险 + 风险明细 */}
          <div className="grid grid-cols-3 gap-5">
            <div className="card bg-white rounded-xl border border-slate-200 p-5">
              <h2 className="font-semibold text-slate-700 text-sm mb-4 flex items-center gap-1.5"><ShieldAlert className="w-4 h-4 text-emerald-500" />质量评分构成</h2>
              <div className="space-y-3.5">
                {profile.dims.map((d) => (
                  <div key={d.label}>
                    <div className="flex justify-between text-xs mb-1.5">
                      <span className="text-slate-600">{d.label}</span>
                      <span className="text-slate-500">{d.score}<span className="text-slate-400"> · {Math.round(d.weight * 100)}%</span></span>
                    </div>
                    <div className="h-2 bg-slate-100 rounded-full">
                      <div className={`h-full rounded-full ${DIM_BAR[d.label] ?? 'bg-emerald-500'}`} style={{ width: `${d.score}%` }} />
                    </div>
                    <div className="text-[10px] text-slate-400 mt-1">{d.note}</div>
                  </div>
                ))}
              </div>
              <div className="mt-4 pt-3 border-t border-slate-100 flex items-center justify-between text-[11px]">
                <span className="text-slate-500">综合评分（加权）</span>
                <span className="text-emerald-600 font-semibold">{profile.qualityScore} / 100</span>
              </div>
            </div>

            <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
              <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
                <h2 className="font-semibold text-slate-700 text-sm">当前未决风险 · {profile.name}</h2>
                <span className="text-[11px] text-red-500 font-medium">{risks.length} 项阻断</span>
              </div>
              <div className="divide-y divide-slate-100">
                {risks.length === 0 ? (
                  <div className="px-5 py-6 text-center text-[11px] text-slate-400">该被测对象门禁全部通过，无未决风险</div>
                ) : risks.map((r) => (
                  <div key={r.name} className="px-5 py-3.5 flex items-start gap-3">
                    <AlertTriangle className="w-4 h-4 text-red-500 flex-shrink-0 mt-0.5" />
                    <div className="flex-1">
                      <div className="text-sm font-medium text-slate-700">{r.name}</div>
                      <div className="text-[11px] text-slate-500 mt-0.5">{r.detail}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
              <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
                <h2 className="font-semibold text-slate-700 text-sm">被测对象风险明细</h2>
                <span className="text-[11px] text-slate-400">{riskAssets.length} 个阻断</span>
              </div>
              <div className="flex items-center justify-between px-5 pt-3">
                <ListFilter search={q} onSearch={setQ} />
                <span className="text-[11px] text-slate-400">共 {riskFiltered.length} / {ASSET_RISKS.length} 项</span>
              </div>
              <div className="divide-y divide-slate-100">
                {riskFiltered.map((a) => (
                  <div key={a.name} className="px-5 py-3 flex items-center gap-3">
                    <div className="flex-1">
                      <div className="text-xs font-medium text-slate-700 flex items-center gap-2">
                        {a.name}
                        <span className="text-[9px] text-slate-400 font-normal">{a.type}</span>
                      </div>
                      <div className="flex items-center gap-2 mt-1.5">
                        <div className="w-14 h-1 bg-slate-100 rounded-full overflow-hidden">
                          <div className={`h-full rounded-full ${a.coverage >= 85 ? 'bg-emerald-500' : 'bg-amber-500'}`} style={{ width: `${a.coverage}%` }} />
                        </div>
                        <span className="text-[9px] text-slate-400">{a.coverage}%</span>
                      </div>
                    </div>
                    {a.gate === '通过'
                      ? <span className="text-[10px] text-emerald-600 bg-emerald-50 px-2 py-0.5 rounded-full flex items-center gap-1 flex-shrink-0"><CheckCircle2 className="w-3 h-3" />通过</span>
                      : <span className="text-[10px] text-red-600 bg-red-50 px-2 py-0.5 rounded-full flex items-center gap-1 flex-shrink-0"><XCircle className="w-3 h-3" />阻断</span>}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* 添加仓库：四步分步录入弹窗（基本信息 → 代码源接入 → 上游源 → 确认） */}
      {showAdd && (
        <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl w-full max-w-2xl shadow-2xl max-h-[90vh] overflow-y-auto">
            <div className="px-6 py-4 border-b border-slate-200 flex items-center justify-between">
              <div>
                <h3 className="font-semibold text-slate-800 text-base">添加被测仓库</h3>
                <p className="text-[11px] text-slate-400 mt-0.5">纳入被测对象库 · 分步录入仓库/分支/版本与上游源接入</p>
              </div>
              <button type="button" onClick={() => setShowAdd(false)} className="w-8 h-8 rounded-lg hover:bg-slate-100 flex items-center justify-center text-slate-400"><X className="w-4 h-4" /></button>
            </div>

            {/* 分步导航 */}
            <div className="px-6 py-3 flex items-center gap-2 border-b border-slate-100">
              {ADD_STEPS.map((s, i) => (
                <div key={s} className="flex items-center gap-2">
                  <div className={'flex items-center gap-1.5 ' + (i === addStep ? 'text-emerald-600' : i < addStep ? 'text-emerald-500' : 'text-slate-400')}>
                    <span className={'w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-bold ' + (i === addStep ? 'bg-emerald-600 text-white' : i < addStep ? 'bg-emerald-100 text-emerald-600' : 'bg-slate-100')}>
                      {i < addStep ? <Check className="w-3 h-3" /> : i + 1}
                    </span>
                    <span className="text-xs font-medium">{s}</span>
                  </div>
                  {i < ADD_STEPS.length - 1 && <span className="w-6 h-px bg-slate-200" />}
                </div>
              ))}
            </div>

            <div className="px-6 py-5 min-h-[260px]">
              {/* 步骤1 基本信息 */}
              {addStep === 0 && (
                <div className="space-y-4">
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="text-xs text-slate-500 mb-1 block">仓库名 <span className="text-red-400">*</span></label>
                      <input value={addForm.name} onChange={(e) => setAddForm({ ...addForm, name: e.target.value })} placeholder="如 svc-notify"
                        className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                    </div>
                    <div>
                      <label className="text-xs text-slate-500 mb-1 block">代码源类型</label>
                      <select value={addForm.type} onChange={(e) => setAddForm({ ...addForm, type: e.target.value })}
                        className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white focus:outline-none focus:border-emerald-400">
                        <option>GitLab</option><option>GitHub</option><option>Gitee</option>
                      </select>
                    </div>
                    <div className="col-span-2">
                      <label className="text-xs text-slate-500 mb-1 block">仓库地址 <span className="text-red-400">*</span></label>
                      <input value={addForm.url} onChange={(e) => setAddForm({ ...addForm, url: e.target.value })} placeholder={urlPrefix + (addForm.name || 'repo')}
                        className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                    </div>
                    <div>
                      <label className="text-xs text-slate-500 mb-1 block">所属被测对象层级</label>
                      <select value={addForm.node} onChange={(e) => setAddForm({ ...addForm, node: e.target.value })}
                        className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white focus:outline-none focus:border-emerald-400">
                        <option>工程</option><option>服务组</option><option>服务</option><option>模块</option>
                      </select>
                    </div>
                    <div>
                      <label className="text-xs text-slate-500 mb-1 block">负责人</label>
                      <input value={addForm.owner} onChange={(e) => setAddForm({ ...addForm, owner: e.target.value })} placeholder="默认当前用户"
                        className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                    </div>
                  </div>
                  <div className="text-[11px] text-slate-400 bg-slate-50 rounded-lg px-3 py-2">代码源类型切换会联动仓库地址前缀（当前：{urlPrefix}…）。必填仓库名 + 地址后方可进入下一步。</div>
                </div>
              )}

              {/* 步骤2 代码源接入 */}
              {addStep === 1 && (
                <div className="space-y-4">
                  <div>
                    <label className="text-xs text-slate-500 mb-1 block">接入方式</label>
                    <div className="flex gap-2">
                      {['Access Token', 'SSH Key'].map((a) => (
                        <button key={a} type="button" onClick={() => setAddForm({ ...addForm, auth: a })}
                          className={'px-3 py-1.5 text-xs rounded-lg border ' + (addForm.auth === a ? 'bg-emerald-600 border-emerald-600 text-white' : 'border-slate-200 text-slate-500 hover:border-emerald-300')}>{a}</button>
                      ))}
                    </div>
                  </div>
                  <div>
                    <label className="text-xs text-slate-500 mb-1 block">{addForm.auth}</label>
                    <input value={addForm.token} onChange={(e) => setAddForm({ ...addForm, token: e.target.value })} placeholder={addForm.auth === 'Access Token' ? 'glpat-xxx' : 'ssh-ed25519 AAAA…'}
                      className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                  </div>
                  <button type="button" onClick={() => { if (!addForm.token.trim()) { toast('需填写凭据', { description: '请先填写接入凭据再检测连接' }); return; } setConnected(true); toast.success('连接成功 · 已获取分支列表'); }}
                    className={'px-3 py-1.5 text-xs rounded-lg border flex items-center gap-1 ' + (connected ? 'bg-emerald-50 border-emerald-200 text-emerald-600' : 'border-emerald-300 text-emerald-600 hover:bg-emerald-50')}>
                    <Plug className="w-3.5 h-3.5" />{connected ? '已连接 · 重新检测' : '检测连接'}
                  </button>
                  {connected && (
                    <div className="space-y-3">
                      <div>
                        <label className="text-xs text-slate-500 mb-1 block">默认分支</label>
                        <select value={addBranches[0] ?? 'main'} onChange={(e) => setAddBranches((p) => { const rest = p.filter((b) => b !== e.target.value); return [e.target.value, ...rest]; })}
                          className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 bg-white focus:outline-none focus:border-emerald-400">
                          {['main', 'develop', 'release', 'feature-refund-v2'].map((b) => <option key={b}>{b}</option>)}
                        </select>
                      </div>
                      <div>
                        <label className="text-xs text-slate-500 mb-1 block">维护分支（测试环境可能测试在指定分支）</label>
                        <div className="flex items-center gap-2 mb-2">
                          <input value={addBrInput} onChange={(e) => setAddBrInput(e.target.value)} placeholder="如 hotfix-1.0.1"
                            className="flex-1 text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                          <button type="button" onClick={() => { const b = addBrInput.trim(); if (b && !addBranches.includes(b)) setAddBranches((p) => [...p, b]); setAddBrInput(''); }}
                            className="text-xs px-2.5 py-1.5 border border-emerald-300 text-emerald-600 rounded-lg hover:bg-emerald-50">添加</button>
                        </div>
                        <div className="flex flex-wrap gap-2">
                          {addBranches.map((b) => (
                            <span key={b} className="flex items-center gap-1 text-[10px] bg-emerald-50 text-emerald-700 border border-emerald-200 px-2 py-0.5 rounded-md">
                              {b}<button type="button" onClick={() => setAddBranches((p) => p.filter((x) => x !== b))} className="text-emerald-400 hover:text-red-500"><X className="w-3 h-3" /></button>
                            </span>
                          ))}
                        </div>
                      </div>
                      <div>
                        <label className="text-xs text-slate-500 mb-1 block">目标版本</label>
                        <input value={addForm.ver} onChange={(e) => setAddForm({ ...addForm, ver: e.target.value })} placeholder="如 v1.0.0"
                          className="w-full text-xs border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:border-emerald-400" />
                      </div>
                    </div>
                  )}
                </div>
              )}

              {/* 步骤3 上游源 */}
              {addStep === 2 && (
                <div className="space-y-3">
                  <div className="text-xs text-slate-500">选择该被测对象接入的上游源（AI 生成用例的数据来源，至少选一类）</div>
                  <div className="grid grid-cols-2 md:grid-cols-3 gap-2.5">
                    {srcOptions.map((s) => {
                      const on = addSrc.includes(s);
                      return (
                        <button key={s} type="button" onClick={() => setAddSrc((p) => (on ? p.filter((x) => x !== s) : [...p, s]))}
                          className={'text-left border rounded-lg px-3 py-2.5 text-xs transition ' + (on ? 'bg-emerald-50 border-emerald-300' : 'border-slate-200 hover:border-emerald-300')}>
                          <div className="flex items-center justify-between">
                            <span className="font-medium text-slate-700">{s}</span>
                            <span className={'w-4 h-4 rounded border flex items-center justify-center ' + (on ? 'bg-emerald-600 border-emerald-600 text-white' : 'border-slate-300')}>{on && <Check className="w-3 h-3" />}</span>
                          </div>
                          <div className="text-[10px] text-slate-400 mt-1">{s === '需求文档' ? 'Jira' : s === '设计文档' ? 'Confluence' : s === 'API 契约' ? 'OpenAPI/Proto' : s === '代码变更' ? 'Git PR' : s === '生产追踪' ? 'APM Trace' : '缺陷库'}</div>
                        </button>
                      );
                    })}
                  </div>
                  <div className="text-[11px] text-slate-400">已选 <b className="text-emerald-600">{addSrc.length}</b> / {srcOptions.length} 类上游源</div>
                </div>
              )}

              {/* 步骤4 确认 */}
              {addStep === 3 && (
                <div className="space-y-3">
                  <div className="text-xs text-slate-500 mb-1">仓库将纳入被测对象库，并在「生成用例」作为生成输入。确认以下汇总：</div>
                  <div className="bg-slate-50 rounded-lg border border-slate-200 p-4 space-y-2 text-xs">
                    <div className="flex gap-2"><span className="text-slate-400 w-20 shrink-0">仓库</span><span className="font-medium text-slate-700">{addForm.name}@{addBranches[0] ?? 'main'}@{addForm.ver.trim() || 'v1.0.0'}</span></div>
                    <div className="flex gap-2"><span className="text-slate-400 w-20 shrink-0">代码源</span><span className="text-slate-700">{addForm.type} · {urlPrefix}{addForm.url || addForm.name}</span></div>
                    <div className="flex gap-2"><span className="text-slate-400 w-20 shrink-0">所属对象</span><span className="text-slate-700">{addForm.node} · {addForm.owner || '当前用户'}</span></div>
                    <div className="flex gap-2"><span className="text-slate-400 w-20 shrink-0">维护分支</span><span className="text-slate-700">{addBranches.length ? addBranches.join('、') : 'main'}</span></div>
                    <div className="flex gap-2"><span className="text-slate-400 w-20 shrink-0">上游源</span><span className="text-slate-700">{addSrc.length ? addSrc.join('、') : '—'}</span></div>
                  </div>
                </div>
              )}
            </div>

            <div className="px-6 py-4 border-t border-slate-200 flex items-center justify-between">
              <span className="text-[11px] text-slate-400">步骤 {addStep + 1} / {ADD_STEPS.length} · {ADD_STEPS[addStep]}</span>
              <div className="flex gap-2">
                {addStep > 0 && (
                  <button type="button" onClick={() => setAddStep((s) => s - 1)} className="px-3 py-1.5 text-xs border border-slate-200 rounded-lg text-slate-500 hover:bg-slate-50">上一步</button>
                )}
                {addStep < ADD_STEPS.length - 1 ? (
                  <button type="button" onClick={() => { if (!canNext) { toast('请完善必填项', { description: addStep === 0 ? '需填写仓库名与地址' : addStep === 1 ? '需检测连接成功' : '需至少接入一类上游源' }); return; } setAddStep((s) => s + 1); }}
                    className="flex items-center gap-1 px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg hover:bg-emerald-700">下一步 · {ADD_STEPS[addStep + 1]}<ChevronRight className="w-3.5 h-3.5" /></button>
                ) : (
                  <button type="button" onClick={submitAdd} className="flex items-center gap-1 px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg hover:bg-emerald-700"><Check className="w-3.5 h-3.5" />确认添加</button>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

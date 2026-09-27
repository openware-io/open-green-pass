import { Link, useNavigate, useOutletContext } from 'react-router-dom';
import { METRICS, GEN_TRACE, ASSET_RISKS, AI_MODELS, PROJECT_MODELS, TEST_SCENARIOS, assetToProfile, type IAsset } from '@/data/mock';
import { scenarioNav } from '@/context/scenarioNav';
import { KpiCard, PageHeader, PrimaryButton } from '@/components/shared';
import { Brain, ShieldAlert, ChevronRight, Sparkles, GitBranch, CircleDot, FileSearch, History, CheckCircle2, AlertTriangle, XCircle, Layers, Cpu, Globe, Smartphone } from 'lucide-react';

const STAGE_FLOW = [
  { path: '/generation', label: '接收与理解', icon: FileSearch, desc: 'AI 解析六类上游源' },
  { path: '/cases', label: '用例生成', icon: Sparkles, desc: 'AI 生成用例种子并验证' },
  { path: '/exec', label: '测试执行', icon: CircleDot, desc: '跨资产执行与证据捕获' },
  { path: '/gate', label: '质量门禁', icon: ShieldAlert, desc: '防 AI 自放水的判定' },
  { path: '/audit', label: '审计留痕', icon: GitBranch, desc: '哈希链可信审计' },
];

// 质量维度条形颜色
const DIM_BAR: Record<string, string> = {
  '需求覆盖率': 'bg-emerald-500',
  '门禁通过': 'bg-amber-500',
  '契约健康': 'bg-emerald-500',
  '用例有效': 'bg-teal-500',
  '追溯完整': 'bg-amber-500',
};

const GEN_ICON: Record<string, typeof History> = { '规格解析': FileSearch, '架构生成': Layers, '代码生成': Sparkles, '自检回放': CheckCircle2, '提交验收': ShieldAlert };

const TYPE_LABEL: Record<string, string> = { service: '服务', 'service-group': '服务组', app: '应用', end: '端', project: '工程', module: '模块' };
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

export default function TargetPage() {
  const navigate = useNavigate();
  const { selectedAsset } = useOutletContext<{ selectedAsset: IAsset }>();
  const profile = assetToProfile(selectedAsset);
  const risks = profile.gateRules.filter((r) => r.status === 'block');
  const riskAssets = ASSET_RISKS.filter((a) => a.gate === '阻断');
  const modelBound = PROJECT_MODELS.find((p) => p.projectId === profile.name);
  const model = modelBound && AI_MODELS.find((m) => m.id === modelBound.modelId);

  return (
    <div>
      <PageHeader title="被测对象画像" desc={`正在验收 ${profile.name} · 当前处于「${profile.stage}」阶段`}>
        <PrimaryButton><Link to="/gate" className="flex items-center gap-1">进入质量门禁<ChevronRight className="w-4 h-4" /></Link></PrimaryButton>
      </PageHeader>

      {/* 被测对象主卡：随所选资产联动 */}
      <div className="card bg-gradient-to-br from-emerald-600 to-teal-700 rounded-2xl border border-transparent p-6 mb-5 text-white">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex items-center gap-4">
            <div className="w-12 h-12 rounded-xl bg-white/10 flex items-center justify-center">
              <Brain className="w-6 h-6" />
            </div>
            <div>
              <div className="text-xs text-emerald-100 flex items-center gap-2">
                被测资产 · {TYPE_LABEL[selectedAsset.type] ?? selectedAsset.type}
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
        {/* 当前工程绑定的测试 AI 模型：每工程可独立选择 */}
        {model && (
          <div className="mt-3 pt-3 border-t border-white/15 flex items-center justify-between text-[11px] text-emerald-100">
            <span className="flex items-center gap-1.5"><Cpu className="w-3.5 h-3.5" />测试所用 AI 模型</span>
            <span className="flex items-center gap-2">
              <span className="font-medium">{model.name}</span>
              <span className="text-[9px] bg-white/15 px-2 py-0.5 rounded-full">{model.vendor}</span>
            </span>
          </div>
        )}
      </div>

      {/* 该资产涉及的测试场景：打通 资产画像 ↔ 测试中心 */}
      <div className="card bg-white rounded-xl border border-slate-200 p-5 mb-5">
        <div className="flex items-center justify-between mb-4">
          <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><Layers className="w-4 h-4 text-emerald-500" />{profile.name} · 测试场景覆盖</h2>
          <span className="text-[11px] text-slate-400">资产关联的测试中心场景 · 点击进入对应场景闭环</span>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-3 2xl:grid-cols-4 gap-3">
          {assetScenarios(selectedAsset).map((s) => {
            const Icon = SCEN_ICON[s.icon] ?? Cpu;
            const lp = s.history[s.history.length - 1].p;
            return (
              <button key={s.id} type="button"
                onClick={() => { scenarioNav.go(s.id, 'cases'); navigate('/scenarios'); }}
                className="flex items-center gap-3 border rounded-xl p-3 text-left transition-all hover:shadow-sm hover:border-emerald-300 hover:-translate-y-0.5">
                <span className="w-9 h-9 rounded-lg bg-emerald-500 text-white flex items-center justify-center flex-shrink-0"><Icon className="w-4.5 h-4.5" /></span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium text-slate-800 flex items-center gap-1.5">{s.name}<span className="text-[10px] text-slate-400 font-mono">{s.id}</span></div>
                  <div className="text-[10px] text-slate-400">通过率 <span className={lp >= 90 ? 'text-emerald-600 font-medium' : 'text-amber-600 font-medium'}>{lp}%</span> · {s.form}</div>
                </div>
              </button>
            );
          })}
        </div>
      </div>
      {/* AI 生成过程还原：被测对象如何由 AI 产出 */}
      <div className="card bg-white rounded-xl border border-slate-200 p-5 mb-5">
        <div className="flex items-center justify-between mb-5">
          <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><History className="w-4 h-4 text-emerald-500" />AI 生成过程还原</h2>
          <span className="text-[11px] text-slate-400">从需求到提交验收 · 本对象由 AI 全流程产出</span>
        </div>
        <div className="relative">
          {/* 连接线 */}
          <div className="absolute left-[7px] top-2 bottom-2 w-px bg-slate-200 md:left-0 md:top-1/2 md:w-auto md:h-px md:right-0" />
          <div className="flex flex-col md:flex-row gap-4">
            {GEN_TRACE.map((g) => {
              const Icon = GEN_ICON[g.step] ?? History;
              const done = g.status === 'done';
              const active = g.status === 'active';
              return (
                <div key={g.seq} className="flex md:flex-1 items-start md:flex-col gap-3 relative">
                  {/* 节点 */}
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
      </div>

      {/* 测试闭环：从接收 AI 产物到放行 */}
      <div className="card bg-white rounded-xl border border-slate-200 p-5 mb-5">
        <div className="flex items-center justify-between mb-4">
          <h2 className="font-semibold text-slate-700 text-sm">测试闭环 · 从接收 AI 产物到放行/拒绝</h2>
          <span className="text-[11px] text-slate-400">当前位于「{profile.stage}」阶段</span>
        </div>
        <div className="flex flex-wrap gap-3 items-stretch">
          {STAGE_FLOW.map((s, i) => {
            const Icon = s.icon;
            const current = s.label.includes('门禁');
            return (
              <div key={s.path} className="flex items-center gap-3 flex-1 min-w-[140px]">
                <Link to={s.path}
                  className={`flex-1 p-3 rounded-xl border transition ${current ? 'bg-emerald-600 border-emerald-600 text-white' : 'border-slate-200 hover:border-emerald-300 bg-white'}`}>
                  <div className="flex items-center gap-2">
                    <Icon className={`w-4 h-4 ${current ? 'text-white' : 'text-emerald-500'}`} />
                    <span className={`text-sm font-medium ${current ? 'text-white' : 'text-slate-700'}`}>{s.label}</span>
                  </div>
                  <div className={`text-[10px] mt-1 ${current ? 'text-emerald-100' : 'text-slate-400'}`}>{s.desc}</div>
                </Link>
                {i < STAGE_FLOW.length - 1 && <ChevronRight className="w-4 h-4 text-slate-300 flex-shrink-0" />}
              </div>
            );
          })}
        </div>
      </div>

      {/* 资产维度 KPI：与明细同源 */}
      <div className="flex items-center justify-between mb-2">
        <h2 className="font-semibold text-slate-700 text-sm">资产库全局指标</h2>
        <span className="text-[11px] text-slate-400">库级汇总对比 · 上方画像为当前所选资产局部维度</span>
      </div>
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-5">
        {METRICS.map((m) => (
          <KpiCard key={m.label} label={m.label} value={`${m.value}%`} color={m.color} note={m.note} />
        ))}
      </div>

      <div className="grid grid-cols-3 gap-5">
        {/* 质量评分构成（随所选资产联动） */}
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

        {/* 未决风险（随所选资产联动，门禁阻断项同源） */}
        <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
            <h2 className="font-semibold text-slate-700 text-sm">当前未决风险 · {profile.name}</h2>
            <span className="text-[11px] text-red-500 font-medium">{risks.length} 项阻断</span>
          </div>
          <div className="divide-y divide-slate-100">
            {risks.length === 0 ? (
              <div className="px-5 py-6 text-center text-[11px] text-slate-400">该资产门禁全部通过，无未决风险</div>
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

        {/* 资产风险明细（横向对比，全局） */}
        <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
            <h2 className="font-semibold text-slate-700 text-sm">资产风险明细</h2>
            <span className="text-[11px] text-slate-400">{riskAssets.length} 个阻断</span>
          </div>
          <div className="divide-y divide-slate-100">
            {ASSET_RISKS.map((a) => (
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
  );
}

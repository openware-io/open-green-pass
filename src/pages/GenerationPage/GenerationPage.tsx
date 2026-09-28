import { useState } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';
import { UPSTREAM_ADAPTERS, TESTED_REPOS, type IAssetNode } from '@/data/mock';
import { PageHeader, GhostButton, PrimaryButton, Card, ListFilter } from '@/components/shared';
import { toast } from 'sonner';
import { Sparkles, Plug, ShieldCheck, UserCheck, Database, GitBranch, BookOpen, CloudDownload, ChevronLeft, ChevronRight, Check, Crosshair, Layers } from 'lucide-react';

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

// 生成流程向导步骤（围绕当前被测对象）
const FLOW_STEPS = [
  { key: 'repo', label: '当前被测对象', icon: Crosshair, meta: '生成流程的轴心对象' },
  { key: 'adapter', label: '适配解析', icon: Plug, meta: '六类上游源 → 统一格式' },
  { key: 'seed', label: 'AI 生成种子', icon: Sparkles, meta: '读取代码与文档生成' },
  { key: 'quality', label: '质量验证', icon: ShieldCheck, meta: '变异 + 断言 + 重复' },
  { key: 'review', label: '人工审核', icon: UserCheck, meta: '逐条审核用例' },
  { key: 'storage', label: '用例入库', icon: Database, meta: '确认入库 · 批次记录' },
];
const FLOW_ICON: Record<string, typeof GitBranch> = Object.fromEntries(FLOW_STEPS.map((s) => [s.key, s.icon]));
// 人工审核示例清单
const AUDIT_ROWS = [
  { id: 'SD-2401', scene: '退款金额含优惠券 · 正路径', assertion: '强', from: 'REQ-104 + refund-v2' },
  { id: 'SD-2402', scene: '退款金额含优惠券 · 金额边界', assertion: '强', from: 'REQ-104 + OpenAPI' },
  { id: 'SD-2403', scene: '支付超时自动撤销 · 正常流程', assertion: '强', from: 'REQ-104 + 状态机' },
  { id: 'SD-2404', scene: '支付超时自动撤销 · 幂等重试', assertion: '中', from: 'REQ-104 + 生产追踪' },
];

const TYPE_LABEL: Record<string, string> = { service: '服务', 'service-group': '服务组', module: '模块', app: '应用', end: '端', project: '工程' };

export default function GenerationPage() {
  const navigate = useNavigate();
  const { selectedAsset } = useOutletContext<{ selectedAsset: IAssetNode }>();
  const [q, setQ] = useState('');
  const kw = q.trim().toLowerCase();
  const upFiltered = UPSTREAM_ADAPTERS.filter((a) => !kw || (a.source + a.status).toLowerCase().includes(kw));

  // 被测对象 = 全局轴心（页首被测对象导航切换），生成流程围绕它展开
  const repo = TESTED_REPOS.find((r) => r.name === selectedAsset.name) ?? null;
  // 生成输入的分支 / 版本（对象切换时 render 期重置为该对象默认分支/版本）
  const [genBranch, setGenBranch] = useState(() => repo?.branches?.[0] ?? 'main');
  const [genVer, setGenVer] = useState(() => repo?.versions?.[0] ?? 'v2.4.1');
  const [prevAssetId, setPrevAssetId] = useState(selectedAsset.id);
  if (prevAssetId !== selectedAsset.id) {
    setPrevAssetId(selectedAsset.id);
    setGenBranch(repo?.branches?.[0] ?? 'main');
    setGenVer(repo?.versions?.[0] ?? 'v2.4.1');
  }

  // 生成流程向导状态
  const [step, setStep] = useState(1);
  const [stored, setStored] = useState(false);
  const [audit, setAudit] = useState<Record<string, string>>({});
  const cur = FLOW_STEPS[step - 1];

  return (
    <div>
      <PageHeader title="生成用例" desc="针对当前被测对象的用例生成流程 · 适配解析 → AI 生成 → 质量验证 → 审核入库">
        <GhostButton onClick={() => toast('配置适配器', { description: '管理六类上游源的同步规则与解析策略（原型示意）' })}>配置适配器</GhostButton>
        <PrimaryButton onClick={() => setStep(1)}>开始生成流程</PrimaryButton>
      </PageHeader>

      {/* 被测对象 = 全局轴心：本流程围绕当前被测对象展开（切换用页首被测对象导航） */}
      <Card title="当前被测对象 · 生成轴心" className="p-5 mb-5">
        <div className="flex flex-wrap items-start gap-4">
          <div className="flex items-center gap-3 flex-1 min-w-[260px]">
            <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-emerald-500 to-teal-600 flex items-center justify-center text-white shrink-0">
              <Crosshair className="w-6 h-6" />
            </div>
            <div>
              <div className="text-[11px] text-slate-400 flex items-center gap-2">
                <Layers className="w-3 h-3 text-emerald-500" />被测对象 · {TYPE_LABEL[selectedAsset.type] ?? selectedAsset.type}
                <span className="text-[9px] bg-emerald-50 text-emerald-700 px-1.5 py-0.5 rounded-full">覆盖率 {selectedAsset.coverage}%</span>
              </div>
              <div className="text-xl font-bold text-slate-800">{selectedAsset.name}</div>
              <div className="text-[11px] text-slate-500 mt-0.5">本用例生成流程的轴心对象 · 切换对象请用页首「被测对象」导航（面包屑 / 层次筛选）</div>
            </div>
          </div>

          {repo ? (
            <div className="flex items-center gap-3 bg-slate-50 rounded-xl border border-slate-200 px-4 py-3 flex-1 min-w-[320px] flex-wrap">
              <span className="flex items-center gap-1.5 text-xs font-medium text-slate-600"><GitBranch className="w-3.5 h-3.5 text-emerald-500" />{repo.source}</span>
              <div>
                <div className="text-[10px] text-slate-400">分支（生成输入）</div>
                <select value={genBranch} onChange={(e) => setGenBranch(e.target.value)}
                  className="text-xs border border-slate-200 rounded-md px-2 py-1 bg-white text-slate-700 focus:outline-none focus:border-emerald-400">
                  {repo.branches.map((b) => <option key={b} value={b}>{b}</option>)}
                </select>
              </div>
              <div>
                <div className="text-[10px] text-slate-400">目标版本（执行前校验）</div>
                <select value={genVer} onChange={(e) => setGenVer(e.target.value)}
                  className="text-xs border border-slate-200 rounded-md px-2 py-1 bg-white text-slate-700 focus:outline-none focus:border-emerald-400">
                  {repo.versions.map((v) => <option key={v} value={v}>{v}</option>)}
                </select>
              </div>
              <span className="text-[11px] text-emerald-600 ml-auto">{repo.src.length}/{UPSTREAM_ADAPTERS.length} 类上游源接入</span>
            </div>
          ) : (
            <div className="flex items-center gap-3 bg-amber-50 rounded-xl border border-amber-200 px-4 py-3 text-[12px] text-amber-700 flex-1 min-w-[300px]">
              <CloudDownload className="w-4 h-4 shrink-0" />
              该被测对象暂无关联仓库，无法生成用例——请到<b>被测对象页</b>添加仓库与上游源接入。
            </div>
          )}

          <GhostButton onClick={() => navigate('/target')}>
            <Crosshair className="w-3.5 h-3.5 mr-1" />去被测对象页切换 / 管理
          </GhostButton>
        </div>

        {repo && (
          <div className="mt-4 pt-3 border-t border-slate-100 flex items-start gap-2 text-[11px] text-slate-500">
            <CloudDownload className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
            <span>关联被测对象后获取<b>仓库代码与版本</b>（{repo.name}@{genBranch}@{genVer}），作为 AI 生成用例的输入；分支/版本将用于执行前环境版本校验。下方为该对象已接入的上游源。</span>
          </div>
        )}
        {repo && (
          <div className="mt-3 flex flex-wrap gap-2">
            {UPSTREAM_ADAPTERS.map((sc) => {
              const on = repo.src.includes(sc.source);
              return (
                <span key={sc.source} className={'flex items-center gap-1 text-[10px] border px-2 py-1 rounded-md ' + (on ? 'bg-emerald-50 border-emerald-200 text-emerald-700' : 'bg-slate-50 border-slate-200 text-slate-300')}>
                  {on ? <Check className="w-3 h-3 text-emerald-500" /> : <span className="w-2 h-2 rounded-sm bg-slate-200" />}
                  <span className="font-medium">{sc.source}</span>
                  <span className={on ? 'text-emerald-400' : 'text-slate-300'}>{sc.system}</span>
                </span>
              );
            })}
          </div>
        )}
      </Card>

      {/* 用例生成流程向导（六步，围绕当前被测对象） */}
      <Card title={`用例生成流程 · 批次 #GEN-2041 · 针对 ${selectedAsset.name}`} className="p-5 mb-5">
        {/* 步骤导航 */}
        <div className="flex items-center mb-5">
          {FLOW_STEPS.map((s, i) => {
            const n = i + 1;
            const done = stored || n < step;
            const active = n === step;
            const Icon = FLOW_ICON[s.key];
            return (
              <div key={s.key} className="flex flex-1 items-center">
                <button type="button" onClick={() => { if (done || active) return; setStep(n); }}
                  className={'flex-1 text-center rounded-xl px-2 py-2.5 border-2 transition ' + (active ? 'bg-emerald-600 border-emerald-600 text-white shadow-md shadow-emerald-200' : done ? 'bg-emerald-50 border-emerald-200 text-emerald-700 cursor-pointer hover:border-emerald-300' : 'bg-white border-slate-200 text-slate-400 hover:border-slate-300')}>
                  <div className="flex items-center justify-center gap-2">
                    <span className={'relative w-9 h-9 rounded-full flex items-center justify-center ring-2 ring-offset-2 ' + (active ? 'bg-white ring-white/40 ring-offset-emerald-600' : done ? 'bg-emerald-600 ring-emerald-100 ring-offset-emerald-50' : 'bg-slate-100 ring-slate-100 ring-offset-white')}>
                      {done && !active ? (
                        <Check className="w-4 h-4 text-white" />
                      ) : (
                        <Icon className={'w-5 h-5 ' + (active ? 'text-emerald-600' : 'text-slate-500')} />
                      )}
                      <span className={'absolute -top-1 -right-1 w-4 h-4 rounded-full text-[9px] font-bold flex items-center justify-center ' + (active ? 'bg-white text-emerald-600' : done ? 'bg-white text-emerald-600' : 'bg-slate-200 text-slate-500')}>{n}</span>
                    </span>
                    <span className="text-xs font-semibold">{s.label}</span>
                  </div>
                  <div className={'text-[9px] mt-1.5 ' + (active ? 'text-emerald-100' : done ? 'text-emerald-500' : 'text-slate-300')}>{s.meta}</div>
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
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-50 text-emerald-600"><Crosshair className="w-4 h-4" /></span>
                当前被测对象（生成流程轴心 · 本流程第一步）
              </div>
              <div className="bg-white border border-emerald-200 rounded-xl p-4 flex items-start gap-3">
                <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-emerald-500 to-teal-600 flex items-center justify-center text-white shrink-0"><Crosshair className="w-5 h-5" /></div>
                <div>
                  <div className="text-[11px] text-slate-400">被测对象（轴心）</div>
                  <div className="text-lg font-bold text-slate-800">{selectedAsset.name}<span className="ml-2 text-[10px] font-normal text-slate-400">{TYPE_LABEL[selectedAsset.type] ?? selectedAsset.type} · 覆盖率 {selectedAsset.coverage}%</span></div>
                  {repo ? (
                    <div className="text-[11px] text-slate-500 mt-1">关联仓库 <b className="text-emerald-600">{repo.name}@{genBranch}@{genVer}</b> · 已接入 {repo.src.length}/{UPSTREAM_ADAPTERS.length} 类上游源 · 负责人 {repo.owner}</div>
                  ) : (
                    <div className="text-[11px] text-amber-600 mt-1">暂无关联仓库——到被测对象页添加后即可生成</div>
                  )}
                </div>
              </div>
              <div className="flex items-start gap-2 text-[11px] text-slate-500 bg-white border border-slate-200 rounded-lg px-3 py-2">
                <Crosshair className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
                <span>本生成流程<b>针对当前被测对象</b>展开：适配解析、AI 生成、质量验证、人工审核、用例入库都是围绕它（{selectedAsset.name}）的步骤。切换被测对象请用页首「被测对象」导航——切换后本流程将针对新对象。</span>
              </div>
            </div>
          )}

          {cur.key === 'adapter' && (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-50 text-emerald-700"><Plug className="w-4 h-4" /></span>
                适配解析 · {selectedAsset.name} 六类上游源 → 统一格式
              </div>
              <div className="grid grid-cols-2 md:grid-cols-3 gap-2.5">
                {UPSTREAM_ADAPTERS.map((a) => {
                  const on = repo?.src.includes(a.source);
                  return (
                    <div key={a.source} className={'bg-white border rounded-lg px-3 py-2 ' + (on ? 'border-slate-200' : 'border-slate-100 opacity-45')}>
                      <div className="flex items-center gap-1.5 text-[11px] font-medium text-slate-600">{on ? <Check className="w-3 h-3 text-emerald-500" /> : null}{a.source}</div>
                      <div className="text-[10px] text-slate-400 mt-0.5">{a.extract}</div>
                      <div className="text-[10px] text-emerald-600 mt-1">{on ? '统一为结构化规格' : '未接入'}</div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {cur.key === 'seed' && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-amber-50 text-amber-600"><Sparkles className="w-4 h-4" /></span>
                AI 生成种子 · 读取 {selectedAsset.name}@{genBranch}@{genVer} 代码与文档
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
                质量验证 · {selectedAsset.name} 变异 + 断言 + 重复 + 追溯
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
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-50 text-emerald-600"><UserCheck className="w-4 h-4" /></span>
                人工审核 · {selectedAsset.name} 逐条确认用例
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
                      <td className="px-3 py-2 font-mono text-emerald-700">{ar.id}</td>
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
                用例入库 · {selectedAsset.name} 确认本批用例进入用例库
              </div>
              <div className="flex items-start gap-2 text-[11px] text-slate-500 bg-white border border-slate-200 rounded-lg px-3 py-2">
                <BookOpen className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
                <span>本批经审核通过 <b>{Object.values(audit).filter((v) => v === '通过').length}</b> 条种子将生成版本化用例并入库（{selectedAsset.name}@{genBranch}@{genVer}）；生成批次 #GEN-2041 将被记录，若用错分支/版本可到用例管理页回退到本次生成之前。</span>
              </div>
            </div>
          )}
        </div>

        {/* 步骤操作 */}
        <div className="mt-4 flex items-center justify-between">
          <div className="text-[11px] text-slate-400">
            {stored ? '本批次已入库并记录，可到用例管理回退' : `步骤 ${step} / ${FLOW_STEPS.length} · ${cur.label}（针对 ${selectedAsset.name}）`}
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
              <button type="button" onClick={() => { setStored(true); toast.success('用例已入库', { description: `批次 #GEN-2041 · ${selectedAsset.name}@${genBranch}@${genVer} 的用例已入库，可到用例管理查看/回退` }); }}
                className="flex items-center gap-1 px-3 py-1.5 text-xs bg-emerald-600 text-white rounded-lg hover:bg-emerald-700">
                <Check className="w-3.5 h-3.5" />确认入库
              </button>
            )}
          </div>
        </div>
      </Card>

      {/* 上游源适配器明细（随当前被测对象联动） */}
      <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
        <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between flex-wrap gap-2">
          <div className="flex items-center gap-2.5">
            <h2 className="font-semibold text-slate-700 text-sm">上游源适配器 · 明细</h2>
            <span className="text-[11px] bg-emerald-50 text-emerald-700 px-2 py-0.5 rounded-full font-medium">被测对象 {selectedAsset.name} · 已接入 {repo?.src.length ?? 0}/{UPSTREAM_ADAPTERS.length}</span>
          </div>
          <span className="text-[11px] text-slate-400">共 {upFiltered.length} / {UPSTREAM_ADAPTERS.length} 个上游源</span>
        </div>
        <div className="px-5 py-2.5 bg-slate-50/50 border-b border-slate-100 flex items-center gap-2 text-[11px] text-slate-500">
          <Crosshair className="w-3.5 h-3.5 text-emerald-500 shrink-0" />
          <span>适配器明细随<b>当前被测对象</b>联动：页首切换被测对象，此处即展示该对象已接入的上游源；未接入源灰显（到被测对象页配置接入）。</span>
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
              <th className="px-5 py-2.5 font-medium">本对象接入</th>
              <th className="px-5 py-2.5 font-medium">状态</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {upFiltered.map((a) => {
              const attached = repo?.src.includes(a.source);
              return (
                <tr key={a.source} className={(attached ? (a.status === '验证中' ? 'bg-emerald-50/30 hover:bg-emerald-50' : 'hover:bg-slate-50') : 'opacity-45 hover:opacity-70 hover:bg-slate-50')}>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2">
                      <span className="w-5 h-5 rounded bg-slate-100 text-slate-600 flex items-center justify-center text-[10px]">源</span>
                      <span className="text-slate-700 font-medium">{a.source}</span>
                    </div>
                  </td>
                  <td className="px-5 py-3 text-slate-500">{a.system}</td>
                  <td className="px-5 py-3 text-slate-500">{a.extract}</td>
                  <td className="px-5 py-3">
                    {attached ? <span className="text-slate-600">{a.seeds}</span> : <span className="text-slate-300">—</span>}
                  </td>
                  <td className="px-5 py-3">
                    {attached
                      ? <span className="text-[10px] text-emerald-600 flex items-center gap-1"><Check className="w-3 h-3" />已接入</span>
                      : <span className="text-[10px] text-slate-300">未接入</span>}
                  </td>
                  <td className="px-5 py-3">
                    {attached ? (
                      <span className={a.status === '验证中' ? 'bg-amber-50 text-amber-600 px-2 py-0.5 rounded-full text-[10px]' : 'bg-emerald-50 text-emerald-600 px-2 py-0.5 rounded-full text-[10px]'}>
                        {a.status}
                      </span>
                    ) : (
                      <span className="bg-slate-50 text-slate-300 px-2 py-0.5 rounded-full text-[10px]">未接入</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

import { useState, useEffect } from 'react';
import { CASES, TEST_SCENARIOS, CURRENT_ITERATION, SERVICE_VERSIONS, CASE_EXEC, type ICase, type CaseChange, type ExecCarrierType } from '@/data/mock';
import { useNavigate } from 'react-router-dom';
import { scenarioNav } from '@/context/scenarioNav';
import { toast } from 'sonner';
import { Cpu, Globe, Smartphone, Sparkles, ShieldCheck, Pencil, Trash2, Power, Check, Ban, GitBranch, History, ArrowLeft, Package, RefreshCcw, Code2, Braces, Activity, Boxes, FlaskConical } from 'lucide-react';
import { PageHeader, GhostButton, PrimaryButton, ListFilter } from '@/components/shared';

const TYPE_BADGE: Record<string, string> = {
  '单元': 'bg-blue-50 text-blue-600',
  '集成': 'bg-purple-50 text-purple-600',
  'Web': 'bg-pink-50 text-pink-600',
  '移动': 'bg-teal-50 text-teal-600',
  '安全': 'bg-orange-50 text-orange-600',
};
const ASSERTION_COLOR: Record<string, string> = { '强': 'text-emerald-600', '中': 'text-amber-600', '弱': 'text-red-600' };
const STATUS_BADGE: Record<string, string> = {
  '已激活': 'bg-emerald-50 text-emerald-600',
  '冲突': 'bg-amber-50 text-amber-600',
  '待审核': 'bg-red-50 text-red-600',
  '已禁用': 'bg-slate-100 text-slate-500',
};
const CHANGE_BADGE: Record<string, string> = {
  '新增': 'bg-emerald-600 text-white',
  '更新': 'bg-amber-500 text-white',
  '删除': 'bg-red-600 text-white',
  '稳定': 'bg-slate-100 text-slate-500',
};
const CHANGE_ORDER: CaseChange[] = ['新增', '更新', '删除', '稳定'];
const CHANGE_ICON: Record<string, typeof GitBranch> = { '新增': GitBranch, '更新': RefreshCcw, '删除': Trash2, '稳定': History };
const EXEC_ICON: Record<string, typeof Code2> = { '脚本': Braces, 'HTTP': Globe, '规则': ShieldCheck, '压测': Activity };
const EXEC_BADGE: Record<ExecCarrierType, string> = {
  '脚本': 'bg-blue-50 text-blue-600',
  'HTTP': 'bg-purple-50 text-purple-600',
  '规则': 'bg-orange-50 text-orange-600',
  '压测': 'bg-teal-50 text-teal-600',
};
const SOURCE_BADGE: Record<string, string> = {
  '固定脚本': 'bg-slate-100 text-slate-500',
  '数据驱动': 'bg-amber-50 text-amber-600',
  'AI 动态生成': 'bg-emerald-50 text-emerald-600',
};

const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles, ShieldCheck };
const SCEN_OF: Record<string, string> = { '单元': 'SCEN-01', '集成': 'SCEN-02', '契约': 'SCEN-03', '安全': 'SCEN-07', 'Web': 'SCEN-08', '移动': 'SCEN-09' };

export default function CasesPage() {
  const navigate = useNavigate();
  const [rows, setRows] = useState<ICase[]>(CASES);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [q, setQ] = useState('');
  const [type, setType] = useState('');
  const [status, setStatus] = useState('');
  const [change, setChange] = useState('');
  const [detail, setDetail] = useState<{ c: ICase; tab: 'exec' | 'version' } | null>(null);

  useEffect(() => {
    if (!detail) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setDetail(null); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [detail]);

  const kw = q.trim().toLowerCase();
  const filtered = rows.filter((c) => {
    if (type && c.type !== type) return false;
    if (status && c.status !== status) return false;
    if (change && c.change !== change) return false;
    if (kw && !(c.id + c.title + c.asset + c.source).toLowerCase().includes(kw)) return false;
    return true;
  });

  const changeCount = CHANGE_ORDER.map((ch) => ({ ch, n: rows.filter((c) => c.change === ch).length }));

  const allSelected = filtered.length > 0 && filtered.every((c) => selected.has(c.id));
  const toggleSelect = (id: string) => setSelected((prev) => {
    const s = new Set(prev);
    if (s.has(id)) s.delete(id); else s.add(id);
    return s;
  });
  const toggleAll = () => {
    if (allSelected) setSelected(new Set());
    else setSelected(new Set(filtered.map((c) => c.id)));
  };
  const toggleStatus = (id: string) => {
    setRows(rows.map((c) => (c.id === id ? { ...c, status: (c.status === '已激活' ? '已禁用' : '已激活') as ICase['status'] } : c)));
    setSelected(new Set());
  };
  const removeCases = (ids: string[]) => {
    setRows(rows.filter((c) => !ids.includes(c.id)));
    setSelected((prev) => { const s = new Set(prev); ids.forEach((id) => s.delete(id)); return s; });
    toast('用例已删除', { description: `已移除 ${ids.length} 条用例（原型 mock：本地删除生效，历史保留在版本记录中）` });
  };
  const batchSet = (next: ICase['status']) => {
    const n = selected.size;
    setRows(rows.map((c) => (selected.has(c.id) ? { ...c, status: next } : c)));
    toast(next === '已激活' ? '批量激活' : '批量禁用', { description: `已将 ${n} 条用例设为「${next}」` });
    setSelected(new Set());
  };
  const restoreCase = (id: string) => {
    setRows(rows.map((c) => (c.id === id ? { ...c, change: '稳定' as CaseChange, status: '已激活' as ICase['status'] } : c)));
    toast('用例已恢复', { description: `${id} · 重新纳入管理（原型 mock）` });
  };

  return (
    <div>
      <PageHeader title="用例管理" desc="版本化用例 · 新增/更新/删除可辨别 · 执行载体与脚本数据 · 测试关联服务版本">
        <GhostButton onClick={() => toast('导入用例（原型 mock）', { description: '支持从上游 / 用例仓库批量导入，解析为版本化用例并绑定执行载体' })}>导入用例</GhostButton>
        <PrimaryButton onClick={() => toast('AI 生成用例（原型 mock）', { description: '将由 AI 依据上游源 / 需求生成用例规格，再翻译为对应场景的执行载体' })}>AI 生成用例</PrimaryButton>
      </PageHeader>

      <div className="card bg-white rounded-xl border border-slate-200 p-4 mb-4">
        <div className="flex items-center justify-between flex-wrap gap-3">
          <div className="flex items-center gap-2.5">
            <span className="flex items-center gap-1.5 text-[11px] font-medium text-emerald-700 bg-emerald-50 px-2.5 py-1 rounded-lg">
              <Package className="w-3.5 h-3.5" />当前迭代 {CURRENT_ITERATION.version}
            </span>
            <span className="text-xs text-slate-500">{CURRENT_ITERATION.name} · 发布于 {CURRENT_ITERATION.releasedAt}</span>
          </div>
          <div className="flex items-center gap-1.5">
            <span className="text-[11px] text-slate-400 mr-1">本轮变更</span>
            {changeCount.map(({ ch, n }) => (
              <button key={ch} type="button" onClick={() => setChange(change === ch ? '' : ch)}
                className={'flex items-center gap-1 px-2 py-1 rounded-md text-[11px] border transition ' + (change === ch ? 'border-emerald-400 bg-emerald-50 ' : 'border-slate-200 bg-white hover:border-emerald-300 ') + (ch === '稳定' ? 'text-slate-500' : '')}>
                <span className={'w-1.5 h-1.5 rounded-full ' + (ch === '新增' ? 'bg-emerald-600' : ch === '更新' ? 'bg-amber-500' : ch === '删除' ? 'bg-red-600' : 'bg-slate-300')} />
                {ch} <b className="font-mono">{n}</b>
              </button>
            ))}
          </div>
        </div>
        <div className="mt-3 pt-3 border-t border-slate-100 flex items-center gap-1.5 flex-wrap">
          <span className="text-[11px] text-slate-400 flex items-center gap-1"><GitBranch className="w-3 h-3" />关联服务版本</span>
          {SERVICE_VERSIONS.map((s) => (
            <span key={s.asset} className="flex items-center gap-1 text-[10px] bg-slate-50 border border-slate-200 px-2 py-0.5 rounded-md">
              <span className="font-mono text-slate-600">{s.asset}</span>
              <span className="font-mono font-semibold text-emerald-600">{s.version}</span>
              <span className={s.change === '稳定' ? 'text-slate-400' : 'text-amber-600'}>{s.change}</span>
            </span>
          ))}
          <span className="text-[10px] text-slate-300 ml-auto">执行 = 用例规格经执行器翻译成载体（脚本/请求/规则/负载），绑定被测对象当时版本</span>
        </div>
      </div>

      <div className="flex items-center justify-between mb-3">
        <ListFilter search={q} onSearch={setQ}
          selects={[
            { key: 'change', label: '变更', options: CHANGE_ORDER, value: change, onChange: setChange },
            { key: 'type', label: '用例类型', options: ['单元', '集成', 'Web', '移动', '安全'], value: type, onChange: setType },
            { key: 'status', label: '状态', options: ['已激活', '冲突', '待审核', '已禁用'], value: status, onChange: setStatus },
          ]} />
        <span className="text-[11px] text-slate-400">共 {filtered.length} / {rows.length} 条</span>
      </div>

      {selected.size > 0 && (
        <div className="flex items-center gap-2 flex-wrap mb-3 px-4 py-2 bg-emerald-50/70 border border-emerald-200 rounded-lg text-xs">
          <span className="text-emerald-700 font-medium">已选 {selected.size} 项</span>
          <button type="button" onClick={() => batchSet('已激活')}
            className="flex items-center gap-1 px-2 py-1 bg-white border border-slate-200 rounded-md text-slate-600 hover:text-emerald-600 hover:border-emerald-300">
            <Check className="w-3.5 h-3.5" />批量激活
          </button>
          <button type="button" onClick={() => batchSet('已禁用')}
            className="flex items-center gap-1 px-2 py-1 bg-white border border-slate-200 rounded-md text-slate-600 hover:text-amber-600 hover:border-amber-300">
            <Ban className="w-3.5 h-3.5" />批量禁用
          </button>
          <button type="button" onClick={() => removeCases([...selected])}
            className="flex items-center gap-1 px-2 py-1 bg-white border border-red-200 rounded-md text-red-600 hover:bg-red-50">
            <Trash2 className="w-3.5 h-3.5" />批量删除
          </button>
          <button type="button" onClick={() => setSelected(new Set())} className="px-2 py-1 text-slate-400 hover:text-slate-600">取消选择</button>
        </div>
      )}

      <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-xs text-slate-500">
              <th className="px-4 py-3 w-8">
                <input type="checkbox" checked={allSelected} onChange={toggleAll} className="accent-emerald-600" />
              </th>
              <th className="px-4 py-3 font-medium">用例 ID</th>
              <th className="px-4 py-3 font-medium">标题</th>
              <th className="px-4 py-3 font-medium">变更</th>
              <th className="px-4 py-3 font-medium">版本 / 最近变更</th>
              <th className="px-4 py-3 font-medium">执行载体</th>
              <th className="px-4 py-3 font-medium">所属被测对象</th>
              <th className="px-4 py-3 font-medium">所属场景</th>
              <th className="px-4 py-3 font-medium">上游源</th>
              <th className="px-4 py-3 font-medium">类型</th>
              <th className="px-4 py-3 font-medium">断言强度</th>
              <th className="px-4 py-3 font-medium">变异分数</th>
              <th className="px-4 py-3 font-medium">状态</th>
              <th className="px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 text-xs">
            {filtered.map((c) => {
              const ex = CASE_EXEC[c.id];
              return (
              <tr key={c.id}
                className={c.change === '删除' ? 'bg-red-50/40' : c.status === '冲突' ? 'bg-amber-50/30' : c.status === '待审核' ? 'bg-red-50/30' : selected.has(c.id) ? 'bg-emerald-50/40' : 'hover:bg-slate-50'}>
                <td className="px-4 py-3">
                  <input type="checkbox" checked={selected.has(c.id)} onChange={() => toggleSelect(c.id)} className="accent-emerald-600" />
                </td>
                <td className="px-4 py-3 font-mono text-indigo-600 font-medium">{c.id}</td>
                <td className={'px-4 py-3 text-slate-700 ' + (c.change === '删除' ? 'line-through text-slate-400' : '')}>{c.title}</td>
                <td className="px-4 py-3">
                  <span className={'inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium ' + CHANGE_BADGE[c.change]}>
                    {(() => { const I = CHANGE_ICON[c.change] ?? GitBranch; return <I className="w-3 h-3" />; })()}{c.change}
                  </span>
                </td>
                <td className="px-4 py-3">
                  <button type="button" onClick={() => setDetail({ c, tab: 'version' })}
                    className="group flex items-center gap-1 font-mono text-[11px] text-slate-600 hover:text-emerald-600">
                    <History className="w-3 h-3 text-slate-300 group-hover:text-emerald-500" />v{c.version}
                    <span className="text-[10px] text-slate-300">{c.changedAt}</span>
                  </button>
                </td>
                <td className="px-4 py-3">
                  {ex ? (
                    <button type="button" onClick={() => setDetail({ c, tab: 'exec' })}
                      className="group flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded-md bg-slate-50 text-slate-500 hover:bg-emerald-50 hover:text-emerald-600">
                      {(() => { const I = EXEC_ICON[ex.type] ?? Code2; return <I className="w-3 h-3" />; })()}
                      {ex.type} · {ex.source}
                    </button>
                  ) : <span className="text-[10px] text-slate-300">—</span>}
                </td>
                <td className="px-4 py-3">
                  <span className="bg-slate-50 text-slate-600 px-1.5 py-0.5 rounded text-[10px]">{c.asset}</span>
                </td>
                <td className="px-4 py-3">
                  {(() => {
                    const sc = TEST_SCENARIOS.find((s) => s.id === SCEN_OF[c.type]);
                    if (!sc) return <span className="text-[10px] text-slate-300">—</span>;
                    const Icon = SCEN_ICON[sc.icon] ?? Cpu;
                    return (
                      <button type="button" onClick={() => { scenarioNav.go(sc.id, 'cases'); navigate('/scenarios'); }}
                        className="flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded-full bg-slate-50 text-slate-500 hover:bg-emerald-50 hover:text-emerald-600">
                        <Icon className="w-3 h-3" />{sc.name}
                      </button>
                    );
                  })()}
                </td>
                <td className="px-4 py-3"><span className="text-[10px] text-slate-500">{c.source}</span></td>
                <td className="px-4 py-3"><span className={TYPE_BADGE[c.type] + ' px-2 py-0.5 rounded-full text-[10px]'}>{c.type}</span></td>
                <td className={'px-4 py-3 font-medium ' + ASSERTION_COLOR[c.assertion]}>{c.assertion}</td>
                <td className={c.mutation !== null && c.mutation < 50 ? 'px-4 py-3 text-red-600 font-medium' : 'px-4 py-3 text-slate-600'}>
                  {c.mutation !== null ? c.mutation + '%' : '—'}
                </td>
                <td className="px-4 py-3">
                  <span className={STATUS_BADGE[c.status] + ' px-2 py-0.5 rounded-full text-[10px]'}>{c.status}</span>
                </td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-1.5">
                    {c.change === '删除' ? (
                      <button type="button" onClick={() => restoreCase(c.id)}
                        className="flex items-center gap-1 px-1.5 py-1 rounded-md text-[10px] text-emerald-600 hover:bg-emerald-50">
                        <RefreshCcw className="w-3 h-3" />恢复
                      </button>
                    ) : (
                      <>
                        <button type="button" onClick={() => toggleStatus(c.id)}
                          className={'flex items-center gap-1 px-1.5 py-1 rounded-md text-[10px] ' + (c.status === '已激活' ? 'text-amber-600 hover:bg-amber-50' : 'text-emerald-600 hover:bg-emerald-50')}>
                          <Power className="w-3 h-3" />{c.status === '已激活' ? '禁用' : '启用'}
                        </button>
                        <button type="button" onClick={() => toast('编辑用例', { description: `${c.id} · 打开编辑表单并新生成一个版本（原型 mock）` })}
                          className="flex items-center gap-1 px-1.5 py-1 rounded-md text-[10px] text-slate-500 hover:bg-slate-100">
                          <Pencil className="w-3 h-3" />编辑
                        </button>
                        <button type="button" onClick={() => removeCases([c.id])}
                          className="flex items-center gap-1 px-1.5 py-1 rounded-md text-[10px] text-red-500 hover:bg-red-50">
                          <Trash2 className="w-3 h-3" />删除
                        </button>
                      </>
                    )}
                  </div>
                </td>
              </tr>
              );
            })}
          </tbody>
        </table>
        {filtered.length === 0 && (
          <div className="px-5 py-8 text-center text-xs text-slate-400">无匹配用例 · 调整查询条件或新增用例</div>
        )}
      </div>

      {detail && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-6" onClick={() => setDetail(null)}>
          <div className="bg-white rounded-xl shadow-xl w-full max-w-2xl max-h-[86vh] overflow-auto" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between px-5 py-4 border-b border-slate-100">
              <div>
                <div className="text-xs text-slate-400 font-mono">{detail.c.id} · {detail.c.asset} · {detail.c.source}</div>
                <div className="text-sm font-semibold text-slate-800">{detail.c.title}</div>
              </div>
              <div className="flex items-center gap-2">
                <span className={'inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium ' + CHANGE_BADGE[detail.c.change]}>
                  {(() => { const I = CHANGE_ICON[detail.c.change] ?? GitBranch; return <I className="w-3 h-3" />; })()}{detail.c.change}
                </span>
                <span className="text-[11px] font-mono bg-slate-100 px-2 py-0.5 rounded text-slate-600">当前 v{detail.c.version}</span>
                <button type="button" onClick={() => setDetail(null)} className="flex items-center gap-1 text-xs text-slate-400 hover:text-slate-600"><ArrowLeft className="w-4 h-4" />关闭</button>
              </div>
            </div>

            <div className="flex items-center gap-1 px-5 pt-3">
              <button type="button" onClick={() => setDetail({ c: detail.c, tab: 'exec' })}
                className={'flex items-center gap-1 px-3 py-1.5 rounded-t-md text-xs font-medium border border-b-0 ' + (detail.tab === 'exec' ? 'bg-emerald-50 text-emerald-700 border-emerald-200' : 'bg-slate-50 text-slate-400 border-slate-200 hover:text-slate-600')}>
                <Braces className="w-3.5 h-3.5" />执行载体 · 脚本/数据
              </button>
              <button type="button" onClick={() => setDetail({ c: detail.c, tab: 'version' })}
                className={'flex items-center gap-1 px-3 py-1.5 rounded-t-md text-xs font-medium border border-b-0 ' + (detail.tab === 'version' ? 'bg-emerald-50 text-emerald-700 border-emerald-200' : 'bg-slate-50 text-slate-400 border-slate-200 hover:text-slate-600')}>
                <History className="w-3.5 h-3.5" />版本历史
              </button>
            </div>

            {detail.tab === 'exec' ? (
              <ExecDetail c={detail.c} />
            ) : (
              <VersionTimeline c={detail.c} />
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function ExecDetail({ c }: { c: ICase }) {
  const ex = CASE_EXEC[c.id];
  if (!ex) return <div className="px-5 py-6 text-xs text-slate-400">该用例暂无执行载体绑定（原型 mock）</div>;
  const I = EXEC_ICON[ex.type] ?? Code2;
  return (
    <div className="px-5 py-4">
      <div className="flex items-center gap-2 flex-wrap mb-4">
        <span className={'inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-medium ' + EXEC_BADGE[ex.type]}>
          <I className="w-3.5 h-3.5" />执行载体 · {ex.type}
        </span>
        <span className={'inline-flex px-2 py-0.5 rounded-full text-[11px] ' + SOURCE_BADGE[ex.source]}>{ex.source}</span>
        <span className="text-[11px] text-slate-500 bg-slate-50 px-2 py-0.5 rounded-md">引擎 / Runner：<b className="font-mono">{ex.engine}</b></span>
      </div>

      <div className="space-y-4">
        <div>
          <div className="flex items-center gap-1.5 text-xs font-medium text-slate-500 mb-1.5"><Braces className="w-3.5 h-3.5 text-indigo-500" />执行脚本 / 请求载体</div>
          <div className="bg-slate-900 rounded-lg px-3 py-2.5 font-mono text-[12px] text-emerald-200 overflow-x-auto">{ex.script}</div>
          <div className="text-[10px] text-slate-300 mt-1">脚本 / 请求模板引用，交由 {ex.engine} 拉起执行</div>
        </div>

        <div>
          <div className="flex items-center gap-1.5 text-xs font-medium text-slate-500 mb-1.5"><Boxes className="w-3.5 h-3.5 text-amber-500" />参与执行的参数</div>
          <div className="flex flex-wrap gap-1.5">
            {ex.params.map((p) => <span key={p} className="text-[11px] font-mono bg-amber-50 text-amber-700 px-2 py-0.5 rounded-md border border-amber-200">{p}</span>)}
          </div>
        </div>

        <div>
          <div className="flex items-center gap-1.5 text-xs font-medium text-slate-500 mb-1.5"><FlaskConical className="w-3.5 h-3.5 text-blue-500" />测试数据样本（本次执行入参）</div>
          <div className="flex flex-wrap gap-1.5">
            {ex.data.map((d) => <span key={d} className="text-[11px] bg-blue-50 text-blue-700 px-2 py-0.5 rounded-md border border-blue-200">{d}</span>)}
          </div>
        </div>

        <div>
          <div className="flex items-center gap-1.5 text-xs font-medium text-slate-500 mb-1.5"><ShieldCheck className="w-3.5 h-3.5 text-emerald-600" />断言规则</div>
          <div className="text-[12px] text-slate-600 bg-emerald-50/60 border border-emerald-200 rounded-lg px-3 py-2">{ex.assert}</div>
        </div>
      </div>

      <div className="mt-4 pt-3 border-t border-slate-100 flex items-start gap-2 text-[11px] text-slate-500">
        <Code2 className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
        <span>执行的本质：本用例（{c.type} · {c.asset}）由「{ex.engine}」将用例规格翻译为 <b>{ex.type}</b> 载体并执行；动态性来源为 <b>{ex.source}</b>，随迭代版本与服务的当时版本一起被记录与回溯。</span>
      </div>
    </div>
  );
}

function VersionTimeline({ c }: { c: ICase }) {
  return (
    <div className="px-5 py-4">
      <div className="flex items-center gap-2 mb-3">
        <span className="text-xs font-medium text-slate-500">版本时间线（随迭代发布演进）</span>
        <span className="text-[10px] text-slate-300">被测对象 {c.asset} · 上游 {c.source}</span>
      </div>
      <div className="space-y-2">
        {[...c.versions].reverse().map((v, i) => (
          <div key={v.v} className="flex items-start gap-3">
            <div className="flex flex-col items-center self-stretch">
              <span className={'w-2.5 h-2.5 rounded-full mt-1.5 ' + (v.change === '新增' ? 'bg-emerald-600' : v.change === '更新' ? 'bg-amber-500' : v.change === '删除' ? 'bg-red-600' : 'bg-slate-300')} />
              {i < c.versions.length - 1 && <span className="w-px flex-1 bg-slate-200" />}
            </div>
            <div className="flex-1 pb-3">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="font-mono text-[11px] font-semibold text-slate-700">v{v.v}</span>
                <span className="text-[10px] font-mono text-emerald-600 bg-emerald-50 px-1.5 py-0.5 rounded">{v.iter}</span>
                <span className={'inline-flex px-1.5 py-0.5 rounded-full text-[10px] ' + (v.change === '新增' ? 'bg-emerald-600 text-white' : v.change === '更新' ? 'bg-amber-500 text-white' : v.change === '删除' ? 'bg-red-600 text-white' : 'bg-slate-100 text-slate-500')}>{v.change}</span>
                <span className="text-[10px] text-slate-300">{v.ts}</span>
                {v.v === c.version && <span className="text-[10px] text-emerald-600">← 当前版本</span>}
              </div>
              <div className="mt-1 text-[11px] text-slate-500 bg-slate-50 rounded px-2 py-1">{v.summary}</div>
            </div>
          </div>
        ))}
      </div>
      <div className="mt-4 pt-3 border-t border-slate-100 flex items-start gap-2 text-[11px] text-slate-500">
        <History className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
        <span>版本化语义：本用例相对上一稳定迭代{ c.change === '新增' ? '「新增」' : c.change === '更新' ? '「更新」（断言 / 输入 / 参数变更）' : c.change === '删除' ? '「删除」（功能下线）' : '「稳定」（无变化）' }。随迭代发布可快速辨别新增 / 变化功能对应用例，并回溯每次测试命中的服务版本。</span>
      </div>
    </div>
  );
}
import { useState } from 'react';
import { CASES, TEST_SCENARIOS, type ICase } from '@/data/mock';
import { useNavigate } from 'react-router-dom';
import { scenarioNav } from '@/context/scenarioNav';
import { toast } from 'sonner';
import { Cpu, Globe, Smartphone, Sparkles, ShieldCheck, Pencil, Trash2, Power, Check, Ban } from 'lucide-react';
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

const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles, ShieldCheck };
const SCEN_OF: Record<string, string> = { '单元': 'SCEN-01', '集成': 'SCEN-02', '契约': 'SCEN-03', '安全': 'SCEN-07', 'Web': 'SCEN-08', '移动': 'SCEN-09' };

export default function CasesPage() {
  const navigate = useNavigate();
  // 本地可变的用例列表副本（原型 mock：增删改在本地 state 生效，不接后端）
  const [rows, setRows] = useState<ICase[]>(CASES);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [q, setQ] = useState('');
  const [type, setType] = useState('');
  const [status, setStatus] = useState('');
  const kw = q.trim().toLowerCase();
  const filtered = rows.filter((c) => {
    if (type && c.type !== type) return false;
    if (status && c.status !== status) return false;
    if (kw && !(c.id + c.title + c.asset + c.source).toLowerCase().includes(kw)) return false;
    return true;
  });

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
    toast('用例已删除', { description: `已移除 ${ids.length} 条用例（原型 mock：本地删除生效）` });
  };
  const batchSet = (next: ICase['status']) => {
    const n = selected.size;
    setRows(rows.map((c) => (selected.has(c.id) ? { ...c, status: next } : c)));
    toast(next === '已激活' ? '批量激活' : '批量禁用', { description: `已将 ${n} 条用例设为「${next}」` });
    setSelected(new Set());
  };

  return (
    <div>
      <PageHeader title="用例管理" desc="用例版本化 · 上游源绑定 · 断言强度 · 变异验证 · 批量启停 / 删除">
        <GhostButton>导入用例</GhostButton>
        <PrimaryButton>AI 生成用例</PrimaryButton>
      </PageHeader>

      <div className="flex items-center justify-between mb-3">
        <ListFilter search={q} onSearch={setQ}
          selects={[
            { key: 'type', label: '用例类型', options: ['单元', '集成', 'Web', '移动', '安全'], value: type, onChange: setType },
            { key: 'status', label: '状态', options: ['已激活', '冲突', '待审核', '已禁用'], value: status, onChange: setStatus },
          ]} />
        <span className="text-[11px] text-slate-400">共 {filtered.length} / {rows.length} 条</span>
      </div>

      {/* 批量操作条：勾选后出现 */}
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
            {filtered.map((c) => (
              <tr key={c.id}
                className={c.status === '冲突' ? 'bg-amber-50/30' : c.status === '待审核' ? 'bg-red-50/30' : selected.has(c.id) ? 'bg-emerald-50/40' : 'hover:bg-slate-50'}>
                <td className="px-4 py-3">
                  <input type="checkbox" checked={selected.has(c.id)} onChange={() => toggleSelect(c.id)} className="accent-emerald-600" />
                </td>
                <td className="px-4 py-3 font-mono text-indigo-600 font-medium">{c.id}</td>
                <td className="px-4 py-3 text-slate-700">{c.title}</td>
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
                    <button type="button" onClick={() => toggleStatus(c.id)}
                      className={'flex items-center gap-1 px-1.5 py-1 rounded-md text-[10px] ' + (c.status === '已激活' ? 'text-amber-600 hover:bg-amber-50' : 'text-emerald-600 hover:bg-emerald-50')}>
                      <Power className="w-3 h-3" />{c.status === '已激活' ? '禁用' : '启用'}
                    </button>
                    <button type="button" onClick={() => toast('编辑用例', { description: `${c.id} · 打开编辑表单（原型 mock）` })}
                      className="flex items-center gap-1 px-1.5 py-1 rounded-md text-[10px] text-slate-500 hover:bg-slate-100">
                      <Pencil className="w-3 h-3" />编辑
                    </button>
                    <button type="button" onClick={() => removeCases([c.id])}
                      className="flex items-center gap-1 px-1.5 py-1 rounded-md text-[10px] text-red-500 hover:bg-red-50">
                      <Trash2 className="w-3 h-3" />删除
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {filtered.length === 0 && (
          <div className="px-5 py-8 text-center text-xs text-slate-400">无匹配用例 · 调整查询条件或新增用例</div>
        )}
      </div>
    </div>
  );
}

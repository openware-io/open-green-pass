import { CASES, TEST_SCENARIOS } from '@/data/mock';
import { useNavigate } from 'react-router-dom';
import { scenarioNav } from '@/context/scenarioNav';
import { Cpu, Globe, Smartphone, Sparkles, ShieldCheck } from 'lucide-react';
import { PageHeader, GhostButton, PrimaryButton } from '@/components/shared';

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
};

const SCEN_ICON: Record<string, typeof Cpu> = { Cpu, Globe, Smartphone, Sparkles, ShieldCheck };
const SCEN_OF: Record<string, string> = { '单元': 'SCEN-01', '集成': 'SCEN-02', '契约': 'SCEN-03', '安全': 'SCEN-07', 'Web': 'SCEN-08', '移动': 'SCEN-09' };

export default function CasesPage() {
  const navigate = useNavigate();

  return (
    <div>
      <PageHeader title="测试用例库" desc="用例版本化 · 上游源绑定 · 断言强度 · 变异验证">
        <GhostButton>导入用例</GhostButton>
        <PrimaryButton>AI 生成用例</PrimaryButton>
      </PageHeader>

      <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr className="text-left text-xs text-slate-500">
              <th className="px-4 py-3 font-medium">用例 ID</th>
              <th className="px-4 py-3 font-medium">标题</th>
              <th className="px-4 py-3 font-medium">所属被测对象</th>
              <th className="px-4 py-3 font-medium">所属场景</th>
              <th className="px-4 py-3 font-medium">上游源</th>
              <th className="px-4 py-3 font-medium">类型</th>
              <th className="px-4 py-3 font-medium">断言强度</th>
              <th className="px-4 py-3 font-medium">变异分数</th>
              <th className="px-4 py-3 font-medium">状态</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 text-xs">
            {CASES.map((c) => (
              <tr key={c.id}
                className={c.status === '冲突' ? 'bg-amber-50/30' : c.status === '待审核' ? 'bg-red-50/30' : 'hover:bg-slate-50'}>
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
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

import { useState } from 'react';
import { toast } from 'sonner';
import { TEAMS, TEAM_MEMBERS, ROLE_META, ROLE_MATRIX, CURRENT_TEAM_ID, ASSET_ACCESS, ASSET_PERM_META, type Role, type ITeamMember, type AssetPerm } from '@/data/mock';
import { PageHeader, PrimaryButton } from '@/components/shared';
import { Building2, Users, ShieldCheck, Plus, Check, X, UserPlus, Circle, KeyRound, Minus } from 'lucide-react';
import { cn } from '@/lib/utils';

const TEAM_COLOR: Record<string, { bg: string; text: string }> = {
  emerald: { bg: 'bg-emerald-500', text: 'text-emerald-600' },
  indigo: { bg: 'bg-indigo-500', text: 'text-indigo-600' },
  amber: { bg: 'bg-amber-500', text: 'text-amber-600' },
};

const ROLE_BADGE: Record<Role, string> = {
  owner: 'bg-purple-50 text-purple-600',
  admin: 'bg-indigo-50 text-indigo-600',
  tester: 'bg-emerald-50 text-emerald-600',
  viewer: 'bg-slate-100 text-slate-500',
};

const STATUS_META: Record<ITeamMember['status'], { label: string; cls: string }> = {
  active: { label: '已激活', cls: 'bg-emerald-50 text-emerald-600' },
  invited: { label: '待接受', cls: 'bg-amber-50 text-amber-600' },
  disabled: { label: '已停用', cls: 'bg-slate-100 text-slate-500' },
};

const PERM_BADGE: Record<AssetPerm, string> = {
  full: 'bg-purple-50 text-purple-600',
  edit: 'bg-emerald-50 text-emerald-600',
  exec: 'bg-indigo-50 text-indigo-600',
  view: 'bg-slate-100 text-slate-500',
  none: 'bg-red-50 text-red-600',
};

export default function TeamPage() {
  const [activeTeamId, setActiveTeamId] = useState<string>(CURRENT_TEAM_ID);
  const activeTeam = TEAMS.find((t) => t.id === activeTeamId) ?? TEAMS[0];
  const color = TEAM_COLOR[activeTeam.color];

  const members = TEAM_MEMBERS.filter((m) => {
    // 原型示意：按团队切分，当前团队展示固定成员集合
    return activeTeamId === 'team-1' ? m.role !== 'viewer' || m.id !== 'u-5' : m;
  });

  const handleChangeRole = (m: ITeamMember, role: Role) => {
    toast.success(`已将 ${m.name} 的角色调整为「${ROLE_META.find((r) => r.role === role)?.label}」`, { description: '原型示意：成员角色变更已生效，权限随矩阵实时联动' });
  };

  const handleAddMember = () => {
    toast('邀请成员', { description: '通过邮箱邀请新成员加入当前团队（原型示意）' });
  };

  const [activeAssetId, setActiveAssetId] = useState<string>('svc-payment');
  const activeAccess = ASSET_ACCESS.find((a) => a.assetId === activeAssetId) ?? ASSET_ACCESS[0];
  const owner = TEAM_MEMBERS.find((m) => m.id === activeAccess.ownerId);
  const permLabel = (perm: AssetPerm) => ASSET_PERM_META.find((p) => p.perm === perm)?.label ?? perm;

  const handleChangeAssetPerm = (m: ITeamMember, perm: AssetPerm) => {
    toast.success(`已调整 ${m.name} 对「${activeAccess.assetName}」的权限为「${permLabel(perm)}」`, { description: '原型示意：资产级权限实时生效，负责人可精细分配' });
  };

  return (
    <div>
      <PageHeader title="团队与权限" desc="多租户团队切分 · 成员管理 · 角色权限矩阵 · 数据隔离">
        <PrimaryButton onClick={handleAddMember}><span className="flex items-center gap-1"><UserPlus className="w-4 h-4" />邀请成员</span></PrimaryButton>
      </PageHeader>

      {/* 团队切分：切换当前团队上下文 */}
      <div className="grid grid-cols-3 gap-4 mb-5">
        {TEAMS.map((t) => {
          const tc = TEAM_COLOR[t.color];
          const active = t.id === activeTeamId;
          return (
            <button key={t.id} type="button" onClick={() => setActiveTeamId(t.id)}
              className={cn('card bg-white rounded-xl border p-4 text-left transition',
                active ? 'border-emerald-400 ring-2 ring-emerald-100' : 'border-slate-200 hover:border-emerald-300')}>
              <div className="flex items-center justify-between mb-2">
                <div className={cn('w-8 h-8 rounded-lg flex items-center justify-center text-white text-xs font-bold', tc.bg)}>
                  {t.name.slice(0, 1)}
                </div>
                {active && <span className="text-[10px] bg-emerald-50 text-emerald-600 px-2 py-0.5 rounded-full">当前团队</span>}
              </div>
              <div className="text-sm font-semibold text-slate-800">{t.name}</div>
              <div className="text-[11px] text-slate-500 mt-1">{t.desc}</div>
              <div className="mt-3 flex items-center gap-3 text-[11px] text-slate-400">
                <span className="flex items-center gap-1"><Users className="w-3 h-3" />{t.memberCount} 成员</span>
                <span className="flex items-center gap-1"><Building2 className="w-3 h-3" />{t.projectCount} 工程</span>
                <span className="ml-auto">{t.plan}</span>
              </div>
            </button>
          );
        })}
      </div>

      {/* 当前团队概览 */}
      <div className="card bg-gradient-to-br from-slate-900 to-slate-800 rounded-2xl border border-transparent p-5 mb-5 text-white">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-4">
            <div className={cn('w-11 h-11 rounded-xl flex items-center justify-center text-white text-base font-bold', color.bg)}>
              {activeTeam.name.slice(0, 1)}
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="text-lg font-bold">{activeTeam.name}</span>
                <span className="text-[10px] bg-white/10 px-2 py-0.5 rounded-full">{activeTeam.plan}</span>
              </div>
              <div className="text-[11px] text-slate-300 mt-0.5">{activeTeam.desc} · 创建于 {activeTeam.createdAt}</div>
            </div>
          </div>
          <div className="flex items-center gap-6">
            <div className="text-center">
              <div className="text-2xl font-bold">{members.length}<span className="text-sm text-slate-400"> / {activeTeam.memberCount}</span></div>
              <div className="text-[10px] text-slate-400">成员 / 席位</div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold">{activeTeam.projectCount}</div>
              <div className="text-[10px] text-slate-400">在管工程</div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold text-emerald-400">1.0</div>
              <div className="text-[10px] text-slate-400">治理评分（团队）</div>
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-5">
        {/* 成员管理 */}
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
            <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><Users className="w-4 h-4 text-emerald-500" />团队成员 · {activeTeam.name}</h2>
            <button type="button" onClick={handleAddMember} className="text-[11px] text-emerald-600 flex items-center gap-1"><Plus className="w-3.5 h-3.5" />添加成员</button>
          </div>
          <table className="w-full text-sm">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-xs text-slate-500">
                <th className="px-5 py-2.5 font-medium">成员</th>
                <th className="px-5 py-2.5 font-medium">角色</th>
                <th className="px-5 py-2.5 font-medium">状态</th>
                <th className="px-5 py-2.5 font-medium">最近活跃</th>
                <th className="px-5 py-2.5 font-medium">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-xs">
              {members.map((m) => (
                <tr key={m.id} className="hover:bg-slate-50">
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2.5">
                      <div className="w-7 h-7 rounded-full bg-gradient-to-br from-slate-400 to-slate-600 flex items-center justify-center text-white text-[10px]">{m.name.slice(0, 1)}</div>
                      <div>
                        <div className="text-slate-700 font-medium">{m.name}</div>
                        <div className="text-[10px] text-slate-400">{m.email}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-5 py-3">
                    <select defaultValue={m.role} onChange={(e) => handleChangeRole(m, e.target.value as Role)}
                      className={cn('border border-slate-200 rounded-lg px-2 py-1 text-[11px] outline-none', ROLE_BADGE[m.role])}>
                      {ROLE_META.map((r) => <option key={r.role} value={r.role}>{r.label}</option>)}
                    </select>
                  </td>
                  <td className="px-5 py-3">
                    <span className={cn('px-2 py-0.5 rounded-full text-[10px]', STATUS_META[m.status].cls)}>{STATUS_META[m.status].label}</span>
                  </td>
                  <td className="px-5 py-3 text-slate-500">{m.lastActive}</td>
                  <td className="px-5 py-3">
                    <button type="button" onClick={() => toast('成员操作', { description: `对 ${m.name} 执行操作（原型示意）` })}
                      className="text-[11px] text-slate-400 hover:text-emerald-600">更多</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* 角色定义 + 权限矩阵 */}
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <h2 className="font-semibold text-slate-700 text-sm mb-4 flex items-center gap-1.5"><ShieldCheck className="w-4 h-4 text-emerald-500" />角色权限矩阵</h2>
          <div className="space-y-3 mb-4">
            {ROLE_META.map((r) => (
              <div key={r.role} className="flex items-start gap-2">
                <span className={cn('w-1.5 h-1.5 rounded-full mt-1.5', ROLE_BADGE[r.role])} />
                <div>
                  <div className="text-xs font-medium text-slate-700">{r.label}</div>
                  <div className="text-[10px] text-slate-400">{r.desc}</div>
                </div>
              </div>
            ))}
          </div>
          <div className="bg-slate-50 rounded-lg p-3">
            <div className="flex items-center justify-between text-[10px] font-semibold text-slate-500 px-1 pb-2">
              <span className="flex-1">能力点</span>
              <span className="w-8 text-center">拥有</span>
              <span className="w-8 text-center">管理</span>
              <span className="w-8 text-center">测试</span>
              <span className="w-8 text-center">只读</span>
            </div>
            <div className="space-y-1">
              {ROLE_MATRIX.map((row) => (
                <div key={row.capability} className="flex items-center justify-between text-[10px] text-slate-600 bg-white rounded px-1 py-1.5 border border-slate-100">
                  <span className="flex-1">{row.capability}</span>
                  <span className="w-8 text-center">{row.owner ? <Check className="w-3 h-3 inline text-emerald-500" /> : <X className="w-3 h-3 inline text-slate-300" />}</span>
                  <span className="w-8 text-center">{row.admin ? <Check className="w-3 h-3 inline text-emerald-500" /> : <X className="w-3 h-3 inline text-slate-300" />}</span>
                  <span className="w-8 text-center">{row.tester ? <Check className="w-3 h-3 inline text-emerald-500" /> : <X className="w-3 h-3 inline text-slate-300" />}</span>
                  <span className="w-8 text-center">{row.viewer ? <Check className="w-3 h-3 inline text-emerald-500" /> : <X className="w-3 h-3 inline text-slate-300" />}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* 团队数据隔离说明 */}
      <div className="card bg-white rounded-xl border border-slate-200 p-4 mt-5 flex items-start gap-3">
        <Circle className="w-4 h-4 text-emerald-500 flex-shrink-0 mt-0.5" />
        <div className="text-[11px] text-slate-500 leading-relaxed">
          <span className="text-slate-700 font-medium">数据隔离说明：</span>
          各团队拥有独立的工程、用例、契约、门禁与审计数据空间。切换团队上下文后，测试闭环各页展示的数据均为该团队范围内的结果（原型以 mock 示意）。成员角色决定其在该团队内可执行的权限点。
        </div>
      </div>

      {/* 资产权限分配：工程负责人完整权限，可对成员精细分配权限与资源 */}
      <div className="card bg-white rounded-xl border border-slate-200 overflow-hidden mt-5">
        <div className="px-5 py-4 border-b border-slate-200">
          <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><KeyRound className="w-4 h-4 text-emerald-500" />资产权限分配 · 按资产精细管控</h2>
          <p className="text-[11px] text-slate-500 mt-1">每个资产由「工程负责人」持有完整权限，负责人可对研发组内成员精细分配该资产的权限级别与资源配额。</p>
        </div>

        {/* 资产选择 */}
        <div className="px-5 pt-4 flex flex-wrap gap-2.5">
          {ASSET_ACCESS.map((a) => {
            const o = TEAM_MEMBERS.find((m) => m.id === a.ownerId);
            const active = a.assetId === activeAssetId;
            return (
              <button key={a.assetId} type="button" onClick={() => setActiveAssetId(a.assetId)}
                className={cn('px-3 py-2 rounded-lg border text-xs transition text-left',
                  active ? 'border-emerald-400 bg-emerald-50 text-emerald-700' : 'border-slate-200 text-slate-600 hover:border-emerald-300')}>
                <div className="font-medium">{a.assetName}</div>
                <div className="text-[10px] opacity-70 mt-0.5">{a.type} · 负责人 {o?.name}</div>
              </button>
            );
          })}
        </div>

        {/* 负责人卡 + 成员分配表 */}
        <div className="px-5 py-4">
          <div className="flex flex-wrap items-center justify-between gap-3 mb-4">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 rounded-lg bg-purple-50 flex items-center justify-center text-purple-600 text-xs font-bold">{owner?.name.slice(0, 1)}</div>
              <div>
                <div className="text-sm font-semibold text-slate-800 flex items-center gap-2">
                  {activeAccess.assetName}
                  <span className="text-[9px] text-slate-400 font-normal">{activeAccess.type}</span>
                </div>
                <div className="text-[11px] text-slate-500 mt-0.5">工程负责人：{owner?.name} <span className="text-purple-600 bg-purple-50 px-1.5 py-0.5 rounded-full ml-1">完整权限 · 可再分配</span></div>
              </div>
            </div>
            <span className="text-[11px] text-slate-400">成员 {activeAccess.members.length} · 配额按成员独立生效</span>
          </div>

          <table className="w-full text-sm">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr className="text-left text-xs text-slate-500">
                <th className="px-4 py-2.5 font-medium">成员</th>
                <th className="px-4 py-2.5 font-medium">团队角色</th>
                <th className="px-4 py-2.5 font-medium">资产权限</th>
                <th className="px-4 py-2.5 font-medium">并发配额</th>
                <th className="px-4 py-2.5 font-medium">沙箱配额</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-xs">
              {activeAccess.members.map((alloc) => {
                const m = TEAM_MEMBERS.find((mm) => mm.id === alloc.memberId);
                if (!m) return null;
                const roleMeta = ROLE_META.find((r) => r.role === m.role);
                return (
                  <tr key={m.id} className="hover:bg-slate-50">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2.5">
                        <div className="w-6 h-6 rounded-full bg-gradient-to-br from-slate-400 to-slate-600 flex items-center justify-center text-white text-[9px]">{m.name.slice(0, 1)}</div>
                        <div>
                          <div className="text-slate-700 font-medium">{m.name}</div>
                          <div className="text-[10px] text-slate-400">{m.email}</div>
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn('px-2 py-0.5 rounded-full text-[10px]', ROLE_BADGE[m.role])}>{roleMeta?.label ?? m.role}</span>
                    </td>
                    <td className="px-4 py-3">
                      <select defaultValue={alloc.perm} onChange={(e) => handleChangeAssetPerm(m, e.target.value as AssetPerm)}
                        className={cn('border border-slate-200 rounded-lg px-2 py-1 text-[11px] outline-none', PERM_BADGE[alloc.perm])}>
                        {ASSET_PERM_META.map((p) => <option key={p.perm} value={p.perm}>{p.label}</option>)}
                      </select>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1.5">
                        <button type="button" onClick={() => toast('并发配额', { description: `${m.name} 对「${activeAccess.assetName}」的并发配额为 ${alloc.quota} 条（原型示意）` })}
                          className="w-5 h-5 rounded bg-slate-100 hover:bg-slate-200 flex items-center justify-center text-slate-500"><Minus className="w-3 h-3" /></button>
                        <span className="w-8 text-center text-slate-700">{alloc.quota}</span>
                        <button type="button" onClick={() => toast('并发配额', { description: `${m.name} 对「${activeAccess.assetName}」的并发配额为 ${alloc.quota} 条（原型示意）` })}
                          className="w-5 h-5 rounded bg-slate-100 hover:bg-slate-200 flex items-center justify-center text-slate-500"><Plus className="w-3 h-3" /></button>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1.5">
                        <span className="text-slate-700">{alloc.sandbox} 个沙箱</span>
                        <span className="text-slate-300">·</span>
                        <span className="text-[10px] text-slate-400">K8s</span>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>

          {/* 权限等级图例 */}
          <div className="mt-4 pt-3 border-t border-slate-100 flex flex-wrap gap-3">
            {ASSET_PERM_META.map((p) => (
              <span key={p.perm} className="flex items-center gap-1 text-[10px] text-slate-500">
                <span className={cn('px-1.5 py-0.5 rounded-full text-[9px] font-medium', p.color)}>{p.label}</span>
                <span className="text-slate-400">{p.desc}</span>
              </span>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

import { useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { SidebarProvider, SidebarInset, Sidebar } from '@/components/ui/sidebar';
import { NAV_GROUPS, ASSET_TREE, CASES, REQUIREMENTS, CONTRACTS, TEAMS, CURRENT_TEAM_ID, ACCOUNTS, type IAssetNode } from '@/data/mock';
import { cn } from '@/lib/utils';
import { useCurrentUser, loginStore } from '@/context/login';
const LOGIN_LABEL: Record<string, string> = { password: '账号密码', phone: '手机号', wechat: '微信扫码' };
import { AssetLevelContext, LEVEL_ORDER, LEVEL_LABEL, type AssetLevel } from '@/context';
import { Target, Settings2, Files, Play, ShieldCheck, ScrollText, GitBranch, ChevronDown, Search, Circle, Check, CornerDownLeft, Users, Wallet, UserCog, History, FileText, Landmark, ListChecks, Server, Brain, LayoutGrid, Boxes, UserCircle, LogOut, Smartphone, MessageCircle, Workflow, Zap, Plug } from 'lucide-react';

const ICONS: Record<string, typeof Target> = {
  '/target': Target,
  '/generation': Settings2,
  '/cases': Files,
  '/exec': Play,
  '/gate': ShieldCheck,
  '/history': History,
  '/report': FileText,
  '/scenarios': Boxes,
  '/audit': LayoutGrid,
  '/audit-cost': Wallet,
  '/audit-exec': ListChecks,
  '/audit-op': UserCog,
  '/trace': GitBranch,
  '/resources': Server,
  '/cicd': Workflow,
  '/cicd-trigger': Zap,
  '/cicd-connector': Plug,
  '/teams': Users,
  '/models': Brain,
};
const PARENT_ICONS: Record<string, typeof Target> = {
  '/audit': Landmark,
};

function findChain(node: IAssetNode, id: string): IAssetNode[] | null {
  if (node.id === id) return [node];
  if (!node.children) return null;
  for (const ch of node.children) {
    const r = findChain(ch, id);
    if (r) return [node, ...r];
  }
  return null;
}


const TYPE_LEVEL_COLOR: Record<string, string> = {
  project: 'bg-indigo-500', 'service-group': 'bg-sky-500', service: 'bg-emerald-500',
  module: 'bg-teal-500', app: 'bg-purple-500', end: 'bg-amber-500',
};

// 被测对象维度下拉：点击面包屑中间节点，显示同级被测对象选项，选中联动全局上下文
function CrumbSelect({ label, siblings, currentId, onSelect }: {
  label: string; siblings: IAssetNode[]; currentId: string; onSelect: (n: IAssetNode) => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <span className="relative inline-flex items-center">
      <button type="button" onClick={() => setOpen(!open)}
        className="inline-flex items-center gap-0.5 text-slate-600 hover:text-emerald-600 font-medium">
        {label}<ChevronDown className="w-3 h-3 text-slate-400" />
      </button>
      {open && (
        <div className="absolute top-full left-0 mt-1 z-50 bg-white border border-slate-200 rounded-lg shadow-lg py-1 min-w-[190px] max-h-[300px] overflow-y-auto">
          <div className="px-3 py-1 text-[10px] text-slate-400">同级被测对象 · {siblings.length} 项</div>
          {siblings.map((s) => (
            <button key={s.id} type="button" onClick={() => { onSelect(s); setOpen(false); }}
              className={'block w-full text-left px-3 py-1.5 text-xs ' + (s.id === currentId ? 'bg-emerald-50 text-emerald-600 font-medium' : 'text-slate-600 hover:bg-slate-50')}>
              <span className="inline-flex items-center gap-1.5">
                <span className={'w-1.5 h-1.5 rounded-sm ' + (TYPE_LEVEL_COLOR[s.type] ?? 'bg-slate-300')} />
                {s.name}
              </span>
              <span className="ml-2 text-[9px] text-slate-400">{s.coverage}%</span>
            </button>
          ))}
        </div>
      )}
    </span>
  );
}

export function Layout() {
  const location = useLocation();
  const navigate = useNavigate();
  const [selectedAsset, setSelectedAsset] = useState<IAssetNode>(ASSET_TREE.children![0].children![0].children![0]);
  const [navOpen, setNavOpen] = useState<Record<string, boolean>>({});
  const [userMenu, setUserMenu] = useState(false);
  const user = useCurrentUser();
  const [level, setLevel] = useState<AssetLevel>('service');
  const [query, setQuery] = useState('');

  const setLevelAndAsset = (l: AssetLevel) => {
    setLevel(l);
    if (l === 'system') setSelectedAsset(ASSET_TREE.children![0]);
    else if (l === 'group') setSelectedAsset(ASSET_TREE.children![0].children![0]);
    else if (l === 'service') { if (selectedAsset.type !== 'service') setSelectedAsset(ASSET_TREE.children![0].children![0].children![0]); }
    else if (l === 'module') { if (selectedAsset.type === 'service' && selectedAsset.children) setSelectedAsset(selectedAsset.children[0]); }
  };
  const chain = findChain(ASSET_TREE, selectedAsset.id) ?? [ASSET_TREE];
  const crumbs = chain;
  // 面包屑选中同级被测对象：同步全局上下文 + 层次高亮（与被测对象维度/层次两筛选联动）
  const selectCrumb = (node: IAssetNode) => {
    setSelectedAsset(node);
    const TYPE_LEVEL: Record<string, AssetLevel> = { project: 'system', 'service-group': 'group', service: 'service', module: 'module', app: 'module', end: 'module' };
    setLevel(TYPE_LEVEL[node.type] ?? 'service');
  };


  // 全局搜索（原型 mock）：在用例/需求/契约/服务/页面 多类索引中模糊匹配
  const handleSearch = () => {
    const q = query.trim().toLowerCase();
    if (!q) { toast('请输入关键词（用例 / 需求 / 契约 / 服务）'); return; }
    const match = (arr: { id: string; title?: string; name?: string; provider?: string }[]) =>
      arr.filter((x) => (x.id + ' ' + (x.title ?? x.name ?? x.provider ?? '')).toLowerCase().includes(q));
    const cases = match(CASES);
    const reqs = match(REQUIREMENTS);
    const cons = match(CONTRACTS);
    const pages = NAV_GROUPS.flatMap((g) => g.items).filter((i) => (i.label + i.path).toLowerCase().includes(q));
    const total = cases.length + reqs.length + cons.length;
    if (total === 0 && pages.length === 0) {
      toast.warning('无匹配结果', { description: `未在用例/需求/契约/服务/页面中找到「${query}」` });
    } else {
      const parts = [`用例 ${cases.length}`, `需求 ${reqs.length}`, `契约 ${cons.length}`, `页面 ${pages.length}`].filter((s) => !s.endsWith(' 0'));
      toast.success('搜索命中', { description: `「${query}」 → ${parts.join(' · ')}（原型示意，回车定位到对应视图）` });
    }
    if (pages.length === 1) navigate(pages[0].path);
  };

  return (
    <AssetLevelContext.Provider value={{ level, setLevel: setLevelAndAsset }}>
    <SidebarProvider>
      <Sidebar className="!bg-[#161a23] !border-white/5" collapsible="offcanvas">
        <div className="flex flex-col h-full">
          <div className="h-16 flex items-center px-5 border-b border-white/5">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-emerald-500 to-green-600 flex items-center justify-center text-white shadow-sm">
              <Check className="w-4 h-4" strokeWidth={3} />
            </div>
            <div className="ml-3">
              <div className="text-white text-sm font-semibold leading-tight">GreenPass</div>
              <div className="text-emerald-400/80 text-[11px]">软件工程测试治理平台</div>
            </div>
          </div>

          <nav className="flex-1 py-3 overflow-y-auto">
            {NAV_GROUPS.map((group) => (
              <div key={group.title} className="mb-1">
                <div className="px-4 py-2 text-[10px] font-semibold text-slate-500 uppercase tracking-wider">{group.title}</div>
                {group.items.map((item) => {
                  if (item.children) {
                    const open = navOpen[item.path] ?? true;
                    const groupActive = item.children.some((c) => location.pathname === c.path);
                    const GroupIcon = PARENT_ICONS[item.path] ?? ICONS[item.path] ?? ScrollText;
                    return (
                      <div key={item.path}>
                        <button type="button" onClick={() => setNavOpen((p) => ({ ...p, [item.path]: !(p[item.path] ?? true) }))}
                          className={cn('flex items-center w-full px-4 py-2.5 text-sm cursor-pointer border-l-[3px]',
                            groupActive ? 'bg-emerald-500/15 text-emerald-200 border-emerald-500' : 'text-slate-400 border-transparent hover:bg-white/5')}>
                          <ChevronDown className={cn('w-3.5 h-3.5 mr-2 transition-transform', !open && '-rotate-90')} />
                          <GroupIcon className="w-4 h-4 mr-3" />
                          <span>{item.label}</span>
                        </button>
                        {open && (
                          <div className="ml-4 border-l border-white/5">
                            {item.children.map((sub) => {
                              const SubIcon = ICONS[sub.path] ?? ScrollText;
                              const subActive = location.pathname === sub.path;
                              return (
                                <NavLink key={sub.path} to={sub.path}
                                  className={cn('flex items-center pl-5 pr-4 py-2 text-[13px] border-l-[3px]',
                                    subActive ? 'bg-emerald-500/10 text-emerald-200 border-emerald-500' : 'text-slate-400 border-transparent hover:bg-white/5')}>
                                  <SubIcon className="w-3.5 h-3.5 mr-2.5" />
                                  <span>{sub.label}</span>
                                </NavLink>
                              );
                            })}
                          </div>
                        )}
                      </div>
                    );
                  }
                  const Icon = ICONS[item.path] ?? ScrollText;
                  const active = location.pathname === item.path;
                  return (
                    <NavLink key={item.path} to={item.path}
                      className={cn('flex items-center px-4 py-2.5 text-sm cursor-pointer border-l-[3px]',
                        active ? 'bg-emerald-500/15 text-emerald-200 border-emerald-500' : 'text-slate-400 border-transparent hover:bg-white/5')}>
                      <Icon className="w-4 h-4 mr-3" />
                      <span>{item.label}</span>
                      {item.badge && (
                        <span className={cn('ml-auto text-[10px] px-1.5 py-0.5 rounded',
                          item.badge.includes('告警') ? 'bg-red-500/20 text-red-400' : 'bg-emerald-500/20 text-emerald-400')}>
                          {item.badge}
                        </span>
                      )}
                    </NavLink>
                  );
                })}
              </div>
            ))}


          </nav>

          <div className="p-4 border-t border-white/5">
            <div className="flex items-center">
              <div className="w-8 h-8 rounded-full bg-gradient-to-br from-indigo-400 to-purple-500 flex items-center justify-center text-white text-xs font-bold">AI</div>
              <div className="ml-2.5">
                <div className="text-slate-300 text-xs font-medium">ai-agent-3</div>
                <div className="text-slate-500 text-[10px] flex items-center">
                  <Circle className="w-1.5 h-1.5 fill-emerald-400 text-emerald-400 mr-1" />
                  执行中
                </div>
              </div>
            </div>
          </div>
        </div>
      </Sidebar>

      <SidebarInset className="flex flex-col overflow-hidden !bg-slate-100">
        <header className="h-16 bg-white border-b border-slate-200 flex items-center px-6 flex-shrink-0">
          <div className="flex items-center text-sm text-slate-500 gap-0 flex-wrap">
            {crumbs.map((n, i) => (
              <span key={n.id} className="inline-flex items-center">
                {i > 0 && <span className="mx-2 text-slate-300">/</span>}
                {i > 0 && i < crumbs.length - 1 ? (
                  <CrumbSelect label={n.name} siblings={crumbs[i - 1]?.children ?? []} currentId={n.id} onSelect={selectCrumb} />
                ) : (
                  <span className={i === crumbs.length - 1 ? 'text-emerald-600 font-medium' : 'text-slate-400'}>{n.name}</span>
                )}
              </span>
            ))}
            <span className="mx-2 text-slate-300">/</span>
            <span className="text-slate-700 font-medium">
              {NAV_GROUPS.flatMap((g) => g.items).flatMap((i) => (i.children ? i.children : [i])).find((i) => i.path === location.pathname)?.label ?? ''}
            </span>
          </div>
          <div className="ml-auto flex items-center gap-4">
            <div className="flex items-center bg-slate-100 rounded-lg p-0.5 text-[11px]">
              {LEVEL_ORDER.map((lv) => (
                <button key={lv} type="button" onClick={() => setLevelAndAsset(lv)}
                  className={cn('px-2.5 py-1 rounded', level === lv ? 'bg-emerald-600 text-white' : 'text-slate-500')}>
                  {LEVEL_LABEL[lv]}
                </button>
              ))}
            </div>
            <div className="relative hidden md:block">
              <Search className="absolute left-3 top-1.5 w-4 h-4 text-slate-400" />
              <input
                type="text"
                placeholder="搜索用例、需求、契约…"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') handleSearch(); }}
                className="w-56 bg-slate-100 border border-transparent focus:border-indigo-300 focus:bg-white rounded-lg pl-9 pr-8 py-1.5 text-sm outline-none transition" />
              <button type="button" onClick={handleSearch}
                className="absolute right-2 top-1.5 text-slate-400 hover:text-emerald-600">
                <CornerDownLeft className="w-3.5 h-3.5" />
              </button>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-[11px] bg-emerald-50 text-emerald-600 px-2 py-1 rounded flex items-center gap-1"><Users className="w-3 h-3" />{TEAMS.find((t) => t.id === CURRENT_TEAM_ID)?.name}</span>
              <span className="text-xs font-mono bg-slate-100 px-2 py-1 rounded text-slate-600">CI #4821</span>
              <span className="w-2 h-2 rounded-full bg-emerald-400" />
            </div>
            <div className="h-8 w-px bg-slate-200" />
            <div className="relative">
              <button type="button" onClick={() => setUserMenu(!userMenu)} className="flex items-center gap-2 hover:opacity-90">
                <div className={'w-8 h-8 rounded-full bg-gradient-to-br ' + user.avatarColor + ' flex items-center justify-center text-white text-xs font-bold'}>{user.name.slice(0, 1)}</div>
                <div className="text-left">
                  <div className="text-xs font-medium text-slate-700 leading-tight">{user.name}</div>
                  <div className="text-[9px] text-slate-400 leading-tight">{user.role}</div>
                </div>
              </button>
              {userMenu && (
                <div className="absolute right-0 top-full mt-2 w-72 z-50 bg-white border border-slate-200 rounded-xl shadow-lg overflow-hidden">
                  <div className="px-4 py-3 bg-slate-50 border-b border-slate-100">
                    <div className="flex items-center gap-3">
                      <div className={'w-10 h-10 rounded-full bg-gradient-to-br ' + user.avatarColor + ' flex items-center justify-center text-white text-sm font-bold'}>{user.name.slice(0, 1)}</div>
                      <div>
                        <div className="text-sm font-medium text-slate-800">{user.name} · {user.role}</div>
                        <div className="text-[10px] text-slate-400">{user.email}</div>
                        <div className="text-[9px] text-emerald-600 mt-0.5">最近登录：{LOGIN_LABEL[user.via]}</div>
                      </div>
                    </div>
                  </div>
                  <div className="px-4 py-3 space-y-2.5">
                    <div className="text-[10px] font-semibold text-slate-500 uppercase">账号绑定</div>
                    <div className="flex items-center justify-between text-xs">
                      <span className="flex items-center gap-1.5 text-slate-600"><Smartphone className="w-3.5 h-3.5" />手机号</span>
                      <span>{user.phone ? <span className="text-slate-500">{user.phone}</span> : <button type="button" onClick={() => toast.success('绑定手机号（原型示意）', { description: '将向当前账号发送短信验证并完成绑定' })} className="text-emerald-600 hover:underline">去绑定</button>}</span>
                    </div>
                    <div className="flex items-center justify-between text-xs">
                      <span className="flex items-center gap-1.5 text-slate-600"><MessageCircle className="w-3.5 h-3.5" />微信号</span>
                      <span>{user.wechat ? <span className="text-slate-500">{user.wechat}</span> : <button type="button" onClick={() => toast.success('绑定微信（原型示意）', { description: '将跳转微信授权并关联当前账号' })} className="text-emerald-600 hover:underline">去绑定</button>}</span>
                    </div>
                  </div>
                  <div className="px-4 py-3 border-t border-slate-100 space-y-1">
                    <button type="button" onClick={() => { setUserMenu(false); navigate('/login'); }} className="w-full text-left text-xs text-slate-600 hover:text-emerald-600 py-1.5 flex items-center gap-2"><UserCircle className="w-3.5 h-3.5" />切换账号</button>
                    <button type="button" onClick={() => { loginStore.setCurrent(ACCOUNTS[0]); setUserMenu(false); toast('已退出登录', { description: '请重新登录' }); navigate('/login'); }} className="w-full text-left text-xs text-red-500 hover:text-red-600 py-1.5 flex items-center gap-2"><LogOut className="w-3.5 h-3.5" />退出登录</button>
                  </div>
                </div>
              )}
            </div>
          </div>
        </header>
        <main className="flex-1 overflow-y-auto p-6">
          <Outlet context={{ selectedAsset, setSelectedAsset }} />
        </main>
      </SidebarInset>
    </SidebarProvider>
    </AssetLevelContext.Provider>
  );
}

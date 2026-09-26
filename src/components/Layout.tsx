import { useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { SidebarProvider, SidebarInset, Sidebar } from '@/components/ui/sidebar';
import { NAV_GROUPS, ASSET_TREE, CASES, REQUIREMENTS, CONTRACTS, TEAMS, CURRENT_TEAM_ID, type IAsset, type IAssetNode } from '@/data/mock';
import { cn } from '@/lib/utils';
import { AssetLevelContext, LEVEL_ORDER, LEVEL_LABEL, type AssetLevel } from '@/context';
import { Target, Settings2, Files, ArrowLeftRight, Play, ShieldCheck, ScrollText, GitBranch, Activity, ChevronDown, Search, Circle, Check, CornerDownLeft, Users, Cpu, Wallet, UserCog, History } from 'lucide-react';

const ICONS: Record<string, typeof Target> = {
  '/target': Target,
  '/generation': Settings2,
  '/cases': Files,
  '/exec': Play,
  '/contracts': ArrowLeftRight,
  '/gate': ShieldCheck,
  '/history': History,
  '/audit': ScrollText,
  '/audit-cost': Wallet,
  '/audit-exec': ScrollText,
  '/audit-op': UserCog,
  '/trace': GitBranch,
  '/concurrency': Activity,
  '/teams': Users,
  '/models': Cpu,
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

function AssetGroup({ node, depth, selectedId, onSelect, expanded, onExpand }: {
  node: IAssetNode; depth: number; selectedId: string;
  onSelect: (a: IAsset) => void; expanded: Record<string, boolean>;
  onExpand: (id: string, defaultOpen: boolean) => void;
}) {
  const isOpen = expanded[node.id] ?? (node.type === 'service' ? false : true);
  const icon = node.type === 'app' ? '▢' : node.type === 'end' ? '▢' : node.type === 'service' ? '●' : '◈';
  const iconColor =
    node.type === 'service' ? 'text-emerald-400'
    : node.type === 'app' ? 'text-blue-400'
    : node.type === 'end' ? 'text-purple-400'
    : 'text-amber-400';

  if (node.children) {
    return (
      <div>
        <button
          type="button"
          onClick={() => { if (node.type === 'service') onSelect(node); onExpand(node.id, node.type === 'service' ? false : true); }}
          className="flex w-full items-center px-2 py-1.5 rounded text-slate-300 hover:bg-white/5 text-left"
        >
          <ChevronDown className={cn('w-3 h-3 text-slate-500 mr-1 transition-transform', !isOpen && '-rotate-90')} />
          <span className={cn(iconColor, 'mr-2 text-xs')}>{icon}</span>
          <span className={cn('text-xs', depth === 0 ? 'font-medium' : '')}>{node.name}</span>
          {node.type === 'service-group' && <span className="ml-1 text-[9px] text-slate-600">{node.children.length}</span>}
        </button>
        {isOpen && (
          <div className={cn('ml-3', depth >= 1 && 'ml-4')}>
            {node.children.map((child) => (
              <AssetGroup key={child.id} node={child} depth={depth + 1} selectedId={selectedId}
                onSelect={onSelect} expanded={expanded} onExpand={onExpand} />
            ))}
          </div>
        )}
      </div>
    );
  }

  return (
    <button
      type="button"
      onClick={() => onSelect(node)}
      className={cn('flex w-full items-center px-2 py-1.5 rounded text-slate-300 hover:bg-white/5 text-left',
        selectedId === node.id && 'bg-emerald-500/20 text-emerald-200')}
    >
      <span className="w-3 mr-1" />
      <span className={cn(iconColor, 'mr-2 text-xs')}>{icon}</span>
      <span className="text-xs">{node.name}</span>
      <span className={cn('ml-auto text-[9px]', node.coverage >= 85 ? 'text-emerald-400' : 'text-amber-400')}>{node.coverage}%</span>
    </button>
  );
}

export function Layout() {
  const location = useLocation();
  const navigate = useNavigate();
  const [selectedAsset, setSelectedAsset] = useState<IAssetNode>(ASSET_TREE.children![0].children![0]);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [navOpen, setNavOpen] = useState<Record<string, boolean>>({});
  const [level, setLevel] = useState<AssetLevel>('service');
  const [query, setQuery] = useState('');

  const setLevelAndAsset = (l: AssetLevel) => {
    setLevel(l);
    if (l === 'system') setSelectedAsset(ASSET_TREE);
    else if (l === 'group') setSelectedAsset(ASSET_TREE.children![0]);
    else if (l === 'service') { if (selectedAsset.type !== 'service') setSelectedAsset(ASSET_TREE.children![0].children![0]); }
    else if (l === 'module') { if (selectedAsset.type === 'service' && selectedAsset.children) setSelectedAsset(selectedAsset.children[0]); }
  };
  const chain = findChain(ASSET_TREE, selectedAsset.id) ?? [ASSET_TREE];
  const crumbs = chain.slice(0, Math.min(LEVEL_ORDER.indexOf(level) + 1, chain.length));


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
                    const GroupIcon = ICONS[item.path] ?? ScrollText;
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
                  const Icon = ICONS[item.path];
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

            <div className="px-4 py-2 mt-3 text-[10px] font-semibold text-slate-500 uppercase tracking-wider flex items-center justify-between">
              <span>资产树</span>
              <span className="text-slate-600 text-[9px]">v2.1</span>
            </div>
            <div className="px-2 text-sm">
              <AssetGroup node={ASSET_TREE} depth={0} selectedId={selectedAsset.id}
                onSelect={setSelectedAsset} expanded={expanded} onExpand={(id, def) => setExpanded((p) => ({ ...p, [id]: !(p[id] ?? def) }))} />
            </div>
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
          <div className="text-sm text-slate-500">
            {crumbs.map((n, i) => (
              <span key={n.id} className="inline-flex items-center">
                {i > 0 && <span className="mx-2 text-slate-300">/</span>}
                <span className={i === crumbs.length - 1 ? 'text-emerald-600 font-medium' : 'text-slate-400'}>{n.name}</span>
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

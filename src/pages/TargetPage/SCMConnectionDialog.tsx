import { useEffect, useState } from 'react';
import { AlertTriangle, CheckCircle2, ExternalLink, LockKeyhole, RefreshCw, RotateCw, ShieldCheck, Trash2, X } from 'lucide-react';
import type { GreenPassClient } from '@/api/client';

export type SCMConnection = {
  id: number;
  provider: 'github';
  name: string;
  credential_status: 'configured' | 'verified' | 'failed';
  failure_summary?: string;
  verified_at?: string;
  token_configured: boolean;
  auth_type?: 'pat' | 'github_app';
  github_app_id?: string;
  github_installation_id?: number | null;
  installation_account_login?: string;
};

type Props = {
  open: boolean;
  client: GreenPassClient;
  connections: SCMConnection[];
  onClose: () => void;
  onRefreshConnections: () => Promise<void> | void;
  onNotify: (type: 'success' | 'error', message: string) => void;
};

const statusMeta: Record<SCMConnection['credential_status'], { label: string; cls: string }> = {
  verified: { label: '已验证', cls: 'bg-emerald-50 text-emerald-700' },
  configured: { label: '待验证', cls: 'bg-amber-50 text-amber-700' },
  failed: { label: '验证失败', cls: 'bg-red-50 text-red-700' },
};

export default function SCMConnectionDialog({ open, client, connections, onClose, onRefreshConnections, onNotify }: Props) {
  const [githubApp, setGitHubApp] = useState<{ configured: boolean; app_id?: string } | null>(null);
  const [name, setName] = useState('GitHub 只读连接');
  const [token, setToken] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [rotateID, setRotateID] = useState<number | null>(null);
  const [rotateToken, setRotateToken] = useState('');

  useEffect(() => {
    if (!open) return;
    void client.githubAppConfig().then(setGitHubApp).catch(() => setGitHubApp({ configured: false }));
    void Promise.resolve(onRefreshConnections()).catch(() => undefined);
  }, [open, client, onRefreshConnections]);

  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    setError('');
    try {
      await action();
    } catch (err) {
      setError(err instanceof Error ? err.message : '真实 API 操作失败');
    } finally {
      setBusy(false);
    }
  };

  const create = () => void run(async () => {
    if (!name.trim() || !token.trim()) throw new Error('请填写连接名称与 fine-grained PAT');
    await client.createSCMConnection({ provider: 'github', name: name.trim(), token: token.trim() });
    setToken('');
    await onRefreshConnections();
    onNotify('success', `源码读取连接「${name.trim()}」已验证并安全保存`);
  });

  const verify = (item: SCMConnection) => void run(async () => {
    await client.verifySCMConnection(item.id);
    await onRefreshConnections();
    onNotify('success', `${item.name} 已重新验证`);
  });

  const remove = (item: SCMConnection) => {
    if (!window.confirm(`删除连接「${item.name}」并解除所有仓库绑定？该操作不可撤销。`)) return;
    void run(async () => {
      await client.deleteSCMConnection(item.id);
      await onRefreshConnections();
      onNotify('success', `${item.name} 已删除`);
    });
  };

  const rotate = (item: SCMConnection) => void run(async () => {
    if (!rotateToken.trim()) throw new Error('请粘贴新的 fine-grained PAT');
    await client.rotateSCMConnection(item.id, rotateToken.trim());
    setRotateID(null);
    setRotateToken('');
    await onRefreshConnections();
    onNotify('success', `${item.name} 的凭据已轮换并重新验证`);
  });

  const authorize = () => void run(async () => {
    const result = await client.beginGitHubAppAuthorization();
    window.location.assign(result.authorization_url);
  });

  if (!open) return null;

  const patForm = (
    <div className="rounded-xl border border-indigo-200 bg-indigo-50/40 p-4">
      <div className="flex items-center gap-2 text-sm font-semibold text-indigo-900"><LockKeyhole className="h-4 w-4" />使用只读 fine-grained PAT</div>
      <p className="mt-1 text-[11px] leading-5 text-indigo-800">
        仅勾选需要测试的仓库，授予 <code>Contents: Read-only</code> 和 <code>Metadata: Read-only</code>；不要使用带 <code>repo</code> / <code>admin:org</code> 等写权限的 classic Token。
        <a href="https://github.com/settings/personal-access-tokens/new" target="_blank" rel="noreferrer" className="ml-1 inline-flex items-center gap-0.5 font-medium underline">去 GitHub 创建 <ExternalLink className="h-3 w-3" /></a>
      </p>
      <div className="mt-3 grid gap-3 sm:grid-cols-[1fr_1.2fr]">
        <label>
          <span className="mb-1 block text-xs font-medium text-slate-600">连接名称</span>
          <input value={name} onChange={(event) => setName(event.target.value)} className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm outline-none focus:border-indigo-400" />
        </label>
        <label>
          <span className="mb-1 block text-xs font-medium text-slate-600">GitHub fine-grained PAT</span>
          <input type="password" autoComplete="new-password" value={token} onChange={(event) => setToken(event.target.value)} className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm outline-none focus:border-indigo-400" />
        </label>
      </div>
      <div className="mt-3 flex justify-end">
        <button type="button" disabled={busy || !name.trim() || !token.trim()} onClick={create} className="rounded-lg bg-indigo-600 px-4 py-2 text-xs font-semibold text-white hover:bg-indigo-700 disabled:opacity-50">
          {busy ? '验证并保存中…' : '验证并保存连接'}
        </button>
      </div>
    </div>
  );

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/45 p-4" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onClose(); }}>
      <div className="flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-200 px-6 py-4">
          <div>
            <h3 className="text-base font-semibold text-slate-800">GitHub 源码读取连接</h3>
            <p className="mt-1 text-[11px] text-slate-500">凭据只保存在集群 Secret；分支刷新、技术画像和源码冻结共用此连接。</p>
          </div>
          <button type="button" disabled={busy} onClick={() => { setError(''); onClose(); }} className="rounded-lg p-2 text-slate-400 hover:bg-slate-100"><X className="h-4 w-4" /></button>
        </div>

        <div className="min-h-0 flex-1 space-y-4 overflow-auto p-6">
          {error && <div role="alert" className="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs leading-5 text-red-700"><AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />{error}</div>}

          {githubApp?.configured ? (
            <div className="rounded-xl border border-emerald-200 bg-emerald-50 p-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p className="flex items-center gap-1.5 text-sm font-semibold text-emerald-800"><ShieldCheck className="h-4 w-4" />GitHub App 授权</p>
                  <p className="mt-1 text-xs leading-5 text-emerald-700">前往 GitHub 选择组织和仓库范围。平台不接收个人 Token，读取源码时仅申请短时安装令牌。</p>
                  <p className="mt-1 text-[11px] text-emerald-700">跳转地址绑定当前团队与操作者，10 分钟内仅可回调一次。</p>
                </div>
                <button type="button" disabled={busy} onClick={authorize} className="rounded-lg bg-emerald-600 px-4 py-2 text-xs font-semibold text-white hover:bg-emerald-700 disabled:opacity-50">{busy ? '准备授权…' : '前往 GitHub 授权'}</button>
              </div>
            </div>
          ) : (
            <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-xs leading-5 text-amber-800">
              <b>当前环境未配置 GitHub App。</b>
              <br />GitHub App 需要 GitHub 可访问的公网 HTTPS 回调；本地 kind 只有 <code>http://greenpass.localhost:30080</code>，因此请使用下面的只读 PAT。平台管理员配置 App ID、私钥 Secret 引用、安装地址与回调地址后，这里会自动切换为 App 授权。
            </div>
          )}

          {githubApp?.configured ? (
            <details className="rounded-lg border border-slate-200 bg-slate-50 p-4">
              <summary className="cursor-pointer text-xs font-medium text-slate-700">兼容方式：fine-grained PAT</summary>
              <div className="mt-4">{patForm}</div>
            </details>
          ) : patForm}

          <div className="border-t border-slate-100 pt-4">
            <div className="flex items-center justify-between">
              <h4 className="text-xs font-semibold text-slate-700">已配置连接</h4>
              <button type="button" disabled={busy} onClick={() => void run(async () => { await onRefreshConnections(); })} className="inline-flex items-center gap-1 text-[11px] text-slate-500 hover:text-slate-700"><RefreshCw className="h-3 w-3" />刷新</button>
            </div>
            <div className="mt-2 space-y-2">
              {connections.map((item) => {
                const meta = statusMeta[item.credential_status];
                return (
                  <div key={item.id} className="rounded-lg border border-slate-200 px-3 py-2.5">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="min-w-0">
                        <span className="text-xs font-medium text-slate-700">{item.name}</span>
                        <span className="ml-2 text-[10px] text-slate-400">GitHub · {item.auth_type === 'github_app' ? `App 授权${item.installation_account_login ? ` · ${item.installation_account_login}` : ''}` : 'PAT 已安全保存'}</span>
                        {item.failure_summary && <p className="mt-1 text-[11px] text-red-600">{item.failure_summary}</p>}
                      </div>
                      <div className="flex items-center gap-2">
                        <span className={`rounded-full px-2 py-1 text-[10px] ${meta.cls}`}>{meta.label}</span>
                        <button type="button" disabled={busy} onClick={() => verify(item)} className="text-xs text-indigo-700 hover:underline">重新验证</button>
                        {item.auth_type !== 'github_app' && <button type="button" disabled={busy} onClick={() => { setRotateID(item.id); setRotateToken(''); }} className="inline-flex items-center gap-0.5 text-xs text-slate-600 hover:underline"><RotateCw className="h-3 w-3" />轮换</button>}
                        <button type="button" disabled={busy} onClick={() => remove(item)} className="inline-flex items-center gap-0.5 text-xs text-red-600 hover:underline"><Trash2 className="h-3 w-3" />删除</button>
                      </div>
                    </div>
                    {rotateID === item.id && (
                      <div className="mt-3 flex flex-col gap-2 rounded-lg bg-slate-50 p-3 sm:flex-row sm:items-center">
                        <input type="password" autoComplete="new-password" value={rotateToken} onChange={(event) => setRotateToken(event.target.value)} placeholder="粘贴新的 fine-grained PAT" className="min-w-0 flex-1 rounded-lg border border-slate-200 px-3 py-2 text-xs outline-none focus:border-indigo-400" />
                        <div className="flex gap-2">
                          <button type="button" disabled={busy || !rotateToken.trim()} onClick={() => rotate(item)} className="rounded-lg bg-slate-800 px-3 py-2 text-xs font-medium text-white disabled:opacity-50">保存轮换</button>
                          <button type="button" disabled={busy} onClick={() => { setRotateID(null); setRotateToken(''); }} className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-600">取消</button>
                        </div>
                      </div>
                    )}
                  </div>
                );
              })}
              {!connections.length && <p className="py-4 text-center text-xs text-slate-400">尚未配置源码读取连接。</p>}
            </div>
          </div>

          <p className="flex items-start gap-1.5 text-[11px] leading-5 text-slate-400"><CheckCircle2 className="mt-0.5 h-3.5 w-3.5 shrink-0" />连接创建、验证、绑定、轮换和删除均会进入可信审计；Token 不会出现在列表、审计载荷或日志中。</p>
        </div>
      </div>
    </div>
  );
}

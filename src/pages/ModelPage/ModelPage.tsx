import { useCallback, useEffect, useMemo, useState } from "react";
import { Cpu, KeyRound, Pencil, ShieldCheck, X } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/shared";
import { configuredGreenPassClient } from "@/api/runtime";

type Provider = { Provider: string; Name: string; Protocol: string; BaseURL: string };
type Model = { id: number; provider: string; model_key: string; status: string };
type Connection = { id: number; provider: string; model_key: string; protocol: "api" | "cc"; base_url: string; credential_status: string; configured_at?: string; token_configured: boolean };
type DiscoveredModel = { id: string; owned_by?: string };

export default function ModelPage() {
  const c = useMemo(() => configuredGreenPassClient(), []),
    [providers, setProviders] = useState<Provider[]>([]),
    [models, setModels] = useState<Model[]>([]),
    [connections, setConnections] = useState<Connection[]>([]),
    [open, setOpen] = useState(false),
    [selectedConnection, setSelectedConnection] = useState<Connection | null>(null),
    [provider, setProvider] = useState("openai"),
    [protocol, setProtocol] = useState("api"),
    [token, setToken] = useState(""),
    [url, setURL] = useState(""),
    [keys, setKeys] = useState<string[]>([]),
    [customModel, setCustomModel] = useState(""),
    [discovered, setDiscovered] = useState<DiscoveredModel[]>([]),
    [discovering, setDiscovering] = useState(false),
    [discoverError, setDiscoverError] = useState(""),
    [editingGroup, setEditingGroup] = useState<{ provider: string; protocol: string; baseURL: string } | null>(null),
    [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    if (!c) return;
    const [catalog, detail] = await Promise.all([c.modelCatalog(), c.modelConnections()]);
    setProviders(catalog.providers);
    setModels(catalog.models);
    setConnections(detail.connections);
  }, [c]);

  useEffect(() => {
    if (!c) return;
    void Promise.all([c.modelCatalog(), c.modelConnections()]).then(([catalog, detail]) => {
      setProviders(catalog.providers);
      setModels(catalog.models);
      setConnections(detail.connections);
    }).catch(() => setConnections([]));
  }, [c]);

  const apiProviders = [...new Set(providers.filter((item) => item.Protocol === "api").map((item) => item.Provider))];
  const providerLabel = (value: string) => value === "cc-proxy" ? "CC 代理" : (providers.find((item) => item.Provider === value)?.Name ?? value);
  const connectionFor = (providerKey: string, modelKey: string) => connections.find((item) => item.provider === providerKey && item.model_key === modelKey);
  const connectionGroups = useMemo(() => {
    const groups = new Map<string, { provider: string; protocol: string; baseURL: string; models: Connection[] }>();
    for (const connection of connections) {
      const key = `${connection.provider}|${connection.protocol}|${connection.base_url}`;
      const current = groups.get(key);
      if (current) current.models.push(connection);
      else groups.set(key, { provider: connection.provider, protocol: connection.protocol, baseURL: connection.base_url, models: [connection] });
    }
    return [...groups.values()];
  }, [connections]);
  const providerRows = useMemo(() => {
    const rows: Provider[] = [...providers];
    for (const group of connectionGroups) {
      if (!rows.some((item) => item.Provider === group.provider && item.Protocol === group.protocol)) {
        rows.push({ Provider: group.provider, Name: group.provider, Protocol: group.protocol, BaseURL: group.baseURL });
      }
    }
    return rows;
  }, [providers, connectionGroups]);
  const modelOptions = useMemo(() => {
    const options = [...discovered];
    for (const key of keys) if (!options.some((item) => item.id === key)) options.push({ id: key });
    return options;
  }, [discovered, keys]);
  const effectiveKeys = protocol === "cc" && customModel.trim() ? [customModel.trim()] : keys;
  const formatConfiguredAt = (value?: string) => value ? new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)) : "—";
  const isValidBaseURL = (value: string) => {
    try {
      const parsed = new URL(value.trim());
      return (parsed.protocol === "http:" || parsed.protocol === "https:") && Boolean(parsed.host);
    } catch {
      return false;
    }
  };

  const discoverModels = async (
    providerArg = provider,
    protocolArg = protocol,
    tokenArg = token,
    urlArg = url,
  ) => {
    if (!c) return;
    setDiscovering(true);
    setDiscoverError("");
    try {
      const result = await c.discoverProviderModels({
        provider: protocolArg === "cc" ? "cc-proxy" : providerArg,
        protocol: protocolArg,
        token: tokenArg,
        base_url: urlArg,
      });
      setDiscovered(result.models);
      setKeys((current) => current.filter((key) => result.models.some((model) => model.id === key)));
      if (!result.models.length) setDiscoverError(protocolArg === "cc" ? "CC 代理不提供模型列表，请直接填写模型名。" : "供应商返回的模型列表为空，请检查 Base URL 与 Token。");
    } catch (e) {
      const message = e instanceof Error ? e.message : "获取模型列表失败";
      setDiscoverError(message === "provider_connection_failed" ? "无法读取供应商模型列表，请检查 Base URL、Token 与网络。" : message === "token_required" ? "请先填写 Token。" : message);
      setDiscovered([]);
    } finally {
      setDiscovering(false);
    }
  };

  const openConnectionForm = (connection?: Connection) => {
    if (!connection) {
      const first = providers.find((item) => item.Protocol === "api") ?? providers[0];
      setEditingGroup(null);
      setProvider(first?.Provider ?? "openai");
      setProtocol("api");
      setURL(first?.BaseURL ?? "");
      setKeys([]);
      setCustomModel("");
      setDiscovered([]);
      setDiscoverError("");
      setToken("");
      setOpen(true);
      return;
    }
    const groupKeys = connections
      .filter((item) => item.provider === connection.provider && item.protocol === connection.protocol)
      .map((item) => item.model_key);
    setEditingGroup({ provider: connection.provider, protocol: connection.protocol, baseURL: connection.base_url });
    setProtocol(connection.protocol);
    setProvider(connection.protocol === "cc" ? "cc-proxy" : connection.provider);
    setURL(connection.base_url);
    setKeys(connection.protocol === "api" ? groupKeys : []);
    setCustomModel(connection.protocol === "cc" ? connection.model_key : "");
    setDiscovered([]);
    setDiscoverError("");
    setToken("");
    setSelectedConnection(null);
    setOpen(true);
    if (connection.protocol === "api") void discoverModels(connection.provider, connection.protocol, "", connection.base_url);
  };

  const openProviderForm = (item: Provider) => {
    setEditingGroup(null);
    setProvider(item.Provider);
    setProtocol(item.Protocol);
    setURL(item.BaseURL);
    setKeys([]);
    setCustomModel("");
    setDiscovered([]);
    setDiscoverError("");
    setToken("");
    setOpen(true);
  };

  const save = async () => {
    if (!c) return;
    if (!editingGroup && !token.trim()) {
      toast.error("请填写供应商 Token / API Key");
      return;
    }
    if (protocol === "api" && !effectiveKeys.length) {
      toast.error("请先获取官方模型列表并选择至少一个模型");
      return;
    }
    if (protocol === "cc" && !customModel.trim()) {
      toast.error("请填写 CC 代理模型名");
      return;
    }
    if (url.trim() && !isValidBaseURL(url)) {
      toast.error("Base URL 格式不正确", { description: "请输入 http:// 或 https:// 开头的代理地址，例如 http://127.0.0.1:8080/v1" });
      return;
    }
    setBusy(true);
    try {
      await c.configureModels({
        provider: protocol === "cc" ? "cc-proxy" : provider,
        protocol,
        token: token.trim(),
        base_url: url,
        model_keys: effectiveKeys,
      });
      toast.success(editingGroup ? "供应商模型已更新" : "供应商连接已验证并保存");
      setToken("");
      setEditingGroup(null);
      setOpen(false);
      await load();
    } catch (e) {
      const message = e instanceof Error ? e.message : "保存失败";
      const friendly = message === "provider_connection_failed"
        ? "模型服务连接失败"
        : message === "invalid_model_selection"
          ? "模型不在供应商官方列表中"
          : message === "invalid_base_url"
            ? "Base URL 格式不正确"
            : message === "token_required"
              ? "该供应商还没有保存 Token，请填写一次"
              : message;
      toast.error(friendly, { description: friendly === "模型服务连接失败" ? "请检查 Base URL、Token，以及代理是否已启动并兼容对应接口。" : undefined });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <PageHeader
        title="AI 模型配置"
        desc="供应商一次接入 · 模型从官方 /models 动态发现 · Token 安全托管"
      >
        <button onClick={() => openConnectionForm()} className="rounded-lg bg-emerald-600 px-4 py-2 text-xs text-white">
          配置供应商连接
        </button>
      </PageHeader>

      <section className="overflow-hidden rounded-xl border bg-white">
        <div className="flex items-center justify-between border-b px-5 py-4">
          <div>
            <h2 className="text-sm font-semibold text-slate-900">供应商</h2>
            <p className="mt-1 text-xs text-slate-500">一个供应商配置一次 Token；模型从供应商官方 <span className="font-mono">/models</span> 动态发现。</p>
          </div>
          <span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-600">{connectionGroups.length} 个已连接 · {models.filter((model) => model.status === "approved").length} 个模型</span>
        </div>
        <table className="w-full text-xs">
          <thead className="bg-slate-50 text-left text-[10px] text-slate-400">
            <tr>
              <th className="px-5 py-2.5 font-medium">供应商</th>
              <th className="py-2.5 font-medium">状态</th>
              <th className="py-2.5 font-medium">Base URL</th>
              <th className="py-2.5 font-medium">模型</th>
              <th className="py-2.5 pr-5 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {providerRows.map((item) => {
              const group = connectionGroups.find((candidate) => candidate.provider === item.Provider && candidate.protocol === item.Protocol);
              const connected = Boolean(group);
              const providerModels = models.filter((model) => model.provider === item.Provider && model.status === "approved");
              return <tr className="border-t" key={`${item.Provider}-${item.Protocol}`}>
                <td className="px-5 py-3">
                  <div className="flex items-center gap-2">
                    <div className="grid h-8 w-8 place-items-center rounded-lg bg-slate-50"><Cpu className="h-4 w-4 text-emerald-500" /></div>
                    <div>
                      <div className="font-medium text-slate-900">{providerLabel(item.Provider)}</div>
                      <div className="text-[10px] text-slate-400">{item.Protocol === "cc" ? "CC / Anthropic Messages" : "API / OpenAI-compatible"}</div>
                    </div>
                  </div>
                </td>
                <td className="py-3">
                  {connected ? <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] text-emerald-700"><ShieldCheck className="h-3 w-3" />已连接</span> : <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[10px] text-slate-500">未配置</span>}
                  <div className="mt-1 flex items-center gap-1 text-[10px] text-slate-400"><KeyRound className="h-3 w-3" />{connected ? "Token 已保存" : "尚未保存 Token"}</div>
                </td>
                <td className="max-w-[260px] py-3">
                  <div className="truncate font-mono text-[11px] text-slate-600">{group?.baseURL || item.BaseURL || "—"}</div>
                </td>
                <td className="py-3">
                  <div className="flex flex-wrap gap-1.5">
                    {providerModels.length ? providerModels.map((model) => {
                      const connection = connectionFor(item.Provider, model.model_key);
                      return <button key={model.id} type="button" onClick={() => connection && setSelectedConnection(connection)} className="rounded-full bg-emerald-50 px-2 py-0.5 font-mono text-[10px] text-emerald-700 hover:bg-emerald-100">{model.model_key}</button>;
                    }) : <span className="text-[11px] text-slate-400">未配置模型</span>}
                  </div>
                </td>
                <td className="py-3 pr-5 text-right">
                  {connected && group ? <button onClick={() => openConnectionForm(group.models[0])} className="inline-flex items-center gap-1 rounded-lg border border-emerald-200 px-3 py-1.5 text-xs text-emerald-700 hover:bg-emerald-50"><Pencil className="h-3.5 w-3.5" />管理模型</button> : <button onClick={() => openProviderForm(item)} className="rounded-lg border border-slate-200 px-3 py-1.5 text-xs text-slate-700 hover:bg-slate-50">配置</button>}
                </td>
              </tr>;
            })}
          </tbody>
        </table>
      </section>

      {selectedConnection && <div className="fixed inset-0 z-50 flex justify-end bg-slate-900/45" onMouseDown={() => setSelectedConnection(null)}>
        <aside className="h-full w-full max-w-md bg-white shadow-2xl" onMouseDown={(event) => event.stopPropagation()}>
          <div className="flex items-center justify-between border-b px-6 py-5"><div><div className="text-sm font-semibold">连接配置</div><div className="mt-1 text-xs text-slate-500">仅展示非敏感连接信息</div></div><button onClick={() => setSelectedConnection(null)} aria-label="关闭"><X className="h-4 w-4" /></button></div>
          <dl className="divide-y px-6"><div className="py-4"><dt className="text-xs text-slate-500">厂商</dt><dd className="mt-1 text-sm font-medium">{providerLabel(selectedConnection.provider)}</dd></div><div className="py-4"><dt className="text-xs text-slate-500">协议</dt><dd className="mt-1 text-sm">{selectedConnection.protocol === "cc" ? "CC / Anthropic Messages" : "API / OpenAI-compatible"}</dd></div><div className="py-4"><dt className="text-xs text-slate-500">模型 ID</dt><dd className="mt-1 font-mono text-sm">{selectedConnection.model_key}</dd></div><div className="py-4"><dt className="text-xs text-slate-500">Base URL</dt><dd className="mt-1 break-all font-mono text-sm">{selectedConnection.base_url}</dd></div><div className="py-4"><dt className="text-xs text-slate-500">Token / API Key</dt><dd className="mt-1 text-sm text-emerald-700">已安全保存，不可回显</dd></div><div className="py-4"><dt className="text-xs text-slate-500">最后配置时间</dt><dd className="mt-1 text-sm">{formatConfiguredAt(selectedConnection.configured_at)}</dd></div></dl>
          <div className="border-t px-6 py-4"><button onClick={() => openConnectionForm(selectedConnection)} className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-sm text-white"><Pencil className="h-4 w-4" />管理模型</button></div>
        </aside>
      </div>}

      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/45">
          <div className="w-full max-w-2xl rounded-2xl bg-white">
            <div className="flex justify-between border-b p-5">
              <b>{editingGroup ? "管理供应商模型" : "配置供应商连接"}</b>
              <button onClick={() => setOpen(false)} aria-label="关闭">
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="space-y-4 p-5">
              <div className="rounded-lg border border-emerald-100 bg-emerald-50/60 px-3 py-2 text-[11px] leading-5 text-emerald-800">
                先配置供应商 Token；模型列表从供应商官方 `/models` 读取，不再内置固定版本。编辑已有连接时可留空 Token，沿用已保存凭据。
              </div>
              <div className="grid grid-cols-2 gap-3">
                {protocol === "cc" ? (
                  <div className="rounded-lg border bg-slate-50 px-3 py-2 text-sm text-slate-700">
                    <div className="text-[10px] text-slate-400">连接类型</div>
                    <div className="font-medium">自定义 CC 代理</div>
                  </div>
                ) : (
                  <select
                    value={provider}
                    disabled={Boolean(editingGroup)}
                    onChange={(e) => {
                      setProvider(e.target.value);
                      setKeys([]);
                      setDiscovered([]);
                      setDiscoverError("");
                      setURL(providers.find((item) => item.Provider === e.target.value && item.Protocol === protocol)?.BaseURL ?? "");
                    }}
                    className="rounded-lg border p-2 disabled:bg-slate-100 disabled:text-slate-500"
                  >
                    {apiProviders.map((value) => <option key={value}>{value}</option>)}
                  </select>
                )}
                <select
                  value={protocol}
                  disabled={Boolean(editingGroup)}
                  onChange={(e) => {
                    setProtocol(e.target.value);
                    setKeys([]);
                    setDiscovered([]);
                    setDiscoverError("");
                    if (e.target.value === "cc") {
                      setProvider("cc-proxy");
                      setURL("");
                    } else {
                      const first = providers.find((item) => item.Protocol === "api");
                      setProvider(first?.Provider ?? "openai");
                      setURL(first?.BaseURL ?? "");
                    }
                  }}
                  className="rounded-lg border p-2 disabled:bg-slate-100 disabled:text-slate-500"
                >
                  <option value="api">API / OpenAI-compatible</option>
                  <option value="cc">CC / Anthropic Messages</option>
                </select>
              </div>

              {protocol === "cc" ? (
                <div className="space-y-3 rounded-lg border border-amber-200 bg-amber-50 p-3">
                  <div className="text-[11px] leading-5 text-amber-800">
                    代理需要兼容 Anthropic Messages API。GreenPass 会调用 <code>POST {"{Base URL}"}/messages</code>，代理后端可以实际转发到指定模型。
                  </div>
                  <label className="block text-xs font-medium text-slate-700">CC 代理 Base URL <span className="text-rose-500">*</span>
                    <input value={url} onChange={(e) => setURL(e.target.value)} placeholder="例如 http://127.0.0.1:8080/v1" className="mt-1 w-full rounded-lg border bg-white p-2" />
                  </label>
                  <label className="block text-xs font-medium text-slate-700">CC 代理模型名 <span className="text-rose-500">*</span>
                    <input value={customModel} onChange={(e) => setCustomModel(e.target.value)} placeholder="例如 claude-sonnet-4-5" className="mt-1 w-full rounded-lg border bg-white p-2" />
                  </label>
                </div>
              ) : (
                <label className="block text-xs font-medium text-slate-700">API Base URL <span className="font-normal text-slate-400">可选，留空使用厂商默认地址</span>
                  <input value={url} onChange={(e) => setURL(e.target.value)} placeholder="例如 https://api.deepseek.com/v1" className="mt-1 w-full rounded-lg border p-2" />
                </label>
              )}

              <label className="block text-xs font-medium text-slate-700">Token / API Key {editingGroup ? <span className="font-normal text-slate-400">留空沿用已保存的 Token</span> : <span className="text-rose-500">*</span>}
                <input type="password" value={token} onChange={(e) => setToken(e.target.value)} placeholder={editingGroup ? "留空则沿用该供应商已保存的 Token" : "请输入供应商 Token"} className="mt-1 w-full rounded-lg border p-2" />
              </label>

              {protocol === "api" && <div>
                <div className="mb-2 flex items-center justify-between">
                  <div className="text-xs font-medium text-slate-700">供应商官方模型</div>
                  <button
                    type="button"
                    disabled={discovering || (!editingGroup && !token.trim())}
                    onClick={() => void discoverModels()}
                    className="rounded-md border border-emerald-200 px-2.5 py-1 text-[11px] text-emerald-700 hover:bg-emerald-50 disabled:opacity-50"
                  >
                    {discovering ? "读取中…" : "获取官方模型列表"}
                  </button>
                </div>
                {discoverError && <div className="mb-2 rounded-md border border-amber-200 bg-amber-50 px-2.5 py-1.5 text-[11px] text-amber-800">{discoverError}</div>}
                {modelOptions.length ? <div className="grid grid-cols-2 gap-2">
                  {modelOptions.map((model) => {
                    const configured = Boolean(connectionFor(provider, model.id));
                    return <label className={`rounded-lg border p-3 ${configured ? "border-emerald-200 bg-emerald-50/40" : ""}`} key={model.id}>
                      <input
                        type="checkbox"
                        className="mr-2 accent-emerald-600"
                        checked={keys.includes(model.id)}
                        disabled={Boolean(editingGroup && configured)}
                        onChange={(e) => setKeys((current) => e.target.checked ? [...current, model.id] : current.filter((key) => key !== model.id))}
                      />
                      <span className="font-mono text-xs">{model.id}</span>
                      {configured && <span className="ml-1 text-[10px] text-emerald-700">已启用</span>}
                    </label>;
                  })}
                </div> : <div className="rounded-lg border border-dashed border-slate-200 px-3 py-4 text-center text-[11px] text-slate-500">{discovering ? "正在读取供应商模型列表…" : "填写 Token 后点击“获取官方模型列表”，再从返回结果中选择模型。"}</div>}
              </div>}
            </div>
            <div className="flex justify-end gap-2 border-t p-4">
              <button onClick={() => setOpen(false)}>取消</button>
              <button
                disabled={busy || !effectiveKeys.length || (!editingGroup && !token.trim()) || (protocol === "cc" && (!url.trim() || !customModel.trim()))}
                onClick={() => void save()}
                className="rounded-lg bg-emerald-600 px-4 py-2 text-white disabled:opacity-50"
              >
                {busy ? "保存中…" : editingGroup ? "保存模型" : "保存连接"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

import { useCallback, useEffect, useMemo, useState } from "react";
import { Cpu, Eye, KeyRound, Pencil, ShieldCheck, X } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/shared";
import { configuredGreenPassClient } from "@/api/runtime";
type P = {
  Provider: string;
  Name: string;
  ModelKey: string;
  Protocol: string;
  BaseURL: string;
  Capabilities: string[];
};
type M = { id: number; provider: string; model_key: string; status: string };
type Connection = { id: number; provider: string; model_key: string; protocol: "api" | "cc"; base_url: string; credential_status: string; configured_at?: string; token_configured: boolean };
export default function ModelPage() {
  const c = useMemo(() => configuredGreenPassClient(), []),
    [ps, setPs] = useState<P[]>([]),
    [ms, setMs] = useState<M[]>([]),
    [connections, setConnections] = useState<Connection[]>([]),
    [open, setOpen] = useState(false),
    [selectedConnection, setSelectedConnection] = useState<Connection | null>(null),
    [provider, setProvider] = useState("openai"),
    [protocol, setProtocol] = useState("api"),
    [token, setToken] = useState(""),
    [url, setURL] = useState(""),
    [keys, setKeys] = useState<string[]>([]),
    [customModel, setCustomModel] = useState(""),
    [editingGroup, setEditingGroup] = useState<{ provider: string; protocol: string; baseURL: string } | null>(null),
    [enablingModel, setEnablingModel] = useState(""),
    [busy, setBusy] = useState(false);
  const load = useCallback(async () => {
    if (c) {
      const [d, detail] = await Promise.all([c.modelCatalog(), c.modelConnections()]);
      setPs(d.presets);
      setMs(d.models);
      setConnections(detail.connections);
    }
  }, [c]);
  useEffect(() => {
    if (!c) return;
    void Promise.all([c.modelCatalog(), c.modelConnections()]).then(([data, detail]) => {
      setPs(data.presets);
      setMs(data.models);
      setConnections(detail.connections);
    }).catch(() => setConnections([]));
  }, [c]);
  const apiProviders = [...new Set(ps.filter((p) => p.Protocol === "api").map((p) => p.Provider))],
    providers = [...new Set([...ps.map((p) => p.Provider), ...ms.map((m) => m.provider)])],
    available = ps.filter(
      (p) => p.Provider === provider && p.Protocol === protocol,
    );
  const effectiveKeys = protocol === "cc" && customModel.trim() ? [customModel.trim()] : keys;
  const providerLabel = (value: string) => value === "cc-proxy" ? "CC 代理" : value;
  const connectionFor = (provider: string, modelKey: string) => connections.find((item) => item.provider === provider && item.model_key === modelKey);
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
  const formatConfiguredAt = (value?: string) => value ? new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)) : "—";
  const isValidBaseURL = (value: string) => {
    try {
      const parsed = new URL(value.trim());
      return (parsed.protocol === "http:" || parsed.protocol === "https:") && Boolean(parsed.host);
    } catch {
      return false;
    }
  };
  const openConnectionForm = (connection?: Connection) => {
    if (!connection) {
      setEditingGroup(null);
      setProvider(apiProviders[0] ?? "openai");
      setProtocol("api");
      setURL("");
      setKeys([]);
      setCustomModel("");
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
    setToken("");
    setSelectedConnection(null);
    setOpen(true);
  };
  const openNewForProvider = (preset: P) => {
    setEditingGroup(null);
    setProvider(preset.Provider);
    setProtocol(preset.Protocol);
    setURL(preset.BaseURL);
    setKeys([preset.ModelKey]);
    setCustomModel(preset.Protocol === "cc" ? preset.ModelKey : "");
    setToken("");
    setOpen(true);
  };
  const enableModel = async (preset: P) => {
    if (!c) return;
    const existing = connections.find((item) => item.provider === preset.Provider && item.protocol === preset.Protocol);
    if (!existing) { openNewForProvider(preset); return; }
    setEnablingModel(preset.ModelKey);
    try {
      await c.configureModels({
        provider: preset.Provider,
        protocol: preset.Protocol,
        token: "",
        base_url: existing.base_url,
        model_keys: [preset.ModelKey],
      });
      toast.success(`${preset.Name} 已启用`, { description: "沿用该供应商已保存的 Token，无需重新输入。" });
      await load();
    } catch (e) {
      const message = e instanceof Error ? e.message : "启用失败";
      toast.error(message === "token_required" ? "该供应商还没有保存 Token" : message);
    } finally {
      setEnablingModel("");
    }
  };
  const save = async () => {
    if (!c) return;
    if (!editingGroup && !token.trim()) {
      toast.error("请填写供应商 Token / API Key");
      return;
    }
    if (!effectiveKeys.length) {
      toast.error("请选择至少一个模型");
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
          ? "模型配置不完整"
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
        desc="主流模型目录 · API / Claude Code 兼容规范 · Token 安全托管"
      >
        <button
          onClick={() => openConnectionForm()}
          className="rounded-lg bg-emerald-600 px-4 py-2 text-xs text-white"
        >
          配置供应商连接
        </button>
      </PageHeader>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3 mb-5">
        {providers.map((v) => (
          <div className="rounded-xl border bg-white p-5" key={v}>
            <div className="flex gap-2 font-semibold">
              <Cpu className="h-4 w-4 text-emerald-500" />
              {providerLabel(v)}
            </div>
            <div className="mt-2 text-xs text-slate-400">
              {ps.filter((p) => p.Provider === v).length} 个版本 ·{" "}
              {ms.filter((m) => m.provider === v).length} 个已配置
            </div>
          </div>
        ))}
      </div>
      <section className="mb-5 overflow-hidden rounded-xl border bg-white">
        <div className="flex items-center justify-between border-b px-5 py-4">
          <div>
            <h2 className="text-sm font-semibold text-slate-900">已配置供应商</h2>
            <p className="mt-1 text-xs text-slate-500">一个供应商配置一次 Token，可承载多个模型；Token 仅显示保存状态，不可读取或回显。</p>
          </div>
          <span className="rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700">{connectionGroups.length} 个供应商</span>
        </div>
        {connectionGroups.length ? <div className="divide-y">
          {connectionGroups.map((group) => <div className="flex flex-wrap items-center gap-x-5 gap-y-2 px-5 py-4" key={`${group.provider}-${group.protocol}-${group.baseURL}`}>
            <div className="min-w-48 flex-1">
              <div className="font-medium text-slate-900">{providerLabel(group.provider)} <span className="ml-1 text-xs font-normal text-slate-400">{group.models.length} 个模型</span></div>
              <div className="mt-1 text-xs text-slate-500">{group.protocol === "cc" ? "CC / Anthropic Messages" : "API / OpenAI-compatible"} · <span className="font-mono">{group.baseURL || "厂商默认地址"}</span></div>
              <div className="mt-2 flex flex-wrap gap-1.5">{group.models.map((model) => <span key={model.id} className="rounded-full bg-emerald-50 px-2 py-0.5 font-mono text-[10px] text-emerald-700">{model.model_key}</span>)}</div>
            </div>
            <div className="flex items-center gap-1.5 text-xs text-emerald-700"><ShieldCheck className="h-4 w-4" />已验证</div>
            <div className="flex items-center gap-1.5 text-xs text-slate-500"><KeyRound className="h-3.5 w-3.5" />Token 已保存</div>
            <div className="ml-auto flex gap-2">
              <button onClick={() => setSelectedConnection(group.models[0])} className="inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs text-slate-700 hover:bg-slate-50"><Eye className="h-3.5 w-3.5" />查看配置</button>
              <button onClick={() => openConnectionForm(group.models[0])} className="inline-flex items-center gap-1 rounded-lg border border-emerald-200 px-3 py-1.5 text-xs text-emerald-700 hover:bg-emerald-50"><Pencil className="h-3.5 w-3.5" />管理模型</button>
            </div>
          </div>)}
        </div> : <div className="px-5 py-8 text-center text-sm text-slate-500">暂无已验证供应商。配置成功后会在这里显示。</div>}
      </section>
      <div className="overflow-hidden rounded-xl border bg-white">
        <table className="w-full text-xs">
          <thead className="bg-slate-50 text-left">
            <tr>
              <th className="p-4">厂商</th>
              <th>模型版本</th>
              <th>规范</th>
              <th>能力</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            {ps.map((p) => (
              <tr className="border-t" key={p.ModelKey}>
                <td className="p-4">{providerLabel(p.Provider)}</td>
                <td>
                  {p.Name}
                  <div className="font-mono text-[10px] text-slate-400">
                    {p.ModelKey}
                  </div>
                </td>
                <td>{p.Protocol === "cc" ? "CC / Anthropic Messages" : "API / OpenAI-compatible"}</td>
                <td>{p.Capabilities.join(" · ")}</td>
                <td>{(() => {
                  const connection = connectionFor(p.Provider, p.ModelKey);
                  if (connection) return <button onClick={() => setSelectedConnection(connection)} className="inline-flex items-center gap-1 text-emerald-600 hover:underline"><ShieldCheck className="h-4 w-4" />已配置</button>;
                  const providerConnection = connections.find((item) => item.provider === p.Provider && item.protocol === p.Protocol);
                  if (providerConnection) return <button disabled={enablingModel === p.ModelKey} onClick={() => void enableModel(p)} className="inline-flex items-center gap-1 rounded-md border border-emerald-200 px-2 py-1 text-emerald-700 hover:bg-emerald-50 disabled:opacity-50">{enablingModel === p.ModelKey ? "启用中…" : "启用"}</button>;
                  return <button onClick={() => openNewForProvider(p)} className="inline-flex items-center gap-1 rounded-md border border-slate-200 px-2 py-1 text-slate-600 hover:bg-slate-50">配置</button>;
                })()}</td>
              </tr>
            ))}
            {ms.filter((m) => m.provider === "cc-proxy" && !ps.some((p) => p.Provider === m.provider && p.ModelKey === m.model_key)).map((m) => (
              <tr className="border-t bg-emerald-50/30" key={`configured-${m.id}`}>
                <td className="p-4">CC 代理</td>
                <td>
                  {m.model_key}
                  <div className="font-mono text-[10px] text-slate-400">{m.model_key}</div>
                </td>
                <td>CC / Anthropic Messages</td>
                <td>code</td>
                <td>{(() => { const connection = connectionFor(m.provider, m.model_key); return connection ? <button onClick={() => setSelectedConnection(connection)} className="inline-flex items-center gap-1 text-emerald-600 hover:underline"><ShieldCheck className="h-4 w-4" />已配置</button> : "—"; })()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {selectedConnection && <div className="fixed inset-0 z-50 flex justify-end bg-slate-900/45" onMouseDown={() => setSelectedConnection(null)}>
        <aside className="h-full w-full max-w-md bg-white shadow-2xl" onMouseDown={(event) => event.stopPropagation()}>
          <div className="flex items-center justify-between border-b px-6 py-5"><div><div className="text-sm font-semibold">连接配置</div><div className="mt-1 text-xs text-slate-500">仅展示非敏感连接信息</div></div><button onClick={() => setSelectedConnection(null)} aria-label="关闭"><X className="h-4 w-4" /></button></div>
          <dl className="divide-y px-6"><div className="py-4"><dt className="text-xs text-slate-500">厂商</dt><dd className="mt-1 text-sm font-medium">{providerLabel(selectedConnection.provider)}</dd></div><div className="py-4"><dt className="text-xs text-slate-500">协议</dt><dd className="mt-1 text-sm">{selectedConnection.protocol === "cc" ? "CC / Anthropic Messages" : "API / OpenAI-compatible"}</dd></div><div className="py-4"><dt className="text-xs text-slate-500">模型名</dt><dd className="mt-1 font-mono text-sm">{selectedConnection.model_key}</dd></div><div className="py-4"><dt className="text-xs text-slate-500">Base URL</dt><dd className="mt-1 break-all font-mono text-sm">{selectedConnection.base_url}</dd></div><div className="py-4"><dt className="text-xs text-slate-500">Token / API Key</dt><dd className="mt-1 text-sm text-emerald-700">已安全保存，不可回显</dd></div><div className="py-4"><dt className="text-xs text-slate-500">最后配置时间</dt><dd className="mt-1 text-sm">{formatConfiguredAt(selectedConnection.configured_at)}</dd></div></dl>
          <div className="border-t px-6 py-4"><button onClick={() => openConnectionForm(selectedConnection)} className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-sm text-white"><Pencil className="h-4 w-4" />管理模型</button></div>
        </aside>
      </div>}
      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/45">
          <div className="w-full max-w-2xl rounded-2xl bg-white">
            <div className="flex justify-between border-b p-5">
              <b>{editingGroup ? "管理供应商模型" : "配置供应商连接"}</b>
              <button onClick={() => setOpen(false)}>
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="space-y-4 p-5">
              <div className="rounded-lg border border-emerald-100 bg-emerald-50/60 px-3 py-2 text-[11px] leading-5 text-emerald-800">
                一个供应商只需配置一次 Token；在下方勾选要启用的模型。编辑已有连接时可留空 Token，沿用已保存凭据。
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
                    }}
                    className="rounded-lg border p-2 disabled:bg-slate-100 disabled:text-slate-500"
                  >
                    {apiProviders.map((v) => (
                      <option key={v}>{v}</option>
                    ))}
                  </select>
                )}
                <select
                  value={protocol}
                  disabled={Boolean(editingGroup)}
                  onChange={(e) => {
                    setProtocol(e.target.value);
                    if (e.target.value === "cc") {
                      setProvider("cc-proxy");
                      setKeys([]);
                    } else {
                      setProvider(apiProviders[0] ?? "openai");
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
                    代理需要兼容 Anthropic Messages API。GreenPass 会调用 <code>POST {"{Base URL}"}/messages</code>，代理后端可以实际转发到 GPT。
                  </div>
                  <label className="block text-xs font-medium text-slate-700">CC 代理 Base URL <span className="text-rose-500">*</span>
                    <input
                      value={url}
                      onChange={(e) => setURL(e.target.value)}
                      placeholder="例如 http://127.0.0.1:8080/v1"
                      className="mt-1 w-full rounded-lg border bg-white p-2"
                    />
                  </label>
                  <label className="block text-xs font-medium text-slate-700">CC 代理模型名 <span className="text-rose-500">*</span>
                    <input
                      value={customModel}
                      onChange={(e) => { setCustomModel(e.target.value); setKeys([]); }}
                      placeholder="例如 gpt-5 或 gpt-5-codex"
                      className="mt-1 w-full rounded-lg border bg-white p-2"
                    />
                  </label>
                </div>
              ) : (
                <label className="block text-xs font-medium text-slate-700">API Base URL <span className="font-normal text-slate-400">可选</span>
                  <input
                    value={url}
                    onChange={(e) => setURL(e.target.value)}
                    placeholder="留空使用厂商地址，例如 https://api.openai.com/v1"
                    className="mt-1 w-full rounded-lg border p-2"
                  />
                </label>
              )}
              <label className="block text-xs font-medium text-slate-700">Token / API Key {editingGroup ? <span className="font-normal text-slate-400">留空沿用已保存的 Token</span> : <span className="text-rose-500">*</span>}
                <input
                  type="password"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  placeholder={editingGroup ? "留空则沿用该供应商已保存的 Token" : "请输入供应商 Token"}
                  className="mt-1 w-full rounded-lg border p-2"
                />
              </label>
              {protocol === "api" && <div>
                <div className="mb-2 text-xs font-medium text-slate-700">该供应商下可用的模型</div>
                <div className="grid grid-cols-2 gap-2">
                  {available.map((p) => {
                    const configured = Boolean(connectionFor(p.Provider, p.ModelKey));
                    return <label className={`rounded-lg border p-3 ${configured ? "border-emerald-200 bg-emerald-50/40" : ""}`} key={p.ModelKey}>
                      <input
                        type="checkbox"
                        className="mr-2 accent-emerald-600"
                        checked={keys.includes(p.ModelKey)}
                        disabled={Boolean(editingGroup && configured)}
                        onChange={(e) =>
                          setKeys((v) =>
                            e.target.checked
                              ? [...v, p.ModelKey]
                              : v.filter((x) => x !== p.ModelKey),
                          )
                        }
                      />
                      {p.Name}
                      {configured && <span className="ml-1 text-[10px] text-emerald-700">已启用</span>}
                    </label>;
                  })}
                </div>
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

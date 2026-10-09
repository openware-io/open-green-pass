import { useCallback, useEffect, useMemo, useState } from "react";
import { Cpu, ShieldCheck, X } from "lucide-react";
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
export default function ModelPage() {
  const c = useMemo(() => configuredGreenPassClient(), []),
    [ps, setPs] = useState<P[]>([]),
    [ms, setMs] = useState<M[]>([]),
    [open, setOpen] = useState(false),
    [provider, setProvider] = useState("openai"),
    [protocol, setProtocol] = useState("api"),
    [token, setToken] = useState(""),
    [url, setURL] = useState(""),
    [keys, setKeys] = useState<string[]>([]),
    [busy, setBusy] = useState(false);
  const load = useCallback(async () => {
    if (c) {
      const d = await c.modelCatalog();
      setPs(d.presets);
      setMs(d.models);
    }
  }, [c]);
  useEffect(() => {
    if (!c) return;
    void c.modelCatalog().then((data) => {
      setPs(data.presets);
      setMs(data.models);
    });
  }, [c]);
  const providers = [...new Set(ps.map((p) => p.Provider))],
    available = ps.filter(
      (p) => p.Provider === provider && p.Protocol === protocol,
    );
  const save = async () => {
    if (!c) return;
    setBusy(true);
    try {
      await c.configureModels({
        provider,
        protocol,
        token,
        base_url: url,
        model_keys: keys,
      });
      toast.success("模型连接已验证并保存");
      setToken("");
      setOpen(false);
      await load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "保存失败");
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
          onClick={() => setOpen(true)}
          className="rounded-lg bg-emerald-600 px-4 py-2 text-xs text-white"
        >
          配置模型连接
        </button>
      </PageHeader>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3 mb-5">
        {providers.map((v) => (
          <div className="rounded-xl border bg-white p-5" key={v}>
            <div className="flex gap-2 font-semibold">
              <Cpu className="h-4 w-4 text-emerald-500" />
              {v}
            </div>
            <div className="mt-2 text-xs text-slate-400">
              {ps.filter((p) => p.Provider === v).length} 个版本 ·{" "}
              {ms.filter((m) => m.provider === v).length} 个已配置
            </div>
          </div>
        ))}
      </div>
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
                <td className="p-4">{p.Provider}</td>
                <td>
                  {p.Name}
                  <div className="font-mono text-[10px] text-slate-400">
                    {p.ModelKey}
                  </div>
                </td>
                <td>
                  {p.Protocol === "cc"
                    ? "CC / Anthropic"
                    : "API / OpenAI-compatible"}
                </td>
                <td>{p.Capabilities.join(" · ")}</td>
                <td>
                  {ms.some(
                    (m) =>
                      m.provider === p.Provider && m.model_key === p.ModelKey,
                  ) ? (
                    <span className="flex gap-1 text-emerald-600">
                      <ShieldCheck className="h-4 w-4" />
                      已配置
                    </span>
                  ) : (
                    "未配置"
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/45">
          <div className="w-full max-w-2xl rounded-2xl bg-white">
            <div className="flex justify-between border-b p-5">
              <b>配置模型连接</b>
              <button onClick={() => setOpen(false)}>
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="space-y-4 p-5">
              <div className="grid grid-cols-2 gap-3">
                <select
                  value={provider}
                  onChange={(e) => {
                    setProvider(e.target.value);
                    setKeys([]);
                  }}
                  className="rounded-lg border p-2"
                >
                  {providers.map((v) => (
                    <option key={v}>{v}</option>
                  ))}
                </select>
                <select
                  value={protocol}
                  onChange={(e) => {
                    setProtocol(e.target.value);
                    setKeys([]);
                  }}
                  className="rounded-lg border p-2"
                >
                  <option value="api">API / OpenAI-compatible</option>
                  <option value="cc">CC / Anthropic</option>
                </select>
              </div>
              <input
                value={url}
                onChange={(e) => setURL(e.target.value)}
                placeholder="自定义 Base URL（可选）"
                className="w-full rounded-lg border p-2"
              />
              <input
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="Token / API Key"
                className="w-full rounded-lg border p-2"
              />
              <div className="grid grid-cols-2 gap-2">
                {available.map((p) => (
                  <label className="rounded-lg border p-3" key={p.ModelKey}>
                    <input
                      type="checkbox"
                      className="mr-2"
                      onChange={(e) =>
                        setKeys((v) =>
                          e.target.checked
                            ? [...v, p.ModelKey]
                            : v.filter((x) => x !== p.ModelKey),
                        )
                      }
                    />
                    {p.Name}
                  </label>
                ))}
              </div>
            </div>
            <div className="flex justify-end gap-2 border-t p-4">
              <button onClick={() => setOpen(false)}>取消</button>
              <button
                disabled={busy || !token || !keys.length}
                onClick={() => void save()}
                className="rounded-lg bg-emerald-600 px-4 py-2 text-white disabled:opacity-50"
              >
                保存连接
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

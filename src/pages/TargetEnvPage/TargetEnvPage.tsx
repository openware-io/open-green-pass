import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { toast } from 'sonner';
import { ArrowLeft, CheckCircle2, Database, KeyRound, LoaderCircle, Plus, RefreshCw, Save, Server, ShieldCheck, Trash2 } from 'lucide-react';
import { PageHeader, Card } from '@/components/shared';
import { configuredGreenPassClient } from '@/api/runtime';
import type { EnvRuntime, Target } from '@/api/client';

type Requirement = {
  key: string;
  label: string;
  required: boolean;
  reason: string;
  evidence?: string;
  sensitive?: boolean;
};

const DEFAULT_TARGET = '2876690473259008';
const DEFAULT_REQUIREMENTS: Requirement[] = [
  { key: 'admin_username', label: '管理员账号', required: true, reason: '管理后台登录', evidence: 'kind: open-im-local' },
  { key: 'admin_password', label: '管理员密码', required: true, reason: '只登记密钥引用，不写明文', sensitive: true },
  { key: 'tenant_id', label: '租户 ID', required: false, reason: '测试数据隔离', evidence: 'im-saas' },
  { key: 'jwt_secret', label: 'JWT 签名密钥', required: false, reason: '生成短期测试令牌', sensitive: true },
];

const inputClass = 'w-full rounded-lg border border-slate-200 bg-white px-3 py-2.5 text-xs text-slate-700 outline-none focus:border-emerald-400';
const labelClass = 'mb-1 block text-[11px] font-medium text-slate-500';

export default function TargetEnvPage() {
  const client = configuredGreenPassClient();
  const [targets, setTargets] = useState<Target[]>([]);
  const [targetID, setTargetID] = useState(DEFAULT_TARGET);
  const [env, setEnv] = useState('test');
  const [version, setVersion] = useState('2.2.0-SNAPSHOT');
  const [baseURL, setBaseURL] = useState('http://gateway.open-im-local.svc.cluster.local:3002');
  const [credentialRef, setCredentialRef] = useState('k8s://open-im-local/open-im-env');
  const [testDataProfile, setTestDataProfile] = useState('im-saas-users');
  const [browserProfile, setBrowserProfile] = useState('');
  const [requirements, setRequirements] = useState<Requirement[]>(DEFAULT_REQUIREMENTS);
  const [values, setValues] = useState<Record<string, string>>({ tenant_id: '1', admin_username: 'admin', test_user_prefix: 'gp-' });
  const [confirmed, setConfirmed] = useState(true);
  const [runtime, setRuntime] = useState<EnvRuntime | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');

  const selectedTarget = useMemo(() => targets.find((item) => String(item.id) === targetID), [targets, targetID]);

  const loadTargets = useCallback(async () => {
    if (!client) return;
    try {
      const rows = await client.listTargets();
      setTargets(rows);
      if (!rows.some((item) => String(item.id) === targetID) && rows[0]) setTargetID(String(rows[0].id));
    } catch (error: unknown) {
      setMessage(error instanceof Error ? error.message : '读取被测对象失败');
    }
  }, [client, targetID]);

  const loadRuntime = useCallback(async () => {
    if (!client || !targetID || !env) return;
    try {
      const row = await client.getRuntime(Number(targetID), env);
      setRuntime(row);
      setVersion(row.running_version || version);
      setBaseURL(row.base_url || baseURL);
      setCredentialRef(row.confirmation?.credential_ref || credentialRef);
      setTestDataProfile(row.confirmation?.test_data_profile || testDataProfile);
      setBrowserProfile(row.confirmation?.browser_profile || '');
      setValues(row.confirmation?.values ?? values);
      setConfirmed(row.confirmation?.confirmed ?? true);
      setRequirements((row.requirements && row.requirements.length > 0 ? row.requirements : DEFAULT_REQUIREMENTS) as Requirement[]);
      setMessage('已加载已登记环境参数');
    } catch {
      setRuntime(null);
      setMessage('该环境尚未登记，请填写后保存');
    }
  }, [client, targetID, env, version, baseURL, credentialRef, testDataProfile, values]);

  useEffect(() => { void loadTargets(); }, [loadTargets]);
  useEffect(() => { void loadRuntime(); }, [targetID, env]); // eslint-disable-line react-hooks/exhaustive-deps

  const save = async () => {
    if (!client) return;
    const id = Number(targetID);
    if (!Number.isSafeInteger(id) || id <= 0) { setMessage('请选择被测对象'); return; }
    if (!version.trim()) { setMessage('运行版本不能为空'); return; }
    if (!baseURL.trim()) { setMessage('Base URL 不能为空'); return; }
    if (!confirmed) { setMessage('请确认已核对环境参数'); return; }
    setBusy(true);
    setMessage('');
    try {
      const row = await client.registerRuntime(id, {
        env: env.trim(),
        version: version.trim(),
        base_url: baseURL.trim(),
        requirements,
        confirmation: {
          confirmed,
          values,
          credential_ref: credentialRef.trim(),
          browser_profile: browserProfile.trim(),
          test_data_profile: testDataProfile.trim(),
        },
      });
      setRuntime(row);
      setMessage(`已保存：${row.env} / ${row.running_version}`);
      toast.success('测试环境参数已保存');
    } catch (error: unknown) {
      setMessage(error instanceof Error ? error.message : '保存失败');
    } finally {
      setBusy(false);
    }
  };

  const updateRequirement = (index: number, patch: Partial<Requirement>) => {
    setRequirements((current) => current.map((item, i) => i === index ? { ...item, ...patch } : item));
  };

  return (
    <div className="mx-auto max-w-6xl space-y-4 p-5">
      <PageHeader title="测试环境参数" desc="被测对象的执行地址、运行版本、凭证引用与测试数据配置；保存后执行器只使用这里的可信环境。">
        <Link to="/target" className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-600 hover:border-emerald-300 hover:text-emerald-700"><ArrowLeft className="h-3.5 w-3.5" />返回被测对象</Link>
      </PageHeader>

      <div className="grid gap-4 lg:grid-cols-[1.15fr_0.85fr]">
        <Card title="环境与执行地址" className="p-5">
          <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-slate-800"><Server className="h-4 w-4 text-emerald-600" />环境与执行地址</div>
          <div className="grid gap-3 sm:grid-cols-2">
            <label><span className={labelClass}>被测对象</span>
              <select value={targetID} onChange={(event) => setTargetID(event.target.value)} className={inputClass}>
                {targets.length === 0 && <option value={DEFAULT_TARGET}>im-saas · #{DEFAULT_TARGET}</option>}
                {targets.map((target) => <option key={target.id} value={target.id}>{target.name} · #{target.id}</option>)}
              </select>
            </label>
            <label><span className={labelClass}>环境</span>
              <select value={env} onChange={(event) => setEnv(event.target.value)} className={inputClass}>
                <option value="test">test</option><option value="staging">staging</option><option value="prod">prod</option>
              </select>
            </label>
            <label><span className={labelClass}>运行版本</span><input value={version} onChange={(event) => setVersion(event.target.value)} className={inputClass} placeholder="2.2.0-SNAPSHOT" /></label>
            <label><span className={labelClass}>测试数据 Profile</span><input value={testDataProfile} onChange={(event) => setTestDataProfile(event.target.value)} className={inputClass} placeholder="im-saas-users" /></label>
          </div>
          <label className="mt-3 block"><span className={labelClass}>Base URL（管理员配置，执行器只使用该 origin）</span><input value={baseURL} onChange={(event) => setBaseURL(event.target.value)} className={inputClass} placeholder="http://gateway.open-im-local.svc.cluster.local:3002" /></label>
          <div className="mt-3 grid gap-3 sm:grid-cols-2">
            <label><span className={labelClass}>凭证引用（不填明文）</span><input value={credentialRef} onChange={(event) => setCredentialRef(event.target.value)} className={inputClass} placeholder="k8s://open-im-local/open-im-env" /></label>
            <label><span className={labelClass}>浏览器 Profile（可选）</span><input value={browserProfile} onChange={(event) => setBrowserProfile(event.target.value)} className={inputClass} placeholder="admin-desktop" /></label>
          </div>
          <p className="mt-2 text-[10px] text-slate-400">密码、Token 只能以密钥引用保存；应用数据库不保存明文。</p>
        </Card>

        <Card title="非敏感确认值" className="p-5">
          <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-slate-800"><Database className="h-4 w-4 text-emerald-600" />非敏感确认值</div>
          <div className="space-y-3">
            {Object.entries(values).map(([key, value]) => (
              <label key={key} className="block"><span className={labelClass}>{key}</span>
                <input value={value} onChange={(event) => setValues((current) => ({ ...current, [key]: event.target.value }))} className={inputClass} />
              </label>
            ))}
            <button type="button" onClick={() => setValues((current) => ({ ...current, new_key: '' }))} className="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-2 text-[11px] text-slate-600 hover:border-emerald-300"><Plus className="h-3.5 w-3.5" />添加非敏感项</button>
          </div>
        </Card>
      </div>

      <Card title="环境参数需求" className="p-5">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2 text-sm font-semibold text-slate-800"><KeyRound className="h-4 w-4 text-emerald-600" />环境参数需求</div>
          <button type="button" onClick={() => setRequirements((current) => [...current, { key: '', label: '', required: false, reason: '' }])} className="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-2 text-[11px] text-slate-600 hover:border-emerald-300"><Plus className="h-3.5 w-3.5" />新增参数</button>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead><tr className="border-b border-slate-100 text-[10px] uppercase text-slate-400"><th className="py-2 pr-2">Key</th><th className="py-2 pr-2">名称</th><th className="py-2 pr-2">必填</th><th className="py-2 pr-2">敏感</th><th className="py-2 pr-2">原因</th><th className="py-2" /></tr></thead>
            <tbody>{requirements.map((item, index) => (
              <tr key={`${item.key}-${index}`} className="border-b border-slate-50">
                <td className="py-2 pr-2"><input value={item.key} onChange={(event) => updateRequirement(index, { key: event.target.value })} className="w-36 rounded border border-slate-200 px-2 py-1.5 font-mono text-[11px]" /></td>
                <td className="py-2 pr-2"><input value={item.label} onChange={(event) => updateRequirement(index, { label: event.target.value })} className="w-40 rounded border border-slate-200 px-2 py-1.5 text-[11px]" /></td>
                <td className="py-2 pr-2"><input type="checkbox" checked={item.required} onChange={(event) => updateRequirement(index, { required: event.target.checked })} /></td>
                <td className="py-2 pr-2"><input type="checkbox" checked={!!item.sensitive} onChange={(event) => updateRequirement(index, { sensitive: event.target.checked })} /></td>
                <td className="py-2 pr-2"><input value={item.reason} onChange={(event) => updateRequirement(index, { reason: event.target.value })} className="w-full rounded border border-slate-200 px-2 py-1.5 text-[11px]" /></td>
                <td className="py-2 text-right"><button type="button" onClick={() => setRequirements((current) => current.filter((_, i) => i !== index))} className="text-slate-300 hover:text-red-500" aria-label="删除参数"><Trash2 className="h-3.5 w-3.5" /></button></td>
              </tr>
            ))}</tbody>
          </table>
        </div>
        <label className="mt-4 flex items-center gap-2 text-xs text-slate-600"><input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} />我已核对被测对象、环境地址、版本与凭证引用</label>
        <div className="mt-4 flex flex-wrap items-center gap-2">
          <button type="button" disabled={busy} onClick={() => void save()} className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2.5 text-xs font-semibold text-white hover:bg-emerald-700 disabled:opacity-50">{busy ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}保存环境参数</button>
          <button type="button" disabled={busy} onClick={() => void loadRuntime()} className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2.5 text-xs text-slate-600 hover:border-emerald-300"><RefreshCw className="h-3.5 w-3.5" />重新读取</button>
          {message && <span className="text-[11px] text-slate-500">{message}</span>}
        </div>
      </Card>

      {runtime && (
        <Card title="已登记配置" className="p-5">
          <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-slate-800"><CheckCircle2 className="h-4 w-4 text-emerald-600" />已登记配置</div>
          <div className="grid gap-2 text-xs text-slate-600 sm:grid-cols-2 lg:grid-cols-4">
            <span><b className="text-slate-400">对象：</b>{selectedTarget?.name ?? runtime.target_id}</span>
            <span><b className="text-slate-400">环境：</b>{runtime.env}</span>
            <span><b className="text-slate-400">版本：</b>{runtime.running_version}</span>
            <span><b className="text-slate-400">确认：</b>{runtime.confirmation?.confirmed ? '已确认' : '未确认'}</span>
            <span className="sm:col-span-2"><b className="text-slate-400">Base URL：</b><code className="break-all">{runtime.base_url || '—'}</code></span>
            <span className="sm:col-span-2"><b className="text-slate-400">凭证：</b><code>{runtime.confirmation?.credential_ref || '—'}</code></span>
          </div>
          <div className="mt-4 flex flex-wrap gap-2">
            <Link to="/exec" className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3 py-2 text-xs font-semibold text-white hover:bg-emerald-700"><ShieldCheck className="h-3.5 w-3.5" />去执行用例</Link>
            <Link to="/cases" className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 px-3 py-2 text-xs text-slate-600 hover:border-emerald-300">查看用例</Link>
          </div>
        </Card>
      )}
    </div>
  );
}

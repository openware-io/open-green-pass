import { toast } from 'sonner';
import { CICD_CONNECTORS } from '@/data/mock';
import { PageHeader, PrimaryButton } from '@/components/shared';
import { PlugZap, GitPullRequest, TerminalSquare, Webhook, FileCode, ShieldCheck } from 'lucide-react';

const SPEC_ITEMS = [
  { icon: TerminalSquare, title: '触发 API', desc: 'CI 调用发起测试：被测对象 / 场景 / 用例范围 / 环境 / 分支 / commit SHA', code: 'POST /api/v1/cicd/trigger\n{ asset, scenarios[], env, branch, commit }' },
  { icon: Webhook, title: '门禁回调 API', desc: '判定结果回调 CI：通过 / 阻断 + 证据链接 + 报告 URL（需签名）', code: 'POST /api/v1/cicd/gate-callback\n{ runId, gate: pass|block, evidenceUrl, reportUrl }' },
  { icon: GitPullRequest, title: '事件规范', desc: 'CI 事件模型：push / merge_request / tag / schedule，统一事件语义', code: 'event: push|merge_request|tag|schedule\nbranch: main|release/*|v*\nenv: dev|staging|prod' },
  { icon: FileCode, title: 'CLI 接入', desc: '流水线脚本内调用 greenpass CLI，一步触发并取回门禁判定', code: '$ greenpass test --asset svc-auth \\\n  --scenarios unit,integration \\\n  --branch main --env staging' },
];

export default function CicdConnectorPage() {
  return (
    <div>
      <PageHeader title="连接器与契约" desc="CI 工具连接器配置，以及 GreenPass ↔ CI/CD 的对接契约（API / 事件 / CLI）">
        <PrimaryButton onClick={() => toast.success('新增连接器（原型示意）', { description: '支持 Jenkins / GitLab CI / GitHub Actions / 自研流水线' })}>新增连接器</PrimaryButton>
      </PageHeader>

      {/* 连接器 */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-5">
        {CICD_CONNECTORS.map((c) => (
          <div key={c.id} className="card bg-white rounded-xl border border-slate-200 p-4">
            <div className="flex items-center gap-2 mb-2">
              <div className={'w-8 h-8 rounded-lg flex items-center justify-center ' + (c.status === 'connected' ? 'bg-emerald-500/10 text-emerald-600' : 'bg-slate-100 text-slate-400')}>
                <PlugZap className="w-4 h-4" />
              </div>
              <div>
                <div className="text-sm font-medium text-slate-800">{c.name}</div>
                <div className="text-[9px] text-slate-400">{c.vendor}</div>
              </div>
            </div>
            <div className="flex items-center gap-1.5 text-[10px] mb-2">
              <span className={'inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full ' + (c.status === 'connected' ? 'bg-emerald-50 text-emerald-600' : 'bg-slate-100 text-slate-400')}>
                <span className={'w-1.5 h-1.5 rounded-full ' + (c.status === 'connected' ? 'bg-emerald-500' : 'bg-slate-300')} />{c.status === 'connected' ? '已连接' : '已停用'}
              </span>
              <span className="text-slate-400">{c.mode}</span>
            </div>
            <div className="flex justify-between text-[10px] text-slate-400 mt-2 pt-2 border-t border-slate-100">
              <span>{c.projects} 工程</span>
              <span>7d {c.runs7d} 次</span>
              <span>同步 {c.lastSync}</span>
            </div>
            {c.status === 'connected' && (
              <button type="button" onClick={() => toast.success('连接测试通过（原型示意）', { description: `${c.name} 握手 + 签名校验正常` })}
                className="mt-2 w-full text-[11px] text-emerald-600 border border-emerald-200 rounded-lg py-1 hover:bg-emerald-50">测试连接</button>
            )}
          </div>
        ))}
      </div>

      {/* 对接契约 */}
      <div className="card bg-white rounded-xl border border-slate-200 p-5">
        <div className="flex items-center justify-between mb-1">
          <h2 className="font-semibold text-slate-700 text-sm flex items-center gap-1.5"><ShieldCheck className="w-4 h-4 text-emerald-500" />对接契约（原型示意）</h2>
          <span className="text-[11px] text-slate-400">统一契约 · 多 CI 适配 · 签名鉴权</span>
        </div>
        <p className="text-[10px] text-slate-400 mb-4">GreenPass 对外提供稳定 API / CLI / 事件契约，各 CI 工具通过适配层统一接入；所有回调需 HMAC 签名校验 + 白名单，防伪造。以下为契约骨架示例。</p>
        <div className="grid grid-cols-2 gap-4">
          {SPEC_ITEMS.map((s) => {
            const Icon = s.icon;
            return (
              <div key={s.title} className="border border-slate-100 rounded-xl p-4">
                <div className="flex items-center gap-2 mb-1.5">
                  <Icon className="w-4 h-4 text-emerald-500" />
                  <span className="text-sm font-medium text-slate-700">{s.title}</span>
                </div>
                <div className="text-[10px] text-slate-400 mb-2">{s.desc}</div>
                <pre className="bg-slate-900 text-emerald-200/90 text-[10px] rounded-lg p-3 overflow-x-auto leading-relaxed font-mono">{s.code}</pre>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

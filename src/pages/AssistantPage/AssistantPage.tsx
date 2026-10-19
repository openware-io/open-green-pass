import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { toast } from 'sonner';
import { Activity, ArrowRight, BotMessageSquare, CheckCircle2, CircleAlert, CircleCheck, ClipboardCheck, FileSearch, LockKeyhole, Sparkles, Timer, UserRoundCheck } from 'lucide-react';
import { Card, GhostButton, PageHeader, PrimaryButton } from '@/components/shared';
import { configuredGreenPassClient } from '@/api/runtime';
import type { AssistantArtifact } from '@/api/client';

type AdviceStatus = '待处理' | '处理中' | '已归档';
type Advice = {
  id: string; title: string; source: string; createdAt: string; priority: '高' | '中' | '低';
  summary: string; recommendation: string; evidence: string[]; status: AdviceStatus;
};

const ADVICE_QUEUE: Advice[] = [
  { id: 'ADV-4821-01', title: '退款接口存在破坏性契约变更', source: 'Run RUN-4821 · 契约 diff', createdAt: '10:31', priority: '高', status: '待处理', summary: 'POST /v2/refund 的响应字段 refundStatus 被替换，3 个消费者存在兼容风险。', recommendation: '先查看 CT-003 的差异证据，再创建受影响消费者的复测草稿。', evidence: ['契约 CT-003 · v2.5.0', '3 个消费者调用点', 'RUN-4821 请求响应证据'] },
  { id: 'ADV-4821-02', title: '检测到 3 处弱断言', source: 'AI 专项审计 · 断言强度', createdAt: '10:29', priority: '高', status: '待处理', summary: '退款金额场景中出现 assertTrue(true) 与宽泛异常捕获，现有通过结果不具备可信度。', recommendation: '查看对应源码证据，生成断言修复草稿后由测试工程师提交。', evidence: ['TC-2024-118', 'svc-payment-refund.log', '变异分数 42%'] },
  { id: 'ADV-4821-03', title: '登录 E2E 存在偶发超时', source: '历史趋势 · Web E2E', createdAt: '09:48', priority: '中', status: '待处理', summary: '近 5 次执行中有 2 次在登录页等待元素超时，尚不足以判定为产品缺陷。', recommendation: '引导查看运行证据并创建一次隔离环境复测。', evidence: ['TC-2024-006', '2/5 次失败', '浏览器运行日志'] },
  { id: 'ADV-4821-04', title: '生成批次存在覆盖缺口', source: '用例生成质量检查', createdAt: '昨天 18:42', priority: '低', status: '已归档', summary: '优惠券叠加边界未覆盖。', recommendation: '下次生成批次补齐。', evidence: ['REQ-104', 'generation batch GEN-2041'] },
];

const STEPS = [
  { title: '查看阻断依据', desc: '核对门禁规则、契约差异与原始证据。', route: '/gate', action: '打开 CT-003 门禁规则' },
  { title: '确认处理范围', desc: '确认受影响消费者与复测环境，不直接修改任何对象。', route: '/cases', action: '预填 3 个受影响用例' },
  { title: '用户提交复测', desc: '由你在测试执行页确认并启动复测。', route: '/exec', action: '前往复测确认页' },
  { title: '核验结果与归档', desc: '读取权威执行结果，保留处理证据与审计链。', route: '/history', action: '等待 Run 结果' },
];

const PRIORITY_STYLE = { 高: 'bg-red-50 text-red-600 border-red-200', 中: 'bg-amber-50 text-amber-700 border-amber-200', 低: 'bg-slate-100 text-slate-500 border-slate-200' };
const AUDIT = [
  ['assistant.artifact.created', '后台生成契约风险建议', '隐式 · 无界面动作', '10:31:02'],
  ['ui.navigate', '打开质量门禁 / CT-003', '显式引导 · 张立', '10:34:10'],
  ['ui.prefill', '预填 3 个受影响复测用例', '显式引导 · 张立', '10:34:42'],
  ['user.command.pending', '等待用户确认创建复测 Run', '用户提交点', '—'],
];

export default function AssistantPage() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const explicitEntry = params.get('mode') === 'guide' || params.get('issue') === 'gate-blocked';
  const [liveArtifacts, setLiveArtifacts] = useState<AssistantArtifact[]>([]);
  const liveClient = useMemo(() => configuredGreenPassClient(), []);
  useEffect(() => { if (liveClient) void liveClient.assistantArtifacts().then(setLiveArtifacts).catch(() => undefined); }, [liveClient]);
  const adviceQueue = useMemo<Advice[]>(() => liveArtifacts.length ? liveArtifacts.map((item) => ({ id: String(item.id), title: item.title, source: 'AI 协作服务 · 已审计', createdAt: new Date(item.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }), priority: item.priority === 'high' ? '高' : item.priority === 'medium' ? '中' : '低', summary: item.summary, recommendation: item.recommendation, evidence: item.evidence, status: item.status === 'archived' ? '已归档' : '待处理' })) : ADVICE_QUEUE, [liveArtifacts]);
  const [selectedId, setSelectedId] = useState(explicitEntry ? 'ADV-4821-01' : 'ADV-4821-02');
  const [guideActive, setGuideActive] = useState(explicitEntry);
  const [step, setStep] = useState(0);
  const [showDraft, setShowDraft] = useState(false);
  const selected = useMemo(() => adviceQueue.find((item) => item.id === selectedId) ?? adviceQueue[0], [adviceQueue, selectedId]);

  const startGuide = () => { setGuideActive(true); setStep(0); setShowDraft(false); toast.success('已开启界面引导', { description: '助手将定位当前步骤；正式提交仍由你完成。' }); };
  const doStep = () => {
    const current = STEPS[step];
    if (step === 1) { setShowDraft(true); toast('已在表单中预填复测范围', { description: '此处只是草稿，尚未创建 Run。' }); return; }
    if (step === 2) { toast('等待你的确认', { description: '助手不会替你启动复测；请在测试执行页使用原有按钮提交。' }); navigate(current.route); return; }
    navigate(current.route);
    if (step < STEPS.length - 1) setStep((value) => value + 1);
  };

  return (
    <div>
      <PageHeader title="AI 协作中心" desc="后台建议只产生待处理事项；你主动求助时，AI 在界面中带着你完成操作。">
        <GhostButton onClick={() => toast('当前策略：隐式不操作界面', { description: '后台只生成 Artifact、通知与待处理事项；无浏览器会话、无 UI 指令。' })}>查看边界</GhostButton>
        <PrimaryButton onClick={startGuide}><span className="inline-flex items-center gap-1.5"><BotMessageSquare className="h-4 w-4" />带我处理当前建议</span></PrimaryButton>
      </PageHeader>

      <div className="mb-5 grid gap-4 lg:grid-cols-3">
        <div className="rounded-xl border border-slate-200 bg-white p-4">
          <div className="mb-2 flex items-center gap-2 text-xs font-medium text-slate-600"><Activity className="h-4 w-4 text-emerald-600" />隐式后台建议</div>
          <div className="text-2xl font-bold text-slate-800">3 <span className="text-sm font-normal text-slate-400">待处理</span></div>
          <p className="mt-1 text-[11px] text-slate-400">分析与建议已入库；不会导航、预填或提交你的界面。</p>
        </div>
        <div className="rounded-xl border border-emerald-200 bg-emerald-50 p-4">
          <div className="mb-2 flex items-center gap-2 text-xs font-medium text-emerald-700"><UserRoundCheck className="h-4 w-4" />显式界面引导</div>
          <div className="text-2xl font-bold text-emerald-800">{guideActive ? '进行中' : '待发起'}</div>
          <p className="mt-1 text-[11px] text-emerald-700/70">定位、解释、预填；正式提交始终由当前用户完成。</p>
        </div>
        <div className="rounded-xl border border-slate-200 bg-white p-4">
          <div className="mb-2 flex items-center gap-2 text-xs font-medium text-slate-600"><LockKeyhole className="h-4 w-4 text-slate-500" />代执行命令</div>
          <div className="text-2xl font-bold text-slate-700">默认关闭</div>
          <p className="mt-1 text-[11px] text-slate-400">仅预留为按次明确授权的例外，不纳入当前引导流程。</p>
        </div>
      </div>

      <div className="grid gap-5 xl:grid-cols-[360px_minmax(0,1fr)]">
        <Card title="后台建议队列" extra={<span className="text-[11px] text-slate-400">隐式 · 不操作 UI</span>}>
          <div className="divide-y divide-slate-100">
            {adviceQueue.map((item) => {
              const active = item.id === selected.id;
              return <button key={item.id} type="button" onClick={() => { setSelectedId(item.id); setGuideActive(false); setStep(0); setShowDraft(false); }} className={`w-full px-4 py-3.5 text-left transition ${active ? 'bg-emerald-50/70' : 'hover:bg-slate-50'}`}>
                <div className="mb-1 flex items-start justify-between gap-2"><span className={`rounded-full border px-1.5 py-0.5 text-[10px] ${PRIORITY_STYLE[item.priority]}`}>{item.priority}优先级</span><span className="text-[10px] text-slate-400">{item.createdAt}</span></div>
                <div className="text-sm font-medium text-slate-800">{item.title}</div>
                <div className="mt-1 text-[11px] text-slate-400">{item.source} · {item.status}</div>
              </button>;
            })}
          </div>
        </Card>

        <div className="space-y-5">
          <Card title={selected.title} extra={<span className={`rounded-full border px-2 py-0.5 text-[10px] ${PRIORITY_STYLE[selected.priority]}`}>{selected.id}</span>}>
            <div className="grid gap-5 p-5 lg:grid-cols-[minmax(0,1fr)_240px]">
              <div>
                <div className="mb-2 flex items-center gap-2 text-xs font-medium text-slate-600"><Sparkles className="h-4 w-4 text-emerald-600" />后台分析结论</div>
                <p className="text-sm leading-6 text-slate-700">{selected.summary}</p>
                <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5 text-xs text-amber-800"><span className="font-semibold">建议：</span>{selected.recommendation}</div>
              </div>
              <div className="rounded-lg bg-slate-50 p-3">
                <div className="mb-2 text-[11px] font-semibold text-slate-500">事实与证据</div>
                <div className="space-y-2">{selected.evidence.map((evidence) => <div key={evidence} className="flex items-start gap-1.5 text-xs text-slate-600"><FileSearch className="mt-0.5 h-3.5 w-3.5 shrink-0 text-emerald-600" />{evidence}</div>)}</div>
                <div className="mt-3 border-t border-slate-200 pt-2 text-[10px] text-slate-400">来源已绑定 Run、规则版本和证据哈希</div>
              </div>
            </div>
          </Card>

          <Card title="界面引导 · 用户主动发起后才运行" extra={<span className={guideActive ? 'text-[11px] font-medium text-emerald-600' : 'text-[11px] text-slate-400'}>{guideActive ? `第 ${step + 1}/${STEPS.length} 步` : '尚未开始'}</span>}>
            <div className="p-5">
              <div className="grid gap-2 md:grid-cols-4">
                {STEPS.map((item, index) => {
                  const current = guideActive && step === index;
                  const done = guideActive && step > index;
                  return <button key={item.title} type="button" disabled={!guideActive || index > step} onClick={() => setStep(index)} className={`rounded-xl border p-3 text-left transition ${current ? 'border-emerald-500 bg-emerald-50 shadow-sm' : done ? 'border-emerald-200 bg-white' : 'border-slate-200 bg-slate-50 opacity-70'}`}>
                    <div className="mb-2 flex items-center justify-between"><span className={`grid h-6 w-6 place-items-center rounded-full text-[11px] font-bold ${done ? 'bg-emerald-600 text-white' : current ? 'bg-emerald-600 text-white' : 'bg-slate-200 text-slate-500'}`}>{done ? <CheckCircle2 className="h-4 w-4" /> : index + 1}</span>{current && <Timer className="h-3.5 w-3.5 text-emerald-600" />}</div>
                    <div className="text-xs font-semibold text-slate-700">{item.title}</div><div className="mt-1 text-[10px] leading-4 text-slate-500">{item.desc}</div>
                  </button>;
                })}
              </div>

              {guideActive ? <div className="mt-5 rounded-xl border border-emerald-200 bg-emerald-50 p-4">
                <div className="flex flex-wrap items-start justify-between gap-3"><div><div className="flex items-center gap-2 text-sm font-semibold text-emerald-900"><CircleCheck className="h-4 w-4" />当前引导：{STEPS[step].title}</div><p className="mt-1 text-xs text-emerald-800">{STEPS[step].desc} 助手将只执行“{STEPS[step].action}”；不会替你提交。</p></div><button type="button" onClick={doStep} className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3 py-2 text-xs font-medium text-white hover:bg-emerald-700">{step === 1 ? '预填复测范围' : step === 2 ? '前往并由我确认' : '打开对应页面'}<ArrowRight className="h-3.5 w-3.5" /></button></div>
                {showDraft && <div className="mt-4 rounded-lg border border-emerald-200 bg-white p-3"><div className="mb-2 flex items-center gap-2 text-xs font-semibold text-slate-700"><ClipboardCheck className="h-4 w-4 text-emerald-600" />已预填的复测草稿 <span className="font-normal text-slate-400">未提交</span></div><div className="grid gap-2 text-xs text-slate-600 sm:grid-cols-3"><span className="rounded bg-slate-50 px-2 py-1.5">环境：staging</span><span className="rounded bg-slate-50 px-2 py-1.5">场景：契约测试</span><span className="rounded bg-slate-50 px-2 py-1.5">用例：TC-118 / 095 / 133</span></div></div>}
              </div> : <div className="mt-5 rounded-xl border border-dashed border-slate-300 bg-slate-50 p-4 text-center"><CircleAlert className="mx-auto h-5 w-5 text-slate-400" /><div className="mt-2 text-sm font-medium text-slate-700">后台建议不会擅自操作你的界面</div><div className="mt-1 text-xs text-slate-500">点击“带我处理当前建议”后，才开始本次可见、可中断的界面引导。</div></div>}
            </div>
          </Card>
        </div>
      </div>

      <Card title="本次 AI 协作审计" className="mt-5" extra={<span className="text-[11px] text-slate-400">每个建议、引导和用户提交点都进入哈希链</span>}>
        <div className="overflow-x-auto"><table className="w-full text-xs"><thead className="border-b border-slate-200 bg-slate-50 text-left text-slate-500"><tr><th className="px-4 py-2.5 font-medium">事件</th><th className="px-4 py-2.5 font-medium">语义操作</th><th className="px-4 py-2.5 font-medium">主体</th><th className="px-4 py-2.5 font-medium">时间</th><th className="px-4 py-2.5 font-medium">状态</th></tr></thead><tbody className="divide-y divide-slate-100">{AUDIT.map(([event, action, actor, time], index) => <tr key={event} className="hover:bg-slate-50"><td className="px-4 py-3 font-mono text-[11px] text-emerald-700">{event}</td><td className="px-4 py-3 text-slate-700">{action}</td><td className="px-4 py-3 text-slate-500">{actor}</td><td className="px-4 py-3 text-slate-400">{time}</td><td className="px-4 py-3">{index === AUDIT.length - 1 ? <span className="rounded-full bg-amber-50 px-2 py-0.5 text-[10px] text-amber-700">等待用户</span> : <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] text-emerald-700">已锚定</span>}</td></tr>)}</tbody></table></div>
      </Card>
    </div>
  );
}

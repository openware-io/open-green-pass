import { useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { toast } from 'sonner';
import { assetToProfile, type IAsset } from '@/data/mock';
import { PageHeader, GhostButton, PrimaryButton, Card } from '@/components/shared';
import { BrainCircuit, Loader2 } from 'lucide-react';

const RULE_ICON = { pass: <span className="text-emerald-600">✓</span>, block: <span className="text-red-600">✕</span> };
const RULE_BG = { pass: 'bg-emerald-100 text-emerald-600', block: 'bg-red-100 text-red-600' };
const RULE_BADGE = { pass: 'bg-emerald-50 text-emerald-600', block: 'bg-red-50 text-red-600' };

const AI_NOTES: Record<string, string> = {
  '覆盖率阈值': 'AI 依据 PR 变更文件集合与库覆盖率报告交叉比对',
  '断言强度检测': 'AI 静态分析用例源码，识别 assertTrue(true)、try/except 包裹、skip 标记',
  '契约门禁 · CT-003': 'AI 解析 OpenAPI 变更 diff，比对消费者调用点，判定破坏性',
  '变异测试分数': 'AI 注入代码变体并回放用例，统计被杀比例',
  '跨服务追溯完整性': 'AI 沿 需求→测试点→用例 链遍历，校验无孤儿用例',
  '原始测试回放': 'AI 在产出代码上回放基线用例，比对断言结果',
};

export default function GatePage() {
  const { selectedAsset } = useOutletContext<{ selectedAsset: IAsset }>();
  const profile = assetToProfile(selectedAsset);
  const gateRules = profile.gateRules;
  const GATE_TOTAL = gateRules.length;
  const GATE_PASS = gateRules.filter((r) => r.status === 'pass').length;
  const GATE_BLOCK = gateRules.filter((r) => r.status === 'block').length;

  // 判定过程状态：idle=初始 · running=AI 重新判定中 · done=刚完成一轮判定
  const [phase, setPhase] = useState<'idle' | 'running' | 'done'>('idle');
  const [note, setNote] = useState('');

  const handleRejudge = () => {
    if (phase === 'running') return;
    setPhase('running');
    setNote(`AI 判定引擎正在重新校验 ${GATE_TOTAL} 条门禁规则（${profile.name}）…`);
    // 原型 mock：模拟 AI 逐条重判的耗时，不接后端
    window.setTimeout(() => {
      setPhase('done');
      const verdict = GATE_BLOCK > 0 ? '阻断' : '通过';
      setNote(`判定完成 · ${GATE_BLOCK} 项规则${GATE_BLOCK > 0 ? '未通过' : '全部通过'}，结论：${verdict}`);
      toast.warning('门禁判定完成', {
        description: `AI 重新判定 ${profile.name}：${GATE_BLOCK} 项阻断，结论：${verdict}`,
      });
      // 短暂展示 done 后复位，便于再次触发
      window.setTimeout(() => setPhase('idle'), 2600);
    }, 1600);
  };

  const handleViewPolicy = () => {
    toast('策略快照 · v2.5.0', {
      description: 'pre-merge-gate：coverage≥80% · 断言强度阻断 · 变异分数≥70% · 无跳过用例 · 契约门禁 · 追溯100%',
    });
  };

  const running = phase === 'running';
  const blockRules = gateRules.filter((r) => r.status === 'block');
  const contractBlocked = gateRules.some((r) => r.name.includes('契约') && r.status === 'block');
  const tamperCount = gateRules.find((r) => r.name.includes('断言'))?.evidence?.length ?? 0;

  const SUMMARY = [
    { label: '本次判定', value: GATE_BLOCK > 0 ? '阻断' : '通过', color: GATE_BLOCK > 0 ? 'text-red-600' : 'text-emerald-600', note: `${profile.name} · ${GATE_BLOCK} 项未通过` },
    { label: '规则总数', value: `${GATE_TOTAL}`, color: 'text-slate-800', note: `通过 ${GATE_PASS} · 失败 ${GATE_BLOCK}` },
    { label: '篡改检测', value: `${tamperCount} 处`, color: GATE_BLOCK > 0 ? 'text-red-600' : 'text-emerald-600', note: '断言弱化 / 跳过标记' },
    { label: '契约门禁', value: contractBlocked ? '阻断' : '通过', color: contractBlocked ? 'text-red-600' : 'text-emerald-600', note: contractBlocked ? 'CT-003 破坏性变更' : '契约均兼容' },
  ];

  return (
    <div>
      <PageHeader title="质量门禁" desc={`${profile.name} · 策略即代码 · AI 判定 · 确定性验证 · 篡改检测 · 契约门禁`}>
        <GhostButton onClick={handleViewPolicy}>查看策略</GhostButton>
        <PrimaryButton onClick={handleRejudge} disabled={running}>
          {running ? <span className="inline-flex items-center gap-1.5"><Loader2 className="w-3.5 h-3.5 animate-spin" />判定中…</span> : phase === 'done' ? '重新判定 ✓' : '重新判定'}
        </PrimaryButton>
      </PageHeader>

      {/* AI 判定过程反馈条 */}
      {(running || phase === 'done') && (
        <div className={`mb-5 px-4 py-3 rounded-xl text-sm border flex items-center gap-3 ${
          running ? 'bg-emerald-50 border-emerald-200 text-emerald-700' : 'bg-amber-50 border-amber-200 text-amber-700'}`}>
          {running
            ? <><Loader2 className="w-4 h-4 animate-spin" />{note}</>
            : <><BrainCircuit className="w-4 h-4" />{note}</>}
        </div>
      )}

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-5">
        {SUMMARY.map((s) => (
          <div key={s.label} className="card bg-white rounded-xl border border-slate-200 p-4">
            <div className="text-xs text-slate-500 mb-1">{s.label}</div>
            <div className={'text-lg font-bold ' + s.color}>{s.value}</div>
            <div className="text-[11px] text-slate-400 mt-1">{s.note}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-3 gap-5">
        <div className="col-span-2 card bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
            <h2 className="font-semibold text-slate-700 text-sm">门禁规则判定结果 · {profile.name}</h2>
            <span className="text-[11px] text-emerald-500 font-medium flex items-center gap-1"><BrainCircuit className="w-3.5 h-3.5" />AI 判定引擎</span>
          </div>
          <div className="divide-y divide-slate-100">
            {gateRules.map((r) => (
              <div key={r.name} className={'px-5 py-3.5 flex items-start gap-3 ' + (r.status === 'block' ? 'bg-red-50/40' : '')}>
                <div className={'w-6 h-6 rounded-full flex items-center justify-center text-xs flex-shrink-0 mt-0.5 ' + RULE_BG[r.status]}>
                  {RULE_ICON[r.status]}
                </div>
                <div className="flex-1">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium text-slate-700">{r.name}</span>
                    <span className={'text-[11px] px-2 py-0.5 rounded-full ' + RULE_BADGE[r.status]}>
                      {r.status === 'pass' ? '通过' : '阻断'}
                    </span>
                  </div>
                  <div className="text-[11px] text-slate-500 mt-1">{r.detail}</div>
                  {/* AI 判定依据链：展示 AI 如何得出该结论 */}
                  {AI_NOTES[r.name] && (
                    <div className="mt-1.5 text-[10px] text-emerald-500 font-mono">AI 依据：{AI_NOTES[r.name]}</div>
                  )}
                  {r.evidence && (
                    <div className="mt-2 bg-white border border-red-200 rounded-lg p-2.5 space-y-1.5">
                      {r.evidence.map((e) => (
                        <div key={e} className="flex items-center gap-2 text-[11px]">
                          <span className="font-mono text-red-600">{e.split(' → ')[0]}</span>
                          <span className="text-slate-400">→</span>
                          <span className="text-slate-600">{e.split(' → ')[1]}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
          {/* 本轮阻断概览 */}
          <div className="px-5 py-3 border-t border-slate-100 flex items-center gap-2 text-[11px]">
            <span className="text-slate-500">本轮阻断</span>
            {blockRules.length === 0
              ? <span className="text-emerald-600">无 · 全部通过</span>
              : blockRules.map((r) => (
                  <span key={r.name} className="bg-red-50 text-red-600 px-2 py-0.5 rounded-full font-medium">{r.name.replace(' · CT-003', '')}</span>
                ))}
          </div>
        </div>

        <div className="space-y-5">
          <Card title="策略快照" className="p-5">
            <div className="bg-slate-900 rounded-lg p-3">
              <pre className="text-[10px] text-slate-300 font-mono leading-relaxed">quality_gate:
  name: pre-merge-gate
  rules:
    - coverage: 80%
    - assertion-strength: block
    - mutation-score: 70%
    - no-skipped-tests: block
    - contract-gate: block
    - traceability: 100%
    - required-types: [unit,
        integration, contract,
        security, smoke]</pre>
            </div>
            <div className="mt-3 flex justify-between text-[11px]">
              <span className="text-slate-500">策略版本</span>
              <span className="font-mono text-slate-600">v2.5.0</span>
            </div>
          </Card>
          <Card title="门禁判定审计" className="p-5">
            <div className="space-y-2.5 text-[11px]">
              <div className="flex items-start gap-2">
                <span className="text-red-500 mt-0.5">●</span>
                <div>
                  <div className="text-slate-700">门禁阻断 · CI #4821</div>
                  <div className="text-slate-400 mt-0.5">2026-09-26 10:30:12</div>
                  <div className="font-mono text-slate-400">log: sha256:0a1f…</div>
                </div>
              </div>
              <div className="flex items-start gap-2">
                <span className="text-red-500 mt-0.5">●</span>
                <div>
                  <div className="text-slate-700">契约变更告警 · CT-003</div>
                  <div className="text-slate-400 mt-0.5">2026-09-26 10:28:00</div>
                  <div className="font-mono text-slate-400">log: sha256:7b2c…</div>
                </div>
              </div>
              <div className="flex items-start gap-2">
                <span className="text-amber-500 mt-0.5">●</span>
                <div>
                  <div className="text-slate-700">篡改检测发现 3 处</div>
                  <div className="text-slate-400 mt-0.5">2026-09-26 10:27:30</div>
                  <div className="font-mono text-slate-400">log: sha256:6b2d…</div>
                </div>
              </div>
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}

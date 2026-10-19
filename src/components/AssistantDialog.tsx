import { useEffect, useMemo, useRef, useState } from 'react';
import { BotMessageSquare, FileSearch, Loader2, Send, Sparkles, WandSparkles, X } from 'lucide-react';
import { configuredGreenPassClient } from '@/api/runtime';
import { GreenPassApiError, type AssistantTurn, type AssistantUIAction } from '@/api/client';

const PAGE_LABEL: Record<string, string> = { '/gate': '质量门禁', '/exec': '测试执行', '/history': '测试历史', '/cases': '用例管理', '/generation': '用例生成', '/trace': '需求追溯', '/target': '被测对象', '/scenarios': '测试中心', '/assistant': 'AI 协作中心' };
const sessionKey = (page: string) => `greenpass_assistant_session:${page}`;

export function AssistantDialog({ open, onOpenChange, path, onGuidance }: { open: boolean; onOpenChange: (open: boolean) => void; path: string; onGuidance: (action: AssistantUIAction) => void }) {
  const [input, setInput] = useState('');
  const [connection, setConnection] = useState<{ path: string; sessionID?: number; messages: AssistantTurn[]; error?: string }>({ path: '', messages: [] });
  const [loading, setLoading] = useState(false);
  const temporaryMessageID = useRef(0);
  const client = useMemo(() => configuredGreenPassClient(), []);
  const pageLabel = PAGE_LABEL[path] ?? '当前页面';
  const suggestions = useMemo(() => path === '/gate' ? ['为什么本次门禁被阻断？', '帮我梳理修复优先级', '带我查看阻断证据'] : ['当前有哪些风险需要处理？', '解释这个页面的数据', '带我完成下一步'], [path]);
  const activeConnection = connection.path === path ? connection : undefined;
  const messages = activeConnection?.messages ?? [];
  const sessionID = activeConnection?.sessionID;
  const error = activeConnection?.error ?? '';
  useEffect(() => {
    if (!open || !client) return;
    let cancelled = false;
    const connect = async () => {
      try {
        let stored = 0;
        try { stored = Number(localStorage.getItem(sessionKey(path))); } catch { /* use a fresh session when storage is unavailable */ }
        const session = stored > 0 ? { id: stored } : await client.createAssistantSession(path as '/gate');
        if (!stored) { try { localStorage.setItem(sessionKey(path), String(session.id)); } catch { /* session remains usable for this view */ } }
        const turns = await client.assistantTurns(session.id);
        if (!cancelled) setConnection({ path, sessionID: session.id, messages: turns });
      } catch { if (!cancelled) setConnection({ path, messages: [], error: '暂时无法连接 AI 协作服务，请稍后重试。' }); }
    };
    void connect(); return () => { cancelled = true; };
  }, [open, path, client]);
  const submit = async (value = input) => {
    const question = value.trim(); if (!question || !client || !sessionID || loading) return;
    setLoading(true); setInput('');
    const optimistic: AssistantTurn = { id: --temporaryMessageID.current, session_id: sessionID, role: 'user', text: question, created_at: new Date().toISOString() };
    setConnection(current => current.path === path && current.sessionID === sessionID ? { ...current, error: undefined, messages: [...current.messages, optimistic] } : current);
    try {
      const result = await client.createAssistantTurn(sessionID, question);
      setConnection(current => current.path === path && current.sessionID === sessionID ? { ...current, messages: [...current.messages.filter(x => x.id !== optimistic.id), result.user, result.assistant] } : current);
    }
    catch (cause) {
      setInput(question);
      let failure = '对话发送失败，请检查连接后重试。';
      if (cause instanceof GreenPassApiError && cause.status === 502) {
        failure = 'AI 模型暂时不可用，对话未写入，请稍后重试。';
      } else if (cause instanceof GreenPassApiError && cause.message) {
        failure = `对话发送失败（${cause.status}）：${cause.message}`;
      }
      setConnection(current => current.path === path && current.sessionID === sessionID ? { ...current, error: failure, messages: current.messages.filter(x => x.id !== optimistic.id) } : current);
    }
    finally { setLoading(false); }
  };
  const startGuide = async () => {
    if (!client || !sessionID || loading) return; setLoading(true);
    setConnection(current => current.path === path && current.sessionID === sessionID ? { ...current, error: undefined } : current);
    try { const actions = await client.startAssistantGuidance(sessionID); const action = actions[0]; if (!action) throw new Error('empty guidance'); onGuidance(action); await client.completeAssistantAction(sessionID, action.id, 'completed'); }
    catch { setConnection(current => current.path === path && current.sessionID === sessionID ? { ...current, error: '无法启动引导；未执行任何界面动作。' } : current); } finally { setLoading(false); }
  };
  return <aside aria-label="AI 对话与操作引导" className={`flex h-full min-h-0 shrink-0 flex-col overflow-hidden bg-white transition-[width,opacity] duration-200 ${open ? 'w-[clamp(320px,32vw,460px)] border-l border-slate-200 opacity-100' : 'w-0 border-l-0 opacity-0'}`}>
    <div className="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3.5"><div className="flex min-w-0 items-center gap-2.5"><span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-emerald-50 text-emerald-700"><BotMessageSquare className="h-4 w-4" /></span><div className="min-w-0"><h2 className="truncate text-sm font-semibold text-slate-800">AI 协作</h2><p className="truncate text-[11px] text-slate-400">当前页面的上下文对话</p></div></div><button type="button" onClick={() => onOpenChange(false)} className="rounded-md p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-700" aria-label="关闭 AI 对话"><X className="h-4 w-4" /></button></div>
    <div className="flex shrink-0 items-center justify-between gap-2 border-b border-slate-100 bg-slate-50 px-5 py-2.5"><div className="min-w-0 text-xs text-slate-600"><span className="text-slate-400">上下文</span><span className="mx-1.5 text-slate-300">/</span><span className="font-medium">{pageLabel}</span></div><span className="shrink-0 text-[11px] font-medium text-emerald-700">服务端审计</span></div>
    <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-5"><div className="flex gap-2.5"><span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-full bg-emerald-100 text-emerald-700"><Sparkles className="h-4 w-4" /></span><div className="max-w-[90%] text-sm leading-6 text-slate-700">我会基于当前页面和你的权限范围回答；需要操作时，只会定位、解释和预填，正式提交仍由你完成。</div></div>
      {!client && <p className="rounded-lg bg-amber-50 p-3 text-xs text-amber-800">尚未配置 AI 服务连接。</p>}{error && <p className="rounded-lg bg-red-50 p-3 text-xs text-red-700">{error}</p>}
      {messages.length === 0 && <div className="ml-9 rounded-lg border border-slate-200 bg-slate-50 p-3"><div className="mb-1.5 text-xs font-medium text-slate-700">从当前工作开始</div><p className="text-xs leading-5 text-slate-500">可以询问本页数据、风险原因或下一步；决定处理后再发起可见的界面引导。</p><div className="mt-3 flex flex-wrap gap-1.5">{suggestions.map(s => <button key={s} type="button" disabled={!sessionID || loading} onClick={() => void submit(s)} className="rounded-md border border-slate-200 bg-white px-2 py-1 text-[11px] text-slate-600 hover:border-emerald-300 hover:text-emerald-700 disabled:opacity-50">{s}</button>)}</div></div>}
      {messages.map(message => message.role === 'user' ? <div key={message.id} className="flex justify-end"><div className="max-w-[85%] rounded-xl rounded-tr-sm bg-emerald-600 px-3 py-2.5 text-sm leading-6 text-white">{message.text}</div></div> : <div key={message.id} className="flex gap-2.5"><span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-full bg-emerald-100 text-emerald-700"><Sparkles className="h-4 w-4" /></span><div className="max-w-[85%] rounded-xl rounded-tl-sm bg-slate-100 px-3 py-2.5 text-sm leading-6 text-slate-700">{message.text}</div></div>)}
      {loading && <div className="flex items-center gap-2 text-xs text-slate-400"><Loader2 className="h-3.5 w-3.5 animate-spin" />正在处理…</div>}</div>
    <div className="shrink-0 border-t border-slate-200 bg-white px-5 py-4"><div className="flex items-end gap-2"><textarea value={input} disabled={!sessionID || loading} onChange={e => setInput(e.target.value)} onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); void submit(); } }} rows={2} placeholder="问问当前页面，或描述你要完成的事…" className="min-h-[54px] flex-1 resize-none rounded-lg border border-slate-200 px-3 py-2.5 text-sm outline-none focus:border-emerald-400 disabled:bg-slate-50" /><button type="button" disabled={!sessionID || loading} onClick={() => void submit()} className="grid h-10 w-10 place-items-center rounded-lg bg-emerald-600 text-white hover:bg-emerald-700 disabled:bg-slate-300" aria-label="发送问题"><Send className="h-4 w-4" /></button></div><div className="mt-3 flex items-center justify-between gap-2"><span className="inline-flex items-center gap-1.5 text-[10px] text-slate-400"><FileSearch className="h-3 w-3" />对话摘要与引导均审计</span><button type="button" disabled={!sessionID || loading} onClick={() => void startGuide()} className="inline-flex shrink-0 items-center gap-1.5 rounded-lg border border-emerald-600 px-2.5 py-1.5 text-xs font-medium text-emerald-700 hover:bg-emerald-50 disabled:border-slate-200 disabled:text-slate-400"><WandSparkles className="h-3.5 w-3.5" />带我操作</button></div></div>
  </aside>;
}

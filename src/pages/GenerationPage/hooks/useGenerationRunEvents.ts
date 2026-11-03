import { useCallback, useEffect, useRef, useState } from 'react';
import { GreenPassApiError, type GenerationRunDetail, type GreenPassClient } from '@/api/client';
import { isTerminalRunState, normalizeRunDetail } from '../types';

export type RunConnectionState = 'idle' | 'connecting' | 'live' | 'reconnecting' | 'fallback' | 'closed';

export type RunEventsController = {
  run: GenerationRunDetail | null;
  /** Applies an authoritative snapshot returned by create/cancel/review calls. */
  setRun: (run: GenerationRunDetail | null) => void;
  connection: RunConnectionState;
  error: string;
  /** One-shot detail read (never polled). */
  refresh: () => Promise<GenerationRunDetail | null>;
};

const MAX_RECONNECT_ATTEMPTS = 5;

/**
 * Subscribes to GET /generation/runs/{id}/events?after_seq=N.
 * - resumption uses after_seq + Last-Event-ID, so missed events are replayed
 * - a dropped connection reconnects with exponential backoff (1/2/4/8/16s)
 * - when the stream is unavailable it degrades to a SINGLE detail read and
 *   stops: there is deliberately no interval/2s polling anywhere in this hook.
 */
export function useGenerationRunEvents(runId: number | null, options: {
  client: GreenPassClient | undefined;
  onApiError?: (error: unknown) => void;
}): RunEventsController {
  const { client, onApiError } = options;
  const [run, setRunState] = useState<GenerationRunDetail | null>(null);
  const [connection, setConnection] = useState<RunConnectionState>('idle');
  const [error, setError] = useState('');
  const runRef = useRef<GenerationRunDetail | null>(null);
  const terminalRef = useRef(false);
  const controllerRef = useRef<AbortController | null>(null);
  const onApiErrorRef = useRef(onApiError);
  useEffect(() => { onApiErrorRef.current = onApiError; });

  const applyRun = useCallback((next: GenerationRunDetail | null) => {
    runRef.current = next;
    terminalRef.current = Boolean(next && isTerminalRunState(next.state));
    // A terminal snapshot means no further events are expected; close the stream.
    if (terminalRef.current) controllerRef.current?.abort();
    setRunState(next);
  }, []);

  const readOnce = useCallback(async (signal?: AbortSignal): Promise<GenerationRunDetail | null> => {
    if (!client || !runId) return null;
    const snapshot = normalizeRunDetail(await client.getGenerationRun(runId, signal));
    applyRun(snapshot);
    return snapshot;
  }, [applyRun, client, runId]);

  const refresh = useCallback(async (): Promise<GenerationRunDetail | null> => {
    try {
      const snapshot = await readOnce();
      if (snapshot && isTerminalRunState(snapshot.state)) setConnection('closed');
      return snapshot;
    } catch (cause) {
      if (cause instanceof GreenPassApiError && (cause.status === 401 || cause.status === 403)) {
        onApiErrorRef.current?.(cause);
        setConnection('fallback');
        setError(cause.status === 403 ? '当前账号无权限查看该生成任务。' : '登录已过期，请重新登录。');
      } else {
        setConnection('fallback');
        setError(cause instanceof Error ? cause.message : '读取生成任务失败');
      }
      return null;
    }
  }, [readOnce]);

  useEffect(() => {
    if (!client || !runId) return;
    let cancelled = false;
    const controller = new AbortController();
    controllerRef.current = controller;
    let attempt = 0;
    let lastSeq = 0;
    terminalRef.current = false;

    const sleep = (ms: number) => new Promise<void>((resolve) => {
      const timer = window.setTimeout(resolve, ms);
      const abort = () => { window.clearTimeout(timer); resolve(); };
      if (controller.signal.aborted) abort();
      else controller.signal.addEventListener('abort', abort, { once: true });
    });

    const runStream = async () => {
      setConnection('connecting');
      setError('');
      // Exactly one bootstrap snapshot. SSE then owns incremental updates.
      try {
        const snapshot = normalizeRunDetail(await client.getGenerationRun(runId, controller.signal));
        if (cancelled) return;
        applyRun(snapshot);
        if (isTerminalRunState(snapshot.state)) { setConnection('closed'); return; }
      } catch (cause) {
        if (cancelled || controller.signal.aborted) return;
        if (cause instanceof GreenPassApiError && (cause.status === 401 || cause.status === 403)) {
          onApiErrorRef.current?.(cause);
          setConnection('fallback');
          setError(cause.status === 403 ? '当前账号无权限查看该生成任务。' : '登录已过期，请重新登录。');
          return;
        }
        // Snapshot failed; the SSE stream can still deliver the run.
        setError('读取任务快照失败，正在尝试实时事件流…');
      }

      for (;;) {
        if (cancelled || controller.signal.aborted) break;
        try {
          await client.streamGenerationRunEvents(runId, {
            afterSeq: lastSeq,
            signal: controller.signal,
            onOpen: () => {
              if (cancelled) return;
              attempt = 0;
              setConnection('live');
              setError('');
            },
            onEvent: (event) => {
              if (cancelled) return;
              lastSeq = Math.max(lastSeq, event.seq ?? 0);
              const next = normalizeRunDetail(event.run);
              applyRun(next);
              if (isTerminalRunState(next.state)) {
                terminalRef.current = true;
                setConnection('closed');
                controller.abort();
              }
            },
          });
          if (cancelled || controller.signal.aborted) break;
          if (terminalRef.current) { setConnection('closed'); break; }
          attempt += 1;
        } catch (cause) {
          if (cancelled || controller.signal.aborted) break;
          if (cause instanceof GreenPassApiError && (cause.status === 401 || cause.status === 403)) {
            onApiErrorRef.current?.(cause);
            setConnection('fallback');
            setError(cause.status === 403 ? '当前账号无权限查看该生成任务。' : '登录已过期，请重新登录。');
            return;
          }
          attempt += 1;
        }
        if (attempt > MAX_RECONNECT_ATTEMPTS) {
          // Degrade gracefully: one final detail read, then stop. No polling.
          try {
            const snapshot = normalizeRunDetail(await client.getGenerationRun(runId, controller.signal));
            if (!cancelled) {
              applyRun(snapshot);
              setConnection(isTerminalRunState(snapshot.state) ? 'closed' : 'fallback');
            }
          } catch {
            if (!cancelled) setConnection('fallback');
          }
          if (!cancelled) setError('实时事件流不可用，已加载一次最新快照；不会自动轮询，可点击“刷新快照”。');
          return;
        }
        setConnection('reconnecting');
        await sleep(Math.min(1000 * 2 ** (attempt - 1), 16000));
      }
      if (!cancelled) setConnection((current) => current === 'live' || current === 'reconnecting' ? 'closed' : current);
    };

    void runStream();
    return () => {
      cancelled = true;
      controller.abort();
      if (controllerRef.current === controller) controllerRef.current = null;
    };
  }, [applyRun, client, runId]);

  return { run, setRun: applyRun, connection, error, refresh };
}

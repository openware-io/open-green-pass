import { useCallback, useEffect, useRef, useState } from 'react';
import { GreenPassApiError, type GenerationPlan, type GenerationPlanRequest, type GreenPassClient } from '@/api/client';
import { capturePlanSignature, isPlanResponseStale } from '../planSignature';
import { errorCodeOf } from '../types';

export type PlanStatus = 'idle' | 'loading' | 'ready' | 'error';

export type PlanController = {
  plan: GenerationPlan | null;
  status: PlanStatus;
  error: string;
  errorCode: string | undefined;
  errorStatus: number | undefined;
  /** True when the draft changed after this plan was produced. */
  stale: boolean;
  preview: () => Promise<GenerationPlan | null>;
  clear: () => void;
};

/**
 * Calls the server-side preflight (POST /generation/plans) for the current
 * draft. The plan is the only thing the Confirm Drawer is allowed to display;
 * it is never re-computed in the browser.
 */
export function useGenerationPlan(options: {
  client: GreenPassClient | undefined;
  request: GenerationPlanRequest | null;
  signature: string;
  onApiError?: (error: unknown) => void;
}): PlanController {
  const { client, request, signature, onApiError } = options;
  const [plan, setPlan] = useState<GenerationPlan | null>(null);
  const [status, setStatus] = useState<PlanStatus>('idle');
  const [error, setError] = useState('');
  const [errorCode, setErrorCode] = useState<string | undefined>(undefined);
  const [errorStatus, setErrorStatus] = useState<number | undefined>(undefined);
  const [plannedSignature, setPlannedSignature] = useState('');
  const requestRef = useRef(request);
  const signatureRef = useRef(signature);
  const onApiErrorRef = useRef(onApiError);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    requestRef.current = request;
    signatureRef.current = signature;
    onApiErrorRef.current = onApiError;
  });

  useEffect(() => () => abortRef.current?.abort(), []);

  const clear = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    setPlan(null);
    setStatus('idle');
    setError('');
    setErrorCode(undefined);
    setErrorStatus(undefined);
    setPlannedSignature('');
  }, []);

  const preview = useCallback(async (): Promise<GenerationPlan | null> => {
    const currentRequest = requestRef.current;
    // Capture the draft signature at request start. The response must be
    // attributed to THIS signature, never to whatever the draft looks like when
    // the response happens to arrive (that would hide a stale plan).
    const requestedSignature = capturePlanSignature(signatureRef.current);
    if (!client) {
      setStatus('error');
      setError('真实 API 未配置，无法预检生成计划。');
      setErrorCode('client_unavailable');
      return null;
    }
    if (!currentRequest || currentRequest.units.length === 0) {
      setStatus('error');
      setError('请先选择至少一个服务，并为其仓库选择分支或 Tag。');
      setErrorCode('empty_units');
      return null;
    }
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setStatus('loading');
    setError('');
    setErrorCode(undefined);
    setErrorStatus(undefined);
    try {
      const next = await client.preflightGenerationPlan(currentRequest, controller.signal);
      if (controller.signal.aborted) return null;
      setPlan(next);
      setPlannedSignature(requestedSignature);
      setStatus('ready');
      return next;
    } catch (cause) {
      if (controller.signal.aborted) return null;
      if (cause instanceof DOMException && cause.name === 'AbortError') return null;
      if (cause instanceof GreenPassApiError && (cause.status === 401 || cause.status === 403)) {
        onApiErrorRef.current?.(cause);
      }
      setPlan(null);
      setStatus('error');
      setError(cause instanceof Error ? cause.message : '预检生成计划失败');
      setErrorCode(errorCodeOf(cause));
      setErrorStatus(cause instanceof GreenPassApiError ? cause.status : undefined);
      return null;
    }
  }, [client]);

  return {
    plan,
    status,
    error,
    errorCode,
    errorStatus,
    stale: Boolean(plan) && isPlanResponseStale(plannedSignature, signature),
    preview,
    clear,
  };
}

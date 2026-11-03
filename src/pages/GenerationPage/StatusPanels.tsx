import { AlertTriangle, CheckCircle2, CircleAlert, LockKeyhole, RefreshCw, ShieldAlert } from 'lucide-react';

export type GenerationErrorKind = 'generic' | 'auth' | 'permission' | 'conflict';

export type GenerationErrorState = {
  kind: GenerationErrorKind;
  message: string;
  code?: string;
  status?: number;
};

const ERROR_TITLE: Record<GenerationErrorKind, string> = {
  generic: '操作失败',
  auth: '登录状态已失效',
  permission: '没有操作权限',
  conflict: '状态冲突',
};

const ERROR_STYLE: Record<GenerationErrorKind, string> = {
  generic: 'border-red-200 bg-red-50 text-red-700',
  auth: 'border-amber-200 bg-amber-50 text-amber-800',
  permission: 'border-amber-200 bg-amber-50 text-amber-800',
  conflict: 'border-amber-200 bg-amber-50 text-amber-800',
};

/** One centralized error banner for the whole page (no scattered error state). */
export function GenerationErrorBanner({ error, onRetry, onDismiss }: {
  error: GenerationErrorState;
  onRetry?: () => void;
  onDismiss?: () => void;
}) {
  return (
    <div role="alert" className={'mb-5 flex flex-wrap items-start gap-2 rounded-xl border px-4 py-3 text-xs ' + ERROR_STYLE[error.kind]}>
      {error.kind === 'permission' ? <LockKeyhole className="mt-0.5 h-4 w-4 shrink-0" /> : error.kind === 'auth' ? <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0" /> : <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />}
      <div className="min-w-0 flex-1">
        <div className="font-medium">{ERROR_TITLE[error.kind]}{error.status ? `（HTTP ${error.status}）` : ''}</div>
        <div className="mt-1 break-words">{error.message}</div>
        {error.code && <div className="mt-1 font-mono text-[10px] opacity-75">code: {error.code}</div>}
      </div>
      {onRetry && <button type="button" onClick={onRetry} className="inline-flex items-center gap-1 underline"><RefreshCw className="h-3 w-3" />重试</button>}
      {onDismiss && <button type="button" onClick={onDismiss} className="underline">关闭</button>}
    </div>
  );
}

/** Centralized permission / API-unavailable / empty state card. */
export function GenerationStateCard({ kind, title, description, action, onAction }: {
  kind: 'permission' | 'auth' | 'empty' | 'unavailable';
  title: string;
  description: string;
  action?: string;
  onAction?: () => void;
}) {
  const Icon = kind === 'permission' || kind === 'auth' ? LockKeyhole : kind === 'unavailable' ? CircleAlert : AlertTriangle;
  return (
    <div className="mb-5 flex flex-col items-center gap-3 rounded-xl border border-slate-200 bg-white px-6 py-12 text-center">
      <span className={'flex h-12 w-12 items-center justify-center rounded-full ' + (kind === 'empty' ? 'bg-emerald-50 text-emerald-600' : 'bg-amber-50 text-amber-600')}>
        <Icon className="h-6 w-6" />
      </span>
      <div>
        <div className="text-sm font-semibold text-slate-800">{title}</div>
        <p className="mx-auto mt-1 max-w-xl text-xs text-slate-500">{description}</p>
      </div>
      {action && onAction && (
        <button type="button" onClick={onAction} className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-xs font-medium text-white hover:bg-emerald-700">
          <CheckCircle2 className="h-3.5 w-3.5" />{action}
        </button>
      )}
    </div>
  );
}

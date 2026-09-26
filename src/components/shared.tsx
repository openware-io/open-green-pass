import { cn } from '@/lib/utils';

export interface IKpiCardProps {
  label: string;
  value: string;
  color: 'primary' | 'success' | 'warning';
  note: string;
}

const BAR: Record<string, string> = {
  primary: 'bg-emerald-500',
  success: 'bg-green-500',
  warning: 'bg-amber-500',
};

export function KpiCard({ label, value, color, note }: IKpiCardProps) {
  const [num, suffix] = value.split(/(?<=[0-9.])%?$/) as [string, string] | [string];
  return (
    <div className="card bg-white rounded-xl p-4 border border-slate-200">
      <div className="text-xs text-slate-500 mb-1">{label}</div>
      <div className="text-2xl font-bold text-slate-800">
        {num}<span className="text-base font-medium text-slate-400">{suffix ?? ''}</span>
      </div>
      <div className="mt-2 h-1.5 bg-slate-100 rounded-full overflow-hidden">
        <div className={cn('progress-bar h-full rounded-full', BAR[color])} style={{ width: `${num}%` }} />
      </div>
      <div className="text-[11px] text-slate-400 mt-2">{note}</div>
    </div>
  );
}

export function PageHeader({ title, desc, children }: { title: string; desc: string; children?: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between mb-5">
      <div>
        <h1 className="text-xl font-bold text-slate-800">{title}</h1>
        <p className="text-sm text-slate-500 mt-0.5">{desc}</p>
      </div>
      {children && <div className="flex gap-2">{children}</div>}
    </div>
  );
}

export function PrimaryButton({ children, onClick, disabled }: { children: React.ReactNode; onClick?: () => void; disabled?: boolean }) {
  return (
    <button type="button" onClick={onClick} disabled={disabled}
      className="px-3 py-1.5 text-sm bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 disabled:opacity-60 disabled:cursor-not-allowed">{children}</button>
  );
}

export function GhostButton({ children, onClick }: { children: React.ReactNode; onClick?: () => void }) {
  return (
    <button type="button" onClick={onClick}
      className="px-3 py-1.5 text-sm border border-slate-300 rounded-lg hover:bg-slate-50 bg-white">{children}</button>
  );
}

export function Card({ title, children, className, extra }: {
  title: string; children: React.ReactNode; className?: string; extra?: React.ReactNode;
}) {
  return (
    <div className={cn('card bg-white rounded-xl border border-slate-200', className)}>
      <div className="px-5 py-3.5 border-b border-slate-200 flex items-center justify-between">
        <h2 className="font-semibold text-slate-700 text-sm">{title}</h2>
        {extra}
      </div>
      {children}
    </div>
  );
}

import { ReactNode } from 'react';

export type StatusTone = 'pending' | 'success' | 'danger' | 'neutral' | 'info';

const TONES: Record<StatusTone, string> = {
    pending: 'bg-yellow-500/10 text-yellow-600 border-yellow-500/20',
    success: 'bg-green-500/10 text-green-600 border-green-500/20',
    danger: 'bg-destructive/10 text-destructive border-destructive/20',
    neutral: 'bg-secondary text-muted-foreground border-border',
    info: 'bg-sky-500/10 text-sky-600 border-sky-500/20',
};

/** Small status pill shared by review submissions and reports so their badges look identical. */
export const StatusBadge = ({ label, tone, icon }: { label: string; tone: StatusTone; icon?: ReactNode }) => (
    <span className={`px-2 py-1 rounded text-xs font-bold border flex items-center gap-1 ${TONES[tone]}`}>
        {icon}{label}
    </span>
);

export default StatusBadge;

import { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { Clock, Check, X, Ban } from 'lucide-react';
import { Report, ReportStatus } from '../types';
import { StatusBadge, StatusTone } from './StatusBadge';
import { imgCard } from '../utils/cfImage';

const REPORT_STATUS: Record<ReportStatus, { tone: StatusTone; icon: ReactNode }> = {
    [ReportStatus.Pending]: { tone: 'pending', icon: <Clock size={12} /> },
    [ReportStatus.Resolved]: { tone: 'success', icon: <Check size={12} /> },
    [ReportStatus.Rejected]: { tone: 'danger', icon: <X size={12} /> },
    [ReportStatus.Cancelled]: { tone: 'neutral', icon: <Ban size={12} /> },
};

interface ReportCardProps {
    report: Report;
    /** Show who filed the report (moderator view). Omitted on a user's own "My Reports". */
    showReporter?: boolean;
    /** Action buttons rendered in the card footer. */
    footer?: ReactNode;
}

/**
 * A report rendered in the same card style as review submissions (status badge header, image banner, reason,
 * footer slot) so the "My Reports" and moderator Reports views stay visually consistent with My Submissions.
 */
export const ReportCard = ({ report, showReporter, footer }: ReportCardProps) => {
    const s = REPORT_STATUS[report.status];
    return (
        <div className="bg-card border border-border rounded-xl overflow-hidden flex flex-col shadow-sm">
            <div className="p-4 border-b border-border">
                <div className="flex items-center gap-2 flex-wrap">
                    <StatusBadge label={report.status} tone={s.tone} icon={s.icon} />
                    <span className="text-xs font-bold text-muted-foreground uppercase tracking-wider">Report #{report.id}</span>
                </div>
                {showReporter && (
                    <p className="text-xs text-muted-foreground mt-2">
                        By:{' '}
                        {report.user ? (
                            <Link to={`/users?includedIds=${report.user.id}`} className="hover:text-primary hover:underline font-medium">{report.user.name}</Link>
                        ) : 'Unknown user'}
                    </p>
                )}
                <p className="text-xs text-muted-foreground mt-1">Submitted {new Date(report.createdAt).toLocaleDateString()}</p>
            </div>

            {report.image ? (
                <Link to={`/images/${report.imageId}`} className="block bg-muted">
                    <div className="w-full aspect-[16/10] overflow-hidden">
                        <img
                            src={imgCard(report.image.url)}
                            alt={`Reported image ${report.imageId}`}
                            className={`w-full h-full object-cover ${report.image.isNsfw ? 'blur-xl hover:blur-none transition-[filter] duration-300 transform-gpu' : ''}`}
                            loading="lazy"
                        />
                    </div>
                </Link>
            ) : (
                <div className="w-full aspect-[16/10] bg-muted flex items-center justify-center text-muted-foreground text-sm italic">Image removed</div>
            )}

            <div className="p-4 flex-1">
                <span className="font-bold text-muted-foreground uppercase text-xs tracking-wider">Reason</span>
                <p className="text-sm mt-1 break-words">{report.description || 'No description provided.'}</p>
                {report.reviewerNote && (
                    <p className="text-xs text-muted-foreground mt-3 italic break-words">Moderator: “{report.reviewerNote}”</p>
                )}
            </div>

            {footer}
        </div>
    );
};

export default ReportCard;

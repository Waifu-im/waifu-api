import { useState, useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../services/api';
import { Report, PaginatedList, ReportStatus } from '../types';
import { useNotification } from '../context/NotificationContext';
import { Check, X, Trash2, Flag } from 'lucide-react';
import ConfirmModal from '../components/modals/ConfirmModal';
import ReasonModal from '../components/modals/ReasonModal';
import Pagination from '../components/Pagination';
import ReportCard from '../components/ReportCard';
import StatusTabs from '../components/StatusTabs';
import { useRequireAuth } from '../hooks/useRequireAuth';
import { invalidateNavBadges } from '../hooks/useNavBadges';

type StatusFilter = 'All' | ReportStatus;

const STATUS_OPTIONS: { value: StatusFilter; label: string }[] = [
    { value: 'All', label: 'All' },
    { value: ReportStatus.Pending, label: 'Pending' },
    { value: ReportStatus.Resolved, label: 'Resolved' },
    { value: ReportStatus.Rejected, label: 'Rejected' },
    { value: ReportStatus.Cancelled, label: 'Cancelled' },
];

const Reports = () => {
    const user = useRequireAuth();
    const { showNotification } = useNotification();
    const queryClient = useQueryClient();
    const [reports, setReports] = useState<Report[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const [statusFilter, setStatusFilter] = useState<StatusFilter>(ReportStatus.Pending);
    const pageSize = 30;

    const [reportToDelete, setReportToDelete] = useState<Report | null>(null);
    const [actionTarget, setActionTarget] = useState<{ report: Report; action: 'validate' | 'reject' } | null>(null);

    const fetchReports = async () => {
        if (!user) return;
        setLoading(true);
        try {
            const params: Record<string, string | number> = { page, pageSize };
            if (statusFilter !== 'All') params.status = statusFilter;
            const { data } = await api.get<PaginatedList<Report>>('/reports', { params });
            setReports(data.items);
            setTotalPages(data.totalPages);
        } catch {
            // Handled globally
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (user) fetchReports();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [page, user, statusFilter]);

    useEffect(() => { setPage(1); }, [statusFilter]);

    const submitAction = async (reason: string) => {
        if (!actionTarget) return;
        const { report, action } = actionTarget;
        try {
            await api.patch(`/reports/${report.id}/${action}`, { reason: reason || undefined });
            showNotification('success', action === 'validate' ? 'Report validated' : 'Report rejected');
            setActionTarget(null);
            invalidateNavBadges(queryClient);
            fetchReports();
        } catch {
            // Handled globally
        }
    };

    const handleDeleteImage = async () => {
        if (!reportToDelete) return;
        try {
            // Deleting the image resolves its pending reports (the FK nulls their image link); they are kept.
            await api.delete(`/images/${reportToDelete.imageId}`);
            showNotification('success', 'Image deleted');
            setReportToDelete(null);
            invalidateNavBadges(queryClient);
            fetchReports();
        } catch {
            // Handled globally
        }
    };

    if (!user) return null;

    return (
        <div className="container mx-auto p-6 md:p-10 h-full flex flex-col">
            <div className="mb-6">
                <h1 className="text-3xl font-black flex items-center gap-3 text-foreground"><Flag className="text-primary" size={32}/> Reports</h1>
                <p className="text-muted-foreground mt-1">Review and act on reported images.</p>
            </div>

            <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3 mb-6">
                <StatusTabs value={statusFilter} onChange={setStatusFilter} options={STATUS_OPTIONS} />
            </div>

            {loading ? (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                    {[...Array(3)].map((_, i) => <div key={i} className="h-80 bg-muted rounded-xl animate-pulse" />)}
                </div>
            ) : reports.length === 0 ? (
                <div className="flex flex-col items-center justify-center h-64 text-muted-foreground border-2 border-dashed border-border rounded-xl bg-card/30">
                    <Check size={48} className="mb-4 opacity-20" />
                    <p className="text-lg font-medium">All clear!</p>
                    <p className="text-sm">No {statusFilter === 'All' ? '' : statusFilter.toLowerCase() + ' '}reports here.</p>
                </div>
            ) : (
                <div className="flex-1">
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        {reports.map(report => (
                            <ReportCard
                                key={report.id}
                                report={report}
                                showReporter
                                footer={report.status === ReportStatus.Pending && (
                                    <div className="p-3 border-t border-border flex gap-2 flex-wrap">
                                        <button
                                            onClick={() => setActionTarget({ report, action: 'validate' })}
                                            className="flex-1 min-w-[88px] py-2 bg-secondary hover:bg-green-500/10 hover:text-green-600 rounded-lg font-bold text-sm transition-colors flex items-center justify-center gap-2"
                                            title="The report is valid (keep image)"
                                        >
                                            <Check size={16}/> Validate
                                        </button>
                                        <button
                                            onClick={() => setActionTarget({ report, action: 'reject' })}
                                            className="flex-1 min-w-[88px] py-2 bg-secondary hover:bg-amber-500/10 hover:text-amber-600 rounded-lg font-bold text-sm transition-colors flex items-center justify-center gap-2"
                                            title="Dismiss the report (keep image)"
                                        >
                                            <X size={16}/> Reject
                                        </button>
                                        <button
                                            onClick={() => setReportToDelete(report)}
                                            className="flex-1 min-w-[88px] py-2 bg-secondary hover:bg-red-500/10 hover:text-red-600 rounded-lg font-bold text-sm transition-colors flex items-center justify-center gap-2"
                                            title="Delete the reported image"
                                        >
                                            <Trash2 size={16}/> Delete image
                                        </button>
                                    </div>
                                )}
                            />
                        ))}
                    </div>
                    <Pagination currentPage={page} totalPages={totalPages} setPage={setPage} />
                </div>
            )}

            <ConfirmModal
                isOpen={!!reportToDelete}
                onClose={() => setReportToDelete(null)}
                onConfirm={handleDeleteImage}
                title="Delete Reported Image"
                message={<>Permanently delete this image? Its reports are kept and marked resolved. This cannot be undone.</>}
                confirmText="Delete Image"
                variant="destructive"
            />

            <ReasonModal
                isOpen={!!actionTarget}
                onClose={() => setActionTarget(null)}
                onSubmit={submitAction}
                title={actionTarget?.action === 'reject' ? 'Reject report' : 'Validate report'}
                description={actionTarget?.action === 'reject'
                    ? 'Optionally tell the reporter why this report was dismissed.'
                    : 'Optionally add a note for the reporter.'}
                placeholder="Reason (optional)..."
                submitText={actionTarget?.action === 'reject' ? 'Reject' : 'Validate'}
                required={false}
                confirmVariant={actionTarget?.action === 'reject' ? 'destructive' : 'primary'}
                maxLength={500}
            />
        </div>
    );
};

export default Reports;

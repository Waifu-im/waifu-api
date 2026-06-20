import { useState, useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../services/api';
import { Report, PaginatedList, ReportStatus } from '../types';
import { useNotification } from '../context/NotificationContext';
import { Flag, Edit2, Ban, Inbox } from 'lucide-react';
import Pagination from '../components/Pagination';
import ReportCard from '../components/ReportCard';
import StatusTabs from '../components/StatusTabs';
import ReasonModal from '../components/modals/ReasonModal';
import ConfirmModal from '../components/modals/ConfirmModal';
import { useRequireAuth } from '../hooks/useRequireAuth';
import { invalidateNavBadges } from '../hooks/useNavBadges';
import { MetaTags } from '../hooks/useMetaTags';

type StatusFilter = 'All' | ReportStatus;

const STATUS_OPTIONS: { value: StatusFilter; label: string }[] = [
    { value: 'All', label: 'All' },
    { value: ReportStatus.Pending, label: 'Pending' },
    { value: ReportStatus.Resolved, label: 'Resolved' },
    { value: ReportStatus.Rejected, label: 'Rejected' },
    { value: ReportStatus.Cancelled, label: 'Cancelled' },
];

const MyReports = () => {
    const user = useRequireAuth();
    const { showNotification } = useNotification();
    const queryClient = useQueryClient();
    const [reports, setReports] = useState<Report[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const [statusFilter, setStatusFilter] = useState<StatusFilter>(ReportStatus.Pending);
    const [editing, setEditing] = useState<Report | null>(null);
    const [cancelTarget, setCancelTarget] = useState<Report | null>(null);
    const pageSize = 30;

    const fetchReports = async () => {
        if (!user) return;
        setLoading(true);
        try {
            // The endpoint auto-scopes non-moderators to their own reports.
            const params: Record<string, string | number> = { page, pageSize };
            if (statusFilter !== 'All') params.status = statusFilter;
            const { data } = await api.get<PaginatedList<Report>>('/reports', { params });
            setReports(data.items);
            setTotalPages(data.totalPages);
        } catch {
            // handled globally
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (user) fetchReports();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [page, user, statusFilter]);

    useEffect(() => { setPage(1); }, [statusFilter]);

    const handleEdit = async (description: string) => {
        if (!editing) return;
        try {
            const { data } = await api.patch<Report>(`/reports/${editing.id}`, { description });
            setReports(prev => prev.map(r => (r.id === data.id ? data : r)));
            showNotification('success', 'Report updated');
            setEditing(null);
        } catch {
            // handled globally
        }
    };

    const handleCancel = async () => {
        if (!cancelTarget) return;
        try {
            await api.patch(`/reports/${cancelTarget.id}/cancel`);
            showNotification('success', 'Report withdrawn');
            setCancelTarget(null);
            invalidateNavBadges(queryClient);
            fetchReports();
        } catch {
            // handled globally
        }
    };

    if (!user) return null;

    return (
        <div className="container mx-auto p-6 md:p-10 h-full flex flex-col">
            <MetaTags title="My Reports" description="Images you have reported." />

            <div className="mb-6">
                <h1 className="text-3xl font-black flex items-center gap-3 text-foreground">
                    <Flag className="text-primary" size={32} /> My Reports
                </h1>
                <p className="text-muted-foreground mt-1">Images you've reported and their status.</p>
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
                    <Inbox size={48} className="mb-4 opacity-20" />
                    <p className="text-lg font-medium">No reports here</p>
                    <p className="text-sm">Reports you submit on images will appear here.</p>
                </div>
            ) : (
                <div className="flex-1">
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        {reports.map(report => (
                            <ReportCard
                                key={report.id}
                                report={report}
                                footer={report.status === ReportStatus.Pending && (
                                    <div className="p-3 border-t border-border flex gap-2">
                                        <button
                                            onClick={() => setEditing(report)}
                                            className="flex-1 py-2 bg-secondary hover:bg-primary/10 hover:text-primary rounded-lg font-bold text-sm transition-colors flex items-center justify-center gap-2"
                                        >
                                            <Edit2 size={16} /> Edit
                                        </button>
                                        <button
                                            onClick={() => setCancelTarget(report)}
                                            className="flex-1 py-2 bg-secondary hover:bg-red-500/10 hover:text-red-600 rounded-lg font-bold text-sm transition-colors flex items-center justify-center gap-2"
                                        >
                                            <Ban size={16} /> Cancel
                                        </button>
                                    </div>
                                )}
                            />
                        ))}
                    </div>
                    <Pagination currentPage={page} totalPages={totalPages} setPage={setPage} />
                </div>
            )}

            <ReasonModal
                isOpen={!!editing}
                onClose={() => setEditing(null)}
                onSubmit={handleEdit}
                title="Edit Report"
                description="Update the description for this report."
                placeholder="Describe the issue with this image..."
                submitText="Save"
                confirmVariant="primary"
                maxLength={500}
                initialValue={editing?.description ?? ''}
            />

            <ConfirmModal
                isOpen={!!cancelTarget}
                onClose={() => setCancelTarget(null)}
                onConfirm={handleCancel}
                title="Withdraw Report"
                message="Withdraw this report? It will no longer be reviewed by moderators."
                confirmText="Withdraw"
                variant="destructive"
            />
        </div>
    );
};

export default MyReports;

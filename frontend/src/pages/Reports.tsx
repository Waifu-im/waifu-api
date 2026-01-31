import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import api from '../services/api';
import { Report, PaginatedList } from '../types';
import { useNotification } from '../context/NotificationContext';
import { Check, Trash2, Flag, ExternalLink } from 'lucide-react';
import ConfirmModal from '../components/modals/ConfirmModal';
import Pagination from '../components/Pagination'; // Import Pagination

const Reports = () => {
    const { showNotification } = useNotification();
    const [reports, setReports] = useState<Report[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const pageSize = 20;

    const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
    const [reportToDelete, setReportToDelete] = useState<Report | null>(null);

    const fetchReports = async () => {
        setLoading(true);
        try {
            const { data } = await api.get<PaginatedList<Report>>('/reports', {
                params: { page, pageSize, isResolved: false }
            });
            setReports(data.items);
            setTotalPages(data.totalPages);
        } catch (err: any) {
            // showNotification('error', 'Failed to load reports'); // Handled globally
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchReports();
    }, [page]);

    const handleResolve = async (id: number) => {
        try {
            await api.put(`/reports/${id}/resolve`);
            showNotification('success', 'Report resolved');
            setReports(prev => prev.filter(r => r.id !== id));
        } catch (err: any) {
            // showNotification('error', 'Failed to resolve report'); // Handled globally
        }
    };

    const handleDeleteImage = async () => {
        if (!reportToDelete) return;
        try {
            await api.delete(`/images/${reportToDelete.imageId}`);
            // Also resolve the report since the image is gone
            await api.put(`/reports/${reportToDelete.id}/resolve`);

            showNotification('success', 'Image deleted and report resolved');
            setReports(prev => prev.filter(r => r.id !== reportToDelete.id));
            setIsDeleteModalOpen(false);
            setReportToDelete(null);
        } catch (err: any) {
            // showNotification('error', 'Failed to delete image'); // Handled globally
        }
    };

    return (
        <div className="container mx-auto p-6 md:p-10 h-full flex flex-col">
            <div className="flex justify-between items-center mb-8">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground"><Flag className="text-primary" size={32}/> Reports</h1>
                    <p className="text-muted-foreground mt-1">Manage reported images.</p>
                </div>
            </div>

            {loading ? (
                <div className="flex-1 flex items-center justify-center text-muted-foreground">Loading reports...</div>
            ) : reports.length === 0 ? (
                <div className="flex flex-col items-center justify-center h-64 text-muted-foreground border-2 border-dashed border-border rounded-xl bg-card/30">
                    <Check size={48} className="mb-4 opacity-20" />
                    <p className="text-lg font-medium">All clear!</p>
                    <p className="text-sm">No pending reports.</p>
                </div>
            ) : (
                <div className="flex-1 overflow-y-auto">
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        {reports.map(report => (
                            <div key={report.id} className="bg-card border border-border rounded-xl overflow-hidden flex flex-col shadow-sm">
                                <div className="p-4 border-b border-border flex justify-between items-start">
                                    <div>
                                        <span className="text-xs font-bold text-muted-foreground uppercase tracking-wider">Report #{report.id}</span>
                                        <p className="text-sm mt-1 font-medium">{report.description || "No description provided."}</p>
                                        <p className="text-xs text-muted-foreground mt-2">
                                            By: {report.user ? (
                                            <Link to={`/users?search=${report.user.id}`} className="hover:text-primary hover:underline font-medium">
                                                {report.user.name}
                                            </Link>
                                        ) : "Unknown User"}
                                        </p>
                                        <p className="text-xs text-muted-foreground">Date: {new Date(report.createdAt).toLocaleDateString()}</p>
                                    </div>
                                </div>

                                <div className="aspect-video relative bg-muted group">
                                    {report.image ? (
                                        <>
                                            <Link to={`/images/${report.imageId}`} className="block w-full h-full">
                                                <img
                                                    src={report.image.url}
                                                    alt={`Reported Image ${report.imageId}`}
                                                    className={`w-full h-full object-cover ${report.image.isNsfw ? 'blur-md hover:blur-none transition-all duration-300' : ''}`}
                                                    loading="lazy"
                                                />
                                            </Link>
                                            <a href={report.image.url} target="_blank" rel="noreferrer" className="absolute top-2 right-2 p-1.5 bg-black/50 text-white rounded-full opacity-0 group-hover:opacity-100 transition-opacity z-10 hover:bg-black/70">
                                                <ExternalLink size={14}/>
                                            </a>
                                        </>
                                    ) : (
                                        <div className="w-full h-full flex items-center justify-center text-muted-foreground text-sm">Image not found</div>
                                    )}
                                </div>

                                <div className="p-3 border-t border-border flex gap-2">
                                    <button
                                        onClick={() => handleResolve(report.id)}
                                        className="flex-1 py-2 bg-secondary hover:bg-green-500/10 hover:text-green-600 rounded-lg font-bold text-sm transition-colors flex items-center justify-center gap-2"
                                        title="Dismiss Report (Keep Image)"
                                    >
                                        <Check size={16}/> Keep Image
                                    </button>
                                    <button
                                        onClick={() => { setReportToDelete(report); setIsDeleteModalOpen(true); }}
                                        className="flex-1 py-2 bg-secondary hover:bg-red-500/10 hover:text-red-600 rounded-lg font-bold text-sm transition-colors flex items-center justify-center gap-2"
                                        title="Delete Image"
                                    >
                                        <Trash2 size={16}/> Delete Image
                                    </button>
                                </div>
                            </div>
                        ))}
                    </div>

                    {!loading && (
                        <Pagination currentPage={page} totalPages={totalPages} setPage={setPage} />
                    )}
                </div>
            )}

            <ConfirmModal
                isOpen={isDeleteModalOpen}
                onClose={() => setIsDeleteModalOpen(false)}
                onConfirm={handleDeleteImage}
                title="Delete Reported Image"
                message={<>Are you sure you want to delete this image? This will also <strong>resolve the report</strong>.</>}
                confirmText="Delete Image"
                variant="destructive"
            />
        </div>
    );
};

export default Reports;
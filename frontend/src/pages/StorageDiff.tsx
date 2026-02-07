import { useState, useEffect } from 'react';
import api from '../services/api';
import { ImageDto, ImageFormData, PaginatedList } from '../types';
import { useNotification } from '../context/NotificationContext';
import { HardDrive, Trash2, Edit2, Check, FileQuestion, Database } from 'lucide-react';
import ImageModal from '../components/modals/ImageModal';
import ConfirmModal from '../components/modals/ConfirmModal';
import Pagination from '../components/Pagination';
import { useRequireAuth } from '../hooks/useRequireAuth';
import { useImageUpdate } from '../hooks/useImageUpdate';

interface StorageDiffDto {
    id: number | null;
    fileName: string;
    image: ImageDto | null;
}

type DiffMode = 'OrphansInS3' | 'MissingFromS3';

const defaultPageSize = 30;

const StorageDiff = () => {
    const user = useRequireAuth();
    const { showNotification } = useNotification();
    const { updateImage } = useImageUpdate();
    const [tab, setTab] = useState<DiffMode>('OrphansInS3');
    const [loading, setLoading] = useState(false);

    const [items, setItems] = useState<StorageDiffDto[]>([]);
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const [totalCount, setTotalCount] = useState(0);
    const [pageSize, setPageSize] = useState(defaultPageSize);

    const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set());
    const [confirmingBulkDelete, setConfirmingBulkDelete] = useState(false);
    const [deletingSingle, setDeletingSingle] = useState<StorageDiffDto | null>(null);
    const [editingImage, setEditingImage] = useState<ImageDto | null>(null);

    const fetchData = async () => {
        if (!user) return;
        setLoading(true);
        try {
            const { data } = await api.get<PaginatedList<StorageDiffDto>>(`/storage-diff?mode=${tab}&page=${page}&pageSize=${pageSize}`);
            setItems(data.items);
            setTotalPages(data.totalPages);
            setTotalCount(data.totalCount);
            if (data.maxPageSize > 0 && pageSize > data.maxPageSize) {
                setPageSize(data.maxPageSize);
            }
        } catch {
            // Handled globally
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        setPage(1);
        setSelectedItems(new Set());
    }, [tab]);

    useEffect(() => {
        setPage(1);
        setSelectedItems(new Set());
    }, [pageSize]);

    useEffect(() => {
        setSelectedItems(new Set());
    }, [page]);

    useEffect(() => {
        if (user) fetchData();
    }, [tab, page, pageSize, user]);

    const getItemKey = (item: StorageDiffDto) =>
        tab === 'OrphansInS3' ? item.fileName : String(item.id);

    const toggleSelect = (key: string) => {
        setSelectedItems(prev => {
            const next = new Set(prev);
            if (next.has(key)) next.delete(key);
            else next.add(key);
            return next;
        });
    };

    const toggleSelectAll = () => {
        if (selectedItems.size === items.length) {
            setSelectedItems(new Set());
        } else {
            setSelectedItems(new Set(items.map(getItemKey)));
        }
    };

    const handleBulkDelete = async () => {
        try {
            if (tab === 'OrphansInS3') {
                await api.delete('/storage-diff/s3', { data: { fileNames: Array.from(selectedItems) } });
            } else {
                await api.delete('/storage-diff/db', { data: { imageIds: Array.from(selectedItems).map(Number) } });
            }
            showNotification('success', `Deleted ${selectedItems.size} item(s)`);
            setSelectedItems(new Set());
            setConfirmingBulkDelete(false);
            fetchData();
        } catch {
            // Handled globally
        }
    };

    const handleSingleDelete = async () => {
        if (!deletingSingle) return;
        try {
            if (tab === 'OrphansInS3') {
                await api.delete('/storage-diff/s3', { data: { fileNames: [deletingSingle.fileName] } });
            } else {
                await api.delete('/storage-diff/db', { data: { imageIds: [deletingSingle.id] } });
            }
            showNotification('success', 'Item deleted');
            setDeletingSingle(null);
            fetchData();
        } catch {
            // Handled globally
        }
    };

    const handleUpdateImage = async (data: ImageFormData) => {
        if (!editingImage) return;
        try {
            await updateImage(editingImage.id, data);
            showNotification('success', 'Image updated');
            setEditingImage(null);
            fetchData();
        } catch {
            // Handled globally
        }
    };

    if (!user) return null;

    return (
        <div className="container mx-auto p-6 md:p-10 h-full flex flex-col">
            <div className="flex flex-col md:flex-row justify-between items-center mb-8 gap-4">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground">
                        <HardDrive className="text-primary" size={32} /> Storage Diff
                    </h1>
                    <p className="text-muted-foreground mt-1">Find and resolve S3 / database inconsistencies.</p>
                </div>

                <div className="flex bg-secondary p-1 rounded-xl">
                    {([['OrphansInS3', 'Orphans in S3'], ['MissingFromS3', 'Missing from S3']] as const).map(([mode, label]) => (
                        <button
                            key={mode}
                            onClick={() => setTab(mode)}
                            className={`px-6 py-2 rounded-lg text-sm font-bold transition-all ${tab === mode ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                        >
                            {label}
                        </button>
                    ))}
                </div>
            </div>

            {/* Action bar */}
            {items.length > 0 && (
                <div className="flex flex-wrap items-center gap-2 sm:gap-4 mb-4 p-3 bg-card border border-border rounded-xl sticky top-0 z-10">
                    <label className="flex items-center gap-2 cursor-pointer">
                        <input
                            type="checkbox"
                            checked={selectedItems.size === items.length && items.length > 0}
                            onChange={toggleSelectAll}
                            className="w-4 h-4 rounded border-border accent-primary"
                        />
                        <span className="text-sm font-medium text-muted-foreground whitespace-nowrap">
                            {selectedItems.size === 0
                                ? 'Select all'
                                : selectedItems.size === items.length
                                    ? `All ${items.length} selected`
                                    : `${selectedItems.size} selected`}
                        </span>
                    </label>
                    <span className="text-xs text-muted-foreground ml-auto whitespace-nowrap">
                        {totalCount} discrepanc{totalCount === 1 ? 'y' : 'ies'}
                    </span>
                    <button
                        onClick={() => setConfirmingBulkDelete(true)}
                        disabled={selectedItems.size === 0}
                        className="px-3 py-1.5 rounded-lg text-sm font-bold bg-destructive text-destructive-foreground disabled:opacity-40 disabled:cursor-not-allowed hover:opacity-90 transition-opacity whitespace-nowrap"
                    >
                        Delete selected
                    </button>
                </div>
            )}

            {loading ? (
                <div className="flex-1 flex items-center justify-center text-muted-foreground">Loading diff...</div>
            ) : items.length === 0 ? (
                <div className="flex flex-col items-center justify-center h-64 text-muted-foreground border-2 border-dashed border-border rounded-xl bg-card/30">
                    <Check size={48} className="mb-4 opacity-20" />
                    <p className="text-lg font-medium">All in sync!</p>
                    <p className="text-sm">No discrepancies found for this mode.</p>
                </div>
            ) : (
                <div className="flex-1 overflow-y-auto">
                    {tab === 'OrphansInS3' ? (
                        <div className="space-y-2">
                            {items.map(item => {
                                const key = getItemKey(item);
                                return (
                                    <div
                                        key={key}
                                        className={`flex items-center gap-4 p-4 bg-card border rounded-xl transition-all ${selectedItems.has(key) ? 'border-primary/50 bg-primary/5' : 'border-border hover:border-primary/30'}`}
                                    >
                                        <input
                                            type="checkbox"
                                            checked={selectedItems.has(key)}
                                            onChange={() => toggleSelect(key)}
                                            className="w-4 h-4 rounded border-border accent-primary shrink-0"
                                        />
                                        <FileQuestion size={20} className="text-orange-500 shrink-0" />
                                        <span className="font-mono text-sm text-foreground truncate flex-1">{item.fileName}</span>
                                        <button
                                            onClick={() => setDeletingSingle(item)}
                                            className="p-1.5 bg-secondary hover:bg-destructive hover:text-destructive-foreground rounded text-muted-foreground transition-colors shadow-sm shrink-0"
                                            title="Delete from S3"
                                        >
                                            <Trash2 size={16} />
                                        </button>
                                    </div>
                                );
                            })}
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                            {items.map(item => {
                                const key = getItemKey(item);
                                const img = item.image;
                                return (
                                    <div
                                        key={key}
                                        className={`group bg-card border rounded-xl overflow-hidden transition-all flex flex-col ${selectedItems.has(key) ? 'border-primary/50 ring-2 ring-primary/20' : 'border-border hover:shadow-md hover:border-primary/50'}`}
                                    >
                                        {/* Broken image thumbnail */}
                                        <div className="aspect-video bg-secondary/50 flex items-center justify-center relative">
                                            <input
                                                type="checkbox"
                                                checked={selectedItems.has(key)}
                                                onChange={() => toggleSelect(key)}
                                                className="absolute top-3 left-3 w-4 h-4 rounded border-border accent-primary z-10"
                                            />
                                            <Database size={32} className="text-muted-foreground/30" />
                                        </div>

                                        <div className="p-4 flex flex-col gap-2 flex-1">
                                            <div className="flex items-center justify-between">
                                                <span className="text-xs font-mono bg-secondary px-2 py-1 rounded text-muted-foreground">#{item.id}</span>
                                                <span className="text-xs text-muted-foreground">{img?.extension}</span>
                                            </div>

                                            {img && (
                                                <div className="text-xs text-muted-foreground space-y-1">
                                                    <p>{img.width} x {img.height}</p>
                                                    <p>{(img.byteSize / 1024).toFixed(1)} KB</p>
                                                    {img.tags.length > 0 && (
                                                        <div className="flex flex-wrap gap-1 mt-1">
                                                            {img.tags.slice(0, 3).map(t => (
                                                                <span key={t.id} className="px-1.5 py-0.5 bg-primary/10 text-primary rounded text-[10px]">{t.name}</span>
                                                            ))}
                                                            {img.tags.length > 3 && <span className="text-[10px] text-muted-foreground">+{img.tags.length - 3}</span>}
                                                        </div>
                                                    )}
                                                </div>
                                            )}

                                            <div className="flex gap-2 mt-auto pt-2">
                                                {img && (
                                                    <button
                                                        onClick={() => setEditingImage(img)}
                                                        className="p-1.5 bg-secondary hover:bg-primary hover:text-primary-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                                        title="Edit"
                                                    >
                                                        <Edit2 size={16} />
                                                    </button>
                                                )}
                                                <button
                                                    onClick={() => setDeletingSingle(item)}
                                                    className="p-1.5 bg-secondary hover:bg-destructive hover:text-destructive-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                                    title="Delete from DB"
                                                >
                                                    <Trash2 size={16} />
                                                </button>
                                            </div>
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    )}

                    <Pagination
                        currentPage={page}
                        totalPages={totalPages}
                        setPage={setPage}
                        pageSize={pageSize}
                        setPageSize={setPageSize}
                    />
                </div>
            )}

            {/* Edit Modal */}
            {editingImage && (
                <ImageModal
                    isOpen={!!editingImage}
                    onClose={() => setEditingImage(null)}
                    onSubmit={handleUpdateImage}
                    initialData={editingImage}
                />
            )}

            {/* Single Delete Confirmation */}
            <ConfirmModal
                isOpen={!!deletingSingle}
                onClose={() => setDeletingSingle(null)}
                onConfirm={handleSingleDelete}
                title={tab === 'OrphansInS3' ? 'Delete S3 File' : 'Delete DB Record'}
                message={
                    tab === 'OrphansInS3'
                        ? `Are you sure you want to delete "${deletingSingle?.fileName}" from S3? This action cannot be undone.`
                        : `Are you sure you want to delete image #${deletingSingle?.id} from the database? This action cannot be undone.`
                }
            />

            {/* Bulk Delete Confirmation */}
            <ConfirmModal
                isOpen={confirmingBulkDelete}
                onClose={() => setConfirmingBulkDelete(false)}
                onConfirm={handleBulkDelete}
                title={`Delete ${selectedItems.size} item(s)`}
                message={
                    tab === 'OrphansInS3'
                        ? `Are you sure you want to delete ${selectedItems.size} file(s) from S3? This action cannot be undone.`
                        : `Are you sure you want to delete ${selectedItems.size} record(s) from the database? This action cannot be undone.`
                }
            />
        </div>
    );
};

export default StorageDiff;

import { useState, ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ImageDto, Role, ImageFormData, ImageOrderBy, ReviewableContentType } from '../../types';
import { useAuth } from '../../context/AuthContext';
import { useNotification } from '../../context/NotificationContext';
import { useImageUpdate } from '../../hooks/useImageUpdate';
import api from '../../services/api';
import ImageGrid from '../ImageGrid';
import Pagination from '../Pagination';
import FilterSidebar from '../FilterSidebar';
import FilterToggleButton from '../FilterToggleButton';
import FloatingRefreshButton from '../FloatingRefreshButton';
import ConfirmModal from '../modals/ConfirmModal';
import ImageModal from '../modals/ImageModal';
import { Option } from '../SearchableSelect';
import { Search, Tags } from 'lucide-react';
import { useTagView } from '../../hooks/useTagView';

interface GalleryLayoutProps {
    images: ImageDto[];
    isLoading: boolean;
    error?: any;
    refetch: () => void;
    page: number;
    totalPages: number;
    setPage: (page: number) => void;
    searchParams: URLSearchParams;
    setSearchParams: (params: URLSearchParams | ((prev: URLSearchParams) => URLSearchParams)) => void;
    headerContent?: ReactNode;
    sortOptions?: Option[];
    emptyState?: ReactNode;
    onRemoveFromAlbum?: (id: number) => void;
    readOnly?: boolean;
}

export const GalleryLayout = ({
                                  images,
                                  isLoading,
                                  error,
                                  refetch,
                                  page,
                                  totalPages,
                                  setPage,
                                  searchParams,
                                  setSearchParams,
                                  headerContent,
                                  sortOptions,
                                  emptyState,
                                  onRemoveFromAlbum,
                                  readOnly = false
                              }: GalleryLayoutProps) => {
    const { user } = useAuth();
    const { showNotification } = useNotification();
    const { updateImage } = useImageUpdate();
    const queryClient = useQueryClient();

    const { showTags, toggle: toggleTags } = useTagView();

    const [showFilters, setShowFilters] = useState(false);
    const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
    const [imageToDelete, setImageToDelete] = useState<number | null>(null);
    const [editingImage, setEditingImage] = useState<ImageDto | null>(null);
    const [isEditModalOpen, setIsEditModalOpen] = useState(false);
    const [suggestingImage, setSuggestingImage] = useState<ImageDto | null>(null);
    const [isSuggestModalOpen, setIsSuggestModalOpen] = useState(false);

    const isAdmin = user?.role === Role.Admin;
    const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);

    const confirmDelete = (id: number) => {
        setImageToDelete(id);
        setIsDeleteModalOpen(true);
    };

    const handleRealDelete = async () => {
        if (!imageToDelete) return;
        try {
            await api.delete(`/images/${imageToDelete}`);
            refetch();
            setIsDeleteModalOpen(false);
            setImageToDelete(null);
            showNotification('success', 'Image deleted');
        } catch(e) { }
    };

    const handleSaveEdit = async (data: ImageFormData) => {
        if (!editingImage) return;
        try {
            await updateImage(editingImage.id, data);
            showNotification('success', 'Image updated');
            refetch();
            setIsEditModalOpen(false);
            setEditingImage(null);
        } catch (err) { }
    };

    const defaultSortOptions = [
        { id: ImageOrderBy.Random, name: 'Random Shuffle' },
        { id: ImageOrderBy.UploadedAt, name: 'Newest First' },
        { id: ImageOrderBy.Favorites, name: 'Most Popular' }
    ];

    const currentSort = searchParams.get('orderBy');

    return (
        <div className="flex h-full relative overflow-hidden">
            <div className="flex-1 flex flex-col relative min-w-0 overflow-hidden">
                <FloatingRefreshButton onRefresh={refetch} />

                <div className="flex-1 overflow-y-auto p-4 md:p-6">
                    <div className="flex flex-col md:flex-row md:items-start justify-between px-4 md:px-6 py-4 border-b border-border backdrop-blur-xl z-10 shrink-0 gap-4 -mx-4 md:-mx-6 -mt-4 md:-mt-6 mb-4">
                        <div className="min-w-0 flex-1">
                            {headerContent || (
                                <h1 className="text-3xl font-black tracking-tight text-foreground flex items-center gap-2 mt-1">
                                    Gallery
                                </h1>
                            )}
                        </div>
                        <div className="flex items-center self-end md:self-start shrink-0 gap-2">
                            <button
                                onClick={toggleTags}
                                title={showTags ? 'Hide tags' : 'Show tags'}
                                aria-pressed={showTags}
                                className={`p-2.5 rounded-lg border transition-colors ${showTags ? 'bg-primary text-primary-foreground border-primary' : 'bg-card text-muted-foreground border-border hover:text-foreground'}`}
                            >
                                <Tags size={20} />
                            </button>
                            <FilterToggleButton isOpen={showFilters} onToggle={() => setShowFilters(!showFilters)} />
                        </div>
                    </div>

                    <ImageGrid
                        images={images}
                        isLoading={isLoading}
                        error={error}
                        onRetry={refetch}
                        onDelete={isAdmin ? confirmDelete : undefined}
                        onEdit={isAdminOrModerator ? (img) => { setEditingImage(img); setIsEditModalOpen(true); } : undefined}
                        onSuggest={user && !isAdminOrModerator ? (img) => { setSuggestingImage(img); setIsSuggestModalOpen(true); } : undefined}
                        onRemove={onRemoveFromAlbum}
                        readOnly={readOnly}
                        showTags={showTags}
                        emptyState={emptyState || (
                            <div className="flex flex-col items-center justify-center h-[50vh] text-muted-foreground">
                                <Search size={48} className="mb-4 opacity-20" />
                                <p className="text-lg font-medium">No images found.</p>
                                <button
                                    onClick={() => { setSearchParams(new URLSearchParams()); refetch(); }}
                                    className="mt-6 px-6 py-2 bg-secondary text-foreground rounded-lg font-bold"
                                >
                                    Clear All Filters
                                </button>
                            </div>
                        )}
                    />

                    <Pagination
                        currentPage={page}
                        totalPages={totalPages}
                        setPage={setPage}
                        disabled={currentSort === ImageOrderBy.Random}
                    />
                </div>
            </div>

            <FilterSidebar
                searchParams={searchParams}
                setSearchParams={setSearchParams}
                showFilters={showFilters}
                setShowFilters={setShowFilters}
                sortOptions={sortOptions || defaultSortOptions}
            />

            <ConfirmModal
                isOpen={isDeleteModalOpen}
                onClose={() => setIsDeleteModalOpen(false)}
                onConfirm={handleRealDelete}
                title="Delete Image"
                message="Permanently delete this image? This cannot be undone."
                confirmText="Delete"
                variant="destructive"
            />

            {editingImage && (
                <ImageModal
                    isOpen={isEditModalOpen}
                    onClose={() => { setIsEditModalOpen(false); setEditingImage(null); }}
                    initialData={editingImage}
                    onSubmit={handleSaveEdit}
                />
            )}

            {suggestingImage && (
                <ImageModal
                    isOpen={isSuggestModalOpen}
                    onClose={() => {
                        setIsSuggestModalOpen(false);
                        setSuggestingImage(null);
                        // A suggestion may have just been submitted, so refresh the caller's pending proposals.
                        queryClient.invalidateQueries({ queryKey: ['my-pending-image-edits'] });
                    }}
                    initialData={suggestingImage}
                    onSubmit={() => {}}
                    suggestContext={{ targetType: ReviewableContentType.Image, targetId: suggestingImage.id, current: suggestingImage }}
                />
            )}
        </div>
    );
};
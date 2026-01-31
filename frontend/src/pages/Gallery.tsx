import { useState, useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useImages } from '../hooks/useImages';
import { Search } from 'lucide-react';
import api from '../services/api';
import { ImageDto, Role, ImageFormData, ImageSort } from '../types';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import ImageModal from '../components/modals/ImageModal';
import FilterSidebar from '../components/FilterSidebar';
import ConfirmModal from '../components/modals/ConfirmModal';
import ImageGrid from '../components/ImageGrid';
import FloatingRefreshButton from '../components/FloatingRefreshButton';
import FilterToggleButton from '../components/FilterToggleButton';
import Pagination from '../components/Pagination'; // Import Pagination

const Gallery = () => {
    const [searchParams, setSearchParams] = useSearchParams();
    const [showFilters, setShowFilters] = useState(false);
    const [showNsfwWarning, setShowNsfwWarning] = useState(false);
    const { user } = useAuth();
    const { showNotification } = useNotification();

    const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
    const [imageToDelete, setImageToDelete] = useState<number | null>(null);
    const [editingImage, setEditingImage] = useState<ImageDto | null>(null);
    const [isEditModalOpen, setIsEditModalOpen] = useState(false);

    // Extract Params
    const isNsfw = searchParams.get('isNsfw') || '0';
    const orderBy = searchParams.get('orderBy') || ImageSort.RANDOM;
    // ... autres params inchangés
    const orientation = searchParams.get('orientation') || '';
    const isAnimatedStr = searchParams.get('isAnimated');
    const height = searchParams.get('height') || '';
    const width = searchParams.get('width') || '';
    const byteSize = searchParams.get('byteSize') || '';
    const includedTags = searchParams.getAll('includedTags');
    const excludedTags = searchParams.getAll('excludedTags');
    const includedArtists = searchParams.getAll('includedArtists');
    const excludedArtists = searchParams.getAll('excludedArtists');
    const includedIds = searchParams.getAll('includedIds');
    const excludedIds = searchParams.getAll('excludedIds');

    const pageStr = searchParams.get('page');
    const page = pageStr ? parseInt(pageStr) : 1;

    // --- LOGIC: RESET PAGINATION ---
    const prevFiltersRef = useRef(JSON.stringify({
        isNsfw, orderBy, orientation, isAnimatedStr, height, width, byteSize,
        includedTags, excludedTags, includedArtists, excludedArtists
    }));

    useEffect(() => {
        const currentFilters = JSON.stringify({
            isNsfw, orderBy, orientation, isAnimatedStr, height, width, byteSize,
            includedTags, excludedTags, includedArtists, excludedArtists
        });

        if (prevFiltersRef.current !== currentFilters) {
            if (page !== 1) {
                setSearchParams(prev => { prev.set('page', '1'); return prev; });
            }
            prevFiltersRef.current = currentFilters;
        }
    }, [
        isNsfw, orderBy, orientation, isAnimatedStr, height, width, byteSize,
        includedTags, excludedTags, includedArtists, excludedArtists,
        page, setSearchParams
    ]);

    useEffect(() => {
        if (!searchParams.has('orderBy')) {
            setSearchParams(prev => { prev.set('orderBy', ImageSort.RANDOM); return prev; }, { replace: true });
        }
    }, [searchParams, setSearchParams]);

    const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);
    const isAdmin = user?.role === Role.Admin;
    const isAnimatedBool = isAnimatedStr === 'true' ? true : isAnimatedStr === 'false' ? false : undefined;

    const { data: paginatedImages, isLoading, error, refetch } = useImages({
        isNsfw: parseInt(isNsfw),
        orderBy,
        // ... autres props
        orientation,
        isAnimated: isAnimatedBool,
        width,
        height,
        byteSize,
        includedTags,
        excludedTags,
        includedArtists,
        excludedArtists,
        includedIds,
        excludedIds,
        page: page,
        pageSize: 50
    });

    const images = paginatedImages?.items || [];
    const totalPages = paginatedImages?.totalPages || 1;

    useEffect(() => {
        if (isNsfw !== '0' && !localStorage.getItem('nsfw-consent')) {
            setShowNsfwWarning(true);
        }
    }, [isNsfw]);

    // Handlers
    const confirmDelete = (id: number) => { setImageToDelete(id); setIsDeleteModalOpen(true); };
    const handleRealDelete = async () => { if (!imageToDelete) return; try { await api.delete(`/images/${imageToDelete}`); refetch(); setIsDeleteModalOpen(false); setImageToDelete(null); showNotification('success', 'Image deleted'); } catch(e) { } };
    const handleSaveEdit = async (data: ImageFormData) => { if (!editingImage) return; try { const payload = { source: data.source || null, isNsfw: data.isNsfw, userId: data.userId || null, tags: data.tags || [], artists: data.artists || [], reviewStatus: data.reviewStatus }; await api.put<ImageDto>(`/images/${editingImage.id}`, payload); showNotification('success', 'Image updated'); refetch(); setIsEditModalOpen(false); setEditingImage(null); } catch (err) { } };

    const handlePageChange = (newPage: number) => {
        setSearchParams(prev => {
            prev.set('page', newPage.toString());
            return prev;
        });
    };

    const sortOptions = [
        { id: ImageSort.RANDOM, name: 'Random Shuffle' },
        { id: ImageSort.UPLOADED_AT, name: 'Newest First' },
        { id: ImageSort.FAVORITES, name: 'Most Popular' }
    ];

    return (
        <div className="flex h-full relative overflow-hidden">
            <div className="flex-1 flex flex-col relative min-w-0 overflow-hidden">
                <FloatingRefreshButton onRefresh={() => refetch()} />

                <div className="flex-1 overflow-y-auto p-4 md:p-6">
                    <div className="flex justify-between items-start px-4 md:px-6 py-4 border-b border-border backdrop-blur-xl z-10 shrink-0 gap-4 -mx-4 md:-mx-6 -mt-4 md:-mt-6 mb-4">
                        <h1 className="text-3xl font-black tracking-tight text-foreground flex items-center gap-2 mt-1">
                            Gallery
                        </h1>
                        <div className="flex items-center self-end md:self-start shrink-0">
                            <FilterToggleButton isOpen={showFilters} onToggle={() => setShowFilters(!showFilters)} />
                        </div>
                    </div>

                    <ImageGrid
                        images={images}
                        isLoading={isLoading}
                        error={error}
                        onRetry={() => refetch()}
                        onDelete={isAdmin ? confirmDelete : undefined}
                        onEdit={isAdminOrModerator ? (img) => { setEditingImage(img); setIsEditModalOpen(true); } : undefined}
                        emptyState={
                            <div className="flex flex-col items-center justify-center h-[50vh] text-muted-foreground">
                                <Search size={48} className="mb-4 opacity-20" />
                                <p className="text-lg font-medium">No images found.</p>
                                <button onClick={() => { setSearchParams({}); refetch(); }} className="mt-6 px-6 py-2 bg-secondary text-foreground rounded-lg font-bold">Clear All Filters</button>
                            </div>
                        }
                    />

                    {/* AJOUT PAGINATION */}
                    <Pagination
                        currentPage={page}
                        totalPages={totalPages}
                        setPage={handlePageChange}
                        disabled={orderBy === ImageSort.RANDOM} // Désactivé si Random
                    />
                </div>
            </div>

            <FilterSidebar searchParams={searchParams} setSearchParams={setSearchParams} showFilters={showFilters} setShowFilters={setShowFilters} sortOptions={sortOptions} />
            <ConfirmModal isOpen={isDeleteModalOpen} onClose={() => setIsDeleteModalOpen(false)} onConfirm={handleRealDelete} title="Delete Image" message="Permanently delete this image? This cannot be undone." confirmText="Delete" variant="destructive" />
            {editingImage && <ImageModal isOpen={isEditModalOpen} onClose={() => { setIsEditModalOpen(false); setEditingImage(null); }} initialData={editingImage} onSubmit={handleSaveEdit} />}
            {showNsfwWarning && (
                <div className="fixed inset-0 z-[60] bg-black/90 flex items-center justify-center p-4 backdrop-blur-md">
                    <div className="bg-card p-8 rounded-2xl max-w-sm w-full text-center border border-border shadow-2xl">
                        <h2 className="text-2xl font-black mb-2">Age Restricted</h2>
                        <button onClick={() => { localStorage.setItem('nsfw-consent', 'true'); setShowNsfwWarning(false); }} className="w-full py-3 bg-primary text-primary-foreground rounded-xl font-bold mb-2">I am 18+</button>
                        <button onClick={() => { setSearchParams(prev => { prev.set('isNsfw', '0'); return prev; }); setShowNsfwWarning(false); }} className="w-full py-3 bg-secondary rounded-xl font-bold">Go Back</button>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Gallery;
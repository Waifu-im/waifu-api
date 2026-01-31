import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useImages } from '../hooks/useImages';
import { SlidersHorizontal, Search, RefreshCw } from 'lucide-react';
import api from '../services/api';
import { ImageDto, Role, ImageFormData, ImageSort } from '../types';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import ImageModal from '../components/modals/ImageModal';
import FilterSidebar from '../components/FilterSidebar';
import ConfirmModal from '../components/modals/ConfirmModal';
import ImageGrid from '../components/ImageGrid';

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

    useEffect(() => {
        if (!searchParams.has('orderBy')) {
            setSearchParams(prev => {
                prev.set('orderBy', ImageSort.RANDOM);
                return prev;
            }, { replace: true });
        }
    }, [searchParams, setSearchParams]);

    // Extract Params
    const isNsfw = searchParams.get('isNsfw') || '0';
    const orderBy = searchParams.get('orderBy') || ImageSort.RANDOM;
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

    const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);
    const isAdmin = user?.role === Role.Admin;
    const isAnimatedBool = isAnimatedStr === 'true' ? true : isAnimatedStr === 'false' ? false : undefined;

    const { data: paginatedImages, isLoading, error, refetch } = useImages({
        isNsfw: parseInt(isNsfw),
        orderBy,
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
    const setPage = (newPage: number) => { setSearchParams(prev => { prev.set('page', newPage.toString()); return prev; }); };

    const sortOptions = [
        { id: ImageSort.RANDOM, name: 'Random Shuffle' },
        { id: ImageSort.UPLOADED_AT, name: 'Newest First' },
        { id: ImageSort.FAVORITES, name: 'Most Popular' }
    ];

    return (
        <div className="flex h-full relative overflow-hidden">
            {/* Custom Scrollbar Styles */}
            <style>{`
            .custom-scrollbar::-webkit-scrollbar { width: 6px; }
            .custom-scrollbar::-webkit-scrollbar-track { background: transparent; }
            .custom-scrollbar::-webkit-scrollbar-thumb { background-color: rgba(156, 163, 175, 0.5); border-radius: 20px; }
            .custom-scrollbar::-webkit-scrollbar-thumb:hover { background-color: rgba(156, 163, 175, 0.8); }
        `}</style>

            {/* WRAPPER */}
            <div className="flex-1 flex flex-col relative min-w-0 overflow-hidden">

                {/* BUTTONS: Mobile (top-3) vs Desktop (top-13px) */}
                <div className="absolute top-3 right-3 md:top-[13px] md:right-6 z-50 flex items-center gap-2 bg-background/80 backdrop-blur-md p-2 rounded-xl border border-border shadow-sm">
                    <button
                        onClick={() => refetch()}
                        className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
                        title="Refresh"
                    >
                        <RefreshCw size={16} />
                    </button>
                    <button onClick={() => setShowFilters(!showFilters)} className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium hover:bg-secondary transition-colors">
                        <SlidersHorizontal size={16} /> <span className="hidden sm:inline">{showFilters ? 'Hide' : 'Filters'}</span>
                    </button>
                </div>

                {/* SCROLL AREA with custom-scrollbar class */}
                <div className="flex-1 overflow-y-auto p-4 md:p-6 custom-scrollbar">

                    {/* Header Content */}
                    <div className="flex justify-between items-start px-4 md:px-6 py-4 border-b border-border backdrop-blur-xl z-10 shrink-0 gap-4 -mx-4 md:-mx-6 -mt-4 md:-mt-6 mb-4">
                        <h1 className="text-3xl font-black tracking-tight text-foreground flex items-center gap-2 mt-1">
                            Gallery
                        </h1>
                        {/* Spacer */}
                        <div className="hidden md:block w-32 shrink-0 h-1"></div>
                    </div>

                    {/* Grid */}
                    <ImageGrid
                        images={images}
                        isLoading={isLoading}
                        error={error}
                        onRetry={() => refetch()}
                        page={page}
                        totalPages={totalPages}
                        setPage={setPage}
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
                </div>
            </div>

            {/* Sidebar */}
            <FilterSidebar
                searchParams={searchParams}
                setSearchParams={setSearchParams}
                showFilters={showFilters}
                setShowFilters={setShowFilters}
                sortOptions={sortOptions}
            />

            {/* Modals... */}
            <ConfirmModal isOpen={isDeleteModalOpen} onClose={() => setIsDeleteModalOpen(false)} onConfirm={handleRealDelete} title="Delete Image" message="Permanently delete this image? This cannot be undone." confirmText="Delete" variant="destructive" />
            {editingImage && (<ImageModal isOpen={isEditModalOpen} onClose={() => { setIsEditModalOpen(false); setEditingImage(null); }} initialData={editingImage} onSubmit={handleSaveEdit} />)}
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
import { useEffect, useState } from 'react';
import { useParams, Link, useSearchParams, useNavigate } from 'react-router-dom';
import api from '../services/api';
import { ImageDto, PaginatedList, AlbumDto, ImageFormData, Role, ImageSort } from '../types';
import { ChevronLeft, FolderOpen, Edit2, Trash2, FolderMinus, SlidersHorizontal } from 'lucide-react';
import { useNotification } from '../context/NotificationContext';
import { useAuth } from '../context/AuthContext';
import Modal from '../components/Modal';
import ImageModal from '../components/modals/ImageModal';
import ConfirmModal from '../components/modals/ConfirmModal';
import FilterSidebar from '../components/FilterSidebar';
import ImageGrid from '../components/ImageGrid';

const AlbumPage = () => {
    const { id } = useParams<{ id: string }>();
    const { user } = useAuth();
    const navigate = useNavigate();
    const [searchParams, setSearchParams] = useSearchParams();
    const { showNotification } = useNotification();

    const [album, setAlbum] = useState<AlbumDto | null>(null);
    const [images, setImages] = useState<ImageDto[]>([]);
    const [loading, setLoading] = useState(true);
    const [showFilters, setShowFilters] = useState(false);

    // Modals
    const [isEditAlbumOpen, setIsEditAlbumOpen] = useState(false);
    const [editAlbumFormData, setEditAlbumFormData] = useState({ name: '', description: '' });
    const [isDeleteAlbumOpen, setIsDeleteAlbumOpen] = useState(false);

    // Image Actions
    const [imageToRemove, setImageToRemove] = useState<number | null>(null);
    const [editingImage, setEditingImage] = useState<ImageDto | null>(null);
    const [isEditImageOpen, setIsEditImageOpen] = useState(false);

    const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);

    useEffect(() => {
        if (!searchParams.has('orderBy')) {
            setSearchParams(prev => {
                prev.set('orderBy', ImageSort.ADDED_TO_ALBUM);
                return prev;
            }, { replace: true });
        }
    }, [searchParams, setSearchParams]);

    const fetchAlbumData = async () => {
        if (!id || !user) return;
        setLoading(true);
        try {
            const infoRes = await api.get<AlbumDto>(`/users/me/albums/${id}`);
            setAlbum(infoRes.data);

            const params = new URLSearchParams(searchParams);
            params.set('pageSize', '50');
            params.set('page', '1');

            const imgsRes = await api.get<PaginatedList<ImageDto>>(`/users/me/albums/${id}/images`, { params });
            if (imgsRes.data && Array.isArray(imgsRes.data.items)) {
                setImages(imgsRes.data.items);
            } else {
                setImages([]);
            }
        } catch (error) {
            console.error(error);
            // showNotification('error', 'Failed to load album'); // Handled globally
            setImages([]);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchAlbumData();
    }, [id, searchParams, user]);

    const handleRemoveImage = async () => {
        if (!id || !imageToRemove) return;
        try {
            await api.delete(`/users/me/albums/${id}/images/${imageToRemove}`);
            setImages(prev => prev.filter(img => img.id !== imageToRemove));
            showNotification('success', 'Image removed from album');
            setImageToRemove(null);
        } catch(e) {
            // Error handled globally
        }
    };

    const handleDeleteAlbum = async () => {
        if (!album) return;
        try {
            await api.delete(`/users/me/albums/${album.id}`);
            showNotification('success', 'Album deleted');
            setIsDeleteAlbumOpen(false);
            navigate('/albums');
        } catch (e) {
            // Error handled globally
        }
    };

    const handleEditAlbum = async () => {
        if (!album) return;
        try {
            const { data } = await api.put<AlbumDto>(`/users/me/albums/${album.id}`, editAlbumFormData);
            setAlbum(data);
            setIsEditAlbumOpen(false);
            showNotification('success', 'Album updated');
        } catch (e) {
            // Error handled globally
        }
    };

    const handleSaveImageEdit = async (data: ImageFormData) => {
        if (!editingImage) return;
        try {
            const payload = {
                source: data.source || null,
                isNsfw: data.isNsfw,
                userId: data.userId || null,
                tags: data.tags || [],
                artists: data.artists || []
            };
            const { data: updatedImage } = await api.put<ImageDto>(`/images/${editingImage.id}`, payload);
            setImages(prev => prev.map(img => img.id === updatedImage.id ? updatedImage : img));
            showNotification('success', 'Image updated');
            setIsEditImageOpen(false);
            setEditingImage(null);
        } catch (err) {
            // Error handled globally
        }
    };

    const sortOptions = [
        { id: ImageSort.ADDED_TO_ALBUM, name: 'Date Added' },
        { id: ImageSort.UPLOADED_AT, name: 'Newest Upload' },
        { id: ImageSort.FAVORITES, name: 'Most Popular' },
        { id: ImageSort.RANDOM, name: 'Random Shuffle' },
    ];

    if (!user) return <div className="p-10 text-center">Please log in.</div>;
    if (loading && !album) return <div className="p-10 text-center">Loading album...</div>;

    return (
        <div className="flex h-full relative overflow-hidden">
            <div className="flex-1 overflow-y-auto h-full p-4 md:p-6">
                <div className="flex flex-col md:flex-row md:items-center justify-between mb-8 gap-4">
                    <div className="flex items-center gap-4">
                        <Link to="/albums" className="p-2 rounded-full bg-secondary hover:bg-secondary/80 transition-colors">
                            <ChevronLeft size={24} />
                        </Link>
                        <div>
                            <div className="flex items-center gap-3">
                                <h1 className="text-3xl font-black flex items-center gap-2 text-foreground">
                                    <FolderOpen className="text-primary" /> {album?.name || "Album"}
                                </h1>
                                <button
                                    onClick={() => { setEditAlbumFormData({ name: album?.name || '', description: album?.description || '' }); setIsEditAlbumOpen(true); }}
                                    className="p-1.5 rounded-full hover:bg-secondary text-muted-foreground hover:text-foreground"
                                    title="Edit Album"
                                >
                                    <Edit2 size={16} />
                                </button>
                                <button
                                    onClick={() => setIsDeleteAlbumOpen(true)}
                                    className="p-1.5 rounded-full hover:bg-destructive/10 text-muted-foreground hover:text-destructive transition-colors"
                                    title="Delete Album"
                                >
                                    <Trash2 size={16} />
                                </button>
                            </div>
                            <p className="text-muted-foreground mt-1">{album?.description || `${images?.length || 0} images`}</p>
                        </div>
                    </div>

                    <button
                        onClick={() => setShowFilters(!showFilters)}
                        className="flex items-center gap-2 px-4 py-2 bg-card border border-border rounded-lg text-sm font-medium shadow-sm hover:bg-secondary transition-colors self-start md:self-auto"
                    >
                        <SlidersHorizontal size={16} />
                        <span className="hidden sm:inline">{showFilters ? 'Hide' : 'Filters'}</span>
                    </button>
                </div>

                <ImageGrid
                    images={images}
                    isLoading={loading && !album}
                    page={1} // Pagination handled by parent for now or simplified
                    totalPages={1} // Simplified for album view as it loads all or handles differently
                    setPage={() => {}} // No-op for now if single page or infinite scroll
                    onRemove={(id) => setImageToRemove(id)}
                    onEdit={isAdminOrModerator ? (img) => { setEditingImage(img); setIsEditImageOpen(true); } : undefined}
                    emptyState={
                        <div className="flex flex-col items-center justify-center h-64 text-muted-foreground border-2 border-dashed border-border rounded-xl bg-card/50">
                            <p className="text-lg font-medium">This album is empty.</p>
                            <Link to="/gallery" className="mt-2 text-primary font-bold hover:underline">Browse Gallery to add images</Link>
                        </div>
                    }
                />
            </div>

            <FilterSidebar
                searchParams={searchParams}
                setSearchParams={setSearchParams}
                showFilters={showFilters}
                setShowFilters={setShowFilters}
                sortOptions={sortOptions}
            />

            <Modal isOpen={isEditAlbumOpen} onClose={() => setIsEditAlbumOpen(false)} title="Edit Album Details">
                <div className="space-y-4">
                    <div><label className="font-bold block mb-1">Name</label><input value={editAlbumFormData.name} onChange={e => setEditAlbumFormData({...editAlbumFormData, name: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none"/></div>
                    <div><label className="font-bold block mb-1">Description</label><textarea value={editAlbumFormData.description} onChange={e => setEditAlbumFormData({...editAlbumFormData, description: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none h-24"/></div>
                    <button onClick={handleEditAlbum} className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg">Save</button>
                </div>
            </Modal>

            <ConfirmModal
                isOpen={!!imageToRemove}
                onClose={() => setImageToRemove(null)}
                onConfirm={handleRemoveImage}
                title="Remove Image"
                message="Remove this image from the album? It will remain in the gallery."
                confirmText="Remove"
                variant="warning"
                icon={FolderMinus}
            />

            <ConfirmModal
                isOpen={isDeleteAlbumOpen}
                onClose={() => setIsDeleteAlbumOpen(false)}
                onConfirm={handleDeleteAlbum}
                title="Delete Album"
                message={
                    <div>
                        <p>Are you sure you want to delete the album <strong>{album?.name}</strong>?</p>
                        <p className="text-sm text-muted-foreground mt-1">The images inside will NOT be deleted from the gallery.</p>
                    </div>
                }
                confirmText="Delete Album"
                variant="destructive"
            />

            {editingImage && (
                <ImageModal
                    isOpen={isEditImageOpen}
                    onClose={() => { setIsEditImageOpen(false); setEditingImage(null); }}
                    initialData={editingImage}
                    onSubmit={handleSaveImageEdit}
                />
            )}
        </div>
    );
};

export default AlbumPage;
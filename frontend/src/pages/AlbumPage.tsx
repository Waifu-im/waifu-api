import { useEffect, useState, useRef } from 'react';
import { useParams, Link, useSearchParams, useNavigate } from 'react-router-dom';
import api from '../services/api';
import { ImageDto, PaginatedList, AlbumDto, ImageFormData, Role, ImageSort } from '../types';
import { ChevronLeft, FolderOpen, Edit2, Trash2, FolderMinus } from 'lucide-react';
import { useNotification } from '../context/NotificationContext';
import { useAuth } from '../context/AuthContext';
import Modal from '../components/Modal';
import ImageModal from '../components/modals/ImageModal';
import ConfirmModal from '../components/modals/ConfirmModal';
import FilterSidebar from '../components/FilterSidebar';
import ImageGrid from '../components/ImageGrid';
import FloatingRefreshButton from '../components/FloatingRefreshButton';
import FilterToggleButton from '../components/FilterToggleButton';
import Pagination from '../components/Pagination'; // Import Pagination

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
    const [totalPages, setTotalPages] = useState(1);

    // Modals
    const [isEditAlbumOpen, setIsEditAlbumOpen] = useState(false);
    const [editAlbumFormData, setEditAlbumFormData] = useState({ name: '', description: '' });
    const [isDeleteAlbumOpen, setIsDeleteAlbumOpen] = useState(false);
    const [imageToRemove, setImageToRemove] = useState<number | null>(null);
    const [editingImage, setEditingImage] = useState<ImageDto | null>(null);
    const [isEditImageOpen, setIsEditImageOpen] = useState(false);

    const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);

    // Pagination Params
    const pageStr = searchParams.get('page');
    const page = pageStr ? parseInt(pageStr) : 1;
    const orderBy = searchParams.get('orderBy') || ImageSort.ADDED_TO_ALBUM;

    // --- CORRECTION DU RESET PAGE ---
    // On utilise un Ref pour stocker l'ancienne valeur de orderBy
    const prevOrderBy = useRef(orderBy);

    useEffect(() => {
        // Si l'ordre de tri change
        if (prevOrderBy.current !== orderBy) {
            // On force le retour à la page 1 directement dans l'URL
            setSearchParams(prev => {
                prev.set('page', '1');
                return prev;
            });
            // On met à jour la ref pour le prochain changement
            prevOrderBy.current = orderBy;
        }
    }, [orderBy, setSearchParams]);
    // --------------------------------

    useEffect(() => {
        if (!user) {
            showNotification('warning', 'You must be logged in to view albums.');
            navigate('/login');
            return;
        }
    }, [user, navigate, showNotification]);

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
            if (!params.has('page')) params.set('page', '1');

            const imgsRes = await api.get<PaginatedList<ImageDto>>(`/users/me/albums/${id}/images`, { params });
            if (imgsRes.data && Array.isArray(imgsRes.data.items)) {
                setImages(imgsRes.data.items);
                setTotalPages(imgsRes.data.totalPages);
            } else {
                setImages([]);
                setTotalPages(1);
            }
        } catch (error) {
            console.error(error);
            setImages([]);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchAlbumData();
    }, [id, searchParams, user]);

    // Handlers
    const handleRemoveImage = async () => { if (!id || !imageToRemove) return; try { await api.delete(`/users/me/albums/${id}/images/${imageToRemove}`); setImages(prev => prev.filter(img => img.id !== imageToRemove)); if (album) { setAlbum({ ...album, imageCount: Math.max(0, (album.imageCount || 0) - 1) }); } showNotification('success', 'Image removed from album'); setImageToRemove(null); } catch(e) { } };
    const handleDeleteAlbum = async () => { if (!album) return; try { await api.delete(`/users/me/albums/${album.id}`); showNotification('success', 'Album deleted'); setIsDeleteAlbumOpen(false); navigate('/albums'); } catch (e) { } };
    const handleEditAlbum = async () => { if (!album) return; try { const { data } = await api.put<AlbumDto>(`/users/me/albums/${album.id}`, editAlbumFormData); setAlbum(data); setIsEditAlbumOpen(false); showNotification('success', 'Album updated'); } catch (e) { } };
    const handleSaveImageEdit = async (data: ImageFormData) => { if (!editingImage) return; try { const payload = { source: data.source || null, isNsfw: data.isNsfw, userId: data.userId || null, tags: data.tags || [], artists: data.artists || [] }; const { data: updatedImage } = await api.put<ImageDto>(`/images/${editingImage.id}`, payload); setImages(prev => prev.map(img => img.id === updatedImage.id ? updatedImage : img)); showNotification('success', 'Image updated'); setIsEditImageOpen(false); setEditingImage(null); } catch (err) { } };

    // Helper pour changer de page via le composant Pagination
    const handlePageChange = (newPage: number) => {
        setSearchParams(prev => {
            prev.set('page', newPage.toString());
            return prev;
        });
    };

    const sortOptions = [
        { id: ImageSort.ADDED_TO_ALBUM, name: 'Date Added' },
        { id: ImageSort.UPLOADED_AT, name: 'Newest Upload' },
        { id: ImageSort.FAVORITES, name: 'Most Popular' },
        { id: ImageSort.RANDOM, name: 'Random Shuffle' },
    ];

    if (!user) return null;
    if (loading && !album) return <div className="p-10 text-center">Loading album...</div>;

    return (
        <div className="flex h-full relative overflow-hidden">
            <div className="flex-1 flex flex-col relative min-w-0 overflow-hidden">
                <FloatingRefreshButton onRefresh={() => fetchAlbumData()} />

                <div className="flex-1 overflow-y-auto p-4 md:p-6">
                    <div className="flex flex-col md:flex-row md:items-start justify-between px-4 md:px-6 py-4 border-b border-border backdrop-blur-xl z-10 shrink-0 gap-4 -mx-4 md:-mx-6 -mt-4 md:-mt-6 mb-4">
                        <div className="flex items-start gap-4 overflow-hidden max-w-full">
                            <Link to="/albums" className="p-2 rounded-full bg-secondary hover:bg-secondary/80 transition-colors shrink-0 mt-1">
                                <ChevronLeft size={24} />
                            </Link>
                            <div className="min-w-0 flex-1">
                                <div className="flex items-center gap-3 flex-wrap">
                                    <h1 className="text-3xl font-black flex items-center gap-2 text-foreground truncate">
                                        <FolderOpen className="text-primary shrink-0" />
                                        <span className="truncate">{album?.name || "Album"}</span>
                                    </h1>
                                    <div className="flex items-center gap-1 shrink-0">
                                        <button onClick={() => { setEditAlbumFormData({ name: album?.name || '', description: album?.description || '' }); setIsEditAlbumOpen(true); }} className="p-1.5 rounded-full hover:bg-secondary text-muted-foreground hover:text-foreground" title="Edit Album"><Edit2 size={16} /></button>
                                        {!album?.isDefault && <button onClick={() => setIsDeleteAlbumOpen(true)} className="p-1.5 rounded-full hover:bg-destructive/10 text-muted-foreground hover:text-destructive transition-colors" title="Delete Album"><Trash2 size={16} /></button>}
                                    </div>
                                </div>
                                <div className="flex items-center gap-2 mt-2 mb-2">
                                    {album?.isDefault && <span className="text-[10px] font-bold uppercase tracking-wider text-rose-500 bg-rose-500/10 px-2 py-1 rounded">Default</span>}
                                    <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground bg-secondary px-2 py-1 rounded">{album?.imageCount || 0} image{album?.imageCount !== 1 ? 's' : ''}</span>
                                </div>
                                {album?.description && <p className="text-muted-foreground text-sm break-words whitespace-pre-wrap max-w-3xl">{album.description}</p>}
                            </div>
                        </div>
                        <div className="flex items-center self-end md:self-start shrink-0">
                            <FilterToggleButton isOpen={showFilters} onToggle={() => setShowFilters(!showFilters)} />
                        </div>
                    </div>

                    <ImageGrid
                        images={images}
                        isLoading={loading && !album}
                        // On retire les props de pagination de ImageGrid car on utilise le composant externe maintenant
                        // (Si ImageGrid a encore besoin de 'page' pour l'affichage, tu peux le laisser)
                        onRemove={(id) => setImageToRemove(id)}
                        onEdit={isAdminOrModerator ? (img) => { setEditingImage(img); setIsEditImageOpen(true); } : undefined}
                        emptyState={
                            <div className="flex flex-col items-center justify-center h-64 text-muted-foreground border-2 border-dashed border-border rounded-xl bg-card/50">
                                <p className="text-lg font-medium">This album is empty.</p>
                                <Link to="/gallery" className="mt-2 text-primary font-bold hover:underline">Browse Gallery to add images</Link>
                            </div>
                        }
                    />

                    {/* AJOUT DU COMPOSANT PAGINATION */}
                    <Pagination
                        currentPage={page}
                        totalPages={totalPages}
                        setPage={handlePageChange}
                        // Désactive la pagination si on trie par Random
                        disabled={orderBy === ImageSort.RANDOM}
                    />
                </div>
            </div>

            <FilterSidebar searchParams={searchParams} setSearchParams={setSearchParams} showFilters={showFilters} setShowFilters={setShowFilters} sortOptions={sortOptions} />
            <Modal isOpen={isEditAlbumOpen} onClose={() => setIsEditAlbumOpen(false)} title="Edit Album Details">
                <div className="space-y-4">
                    <div><label className="font-bold block mb-1">Name</label><input value={editAlbumFormData.name} onChange={e => setEditAlbumFormData({...editAlbumFormData, name: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none"/></div>
                    <div><label className="font-bold block mb-1">Description</label><textarea value={editAlbumFormData.description} onChange={e => setEditAlbumFormData({...editAlbumFormData, description: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none h-24"/></div>
                    <button onClick={handleEditAlbum} className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg">Save</button>
                </div>
            </Modal>
            <ConfirmModal isOpen={!!imageToRemove} onClose={() => setImageToRemove(null)} onConfirm={handleRemoveImage} title="Remove Image" message="Remove this image from the album? It will remain in the gallery." confirmText="Remove" variant="warning" icon={FolderMinus} />
            <ConfirmModal isOpen={isDeleteAlbumOpen} onClose={() => setIsDeleteAlbumOpen(false)} onConfirm={handleDeleteAlbum} title="Delete Album" message={<div><p>Are you sure you want to delete the album <strong>{album?.name}</strong>?</p><p className="text-sm text-muted-foreground mt-1">The images inside will NOT be deleted from the gallery.</p></div>} confirmText="Delete Album" variant="destructive" />
            {editingImage && <ImageModal isOpen={isEditImageOpen} onClose={() => { setIsEditImageOpen(false); setEditingImage(null); }} initialData={editingImage} onSubmit={handleSaveImageEdit} />}
        </div>
    );
};

export default AlbumPage;
import { useEffect, useState } from 'react';
import { useParams, Link, useNavigate, Navigate } from 'react-router-dom';
import api, { showApiError } from '../services/api';
import { AlbumDto, ImageOrderBy, NsfwMode, User as UserType } from '../types';
import { ChevronLeft, FolderOpen, Edit2, Trash2, FolderMinus } from 'lucide-react';
import { useNotification } from '../context/NotificationContext';
import Modal from '../components/Modal';
import ConfirmModal from '../components/modals/ConfirmModal';
import { useRequireAuth } from '../hooks/useRequireAuth';
import { useImages } from '../hooks/useImages';
import { GalleryLayout } from '../components/layout/GalleryLayout';
import { useGalleryParams } from '../hooks/useGalleryParams';
import { useAlternativeSearch } from '../hooks/useAlternativeSearch';
import EmptyState from '../components/EmptyState';
import NotFound from './NotFound';
import { MetaTags } from '../hooks/useMetaTags';

const AlbumPage = () => {
    const { id, userId } = useParams<{ id: string; userId?: string }>();
    const user = useRequireAuth();
    const navigate = useNavigate();
    const isViewingOther = !!userId;
    const apiUserPath = isViewingOther ? `/users/${userId}` : '/users/me';
    const albumsBasePath = isViewingOther ? `/users/${userId}/albums` : '/albums';
    const { showNotification } = useNotification();

    const { filters, page, setPage, searchParams, setSearchParams, isNsfw } = useGalleryParams(ImageOrderBy.AddedToAlbum);

    const [album, setAlbum] = useState<AlbumDto | null>(null);
    const [targetUserName, setTargetUserName] = useState<string | null>(null);
    const [loadingAlbum, setLoadingAlbum] = useState(true);
    const [notFound, setNotFound] = useState(false);
    const [isEditAlbumOpen, setIsEditAlbumOpen] = useState(false);
    const [editAlbumFormData, setEditAlbumFormData] = useState({ name: '', description: '' });
    const [isDeleteAlbumOpen, setIsDeleteAlbumOpen] = useState(false);
    const [imageToRemove, setImageToRemove] = useState<number | null>(null);

    useEffect(() => {
        if (user && id) {
            setLoadingAlbum(true);
            api.get<AlbumDto>(`${apiUserPath}/albums/${id}`, { skipGlobalErrorHandler: true })
                .then(res => setAlbum(res.data))
                .catch((e: any) => { if (e?.response?.status === 404) setNotFound(true); else showApiError(e); })
                .finally(() => setLoadingAlbum(false));
            if (isViewingOther) {
                api.get<UserType>(`/users/${userId}`, { skipGlobalErrorHandler: true })
                    .then(res => setTargetUserName(res.data.name))
                    .catch((e: any) => { if (e?.response?.status !== 403) showApiError(e); });
            }
        }
    }, [id, user]);

    const { data: paginatedImages, isLoading: loadingImages, refetch } = useImages({ ...filters, albumId: id, albumUserId: userId });

    const { alternativeCount, isCheckingAlternatives } = useAlternativeSearch(
        loadingImages,
        paginatedImages?.items,
        isNsfw,
        { ...filters, albumId: id, albumUserId: userId }
    );

    const handleRemoveImage = async () => {
        if (!id || !imageToRemove) return;
        try {
            await api.delete(`${apiUserPath}/albums/${id}/images/${imageToRemove}`);
            refetch();
            if (album) setAlbum({ ...album, imageCount: Math.max(0, (album.imageCount || 0) - 1) });
            showNotification('success', 'Image removed from album');
            setImageToRemove(null);
        } catch(e) { }
    };

    const handleDeleteAlbum = async () => {
        if (!album) return;
        try {
            await api.delete(`${apiUserPath}/albums/${album.id}`);
            showNotification('success', 'Album deleted');
            setIsDeleteAlbumOpen(false);
            navigate(albumsBasePath);
        } catch (e) { }
    };

    const handleEditAlbum = async () => {
        if (!album) return;
        try {
            const { data } = await api.patch<AlbumDto>(`${apiUserPath}/albums/${album.id}`, editAlbumFormData);
            setAlbum(data);
            setIsEditAlbumOpen(false);
            showNotification('success', 'Album updated');
        } catch (e) { }
    };

    const handleEnableNsfw = () => {
        setSearchParams(prev => {
            prev.set('isNsfw', NsfwMode.All);
            return prev;
        });
    };

    const sortOptions = [
        { id: ImageOrderBy.AddedToAlbum, name: 'Date Added' },
        { id: ImageOrderBy.UploadedAt, name: 'Newest Upload' },
        { id: ImageOrderBy.Favorites, name: 'Most Popular' },
        { id: ImageOrderBy.Random, name: 'Random Shuffle' },
    ];

    if (!user) return null;
    if (userId && user.id.toString() === userId) return <Navigate to={`/albums/${id}`} replace />;
    if (notFound) return <NotFound />;
    if (loadingAlbum && !album) return <div className="p-10 text-center">Loading album info...</div>;

    const AlbumHeader = (
        <div className="flex items-start gap-4 overflow-hidden max-w-full">
            <Link to={albumsBasePath} className="p-2 rounded-full bg-secondary hover:bg-secondary/80 transition-colors shrink-0 mt-1">
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
                {isViewingOther && (
                    <div className="mt-2 inline-flex items-center gap-1.5 px-3 py-1.5 bg-amber-500/10 border border-amber-500/30 rounded-md text-amber-600 dark:text-amber-400 text-xs font-semibold">
                        &#9888; Any changes here will affect {targetUserName || `user ${userId}`}'s album, not your own.
                    </div>
                )}
            </div>
        </div>
    );

    return (
        <>
            <MetaTags
              title={album?.name || 'Album'}
              description={album?.description || `Album with ${album?.imageCount || 0} images.`}
            />
            <GalleryLayout
                images={paginatedImages?.items || []}
                isLoading={loadingImages}
                refetch={refetch}
                page={page}
                totalPages={paginatedImages?.totalPages || 1}
                setPage={setPage}
                searchParams={searchParams}
                setSearchParams={setSearchParams}
                headerContent={AlbumHeader}
                sortOptions={sortOptions}
                onRemoveFromAlbum={setImageToRemove}
                readOnly={isViewingOther}
                emptyState={
                    <EmptyState
                        isCheckingAlternatives={isCheckingAlternatives}
                        alternativeCount={alternativeCount}
                        onEnableNsfw={handleEnableNsfw}
                        onClearFilters={() => { setSearchParams(new URLSearchParams()); refetch(); }}
                        customEmptyMessage={
                            !isCheckingAlternatives && (!alternativeCount || alternativeCount === 0) ? (
                                <div className="flex flex-col items-center">
                                    <p className="text-lg font-medium">This album is empty.</p>
                                    <Link to="/gallery" className="mt-2 text-primary font-bold hover:underline">Browse Gallery to add images</Link>
                                </div>
                            ) : undefined
                        }
                    />
                }
            />

            <Modal isOpen={isEditAlbumOpen} onClose={() => setIsEditAlbumOpen(false)} title="Edit Album Details">
                <div className="space-y-4">
                    <div><label className="font-bold block mb-1">Name</label><input value={editAlbumFormData.name} onChange={e => setEditAlbumFormData({...editAlbumFormData, name: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none"/></div>
                    <div><label className="font-bold block mb-1">Description</label><textarea value={editAlbumFormData.description} onChange={e => setEditAlbumFormData({...editAlbumFormData, description: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none h-24"/></div>
                    <button onClick={handleEditAlbum} className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg">Save</button>
                </div>
            </Modal>

            <ConfirmModal isOpen={!!imageToRemove} onClose={() => setImageToRemove(null)} onConfirm={handleRemoveImage} title="Remove Image" message="Remove this image from the album? It will remain in the gallery." confirmText="Remove" variant="warning" icon={FolderMinus} />

            <ConfirmModal isOpen={isDeleteAlbumOpen} onClose={() => setIsDeleteAlbumOpen(false)} onConfirm={handleDeleteAlbum} title="Delete Album" message={<div><p>Are you sure you want to delete the album <strong>{album?.name}</strong>?</p><p className="text-sm text-muted-foreground mt-1">The images inside will NOT be deleted from the gallery.</p></div>} confirmText="Delete Album" variant="destructive" />
        </>
    );
};

export default AlbumPage;
import { useEffect, useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import api from '../services/api';
import { AlbumDto, ImageSort } from '../types';
import { ChevronLeft, FolderOpen, Edit2, Trash2, FolderMinus } from 'lucide-react';
import { useNotification } from '../context/NotificationContext';
import Modal from '../components/Modal';
import ConfirmModal from '../components/modals/ConfirmModal';
import { useRequireAuth } from '../hooks/useRequireAuth';
import { useImages } from '../hooks/useImages';
import { GalleryLayout } from '../components/layout/GalleryLayout';
import { useGalleryParams } from '../hooks/useGalleryParams';

const AlbumPage = () => {
    const { id } = useParams<{ id: string }>();
    const user = useRequireAuth();
    const navigate = useNavigate();
    const { showNotification } = useNotification();

    const { filters, page, setPage, searchParams, setSearchParams } = useGalleryParams(ImageSort.ADDED_TO_ALBUM);

    const [album, setAlbum] = useState<AlbumDto | null>(null);
    const [loadingAlbum, setLoadingAlbum] = useState(true);
    const [isEditAlbumOpen, setIsEditAlbumOpen] = useState(false);
    const [editAlbumFormData, setEditAlbumFormData] = useState({ name: '', description: '' });
    const [isDeleteAlbumOpen, setIsDeleteAlbumOpen] = useState(false);
    const [imageToRemove, setImageToRemove] = useState<number | null>(null);

    useEffect(() => {
        if (user && id) {
            setLoadingAlbum(true);
            api.get<AlbumDto>(`/users/me/albums/${id}`)
                .then(res => setAlbum(res.data))
                .catch(console.error)
                .finally(() => setLoadingAlbum(false));
        }
    }, [id, user]);

    const { data: paginatedImages, isLoading: loadingImages, refetch } = useImages({ ...filters, albumId: id });

    const handleRemoveImage = async () => {
        if (!id || !imageToRemove) return;
        try {
            await api.delete(`/users/me/albums/${id}/images/${imageToRemove}`);
            refetch();
            if (album) setAlbum({ ...album, imageCount: Math.max(0, (album.imageCount || 0) - 1) });
            showNotification('success', 'Image removed from album');
            setImageToRemove(null);
        } catch(e) { }
    };

    const handleDeleteAlbum = async () => {
        if (!album) return;
        try {
            await api.delete(`/users/me/albums/${album.id}`);
            showNotification('success', 'Album deleted');
            setIsDeleteAlbumOpen(false);
            navigate('/albums');
        } catch (e) { }
    };

    const handleEditAlbum = async () => {
        if (!album) return;
        try {
            const { data } = await api.put<AlbumDto>(`/users/me/albums/${album.id}`, editAlbumFormData);
            setAlbum(data);
            setIsEditAlbumOpen(false);
            showNotification('success', 'Album updated');
        } catch (e) { }
    };

    const sortOptions = [
        { id: ImageSort.ADDED_TO_ALBUM, name: 'Date Added' },
        { id: ImageSort.UPLOADED_AT, name: 'Newest Upload' },
        { id: ImageSort.FAVORITES, name: 'Most Popular' },
        { id: ImageSort.RANDOM, name: 'Random Shuffle' },
    ];

    if (!user) return null;
    if (loadingAlbum && !album) return <div className="p-10 text-center">Loading album info...</div>;

    const AlbumHeader = (
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
    );

    return (
        <>
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
                emptyState={
                    <div className="flex flex-col items-center justify-center h-64 text-muted-foreground border-2 border-dashed border-border rounded-xl bg-card/50">
                        <p className="text-lg font-medium">This album is empty.</p>
                        <Link to="/gallery" className="mt-2 text-primary font-bold hover:underline">Browse Gallery to add images</Link>
                    </div>
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
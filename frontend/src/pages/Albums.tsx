import { useEffect, useState } from 'react';
import api from '../services/api';
import { Link } from 'react-router-dom';
import { Folder, Plus, Edit2, Trash2, Library } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import { AlbumDto } from '../types';
import AlbumModal, { AlbumFormData } from '../components/modals/AlbumModal';
import ConfirmModal from '../components/modals/ConfirmModal';

const Albums = () => {
    const { user } = useAuth();
    const { showNotification } = useNotification();
    const [albums, setAlbums] = useState<AlbumDto[]>([]);
    const [loading, setLoading] = useState(true);

    const [isCreateOpen, setIsCreateOpen] = useState(false);
    const [isEditOpen, setIsEditOpen] = useState(false);
    const [isDeleteOpen, setIsDeleteOpen] = useState(false);
    const [selectedAlbum, setSelectedAlbum] = useState<AlbumDto | null>(null);

    const fetchAlbums = async () => {
        if(!user) return;
        setLoading(true);
        try {
            const { data } = await api.get<any>(`/users/me/albums`);
            if (Array.isArray(data)) setAlbums(data);
            else if (data?.items) setAlbums(data.items);
            else setAlbums([]);
        } catch { showNotification('error', 'Failed to load albums'); }
        finally { setLoading(false); }
    };

    useEffect(() => { fetchAlbums(); }, [user]);

    const handleCreate = async (data: AlbumFormData) => {
        try {
            await api.post(`/users/me/albums`, data);
            showNotification('success', 'Album created');
            setIsCreateOpen(false);
            fetchAlbums();
        } catch { showNotification('error', 'Failed to create album'); }
    };

    const handleEdit = async (data: AlbumFormData) => {
        if (!selectedAlbum) return;
        try {
            await api.put(`/users/me/albums/${selectedAlbum.id}`, data);
            showNotification('success', 'Album updated');
            setIsEditOpen(false);
            fetchAlbums();
        } catch { showNotification('error', 'Failed to update album'); }
    };

    const handleDelete = async () => {
        if (!selectedAlbum) return;
        try {
            await api.delete(`/users/me/albums/${selectedAlbum.id}`);
            showNotification('success', 'Album deleted');
            setIsDeleteOpen(false);
            fetchAlbums();
        } catch { showNotification('error', 'Failed to delete album'); }
    };

    if (!user) return <div className="p-10 text-center text-muted-foreground">Please log in.</div>;

    return (
        <div className="container mx-auto p-6 md:p-10">
            <div className="flex justify-between items-center mb-10">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground"><Library className="text-primary" size={32}/> My Albums</h1>
                    <p className="text-muted-foreground mt-1">Manage your collections.</p>
                </div>
                <button onClick={() => setIsCreateOpen(true)} className="flex items-center gap-2 px-6 py-3 bg-primary text-primary-foreground rounded-xl font-bold hover:opacity-90 shadow-lg transition-all"><Plus size={20} /> Create Album</button>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
                {loading ? [...Array(4)].map((_,i) => <div key={i} className="h-48 bg-muted rounded-2xl animate-pulse"/>) : albums.map(album => (
                    <div key={album.id} className="group relative bg-card border border-border rounded-2xl hover:shadow-xl transition-all duration-300 hover:border-primary/30">
                        <Link to={`/albums/${album.id}`} className="block p-6 h-full flex flex-col items-center justify-center text-center">
                            <div className={`w-20 h-20 rounded-full flex items-center justify-center mb-4 transition-transform group-hover:scale-110 ${album.isDefault ? 'bg-rose-500/10 text-rose-500' : 'bg-secondary text-primary'}`}>
                                <Folder size={40} className={album.isDefault ? "fill-current" : ""} />
                            </div>
                            <h3 className="font-bold text-xl mb-1 text-foreground">{album.name}</h3>
                            <p className="text-sm text-muted-foreground line-clamp-2 min-h-[2.5em]">{album.description || "No description"}</p>
                            {album.isDefault && <span className="mt-3 text-[10px] font-bold uppercase tracking-wider text-rose-500 bg-rose-500/10 px-2 py-1 rounded">Default</span>}
                        </Link>
                        <div className="absolute top-4 right-4 opacity-0 group-hover:opacity-100 transition-opacity flex gap-2">
                            <button onClick={() => { setSelectedAlbum(album); setIsEditOpen(true); }} className="p-2 bg-background border border-border rounded-full hover:bg-secondary text-muted-foreground hover:text-foreground shadow-sm"><Edit2 size={14} /></button>
                            {!album.isDefault && (
                                <button onClick={() => { setSelectedAlbum(album); setIsDeleteOpen(true); }} className="p-2 bg-background border border-border rounded-full hover:bg-destructive text-muted-foreground hover:text-destructive-foreground shadow-sm"><Trash2 size={14} /></button>
                            )}
                        </div>
                    </div>
                ))}
            </div>

            <AlbumModal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} onSubmit={handleCreate} title="Create New Album" submitLabel="Create" />
            <AlbumModal isOpen={isEditOpen} onClose={() => setIsEditOpen(false)} onSubmit={handleEdit} initialData={selectedAlbum || undefined} title="Edit Album" submitLabel="Save" />
            <ConfirmModal isOpen={isDeleteOpen} onClose={() => setIsDeleteOpen(false)} onConfirm={handleDelete} title="Delete Album" message={<>Delete album <strong>{selectedAlbum?.name}</strong>? Images inside will not be deleted.</>} />
        </div>
    );
};

export default Albums;
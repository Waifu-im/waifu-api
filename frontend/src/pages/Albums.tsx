import { useEffect, useState } from 'react';
import api from '../services/api';
import { Link } from 'react-router-dom';
import { Folder, Plus, Edit2, Trash2, Library } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import Modal from '../components/Modal';
import { AlbumDto } from '../types';

const Albums = () => {
    const { user } = useAuth();
    const { showNotification } = useNotification();
    const [albums, setAlbums] = useState<AlbumDto[]>([]);
    const [loading, setLoading] = useState(true);

    // Modal States
    const [isCreateOpen, setIsCreateOpen] = useState(false);
    const [isEditOpen, setIsEditOpen] = useState(false);
    const [isDeleteOpen, setIsDeleteOpen] = useState(false);

    // Form/Action States
    const [selectedAlbum, setSelectedAlbum] = useState<AlbumDto | null>(null);
    const [formData, setFormData] = useState({ name: '', description: '' });

    const fetchAlbums = async () => {
        if(!user) return;
        setLoading(true);
        try {
            const { data } = await api.get<AlbumDto[]>(`/users/me/albums`);
            setAlbums(data);
        } catch (err) {
            console.error(err);
            showNotification('error', 'Failed to load albums');
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => { fetchAlbums(); }, [user]);

    const handleCreate = async () => {
        try {
            await api.post(`/users/me/albums`, formData);
            showNotification('success', 'Album created successfully');
            setIsCreateOpen(false);
            setFormData({ name: '', description: '' });
            fetchAlbums();
        } catch (e) {
            showNotification('error', 'Failed to create album');
        }
    };

    const handleEdit = async () => {
        if (!selectedAlbum) return;
        try {
            await api.put(`/users/me/albums/${selectedAlbum.id}`, formData);
            showNotification('success', 'Album updated');
            setIsEditOpen(false);
            fetchAlbums();
        } catch (e) {
            showNotification('error', 'Failed to update album');
        }
    };

    const handleDelete = async () => {
        if (!selectedAlbum) return;
        try {
            await api.delete(`/users/me/albums/${selectedAlbum.id}`);
            showNotification('success', 'Album deleted');
            setIsDeleteOpen(false);
            fetchAlbums();
        } catch (e) {
            showNotification('error', 'Failed to delete album');
        }
    };

    const openEdit = (e: React.MouseEvent, album: AlbumDto) => {
        e.preventDefault(); e.stopPropagation();
        setSelectedAlbum(album);
        setFormData({ name: album.name, description: album.description });
        setIsEditOpen(true);
    };

    const openDelete = (e: React.MouseEvent, album: AlbumDto) => {
        e.preventDefault(); e.stopPropagation();
        if(album.isDefault) return;
        setSelectedAlbum(album);
        setIsDeleteOpen(true);
    };

    if (!user) return <div className="p-10 text-center text-muted-foreground">Please log in to view your albums.</div>;

    return (
        <div className="container mx-auto p-6 md:p-10">
            <div className="flex flex-col md:flex-row justify-between items-center mb-10 gap-4">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground">
                        <Library className="text-primary" size={32}/> My Albums
                    </h1>
                    <p className="text-muted-foreground mt-1">Manage your collections and favorites.</p>
                </div>
                <button
                    onClick={() => { setFormData({name:'', description:''}); setIsCreateOpen(true); }}
                    className="flex items-center gap-2 px-6 py-3 bg-primary text-primary-foreground rounded-xl font-bold hover:opacity-90 shadow-lg transition-all transform hover:-translate-y-0.5"
                >
                    <Plus size={20} /> Create Album
                </button>
            </div>

            {loading ? (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
                    {[...Array(4)].map((_,i) => <div key={i} className="h-48 bg-muted rounded-2xl animate-pulse"/>)}
                </div>
            ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
                    {albums.map(album => (
                        <div key={album.id} className="group relative bg-card border border-border rounded-2xl hover:shadow-xl transition-all duration-300 hover:border-primary/30">
                            <Link to={`/albums/${album.id}`} className="block p-6 h-full flex flex-col items-center justify-center text-center">
                                <div className={`w-20 h-20 rounded-full flex items-center justify-center mb-4 transition-transform group-hover:scale-110 ${album.isDefault ? 'bg-rose-500/10 text-rose-500' : 'bg-secondary text-primary'}`}>
                                    <Folder size={40} className={album.isDefault ? "fill-current" : ""} />
                                </div>
                                <h3 className="font-bold text-xl mb-1 text-foreground">{album.name}</h3>
                                <p className="text-sm text-muted-foreground line-clamp-2 min-h-[2.5em]">
                                    {album.description || "No description"}
                                </p>
                                {album.isDefault && <span className="mt-3 text-[10px] font-bold uppercase tracking-wider text-rose-500 bg-rose-500/10 px-2 py-1 rounded">Default</span>}
                            </Link>

                            {/* Actions */}
                            <div className="absolute top-4 right-4 opacity-0 group-hover:opacity-100 transition-opacity flex gap-2">
                                <button
                                    onClick={(e) => openEdit(e, album)}
                                    className="p-2 bg-background border border-border rounded-full hover:bg-secondary text-muted-foreground hover:text-foreground shadow-sm"
                                    title="Edit"
                                >
                                    <Edit2 size={14} />
                                </button>
                                {!album.isDefault && (
                                    <button
                                        onClick={(e) => openDelete(e, album)}
                                        className="p-2 bg-background border border-border rounded-full hover:bg-destructive text-muted-foreground hover:text-destructive-foreground shadow-sm"
                                        title="Delete"
                                    >
                                        <Trash2 size={14} />
                                    </button>
                                )}
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {/* Create Modal */}
            <Modal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} title="Create New Album">
                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-bold mb-1">Name</label>
                        <input
                            value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})}
                            className="w-full p-3 bg-secondary rounded-lg border-transparent focus:border-primary outline-none text-foreground"
                            placeholder="Collection Name"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-bold mb-1">Description</label>
                        <textarea
                            value={formData.description} onChange={e => setFormData({...formData, description: e.target.value})}
                            className="w-full p-3 bg-secondary rounded-lg border-transparent focus:border-primary outline-none h-24 resize-none text-foreground"
                            placeholder="Description..."
                        />
                    </div>
                    <button onClick={handleCreate} className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg mt-2">
                        Create Album
                    </button>
                </div>
            </Modal>

            {/* Edit Modal */}
            <Modal isOpen={isEditOpen} onClose={() => setIsEditOpen(false)} title="Edit Album">
                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-bold mb-1">Name</label>
                        <input
                            value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})}
                            className="w-full p-3 bg-secondary rounded-lg border-transparent focus:border-primary outline-none text-foreground"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-bold mb-1">Description</label>
                        <textarea
                            value={formData.description} onChange={e => setFormData({...formData, description: e.target.value})}
                            className="w-full p-3 bg-secondary rounded-lg border-transparent focus:border-primary outline-none h-24 resize-none text-foreground"
                        />
                    </div>
                    <button onClick={handleEdit} className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg mt-2">
                        Save Changes
                    </button>
                </div>
            </Modal>

            {/* Delete Modal */}
            <Modal isOpen={isDeleteOpen} onClose={() => setIsDeleteOpen(false)} title="Delete Album">
                <div className="space-y-4 text-center">
                    <div className="w-16 h-16 bg-destructive/10 text-destructive rounded-full flex items-center justify-center mx-auto mb-4">
                        <Trash2 size={32} />
                    </div>
                    <p className="text-muted-foreground">
                        Are you sure you want to delete <strong>{selectedAlbum?.name}</strong>?<br/>
                        All images in this album will be removed from it.
                    </p>
                    <div className="flex gap-3 mt-6">
                        <button onClick={() => setIsDeleteOpen(false)} className="flex-1 py-3 bg-secondary rounded-lg font-bold">Cancel</button>
                        <button onClick={handleDelete} className="flex-1 py-3 bg-destructive text-destructive-foreground rounded-lg font-bold">Delete</button>
                    </div>
                </div>
            </Modal>
        </div>
    );
};

export default Albums;
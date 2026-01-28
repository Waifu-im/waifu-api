import { useState, useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import api from '../services/api';
import { Artist, PaginatedList } from '../types';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import { Plus, Edit2, Trash2, User as UserIcon, ChevronLeft, ChevronRight } from 'lucide-react';
import ArtistModal, { ArtistFormData } from '../components/modals/ArtistModal';
import ConfirmModal from '../components/modals/ConfirmModal';

const Artists = () => {
    const { user } = useAuth();
    const { showNotification } = useNotification();
    const navigate = useNavigate();
    const location = useLocation();

    const [artists, setArtists] = useState<Artist[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const pageSize = 50;

    const [isCreateOpen, setIsCreateOpen] = useState(false);
    const [isEditOpen, setIsEditOpen] = useState(false);
    const [isDeleteOpen, setIsDeleteOpen] = useState(false);
    const [selectedArtist, setSelectedArtist] = useState<Artist | null>(null);

    const fetchArtists = async () => {
        setLoading(true);
        try {
            const { data } = await api.get<PaginatedList<Artist>>('/artists', { params: { page, pageSize } });
            setArtists(data.items);
            setTotalPages(data.totalPages);
        } catch { showNotification('error', 'Failed to load artists'); }
        finally { setLoading(false); }
    };

    useEffect(() => { fetchArtists(); }, [page]);

    const handleOpenCreate = () => {
        if (!user) {
            showNotification('warning', 'Log in to create artists.');
            navigate('/login', { state: { from: location } });
            return;
        }
        setIsCreateOpen(true);
    };

    const handleCreate = async (data: ArtistFormData) => {
        try {
            await api.post('/artists', data);
            setIsCreateOpen(false);
            fetchArtists();
            const isMod = user?.role === 2 || user?.role === 3;
            showNotification(isMod ? 'success' : 'info', isMod ? 'Artist created' : 'Artist submitted for review');
        } catch (e: any) {
            showNotification('error', e.response?.status === 409 ? 'Artist exists' : 'Failed to create artist');
        }
    };

    const handleEdit = async (data: ArtistFormData) => {
        if (!selectedArtist) return;
        try {
            await api.put(`/artists/${selectedArtist.id}`, data);
            showNotification('success', 'Artist updated');
            setIsEditOpen(false);
            fetchArtists();
        } catch { showNotification('error', 'Failed to update artist'); }
    };

    const handleDelete = async () => {
        if (!selectedArtist) return;
        try {
            await api.delete(`/artists/${selectedArtist.id}`);
            showNotification('success', 'Artist deleted');
            setIsDeleteOpen(false);
            fetchArtists();
        } catch { showNotification('error', 'Failed to delete artist'); }
    };

    const canManage = user?.role === 3 || user?.role === 2;
    const isReviewMode = !(user?.role === 2 || user?.role === 3);

    return (
        <div className="container mx-auto p-6 md:p-10">
            <div className="flex justify-between items-center mb-10">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground"><UserIcon className="text-primary" size={32}/> Artists</h1>
                    <p className="text-muted-foreground mt-1">Browse and manage artists.</p>
                </div>
                <button onClick={handleOpenCreate} className="flex items-center gap-2 px-6 py-3 bg-primary text-primary-foreground rounded-xl font-bold hover:opacity-90 shadow-lg transition-all"><Plus size={20} /> New Artist</button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                {loading ? [...Array(8)].map((_,i) => <div key={i} className="h-32 bg-muted rounded-xl animate-pulse"/>) : artists.map(artist => (
                    <div key={artist.id} className="bg-card border border-border rounded-xl p-6 hover:shadow-md transition-all group relative">
                        <h3 className="font-bold text-lg mb-2 truncate">{artist.name}</h3>
                        <div className="flex flex-wrap gap-2 text-xs text-muted-foreground mt-2">
                            {artist.twitter && <span className="bg-secondary px-2 py-1 rounded">TW</span>}
                            {artist.pixiv && <span className="bg-secondary px-2 py-1 rounded">PX</span>}
                        </div>
                        {canManage && (
                            <div className="absolute top-4 right-4 opacity-0 group-hover:opacity-100 transition-opacity flex gap-2">
                                <button onClick={() => { setSelectedArtist(artist); setIsEditOpen(true); }} className="p-1.5 hover:bg-secondary rounded text-muted-foreground hover:text-foreground"><Edit2 size={16}/></button>
                                <button onClick={() => { setSelectedArtist(artist); setIsDeleteOpen(true); }} className="p-1.5 hover:bg-destructive/10 rounded text-muted-foreground hover:text-destructive"><Trash2 size={16}/></button>
                            </div>
                        )}
                    </div>
                ))}
            </div>

            {!loading && totalPages > 1 && (
                <div className="flex justify-center items-center gap-4 mt-10">
                    <button disabled={page === 1} onClick={() => setPage(p => Math.max(1, p - 1))} className="p-2 rounded-full bg-secondary disabled:opacity-50"><ChevronLeft size={20} /></button>
                    <span className="text-sm font-bold">Page {page} of {totalPages}</span>
                    <button disabled={page === totalPages} onClick={() => setPage(p => Math.min(totalPages, p + 1))} className="p-2 rounded-full bg-secondary disabled:opacity-50"><ChevronRight size={20} /></button>
                </div>
            )}

            <ArtistModal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} onSubmit={handleCreate} title="New Artist" isReviewMode={isReviewMode} submitLabel="Create" />
            <ArtistModal isOpen={isEditOpen} onClose={() => setIsEditOpen(false)} onSubmit={handleEdit} initialData={selectedArtist || undefined} title="Edit Artist" submitLabel="Save" />
            <ConfirmModal isOpen={isDeleteOpen} onClose={() => setIsDeleteOpen(false)} onConfirm={handleDelete} title="Delete Artist" message={<>Delete artist <strong>{selectedArtist?.name}</strong>?</>} />
        </div>
    );
};

export default Artists;
import { useState, useEffect } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import api from '../services/api';
import { Artist, PaginatedList, Role } from '../types';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import { Plus, Edit2, Trash2, User as UserIcon, ChevronLeft, ChevronRight, Search, ExternalLink } from 'lucide-react';
import ArtistModal, { ArtistFormData } from '../components/modals/ArtistModal';
import ConfirmModal from '../components/modals/ConfirmModal';
import { useDebounce } from '../hooks/useDebounce';

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

    const [search, setSearch] = useState('');
    const debouncedSearch = useDebounce(search, 500);

    const [isCreateOpen, setIsCreateOpen] = useState(false);
    const [isEditOpen, setIsEditOpen] = useState(false);
    const [isDeleteOpen, setIsDeleteOpen] = useState(false);
    const [selectedArtist, setSelectedArtist] = useState<Artist | null>(null);

    const fetchArtists = async () => {
        setLoading(true);
        try {
            const params: any = { page, pageSize };
            if (debouncedSearch) params.search = debouncedSearch;

            const { data } = await api.get<PaginatedList<Artist>>('/artists', { params });
            setArtists(data.items);
            setTotalPages(data.totalPages);
        } catch { showNotification('error', 'Failed to load artists'); }
        finally { setLoading(false); }
    };

    useEffect(() => {
        setPage(1);
    }, [debouncedSearch]);

    useEffect(() => { fetchArtists(); }, [page, debouncedSearch]);

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
            const isMod = user?.role === Role.Moderator || user?.role === Role.Admin;
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

    const canManage = user && (user.role === Role.Admin || user.role === Role.Moderator);
    const isAdmin = user?.role === Role.Admin;
    const isReviewMode = !(canManage);

    return (
        <div className="container mx-auto p-6 md:p-10">
            <div className="flex flex-col md:flex-row justify-between items-center mb-10 gap-4">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground"><UserIcon className="text-primary" size={32}/> Artists</h1>
                    <p className="text-muted-foreground mt-1">Browse and manage artists.</p>
                </div>

                <div className="flex gap-3 w-full md:w-auto">
                    <div className="relative w-full md:w-64">
                        <Search className="absolute left-3 top-3 text-muted-foreground" size={18} />
                        <input
                            type="text"
                            placeholder="Search artists..."
                            value={search}
                            onChange={(e) => setSearch(e.target.value)}
                            className="w-full pl-10 p-3 bg-card border border-border rounded-xl outline-none focus:ring-2 focus:ring-primary"
                        />
                    </div>
                    <button onClick={handleOpenCreate} className="flex items-center gap-2 px-6 py-3 bg-primary text-primary-foreground rounded-xl font-bold hover:opacity-90 shadow-lg transition-all whitespace-nowrap">
                        <Plus size={20} /> <span className="hidden sm:inline">New</span>
                    </button>
                </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                {loading ? [...Array(8)].map((_,i) => <div key={i} className="h-32 bg-muted rounded-xl animate-pulse"/>) : artists.map(artist => (
                    <div key={artist.id} className="group bg-card border border-border rounded-xl hover:shadow-md hover:border-primary/50 transition-all overflow-hidden p-6 flex flex-col">
                        <div className="flex justify-between items-start mb-2">
                            <div className="flex flex-col gap-1 overflow-hidden">
                                <h3 className="font-bold text-lg truncate text-foreground group-hover:text-primary transition-colors">{artist.name}</h3>
                                <span className="text-xs font-mono bg-secondary px-2 py-1 rounded text-muted-foreground select-text w-fit">#{artist.id}</span>
                            </div>
                            
                            <div className="flex gap-2 flex-shrink-0 ml-2">
                                <Link
                                    to={`/gallery?includedArtists=${artist.id}`}
                                    className="p-1.5 bg-secondary hover:bg-primary hover:text-primary-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                    title="View in Gallery"
                                >
                                    <ExternalLink size={16} />
                                </Link>

                                {canManage && (
                                    <>
                                        <button
                                            onClick={(e) => { e.preventDefault(); e.stopPropagation(); setSelectedArtist(artist); setIsEditOpen(true); }}
                                            className="p-1.5 bg-secondary hover:bg-primary hover:text-primary-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                            title="Edit"
                                        >
                                            <Edit2 size={16}/>
                                        </button>
                                        {isAdmin && (
                                            <button
                                                onClick={(e) => { e.preventDefault(); e.stopPropagation(); setSelectedArtist(artist); setIsDeleteOpen(true); }}
                                                className="p-1.5 bg-secondary hover:bg-destructive hover:text-destructive-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                                title="Delete"
                                            >
                                                <Trash2 size={16}/>
                                            </button>
                                        )}
                                    </>
                                )}
                            </div>
                        </div>

                        <div className="flex flex-wrap gap-2 text-xs text-muted-foreground mt-2">
                            {artist.twitter && <span className="bg-secondary px-2 py-1 rounded">TW</span>}
                            {artist.pixiv && <span className="bg-secondary px-2 py-1 rounded">PX</span>}
                        </div>
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
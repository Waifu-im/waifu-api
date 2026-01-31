import { useState } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import { Artist, Role } from '../types';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import { Plus, Edit2, Trash2, User as UserIcon, ChevronLeft, ChevronRight, Search, ExternalLink, Info, Link as LinkIcon } from 'lucide-react';
import ArtistModal, { ArtistFormData } from '../components/modals/ArtistModal';
import ConfirmModal from '../components/modals/ConfirmModal';
import Modal from '../components/Modal';
import { useResource } from '../hooks/useResource';

const Artists = () => {
    const { user } = useAuth();
    const { showNotification } = useNotification();
    const navigate = useNavigate();
    const location = useLocation();

    const { 
        items: artists, 
        loading, 
        page, 
        setPage, 
        totalPages, 
        search, 
        setSearch, 
        createItem, 
        updateItem, 
        deleteItem 
    } = useResource<Artist>('/artists');

    const [isCreateOpen, setIsCreateOpen] = useState(false);
    const [isEditOpen, setIsEditOpen] = useState(false);
    const [isDeleteOpen, setIsDeleteOpen] = useState(false);
    const [selectedArtist, setSelectedArtist] = useState<Artist | null>(null);
    const [infoArtist, setInfoArtist] = useState<Artist | null>(null);
    const [formData, setFormData] = useState<ArtistFormData>({ name: '', twitter: '', pixiv: '', patreon: '', deviantArt: '' });

    const handleOpenCreate = () => {
        if (!user) {
            showNotification('warning', 'Log in to create artists.');
            navigate('/login', { state: { from: location } });
            return;
        }
        setFormData({ name: '', twitter: '', pixiv: '', patreon: '', deviantArt: '' });
        setIsCreateOpen(true);
    };

    const handleCreate = async (data: ArtistFormData) => {
        const isMod = user?.role === Role.Moderator || user?.role === Role.Admin;
        const success = await createItem(data, isMod ? 'Artist created' : 'Artist submitted for review');
        if (success) {
            setIsCreateOpen(false);
            setFormData({ name: '', twitter: '', pixiv: '', patreon: '', deviantArt: '' });
        }
    };

    const handleEdit = async (data: ArtistFormData) => {
        if (!selectedArtist) return;
        const success = await updateItem(selectedArtist.id, data, 'Artist updated');
        if (success) setIsEditOpen(false);
    };

    const handleDelete = async () => {
        if (!selectedArtist) return;
        const success = await deleteItem(selectedArtist.id, 'Artist deleted');
        if (success) setIsDeleteOpen(false);
    };

    const openEdit = (e: React.MouseEvent, artist: Artist) => {
        e.preventDefault();
        e.stopPropagation();
        setSelectedArtist(artist);
        setFormData({ 
            name: artist.name, 
            twitter: artist.twitter, 
            pixiv: artist.pixiv, 
            patreon: artist.patreon, 
            deviantArt: artist.deviantArt,
            reviewStatus: artist.reviewStatus 
        });
        setIsEditOpen(true);
    };

    const openDelete = (e: React.MouseEvent, artist: Artist) => {
        e.preventDefault();
        e.stopPropagation();
        setSelectedArtist(artist);
        setIsDeleteOpen(true);
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
                {loading ? [...Array(8)].map((_,i) => <div key={i} className="h-40 bg-muted rounded-xl animate-pulse"/>) : artists.map(artist => (
                    <div key={artist.id} className="group bg-card border border-border rounded-xl hover:shadow-md hover:border-primary/50 transition-all overflow-hidden p-6 flex flex-col">
                        <div className="flex justify-between items-start mb-2">
                            <div className="flex flex-col gap-1 overflow-hidden min-w-0">
                                <h3 className="font-bold text-lg truncate text-foreground group-hover:text-primary transition-colors">{artist.name}</h3>
                                <span className="text-xs font-mono bg-secondary px-2 py-1 rounded text-muted-foreground select-text w-fit">#{artist.id}</span>
                            </div>
                            
                            <div className="flex gap-2 flex-shrink-0 ml-2">
                                <button
                                    onClick={() => setInfoArtist(artist)}
                                    className="p-1.5 bg-secondary hover:bg-primary hover:text-primary-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                    title="Info"
                                >
                                    <Info size={16} />
                                </button>
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
                                            onClick={(e) => openEdit(e, artist)}
                                            className="p-1.5 bg-secondary hover:bg-primary hover:text-primary-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                            title="Edit"
                                        >
                                            <Edit2 size={16}/>
                                        </button>
                                        {isAdmin && (
                                            <button
                                                onClick={(e) => openDelete(e, artist)}
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

                        <div className="flex flex-col gap-2 text-xs text-muted-foreground mt-auto pt-4">
                            {artist.twitter && (
                                <a href={artist.twitter} target="_blank" rel="noopener noreferrer" className="flex items-center gap-2 bg-secondary px-2 py-1 rounded hover:bg-primary hover:text-primary-foreground transition-colors" title={artist.twitter}>
                                    <LinkIcon size={14} className="text-muted-foreground shrink-0"/>
                                    <span className="truncate">{artist.twitter.replace('https://', '')}</span>
                                </a>
                            )}
                            {artist.pixiv && (
                                <a href={artist.pixiv} target="_blank" rel="noopener noreferrer" className="flex items-center gap-2 bg-secondary px-2 py-1 rounded hover:bg-primary hover:text-primary-foreground transition-colors" title={artist.pixiv}>
                                    <LinkIcon size={14} className="text-muted-foreground shrink-0"/>
                                    <span className="truncate">{artist.pixiv.replace('https://', '')}</span>
                                </a>
                            )}
                            {artist.deviantArt && (
                                <a href={artist.deviantArt} target="_blank" rel="noopener noreferrer" className="flex items-center gap-2 bg-secondary px-2 py-1 rounded hover:bg-primary hover:text-primary-foreground transition-colors" title={artist.deviantArt}>
                                    <LinkIcon size={14} className="text-muted-foreground shrink-0"/>
                                    <span className="truncate">{artist.deviantArt.replace('https://', '')}</span>
                                </a>
                            )}
                            {artist.patreon && (
                                <a href={artist.patreon} target="_blank" rel="noopener noreferrer" className="flex items-center gap-2 bg-secondary px-2 py-1 rounded hover:bg-primary hover:text-primary-foreground transition-colors" title={artist.patreon}>
                                    <LinkIcon size={14} className="text-muted-foreground shrink-0"/>
                                    <span className="truncate">{artist.patreon.replace('https://', '')}</span>
                                </a>
                            )}
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
            
            <ArtistModal 
                isOpen={isEditOpen} 
                onClose={() => setIsEditOpen(false)} 
                onSubmit={handleEdit} 
                initialData={selectedArtist ? { ...formData, id: selectedArtist.id } : formData} 
                title="Edit Artist" 
                submitLabel="Save" 
                onDelete={isAdmin ? () => setIsDeleteOpen(true) : undefined}
            />

            <ConfirmModal isOpen={isDeleteOpen} onClose={() => setIsDeleteOpen(false)} onConfirm={handleDelete} title="Delete Artist" message={<>Delete artist <strong>{selectedArtist?.name}</strong>?</>} />
            
            <Modal 
                isOpen={!!infoArtist} 
                onClose={() => setInfoArtist(null)} 
                title="Artist Details"
            >
                {infoArtist && (
                    <div className="space-y-4">
                        <div>
                            <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-wider">Name</h3>
                            <p className="text-lg font-medium break-words">{infoArtist.name}</p>
                        </div>
                        <div className="grid grid-cols-1 gap-4">
                            {infoArtist.twitter && (
                                <div>
                                    <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-wider">Twitter</h3>
                                    <a href={infoArtist.twitter} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline break-all">{infoArtist.twitter}</a>
                                </div>
                            )}
                            {infoArtist.pixiv && (
                                <div>
                                    <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-wider">Pixiv</h3>
                                    <a href={infoArtist.pixiv} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline break-all">{infoArtist.pixiv}</a>
                                </div>
                            )}
                            {infoArtist.deviantArt && (
                                <div>
                                    <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-wider">DeviantArt</h3>
                                    <a href={infoArtist.deviantArt} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline break-all">{infoArtist.deviantArt}</a>
                                </div>
                            )}
                            {infoArtist.patreon && (
                                <div>
                                    <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-wider">Patreon</h3>
                                    <a href={infoArtist.patreon} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline break-all">{infoArtist.patreon}</a>
                                </div>
                            )}
                        </div>
                        <div className="pt-4 border-t border-border flex justify-end">
                            <Link to={`/gallery?includedArtists=${infoArtist.id}`} className="px-4 py-2 bg-primary text-primary-foreground rounded-lg font-bold hover:opacity-90">
                                View Gallery
                            </Link>
                        </div>
                    </div>
                )}
            </Modal>
        </div>
    );
};

export default Artists;
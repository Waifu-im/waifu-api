import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import api from '../services/api';
import { ImageDto, Artist, Tag, ImageFormData, PaginatedList, Role } from '../types';
import { useNotification } from '../context/NotificationContext';
import { Check, FileCheck, Edit2, ExternalLink, ChevronLeft, ChevronRight, Link as LinkIcon, Trash2 } from 'lucide-react';
import ImageModal from '../components/modals/ImageModal';
import ArtistModal, { ArtistFormData } from '../components/modals/ArtistModal';
import TagModal, { TagFormData } from '../components/modals/TagModal';
import ConfirmModal from '../components/modals/ConfirmModal';
import ImageGrid from '../components/ImageGrid';
import { useAuth } from '../context/AuthContext';

const Review = () => {
    const { showNotification } = useNotification();
    const { user } = useAuth();
    const [tab, setTab] = useState<'images' | 'artists' | 'tags'>('images');
    const [loading, setLoading] = useState(false);

    const [images, setImages] = useState<ImageDto[]>([]);
    const [artists, setArtists] = useState<Artist[]>([]);
    const [tags, setTags] = useState<Tag[]>([]);

    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const pageSize = 30;

    const [editingImage, setEditingImage] = useState<ImageDto | null>(null);
    const [editingArtist, setEditingArtist] = useState<Artist | null>(null);
    const [editingTag, setEditingTag] = useState<Tag | null>(null);

    const [deletingImage, setDeletingImage] = useState<ImageDto | null>(null);
    const [deletingArtist, setDeletingArtist] = useState<Artist | null>(null);
    const [deletingTag, setDeletingTag] = useState<Tag | null>(null);

    const isAdmin = user?.role === Role.Admin;

    const fetchData = async () => {
        setLoading(true);
        try {
            if (tab === 'images') {
                const { data } = await api.get<PaginatedList<ImageDto>>(`/review/images?page=${page}&pageSize=${pageSize}`, { skipGlobalErrorHandler: true });
                setImages(data.items);
                setTotalPages(data.totalPages);
            } else if (tab === 'artists') {
                const { data } = await api.get<PaginatedList<Artist>>(`/review/artists?page=${page}&pageSize=${pageSize}`, { skipGlobalErrorHandler: true });
                setArtists(data.items);
                setTotalPages(data.totalPages);
            } else {
                const { data } = await api.get<PaginatedList<Tag>>(`/review/tags?page=${page}&pageSize=${pageSize}`, { skipGlobalErrorHandler: true });
                setTags(data.items);
                setTotalPages(data.totalPages);
            }
        } catch (err: any) { 
            // showNotification('error', err.message); // Handled globally
        } finally { setLoading(false); }
    };

    useEffect(() => {
        setPage(1);
    }, [tab]);

    useEffect(() => { fetchData(); }, [tab, page]);

    const handleUpdateImage = async (data: ImageFormData) => {
        if (!editingImage) return;
        try {
            const payload = {
                source: data.source || null,
                isNsfw: data.isNsfw,
                userId: data.userId || null,
                tags: data.tags,
                artists: data.artists,
                reviewStatus: data.reviewStatus
            };
            await api.put(`/images/${editingImage.id}`, payload);
            showNotification('success', 'Image updated');
            setEditingImage(null);
            fetchData();
        } catch (err: any) { 
            // showNotification('error', err.message); // Handled globally
        }
    };

    const handleUpdateArtist = async (data: ArtistFormData) => {
        if (!editingArtist) return;
        try {
            await api.put(`/artists/${editingArtist.id}`, data);
            showNotification('success', 'Artist updated');
            setEditingArtist(null);
            fetchData();
        } catch (err: any) { 
            // showNotification('error', err.message); // Handled globally
        }
    };

    const handleUpdateTag = async (data: TagFormData) => {
        if (!editingTag) return;
        try {
            await api.put(`/tags/${editingTag.id}`, data);
            showNotification('success', 'Tag updated');
            setEditingTag(null);
            fetchData();
        } catch (err: any) { 
            // showNotification('error', err.message); // Handled globally
        }
    };

    const handleDeleteImage = async () => {
        if (!deletingImage) return;
        try {
            await api.delete(`/images/${deletingImage.id}`);
            showNotification('success', 'Image deleted');
            setDeletingImage(null);
            setEditingImage(null);
            fetchData();
        } catch (err: any) {
            // showNotification('error', err.message); // Handled globally
        }
    };

    const handleDeleteArtist = async () => {
        if (!deletingArtist) return;
        try {
            await api.delete(`/artists/${deletingArtist.id}`);
            showNotification('success', 'Artist deleted');
            setDeletingArtist(null);
            setEditingArtist(null);
            fetchData();
        } catch (err: any) {
            // showNotification('error', err.message); // Handled globally
        }
    };

    const handleDeleteTag = async () => {
        if (!deletingTag) return;
        try {
            await api.delete(`/tags/${deletingTag.id}`);
            showNotification('success', 'Tag deleted');
            setDeletingTag(null);
            setEditingTag(null);
            fetchData();
        } catch (err: any) {
            // showNotification('error', err.message); // Handled globally
        }
    };

    return (
        <div className="container mx-auto p-6 md:p-10 h-full flex flex-col">
            <div className="flex flex-col md:flex-row justify-between items-center mb-8 gap-4">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground"><FileCheck className="text-primary" size={32}/> Moderation Queue</h1>
                    <p className="text-muted-foreground mt-1">Review pending submissions.</p>
                </div>

                <div className="flex bg-secondary p-1 rounded-xl">
                    {(['images', 'artists', 'tags'] as const).map(t => (
                        <button
                            key={t}
                            onClick={() => setTab(t)}
                            className={`px-6 py-2 rounded-lg text-sm font-bold capitalize transition-all ${tab === t ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                        >
                            {t}
                        </button>
                    ))}
                </div>
            </div>

            {loading ? (
                <div className="flex-1 flex items-center justify-center text-muted-foreground">Loading queue...</div>
            ) : (
                <div className="flex-1 overflow-y-auto">
                    {/* Images Tab */}
                    {tab === 'images' && (
                        <ImageGrid
                            images={images}
                            isLoading={loading}
                            page={page}
                            totalPages={totalPages}
                            setPage={setPage}
                            onEdit={(img) => setEditingImage(img)}
                            onDelete={isAdmin ? (id) => setDeletingImage(images.find(i => i.id === id) || null) : undefined}
                            emptyState={<EmptyState type="images"/>}
                            forceOverlay={true}
                        />
                    )}

                    {/* Artists Tab */}
                    {tab === 'artists' && (
                        artists.length === 0 ? <EmptyState type="artists"/> :
                            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                                {artists.map(artist => (
                                    <div key={artist.id} className="group bg-card border border-border rounded-xl hover:shadow-md hover:border-primary/50 transition-all overflow-hidden p-6 flex flex-col">
                                        <div className="flex justify-between items-start mb-2">
                                            <div className="flex flex-col gap-1 overflow-hidden min-w-0">
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
                                                <button
                                                    onClick={(e) => { e.preventDefault(); e.stopPropagation(); setEditingArtist(artist); }}
                                                    className="p-1.5 bg-secondary hover:bg-primary hover:text-primary-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                                    title="Edit"
                                                >
                                                    <Edit2 size={16}/>
                                                </button>
                                                {isAdmin && (
                                                    <button
                                                        onClick={(e) => { e.preventDefault(); e.stopPropagation(); setDeletingArtist(artist); }}
                                                        className="p-1.5 bg-secondary hover:bg-destructive hover:text-destructive-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                                        title="Delete"
                                                    >
                                                        <Trash2 size={16}/>
                                                    </button>
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
                    )}

                    {/* Tags Tab */}
                    {tab === 'tags' && (
                        tags.length === 0 ? <EmptyState type="tags"/> :
                            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                                {tags.map(tag => (
                                    <div key={tag.id} className="group bg-card border border-border rounded-xl hover:shadow-md hover:border-primary/50 transition-all overflow-hidden flex flex-col p-6">
                                        <div className="flex justify-between items-start mb-2">
                                            <div className="flex flex-col gap-1 min-w-0">
                                                <h3 className="font-bold text-lg text-foreground group-hover:text-primary transition-colors truncate">{tag.name}</h3>
                                                <div className="flex items-center gap-2">
                                                    <span className="text-xs font-mono bg-secondary px-2 py-1 rounded text-muted-foreground select-text">#{tag.id}</span>
                                                    <code className="text-xs text-primary/80 bg-primary/5 px-1.5 py-0.5 rounded select-text truncate" title="API Slug">{tag.slug}</code>
                                                </div>
                                            </div>

                                            <div className="flex gap-2 flex-shrink-0 ml-2">
                                                <Link
                                                    to={`/gallery?includedTags=${encodeURIComponent(tag.slug)}`}
                                                    className="p-1.5 bg-secondary hover:bg-primary hover:text-primary-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                                    title="View in Gallery"
                                                >
                                                    <ExternalLink size={16} />
                                                </Link>
                                                <button
                                                    onClick={(e) => { e.preventDefault(); e.stopPropagation(); setEditingTag(tag); }}
                                                    className="p-1.5 bg-secondary hover:bg-primary hover:text-primary-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                                    title="Edit"
                                                >
                                                    <Edit2 size={16}/>
                                                </button>
                                                {isAdmin && (
                                                    <button
                                                        onClick={(e) => { e.preventDefault(); e.stopPropagation(); setDeletingTag(tag); }}
                                                        className="p-1.5 bg-secondary hover:bg-destructive hover:text-destructive-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                                        title="Delete"
                                                    >
                                                        <Trash2 size={16}/>
                                                    </button>
                                                )}
                                            </div>
                                        </div>

                                        <p className="text-sm text-muted-foreground line-clamp-3 mt-2">{tag.description || "No description"}</p>
                                    </div>
                                ))}
                            </div>
                    )}

                    {/* Pagination Controls */}
                    {totalPages > 1 && tab !== 'images' && (
                        <div className="flex justify-center items-center gap-4 mt-8 pb-4">
                            <button
                                onClick={() => setPage(p => Math.max(1, p - 1))}
                                disabled={page === 1}
                                className="p-2 rounded-full bg-secondary hover:bg-secondary/80 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                            >
                                <ChevronLeft size={20} />
                            </button>
                            <span className="text-sm font-medium">
                                Page {page} of {totalPages}
                            </span>
                            <button
                                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                                disabled={page === totalPages}
                                className="p-2 rounded-full bg-secondary hover:bg-secondary/80 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                            >
                                <ChevronRight size={20} />
                            </button>
                        </div>
                    )}
                </div>
            )}

            {/* Edit Modals */}
            {editingImage && <ImageModal isOpen={!!editingImage} onClose={() => setEditingImage(null)} onSubmit={handleUpdateImage} initialData={editingImage} onDelete={() => setDeletingImage(editingImage)} />}
            {editingArtist && <ArtistModal isOpen={!!editingArtist} onClose={() => setEditingArtist(null)} onSubmit={handleUpdateArtist} initialData={editingArtist} title="Edit Artist" onDelete={() => setDeletingArtist(editingArtist)} isReviewMode={true} />}
            {editingTag && <TagModal isOpen={!!editingTag} onClose={() => setEditingTag(null)} onSubmit={handleUpdateTag} initialData={editingTag} title="Edit Tag" onDelete={() => setDeletingTag(editingTag)} isReviewMode={true} />}

            {/* Delete Confirmation Modals */}
            <ConfirmModal
                isOpen={!!deletingImage}
                onClose={() => setDeletingImage(null)}
                onConfirm={handleDeleteImage}
                title="Delete Image"
                message="Are you sure you want to delete this image? This action cannot be undone."
            />
            <ConfirmModal
                isOpen={!!deletingArtist}
                onClose={() => setDeletingArtist(null)}
                onConfirm={handleDeleteArtist}
                title="Delete Artist"
                message={`Are you sure you want to delete artist "${deletingArtist?.name}"? This action cannot be undone.`}
            />
            <ConfirmModal
                isOpen={!!deletingTag}
                onClose={() => setDeletingTag(null)}
                onConfirm={handleDeleteTag}
                title="Delete Tag"
                message={`Are you sure you want to delete tag "${deletingTag?.name}"? This action cannot be undone.`}
            />
        </div>
    );
};

const EmptyState = ({type}: {type: string}) => (
    <div className="flex flex-col items-center justify-center h-64 text-muted-foreground border-2 border-dashed border-border rounded-xl bg-card/30">
        <Check size={48} className="mb-4 opacity-20" />
        <p className="text-lg font-medium">All caught up!</p>
        <p className="text-sm">No pending {type} to review.</p>
    </div>
);

export default Review;
import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import api from '../services/api';
import { ImageDto, Artist, Tag, ImageFormData, PaginatedList } from '../types';
import { useNotification } from '../context/NotificationContext';
import { Check, X, FileCheck, Edit2, ExternalLink, Clock, ChevronLeft, ChevronRight, User as UserIcon, Tag as TagIcon } from 'lucide-react';
import ImageModal from '../components/modals/ImageModal';
import ArtistModal, { ArtistFormData } from '../components/modals/ArtistModal';
import TagModal, { TagFormData } from '../components/modals/TagModal';

const Review = () => {
    const { showNotification } = useNotification();
    const [tab, setTab] = useState<'images' | 'artists' | 'tags'>('images');
    const [loading, setLoading] = useState(false);

    const [images, setImages] = useState<ImageDto[]>([]);
    const [artists, setArtists] = useState<Artist[]>([]);
    const [tags, setTags] = useState<Tag[]>([]);

    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const pageSize = 30; // Updated to match backend default

    const [editingImage, setEditingImage] = useState<ImageDto | null>(null);
    const [editingArtist, setEditingArtist] = useState<Artist | null>(null);
    const [editingTag, setEditingTag] = useState<Tag | null>(null);

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

    const handleReview = async (id: number, accepted: boolean) => {
        try {
            await api.post(`/review/${tab}/${id}`, { accepted });
            showNotification('success', accepted ? 'Approved' : 'Rejected');

            if (tab === 'images') setImages(prev => prev.filter(i => i.id !== id));
            else if (tab === 'artists') setArtists(prev => prev.filter(a => a.id !== id));
            else setTags(prev => prev.filter(t => t.id !== id));
        } catch (err: any) { 
            // showNotification('error', err.message); // Handled globally
        }
    };

    const handleUpdateImage = async (data: ImageFormData) => {
        if (!editingImage) return;
        try {
            const payload = {
                source: data.source || null,
                isNsfw: data.isNsfw,
                userId: data.userId || null,
                tags: data.tags,
                artists: data.artists
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
                        images.length === 0 ? <EmptyState type="images"/> :
                            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-6">
                                {images.map(img => (
                                    <div key={img.id} className="bg-card border border-border rounded-xl overflow-hidden flex flex-col shadow-sm">
                                        <div className="aspect-[2/3] relative bg-muted group">
                                            {/* Link to Image Page */}
                                            <Link to={`/images/${img.id}`} className="block w-full h-full">
                                                <img src={img.url} alt={`Review ${img.id}`} className="w-full h-full object-cover" loading="lazy" />
                                            </Link>

                                            {/* External Link Overlay */}
                                            <a href={img.url} target="_blank" rel="noreferrer" className="absolute top-2 right-2 p-1.5 bg-black/50 text-white rounded-full opacity-0 group-hover:opacity-100 transition-opacity z-10 hover:bg-black/70">
                                                <ExternalLink size={14}/>
                                            </a>
                                        </div>

                                        <div className="p-3 border-t border-border space-y-2">
                                            {/* Metadata Section */}
                                            <div className="flex flex-wrap gap-1">
                                                {/* Artist Link */}
                                                {img.artists && img.artists.length > 0 ? (
                                                    img.artists.map(artist => (
                                                        <Link
                                                            key={artist.id}
                                                            to={`/gallery?includedArtists=${artist.id}`}
                                                            className={`text-xs px-2 py-1 rounded flex items-center gap-1 hover:bg-primary/10 transition-colors max-w-full ${artist.reviewStatus === 0 ? 'bg-orange-500/10 text-orange-600 border border-orange-500/20' : 'bg-secondary'}`}
                                                            title={artist.reviewStatus === 0 ? "Artist Pending Review" : "View Artist in Gallery"}
                                                        >
                                                            <UserIcon size={10} className="shrink-0" />
                                                            {artist.reviewStatus === 0 && <Clock size={10} className="shrink-0" />}
                                                            <span className="truncate">{artist.name}</span>
                                                        </Link>
                                                    ))
                                                ) : <span className="text-xs text-muted-foreground italic">No Artist</span>}

                                                {/* Tags Links */}
                                                {img.tags.map(tag => (
                                                    <Link
                                                        key={tag.id}
                                                        to={`/gallery?includedTags=${encodeURIComponent(tag.slug)}`}
                                                        className={`text-xs px-2 py-1 rounded flex items-center gap-1 hover:bg-primary/10 transition-colors max-w-full ${tag.reviewStatus === 0 ? 'bg-orange-500/10 text-orange-600 border border-orange-500/20' : 'bg-secondary'}`}
                                                        title={tag.reviewStatus === 0 ? "Tag Pending Review" : "View Tag in Gallery"}
                                                    >
                                                        <TagIcon size={10} className="shrink-0" />
                                                        {tag.reviewStatus === 0 && <Clock size={10} className="shrink-0" />}
                                                        <span className="truncate">{tag.name}</span>
                                                    </Link>
                                                ))}
                                            </div>

                                            <div className="flex gap-2 pt-2">
                                                <button onClick={() => setEditingImage(img)} className="p-2 bg-secondary text-foreground hover:bg-secondary/80 rounded-lg flex-1 flex justify-center"><Edit2 size={18}/></button>
                                                <button onClick={() => handleReview(img.id, true)} className="p-2 bg-green-500/10 text-green-600 hover:bg-green-500/20 rounded-lg flex-1 flex justify-center"><Check size={18}/></button>
                                                <button onClick={() => handleReview(img.id, false)} className="p-2 bg-red-500/10 text-red-600 hover:bg-red-500/20 rounded-lg flex-1 flex justify-center"><X size={18}/></button>
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                    )}

                    {/* Artists Tab */}
                    {tab === 'artists' && (
                        artists.length === 0 ? <EmptyState type="artists"/> :
                            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                                {artists.map(artist => (
                                    <div key={artist.id} className="bg-card border border-border p-6 rounded-xl flex justify-between items-center gap-4">
                                        <div className="overflow-hidden flex-1 min-w-0">
                                            <h3 className="font-bold text-lg truncate" title={artist.name}>
                                                {/* Link to Gallery filtered by Artist */}
                                                <Link to={`/gallery?includedArtists=${artist.id}`} className="hover:underline hover:text-primary transition-colors">
                                                    {artist.name}
                                                </Link>
                                            </h3>
                                            <div className="flex flex-wrap gap-2 mt-2 text-xs text-muted-foreground">
                                                {artist.twitter && <span className="bg-secondary px-2 py-1 rounded">TW</span>}
                                                {artist.pixiv && <span className="bg-secondary px-2 py-1 rounded">PX</span>}
                                            </div>
                                        </div>
                                        <div className="flex gap-2 shrink-0">
                                            <button onClick={() => setEditingArtist(artist)} className="p-3 bg-secondary text-foreground rounded-lg hover:bg-secondary/80"><Edit2 size={20}/></button>
                                            <button onClick={() => handleReview(artist.id, true)} className="p-3 bg-green-500/10 text-green-600 rounded-lg hover:bg-green-500/20"><Check size={20}/></button>
                                            <button onClick={() => handleReview(artist.id, false)} className="p-3 bg-red-500/10 text-red-600 rounded-lg hover:bg-red-500/20"><X size={20}/></button>
                                        </div>
                                    </div>
                                ))}
                            </div>
                    )}

                    {/* Tags Tab */}
                    {tab === 'tags' && (
                        tags.length === 0 ? <EmptyState type="tags"/> :
                            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                                {tags.map(tag => (
                                    <div key={tag.id} className="bg-card border border-border p-6 rounded-xl flex justify-between items-center gap-4">
                                        <div className="flex-1 min-w-0">
                                            <h3 className="font-bold text-lg truncate" title={tag.name}>
                                                {/* Link to Gallery filtered by Tag */}
                                                <Link to={`/gallery?includedTags=${encodeURIComponent(tag.slug)}`} className="hover:underline hover:text-primary transition-colors">
                                                    {tag.name}
                                                </Link>
                                            </h3>
                                            <p className="text-sm text-muted-foreground line-clamp-2">{tag.description}</p>
                                        </div>
                                        <div className="flex gap-2 shrink-0">
                                            <button onClick={() => setEditingTag(tag)} className="p-3 bg-secondary text-foreground rounded-lg hover:bg-secondary/80"><Edit2 size={20}/></button>
                                            <button onClick={() => handleReview(tag.id, true)} className="p-3 bg-green-500/10 text-green-600 rounded-lg hover:bg-green-500/20"><Check size={20}/></button>
                                            <button onClick={() => handleReview(tag.id, false)} className="p-3 bg-red-500/10 text-red-600 rounded-lg hover:bg-red-500/20"><X size={20}/></button>
                                        </div>
                                    </div>
                                ))}
                            </div>
                    )}

                    {/* Pagination Controls */}
                    {totalPages > 1 && (
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
            {editingImage && <ImageModal isOpen={!!editingImage} onClose={() => setEditingImage(null)} onSubmit={handleUpdateImage} initialData={editingImage} />}
            {editingArtist && <ArtistModal isOpen={!!editingArtist} onClose={() => setEditingArtist(null)} onSubmit={handleUpdateArtist} initialData={editingArtist} title="Edit Artist" />}
            {editingTag && <TagModal isOpen={!!editingTag} onClose={() => setEditingTag(null)} onSubmit={handleUpdateTag} initialData={editingTag} title="Edit Tag" />}
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
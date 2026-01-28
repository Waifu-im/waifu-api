import { useEffect, useState } from 'react';
import { useParams, Link, useSearchParams } from 'react-router-dom';
import api from '../services/api';
import ImageCard from '../components/ImageCard';
import { ImageDto, PaginatedList, AlbumDto } from '../types';
import { ChevronLeft, FolderOpen, Trash2, SlidersHorizontal, Edit2 } from 'lucide-react';
import { useNotification } from '../context/NotificationContext';
import SearchableSelect from '../components/SearchableSelect';
import { useAuth } from '../context/AuthContext';
import Modal from '../components/Modal';

const AlbumPage = () => {
    const { id } = useParams<{ id: string }>();
    const { user } = useAuth();
    const [album, setAlbum] = useState<AlbumDto | null>(null);
    const [images, setImages] = useState<ImageDto[]>([]);
    const [loading, setLoading] = useState(true);
    const [showFilters, setShowFilters] = useState(false);
    const { showNotification } = useNotification();
    const [searchParams, setSearchParams] = useSearchParams();

    // Edit Modal State
    const [isEditOpen, setIsEditOpen] = useState(false);
    const [editFormData, setEditFormData] = useState({ name: '', description: '' });

    // Filters
    const orderBy = searchParams.get('orderBy') || 'UPLOADED_AT';
    const isNsfw = searchParams.get('isNsfw') || 'false';

    const fetchAlbumData = async () => {
        if (!id || !user) return;
        setLoading(true);
        try {
            // 1. Fetch Metadata (AlbumDto)
            const infoRes = await api.get<AlbumDto>(`/users/me/albums/${id}`);
            setAlbum(infoRes.data);

            // 2. Fetch Images (PaginatedList)
            const params = new URLSearchParams(searchParams);
            params.set('pageSize', '50'); // Replaced limit with pageSize
            params.set('page', '1');      // Default page 1

            const imgsRes = await api.get<PaginatedList<ImageDto>>(`/users/me/albums/${id}/images`, { params });

            // Robust check for PaginatedList vs Array (just in case)
            if (imgsRes.data && Array.isArray(imgsRes.data.items)) {
                setImages(imgsRes.data.items);
            } else {
                setImages([]);
            }

        } catch (error) {
            console.error(error);
            showNotification('error', 'Failed to load album');
            setImages([]);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchAlbumData();
    }, [id, searchParams, user]);

    const removeImage = async (imageId: number) => {
        if (!id) return;
        if(confirm("Remove image from album?")) {
            try {
                await api.delete(`/users/me/albums/${id}/images/${imageId}`);
                setImages(prev => prev.filter(img => img.id !== imageId));
                showNotification('success', 'Image removed');
            } catch(e) {
                showNotification('error', 'Failed to remove image');
            }
        }
    };

    const handleEditAlbum = async () => {
        if (!album) return;
        try {
            const { data } = await api.put<AlbumDto>(`/users/me/albums/${album.id}`, editFormData);
            setAlbum(data);
            setIsEditOpen(false);
            showNotification('success', 'Album updated');
        } catch (e) {
            showNotification('error', 'Failed to update album');
        }
    };

    const openEditModal = () => {
        if (album) {
            setEditFormData({ name: album.name, description: album.description });
            setIsEditOpen(true);
        }
    };

    const updateFilter = (key: string, value: string) => {
        setSearchParams(prev => {
            if (value) prev.set(key, value); else prev.delete(key);
            return prev;
        });
    };

    const sortOptions = [
        { id: 'UPLOADED_AT', name: 'Newest Added' },
        { id: 'FAVORITES', name: 'Most Popular' },
    ];
    const ratingOptions = [
        { id: 'false', name: 'Safe' },
        { id: 'true', name: 'NSFW' },
        { id: 'null', name: 'All' }
    ];

    if (!user) return <div className="p-10 text-center">Please log in.</div>;
    if (loading && !album) return <div className="p-10 text-center">Loading album...</div>;

    return (
        <div className="flex h-full relative overflow-hidden">

            {/* Content Area */}
            <div className="flex-1 overflow-y-auto h-full p-4 md:p-6">
                <div className="flex flex-col md:flex-row md:items-center justify-between mb-8 gap-4">
                    <div className="flex items-center gap-4">
                        <Link to="/albums" className="p-2 rounded-full bg-secondary hover:bg-secondary/80 transition-colors">
                            <ChevronLeft size={24} />
                        </Link>
                        <div>
                            <div className="flex items-center gap-3">
                                <h1 className="text-3xl font-black flex items-center gap-2 text-foreground">
                                    <FolderOpen className="text-primary" /> {album?.name || "Album"}
                                </h1>
                                {/* Edit Button */}
                                <button onClick={openEditModal} className="p-1.5 rounded-full hover:bg-secondary text-muted-foreground hover:text-foreground">
                                    <Edit2 size={16} />
                                </button>
                            </div>
                            <p className="text-muted-foreground mt-1">{album?.description || `${images?.length || 0} images`}</p>
                        </div>
                    </div>
                    <button onClick={() => setShowFilters(!showFilters)} className="flex items-center gap-2 px-4 py-2 bg-card border border-border rounded-lg text-sm font-medium shadow-sm hover:bg-secondary transition-colors self-start md:self-auto">
                        <SlidersHorizontal size={16} /> Filters
                    </button>
                </div>

                {(!images || images.length === 0) && !loading ? (
                    <div className="flex flex-col items-center justify-center h-64 text-muted-foreground border-2 border-dashed border-border rounded-xl bg-card/50">
                        <p className="text-lg font-medium">This album is empty.</p>
                        <Link to="/gallery" className="mt-2 text-primary font-bold hover:underline">Browse Gallery to add images</Link>
                    </div>
                ) : (
                    <div className="columns-2 sm:columns-3 md:columns-4 xl:columns-5 gap-4 space-y-4 pb-10">
                        {images?.map((img) => (
                            <div key={img.id} className="break-inside-avoid relative group">
                                <ImageCard image={img} />
                                <button
                                    onClick={(e) => { e.preventDefault(); removeImage(img.id); }}
                                    className="absolute top-2 right-2 p-2 bg-destructive/90 text-white rounded-full opacity-0 group-hover:opacity-100 transition-opacity shadow-sm hover:bg-destructive transform hover:scale-110 duration-200"
                                    title="Remove from Album"
                                >
                                    <Trash2 size={14} />
                                </button>
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {/* Sidebar Filter */}
            <aside className={`
        fixed inset-y-0 right-0 z-40 w-80 bg-card border-l border-border shadow-2xl 
        transform transition-transform duration-300 ease-in-out h-full p-6 flex flex-col
        ${showFilters ? 'translate-x-0' : 'translate-x-full'}
      `}>
                <h3 className="font-bold mb-6 text-xl">Album Options</h3>
                <div className="space-y-6 flex-1">
                    <SearchableSelect
                        label="Sort By"
                        options={sortOptions}
                        selectedOptions={sortOptions.filter(o => o.id === orderBy)}
                        onSelect={(o) => updateFilter('orderBy', o.id as string)}
                        onRemove={() => {}}
                        isMulti={false}
                        clearable={false}
                    />
                    <SearchableSelect
                        label="Content Rating"
                        options={ratingOptions}
                        selectedOptions={ratingOptions.filter(o => o.id === isNsfw)}
                        onSelect={(o) => updateFilter('isNsfw', o.id as string)}
                        onRemove={() => {}}
                        isMulti={false}
                        clearable={false}
                    />
                </div>
                <button onClick={() => setShowFilters(false)} className="mt-auto w-full py-3 bg-secondary rounded-xl font-bold">Close</button>
            </aside>

            {/* Edit Album Modal */}
            <Modal isOpen={isEditOpen} onClose={() => setIsEditOpen(false)} title="Edit Album Details">
                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-bold mb-1">Name</label>
                        <input
                            value={editFormData.name}
                            onChange={e => setEditFormData({...editFormData, name: e.target.value})}
                            className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-bold mb-1">Description</label>
                        <textarea
                            value={editFormData.description}
                            onChange={e => setEditFormData({...editFormData, description: e.target.value})}
                            className="w-full p-3 bg-secondary rounded-lg outline-none h-24 resize-none text-foreground"
                        />
                    </div>
                    <button onClick={handleEditAlbum} className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg mt-2">
                        Save Changes
                    </button>
                </div>
            </Modal>

        </div>
    );
};

export default AlbumPage;
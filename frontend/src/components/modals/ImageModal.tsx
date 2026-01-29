import { useState, useEffect } from 'react';
import Modal from '../Modal';
import { ImageDto, ImageFormData, Tag, Artist, PaginatedList } from '../../types';
import SearchableSelect from '../SearchableSelect';
import api from '../../services/api';

interface Option {
    id: number | string;
    name: string;
    description?: string;
}

interface ImageModalProps {
    isOpen: boolean;
    onClose: () => void;
    initialData?: ImageDto;
    onSubmit: (data: ImageFormData) => Promise<void> | void;
}

const ImageModal = ({ isOpen, onClose, initialData, onSubmit }: ImageModalProps) => {
    const [source, setSource] = useState('');
    const [isNsfw, setIsNsfw] = useState(false);
    const [userId, setUserId] = useState<string>('');
    
    const [tags, setTags] = useState<Option[]>([]);
    const [artists, setArtists] = useState<Option[]>([]);
    const [selectedTags, setSelectedTags] = useState<Option[]>([]);
    const [selectedArtists, setSelectedArtists] = useState<Option[]>([]);

    useEffect(() => {
        if (isOpen) {
            const fetchData = async () => {
                try {
                    const [tagsRes, artistsRes] = await Promise.all([
                        api.get<PaginatedList<Tag>>('/tags', { params: { pageSize: 1000, reviewStatus: 1 } }),
                        api.get<PaginatedList<Artist>>('/artists', { params: { pageSize: 1000, reviewStatus: 1 } })
                    ]);
                    setTags(tagsRes.data.items.map(t => ({ id: t.id, name: t.name, description: t.description })));
                    setArtists(artistsRes.data.items.map(a => ({ id: a.id, name: a.name })));
                } catch (e) { console.error("Failed to fetch tags/artists", e); }
            };
            fetchData();
        }
    }, [isOpen]);

    useEffect(() => {
        if (initialData) {
            setSource(initialData.source || '');
            setIsNsfw(initialData.isNsfw);
            setUserId(initialData.uploaderId?.toString() || '');
            setSelectedTags(initialData.tags ? initialData.tags.map(t => ({ id: t.id, name: t.name, description: t.description })) : []);
            setSelectedArtists(initialData.artists ? initialData.artists.map(a => ({ id: a.id, name: a.name })) : []);
        } else {
            setSource('');
            setIsNsfw(false);
            setUserId('');
            setSelectedTags([]);
            setSelectedArtists([]);
        }
    }, [initialData, isOpen]);

    const handleSubmit = async (e?: React.FormEvent) => {
        if (e) e.preventDefault();
        await onSubmit({
            source: source || undefined,
            isNsfw,
            tagIds: selectedTags.map(t => Number(t.id)),
            artistIds: selectedArtists.map(a => Number(a.id)),
            userId: userId ? parseInt(userId) : undefined,
        });
        onClose();
    };

    return (
        <Modal
            isOpen={isOpen}
            onClose={onClose}
            title={initialData ? `Edit Image #${initialData.id}` : 'Edit Image'}
        >
            <div className="space-y-5">
                <SearchableSelect
                    label="Artists"
                    placeholder="Search artists..."
                    options={artists}
                    selectedOptions={selectedArtists}
                    onSelect={(o) => setSelectedArtists(p => p.some(a => a.id === o.id) ? p : [...p, o])}
                    onRemove={(o) => setSelectedArtists(p => p.filter(a => a.id !== o.id))}
                    isMulti={true}
                />
                <SearchableSelect
                    label="Tags"
                    placeholder="Add tags..."
                    options={tags}
                    selectedOptions={selectedTags}
                    onSelect={(o) => setSelectedTags(p => p.some(t => t.id === o.id) ? p : [...p, o])}
                    onRemove={(o) => setSelectedTags(p => p.filter(t => t.id !== o.id))}
                    isMulti={true}
                />
                <div>
                    <label className="block text-sm font-bold mb-1.5 text-muted-foreground uppercase tracking-wider">Source URL</label>
                    <input
                        type="text"
                        value={source}
                        onChange={e => setSource(e.target.value)}
                        className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground transition-all placeholder:text-muted-foreground/50"
                        placeholder="https://..."
                    />
                </div>

                <div className="flex items-center gap-3 p-3 bg-secondary/50 rounded-lg border border-transparent hover:border-input transition-colors">
                    <input
                        type="checkbox"
                        id="isNsfw"
                        checked={isNsfw}
                        onChange={e => setIsNsfw(e.target.checked)}
                        className="h-5 w-5 text-primary focus:ring-primary rounded bg-card border-input cursor-pointer"
                    />
                    <label htmlFor="isNsfw" className="font-bold cursor-pointer select-none flex-1">Is NSFW Content?</label>
                </div>

                <div>
                    <label className="block text-sm font-bold mb-1.5 text-muted-foreground uppercase tracking-wider">Uploader User ID</label>
                    <input
                        type="number"
                        value={userId}
                        onChange={e => setUserId(e.target.value)}
                        className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground transition-all"
                        placeholder="e.g. 1"
                    />
                </div>

                <button
                    onClick={() => handleSubmit()}
                    className="w-full bg-primary text-primary-foreground py-3.5 rounded-xl font-bold hover:bg-primary/90 transition-all shadow-lg shadow-primary/20 mt-2"
                >
                    Save Changes
                </button>
            </div>
        </Modal>
    );
};

export default ImageModal;
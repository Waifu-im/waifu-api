import { useState, useEffect } from 'react';
import Modal from '../Modal';
import { ImageDto, ImageFormData, PaginatedList, User } from '../../types';
import SearchableSelect, { Option } from '../SearchableSelect';
import api from '../../services/api';
import TagModal from './TagModal';
import ArtistModal from './ArtistModal';
import { useMetadata } from '../../hooks/useMetadata';

interface ImageModalProps {
    isOpen: boolean;
    onClose: () => void;
    initialData?: ImageDto;
    onSubmit: (data: ImageFormData) => Promise<void> | void;
}

const ImageModal = ({ isOpen, onClose, initialData, onSubmit }: ImageModalProps) => {
    const [source, setSource] = useState('');
    const [isNsfw, setIsNsfw] = useState(false);
    const [selectedUser, setSelectedUser] = useState<Option | null>(null);

    const {
        selectedTags, setSelectedTags,
        selectedArtists, setSelectedArtists,
        showCreateTagModal, setShowCreateTagModal,
        newTagName, setNewTagName,
        showCreateArtistModal, setShowCreateArtistModal,
        newArtistName, setNewArtistName,
        loadTags, loadArtists,
        handleCreateTag, handleCreateArtist
    } = useMetadata();

    useEffect(() => {
        if (isOpen && initialData) {
            setSource(initialData.source || '');
            setIsNsfw(initialData.isNsfw);
            setSelectedTags(initialData.tags ? initialData.tags.map(t => ({ id: t.id, name: t.name, slug: t.slug, description: t.description })) : []);
            setSelectedArtists(initialData.artists ? initialData.artists.map(a => ({ id: a.id, name: a.name })) : []);

            if (initialData.uploaderId) {
                api.get<User>(`/users/${initialData.uploaderId}`).then(res => {
                     setSelectedUser({ id: res.data.id, name: res.data.name });
                }).catch(() => {
                    setSelectedUser({ id: initialData.uploaderId, name: `User #${initialData.uploaderId}` });
                });
            } else {
                setSelectedUser(null);
            }
        } else if (isOpen) {
            setSource('');
            setIsNsfw(false);
            setSelectedTags([]);
            setSelectedArtists([]);
            setSelectedUser(null);
        }
    }, [initialData, isOpen]);

    const loadUsers = async (query: string) => {
        const { data } = await api.get<PaginatedList<User>>('/users', { params: { search: query, pageSize: 20 } });
        return data.items.map(u => ({ id: u.id, name: u.name }));
    };

    const handleSubmit = async (e?: React.FormEvent) => {
        if (e) e.preventDefault();
        await onSubmit({
            source: source || undefined,
            isNsfw,
            tags: selectedTags.map(t => t.slug!),
            artists: selectedArtists.map(a => Number(a.id)),
            userId: selectedUser ? Number(selectedUser.id) : undefined,
        });
        onClose();
    };

    return (
        <>
            <Modal
                isOpen={isOpen}
                onClose={onClose}
                title={initialData ? `Edit Image #${initialData.id}` : 'Edit Image'}
            >
                <div className="space-y-5">
                    <SearchableSelect
                        label="Artists"
                        placeholder="Search artists..."
                        loadOptions={loadArtists}
                        selectedOptions={selectedArtists}
                        onSelect={(o) => setSelectedArtists(p => p.some(a => a.id === o.id) ? p : [...p, o])}
                        onRemove={(o) => setSelectedArtists(p => p.filter(a => a.id !== o.id))}
                        onCreate={(name) => { setNewArtistName(name); setShowCreateArtistModal(true); }}
                        isMulti={true}
                    />
                    <SearchableSelect
                        label="Tags"
                        placeholder="Add tags..."
                        loadOptions={loadTags}
                        selectedOptions={selectedTags}
                        onSelect={(o) => setSelectedTags(p => p.some(t => t.id === o.id) ? p : [...p, o])}
                        onRemove={(o) => setSelectedTags(p => p.filter(t => t.id !== o.id))}
                        onCreate={(name) => { setNewTagName(name); setShowCreateTagModal(true); }}
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

                    <label className="flex items-center gap-3 p-4 border border-border rounded-xl cursor-pointer hover:bg-secondary/50 transition-colors">
                        <input
                            type="checkbox"
                            id="isNsfw"
                            checked={isNsfw}
                            onChange={e => setIsNsfw(e.target.checked)}
                            className="w-5 h-5 rounded text-destructive focus:ring-destructive"
                        />
                        <div>
                            <span className="block font-bold">NSFW Content</span>
                            <span className="text-xs text-muted-foreground">Contains adult material</span>
                        </div>
                    </label>

                    <SearchableSelect
                        label="Uploader"
                        placeholder="Search and select a user..."
                        loadOptions={loadUsers}
                        selectedOptions={selectedUser ? [selectedUser] : []}
                        onSelect={(o) => setSelectedUser(o)}
                        onRemove={() => setSelectedUser(null)}
                        isMulti={false}
                    />

                    <button
                        onClick={() => handleSubmit()}
                        className="w-full bg-primary text-primary-foreground py-3.5 rounded-xl font-bold hover:bg-primary/90 transition-all shadow-lg shadow-primary/20 mt-2"
                    >
                        Save Changes
                    </button>
                </div>
            </Modal>

            <TagModal
                isOpen={showCreateTagModal}
                onClose={() => setShowCreateTagModal(false)}
                onSubmit={handleCreateTag}
                initialData={{ name: newTagName }}
                title="New Tag"
                submitLabel="Create"
            />

            <ArtistModal
                isOpen={showCreateArtistModal}
                onClose={() => setShowCreateArtistModal(false)}
                onSubmit={handleCreateArtist}
                initialData={{ name: newArtistName }}
                title="New Artist"
                submitLabel="Create"
            />
        </>
    );
};

export default ImageModal;
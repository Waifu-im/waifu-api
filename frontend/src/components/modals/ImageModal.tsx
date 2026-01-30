import { useState, useEffect } from 'react';
import Modal from '../Modal';
import { ImageDto, ImageFormData, PaginatedList, User, ReviewStatus, Role } from '../../types';
import SearchableSelect, { Option } from '../SearchableSelect';
import api from '../../services/api';
import TagModal from './TagModal';
import ArtistModal from './ArtistModal';
import { useMetadata } from '../../hooks/useMetadata';
import { Dropdown, DropdownItem } from '../Dropdown';
import { Check, Clock, ChevronDown, Trash2 } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';

interface ImageModalProps {
    isOpen: boolean;
    onClose: () => void;
    initialData?: ImageDto;
    onSubmit: (data: ImageFormData) => Promise<void> | void;
    onDelete?: () => void;
}

const ImageModal = ({ isOpen, onClose, initialData, onSubmit, onDelete }: ImageModalProps) => {
    const { user } = useAuth();
    const [source, setSource] = useState('');
    const [isNsfw, setIsNsfw] = useState(false);
    const [selectedUser, setSelectedUser] = useState<Option | null>(null);
    const [reviewStatus, setReviewStatus] = useState<ReviewStatus>(ReviewStatus.Pending);

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

    const isAdmin = user?.role === Role.Admin;

    useEffect(() => {
        if (isOpen && initialData) {
            setSource(initialData.source || '');
            setIsNsfw(initialData.isNsfw);
            setSelectedTags(initialData.tags ? initialData.tags.map(t => ({ id: t.id, name: t.name, slug: t.slug, description: t.description })) : []);
            setSelectedArtists(initialData.artists ? initialData.artists.map(a => ({ id: a.id, name: a.name })) : []);
            setReviewStatus(initialData.reviewStatus ?? ReviewStatus.Pending);

            if (initialData.uploaderId) {
                // Try to fetch user details, but don't block the modal if it fails
                api.get<User>(`/users/${initialData.uploaderId}`, { skipGlobalErrorHandler: true })
                    .then(res => {
                         setSelectedUser({ id: res.data.id, name: res.data.name });
                    })
                    .catch(() => {
                        // Fallback if user not found or error
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
            setReviewStatus(ReviewStatus.Pending);
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
            reviewStatus: reviewStatus
        });
        onClose();
    };

    const getStatusBadge = (status?: ReviewStatus) => {
        switch (status) {
            case ReviewStatus.Pending: return <span className="bg-yellow-500/10 text-yellow-600 px-2 py-1 rounded text-xs font-bold border border-yellow-500/20 flex items-center gap-1"><Clock size={12}/> Pending</span>;
            case ReviewStatus.Accepted: return <span className="bg-green-500/10 text-green-600 px-2 py-1 rounded text-xs font-bold border border-green-500/20 flex items-center gap-1"><Check size={12}/> Accepted</span>;
            default: return <span className="text-muted-foreground text-xs">Select Status</span>;
        }
    };

    const renderStatusDropdown = () => {
        return (
            <Dropdown 
                trigger={
                    <div className="w-full p-3 bg-secondary rounded-lg flex items-center justify-between cursor-pointer hover:bg-secondary/80 transition-colors border border-transparent focus-within:border-primary">
                        {getStatusBadge(reviewStatus)}
                        <ChevronDown size={16} className="text-muted-foreground"/>
                    </div>
                }
                align="left"
                width="w-full"
            >
                <DropdownItem 
                    onClick={() => setReviewStatus(ReviewStatus.Pending)}
                    active={reviewStatus === ReviewStatus.Pending}
                    icon={<Clock size={14}/>}
                >
                    Pending
                </DropdownItem>
                <DropdownItem 
                    onClick={() => setReviewStatus(ReviewStatus.Accepted)}
                    active={reviewStatus === ReviewStatus.Accepted}
                    icon={<Check size={14}/>}
                >
                    Accepted
                </DropdownItem>
            </Dropdown>
        );
    };

    return (
        <>
            <Modal
                isOpen={isOpen}
                onClose={onClose}
                title={initialData ? `Edit Image #${initialData.id}` : 'Edit Image'}
            >
                <div className="space-y-5">
                    <div>
                        <label className="block text-sm font-bold mb-1">Review Status</label>
                        {renderStatusDropdown()}
                    </div>

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

                    <div className="flex gap-3 pt-2">
                        <button
                            onClick={() => handleSubmit()}
                            className="flex-1 bg-primary text-primary-foreground py-3.5 rounded-xl font-bold hover:bg-primary/90 transition-all shadow-lg shadow-primary/20"
                        >
                            Save Changes
                        </button>
                        {onDelete && isAdmin && (
                            <button
                                onClick={onDelete}
                                className="px-4 py-3.5 bg-secondary text-red-600 hover:bg-red-500/10 font-bold rounded-xl flex items-center justify-center gap-2 transition-colors"
                            >
                                <Trash2 size={20} />
                            </button>
                        )}
                    </div>
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
import { useState, useEffect } from 'react';
import Modal from '../Modal';
import { ImageDto, ImageFormData, PaginatedList, User, ReviewStatus, Role } from '../../types';
import SearchableSelect, { Option } from '../SearchableSelect';
import api from '../../services/api';
import TagModal from './TagModal';
import ArtistModal from './ArtistModal';
import { useMetadata } from '../../hooks/useMetadata';
import { Dropdown, DropdownItem } from '../Dropdown';
import { Check, Clock, ChevronDown, Trash2, Upload, Loader2 } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';

interface ImageModalProps {
    isOpen: boolean;
    onClose: () => void;
    initialData?: ImageDto;
    onSubmit: (data: ImageFormData) => Promise<void> | void;
    onDelete?: () => void;
}

const ImageModal = ({ isOpen, onClose, initialData, onSubmit, onDelete }: ImageModalProps) => {
    const usersPageSize = 30;
    const { user } = useAuth();
    const [source, setSource] = useState('');
    const [isNsfw, setIsNsfw] = useState(false);
    const [selectedUser, setSelectedUser] = useState<Option | null>(null);
    const [reviewStatus, setReviewStatus] = useState<ReviewStatus>(ReviewStatus.Pending);
    const [selectedFile, setSelectedFile] = useState<File | null>(null);
    const [previewUrl, setPreviewUrl] = useState<string | null>(null);
    const [isSubmitting, setIsSubmitting] = useState(false);

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
            setSelectedFile(null);
            setPreviewUrl(null);
            setIsSubmitting(false);

            if (initialData.uploaderId) {
                const uploaderId = initialData.uploaderId;
                // Try to fetch user details, but don't block the modal if it fails
                api.get<User>(`/users/${uploaderId}`, { skipGlobalErrorHandler: true })
                    .then(res => {
                         setSelectedUser({ id: res.data.id, name: res.data.name });
                    })
                    .catch(() => {
                        // Fallback if user not found or error
                        setSelectedUser({ id: uploaderId, name: `User #${uploaderId}` });
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
            setSelectedFile(null);
            setPreviewUrl(null);
            setIsSubmitting(false);
        }
    }, [initialData, isOpen]);

    // Handle file preview URL
    useEffect(() => {
        if (selectedFile) {
            const url = URL.createObjectURL(selectedFile);
            setPreviewUrl(url);
            return () => URL.revokeObjectURL(url);
        } else {
            setPreviewUrl(null);
        }
    }, [selectedFile]);

    const loadUsers = async (query: string) => {
        const { data } = await api.get<PaginatedList<User>>('/users', { params: { name: query, pageSize: usersPageSize } });
        return data.items.map(u => ({ id: u.id, name: u.name }));
    };

    const handleSubmit = async (e?: React.FormEvent) => {
        if (e) e.preventDefault();
        if (isSubmitting) return;

        setIsSubmitting(true);
        try {
            await onSubmit({
                file: selectedFile || undefined,
                source: source || undefined,
                isNsfw,
                tags: selectedTags.map(t => t.slug!),
                artists: selectedArtists.map(a => Number(a.id)),
                userId: selectedUser ? Number(selectedUser.id) : undefined,
                reviewStatus: reviewStatus
            });
            onClose();
        } catch (err) {
            // Error handled by parent
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (file) {
            setSelectedFile(file);
        }
    };

    const clearSelectedFile = () => {
        setSelectedFile(null);
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
                    {/* Image Preview & File Upload */}
                    <div>
                        <label className="block text-sm font-bold mb-1.5 text-muted-foreground uppercase tracking-wider">Image</label>
                        <div className="relative">
                            <label className="block w-full h-48 border-2 border-dashed border-border rounded-xl cursor-pointer overflow-hidden hover:border-primary/50 transition-colors group">
                                {previewUrl ? (
                                    <div className="relative w-full h-full">
                                        <img src={previewUrl} className="w-full h-full object-contain" alt="New image preview" />
                                        <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center text-white font-medium">
                                            Click to change
                                        </div>
                                    </div>
                                ) : initialData?.url ? (
                                    <div className="relative w-full h-full">
                                        <img src={initialData.url} className="w-full h-full object-contain" alt="Current image" />
                                        <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center text-white font-medium">
                                            <Upload size={20} className="mr-2" /> Replace image
                                        </div>
                                    </div>
                                ) : (
                                    <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
                                        <Upload size={24} className="mb-2" />
                                        <span className="text-sm">Click to upload</span>
                                    </div>
                                )}
                                <input
                                    type="file"
                                    accept="image/jpeg,image/png,image/gif,image/webp"
                                    onChange={handleFileChange}
                                    className="hidden"
                                />
                            </label>
                            {selectedFile && (
                                <button
                                    type="button"
                                    onClick={clearSelectedFile}
                                    className="absolute top-2 right-2 bg-destructive text-white p-1.5 rounded-full hover:bg-destructive/90 transition-colors"
                                    title="Remove new image"
                                >
                                    <Trash2 size={14} />
                                </button>
                            )}
                        </div>
                        {selectedFile && (
                            <p className="text-xs text-muted-foreground mt-2">
                                New file: {selectedFile.name} ({(selectedFile.size / (1024 * 1024)).toFixed(2)} MB)
                            </p>
                        )}
                    </div>

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
                            disabled={isSubmitting}
                            className="flex-1 bg-primary text-primary-foreground py-3.5 rounded-xl font-bold hover:bg-primary/90 transition-all shadow-lg shadow-primary/20 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                        >
                            {isSubmitting ? (
                                <>
                                    <Loader2 size={18} className="animate-spin" />
                                    Saving...
                                </>
                            ) : (
                                'Save Changes'
                            )}
                        </button>
                        {onDelete && isAdmin && (
                            <button
                                onClick={onDelete}
                                disabled={isSubmitting}
                                className="px-4 py-3.5 bg-secondary text-red-600 hover:bg-red-500/10 font-bold rounded-xl flex items-center justify-center gap-2 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
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
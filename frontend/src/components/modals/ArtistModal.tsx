import { useState, useEffect, useRef } from 'react';
import Modal from '../Modal';
import { Info, Loader2, Check, Clock, ChevronDown, Trash2, GitPullRequest } from 'lucide-react';
import { ReviewStatus, Role, PaginatedList, User, Artist, ReviewableContentType, SubmitEditBody } from '../../types';
import { Dropdown, DropdownItem } from '../Dropdown';
import { useAuth } from '../../context/AuthContext';
import { useNotification } from '../../context/NotificationContext';
import SearchableSelect, { Option } from '../SearchableSelect';
import api from '../../services/api';
import { buildArtistPayload } from '../../utils/editRequests';
import AcceptSubmissionNote from '../AcceptSubmissionNote';

export interface ArtistFormData {
    name: string;
    twitter?: string;
    pixiv?: string;
    patreon?: string;
    deviantArt?: string;
    reviewStatus?: ReviewStatus;
    creatorId?: number;
}

interface ArtistModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (data: ArtistFormData) => Promise<void> | void;
    initialData?: Partial<ArtistFormData> & { id?: number; creatorId?: number };
    title: string;
    isReviewMode?: boolean;
    submitLabel?: string;
    onDelete?: () => void;
    /** When provided, the modal acts as a "suggest an edit" form and POSTs an edit request itself. */
    suggestContext?: { targetType: ReviewableContentType; targetId: number; current: Artist };
    /** Content-only edit (moderating a pending submission): hides review status and creator; submits via onSubmit. */
    contentOnly?: boolean;
}

const ArtistModal = ({ isOpen, onClose, onSubmit, initialData, title, isReviewMode, submitLabel = "Save", onDelete, suggestContext, contentOnly }: ArtistModalProps) => {
    const { user } = useAuth();
    const { showNotification } = useNotification();
    const [formData, setFormData] = useState<ArtistFormData>({
        name: '',
        twitter: '',
        pixiv: '',
        patreon: '',
        deviantArt: '',
        reviewStatus: ReviewStatus.Pending
    });
    const [selectedCreator, setSelectedCreator] = useState<Option | null>(null);
    const [reason, setReason] = useState('');
    const [isSubmitting, setIsSubmitting] = useState(false);
    const hasInitialized = useRef(false);

    const isSuggestMode = !!suggestContext;
    const isEditMode = !!initialData?.id && !isSuggestMode && !contentOnly;
    const isAdmin = user?.role === Role.Admin;

    useEffect(() => {
        if (!isOpen) {
            hasInitialized.current = false;
            return;
        }
        if (hasInitialized.current) return;
        hasInitialized.current = true;

        setFormData({
            name: initialData?.name || '',
            twitter: initialData?.twitter || '',
            pixiv: initialData?.pixiv || '',
            patreon: initialData?.patreon || '',
            deviantArt: initialData?.deviantArt || '',
            reviewStatus: initialData?.reviewStatus ?? ReviewStatus.Pending
        });
        setIsSubmitting(false);
        setReason('');

        if (initialData?.creatorId) {
            const creatorId = initialData.creatorId;
            api.get<User>(`/users/${creatorId}`, { skipGlobalErrorHandler: true })
                .then(res => setSelectedCreator({ id: res.data.id, name: res.data.name }))
                .catch(() => setSelectedCreator({ id: creatorId, name: `User #${creatorId}` }));
        } else {
            setSelectedCreator(null);
        }
    }, [isOpen, initialData]);

    const loadUsers = async (query: string, page: number = 1) => {
        const { data } = await api.get<PaginatedList<User>>('/users', { params: { name: query, pageSize: 30, page } });
        return data.items.map(u => ({ id: u.id, name: u.name }));
    };

    const handleSubmit = async () => {
        if (isSubmitting) return;

        if (isSuggestMode && suggestContext) {
            const { payload, isEmpty } = buildArtistPayload(suggestContext.current, {
                name: formData.name,
                patreon: formData.patreon,
                pixiv: formData.pixiv,
                twitter: formData.twitter,
                deviantArt: formData.deviantArt,
            });
            if (isEmpty) {
                showNotification('warning', 'No changes to suggest');
                return;
            }
            setIsSubmitting(true);
            try {
                const body: SubmitEditBody = {
                    targetType: suggestContext.targetType,
                    targetId: suggestContext.targetId,
                    reason: reason.trim() || undefined,
                    payload,
                };
                await api.post('/review/tasks', body);
                showNotification('success', 'Suggestion submitted for review');
                onClose();
            } catch {
                // Error toast handled globally; keep modal open so the user can retry.
            } finally {
                setIsSubmitting(false);
            }
            return;
        }

        setIsSubmitting(true);
        try {
            await onSubmit({
                ...formData,
                creatorId: selectedCreator ? Number(selectedCreator.id) : undefined
            });
        } finally {
            setIsSubmitting(false);
        }
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
                        {getStatusBadge(formData.reviewStatus)}
                        <ChevronDown size={16} className="text-muted-foreground"/>
                    </div>
                }
                align="left"
                width="w-full"
            >
                <DropdownItem 
                    onClick={() => setFormData({...formData, reviewStatus: ReviewStatus.Pending})}
                    active={formData.reviewStatus === ReviewStatus.Pending}
                    icon={<Clock size={14}/>}
                >
                    Pending
                </DropdownItem>
                <DropdownItem 
                    onClick={() => setFormData({...formData, reviewStatus: ReviewStatus.Accepted})}
                    active={formData.reviewStatus === ReviewStatus.Accepted}
                    icon={<Check size={14}/>}
                >
                    Accepted
                </DropdownItem>
            </Dropdown>
        );
    };

    return (
        <Modal 
            isOpen={isOpen} 
            onClose={onClose} 
            title={title}
        >
            <div className="space-y-4">
                {isSuggestMode && (
                    <div className="bg-blue-500/10 border border-blue-500/20 rounded-xl p-4 flex items-start gap-3">
                        <GitPullRequest className="text-blue-500 shrink-0 mt-0.5" size={20} />
                        <div>
                            <h4 className="font-bold text-blue-600 dark:text-blue-400 text-sm">Suggesting an edit</h4>
                            <p className="text-xs text-muted-foreground mt-1">
                                Your changes will be submitted to the moderators for review.
                            </p>
                        </div>
                    </div>
                )}

                {isReviewMode && !isEditMode && !isSuggestMode && (
                    <div className="bg-blue-500/10 border border-blue-500/20 rounded-xl p-4 flex items-start gap-3">
                        <Info className="text-blue-500 shrink-0 mt-0.5" size={20} />
                        <div>
                            <h4 className="font-bold text-blue-600 dark:text-blue-400 text-sm">Review Required</h4>
                            <p className="text-xs text-muted-foreground mt-1">
                                New artists must be approved by a moderator before appearing in public lists.
                            </p>
                        </div>
                    </div>
                )}

                {/* Always show review status in edit mode */}
                {isEditMode && (
                    <div>
                        <label className="block text-sm font-bold mb-1">Review Status</label>
                        {renderStatusDropdown()}
                        <AcceptSubmissionNote current={initialData?.reviewStatus} selected={formData.reviewStatus} />
                    </div>
                )}

                {isEditMode && (
                    <SearchableSelect
                        label="Creator"
                        placeholder="Search and select a user..."
                        loadOptions={loadUsers}
                        selectedOptions={selectedCreator ? [selectedCreator] : []}
                        onSelect={(o) => setSelectedCreator(o)}
                        onRemove={() => setSelectedCreator(null)}
                        isMulti={false}
                    />
                )}

                <div>
                    <label className="block text-sm font-bold mb-1">Name</label>
                    <input
                        value={formData.name}
                        onChange={e => setFormData({...formData, name: e.target.value})}
                        className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground"
                        placeholder="Artist Name"
                        disabled={isSubmitting}
                    />
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    {['twitter', 'pixiv', 'patreon', 'deviantArt'].map((social) => (
                        <div key={social}>
                            <label className="block text-xs font-bold uppercase text-muted-foreground mb-1">{social}</label>
                            <input
                                type="url"
                                placeholder={`https://${social}.com/...`}
                                value={(formData as any)[social]}
                                onChange={e => setFormData({...formData, [social]: e.target.value})}
                                className="w-full p-2 bg-secondary rounded text-sm outline-none focus:ring-1 focus:ring-primary text-foreground"
                                disabled={isSubmitting}
                            />
                        </div>
                    ))}
                </div>

                {isSuggestMode && (
                    <div>
                        <label className="block text-sm font-bold mb-1">Reason <span className="text-xs font-normal text-muted-foreground">(optional)</span></label>
                        <textarea
                            value={reason}
                            onChange={e => setReason(e.target.value)}
                            className="w-full p-3 bg-secondary rounded-lg outline-none h-20 resize-none text-foreground"
                            placeholder="Why are you suggesting this change?"
                            disabled={isSubmitting}
                        />
                    </div>
                )}

                <div className="flex gap-3 pt-2">
                    <button
                        onClick={handleSubmit}
                        disabled={isSubmitting}
                        className="flex-1 py-3 bg-primary text-primary-foreground font-bold rounded-lg flex items-center justify-center gap-2 disabled:opacity-70"
                    >
                        {isSubmitting && <Loader2 size={18} className="animate-spin" />}
                        {isSuggestMode ? 'Submit suggestion' : submitLabel}
                    </button>
                    {onDelete && isAdmin && (
                        <button
                            onClick={onDelete}
                            disabled={isSubmitting}
                            className="px-4 py-3 bg-secondary text-red-600 hover:bg-red-500/10 font-bold rounded-lg flex items-center justify-center gap-2 disabled:opacity-70 transition-colors"
                        >
                            <Trash2 size={18} />
                        </button>
                    )}
                </div>
            </div>
        </Modal>
    );
};

export default ArtistModal;
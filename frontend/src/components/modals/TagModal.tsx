import { useState, useEffect } from 'react';
import Modal from '../Modal';
import { Info, Loader2, Link as LinkIcon } from 'lucide-react';

export interface TagFormData {
    name: string;
    slug: string;
    description: string;
}

const slugify = (text: string) => {
    return text
        .toString()
        .toLowerCase()
        .normalize('NFD')
        .replace(/[\u0300-\u036f]/g, '')
        .replace(/\s+/g, '-')
        .replace(/[^\w-]+/g, '')
        .replace(/--+/g, '-');
};

interface TagModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (data: TagFormData) => Promise<void> | void;
    initialData?: Partial<TagFormData>;
    title: string;
    isReviewMode?: boolean;
    submitLabel?: string;
}

const TagModal = ({ isOpen, onClose, onSubmit, initialData, title, isReviewMode, submitLabel = "Save" }: TagModalProps) => {
    const [formData, setFormData] = useState<TagFormData>({ name: '', slug: '', description: '' });
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [isSlugManuallyEdited, setIsSlugManuallyEdited] = useState(false);

    useEffect(() => {
        if (isOpen) {
            setFormData({
                name: initialData?.name || '',
                slug: initialData?.slug || '',
                description: initialData?.description || ''
            });
            setIsSubmitting(false);
            setIsSlugManuallyEdited(!!initialData?.slug);
        }
    }, [isOpen, initialData]);

    const handleNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const name = e.target.value;
        setFormData(prev => ({
            ...prev,
            name,
            slug: isSlugManuallyEdited ? prev.slug : slugify(name)
        }));
    };

    const handleSlugChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setFormData(prev => ({ ...prev, slug: e.target.value }));
        setIsSlugManuallyEdited(true);
    };

    const handleSubmit = async () => {
        if (isSubmitting) return;
        setIsSubmitting(true);
        try {
            await onSubmit(formData);
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <Modal isOpen={isOpen} onClose={onClose} title={title}>
            <div className="space-y-4">
                {isReviewMode && (
                    <div className="bg-blue-500/10 border border-blue-500/20 rounded-xl p-4 flex items-start gap-3">
                        <Info className="text-blue-500 shrink-0 mt-0.5" size={20} />
                        <div>
                            <h4 className="font-bold text-blue-600 dark:text-blue-400 text-sm">Review Required</h4>
                            <p className="text-xs text-muted-foreground mt-1">
                                New tags must be approved by a moderator.
                            </p>
                        </div>
                    </div>
                )}

                <div>
                    <label className="block text-sm font-bold mb-1">Name</label>
                    <input
                        value={formData.name}
                        onChange={handleNameChange}
                        className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground"
                        placeholder="Tag name"
                        disabled={isSubmitting}
                    />
                </div>

                <div>
                    <label className="block text-sm font-bold mb-1 flex items-center gap-2">
                        Slug <span className="text-xs font-normal text-muted-foreground">(URL Friendly ID)</span>
                    </label>
                    <div className="relative">
                        <LinkIcon size={16} className="absolute left-3 top-3.5 text-muted-foreground"/>
                        <input
                            value={formData.slug}
                            onChange={handleSlugChange}
                            className="w-full pl-9 p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground font-mono text-sm"
                            placeholder="tag-slug"
                            disabled={isSubmitting}
                        />
                    </div>
                </div>

                <div>
                    <label className="block text-sm font-bold mb-1">Description</label>
                    <textarea
                        value={formData.description}
                        onChange={e => setFormData({...formData, description: e.target.value})}
                        className="w-full p-3 bg-secondary rounded-lg outline-none h-24 resize-none text-foreground"
                        placeholder="Description..."
                        disabled={isSubmitting}
                    />
                </div>
                <button
                    onClick={handleSubmit}
                    disabled={isSubmitting}
                    className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg mt-2 flex items-center justify-center gap-2 disabled:opacity-70"
                >
                    {isSubmitting && <Loader2 size={18} className="animate-spin" />}
                    {submitLabel}
                </button>
            </div>
        </Modal>
    );
};

export default TagModal;
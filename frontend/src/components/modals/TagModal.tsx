import { useState, useEffect } from 'react';
import Modal from '../Modal';
import { Info, Loader2 } from 'lucide-react'; // Added Loader

export interface TagFormData {
    name: string;
    description: string;
}

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
    const [formData, setFormData] = useState<TagFormData>({ name: '', description: '' });
    const [isSubmitting, setIsSubmitting] = useState(false);

    useEffect(() => {
        if (isOpen) {
            setFormData({
                name: initialData?.name || '',
                description: initialData?.description || ''
            });
            setIsSubmitting(false);
        }
    }, [isOpen, initialData]);

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
                                New tags must be approved by a moderator before appearing in public lists.
                            </p>
                        </div>
                    </div>
                )}

                <div>
                    <label className="block text-sm font-bold mb-1">Name</label>
                    <input
                        value={formData.name}
                        onChange={e => setFormData({...formData, name: e.target.value})}
                        className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground"
                        placeholder="Tag name"
                        disabled={isSubmitting}
                    />
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
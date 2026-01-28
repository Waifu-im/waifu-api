import { useState, useEffect } from 'react';
import Modal from '../Modal';
import { Loader2 } from 'lucide-react';

export interface AlbumFormData {
    name: string;
    description: string;
}

interface AlbumModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (data: AlbumFormData) => Promise<void> | void;
    initialData?: Partial<AlbumFormData>;
    title: string;
    submitLabel?: string;
}

const AlbumModal = ({ isOpen, onClose, onSubmit, initialData, title, submitLabel = "Save" }: AlbumModalProps) => {
    const [formData, setFormData] = useState<AlbumFormData>({ name: '', description: '' });
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
                <div>
                    <label className="block text-sm font-bold mb-1">Name</label>
                    <input
                        value={formData.name}
                        onChange={e => setFormData({...formData, name: e.target.value})}
                        className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground"
                        placeholder="Album Name"
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

export default AlbumModal;
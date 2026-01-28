import { useState, useEffect } from 'react';
import Modal from '../Modal';
import { Info, Loader2 } from 'lucide-react';

export interface ArtistFormData {
    name: string;
    twitter?: string;
    pixiv?: string;
    patreon?: string;
    deviantArt?: string;
}

interface ArtistModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (data: ArtistFormData) => Promise<void> | void;
    initialData?: Partial<ArtistFormData>;
    title: string;
    isReviewMode?: boolean;
    submitLabel?: string;
}

const ArtistModal = ({ isOpen, onClose, onSubmit, initialData, title, isReviewMode, submitLabel = "Save" }: ArtistModalProps) => {
    const [formData, setFormData] = useState<ArtistFormData>({ name: '', twitter: '', pixiv: '', patreon: '', deviantArt: '' });
    const [isSubmitting, setIsSubmitting] = useState(false);

    useEffect(() => {
        if (isOpen) {
            setFormData({
                name: initialData?.name || '',
                twitter: initialData?.twitter || '',
                pixiv: initialData?.pixiv || '',
                patreon: initialData?.patreon || '',
                deviantArt: initialData?.deviantArt || ''
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
                                New artists must be approved by a moderator before appearing in public lists.
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

export default ArtistModal;
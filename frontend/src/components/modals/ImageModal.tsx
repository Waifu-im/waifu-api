import { useState, useEffect } from 'react';
import Modal from '../Modal';
import { ImageDto, ImageFormData } from '../../types';

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
    const [tagIds, setTagIds] = useState<number[]>([]);
    const [artistId, setArtistId] = useState<number | null>(null);

    useEffect(() => {
        if (initialData) {
            setSource(initialData.source || '');
            setIsNsfw(initialData.isNsfw);
            setUserId(initialData.uploaderId?.toString() || '');
            setTagIds(initialData.tags ? initialData.tags.map(t => t.id) : []);
            setArtistId(initialData.artist?.id || null);
        } else {
            setSource('');
            setIsNsfw(false);
            setUserId('');
            setTagIds([]);
            setArtistId(null);
        }
    }, [initialData, isOpen]);

    const handleSubmit = async (e?: React.FormEvent) => {
        if (e) e.preventDefault();
        await onSubmit({
            source: source || undefined,
            isNsfw,
            tagIds,
            artistId,
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
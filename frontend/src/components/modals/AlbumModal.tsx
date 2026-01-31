import { useForm } from 'react-hook-form';
import Modal from '../Modal';
import { useEffect } from 'react';

export interface AlbumFormData {
    name: string;
    description?: string;
}

interface AlbumModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (data: AlbumFormData) => void;
    initialData?: AlbumFormData;
    title?: string;
    submitLabel?: string;
}

const AlbumModal = ({ isOpen, onClose, onSubmit, initialData, title = "Create Album", submitLabel = "Create" }: AlbumModalProps) => {
    const { register, handleSubmit, reset, formState: { errors } } = useForm<AlbumFormData>();

    useEffect(() => {
        if (isOpen) {
            reset(initialData || { name: '', description: '' });
        }
    }, [isOpen, initialData, reset]);

    const handleFormSubmit = (data: AlbumFormData) => {
        onSubmit(data);
        // Don't reset here immediately if we want to keep values on error, 
        // but usually parent handles close/reset. 
        // For now, let's rely on parent closing the modal to trigger the useEffect reset next time.
    };

    return (
        <Modal isOpen={isOpen} onClose={onClose} title={title} maxWidth="max-w-md">
            <form onSubmit={handleSubmit(handleFormSubmit)} className="space-y-4">
                <div>
                    <label className="block text-sm font-bold mb-1">Name</label>
                    <input 
                        {...register('name', { required: 'Name is required' })}
                        className="w-full p-2 rounded-lg bg-secondary border-transparent focus:ring-2 focus:ring-primary outline-none"
                        placeholder="My Awesome Album"
                        autoFocus
                    />
                    {errors.name && <p className="text-destructive text-xs mt-1">{errors.name.message}</p>}
                </div>
                
                <div>
                    <label className="block text-sm font-bold mb-1">Description</label>
                    <textarea 
                        {...register('description')}
                        className="w-full p-2 rounded-lg bg-secondary border-transparent focus:ring-2 focus:ring-primary outline-none min-h-[100px]"
                        placeholder="Optional description..."
                    />
                </div>

                <div className="flex justify-end gap-2 pt-2">
                    <button type="button" onClick={onClose} className="px-4 py-2 rounded-lg hover:bg-secondary transition-colors font-medium">Cancel</button>
                    <button type="submit" className="px-4 py-2 rounded-lg bg-primary text-primary-foreground font-bold hover:opacity-90 transition-opacity">{submitLabel}</button>
                </div>
            </form>
        </Modal>
    );
};

export default AlbumModal;
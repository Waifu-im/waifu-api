import { useState } from 'react';
import Modal from '../Modal';

interface ReportModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (description: string) => void;
}

const ReportModal = ({ isOpen, onClose, onSubmit }: ReportModalProps) => {
    const [description, setDescription] = useState('');

    const handleSubmit = () => {
        if (!description.trim()) return;
        onSubmit(description);
        setDescription('');
    };

    return (
        <Modal isOpen={isOpen} onClose={onClose} title="Report Image">
            <div className="space-y-4">
                <p className="text-sm text-muted-foreground">Please describe why you are reporting this image.</p>
                <textarea
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    placeholder="Reason for reporting..."
                    className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground min-h-[100px] resize-none"
                />
                <div className="flex gap-3 justify-end">
                    <button onClick={onClose} className="px-4 py-2 bg-secondary rounded-lg font-bold">Cancel</button>
                    <button 
                        onClick={handleSubmit} 
                        disabled={!description.trim()}
                        className="px-4 py-2 bg-destructive text-destructive-foreground rounded-lg font-bold disabled:opacity-50"
                    >
                        Submit Report
                    </button>
                </div>
            </div>
        </Modal>
    );
};

export default ReportModal;
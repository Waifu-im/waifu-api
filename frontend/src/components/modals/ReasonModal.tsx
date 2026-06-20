import { useState, useEffect } from 'react';
import Modal from '../Modal';

interface ReasonModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (reason: string) => void;
    title: string;
    description: string;
    placeholder?: string;
    submitText?: string;
    required?: boolean;
    maxLength?: number;
    /** Pre-fill the textarea (e.g. when editing an existing reason/report). */
    initialValue?: string;
    /** Confirm button colour. Defaults to destructive (reporting/rejecting); use 'primary' for plain edits. */
    confirmVariant?: 'primary' | 'destructive';
}

const ReasonModal = ({ isOpen, onClose, onSubmit, title, description, placeholder = "Reason...", submitText = "Submit", required = true, maxLength, initialValue = '', confirmVariant = 'destructive' }: ReasonModalProps) => {
    const [reason, setReason] = useState(initialValue);

    useEffect(() => {
        if (isOpen) setReason(initialValue);
    }, [isOpen, initialValue]);

    const handleSubmit = () => {
        if (required && !reason.trim()) return;
        onSubmit(reason.trim());
    };

    const isDisabled = required && !reason.trim();

    return (
        <Modal isOpen={isOpen} onClose={onClose} title={title}>
            <div className="space-y-4">
                <p className="text-sm text-muted-foreground">{description}</p>
                <div>
                    <textarea
                        value={reason}
                        onChange={(e) => setReason(maxLength ? e.target.value.slice(0, maxLength) : e.target.value)}
                        placeholder={placeholder}
                        className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground min-h-[100px] resize-none"
                    />
                    {maxLength && (
                        <p className="text-xs text-muted-foreground text-right mt-1">{reason.length}/{maxLength}</p>
                    )}
                </div>
                <div className="flex gap-3 justify-end">
                    <button onClick={onClose} className="px-4 py-2 bg-secondary rounded-lg font-bold">Cancel</button>
                    <button
                        onClick={handleSubmit}
                        disabled={isDisabled}
                        className={`px-4 py-2 rounded-lg font-bold disabled:opacity-50 ${confirmVariant === 'primary' ? 'bg-primary text-primary-foreground' : 'bg-destructive text-destructive-foreground'}`}
                    >
                        {submitText}
                    </button>
                </div>
            </div>
        </Modal>
    );
};

export default ReasonModal;

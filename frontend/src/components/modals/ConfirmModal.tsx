import Modal from '../Modal';
import { Trash2, AlertTriangle } from 'lucide-react';

interface ConfirmModalProps {
    isOpen: boolean;
    onClose: () => void;
    onConfirm: () => void;
    title: string;
    message: React.ReactNode;
    confirmText?: string;
    cancelText?: string;
    variant?: 'destructive' | 'warning';
}

const ConfirmModal = ({
                          isOpen,
                          onClose,
                          onConfirm,
                          title,
                          message,
                          confirmText = "Confirm",
                          cancelText = "Cancel",
                          variant = 'destructive'
                      }: ConfirmModalProps) => {
    return (
        <Modal isOpen={isOpen} onClose={onClose} title={title}>
            <div className="text-center space-y-4">
                <div className={`w-16 h-16 rounded-full flex items-center justify-center mx-auto ${variant === 'destructive' ? 'bg-destructive/10 text-destructive' : 'bg-orange-500/10 text-orange-500'}`}>
                    {variant === 'destructive' ? <Trash2 size={32} /> : <AlertTriangle size={32} />}
                </div>
                <div className="text-muted-foreground">
                    {message}
                </div>
                <div className="flex gap-3 mt-6">
                    <button onClick={onClose} className="flex-1 py-3 bg-secondary rounded-lg font-bold hover:bg-secondary/80 transition-colors">
                        {cancelText}
                    </button>
                    <button
                        onClick={onConfirm}
                        className={`flex-1 py-3 text-white rounded-lg font-bold transition-colors ${variant === 'destructive' ? 'bg-destructive hover:bg-destructive/90' : 'bg-orange-500 hover:bg-orange-600'}`}
                    >
                        {confirmText}
                    </button>
                </div>
            </div>
        </Modal>
    );
};

export default ConfirmModal;
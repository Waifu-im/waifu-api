import Modal from '../Modal';
import { Trash2, AlertTriangle, LucideIcon } from 'lucide-react';

interface ConfirmModalProps {
    isOpen: boolean;
    onClose: () => void;
    onConfirm: () => void;
    title: string;
    message: React.ReactNode;
    confirmText?: string;
    cancelText?: string;
    variant?: 'destructive' | 'warning' | 'info';
    icon?: LucideIcon; // Permet de passer une icône personnalisée (ex: FolderMinus)
}

const ConfirmModal = ({
                          isOpen,
                          onClose,
                          onConfirm,
                          title,
                          message,
                          confirmText = "Confirm",
                          cancelText = "Cancel",
                          variant = 'destructive',
                          icon: Icon
                      }: ConfirmModalProps) => {

    // Choix de l'icône : Icon perso > Variant
    const DisplayIcon = Icon || (variant === 'destructive' ? Trash2 : AlertTriangle);

    // Choix des couleurs
    const colorClass = variant === 'destructive'
        ? 'bg-destructive/10 text-destructive'
        : variant === 'warning'
            ? 'bg-orange-500/10 text-orange-500'
            : 'bg-blue-500/10 text-blue-500';

    const buttonClass = variant === 'destructive'
        ? 'bg-destructive hover:bg-destructive/90 text-destructive-foreground'
        : variant === 'warning'
            ? 'bg-orange-500 hover:bg-orange-600 text-white'
            : 'bg-primary hover:bg-primary/90 text-primary-foreground';

    return (
        <Modal isOpen={isOpen} onClose={onClose} title={title}>
            <div className="text-center space-y-4">
                <div className={`w-16 h-16 rounded-full flex items-center justify-center mx-auto ${colorClass}`}>
                    <DisplayIcon size={32} />
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
                        className={`flex-1 py-3 rounded-lg font-bold transition-colors ${buttonClass}`}
                    >
                        {confirmText}
                    </button>
                </div>
            </div>
        </Modal>
    );
};

export default ConfirmModal;
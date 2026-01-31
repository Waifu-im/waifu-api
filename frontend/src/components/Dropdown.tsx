import { useState, useRef, useEffect, ReactNode } from 'react';

interface DropdownProps {
    trigger: ReactNode;
    children: ReactNode;
    align?: 'left' | 'right';
    width?: string;
    className?: string;
    isOpen?: boolean;
    onClose?: () => void;
    onOpen?: () => void;
    padding?: string;
}

export const Dropdown = ({ trigger, children, align = 'right', width = 'w-48', className = '', isOpen: controlledIsOpen, onClose, onOpen, padding = 'p-1' }: DropdownProps) => {
    const [internalIsOpen, setInternalIsOpen] = useState(false);
    const ref = useRef<HTMLDivElement>(null);

    const isControlled = controlledIsOpen !== undefined;
    const isOpen = isControlled ? controlledIsOpen : internalIsOpen;

    const handleToggle = () => {
        if (isControlled) {
            if (isOpen) onClose?.();
            else onOpen?.();
        } else {
            setInternalIsOpen(!internalIsOpen);
        }
    };

    const handleClose = () => {
        if (isControlled) onClose?.();
        else setInternalIsOpen(false);
    };

    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (ref.current && !ref.current.contains(event.target as Node)) {
                handleClose();
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, [isControlled, onClose]);

    return (
        <div className="relative" ref={ref}>
            <div onClick={handleToggle} className="cursor-pointer">
                {trigger}
            </div>
            {isOpen && (
                <div
                    className={`absolute mt-2 bg-card border border-border rounded-xl shadow-xl animate-in fade-in zoom-in-95 duration-200 z-50 ${padding} overflow-hidden ${align === 'right' ? 'right-0' : 'left-0'} ${width} ${className}`}
                    onClick={(e) => e.stopPropagation()} // Prevent closing when clicking inside dropdown content
                >
                    {children}
                </div>
            )}
        </div>
    );
};

interface DropdownItemProps {
    children: ReactNode;
    onClick?: (e: React.MouseEvent) => void;
    className?: string;
    active?: boolean;
    icon?: ReactNode;
    danger?: boolean;
}

export const DropdownItem = ({ children, onClick, className = '', active = false, icon, danger = false }: DropdownItemProps) => (
    <div
        onClick={onClick}
        className={`px-3 py-2 rounded-lg text-sm font-medium cursor-pointer flex items-center gap-3 transition-colors mb-0.5 last:mb-0
            ${active ? 'bg-primary/10 text-primary' : ''} 
            ${danger ? 'text-red-600 hover:bg-red-500/10' : !active ? 'hover:bg-secondary text-muted-foreground hover:text-foreground' : ''}
            ${className}`}
    >
        {icon && <span className={danger ? "text-red-500" : (active ? "text-primary" : "text-muted-foreground")}>{icon}</span>}
        {children}
    </div>
);

export const DropdownSeparator = () => <div className="h-px bg-border my-1 mx-2" />;

export const DropdownLabel = ({ children }: { children: ReactNode }) => (
    <div className="px-3 py-2 text-[10px] font-bold uppercase text-muted-foreground tracking-wider">
        {children}
    </div>
);

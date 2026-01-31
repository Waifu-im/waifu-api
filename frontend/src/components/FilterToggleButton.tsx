import { SlidersHorizontal } from 'lucide-react';

interface FilterToggleButtonProps {
    isOpen: boolean;
    onToggle: () => void;
    className?: string;
}

const FilterToggleButton = ({ isOpen, onToggle, className = "" }: FilterToggleButtonProps) => {
    return (
        <button
            onClick={onToggle}
            className={`flex items-center gap-2 px-4 py-2 bg-card border border-border rounded-lg text-sm font-medium shadow-sm hover:bg-secondary transition-colors ${className}`}
        >
            <SlidersHorizontal size={16} />
            <span className="hidden sm:inline">{isOpen ? 'Hide' : 'Filters'}</span>
        </button>
    );
};

export default FilterToggleButton;
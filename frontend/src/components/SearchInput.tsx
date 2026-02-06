import React from 'react';
import { Search, Hash, ArrowRightLeft } from 'lucide-react';

interface SearchInputProps {
    value: string;
    onChange: (value: string) => void;
    searchType: 'name' | 'id';
    onSearchTypeChange: (type: 'name' | 'id') => void;
    className?: string;
}

const SearchInput: React.FC<SearchInputProps> = ({
                                                     value,
                                                     onChange,
                                                     searchType,
                                                     onSearchTypeChange,
                                                     className = "w-full md:w-64" // Default width
                                                 }) => {

    const handleToggle = () => {
        const nextType = searchType === 'name' ? 'id' : 'name';

        // Centralized logic: Clear input if switching to ID with invalid text
        if (nextType === 'id' && value && !/^\d+$/.test(value)) {
            onChange('');
        }

        onSearchTypeChange(nextType);
    };

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const val = e.target.value;
        // Centralized logic: Prevent typing non-numbers in ID mode
        if (searchType === 'id' && val && !/^\d*$/.test(val)) {
            return;
        }
        onChange(val);
    };

    return (
        <div className={`relative flex items-center bg-card border border-border rounded-xl focus-within:ring-2 focus-within:ring-primary overflow-hidden shadow-sm transition-all ${className}`}>
            <div className="absolute left-3 text-muted-foreground pointer-events-none">
                {searchType === 'id' ? <Hash size={18} /> : <Search size={18} />}
            </div>

            <input
                type="text"
                placeholder={searchType === 'id' ? "Search ID..." : "Search name..."}
                value={value}
                onChange={handleChange}
                className="w-full pl-10 p-3 bg-transparent outline-none border-none placeholder:text-muted-foreground/50"
            />

            <button
                onClick={handleToggle}
                className="flex items-center gap-2 px-3 self-stretch bg-secondary/50 border-l border-border hover:bg-secondary hover:text-foreground text-muted-foreground transition-colors text-xs font-bold uppercase tracking-wider"
                title={`Switch to ${searchType === 'name' ? 'ID' : 'Name'} search`}
            >
                <span className="hidden sm:inline">{searchType}</span>
                <ArrowRightLeft size={14} />
            </button>
        </div>
    );
};

export default SearchInput;
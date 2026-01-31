import { useState, useEffect, useRef, useCallback } from 'react';
import { X, Plus, ChevronDown, Check, Loader2 } from 'lucide-react';
import { useDebounce } from '../hooks/useDebounce';

export interface Option {
    id: number | string;
    name: string;
    description?: string;
    slug?: string;
    [key: string]: any;
}

interface SearchableSelectProps {
    options?: Option[];
    selectedOptions: Option[];
    onSelect: (option: Option) => void;
    onRemove: (option: Option) => void;
    onCreate?: (name: string) => void;
    loadOptions?: (query: string, page?: number) => Promise<Option[]>;
    placeholder?: string;
    label?: string;
    isMulti?: boolean;
    clearable?: boolean;
}

const SearchableSelect = ({
                              options = [],
                              selectedOptions,
                              onSelect,
                              onRemove,
                              onCreate,
                              loadOptions,
                              placeholder = "Select...",
                              label,
                              isMulti = true,
                              clearable = true
                          }: SearchableSelectProps) => {
    const [isOpen, setIsOpen] = useState(false);
    const [searchTerm, setSearchTerm] = useState('');
    const [asyncOptions, setAsyncOptions] = useState<Option[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [page, setPage] = useState(1);
    const [hasMore, setHasMore] = useState(true);
    const wrapperRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLInputElement>(null);
    const listRef = useRef<HTMLDivElement>(null);

    const debouncedSearchTerm = useDebounce(searchTerm, 300);

    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (wrapperRef.current && !wrapperRef.current.contains(event.target as Node)) {
                setIsOpen(false);
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, []);

    // Reset pagination when search term changes or dropdown opens
    useEffect(() => {
        if (isOpen) {
            setPage(1);
            setHasMore(true);
            setAsyncOptions([]);
        }
    }, [debouncedSearchTerm, isOpen]);

    useEffect(() => {
        if (!loadOptions || !isOpen) return;

        let isMounted = true;
        const fetchOptions = async () => {
            setIsLoading(true);
            try {
                const results = await loadOptions(debouncedSearchTerm, page);
                if (isMounted) {
                    if (page === 1) {
                        setAsyncOptions(results);
                    } else {
                        setAsyncOptions(prev => {
                            // Filter out duplicates just in case
                            const existingIds = new Set(prev.map(o => o.id));
                            const newOptions = results.filter(o => !existingIds.has(o.id));
                            return [...prev, ...newOptions];
                        });
                    }
                    setHasMore(results.length > 0);
                }
            } catch (error) {
                console.error("Failed to load options", error);
            } finally {
                if (isMounted) setIsLoading(false);
            }
        };

        fetchOptions();

        return () => { isMounted = false; };
    }, [debouncedSearchTerm, loadOptions, isOpen, page]);

    const handleScroll = useCallback((e: React.UIEvent<HTMLDivElement>) => {
        const { scrollTop, clientHeight, scrollHeight } = e.currentTarget;
        if (scrollHeight - scrollTop <= clientHeight + 50 && !isLoading && hasMore && loadOptions) {
            setPage(prev => prev + 1);
        }
    }, [isLoading, hasMore, loadOptions]);

    const displayedOptions = loadOptions
        ? asyncOptions
        : options.filter(option => option.name.toLowerCase().includes(searchTerm.toLowerCase()));

    const handleOptionClick = (option: Option) => {
        // Use a more robust comparison that handles both ID and Name/Slug matching
        // This is crucial because sometimes we might have objects with just ID or just Name depending on source
        const isSelected = selectedOptions.some(selected => {
            // Strict ID match if both have IDs
            if (selected.id !== undefined && option.id !== undefined) {
                return String(selected.id) === String(option.id);
            }
            // Fallback to name/slug match
            const selectedKey = selected.slug || selected.name;
            const optionKey = option.slug || option.name;
            return selectedKey?.toLowerCase() === optionKey?.toLowerCase();
        });

        if (isSelected) {
            // Find the exact object reference or construct a valid one to remove
            const itemToRemove = selectedOptions.find(selected => {
                 if (selected.id !== undefined && option.id !== undefined) {
                    return String(selected.id) === String(option.id);
                }
                const selectedKey = selected.slug || selected.name;
                const optionKey = option.slug || option.name;
                return selectedKey?.toLowerCase() === optionKey?.toLowerCase();
            }) || option;
            
            onRemove(itemToRemove);
        } else {
            onSelect(option);
        }

        if (!isMulti) setIsOpen(false);
        setSearchTerm('');
    };

    return (
        <div className="relative group" ref={wrapperRef}>
            {label && <label className="block text-xs font-bold uppercase text-muted-foreground tracking-wider mb-1.5">{label}</label>}

            <div
                className="min-h-[42px] p-1.5 border border-input rounded-xl bg-card focus-within:ring-2 focus-within:ring-primary focus-within:border-transparent flex flex-wrap items-center gap-1.5 cursor-pointer shadow-sm hover:border-primary/50 transition-colors"
                onClick={() => { setIsOpen(true); inputRef.current?.focus(); }}
            >
                {selectedOptions.length === 0 && !searchTerm && (
                    <span className="text-muted-foreground text-sm px-2 select-none pointer-events-none absolute">{placeholder}</span>
                )}

                {selectedOptions.map(option => (
                    <span key={option.id} className="bg-secondary text-secondary-foreground px-2.5 py-1 rounded-lg text-sm font-medium flex items-center gap-1.5 animate-in fade-in zoom-in duration-200">
                        {option.name}
                        {clearable && (
                            <button
                                onClick={(e) => { e.stopPropagation(); onRemove(option); }}
                                className="hover:text-destructive transition-colors p-0.5 rounded-full hover:bg-black/5 dark:hover:bg-white/10"
                            >
                                <X size={12} />
                            </button>
                        )}
                    </span>
                ))}

                <input
                    ref={inputRef}
                    type="text"
                    className="flex-1 bg-transparent outline-none text-sm px-1 min-w-[2rem] h-6"
                    value={searchTerm}
                    onChange={(e) => { setSearchTerm(e.target.value); setIsOpen(true); }}
                    onFocus={() => setIsOpen(true)}
                />

                <div className="mr-1 text-muted-foreground">
                    {isLoading ? <Loader2 size={16} className="animate-spin" /> : <ChevronDown size={16} />}
                </div>
            </div>

            {isOpen && (
                <div 
                    className="absolute z-50 w-full mt-2 bg-popover border border-border rounded-xl shadow-xl max-h-60 overflow-auto animate-in fade-in slide-in-from-top-2 flex flex-col"
                    onScroll={handleScroll}
                    ref={listRef}
                >
                    {/* Create Option - Always First */}
                    {onCreate && !displayedOptions.some(o => o.name.toLowerCase() === searchTerm.toLowerCase()) && (
                        <div
                            className="px-4 py-3 hover:bg-accent hover:text-accent-foreground cursor-pointer flex items-center gap-2 text-primary font-medium text-sm border-b border-border sticky top-0 bg-popover z-10"
                            onClick={() => { onCreate(searchTerm); setIsOpen(false); setSearchTerm(''); }}
                        >
                            <Plus size={16} /> {searchTerm ? `Create "${searchTerm}"` : "Create New"}
                        </div>
                    )}

                    {/* Options List */}
                    {displayedOptions.map(option => {
                        // Improved selection check for rendering
                        const isSelected = selectedOptions.some(s => {
                            if (s.id !== undefined && option.id !== undefined) {
                                return String(s.id) === String(option.id);
                            }
                            const sKey = s.slug || s.name;
                            const oKey = option.slug || option.name;
                            return sKey?.toLowerCase() === oKey?.toLowerCase();
                        });

                        return (
                            <div
                                key={option.id}
                                className={`px-4 py-2.5 cursor-pointer flex justify-between items-center text-sm transition-colors ${
                                    isSelected
                                        ? 'bg-primary/10 text-primary font-medium'
                                        : 'hover:bg-accent hover:text-accent-foreground'
                                }`}
                                onClick={() => handleOptionClick(option)}
                            >
                                <div className="flex items-center gap-2 overflow-hidden">
                                    {isSelected && <Check size={14} />}
                                    <span className="truncate">{option.name}</span>
                                </div>
                                {option.description && <span className="text-xs text-muted-foreground truncate ml-2 max-w-[40%] opacity-70">{option.description}</span>}
                            </div>
                        );
                    })}

                    {displayedOptions.length === 0 && !searchTerm && !isLoading && !onCreate && (
                        <div className="px-4 py-8 text-center text-muted-foreground text-sm">No options found.</div>
                    )}
                    
                    {isLoading && (
                         <div className="px-4 py-3 text-center text-muted-foreground text-sm flex items-center justify-center gap-2">
                             <Loader2 size={14} className="animate-spin" /> Loading...
                         </div>
                    )}
                </div>
            )}
        </div>
    );
};

export default SearchableSelect;
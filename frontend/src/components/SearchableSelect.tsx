import { useState, useEffect, useRef } from 'react';
import { X, Plus, Check } from 'lucide-react';

interface Option {
  id: number | string;
  name: string;
  description?: string;
}

interface SearchableSelectProps {
  options: Option[];
  selectedOptions: Option[];
  onSelect: (option: Option) => void;
  onRemove: (option: Option) => void;
  onCreate?: (name: string) => void;
  placeholder?: string;
  label?: string;
  isMulti?: boolean;
}

const SearchableSelect = ({
  options,
  selectedOptions,
  onSelect,
  onRemove,
  onCreate,
  placeholder = "Select...",
  label,
  isMulti = true
}: SearchableSelectProps) => {
  const [isOpen, setIsOpen] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const wrapperRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (wrapperRef.current && !wrapperRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const filteredOptions = options.filter(option => 
    option.name.toLowerCase().includes(searchTerm.toLowerCase()) &&
    !selectedOptions.some(selected => selected.id === option.id)
  );

  const handleSelect = (option: Option) => {
    onSelect(option);
    if (!isMulti) setIsOpen(false);
    setSearchTerm('');
  };

  const handleCreate = () => {
    if (onCreate && searchTerm) {
      onCreate(searchTerm);
      setSearchTerm('');
      setIsOpen(false);
    }
  };

  return (
    <div className="relative" ref={wrapperRef}>
      {label && <label className="block text-sm font-medium mb-1">{label}</label>}
      
      <div 
        className="min-h-[42px] p-1 border border-input rounded-md bg-background focus-within:ring-2 focus-within:ring-primary focus-within:border-transparent flex flex-wrap gap-1 cursor-text"
        onClick={() => setIsOpen(true)}
      >
        {selectedOptions.map(option => (
          <span key={option.id} className="bg-secondary text-secondary-foreground px-2 py-1 rounded-md text-sm flex items-center gap-1 group relative">
            {option.name}
            <button 
              onClick={(e) => { e.stopPropagation(); onRemove(option); }}
              className="hover:text-destructive"
            >
              <X size={14} />
            </button>
            {option.description && (
              <div className="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 w-48 p-2 bg-popover text-popover-foreground text-xs rounded shadow-lg opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none z-50">
                {option.description}
              </div>
            )}
          </span>
        ))}
        <input
          type="text"
          className="flex-1 min-w-[100px] bg-transparent outline-none p-1"
          placeholder={selectedOptions.length === 0 ? placeholder : ""}
          value={searchTerm}
          onChange={(e) => {
            setSearchTerm(e.target.value);
            setIsOpen(true);
          }}
          onFocus={() => setIsOpen(true)}
        />
      </div>

      {isOpen && (searchTerm || filteredOptions.length > 0) && (
        <div className="absolute z-50 w-full mt-1 bg-popover border border-border rounded-md shadow-lg max-h-60 overflow-auto">
          {filteredOptions.map(option => (
            <div
              key={option.id}
              className="px-3 py-2 hover:bg-accent hover:text-accent-foreground cursor-pointer flex justify-between items-center group relative"
              onClick={() => handleSelect(option)}
            >
              <span>{option.name}</span>
              {option.description && (
                 <div className="absolute left-full top-0 ml-2 w-48 p-2 bg-popover text-popover-foreground text-xs rounded shadow-lg opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none z-50 border border-border">
                  {option.description}
                </div>
              )}
            </div>
          ))}
          
          {searchTerm && onCreate && !options.some(o => o.name.toLowerCase() === searchTerm.toLowerCase()) && (
            <div
              className="px-3 py-2 hover:bg-accent hover:text-accent-foreground cursor-pointer flex items-center gap-2 text-primary"
              onClick={handleCreate}
            >
              <Plus size={16} /> Create "{searchTerm}"
            </div>
          )}
          
          {filteredOptions.length === 0 && !searchTerm && (
            <div className="px-3 py-2 text-muted-foreground text-sm">No options found.</div>
          )}
        </div>
      )}
    </div>
  );
};

export default SearchableSelect;

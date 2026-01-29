import { useEffect, useState } from 'react';
import SearchableSelect from './SearchableSelect';
import api from '../services/api';
import { PaginatedList, Tag, Artist } from '../types';
import { X } from 'lucide-react';

interface Option {
    id: string;
    name: string;
}

interface FilterSidebarProps {
    searchParams: URLSearchParams;
    setSearchParams: (params: URLSearchParams | ((prev: URLSearchParams) => URLSearchParams)) => void;
    showFilters: boolean;
    setShowFilters: (show: boolean) => void;
    sortOptions: Option[];
}

const FilterSidebar = ({ searchParams, setSearchParams, showFilters, setShowFilters, sortOptions }: FilterSidebarProps) => {
    const [allTags, setAllTags] = useState<{ id: number, name: string }[]>([]);
    const [allArtists, setAllArtists] = useState<{ id: number, name: string }[]>([]);
    const [includedIdInput, setIncludedIdInput] = useState('');
    const [excludedIdInput, setExcludedIdInput] = useState('');

    useEffect(() => {
        api.get<PaginatedList<Tag>>('/tags', { params: { pageSize: 1000 } })
            .then(res => setAllTags(res.data.items))
            .catch(console.error);
            
        api.get<PaginatedList<Artist>>('/artists', { params: { pageSize: 1000 } })
            .then(res => setAllArtists(res.data.items))
            .catch(console.error);
    }, []);

    const updateFilter = (key: string, value: string | null) => {
        setSearchParams(prev => {
            if (value) prev.set(key, value);
            else prev.delete(key);
            return prev;
        });
    };

    const handleTagChange = (key: 'includedTags' | 'excludedTags', name: string, action: 'add' | 'remove') => {
        setSearchParams(prev => {
            const current = prev.getAll(key);
            prev.delete(key);
            if (action === 'add' && !current.includes(name)) {
                [...current, name].forEach(t => prev.append(key, t));
            } else if (action === 'remove') {
                current.filter(t => t !== name).forEach(t => prev.append(key, t));
            } else {
                current.forEach(t => prev.append(key, t));
            }
            return prev;
        });
    };
    
    const handleArtistChange = (key: 'includedArtists' | 'excludedArtists', name: string, action: 'add' | 'remove') => {
        setSearchParams(prev => {
            const current = prev.getAll(key);
            prev.delete(key);
            if (action === 'add' && !current.includes(name)) {
                [...current, name].forEach(t => prev.append(key, t));
            } else if (action === 'remove') {
                current.filter(t => t !== name).forEach(t => prev.append(key, t));
            } else {
                current.forEach(t => prev.append(key, t));
            }
            return prev;
        });
    };

    const handleIdChange = (key: 'includedIds' | 'excludedIds', id: string, action: 'add' | 'remove') => {
        setSearchParams(prev => {
            const current = prev.getAll(key);
            prev.delete(key);
            if (action === 'add' && !current.includes(id)) {
                [...current, id].forEach(t => prev.append(key, t));
            } else if (action === 'remove') {
                current.filter(t => t !== id).forEach(t => prev.append(key, t));
            } else {
                current.forEach(t => prev.append(key, t));
            }
            return prev;
        });
    };

    const handleIdKeyDown = (e: React.KeyboardEvent<HTMLInputElement>, key: 'includedIds' | 'excludedIds', input: string, setInput: (val: string) => void) => {
        if (e.key === 'Enter' && input.trim()) {
            e.preventDefault();
            handleIdChange(key, input.trim(), 'add');
            setInput('');
        }
    };

    const orderBy = searchParams.get('orderBy') || 'UPLOADED_AT';
    const isNsfw = searchParams.get('isNsfw') || '0';
    const orientation = searchParams.get('orientation') || '';
    const isAnimatedStr = searchParams.get('isAnimated') || '';
    const includedTags = searchParams.getAll('includedTags');
    const excludedTags = searchParams.getAll('excludedTags');
    const includedArtists = searchParams.getAll('includedArtists');
    const excludedArtists = searchParams.getAll('excludedArtists');
    const includedIds = searchParams.getAll('includedIds');
    const excludedIds = searchParams.getAll('excludedIds');
    const width = searchParams.get('width') || '';
    const height = searchParams.get('height') || '';
    const byteSize = searchParams.get('byteSize') || '';

    const ratingOptions = [
        { id: '0', name: 'Safe' },
        { id: '1', name: 'NSFW (18+)' },
        { id: '2', name: 'All Content' }
    ];

    const orientationOptions = [
        { id: '', name: 'Any' },
        { id: 'LANDSCAPE', name: 'Landscape' },
        { id: 'PORTRAIT', name: 'Portrait' }
    ];

    const animationOptions = [
        { id: '', name: 'Any' },
        { id: 'false', name: 'Static Image' },
        { id: 'true', name: 'Animated (GIF)' }
    ];

    return (
        <aside
            className={`
                fixed inset-y-0 right-0 z-[60] bg-card border-l border-border shadow-2xl h-full flex flex-col
                transition-all duration-300 ease-in-out
                ${showFilters
                ? 'translate-x-0 w-full md:w-80 md:translate-x-0'
                : 'translate-x-full md:translate-x-0 md:w-0 md:border-l-0 md:overflow-hidden'
            }
                md:relative md:z-40 md:shadow-none
            `}
        >
            {/* Scrollable content area */}
            <div className="flex-1 p-5 overflow-y-auto space-y-6 w-full md:w-80 min-w-[20rem] scrollbar-thin pt-10 md:pt-6">
                <SearchableSelect
                    label="Sort By"
                    options={sortOptions}
                    selectedOptions={sortOptions.filter(o => o.id === orderBy)}
                    onSelect={(o) => updateFilter('orderBy', o.id as string)}
                    onRemove={() => {}}
                    isMulti={false}
                    clearable={false}
                />

                <SearchableSelect
                    label="Content Rating"
                    options={ratingOptions}
                    selectedOptions={ratingOptions.filter(o => o.id === isNsfw)}
                    onSelect={(o) => updateFilter('isNsfw', o.id as string)}
                    onRemove={() => { }}
                    isMulti={false}
                    clearable={false}
                />

                <div className="border-t border-border"></div>

                <SearchableSelect
                    label="Orientation"
                    options={orientationOptions}
                    selectedOptions={orientationOptions.filter(o => o.id === orientation)}
                    onSelect={(o) => updateFilter('orientation', o.id as string)}
                    onRemove={() => updateFilter('orientation', null)}
                    isMulti={false}
                />

                <SearchableSelect
                    label="Type"
                    options={animationOptions}
                    selectedOptions={animationOptions.filter(o => o.id === isAnimatedStr)}
                    onSelect={(o) => updateFilter('isAnimated', o.id as string)}
                    onRemove={() => updateFilter('isAnimated', null)}
                    isMulti={false}
                />

                <div className="grid grid-cols-2 gap-3">
                    <div className="col-span-2">
                        <label className="block text-xs font-bold uppercase text-muted-foreground mb-1">Dimensions (px)</label>
                        <div className="flex gap-2">
                            <input
                                type="text" placeholder="Width (e.g. 1000)"
                                value={width} onChange={e => updateFilter('width', e.target.value)}
                                className="w-1/2 p-2 rounded bg-secondary text-sm border-transparent focus:ring-1 focus:ring-primary outline-none"
                            />
                            <input
                                type="text" placeholder="Height (e.g. 1000)"
                                value={height} onChange={e => updateFilter('height', e.target.value)}
                                className="w-1/2 p-2 rounded bg-secondary text-sm border-transparent focus:ring-1 focus:ring-primary outline-none"
                            />
                        </div>
                    </div>

                    <div className="col-span-2">
                        <label className="block text-xs font-bold uppercase text-muted-foreground mb-1">Byte Size</label>
                        <input
                            type="text" placeholder="Size (e.g. < 1000000)"
                            value={byteSize} onChange={e => updateFilter('byteSize', e.target.value)}
                            className="w-full p-2 rounded bg-secondary text-sm border-transparent focus:ring-1 focus:ring-primary outline-none"
                        />
                    </div>
                </div>

                <div className="border-t border-border"></div>

                <SearchableSelect
                    label="Included Tags"
                    placeholder="Search tags..."
                    options={allTags}
                    selectedOptions={includedTags.map(t => ({ id: t, name: t }))}
                    onSelect={(o) => handleTagChange('includedTags', o.name, 'add')}
                    onRemove={(o) => handleTagChange('includedTags', o.name, 'remove')}
                    isMulti={true}
                />

                <SearchableSelect
                    label="Excluded Tags"
                    placeholder="Exclude tags..."
                    options={allTags}
                    selectedOptions={excludedTags.map(t => ({ id: t, name: t }))}
                    onSelect={(o) => handleTagChange('excludedTags', o.name, 'add')}
                    onRemove={(o) => handleTagChange('excludedTags', o.name, 'remove')}
                    isMulti={true}
                />
                
                <div className="border-t border-border"></div>

                <SearchableSelect
                    label="Included Artists"
                    placeholder="Search artists..."
                    options={allArtists}
                    selectedOptions={includedArtists.map(t => ({ id: t, name: t }))}
                    onSelect={(o) => handleArtistChange('includedArtists', o.name, 'add')}
                    onRemove={(o) => handleArtistChange('includedArtists', o.name, 'remove')}
                    isMulti={true}
                />

                <SearchableSelect
                    label="Excluded Artists"
                    placeholder="Exclude artists..."
                    options={allArtists}
                    selectedOptions={excludedArtists.map(t => ({ id: t, name: t }))}
                    onSelect={(o) => handleArtistChange('excludedArtists', o.name, 'add')}
                    onRemove={(o) => handleArtistChange('excludedArtists', o.name, 'remove')}
                    isMulti={true}
                />

                <div className="border-t border-border"></div>

                <div>
                    <label className="block text-xs font-bold uppercase text-muted-foreground mb-1">Included IDs</label>
                    <div className="flex flex-wrap gap-2 mb-2">
                        {includedIds.map(id => (
                            <span key={id} className="inline-flex items-center gap-1 px-2 py-1 bg-primary/10 text-primary rounded text-xs font-medium">
                                {id}
                                <button onClick={() => handleIdChange('includedIds', id, 'remove')} className="hover:text-destructive"><X size={12} /></button>
                            </span>
                        ))}
                    </div>
                    <input
                        type="text"
                        placeholder="Press Enter to add ID..."
                        value={includedIdInput}
                        onChange={e => setIncludedIdInput(e.target.value)}
                        onKeyDown={e => handleIdKeyDown(e, 'includedIds', includedIdInput, setIncludedIdInput)}
                        className="w-full p-2 rounded bg-secondary text-sm border-transparent focus:ring-1 focus:ring-primary outline-none"
                    />
                </div>

                <div>
                    <label className="block text-xs font-bold uppercase text-muted-foreground mb-1">Excluded IDs</label>
                    <div className="flex flex-wrap gap-2 mb-2">
                        {excludedIds.map(id => (
                            <span key={id} className="inline-flex items-center gap-1 px-2 py-1 bg-destructive/10 text-destructive rounded text-xs font-medium">
                                {id}
                                <button onClick={() => handleIdChange('excludedIds', id, 'remove')} className="hover:text-destructive"><X size={12} /></button>
                            </span>
                        ))}
                    </div>
                    <input
                        type="text"
                        placeholder="Press Enter to add ID..."
                        value={excludedIdInput}
                        onChange={e => setExcludedIdInput(e.target.value)}
                        onKeyDown={e => handleIdKeyDown(e, 'excludedIds', excludedIdInput, setExcludedIdInput)}
                        className="w-full p-2 rounded bg-secondary text-sm border-transparent focus:ring-1 focus:ring-primary outline-none"
                    />
                </div>
            </div>

            {/* Bottom button for mobile */}
            <div className="p-4 border-t border-border md:hidden bg-card">
                <button onClick={() => setShowFilters(false)} className="w-full py-3 bg-primary text-primary-foreground rounded-xl font-bold">Done</button>
            </div>
        </aside>
    );
};

export default FilterSidebar;
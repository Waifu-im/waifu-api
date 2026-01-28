import { useEffect, useState } from 'react';
import { X } from 'lucide-react';
import SearchableSelect from './SearchableSelect';
import api from '../services/api';
import { PaginatedList, Tag } from '../types';

interface Option {
    id: string;
    name: string;
}

interface FilterSidebarProps {
    searchParams: URLSearchParams;
    setSearchParams: (params: URLSearchParams | ((prev: URLSearchParams) => URLSearchParams)) => void;
    showFilters: boolean;
    setShowFilters: (show: boolean) => void;
    sortOptions: Option[]; // Added prop to support custom sort options
}

const FilterSidebar = ({ searchParams, setSearchParams, showFilters, setShowFilters, sortOptions }: FilterSidebarProps) => {
    const [allTags, setAllTags] = useState<{ id: number, name: string }[]>([]);

    useEffect(() => {
        api.get<PaginatedList<Tag>>('/tags', { params: { pageSize: 1000 } })
            .then(res => setAllTags(res.data.items))
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

    const orderBy = searchParams.get('orderBy') || 'UPLOADED_AT';
    const isNsfw = searchParams.get('isNsfw') || '0';
    const orientation = searchParams.get('orientation') || '';
    const isAnimatedStr = searchParams.get('isAnimated') || '';
    const includedTags = searchParams.getAll('includedTags');
    const excludedTags = searchParams.getAll('excludedTags');
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
        <aside className={`
            fixed inset-y-0 right-0 z-40 w-80 max-w-[85vw] bg-card border-l border-border shadow-2xl 
            transform transition-transform duration-300 ease-in-out h-full flex flex-col
            ${showFilters ? 'translate-x-0' : 'translate-x-full'}
            md:relative md:shadow-none md:translate-x-0
            ${!showFilters && 'hidden md:block md:w-0 md:border-none md:overflow-hidden'} 
        `}>
            <div className="p-5 border-b border-border flex justify-between items-center bg-card/95 backdrop-blur">
                <h3 className="font-bold">Filters</h3>
                <button onClick={() => setShowFilters(false)} className="md:hidden"><X size={20} /></button>
            </div>

            <div className="flex-1 p-5 overflow-y-auto space-y-6 w-80 scrollbar-thin">
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
            </div>

            <div className="p-4 border-t border-border md:hidden">
                <button onClick={() => setShowFilters(false)} className="w-full py-3 bg-primary text-primary-foreground rounded-xl font-bold">Done</button>
            </div>
        </aside>
    );
};

export default FilterSidebar;
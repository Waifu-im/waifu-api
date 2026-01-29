import { useEffect, useState } from 'react';
import SearchableSelect, { Option } from './SearchableSelect';
import api from '../services/api';
import { PaginatedList, Tag, Artist } from '../types';
import { X } from 'lucide-react';

interface FilterSidebarProps {
    searchParams: URLSearchParams;
    setSearchParams: (params: URLSearchParams | ((prev: URLSearchParams) => URLSearchParams)) => void;
    showFilters: boolean;
    setShowFilters: (show: boolean) => void;
    sortOptions: Option[];
}

const FilterSidebar = ({ searchParams, setSearchParams, showFilters, setShowFilters, sortOptions }: FilterSidebarProps) => {
    const [initialTags, setInitialTags] = useState<Option[]>([]);
    const [initialArtists, setInitialArtists] = useState<Option[]>([]);

    const [includedIdInput, setIncludedIdInput] = useState('');
    const [excludedIdInput, setExcludedIdInput] = useState('');

    // Load initial data for selected items if needed, or just some defaults
    useEffect(() => {
        // We can load a small batch initially or just rely on async search
        // For better UX, let's load the first 20 of each to populate the list initially
        api.get<PaginatedList<Tag>>('/tags', { params: { pageSize: 20 } })
            .then(res => setInitialTags(res.data.items.map(t => ({ id: t.id, name: t.name, slug: t.slug }))))
            .catch(console.error);

        api.get<PaginatedList<Artist>>('/artists', { params: { pageSize: 20 } })
            .then(res => setInitialArtists(res.data.items.map(a => ({ id: a.id, name: a.name }))))
            .catch(console.error);
    }, []);

    const loadTags = async (query: string) => {
        const { data } = await api.get<PaginatedList<Tag>>('/tags', { params: { search: query, pageSize: 20 } });
        return data.items.map(t => ({ id: t.id, name: t.name, slug: t.slug }));
    };

    const loadArtists = async (query: string) => {
        const { data } = await api.get<PaginatedList<Artist>>('/artists', { params: { search: query, pageSize: 20 } });
        return data.items.map(a => ({ id: a.id, name: a.name }));
    };

    const updateFilter = (key: string, value: string | null) => {
        setSearchParams(prev => {
            if (value) prev.set(key, value);
            else prev.delete(key);
            return prev;
        });
    };

    const handleTagChange = (key: 'includedTags' | 'excludedTags', tag: Option, action: 'add' | 'remove') => {
        const value = tag.slug || tag.id.toString();
        setSearchParams(prev => {
            const current = prev.getAll(key);
            prev.delete(key);
            if (action === 'add' && !current.includes(value)) {
                [...current, value].forEach(t => prev.append(key, t));
            } else if (action === 'remove') {
                current.filter(t => t !== value).forEach(t => prev.append(key, t));
            } else {
                current.forEach(t => prev.append(key, t));
            }
            return prev;
        });
    };

    const handleArtistChange = (key: 'includedArtists' | 'excludedArtists', artist: Option, action: 'add' | 'remove') => {
        const value = artist.id.toString();
        setSearchParams(prev => {
            const current = prev.getAll(key);
            prev.delete(key);
            if (action === 'add' && !current.includes(value)) {
                [...current, value].forEach(t => prev.append(key, t));
            } else if (action === 'remove') {
                current.filter(t => t !== value).forEach(t => prev.append(key, t));
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

    const includedTagSlugs = searchParams.getAll('includedTags');
    const excludedTagSlugs = searchParams.getAll('excludedTags');
    const includedArtistIds = searchParams.getAll('includedArtists');
    const excludedArtistIds = searchParams.getAll('excludedArtists');
    const includedIds = searchParams.getAll('includedIds');
    const excludedIds = searchParams.getAll('excludedIds');
    const width = searchParams.get('width') || '';
    const height = searchParams.get('height') || '';
    const byteSize = searchParams.get('byteSize') || '';

    // Helper to reconstruct selected options from URL params
    // Note: This is imperfect because we might not have the full object (name) if it wasn't in the initial list
    // Ideally, we would fetch the details for selected IDs if they are missing, but for now we fallback to displaying the ID/Slug
    const getSelectedTags = (slugs: string[]) => {
        return slugs.map(slug => {
            // Try to find in initial list or just create a placeholder
            // In a real app, you might want to fetch these specific tags by slug if missing
            const found = initialTags.find(t => t.slug === slug);
            return found || { id: slug, name: slug, slug: slug };
        });
    };

    const getSelectedArtists = (ids: string[]) => {
        return ids.map(id => {
            const found = initialArtists.find(a => a.id.toString() === id);
            return found || { id: id, name: `ID: ${id}` };
        });
    };

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
                                type="text" placeholder="Width"
                                value={width} onChange={e => updateFilter('width', e.target.value)}
                                className="w-1/2 p-2 rounded bg-secondary text-sm border-transparent focus:ring-1 focus:ring-primary outline-none"
                            />
                            <input
                                type="text" placeholder="Height"
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
                    loadOptions={loadTags}
                    selectedOptions={getSelectedTags(includedTagSlugs)}
                    onSelect={(o) => handleTagChange('includedTags', o, 'add')}
                    onRemove={(o) => handleTagChange('includedTags', o, 'remove')}
                    isMulti={true}
                />

                <SearchableSelect
                    label="Excluded Tags"
                    placeholder="Exclude tags..."
                    loadOptions={loadTags}
                    selectedOptions={getSelectedTags(excludedTagSlugs)}
                    onSelect={(o) => handleTagChange('excludedTags', o, 'add')}
                    onRemove={(o) => handleTagChange('excludedTags', o, 'remove')}
                    isMulti={true}
                />

                <div className="border-t border-border"></div>

                <SearchableSelect
                    label="Included Artists"
                    placeholder="Search artists..."
                    loadOptions={loadArtists}
                    selectedOptions={getSelectedArtists(includedArtistIds)}
                    onSelect={(o) => handleArtistChange('includedArtists', o, 'add')}
                    onRemove={(o) => handleArtistChange('includedArtists', o, 'remove')}
                    isMulti={true}
                />

                <SearchableSelect
                    label="Excluded Artists"
                    placeholder="Exclude artists..."
                    loadOptions={loadArtists}
                    selectedOptions={getSelectedArtists(excludedArtistIds)}
                    onSelect={(o) => handleArtistChange('excludedArtists', o, 'add')}
                    onRemove={(o) => handleArtistChange('excludedArtists', o, 'remove')}
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

            <div className="p-4 border-t border-border md:hidden bg-card">
                <button onClick={() => setShowFilters(false)} className="w-full py-3 bg-primary text-primary-foreground rounded-xl font-bold">Done</button>
            </div>
        </aside>
    );
};

export default FilterSidebar;
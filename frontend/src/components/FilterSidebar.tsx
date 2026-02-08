import { useEffect, useState, useRef } from 'react';
import SearchableSelect, { Option } from './SearchableSelect';
import api from '../services/api';
import { PaginatedList, Tag, Artist, User, NsfwMode, AnimatedMode, Orientation, Role } from '../types';
import { X } from 'lucide-react';
import { useAuth } from '../context/AuthContext';

interface FilterSidebarProps {
    searchParams: URLSearchParams;
    setSearchParams: (params: URLSearchParams | ((prev: URLSearchParams) => URLSearchParams)) => void;
    showFilters: boolean;
    setShowFilters: (show: boolean) => void;
    sortOptions: Option[];
}

const FilterSidebar = ({ searchParams, setSearchParams, showFilters, setShowFilters, sortOptions }: FilterSidebarProps) => {
    const { user } = useAuth();
    const isModeratorOrAdmin = user?.role === Role.Moderator || user?.role === Role.Admin;

    const tagsPageSize = 30;
    const artistsPageSize = 30;
    const usersPageSize = 30;
    const [initialTags, setInitialTags] = useState<Option[]>([]);
    const [initialArtists, setInitialArtists] = useState<Option[]>([]);
    const [initialUsers, setInitialUsers] = useState<Option[]>([]);
    const [initialLoadDone, setInitialLoadDone] = useState(false);
    const fetchedArtistIds = useRef<Set<string>>(new Set());
    const fetchedTagSlugs = useRef<Set<string>>(new Set());
    const fetchedUserIds = useRef<Set<string>>(new Set());

    const [includedIdInput, setIncludedIdInput] = useState('');
    const [excludedIdInput, setExcludedIdInput] = useState('');

    // Load initial data for selected items if needed, or just some defaults
    useEffect(() => {
        const loadInitialData = async () => {
            try {
                const [tagsRes, artistsRes] = await Promise.all([
                    api.get<PaginatedList<Tag>>('/tags', { params: { pageSize: tagsPageSize } }),
                    api.get<PaginatedList<Artist>>('/artists', { params: { pageSize: artistsPageSize } })
                ]);

                setInitialTags(tagsRes.data.items.map(t => ({ id: t.id, name: t.name, slug: t.slug })));
                setInitialArtists(artistsRes.data.items.map(a => ({ id: a.id, name: a.name })));

                if (isModeratorOrAdmin) {
                    const usersRes = await api.get<PaginatedList<User>>('/users', { params: { pageSize: usersPageSize } });
                    setInitialUsers(usersRes.data.items.map(u => ({ id: u.id, name: u.name })));
                }
            } catch (error) {
                console.error(error);
            } finally {
                setInitialLoadDone(true);
            }
        };

        loadInitialData();
    }, [isModeratorOrAdmin]);

    // Load selected artists from URL that might not be in initial batch (single batch request)
    useEffect(() => {
        if (!initialLoadDone) return;

        const artistIdsFromUrl = [...searchParams.getAll('includedArtists'), ...searchParams.getAll('excludedArtists')];
        const missingIds = artistIdsFromUrl.filter(id =>
            !initialArtists.some(a => a.id.toString() === id) && !fetchedArtistIds.current.has(id)
        );

        if (missingIds.length > 0) {
            missingIds.forEach(id => fetchedArtistIds.current.add(id));

            api.get<PaginatedList<Artist>>('/artists', { params: { includedIds: missingIds } })
                .then(res => {
                    const newArtists = res.data.items.map(a => ({ id: a.id, name: a.name } as Option));
                    if (newArtists.length > 0) {
                        setInitialArtists(prev => [...prev, ...newArtists]);
                    }
                })
                .catch(() => {});
        }
    }, [searchParams, initialArtists, initialLoadDone]);

    // Load selected tags from URL that might not be in initial batch (single batch request)
    useEffect(() => {
        if (!initialLoadDone) return;

        const tagSlugsFromUrl = [...searchParams.getAll('includedTags'), ...searchParams.getAll('excludedTags')];
        const missingSlugs = tagSlugsFromUrl.filter(slug =>
            !initialTags.some(t => t.slug === slug) && !fetchedTagSlugs.current.has(slug)
        );

        if (missingSlugs.length > 0) {
            missingSlugs.forEach(slug => fetchedTagSlugs.current.add(slug));

            api.get<PaginatedList<Tag>>('/tags', { params: { includedSlugs: missingSlugs } })
                .then(res => {
                    const newTags = res.data.items.map(t => ({ id: t.id, name: t.name, slug: t.slug } as Option));
                    if (newTags.length > 0) {
                        setInitialTags(prev => [...prev, ...newTags]);
                    }
                })
                .catch(() => {});
        }
    }, [searchParams, initialTags, initialLoadDone]);

    // Load selected uploader from URL that might not be in initial batch
    useEffect(() => {
        if (!initialLoadDone) return;

        const uploaderIdFromUrl = searchParams.get('uploaderId');
        if (uploaderIdFromUrl &&
            !initialUsers.some(u => u.id.toString() === uploaderIdFromUrl) &&
            !fetchedUserIds.current.has(uploaderIdFromUrl)) {

            fetchedUserIds.current.add(uploaderIdFromUrl);

            api.get<PaginatedList<User>>('/users', { params: { includedIds: [uploaderIdFromUrl] } })
                .then(res => {
                    if (res.data.items.length > 0) {
                        const user = res.data.items[0];
                        setInitialUsers(prev => [...prev, { id: user.id, name: user.name } as Option]);
                    }
                })
                .catch(() => {});
        }
    }, [searchParams, initialUsers, initialLoadDone]);

    const loadTags = async (query: string, page: number = 1) => {
        const { data } = await api.get<PaginatedList<Tag>>('/tags', { params: { name: query, pageSize: tagsPageSize, page } });
        return data.items.map(t => ({ id: t.id, name: t.name, slug: t.slug, description: t.description }));
    };

    const loadArtists = async (query: string, page: number = 1) => {
        const { data } = await api.get<PaginatedList<Artist>>('/artists', { params: { name: query, pageSize: artistsPageSize, page } });
        return data.items.map(a => ({ id: a.id, name: a.name }));
    };

    const loadUsers = async (query: string, page: number = 1) => {
        // Check if query is a number (ID) or string (name)
        const params: Record<string, string | number> = { pageSize: usersPageSize, page };
        if (query) {
            if (/^\d+$/.test(query)) {
                params.includedIds = query;
            } else {
                params.name = query;
            }
        }
        const { data } = await api.get<PaginatedList<User>>('/users', { params });
        return data.items.map(u => ({ id: u.id, name: u.name }));
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

        // Add tag to cache if not already present
        if (action === 'add' && !initialTags.some(t => t.slug === value)) {
            setInitialTags(prev => [...prev, tag]);
        }

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

        // Add artist to cache if not already present (fixes "ID: xxx" display issue)
        if (action === 'add' && !initialArtists.some(a => a.id.toString() === value)) {
            setInitialArtists(prev => [...prev, artist]);
        }

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
            // Validate that input is an integer
            if (!/^\d+$/.test(input.trim())) {
                return; // Or show an error/toast
            }
            handleIdChange(key, input.trim(), 'add');
            setInput('');
        }
    };

    const handleIdInputChange = (e: React.ChangeEvent<HTMLInputElement>, setInput: (val: string) => void) => {
        const val = e.target.value;
        // Only allow digits
        if (val === '' || /^\d+$/.test(val)) {
            setInput(val);
        }
    };

    const orderBy = searchParams.get('orderBy') || 'UploadedAt';
    const isNsfw = searchParams.get('isNsfw') || NsfwMode.False;
    const orientation = searchParams.get('orientation') || Orientation.All;
    const isAnimated = searchParams.get('isAnimated') || AnimatedMode.All;

    const includedTagSlugs = searchParams.getAll('includedTags');
    const excludedTagSlugs = searchParams.getAll('excludedTags');
    const includedArtistIds = searchParams.getAll('includedArtists');
    const excludedArtistIds = searchParams.getAll('excludedArtists');
    const includedIds = searchParams.getAll('includedIds');
    const excludedIds = searchParams.getAll('excludedIds');
    const uploaderId = searchParams.get('uploaderId') || '';
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

    const getSelectedUploader = (id: string): Option[] => {
        if (!id) return [];
        const found = initialUsers.find(u => u.id.toString() === id);
        return [found || { id: id, name: `ID: ${id}` }];
    };

    const ratingOptions = [
        { id: NsfwMode.False, name: 'Safe' },
        { id: NsfwMode.True, name: 'NSFW (18+)' },
        { id: NsfwMode.All, name: 'All Content' }
    ];

    const orientationOptions = [
        { id: Orientation.All, name: 'Any' },
        { id: Orientation.Landscape, name: 'Landscape' },
        { id: Orientation.Portrait, name: 'Portrait' },
        { id: Orientation.Square, name: 'Square' }
    ];

    const animationOptions = [
        { id: AnimatedMode.All, name: 'Any' },
        { id: AnimatedMode.False, name: 'Static Image' },
        { id: AnimatedMode.True, name: 'Animated (GIF)' }
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
                    selectedOptions={animationOptions.filter(o => o.id === isAnimated)}
                    onSelect={(o) => updateFilter('isAnimated', o.id as string)}
                    onRemove={() => updateFilter('isAnimated', null)}
                    isMulti={false}
                />

                <div className="grid grid-cols-2 gap-3">
                    <div className="col-span-2">
                        <label className="block text-xs font-bold uppercase text-muted-foreground mb-1">Dimensions (px)</label>
                        <div className="flex gap-2">
                            <input
                                type="text" placeholder="Width (e.g. > 1920)"
                                value={width} onChange={e => updateFilter('width', e.target.value)}
                                className="w-1/2 p-2 rounded bg-secondary text-sm border-transparent focus:ring-1 focus:ring-primary outline-none"
                            />
                            <input
                                type="text" placeholder="Height (e.g. < 1080)"
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

                {isModeratorOrAdmin && (
                    <>
                        <div className="border-t border-border"></div>

                        <SearchableSelect
                            label="Uploader"
                            placeholder="Search users by name or ID..."
                            loadOptions={loadUsers}
                            selectedOptions={getSelectedUploader(uploaderId)}
                            onSelect={(o) => {
                                // Add user to cache if not already present
                                if (!initialUsers.some(u => u.id.toString() === o.id.toString())) {
                                    setInitialUsers(prev => [...prev, o]);
                                }
                                updateFilter('uploaderId', o.id.toString());
                            }}
                            onRemove={() => updateFilter('uploaderId', null)}
                            isMulti={false}
                        />
                    </>
                )}

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
                        onChange={e => handleIdInputChange(e, setIncludedIdInput)}
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
                        onChange={e => handleIdInputChange(e, setExcludedIdInput)}
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
import { useState, useEffect, useCallback, Dispatch, SetStateAction, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import api from '../services/api';
import { PaginatedList } from '../types';
import { useNotification } from '../context/NotificationContext';

export interface UseResourceResult<T> {
    items: T[];
    loading: boolean;
    page: number;
    setPage: Dispatch<SetStateAction<number>>;
    totalPages: number;
    search: string;
    setSearch: (value: string) => void;
    searchType: 'name' | 'id';
    setSearchType: (type: 'name' | 'id') => void;
    createItem: (data: any, successMessage?: string) => Promise<boolean>;
    updateItem: (id: number | string, data: any, successMessage?: string) => Promise<boolean>;
    deleteItem: (id: number | string, successMessage?: string) => Promise<boolean>;
    refresh: () => Promise<void>;
}

export function useResource<T extends { id: number | string }>(
    endpoint: string,
    pageSize: number = 50,
    initialParams: Record<string, any> = {}
): UseResourceResult<T> {
    const { showNotification } = useNotification();
    const [searchParams, setSearchParams] = useSearchParams();

    const [items, setItems] = useState<T[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);

    // Initialize search from URL query param if present
    const [search, _setSearch] = useState(searchParams.get('name') || searchParams.get('id') || '');
    const [searchType, _setSearchType] = useState<'name' | 'id'>(searchParams.get('id') ? 'id' : 'name');

    // Ref to prevent URL from overwriting local state immediately after a user action
    const ignoreUrlSync = useRef(false);

    // Wrapper for setSearch to handle the sync flag
    const setSearch = (value: string) => {
        if (value !== search) {
            ignoreUrlSync.current = true;
            _setSearch(value);
        }
    };

    // Wrapper for setSearchType to handle the sync flag
    const setSearchType = (type: 'name' | 'id') => {
        if (type !== searchType) {
            ignoreUrlSync.current = true;
            _setSearchType(type);
        }
    };

    // Custom debounce logic
    const [debouncedSearch, setDebouncedSearch] = useState(search);

    useEffect(() => {
        if (search === '') {
            setDebouncedSearch('');
            return;
        }

        const handler = setTimeout(() => {
            setDebouncedSearch(search);
        }, 500);

        return () => {
            clearTimeout(handler);
        };
    }, [search]);

    const fetchData = useCallback(async () => {
        // Prevent loading flicker/requests if searchType is ID but search term is invalid
        if (searchType === 'id' && debouncedSearch && !/^\d+$/.test(debouncedSearch)) {
            return;
        }

        setLoading(true);
        try {
            const params: any = { page, pageSize, ...initialParams };

            if (debouncedSearch) {
                if (searchType === 'id') {
                    if (/^\d+$/.test(debouncedSearch)) {
                        params.id = debouncedSearch;
                    }
                } else {
                    params.name = debouncedSearch;
                }
            }

            const { data } = await api.get<PaginatedList<T>>(endpoint, { params });
            setItems(data.items);
            setTotalPages(data.totalPages);
        } catch (error) {
            console.error(error);
        } finally {
            setLoading(false);
        }
    }, [endpoint, page, pageSize, debouncedSearch, searchType, JSON.stringify(initialParams)]);

    useEffect(() => {
        setPage(1);
    }, [debouncedSearch, searchType]);

    // Sync URL with debounced search
    useEffect(() => {
        const currentParams = new URLSearchParams(searchParams);
        const currentNameParam = currentParams.get('name') || '';
        const currentIdParam = currentParams.get('id') || '';

        if (debouncedSearch) {
            if (searchType === 'id') {
                if (/^\d+$/.test(debouncedSearch)) {
                    // Only update if actually different to avoid redundant history/loops
                    if (currentIdParam !== debouncedSearch || currentParams.has('name')) {
                        currentParams.set('id', debouncedSearch);
                        currentParams.delete('name');
                        setSearchParams(currentParams, { replace: true });
                    }
                }
            } else {
                if (currentNameParam !== debouncedSearch || currentParams.has('id')) {
                    currentParams.set('name', debouncedSearch);
                    currentParams.delete('id');
                    setSearchParams(currentParams, { replace: true });
                }
            }
        } else {
            if (currentNameParam || currentIdParam) {
                currentParams.delete('name');
                currentParams.delete('id');
                setSearchParams(currentParams, { replace: true });
            }
        }
    }, [debouncedSearch, searchType]);

    // Sync state with URL (for back/forward navigation)
    const urlName = searchParams.get('name') || '';
    const urlId = searchParams.get('id') || '';
    const urlSearch = urlId || urlName;

    useEffect(() => {
        // If we just updated locally, ignore the stale URL values for one render cycle
        if (ignoreUrlSync.current) {
            ignoreUrlSync.current = false;
            return;
        }

        if (urlSearch !== search) {
            _setSearch(urlSearch);
        }

        // Only switch type if the URL explicitly has a value
        if (urlId && searchType !== 'id') {
            _setSearchType('id');
        } else if (urlName && searchType !== 'name') {
            _setSearchType('name');
        }
    }, [urlSearch, urlId, urlName, searchType, search]);

    useEffect(() => {
        // Skip fetch if invalid ID format
        if (searchType === 'id' && search && !/^\d+$/.test(search)) {
            return;
        }
        fetchData();
    }, [fetchData, searchType, debouncedSearch]); // Trigger on debouncedSearch, not raw search

    const createItem = async (data: any, successMessage = 'Created successfully') => {
        try {
            await api.post(endpoint, data);
            showNotification('success', successMessage);
            fetchData();
            return true;
        } catch (e) {
            return false;
        }
    };

    const updateItem = async (id: number | string, data: any, successMessage = 'Updated successfully') => {
        try {
            await api.put(`${endpoint}/${id}`, data);
            showNotification('success', successMessage);
            fetchData();
            return true;
        } catch (e) {
            return false;
        }
    };

    const deleteItem = async (id: number | string, successMessage = 'Deleted successfully') => {
        try {
            await api.delete(`${endpoint}/${id}`);
            showNotification('success', successMessage);
            fetchData();
            return true;
        } catch (e) {
            return false;
        }
    };

    return {
        items,
        loading,
        page,
        setPage,
        totalPages,
        search,
        setSearch,
        searchType,
        setSearchType,
        createItem,
        updateItem,
        deleteItem,
        refresh: fetchData
    };
}
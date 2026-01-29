import { useState, useEffect, useCallback } from 'react';
import api from '../services/api';
import { PaginatedList } from '../types';
import { useNotification } from '../context/NotificationContext';
import { useDebounce } from './useDebounce';

export function useResource<T extends { id: number | string }>(
    endpoint: string,
    pageSize: number = 50,
    initialParams: Record<string, any> = {}
) {
    const { showNotification } = useNotification();
    const [items, setItems] = useState<T[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const [search, setSearch] = useState('');
    const debouncedSearch = useDebounce(search, 500);

    const fetchData = useCallback(async () => {
        setLoading(true);
        try {
            const params: any = { page, pageSize, ...initialParams };
            if (debouncedSearch) params.search = debouncedSearch;
            
            const { data } = await api.get<PaginatedList<T>>(endpoint, { params });
            setItems(data.items);
            setTotalPages(data.totalPages);
        } catch (error) {
            console.error(error);
        } finally {
            setLoading(false);
        }
    }, [endpoint, page, pageSize, debouncedSearch, JSON.stringify(initialParams)]);

    useEffect(() => {
        setPage(1);
    }, [debouncedSearch]);

    useEffect(() => {
        fetchData();
    }, [fetchData]);

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
        createItem,
        updateItem,
        deleteItem,
        refresh: fetchData
    };
}
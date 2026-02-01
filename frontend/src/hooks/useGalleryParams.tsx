import { useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ImageSort } from '../types';

export const useGalleryParams = (defaultSort = ImageSort.RANDOM) => {
    const [searchParams, setSearchParams] = useSearchParams();

    const isNsfw = searchParams.get('isNsfw') || '0';
    const orderBy = searchParams.get('orderBy') || defaultSort;
    const orientation = searchParams.get('orientation') || '';
    const isAnimatedStr = searchParams.get('isAnimated');
    const height = searchParams.get('height') || '';
    const width = searchParams.get('width') || '';
    const byteSize = searchParams.get('byteSize') || '';
    const includedTags = searchParams.getAll('includedTags');
    const excludedTags = searchParams.getAll('excludedTags');
    const includedArtists = searchParams.getAll('includedArtists');
    const excludedArtists = searchParams.getAll('excludedArtists');
    const includedIds = searchParams.getAll('includedIds');
    const excludedIds = searchParams.getAll('excludedIds');

    const pageStr = searchParams.get('page');
    const page = pageStr ? parseInt(pageStr) : 1;

    const prevFiltersRef = useRef(JSON.stringify({ isNsfw, orderBy, orientation, isAnimatedStr, height, width, byteSize, includedTags, excludedTags, includedArtists, excludedArtists, includedIds, excludedIds }));

    useEffect(() => {
        const currentFilters = JSON.stringify({ isNsfw, orderBy, orientation, isAnimatedStr, height, width, byteSize, includedTags, excludedTags, includedArtists, excludedArtists, includedIds, excludedIds });
        if (prevFiltersRef.current !== currentFilters) {
            if (page !== 1) setSearchParams(prev => { prev.set('page', '1'); return prev; });
            prevFiltersRef.current = currentFilters;
        }
    }, [isNsfw, orderBy, orientation, isAnimatedStr, height, width, byteSize, includedTags, excludedTags, includedArtists, excludedArtists, includedIds, excludedIds, page, setSearchParams]);

    useEffect(() => {
        if (!searchParams.has('orderBy')) setSearchParams(prev => { prev.set('orderBy', defaultSort); return prev; }, { replace: true });
    }, [searchParams, setSearchParams, defaultSort]);

    const setPage = (newPage: number) => setSearchParams(prev => { prev.set('page', newPage.toString()); return prev; });

    const filters = {
        isNsfw: parseInt(isNsfw),
        orderBy,
        orientation,
        isAnimated: isAnimatedStr === 'true' ? true : isAnimatedStr === 'false' ? false : undefined,
        width, height, byteSize,
        includedTags, excludedTags, includedArtists, excludedArtists, includedIds, excludedIds,
        page, pageSize: 30
    };

    return {
        filters,
        page,
        setPage,
        searchParams,
        setSearchParams,
        isNsfw
    };
};
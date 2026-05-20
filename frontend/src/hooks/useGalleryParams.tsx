import { useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ImageOrderBy, NsfwMode, AnimatedMode, Orientation, ReviewStatusFilter } from '../types';

export const useGalleryParams = (defaultSort = ImageOrderBy.Random) => {
    const [searchParams, setSearchParams] = useSearchParams();

    const isNsfw = (searchParams.get('isNsfw') as NsfwMode) || NsfwMode.False;
    const orderBy = (searchParams.get('orderBy') as ImageOrderBy) || defaultSort;
    const orientation = (searchParams.get('orientation') as Orientation) || Orientation.All;
    const isAnimated = (searchParams.get('isAnimated') as AnimatedMode) || AnimatedMode.All;
    const height = searchParams.get('height') || '';
    const width = searchParams.get('width') || '';
    const byteSize = searchParams.get('byteSize') || '';
    const includedTags = searchParams.getAll('includedTags');
    const excludedTags = searchParams.getAll('excludedTags');
    const includedArtists = searchParams.getAll('includedArtists');
    const excludedArtists = searchParams.getAll('excludedArtists');
    const includedIds = searchParams.getAll('includedIds');
    const excludedIds = searchParams.getAll('excludedIds');
    const uploaderId = searchParams.get('uploaderId') || '';
    const reviewStatus = (searchParams.get('reviewStatus') as ReviewStatusFilter) || ReviewStatusFilter.Accepted;

    const pageStr = searchParams.get('page');
    const page = pageStr ? parseInt(pageStr) : 1;

    const prevFiltersRef = useRef(JSON.stringify({ isNsfw, orderBy, orientation, isAnimated, height, width, byteSize, includedTags, excludedTags, includedArtists, excludedArtists, includedIds, excludedIds, uploaderId, reviewStatus }));

    useEffect(() => {
        const currentFilters = JSON.stringify({ isNsfw, orderBy, orientation, isAnimated, height, width, byteSize, includedTags, excludedTags, includedArtists, excludedArtists, includedIds, excludedIds, uploaderId, reviewStatus });
        if (prevFiltersRef.current !== currentFilters) {
            if (page !== 1) setSearchParams(prev => { prev.set('page', '1'); return prev; });
            prevFiltersRef.current = currentFilters;
        }
    }, [isNsfw, orderBy, orientation, isAnimated, height, width, byteSize, includedTags, excludedTags, includedArtists, excludedArtists, includedIds, excludedIds, uploaderId, reviewStatus, page, setSearchParams]);

    useEffect(() => {
        if (!searchParams.has('orderBy')) setSearchParams(prev => { prev.set('orderBy', defaultSort); return prev; }, { replace: true });
    }, [searchParams, setSearchParams, defaultSort]);

    const setPage = (newPage: number) => setSearchParams(prev => { prev.set('page', newPage.toString()); return prev; });

    const filters = {
        isNsfw,
        orderBy,
        orientation: orientation !== Orientation.All ? orientation : undefined,
        isAnimated: isAnimated !== AnimatedMode.All ? isAnimated : undefined,
        width, height, byteSize,
        includedTags, excludedTags, includedArtists, excludedArtists, includedIds, excludedIds,
        uploaderId: uploaderId || undefined,
        reviewStatus: reviewStatus !== ReviewStatusFilter.Accepted ? reviewStatus : undefined,
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
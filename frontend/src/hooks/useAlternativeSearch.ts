import { useImages, ImageFilters } from './useImages';
import { ImageDto, NsfwMode } from '../types';

export const useAlternativeSearch = (
    isLoading: boolean,
    images: ImageDto[] | undefined,
    isNsfw: NsfwMode,
    filters: ImageFilters
) => {
    const shouldCheck = !isLoading && images?.length === 0 && isNsfw === NsfwMode.False;

    // We reuse the existing useImages hook but override specific parameters
    const { data: alternativeData, isLoading: isCheckingAlternatives } = useImages({
        ...filters,
        isNsfw: NsfwMode.All, // Force search everywhere
        page: 1,
        pageSize: 1, // Minimal fetch just to get totalItems
        enabled: shouldCheck // Only run this query if we need to check
    });

    // Only return valid count if we actually intended to check
    const alternativeCount = shouldCheck && alternativeData?.totalCount && alternativeData.totalCount > 0
        ? alternativeData.totalCount
        : null;

    return {
        alternativeCount,
        isCheckingAlternatives: shouldCheck && isCheckingAlternatives
    };
};

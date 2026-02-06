import { useQuery } from '@tanstack/react-query';
import api from '../services/api';
import { ImageDto, PaginatedList, NsfwMode, AnimatedMode, Orientation, ImageOrderBy } from '../types';
import { useAuth } from '../context/AuthContext';
import { buildQueryParams } from '../utils/queryParams';

export interface ImageFilters {
  isNsfw?: NsfwMode;
  includedTags?: string[];
  excludedTags?: string[];
  includedArtists?: string[];
  excludedArtists?: string[];
  includedIds?: string[];
  excludedIds?: string[];
  isAnimated?: AnimatedMode;
  orientation?: Orientation;
  orderBy?: ImageOrderBy;
  page?: number;
  pageSize?: number;
  width?: string;
  height?: string;
  byteSize?: string;
  albumId?: string;
  enabled?: boolean;
  uploaderId?: string;
}

export const useImages = (filters: ImageFilters) => {
  const { user } = useAuth();
  const isAlbumRequest = !!filters.albumId;
  const isEnabled = filters.enabled !== undefined ? filters.enabled : true;

  return useQuery({
    queryKey: ['images', filters, user?.id],
    queryFn: async () => {
      const params = buildQueryParams({
        params: {
          isNsfw: filters.isNsfw,
          isAnimated: filters.isAnimated,
          orderBy: filters.orderBy,
          orientation: filters.orientation,
          width: filters.width,
          height: filters.height,
          byteSize: filters.byteSize,
          page: filters.page,
          pageSize: filters.pageSize,
          uploaderId: filters.uploaderId,
        },
        arrays: {
          includedTags: filters.includedTags,
          excludedTags: filters.excludedTags,
          includedArtists: filters.includedArtists,
          excludedArtists: filters.excludedArtists,
          includedIds: filters.includedIds,
          excludedIds: filters.excludedIds,
        },
      });

      const endpoint = isAlbumRequest
          ? `/users/me/albums/${filters.albumId}/images`
          : '/images';

      const { data } = await api.get<PaginatedList<ImageDto>>(endpoint, { params });
      return data;
    },
    enabled: isEnabled && (!isAlbumRequest || (isAlbumRequest && !!user)),
    staleTime: 0,
    refetchOnWindowFocus: false,
  });
};
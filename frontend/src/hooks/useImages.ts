import { useQuery } from '@tanstack/react-query';
import api from '../services/api';
import { ImageDto, PaginatedList } from '../types';
import { useAuth } from '../context/AuthContext';

export interface ImageFilters {
  isNsfw?: number;
  includedTags?: string[];
  excludedTags?: string[];
  includedArtists?: string[]; // IDs (en string)
  excludedArtists?: string[]; // IDs (en string)
  includedIds?: string[];
  excludedIds?: string[];
  isAnimated?: boolean;
  orientation?: string;
  orderBy?: string;
  page?: number;
  pageSize?: number;
  width?: string;
  height?: string;
  byteSize?: string;
  albumId?: number; // J'ai ajouté ça au cas où, vu le backend
}

export const useImages = (filters: ImageFilters) => {
  const { user } = useAuth();

  return useQuery({
    queryKey: ['images', filters, user?.id],
    queryFn: async () => {
      const params = new URLSearchParams();

      if (filters.isNsfw !== undefined) params.append('isNsfw', filters.isNsfw.toString());
      if (filters.isAnimated !== undefined) params.append('isAnimated', filters.isAnimated.toString());

      if (filters.orderBy) params.append('orderBy', filters.orderBy);
      if (filters.orientation) params.append('orientation', filters.orientation);
      if (filters.width) params.append('width', filters.width);
      if (filters.height) params.append('height', filters.height);
      if (filters.byteSize) params.append('byteSize', filters.byteSize);

      if (filters.page) params.append('page', filters.page.toString());
      if (filters.pageSize) params.append('pageSize', filters.pageSize.toString());

      // Les tags envoient maintenant les slugs
      filters.includedTags?.forEach(tag => params.append('includedTags', tag));
      filters.excludedTags?.forEach(tag => params.append('excludedTags', tag));

      filters.includedArtists?.forEach(artist => params.append('includedArtists', artist));
      filters.excludedArtists?.forEach(artist => params.append('excludedArtists', artist));

      filters.includedIds?.forEach(id => params.append('includedIds', id));
      filters.excludedIds?.forEach(id => params.append('excludedIds', id));

      const { data } = await api.get<PaginatedList<ImageDto>>('/images', { params });
      return data;
    },
    staleTime: 1000 * 60 * 5,
  });
};
import { useQuery } from '@tanstack/react-query';
import api from '../services/api';
import { ImageDto } from '../types';

export interface ImageFilters {
  isNsfw?: number; // 0 = Safe, 1 = Nsfw, 2 = All
  includedTags?: string[];
  excludedTags?: string[];
  orderBy?: string;
  limit?: number;
}

export const useImages = (filters: ImageFilters) => {
  return useQuery({
    queryKey: ['images', filters],
    queryFn: async () => {
      // Convert array params to repeated query params if needed, or comma separated
      // Axios handles arrays as 'key[]=value&key[]=value' by default.
      // Backend expects 'IncludedTags=tag1&IncludedTags=tag2'.
      // We need to ensure params are passed correctly.
      
      const params = new URLSearchParams();
      if (filters.isNsfw !== undefined) params.append('isNsfw', filters.isNsfw.toString());
      if (filters.orderBy) params.append('orderBy', filters.orderBy);
      if (filters.limit) params.append('limit', filters.limit.toString());
      
      filters.includedTags?.forEach(tag => params.append('includedTags', tag));
      filters.excludedTags?.forEach(tag => params.append('excludedTags', tag));

      const { data } = await api.get<ImageDto[]>('/images', { params });
      return data;
    },
    staleTime: 1000 * 60 * 5, // 5 minutes
  });
};

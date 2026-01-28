import { useQuery } from '@tanstack/react-query';
import api from '../services/api';
import { ImageDto, PaginatedList } from '../types';
import { useAuth } from '../context/AuthContext';

export interface ImageFilters {
  isNsfw?: number; // 0 = Safe, 1 = Nsfw, 2 = All
  includedTags?: string[];
  excludedTags?: string[];
  orderBy?: string;
  page?: number;      // Changed from limit
  pageSize?: number;  // Added
}

export const useImages = (filters: ImageFilters) => {
  const { user } = useAuth();

  return useQuery({
    queryKey: ['images', filters, user?.id],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (filters.isNsfw !== undefined) params.append('isNsfw', filters.isNsfw.toString());
      if (filters.orderBy) params.append('orderBy', filters.orderBy);

      // Pagination params
      if (filters.page) params.append('page', filters.page.toString());
      if (filters.pageSize) params.append('pageSize', filters.pageSize.toString());

      filters.includedTags?.forEach(tag => params.append('includedTags', tag));
      filters.excludedTags?.forEach(tag => params.append('excludedTags', tag));
      
      const { data } = await api.get<PaginatedList<ImageDto>>('/images', { params });
      return data;
    },
    staleTime: 1000 * 60 * 5,
  });
};
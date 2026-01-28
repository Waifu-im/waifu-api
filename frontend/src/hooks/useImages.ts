import { useQuery } from '@tanstack/react-query';
import api from '../services/api';
import { ImageDto } from '../types';

export const useImages = (filters?: any) => {
  return useQuery({
    queryKey: ['images', filters],
    queryFn: async () => {
      const { data } = await api.get<ImageDto[]>('/images', { params: filters });
      return data;
    },
  });
};

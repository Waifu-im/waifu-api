import { useQuery } from '@tanstack/react-query';
import api from '../services/api';
import { Tag } from '../types';

export const useTags = () => {
  return useQuery({
    queryKey: ['tags'],
    queryFn: async () => {
      const { data } = await api.get<Tag[]>('/tags');
      return data;
    },
    staleTime: 1000 * 60 * 60, // 1 hour
  });
};

import { useQuery } from '@tanstack/react-query';
import api from '../services/api';
import { User } from '../types';

export const useUser = (enabled: boolean) => {
  return useQuery({
    queryKey: ['user', 'me'],
    queryFn: async () => {
      console.log('Fetching user...');
      const { data } = await api.get<User>('/users/me');
      console.log('User fetched:', data);
      return data;
    },
    enabled: enabled,
    retry: false,
    refetchOnMount: true, // Force fetch on mount
  });
};

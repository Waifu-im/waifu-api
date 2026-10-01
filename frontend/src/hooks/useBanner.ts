import { useQuery, QueryClient } from '@tanstack/react-query';
import api from '../services/api';
import { useAuth } from '../context/AuthContext';
import { Banner, UserPermissions } from '../types';

/** Shared query key for the active banner. */
export const BANNER_KEY = ['banner'];

/** Refetch the active banner — call after the banner settings change. */
export const invalidateBanner = (queryClient: QueryClient) =>
    queryClient.invalidateQueries({ queryKey: BANNER_KEY });

/** The banner shown to visitors, or null when none is active (the API answers 204). */
export const useActiveBanner = () =>
    useQuery({
        queryKey: BANNER_KEY,
        staleTime: 5 * 60 * 1000,
        queryFn: async () => {
            const { data } = await api.get<Banner | ''>('/banner', { skipGlobalErrorHandler: true });
            return data || null;
        },
    });

/**
 * The current user's configurable permissions. The minimum role for each restricted feature lives in the
 * backend configuration (e.g. Permissions__BannerManagementMinRole) and is enforced there; the frontend only
 * uses this to decide what to show.
 */
export const usePermissions = () => {
    const { user } = useAuth();

    const query = useQuery({
        queryKey: ['user', 'me', 'permissions', user?.id, user?.role],
        enabled: !!user,
        queryFn: async () => {
            const { data } = await api.get<UserPermissions>('/users/me/permissions', { skipGlobalErrorHandler: true });
            return data;
        },
    });

    return {
        canManageBanner: query.data?.canManageBanner ?? false,
        isLoading: !!user && query.isLoading,
    };
};

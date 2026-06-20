import { useQuery, QueryClient } from '@tanstack/react-query';
import api from '../services/api';
import { useAuth } from '../context/AuthContext';
import { canModerate } from '../utils/roles';

/** Shared query key for the nav count badges (pending submissions + unresolved reports). */
export const NAV_BADGE_KEY = ['nav-badge'];

/** Invalidate the nav count badges so they refetch — call after any action that changes the counts. */
export const invalidateNavBadges = (queryClient: QueryClient) =>
    queryClient.invalidateQueries({ queryKey: NAV_BADGE_KEY });

const countOf = async (url: string, params: Record<string, unknown>) => {
    const { data } = await api.get<{ totalCount: number }>(url, { params, skipGlobalErrorHandler: true });
    return data.totalCount ?? 0;
};

/**
 * Counts for the nav badges. The pending-submissions count auto-scopes (mods get the queue, users get their own)
 * and shows for everyone. The report count is the moderator report queue only (users have no report badge).
 * Backed by react-query so the badges refetch on navigation (the layout invalidates on route change) and
 * immediately after actions like approve/reject/validate (handlers call invalidateNavBadges).
 */
export const useNavBadges = () => {
    const { user } = useAuth();

    const pending = useQuery({
        queryKey: [...NAV_BADGE_KEY, 'review', user?.id],
        enabled: !!user,
        staleTime: 0,
        queryFn: () => countOf('/review/tasks', { status: 'Pending', page: 1, pageSize: 1 }),
    });

    const reports = useQuery({
        queryKey: [...NAV_BADGE_KEY, 'reports', user?.id],
        enabled: !!user && canModerate(user),
        staleTime: 0,
        queryFn: () => countOf('/reports', { status: 'Pending', page: 1, pageSize: 1 }),
    });

    return { pendingCount: pending.data ?? 0, reportCount: reports.data ?? 0 };
};

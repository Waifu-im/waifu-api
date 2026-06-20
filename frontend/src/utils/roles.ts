import { Role, User } from '../types';

/**
 * True for Moderator/Admin. Used to decide who drives visibility via the explicit `reviewStatus` filter
 * (moderators) versus the implicit `includeMyPending` union (regular users see their own pending content).
 */
export const canModerate = (user?: User | null): boolean =>
    !!user && (user.role === Role.Moderator || user.role === Role.Admin);

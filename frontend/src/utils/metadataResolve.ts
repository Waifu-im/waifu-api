import api from '../services/api';
import { PaginatedList, Tag, Artist } from '../types';

/**
 * Resolve tags by slug — used to turn an edit payload's `addTagSlugs` (slugs only) into full tags (id + name),
 * both for the review diff and the gallery "pending tags" strip. Returns [] on failure (callers fall back to slug).
 */
export async function resolveTagsBySlugs(slugs: string[]): Promise<Tag[]> {
    if (slugs.length === 0) return [];
    const params = new URLSearchParams({ pageSize: String(slugs.length) });
    slugs.forEach(s => params.append('IncludedSlugs', s));
    try {
        const { data } = await api.get<PaginatedList<Tag>>(`/tags?${params.toString()}`, { skipGlobalErrorHandler: true });
        return data.items;
    } catch {
        return [];
    }
}

/** Resolve artists by id (e.g. an edit payload's `addArtistIds`). Returns [] on failure. */
export async function resolveArtistsByIds(ids: number[]): Promise<Artist[]> {
    if (ids.length === 0) return [];
    const params = new URLSearchParams({ pageSize: String(ids.length) });
    ids.forEach(id => params.append('IncludedIds', String(id)));
    try {
        const { data } = await api.get<PaginatedList<Artist>>(`/artists?${params.toString()}`, { skipGlobalErrorHandler: true });
        return data.items;
    } catch {
        return [];
    }
}

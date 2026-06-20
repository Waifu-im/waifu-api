import { useQuery } from '@tanstack/react-query';
import api from '../services/api';
import { useAuth } from '../context/AuthContext';
import {
    PaginatedList, ReviewTask, ReviewTaskKind, ReviewTaskStatus, ReviewableContentType,
    ImageEditPayload, Tag, Artist,
} from '../types';
import { resolveTagsBySlugs, resolveArtistsByIds } from '../utils/metadataResolve';

/** A user's own pending (awaiting-review) proposed tag/artist changes to a single image. */
export interface ProposedImageEdit {
    taskId: number;
    addTags: Tag[];
    removeTags: Tag[];
    addArtists: Artist[];
    removeArtists: Artist[];
}

/**
 * The caller's own pending image-edit proposals, keyed by image id, sourced from the existing review-tasks
 * API (which auto-scopes non-moderators to their own submissions). Lets the gallery and image page show a
 * user the not-yet-approved changes they've proposed. One shared request regardless of how many cards call it
 * (react-query dedupes); proposed additions are resolved to names in a single batch, removals come from each
 * task's target image. Returns an empty map for anonymous users.
 */
export const useMyPendingImageEdits = (enabled = true) => {
    const { user } = useAuth();
    return useQuery({
        queryKey: ['my-pending-image-edits', user?.id],
        enabled: enabled && !!user,
        staleTime: 10_000,
        queryFn: async (): Promise<Map<number, ProposedImageEdit>> => {
            const { data } = await api.get<PaginatedList<ReviewTask>>('/review/tasks', {
                params: {
                    mine: true,
                    kind: ReviewTaskKind.Edit,
                    targetType: ReviewableContentType.Image,
                    status: ReviewTaskStatus.Pending,
                    pageSize: 100,
                },
                skipGlobalErrorHandler: true,
            });

            const tasks = data.items.filter(t => t.payload && t.target?.image);

            // Proposed additions aren't linked to the image yet, so resolve all of them to names in one batch each.
            const addSlugs = [...new Set(tasks.flatMap(t => (t.payload as ImageEditPayload).addTagSlugs ?? []))];
            const addArtistIds = [...new Set(tasks.flatMap(t => (t.payload as ImageEditPayload).addArtistIds ?? []))];
            const [resolvedTags, resolvedArtists] = await Promise.all([
                resolveTagsBySlugs(addSlugs),
                resolveArtistsByIds(addArtistIds),
            ]);
            const tagBySlug = new Map(resolvedTags.map(t => [t.slug, t]));
            const artistById = new Map(resolvedArtists.map(a => [a.id, a]));

            const map = new Map<number, ProposedImageEdit>();
            for (const t of tasks) {
                const p = t.payload as ImageEditPayload;
                const img = t.target!.image!;
                const removeTagSlugs = new Set(p.removeTagSlugs ?? []);
                const removeArtistIds = new Set(p.removeArtistIds ?? []);

                const addTags = (p.addTagSlugs ?? []).map(s => tagBySlug.get(s)).filter((x): x is Tag => !!x);
                const addArtists = (p.addArtistIds ?? []).map(id => artistById.get(id)).filter((x): x is Artist => !!x);
                // Removed entities are currently on the image, so their names come straight from the target.
                const removeTags = (img.tags ?? []).filter(tag => removeTagSlugs.has(tag.slug));
                const removeArtists = (img.artists ?? []).filter(a => removeArtistIds.has(a.id));

                if (addTags.length || removeTags.length || addArtists.length || removeArtists.length) {
                    map.set(t.targetId, { taskId: t.id, addTags, removeTags, addArtists, removeArtists });
                }
            }
            return map;
        },
    });
};

import {
    ImageDto,
    Tag,
    Artist,
    ImageEditPayload,
    TagEditPayload,
    ArtistEditPayload,
} from '../types';

/**
 * Helpers that build an edit-request payload by diffing the current value of a
 * target (Image/Tag/Artist) against the proposed value coming out of an edit modal.
 *
 * Rules (per the v1 backend contract):
 *  - Only include a field if it actually changed.
 *  - Clearing a field is NOT supported: a scalar is only sent when it changed to a
 *    non-empty value.
 *  - Image tag/artist membership is expressed as add/remove deltas.
 *  - An empty payload would make the backend return 400, so each builder also reports
 *    whether the resulting payload is empty (`isEmpty`) so callers can guard submission.
 */

export interface PayloadResult<T> {
    payload: T;
    isEmpty: boolean;
}

/** Trim + treat empty string as "no value". Returns undefined when blank. */
const normalize = (value?: string): string | undefined => {
    const trimmed = (value ?? '').trim();
    return trimmed.length > 0 ? trimmed : undefined;
};

/**
 * Diff a single scalar string field. Only returns a value when it changed to a
 * non-empty value (clearing is unsupported in v1).
 */
const diffScalar = (current?: string, proposed?: string): string | undefined => {
    const next = normalize(proposed);
    const prev = normalize(current);
    if (next === undefined) return undefined; // blank or cleared -> ignore
    if (next === prev) return undefined; // unchanged
    return next;
};

/** Proposed image values (mirrors the fields editable in ImageModal). */
export interface ProposedImage {
    source?: string;
    isNsfw: boolean;
    /** Selected tag slugs. */
    tagSlugs: string[];
    /** Selected artist ids. */
    artistIds: number[];
}

export function buildImagePayload(
    current: ImageDto,
    proposed: ProposedImage,
): PayloadResult<ImageEditPayload> {
    const payload: ImageEditPayload = {};

    const source = diffScalar(current.source, proposed.source);
    if (source !== undefined) payload.source = source;

    if (proposed.isNsfw !== current.isNsfw) payload.isNsfw = proposed.isNsfw;

    // Tag membership deltas (compare slugs).
    const currentTagSlugs = (current.tags ?? []).map(t => t.slug);
    const proposedTagSlugs = proposed.tagSlugs;
    const addTagSlugs = proposedTagSlugs.filter(s => !currentTagSlugs.includes(s));
    const removeTagSlugs = currentTagSlugs.filter(s => !proposedTagSlugs.includes(s));
    if (addTagSlugs.length > 0) payload.addTagSlugs = addTagSlugs;
    if (removeTagSlugs.length > 0) payload.removeTagSlugs = removeTagSlugs;

    // Artist membership deltas (compare ids).
    const currentArtistIds = (current.artists ?? []).map(a => a.id);
    const proposedArtistIds = proposed.artistIds;
    const addArtistIds = proposedArtistIds.filter(id => !currentArtistIds.includes(id));
    const removeArtistIds = currentArtistIds.filter(id => !proposedArtistIds.includes(id));
    if (addArtistIds.length > 0) payload.addArtistIds = addArtistIds;
    if (removeArtistIds.length > 0) payload.removeArtistIds = removeArtistIds;

    return { payload, isEmpty: Object.keys(payload).length === 0 };
}

/** Proposed tag values (mirrors TagModal). Note: slug is not editable via suggestions in v1. */
export interface ProposedTag {
    name?: string;
    description?: string;
}

export function buildTagPayload(
    current: Tag,
    proposed: ProposedTag,
): PayloadResult<TagEditPayload> {
    const payload: TagEditPayload = {};

    const name = diffScalar(current.name, proposed.name);
    if (name !== undefined) payload.name = name;

    const description = diffScalar(current.description, proposed.description);
    if (description !== undefined) payload.description = description;

    return { payload, isEmpty: Object.keys(payload).length === 0 };
}

/** Proposed artist values (mirrors ArtistModal). */
export interface ProposedArtist {
    name?: string;
    patreon?: string;
    pixiv?: string;
    twitter?: string;
    deviantArt?: string;
}

export function buildArtistPayload(
    current: Artist,
    proposed: ProposedArtist,
): PayloadResult<ArtistEditPayload> {
    const payload: ArtistEditPayload = {};

    const name = diffScalar(current.name, proposed.name);
    if (name !== undefined) payload.name = name;

    const patreon = diffScalar(current.patreon, proposed.patreon);
    if (patreon !== undefined) payload.patreon = patreon;

    const pixiv = diffScalar(current.pixiv, proposed.pixiv);
    if (pixiv !== undefined) payload.pixiv = pixiv;

    const twitter = diffScalar(current.twitter, proposed.twitter);
    if (twitter !== undefined) payload.twitter = twitter;

    const deviantArt = diffScalar(current.deviantArt, proposed.deviantArt);
    if (deviantArt !== undefined) payload.deviantArt = deviantArt;

    return { payload, isEmpty: Object.keys(payload).length === 0 };
}

import { useState, useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import api from '../services/api';
import {
    ReviewTask,
    ReviewTaskStatus,
    ReviewTaskKind,
    ReviewableContentType,
    ImageEditPayload,
    TagEditPayload,
    ArtistEditPayload,
    UserMinimal,
    PaginatedList,
    Role,
    ImageDto,
    ImageFormData,
    Tag,
    Artist,
} from '../types';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import { useRequireAuth } from '../hooks/useRequireAuth';
import { buildImagePayload, buildTagPayload, buildArtistPayload } from '../utils/editRequests';
import ReasonModal from '../components/modals/ReasonModal';
import ConfirmModal from '../components/modals/ConfirmModal';
import ImageModal from '../components/modals/ImageModal';
import ArtistModal, { ArtistFormData } from '../components/modals/ArtistModal';
import TagModal, { TagFormData } from '../components/modals/TagModal';
import Pagination from '../components/Pagination';
import TagChip from '../components/TagChip';
import { StatusBadge as SharedStatusBadge, StatusTone } from '../components/StatusBadge';
import { resolveTagsBySlugs, resolveArtistsByIds } from '../utils/metadataResolve';
import { imgCard } from '../utils/cfImage';
import { invalidateNavBadges } from '../hooks/useNavBadges';
import {
    Check, X, ExternalLink, Image as ImageIcon, Tag as TagIcon,
    Palette, Clock, Ban, ArrowRight, Sparkles, GitPullRequest, Edit2, Link as LinkIcon,
} from 'lucide-react';

const PAGE_SIZE = 20;

const TYPE_TABS = [ReviewableContentType.Image, ReviewableContentType.Tag, ReviewableContentType.Artist] as const;

// ---- helpers -------------------------------------------------------------

const TARGET_LABELS: Record<ReviewableContentType, string> = {
    [ReviewableContentType.Image]: 'Image',
    [ReviewableContentType.Tag]: 'Tag',
    [ReviewableContentType.Artist]: 'Artist',
};

const targetIcon = (type: ReviewableContentType, size = 14) => {
    switch (type) {
        case ReviewableContentType.Image: return <ImageIcon size={size} />;
        case ReviewableContentType.Tag: return <TagIcon size={size} />;
        case ReviewableContentType.Artist: return <Palette size={size} />;
    }
};

const targetLink = (task: ReviewTask): string => {
    switch (task.targetType) {
        case ReviewableContentType.Image: return `/images/${task.targetId}`;
        case ReviewableContentType.Tag: return `/tags?includedIds=${task.targetId}`;
        case ReviewableContentType.Artist: return `/artists?includedIds=${task.targetId}`;
    }
};

const REVIEW_STATUS: Record<ReviewTaskStatus, { tone: StatusTone; icon: React.ReactNode }> = {
    [ReviewTaskStatus.Pending]: { tone: 'pending', icon: <Clock size={12} /> },
    [ReviewTaskStatus.Approved]: { tone: 'success', icon: <Check size={12} /> },
    [ReviewTaskStatus.Rejected]: { tone: 'danger', icon: <X size={12} /> },
    [ReviewTaskStatus.Cancelled]: { tone: 'neutral', icon: <Ban size={12} /> },
};

const StatusBadge = ({ status }: { status: ReviewTaskStatus }) => (
    <SharedStatusBadge label={status} tone={REVIEW_STATUS[status].tone} icon={REVIEW_STATUS[status].icon} />
);

const KindBadge = ({ kind }: { kind: ReviewTaskKind }) => (
    kind === ReviewTaskKind.NewContent ? (
        <span className="px-2 py-1 rounded text-xs font-bold border flex items-center gap-1 bg-primary/10 text-primary border-primary/20">
            <Sparkles size={12} /> New
        </span>
    ) : (
        <span className="px-2 py-1 rounded text-xs font-bold border flex items-center gap-1 bg-sky-500/10 text-sky-600 border-sky-500/20">
            <GitPullRequest size={12} /> Edit
        </span>
    )
);

/** Shown to the submitter when a moderator changed their submission's content. */
const ModEditedCue = ({ at }: { at?: string }) => at ? (
    <span
        className="px-2 py-1 rounded text-xs font-bold border flex items-center gap-1 bg-amber-500/10 text-amber-600 border-amber-500/20"
        title={`A moderator edited this submission on ${new Date(at).toLocaleString()}`}
    >
        <Edit2 size={12} /> Edited by moderator
    </span>
) : null;

/**
 * Clickable user link, matching the Reports.tsx convention. Falls back to "User #id".
 * `plain` renders just the name (no link) — regular users can't open the /users page.
 */
const UserLink = ({ user, id, plain }: { user?: UserMinimal; id?: number; plain?: boolean }) => {
    const label = user ? user.name : id !== undefined ? `User #${id}` : 'Unknown';
    if (plain || (!user && id === undefined)) {
        return <span className="font-medium">{label}</span>;
    }
    return (
        <Link to={`/users?includedIds=${user ? user.id : id}`} className="hover:text-primary hover:underline font-medium">
            {label}
        </Link>
    );
};

// A single old -> new scalar diff row.
const ScalarDiff = ({ label, oldValue, newValue }: { label: string; oldValue?: string; newValue?: string }) => (
    <div className="text-sm">
        <span className="font-bold text-muted-foreground uppercase text-xs tracking-wider">{label}</span>
        <div className="flex items-center gap-2 flex-wrap mt-0.5">
            <span className="line-through text-muted-foreground break-all">{oldValue || <em>empty</em>}</span>
            <ArrowRight size={14} className="text-muted-foreground shrink-0" />
            <span className="text-foreground font-medium break-all">{newValue || <em>empty</em>}</span>
        </div>
    </div>
);

const FieldLabel = ({ children }: { children: React.ReactNode }) => (
    <span className="font-bold text-muted-foreground uppercase text-xs tracking-wider">{children}</span>
);

const Unavailable = ({ label }: { label: string }) => (
    <p className="text-sm text-muted-foreground italic">The submitted {label.toLowerCase()} is no longer available.</p>
);

const SocialRow = ({ url }: { url: string }) => (
    <a
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        className="flex items-center gap-2 bg-secondary px-2 py-1 rounded hover:bg-primary hover:text-primary-foreground transition-colors text-xs"
        title={url}
    >
        <LinkIcon size={14} className="text-muted-foreground shrink-0" />
        <span className="truncate">{url.replace('https://', '')}</span>
    </a>
);

// ---- Edit diff (current -> proposed) -------------------------------------

const EditDiff = ({ task }: { task: ReviewTask }) => {
    if (task.targetType === ReviewableContentType.Tag) {
        const p = (task.payload ?? {}) as TagEditPayload;
        const current = task.target?.tag;
        return (
            <div className="space-y-2">
                {p.name !== undefined && <ScalarDiff label="Name" oldValue={current?.name} newValue={p.name} />}
                {p.description !== undefined && <ScalarDiff label="Description" oldValue={current?.description} newValue={p.description} />}
            </div>
        );
    }

    if (task.targetType === ReviewableContentType.Artist) {
        const p = (task.payload ?? {}) as ArtistEditPayload;
        const current = task.target?.artist;
        return (
            <div className="space-y-2">
                {p.name !== undefined && <ScalarDiff label="Name" oldValue={current?.name} newValue={p.name} />}
                {p.twitter !== undefined && <ScalarDiff label="Twitter" oldValue={current?.twitter} newValue={p.twitter} />}
                {p.pixiv !== undefined && <ScalarDiff label="Pixiv" oldValue={current?.pixiv} newValue={p.pixiv} />}
                {p.patreon !== undefined && <ScalarDiff label="Patreon" oldValue={current?.patreon} newValue={p.patreon} />}
                {p.deviantArt !== undefined && <ScalarDiff label="DeviantArt" oldValue={current?.deviantArt} newValue={p.deviantArt} />}
            </div>
        );
    }

    return <ImageEditDiff task={task} />;
};

// Image edit diff. The payload only carries tag slugs / artist ids, so added entities are resolved to
// their names via the API; every tag and artist is rendered as a link to its page.
const ImageEditDiff = ({ task }: { task: ReviewTask }) => {
    const p = (task.payload ?? {}) as ImageEditPayload;
    const current = task.target?.image;

    const addTagSlugs = p.addTagSlugs ?? [];
    const removeTagSlugs = p.removeTagSlugs ?? [];
    const addArtistIds = p.addArtistIds ?? [];
    const removeArtistIds = p.removeArtistIds ?? [];

    // Current membership gives full objects (id + name) for the removed entities.
    const tagBySlug = new Map((current?.tags ?? []).map(t => [t.slug, t]));
    const artistById = new Map((current?.artists ?? []).map(a => [a.id, a]));

    // Added entities aren't on the target yet — resolve their names from the API.
    const [addedTags, setAddedTags] = useState<Map<string, Tag>>(new Map());
    const [addedArtists, setAddedArtists] = useState<Map<number, Artist>>(new Map());

    const tagKey = addTagSlugs.join(',');
    const artistKey = addArtistIds.join(',');
    useEffect(() => {
        let alive = true;
        (async () => {
            const [tags, artists] = await Promise.all([
                resolveTagsBySlugs(addTagSlugs),
                resolveArtistsByIds(addArtistIds),
            ]);
            if (!alive) return;
            setAddedTags(new Map(tags.map(t => [t.slug, t])));
            setAddedArtists(new Map(artists.map(a => [a.id, a])));
        })();
        return () => { alive = false; };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [tagKey, artistKey]);

    const hasTagChanges = addTagSlugs.length || removeTagSlugs.length;
    const hasArtistChanges = addArtistIds.length || removeArtistIds.length;

    return (
        <div className="space-y-2">
            {p.source !== undefined && <ScalarDiff label="Source" oldValue={current?.source} newValue={p.source} />}
            {p.isNsfw !== undefined && (
                <ScalarDiff label="NSFW" oldValue={current ? String(current.isNsfw) : undefined} newValue={String(p.isNsfw)} />
            )}
            {hasTagChanges ? (
                <div>
                    <FieldLabel>Tags</FieldLabel>
                    <div className="flex flex-wrap gap-1.5 mt-1">
                        {addTagSlugs.map(slug => {
                            const t = addedTags.get(slug);
                            return <TagChip key={`at-${slug}`} tone="add" href={t ? `/tags?includedIds=${t.id}` : undefined} label={t?.name ?? slug} />;
                        })}
                        {removeTagSlugs.map(slug => {
                            const t = tagBySlug.get(slug);
                            return <TagChip key={`rt-${slug}`} tone="remove" href={t ? `/tags?includedIds=${t.id}` : undefined} label={t?.name ?? slug} />;
                        })}
                    </div>
                </div>
            ) : null}
            {hasArtistChanges ? (
                <div>
                    <FieldLabel>Artists</FieldLabel>
                    <div className="flex flex-wrap gap-1.5 mt-1">
                        {addArtistIds.map(id => {
                            const a = addedArtists.get(id) ?? artistById.get(id);
                            return <TagChip key={`aa-${id}`} tone="add" href={`/artists?includedIds=${id}`} label={a?.name ?? `#${id}`} />;
                        })}
                        {removeArtistIds.map(id => {
                            const a = artistById.get(id);
                            return <TagChip key={`ra-${id}`} tone="remove" href={`/artists?includedIds=${id}`} label={a?.name ?? `#${id}`} />;
                        })}
                    </div>
                </div>
            ) : null}
        </div>
    );
};

// ---- shared card chrome --------------------------------------------------

interface CardActions {
    isModerator: boolean;
    isOwn: boolean;
    canEdit: boolean;
    onApprove: () => void;
    onReject: () => void;
    onEdit: () => void;
    onCancel: () => void;
}

// Badges, target link, submitter / reviewer attribution and reason — common to every card type.
// Moderators see (and can open) the submitter; on a user's own "My Submissions" the submitter is
// themselves, so it's omitted and reviewers are shown as plain names (they can't open /users).
const TaskMeta = ({ task, isModerator }: { task: ReviewTask; isModerator: boolean }) => (
    <div className="min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
            <KindBadge kind={task.kind} />
            <StatusBadge status={task.status} />
            <ModEditedCue at={task.moderatorEditedAt} />
        </div>
        <Link
            to={targetLink(task)}
            className="mt-2 inline-flex items-center gap-1.5 text-sm font-medium hover:text-primary hover:underline"
        >
            {targetIcon(task.targetType)}
            {TARGET_LABELS[task.targetType]} #{task.targetId}
            <ExternalLink size={12} className="text-muted-foreground" />
        </Link>
        <p className="text-xs text-muted-foreground mt-1">
            {isModerator ? (
                <>By: <UserLink user={task.submitter} id={task.submitterId} />{' · '}</>
            ) : 'Submitted '}
            {new Date(task.createdAt).toLocaleDateString()}
        </p>
        {task.reviewer && (
            <p className="text-xs text-muted-foreground">
                Reviewed by: <UserLink user={task.reviewer} id={task.reviewerId} plain={!isModerator} />
            </p>
        )}
        {task.reviewerNote && (
            <p className="text-xs text-muted-foreground mt-1 italic break-words">“{task.reviewerNote}”</p>
        )}
    </div>
);

const TaskFooter = ({ task, isModerator, isOwn, canEdit, onApprove, onReject, onEdit, onCancel }: { task: ReviewTask } & CardActions) => {
    if (task.status !== ReviewTaskStatus.Pending) return null;
    const isNew = task.kind === ReviewTaskKind.NewContent;
    const showEdit = canEdit && (isModerator || isOwn);

    return (
        <div className="p-3 border-t border-border flex gap-2 flex-wrap">
            {isModerator && (
                <>
                    <button
                        onClick={onApprove}
                        className="flex-1 min-w-[88px] py-2 bg-secondary hover:bg-green-500/10 hover:text-green-600 rounded-lg font-bold text-sm transition-colors flex items-center justify-center gap-2"
                    >
                        <Check size={16} /> Approve
                    </button>
                    <button
                        onClick={onReject}
                        className="flex-1 min-w-[88px] py-2 bg-secondary hover:bg-red-500/10 hover:text-red-600 rounded-lg font-bold text-sm transition-colors flex items-center justify-center gap-2"
                    >
                        <X size={16} /> Reject
                    </button>
                </>
            )}
            {showEdit && (
                <button
                    onClick={onEdit}
                    className="flex-1 min-w-[88px] py-2 bg-secondary hover:bg-primary/10 hover:text-primary rounded-lg font-bold text-sm transition-colors flex items-center justify-center gap-2"
                    title={isNew ? 'Edit the submitted content' : 'Refine the proposed change'}
                >
                    <Edit2 size={16} /> Edit
                </button>
            )}
            {!isModerator && isOwn && (
                <button
                    onClick={onCancel}
                    className="flex-1 min-w-[88px] py-2 bg-secondary hover:bg-red-500/10 hover:text-red-600 rounded-lg font-bold text-sm transition-colors flex items-center justify-center gap-2"
                >
                    <Ban size={16} /> {isNew ? 'Withdraw' : 'Cancel'}
                </button>
            )}
        </div>
    );
};

// Base card: optional banner (image-forward), the common meta header, a body slot and the footer.
const TaskCard = ({ task, actions, banner, children }: { task: ReviewTask; actions: CardActions; banner?: React.ReactNode; children: React.ReactNode }) => (
    <div className="bg-card border border-border rounded-xl overflow-hidden flex flex-col shadow-sm">
        {banner}
        <div className="p-4 border-b border-border">
            <TaskMeta task={task} isModerator={actions.isModerator} />
        </div>
        <div className="p-4 space-y-3 flex-1">
            {task.reason && (
                <div className="text-sm">
                    <FieldLabel>Reason</FieldLabel>
                    <p className="mt-0.5 text-foreground whitespace-pre-wrap break-words">{task.reason}</p>
                </div>
            )}
            <div className="rounded-lg bg-secondary/40 border border-border p-3">
                {children}
            </div>
        </div>
        <TaskFooter task={task} {...actions} />
    </div>
);

// ---- per-type cards ------------------------------------------------------

const ImageTaskCard = ({ task, actions }: { task: ReviewTask; actions: CardActions }) => {
    const image = task.target?.image;
    const isNew = task.kind === ReviewTaskKind.NewContent;

    const banner = image ? (
        <Link to={`/images/${task.targetId}`} className="block bg-muted">
            <div className="w-full aspect-[16/10] overflow-hidden" style={{ backgroundColor: image.dominantColor || undefined }}>
                <img
                    src={imgCard(image.url)}
                    alt={`Image ${task.targetId}`}
                    className={`w-full h-full object-cover ${image.isNsfw ? 'blur-xl hover:blur-none transition-[filter] duration-300 transform-gpu' : ''}`}
                    loading="lazy"
                />
            </div>
        </Link>
    ) : null;

    return (
        <TaskCard task={task} actions={actions} banner={banner}>
            {!image ? (
                <Unavailable label="image" />
            ) : isNew ? (
                <div className="space-y-3">
                    <div className="flex flex-wrap gap-1.5 items-center">
                        {image.isNsfw && <span className="px-2 py-0.5 rounded text-xs font-bold bg-destructive/10 text-destructive">NSFW</span>}
                        {image.tags?.map(t => <TagChip key={`t-${t.id}`} href={`/tags?includedIds=${t.id}`} icon={<TagIcon size={11} />} label={t.name} />)}
                        {image.artists?.map(a => <TagChip key={`a-${a.id}`} href={`/artists?includedIds=${a.id}`} icon={<Palette size={11} />} label={a.name} />)}
                        {!image.isNsfw && !image.tags?.length && !image.artists?.length && (
                            <span className="text-xs text-muted-foreground italic">No tags or artists</span>
                        )}
                    </div>
                    {image.source && (
                        <div>
                            <FieldLabel>Source</FieldLabel>
                            <a href={image.source} target="_blank" rel="noopener noreferrer" className="block text-sm text-primary hover:underline break-all mt-0.5">
                                {image.source}
                            </a>
                        </div>
                    )}
                </div>
            ) : (
                <EditDiff task={task} />
            )}
        </TaskCard>
    );
};

const TagTaskCard = ({ task, actions }: { task: ReviewTask; actions: CardActions }) => {
    const tag = task.target?.tag;
    const isNew = task.kind === ReviewTaskKind.NewContent;

    return (
        <TaskCard task={task} actions={actions}>
            {!tag ? (
                <Unavailable label="tag" />
            ) : isNew ? (
                <div className="space-y-2">
                    <div className="flex items-center gap-2 flex-wrap">
                        <h3 className="font-bold text-base text-foreground">{tag.name}</h3>
                        <code className="text-xs text-primary/80 bg-primary/5 px-1.5 py-0.5 rounded select-text">{tag.slug}</code>
                    </div>
                    <p className="text-sm text-muted-foreground whitespace-pre-wrap break-words">{tag.description || 'No description'}</p>
                </div>
            ) : (
                <EditDiff task={task} />
            )}
        </TaskCard>
    );
};

const ArtistTaskCard = ({ task, actions }: { task: ReviewTask; actions: CardActions }) => {
    const artist = task.target?.artist;
    const isNew = task.kind === ReviewTaskKind.NewContent;
    const hasSocials = artist && (artist.twitter || artist.pixiv || artist.deviantArt || artist.patreon);

    return (
        <TaskCard task={task} actions={actions}>
            {!artist ? (
                <Unavailable label="artist" />
            ) : isNew ? (
                <div className="space-y-2">
                    <h3 className="font-bold text-base text-foreground">{artist.name}</h3>
                    {hasSocials ? (
                        <div className="flex flex-col gap-1.5">
                            {artist.twitter && <SocialRow url={artist.twitter} />}
                            {artist.pixiv && <SocialRow url={artist.pixiv} />}
                            {artist.deviantArt && <SocialRow url={artist.deviantArt} />}
                            {artist.patreon && <SocialRow url={artist.patreon} />}
                        </div>
                    ) : (
                        <span className="text-xs text-muted-foreground italic">No socials</span>
                    )}
                </div>
            ) : (
                <EditDiff task={task} />
            )}
        </TaskCard>
    );
};

// ---- modal initial-data builders -----------------------------------------
// For NewContent we pre-fill the live entity; for Edit we overlay the proposed scalar values so the
// submitter can refine their suggestion (membership stays current and is re-applied on submit).

const imageInitial = (task: ReviewTask): ImageDto | undefined => {
    const image = task.target?.image;
    if (!image) return undefined;
    if (task.kind === ReviewTaskKind.NewContent) return image;
    // Edit: overlay the proposed scalars and reconstruct the proposed tag/artist membership from the
    // deltas, so refining the suggestion doesn't silently drop its membership changes. Added items lack
    // friendly names here (we only have slugs/ids), but only their slug/id is used on submit.
    const p = (task.payload ?? {}) as ImageEditPayload;
    const removeTagSlugs = new Set(p.removeTagSlugs ?? []);
    const removeArtistIds = new Set(p.removeArtistIds ?? []);
    const tags = [
        ...(image.tags ?? []).filter(t => !removeTagSlugs.has(t.slug)),
        ...(p.addTagSlugs ?? []).map((slug, i) => ({ id: -1 - i, name: slug, slug, description: '' })),
    ];
    const artists = [
        ...(image.artists ?? []).filter(a => !removeArtistIds.has(a.id)),
        ...(p.addArtistIds ?? []).map(id => ({ id, name: `#${id}` })),
    ];
    return { ...image, source: p.source ?? image.source, isNsfw: p.isNsfw ?? image.isNsfw, tags, artists };
};

const tagInitial = (task: ReviewTask): (Partial<TagFormData> & { id?: number }) | undefined => {
    const tag = task.target?.tag;
    if (!tag) return undefined;
    if (task.kind === ReviewTaskKind.NewContent) return { id: tag.id, name: tag.name, slug: tag.slug, description: tag.description };
    const p = (task.payload ?? {}) as TagEditPayload;
    return { id: tag.id, name: p.name ?? tag.name, slug: tag.slug, description: p.description ?? tag.description };
};

const artistInitial = (task: ReviewTask): (Partial<ArtistFormData> & { id?: number }) | undefined => {
    const artist = task.target?.artist;
    if (!artist) return undefined;
    if (task.kind === ReviewTaskKind.NewContent) {
        return { id: artist.id, name: artist.name, twitter: artist.twitter, pixiv: artist.pixiv, patreon: artist.patreon, deviantArt: artist.deviantArt };
    }
    const p = (task.payload ?? {}) as ArtistEditPayload;
    return {
        id: artist.id,
        name: p.name ?? artist.name,
        twitter: p.twitter ?? artist.twitter,
        pixiv: p.pixiv ?? artist.pixiv,
        patreon: p.patreon ?? artist.patreon,
        deviantArt: p.deviantArt ?? artist.deviantArt,
    };
};

// Title for the content-edit modal — distinguishes editing a new submission from refining a suggestion.
const editTitle = (task: ReviewTask) =>
    `${task.kind === ReviewTaskKind.NewContent ? 'Edit submission' : 'Refine suggestion'} · ${TARGET_LABELS[task.targetType]} #${task.targetId}`;

// ---- page ----------------------------------------------------------------

const Review = () => {
    const user = useRequireAuth();
    const { user: authUser } = useAuth();
    const { showNotification } = useNotification();

    const isModerator = !!authUser && (authUser.role === Role.Moderator || authUser.role === Role.Admin);

    const [tasks, setTasks] = useState<ReviewTask[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const [counts, setCounts] = useState<Record<ReviewableContentType, number>>({
        [ReviewableContentType.Image]: 0,
        [ReviewableContentType.Tag]: 0,
        [ReviewableContentType.Artist]: 0,
    });

    // Primary navigation is the target type; status/kind are secondary filters.
    // The initial tab can be set via ?type= (e.g. redirect here after an upload).
    const [searchParams] = useSearchParams();
    const initialType = searchParams.get('type') === ReviewableContentType.Tag
        ? ReviewableContentType.Tag
        : searchParams.get('type') === ReviewableContentType.Artist
            ? ReviewableContentType.Artist
            : ReviewableContentType.Image;
    const [typeTab, setTypeTab] = useState<ReviewableContentType>(initialType);
    const [statusFilter, setStatusFilter] = useState<ReviewTaskStatus>(ReviewTaskStatus.Pending);
    const [kindFilter, setKindFilter] = useState<ReviewTaskKind | 'All'>('All');

    // Reject / cancel / edit modals
    const [rejectTarget, setRejectTarget] = useState<ReviewTask | null>(null);
    const [cancelTarget, setCancelTarget] = useState<ReviewTask | null>(null);
    const [editingTask, setEditingTask] = useState<ReviewTask | null>(null);
    const [editingImageData, setEditingImageData] = useState<ImageDto | undefined>(undefined);
    const queryClient = useQueryClient();

    // Open the content-edit modal. For an image Edit, the payload only carries slugs/ids for proposed additions,
    // so resolve their real names first — otherwise the modal shows raw lowercase slugs / "#id" instead of names.
    const openEditTask = async (task: ReviewTask) => {
        if (task.targetType === ReviewableContentType.Image) {
            const base = imageInitial(task);
            if (base && task.kind === ReviewTaskKind.Edit) {
                const p = (task.payload ?? {}) as ImageEditPayload;
                const [addedTags, addedArtists] = await Promise.all([
                    resolveTagsBySlugs(p.addTagSlugs ?? []),
                    resolveArtistsByIds(p.addArtistIds ?? []),
                ]);
                const bySlug = new Map(addedTags.map(t => [t.slug, t]));
                const byId = new Map(addedArtists.map(a => [a.id, a]));
                setEditingImageData({
                    ...base,
                    tags: base.tags.map(t => (t.id < 0 ? bySlug.get(t.slug) ?? t : t)),
                    artists: base.artists.map(a => byId.get(a.id) ?? a),
                });
            } else {
                setEditingImageData(base);
            }
        }
        setEditingTask(task);
    };

    const fetchTasks = async () => {
        if (!user) return;
        setLoading(true);
        try {
            const params: Record<string, string | number> = { page, pageSize: PAGE_SIZE, status: statusFilter, targetType: typeTab };
            if (kindFilter !== 'All') params.kind = kindFilter;
            const { data } = await api.get<PaginatedList<ReviewTask>>('/review/tasks', { params });
            setTasks(data.items);
            setTotalPages(data.totalPages);
        } catch {
            // Handled globally
        } finally {
            setLoading(false);
        }
    };

    // Per-type pending counts for the tab badges (respecting the kind filter, independent of the status view).
    const fetchCounts = async () => {
        if (!user) return;
        try {
            const results = await Promise.all(TYPE_TABS.map(t => {
                const params: Record<string, string | number> = { status: ReviewTaskStatus.Pending, targetType: t, page: 1, pageSize: 1 };
                if (kindFilter !== 'All') params.kind = kindFilter;
                return api.get<PaginatedList<ReviewTask>>('/review/tasks', { params, skipGlobalErrorHandler: true })
                    .then(r => r.data.totalCount)
                    .catch(() => 0);
            }));
            setCounts({
                [ReviewableContentType.Image]: results[0],
                [ReviewableContentType.Tag]: results[1],
                [ReviewableContentType.Artist]: results[2],
            });
        } catch {
            // Best-effort; badges just stay at their previous values.
        }
    };

    useEffect(() => {
        setPage(1);
    }, [statusFilter, kindFilter, typeTab]);

    useEffect(() => {
        if (user) fetchTasks();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [page, user, statusFilter, kindFilter, typeTab]);

    useEffect(() => {
        if (user) fetchCounts();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [user, kindFilter]);

    const handleApprove = async (task: ReviewTask) => {
        try {
            await api.patch(`/review/tasks/${task.id}/approve`);
            showNotification('success', task.kind === ReviewTaskKind.NewContent ? 'Content published' : 'Edit applied');
            setTasks(prev => prev.filter(t => t.id !== task.id));
            fetchCounts();
            invalidateNavBadges(queryClient);
        } catch {
            // Handled globally (e.g. 409 conflict when an edit drifted)
        }
    };

    const handleReject = async (reason: string) => {
        if (!rejectTarget) return;
        try {
            await api.patch(`/review/tasks/${rejectTarget.id}/reject`, { reason: reason || undefined });
            showNotification('success', rejectTarget.kind === ReviewTaskKind.NewContent ? 'Submission rejected' : 'Edit declined');
            setTasks(prev => prev.filter(t => t.id !== rejectTarget.id));
            setRejectTarget(null);
            fetchCounts();
            invalidateNavBadges(queryClient);
        } catch {
            // Handled globally
        }
    };

    const handleCancel = async () => {
        if (!cancelTarget) return;
        try {
            await api.patch(`/review/tasks/${cancelTarget.id}/cancel`);
            showNotification('success', 'Task cancelled');
            setTasks(prev => prev.filter(t => t.id !== cancelTarget.id));
            setCancelTarget(null);
            fetchCounts();
            invalidateNavBadges(queryClient);
        } catch {
            // Handled globally
        }
    };

    // Content-only edit: PATCH the task. NewContent sends the full content; Edit sends the recomputed delta.
    const patchTaskContent = async (task: ReviewTask, content: Record<string, unknown>) => {
        await api.patch(`/review/tasks/${task.id}`, { content });
        showNotification('success', task.kind === ReviewTaskKind.NewContent ? 'Submission updated' : 'Suggestion updated');
        setEditingTask(null);
        fetchTasks();
        fetchCounts();
    };

    const submitImageEdit = async (task: ReviewTask, data: ImageFormData) => {
        const current = task.target?.image;
        if (!current) return;
        if (task.kind === ReviewTaskKind.NewContent) {
            await patchTaskContent(task, { source: data.source ?? '', isNsfw: data.isNsfw, tags: data.tags, artists: data.artists });
            return;
        }
        const { payload, isEmpty } = buildImagePayload(current, {
            source: data.source,
            isNsfw: data.isNsfw,
            tagSlugs: data.tags,
            artistIds: data.artists,
        });
        if (isEmpty) { showNotification('warning', 'No changes to save'); return; }
        await patchTaskContent(task, payload as unknown as Record<string, unknown>);
    };

    const submitTagEdit = async (task: ReviewTask, data: TagFormData) => {
        const current = task.target?.tag;
        if (!current) return;
        if (task.kind === ReviewTaskKind.NewContent) {
            await patchTaskContent(task, { name: data.name, description: data.description });
            return;
        }
        const { payload, isEmpty } = buildTagPayload(current, { name: data.name, description: data.description });
        if (isEmpty) { showNotification('warning', 'No changes to save'); return; }
        await patchTaskContent(task, payload as unknown as Record<string, unknown>);
    };

    const submitArtistEdit = async (task: ReviewTask, data: ArtistFormData) => {
        const current = task.target?.artist;
        if (!current) return;
        if (task.kind === ReviewTaskKind.NewContent) {
            await patchTaskContent(task, {
                name: data.name,
                patreon: data.patreon ?? '',
                pixiv: data.pixiv ?? '',
                twitter: data.twitter ?? '',
                deviantArt: data.deviantArt ?? '',
            });
            return;
        }
        const { payload, isEmpty } = buildArtistPayload(current, {
            name: data.name,
            patreon: data.patreon,
            pixiv: data.pixiv,
            twitter: data.twitter,
            deviantArt: data.deviantArt,
        });
        if (isEmpty) { showNotification('warning', 'No changes to save'); return; }
        await patchTaskContent(task, payload as unknown as Record<string, unknown>);
    };

    if (!user) return null;

    const kindLabel = (k: ReviewTaskKind | 'All') =>
        k === 'All' ? 'All' : k === ReviewTaskKind.NewContent ? 'New content' : 'Edits';

    const emptyTitle = isModerator ? 'All caught up!' : 'Nothing here yet';
    const emptySubtitle = isModerator
        ? `No ${statusFilter.toLowerCase()} ${TARGET_LABELS[typeTab].toLowerCase()} tasks.`
        : 'Your uploads and suggestions will appear here.';

    return (
        <div className="container mx-auto p-6 md:p-10">
            <div className="flex flex-col md:flex-row justify-between items-start md:items-center mb-8 gap-4">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground">
                        <GitPullRequest className="text-primary" size={32} />
                        {isModerator ? 'Review queue' : 'My Submissions'}
                    </h1>
                    <p className="text-muted-foreground mt-1">
                        {isModerator
                            ? 'Review new submissions and proposed edits from the community.'
                            : 'Track your uploads and suggested edits.'}
                    </p>
                </div>
            </div>

            {/* Type, status and kind selectors — all on one level (full-width on mobile). */}
            <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3 mb-6">
                <div className="flex bg-secondary p-1 rounded-xl">
                    {TYPE_TABS.map(t => (
                        <button
                            key={t}
                            onClick={() => setTypeTab(t)}
                            className={`flex-1 sm:flex-initial px-4 py-2 rounded-lg text-sm font-bold transition-all flex items-center justify-center gap-2 ${typeTab === t ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                        >
                            {TARGET_LABELS[t]}s
                            {counts[t] > 0 && (
                                <span className={`px-1.5 py-0.5 rounded-full text-[11px] font-bold leading-none ${typeTab === t ? 'bg-primary text-primary-foreground' : 'bg-primary/15 text-primary'}`}>
                                    {counts[t] > 99 ? '99+' : counts[t]}
                                </span>
                            )}
                        </button>
                    ))}
                </div>
                <div className="flex bg-secondary p-1 rounded-xl">
                    {[ReviewTaskStatus.Pending, ReviewTaskStatus.Approved, ReviewTaskStatus.Rejected].map(s => (
                        <button
                            key={s}
                            onClick={() => setStatusFilter(s)}
                            className={`flex-1 sm:flex-initial px-4 py-2 rounded-lg text-sm font-bold transition-all ${statusFilter === s ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                        >
                            {s}
                        </button>
                    ))}
                </div>
                <div className="flex bg-secondary p-1 rounded-xl">
                    {(['All', ReviewTaskKind.NewContent, ReviewTaskKind.Edit] as const).map(k => (
                        <button
                            key={k}
                            onClick={() => setKindFilter(k)}
                            className={`flex-1 sm:flex-initial px-4 py-2 rounded-lg text-sm font-bold transition-all ${kindFilter === k ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                        >
                            {kindLabel(k)}
                        </button>
                    ))}
                </div>
            </div>

            {loading ? (
                <div className="py-20 text-center text-muted-foreground">Loading tasks...</div>
            ) : tasks.length === 0 ? (
                <div className="flex flex-col items-center justify-center h-64 text-muted-foreground border-2 border-dashed border-border rounded-xl bg-card/30">
                    <Check size={48} className="mb-4 opacity-20" />
                    <p className="text-lg font-medium">{emptyTitle}</p>
                    <p className="text-sm">{emptySubtitle}</p>
                </div>
            ) : (
                <div>
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                        {tasks.map(task => {
                            const isOwn = authUser?.id === task.submitterId;
                            const canEdit = !!task.target && (
                                (task.targetType === ReviewableContentType.Image && !!task.target.image) ||
                                (task.targetType === ReviewableContentType.Tag && !!task.target.tag) ||
                                (task.targetType === ReviewableContentType.Artist && !!task.target.artist)
                            );
                            const actions: CardActions = {
                                isModerator,
                                isOwn,
                                canEdit,
                                onApprove: () => handleApprove(task),
                                onReject: () => setRejectTarget(task),
                                onEdit: () => openEditTask(task),
                                onCancel: () => setCancelTarget(task),
                            };
                            switch (task.targetType) {
                                case ReviewableContentType.Image: return <ImageTaskCard key={task.id} task={task} actions={actions} />;
                                case ReviewableContentType.Tag: return <TagTaskCard key={task.id} task={task} actions={actions} />;
                                case ReviewableContentType.Artist: return <ArtistTaskCard key={task.id} task={task} actions={actions} />;
                            }
                        })}
                    </div>

                    {!loading && (
                        <Pagination currentPage={page} totalPages={totalPages} setPage={setPage} />
                    )}
                </div>
            )}

            <ReasonModal
                isOpen={!!rejectTarget}
                onClose={() => setRejectTarget(null)}
                onSubmit={handleReject}
                title={rejectTarget?.kind === ReviewTaskKind.NewContent ? 'Reject Submission' : 'Reject Edit'}
                description={
                    rejectTarget?.kind === ReviewTaskKind.NewContent
                        ? 'Rejecting deletes the submitted content. Optionally provide a reason for the submitter.'
                        : 'Optionally provide a reason. This will be visible to the submitter.'
                }
                placeholder="Reason for rejection..."
                submitText="Reject"
                required={false}
            />

            <ConfirmModal
                isOpen={!!cancelTarget}
                onClose={() => setCancelTarget(null)}
                onConfirm={handleCancel}
                title={cancelTarget?.kind === ReviewTaskKind.NewContent ? 'Withdraw Submission' : 'Cancel Suggestion'}
                message={
                    cancelTarget?.kind === ReviewTaskKind.NewContent
                        ? 'Are you sure you want to withdraw this submission? Your upload will be deleted.'
                        : 'Are you sure you want to cancel this suggestion?'
                }
                confirmText={cancelTarget?.kind === ReviewTaskKind.NewContent ? 'Withdraw' : 'Cancel Suggestion'}
                cancelText="Keep"
                variant="warning"
            />

            {/* Content-only edit (submitter or moderator). PATCHes the task; never touches review status. */}
            {editingTask?.targetType === ReviewableContentType.Image && (
                <ImageModal
                    isOpen={true}
                    onClose={() => setEditingTask(null)}
                    contentOnly
                    title={editTitle(editingTask)}
                    initialData={editingImageData}
                    onSubmit={(data) => submitImageEdit(editingTask, data)}
                />
            )}
            {editingTask?.targetType === ReviewableContentType.Tag && (
                <TagModal
                    isOpen={true}
                    onClose={() => setEditingTask(null)}
                    contentOnly
                    title={editTitle(editingTask)}
                    initialData={tagInitial(editingTask)}
                    onSubmit={(data) => submitTagEdit(editingTask, data)}
                />
            )}
            {editingTask?.targetType === ReviewableContentType.Artist && (
                <ArtistModal
                    isOpen={true}
                    onClose={() => setEditingTask(null)}
                    contentOnly
                    title={editTitle(editingTask)}
                    initialData={artistInitial(editingTask)}
                    onSubmit={(data) => submitArtistEdit(editingTask, data)}
                />
            )}
        </div>
    );
};

export default Review;

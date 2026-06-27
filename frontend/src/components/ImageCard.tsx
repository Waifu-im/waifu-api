import { ImageDto, Role, ReviewStatus } from "../types";
import { Heart, Trash2, Edit2, FolderMinus, FolderPlus, Clock, GitPullRequest } from "lucide-react";
import { Link } from "react-router-dom";
import TagChip from "./TagChip";
import { useAuth } from "../context/AuthContext";
import { useMyPendingImageEdits } from "../hooks/useMyPendingImageEdits";
import { useAuthGuard } from "../hooks/useRequireAuth";
import { useState, useEffect } from "react";
import api from "../services/api";
import AlbumSelectionModal from "./modals/AlbumSelectionModal";
import Skeleton from "./Skeleton";
import { imgCard } from "../utils/cfImage";

interface ImageCardProps {
    image: ImageDto;
    onDelete?: (id: number) => void;
    onRemove?: (id: number) => void;
    onEdit?: (image: ImageDto) => void;
    /** Lets a logged-in non-moderator propose an edit (shown instead of the moderator edit button). */
    onSuggest?: (image: ImageDto) => void;
    forceOverlay?: boolean;
    readOnly?: boolean;
    /** When true, show a tag strip below the image: the image's accepted tags + the user's own pending tags. */
    showTags?: boolean;
}

const ImageCard = ({ image, onDelete, onRemove, onEdit, onSuggest, forceOverlay = false, readOnly = false, showTags = false }: ImageCardProps) => {
    const { user } = useAuth();
    const checkAuth = useAuthGuard();
    const [isLiked, setIsLiked] = useState(!!image.likedAt);
    const [likesCount, setLikesCount] = useState(image.favorites);
    const [isLikeLoading, setIsLikeLoading] = useState(false);

    // État pour gérer le chargement visuel de l'image
    const [isImageLoaded, setIsImageLoaded] = useState(false);

    const [localAlbums, setLocalAlbums] = useState(image.albums || []);
    const [isAlbumModalOpen, setIsAlbumModalOpen] = useState(false);

    const isAdmin = user && user.role === Role.Admin;

    // The user's own not-yet-approved tag proposals for this image (add/remove), fetched once via the review API.
    const { data: pendingEdits } = useMyPendingImageEdits(showTags);
    const edit = pendingEdits?.get(image.id);
    const removedTagIds = new Set((edit?.removeTags ?? []).map(t => t.id));
    const addedTags = edit?.addTags ?? [];

    // The strip shows the image's accepted tags (capped + linkable) plus the user's own pending tags linked to it.
    // Their not-yet-reviewed edit proposal contributes additions (addedTags, not linked yet) and marks removals.
    const allTags = image.tags ?? [];
    const removedTags = allTags.filter(t => removedTagIds.has(t.id));
    const myPendingTags = allTags.filter(t => t.reviewStatus === ReviewStatus.Pending && !removedTagIds.has(t.id));
    const acceptedTags = allTags.filter(t => t.reviewStatus !== ReviewStatus.Pending && !removedTagIds.has(t.id));
    const TAG_CAP = 6;
    const shownTags = acceptedTags.slice(0, TAG_CAP);
    const hiddenTagCount = acceptedTags.length - shownTags.length;
    const hasAnyTag = acceptedTags.length || myPendingTags.length || removedTags.length || addedTags.length;

    useEffect(() => {
        setIsLiked(!!image.likedAt);
        setLikesCount(image.favorites);
        setLocalAlbums(image.albums || []);
    }, [image.likedAt, image.favorites, image.albums]);

    const toggleLike = async (e: React.MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        if (!checkAuth('You must be logged in to like images.', `/images/${image.id}`)) return;
        if (isLikeLoading) return;

        setIsLikeLoading(true);
        const prevLiked = isLiked;
        setIsLiked(!prevLiked);
        setLikesCount(prev => prevLiked ? prev - 1 : prev + 1);

        try {
            if (prevLiked) {
                await api.delete(`/users/me/albums/favorites/images/${image.id}`);
                setLocalAlbums(prev => prev.filter(a => !a.isDefault));
            } else {
                await api.post(`/users/me/albums/favorites/images/${image.id}`);
            }
        } catch (error) {
            setIsLiked(prevLiked);
            setLikesCount(prev => prevLiked ? prev : prev - 1);
        } finally {
            setIsLikeLoading(false);
        }
    };

    const handleAlbumUpdate = (updatedImage: ImageDto) => {
        setIsLiked(!!updatedImage.likedAt);
        setLikesCount(updatedImage.favorites);
        setLocalAlbums(updatedImage.albums || []);

        image.albums = updatedImage.albums;
        image.likedAt = updatedImage.likedAt;
        image.favorites = updatedImage.favorites;
    };

    return (
        <div className="relative group rounded-xl bg-card border border-border shadow-sm hover:shadow-md transition-shadow overflow-hidden flex flex-col">
            <div className="relative">
            <Link to={`/images/${image.id}`} className="block w-full">
                {/* Conteneur intelligent :
                   Utilise aspect-ratio pour réserver la place exacte de l'image
                   avant qu'elle ne soit téléchargée.
                */}
                <div
                    className="relative w-full bg-secondary/30 overflow-hidden"
                    style={{ aspectRatio: `${image.width} / ${image.height}` }}
                >
                    {/* Le Skeleton reste visible tant que l'image n'est pas chargée */}
                    {!isImageLoaded && (
                        <Skeleton className="absolute inset-0 z-10 w-full h-full rounded-none" />
                    )}

                    <img
                        src={imgCard(image.url)}
                        alt={`Img ${image.id}`}
                        loading="lazy"
                        onLoad={() => setIsImageLoaded(true)}
                        className={`w-full h-full object-cover transition-[opacity,transform] duration-700 xl:group-hover:scale-105 ${
                            isImageLoaded ? 'opacity-100' : 'opacity-0'
                        }`}
                    />
                </div>
            </Link>

            {/* Action buttons overlay. On xl+ revealed on hover. On smaller screens hidden unless forceOverlay is true. */}
            <div className={`absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent flex-col justify-between p-3 pointer-events-none transition-opacity duration-300 xl:flex xl:opacity-0 xl:group-hover:opacity-100 ${forceOverlay ? 'flex' : 'hidden'}`}>
                <div className="flex justify-end gap-2 pointer-events-auto">
                    {!readOnly && user && (
                        <>
                            <button
                                className="p-2 bg-primary/90 text-primary-foreground rounded-full shadow-sm hover:bg-primary transition-transform hover:scale-110"
                                title="Add to Album"
                                onClick={(e) => { e.preventDefault(); e.stopPropagation(); setIsAlbumModalOpen(true); }}
                            >
                                <FolderPlus size={14} />
                            </button>
                            <AlbumSelectionModal
                                isOpen={isAlbumModalOpen}
                                onClose={() => setIsAlbumModalOpen(false)}
                                image={{...image, albums: localAlbums}}
                                onUpdate={handleAlbumUpdate}
                            />
                        </>
                    )}

                    {!readOnly && onRemove && (
                        <button
                            onClick={(e) => { e.preventDefault(); e.stopPropagation(); onRemove(image.id); }}
                            className="p-2 bg-secondary/90 text-secondary-foreground rounded-full shadow-sm hover:bg-secondary transition-transform hover:scale-110"
                            title="Remove from Album"
                        >
                            <FolderMinus size={14} />
                        </button>
                    )}

                    {onEdit && (
                        <button
                            onClick={(e) => { e.preventDefault(); e.stopPropagation(); onEdit(image); }}
                            className="p-2 bg-accent/90 text-accent-foreground rounded-full shadow-sm hover:bg-accent transition-transform hover:scale-110"
                            title="Edit Image"
                        >
                            <Edit2 size={14} />
                        </button>
                    )}

                    {onSuggest && (
                        <button
                            onClick={(e) => { e.preventDefault(); e.stopPropagation(); onSuggest(image); }}
                            className="p-2 bg-accent/90 text-accent-foreground rounded-full shadow-sm hover:bg-accent transition-transform hover:scale-110"
                            title="Suggest an edit"
                        >
                            <GitPullRequest size={14} />
                        </button>
                    )}

                    {onDelete && isAdmin && (
                        <button
                            onClick={(e) => { e.preventDefault(); e.stopPropagation(); onDelete(image.id); }}
                            className="p-2 bg-red-600/90 text-white rounded-full shadow-sm hover:bg-red-700 transition-transform hover:scale-110"
                            title="Delete"
                        >
                            <Trash2 size={14} />
                        </button>
                    )}
                </div>
                <div></div>
            </div>

            {/* Footer (ID + Likes) - always visible on mobile (smaller gradient), hover on desktop (full gradient) */}
            <div className="absolute bottom-0 left-0 right-0 p-3 pointer-events-none
                bg-gradient-to-t from-black/50 to-transparent
                xl:from-black/80 xl:opacity-0 xl:group-hover:opacity-100 xl:transition-opacity xl:duration-300">
                <div className="flex items-center justify-between text-white">
                    <span className="text-xs font-mono opacity-75">#{image.id}</span>
                    {readOnly ? (
                        <div className="flex items-center gap-1.5 px-2 py-1 rounded-full bg-white/10 backdrop-blur-md">
                            <Heart size={14} className="text-white" />
                            <span className="text-xs font-bold">{likesCount}</span>
                        </div>
                    ) : (
                        <button
                            onClick={toggleLike}
                            className="flex items-center gap-1.5 px-2 py-1 rounded-full bg-white/10 hover:bg-white/20 backdrop-blur-md transition-colors pointer-events-auto"
                        >
                            <Heart
                                size={14}
                                className={`transition-colors ${isLiked ? "fill-rose-500 text-rose-500" : "text-white"}`}
                            />
                            <span className="text-xs font-bold">{likesCount}</span>
                        </button>
                    )}
                </div>
            </div>
            </div>

            {showTags && (
                <div className="px-2 py-2 flex flex-wrap gap-1 border-t border-border bg-card">
                    {shownTags.map(t => (
                        <TagChip key={`t-${t.id}`} label={t.name} href={`/gallery?includedTags=${encodeURIComponent(t.slug)}`} />
                    ))}
                    {hiddenTagCount > 0 && (
                        <TagChip label={`+${hiddenTagCount} more`} href={`/images/${image.id}`} title="See all tags" />
                    )}
                    {addedTags.map(t => (
                        <TagChip key={`add-${t.id}`} tone="add" label={t.name}
                            href={`/gallery?includedTags=${encodeURIComponent(t.slug)}`}
                            trailingIcon={t.reviewStatus === ReviewStatus.Pending ? <Clock size={11} className="shrink-0" /> : undefined}
                            title={t.reviewStatus === ReviewStatus.Pending
                                ? 'You proposed adding this tag (also awaiting review)'
                                : 'You proposed adding this tag (awaiting review)'} />
                    ))}
                    {removedTags.map(t => (
                        <TagChip key={`rm-${t.id}`} tone="remove" label={t.name} title="You proposed removing this tag (awaiting review)" />
                    ))}
                    {myPendingTags.map(t => (
                        <TagChip key={`p-${t.id}`} tone="pending" label={t.name} title="Your tag (awaiting review)" />
                    ))}
                    {!hasAnyTag && (
                        <span className="text-[11px] text-muted-foreground italic">No tags</span>
                    )}
                </div>
            )}
        </div>
    );
};

export default ImageCard;

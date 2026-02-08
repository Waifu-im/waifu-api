import { ImageDto, Role } from "../types";
import { Heart, Trash2, Edit2, FolderMinus, FolderPlus } from "lucide-react";
import { Link } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { useAuthGuard } from "../hooks/useRequireAuth";
import { useState, useEffect } from "react";
import api from "../services/api";
import AlbumSelectionModal from "./modals/AlbumSelectionModal";
import Skeleton from "./Skeleton";

interface ImageCardProps {
    image: ImageDto;
    onDelete?: (id: number) => void;
    onRemove?: (id: number) => void;
    onEdit?: (image: ImageDto) => void;
    forceOverlay?: boolean;
    readOnly?: boolean;
}

const ImageCard = ({ image, onDelete, onRemove, onEdit, forceOverlay = false, readOnly = false }: ImageCardProps) => {
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
        <div className="relative group rounded-xl bg-card border border-border shadow-sm hover:shadow-md transition-all overflow-hidden">
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
                        src={image.url}
                        alt={`Img ${image.id}`}
                        loading="lazy"
                        onLoad={() => setIsImageLoaded(true)}
                        className={`w-full h-full object-cover transition-all duration-700 xl:group-hover:scale-105 ${
                            isImageLoaded ? 'opacity-100' : 'opacity-0'
                        }`}
                    />
                </div>
            </Link>

            {/* Action buttons overlay (desktop only, hover to reveal) */}
            <div className={`absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent flex-col justify-between p-3 pointer-events-none transition-all duration-300 rounded-xl hidden xl:flex ${forceOverlay ? 'xl:opacity-100' : 'xl:opacity-0 xl:group-hover:opacity-100'}`}>
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
            <div className="absolute bottom-0 left-0 right-0 p-3 pointer-events-none rounded-b-xl
                bg-gradient-to-t from-black/50 to-transparent
                xl:from-black/80 xl:opacity-0 xl:group-hover:opacity-100 xl:transition-all xl:duration-300">
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
    );
};

export default ImageCard;

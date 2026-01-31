import { ImageDto, Role } from "../types";
import { Heart, Trash2, Edit2, FolderMinus, FolderPlus } from "lucide-react";
import { Link } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { useState, useEffect } from "react";
import api from "../services/api";
import AlbumSelectionModal from "./modals/AlbumSelectionModal";

interface ImageCardProps {
    image: ImageDto;
    onDelete?: (id: number) => void; // Suppression définitive (Admin)
    onRemove?: (id: number) => void; // Retrait de la collection/album courant
    onEdit?: (image: ImageDto) => void;
    forceOverlay?: boolean;
}

const ImageCard = ({ image, onDelete, onRemove, onEdit, forceOverlay = false }: ImageCardProps) => {
    const { user } = useAuth();
    const [isLiked, setIsLiked] = useState(!!image.likedAt);
    const [likesCount, setLikesCount] = useState(image.favorites);
    const [isLikeLoading, setIsLikeLoading] = useState(false);
    // Local state to track albums for immediate UI feedback in dropdown
    const [localAlbums, setLocalAlbums] = useState(image.albums || []);
    const [isAlbumModalOpen, setIsAlbumModalOpen] = useState(false);

    const isAdmin = user && user.role === Role.Admin;

    // Sync local state with prop changes (e.g. when returning from another page)
    useEffect(() => {
        setIsLiked(!!image.likedAt);
        setLikesCount(image.favorites);
        setLocalAlbums(image.albums || []);
    }, [image.likedAt, image.favorites, image.albums]);

    const toggleLike = async (e: React.MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        if (!user || isLikeLoading) return;

        setIsLikeLoading(true);
        const prevLiked = isLiked;
        setIsLiked(!prevLiked);
        setLikesCount(prev => prevLiked ? prev - 1 : prev + 1);

        try {
            if (prevLiked) {
                await api.delete(`/users/me/albums/favorites/images/${image.id}`);
                // Update local albums to remove favorites if it exists there
                setLocalAlbums(prev => prev.filter(a => !a.isDefault));
            } else {
                await api.post(`/users/me/albums/favorites/images/${image.id}`);
                // We can't easily add the full album object here without fetching, 
                // but AlbumDropdown handles its own state for the list.
                // However, for consistency, we might want to trigger a refresh if possible.
            }
        } catch (error) {
            setIsLiked(prevLiked);
            setLikesCount(prev => prevLiked ? prev : prev - 1);
            console.error("Like failed", error);
        } finally {
            setIsLikeLoading(false);
        }
    };

    // Callback to update local state when album changes via dropdown
    const handleAlbumUpdate = (updatedImage: ImageDto) => {
        setIsLiked(!!updatedImage.likedAt);
        setLikesCount(updatedImage.favorites);
        setLocalAlbums(updatedImage.albums || []);
        
        // Mutate the prop object so if the dropdown is opened again, it has fresh data
        // This is a bit of a hack but necessary since we don't have a global store for image list state
        image.albums = updatedImage.albums;
        image.likedAt = updatedImage.likedAt;
        image.favorites = updatedImage.favorites;
    };

    return (
        <div className="relative group rounded-xl bg-card border border-border shadow-sm hover:shadow-md transition-all">
            <Link to={`/images/${image.id}`} className="block w-full overflow-hidden rounded-xl">
                <img
                    src={image.url}
                    alt={`Img ${image.id}`}
                    loading="lazy"
                    className="w-full h-auto object-cover transition-transform duration-700 group-hover:scale-105"
                />
            </Link>

            <div className={`absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent flex flex-col justify-between p-3 pointer-events-none transition-all duration-300 rounded-xl ${forceOverlay ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}`}>

                {/* Actions en haut à droite */}
                <div className="flex justify-end gap-2 pointer-events-auto">
                    {/* Album Dropdown */}
                    {user && (
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

                    {/* Bouton Retirer de l'album (FolderMinus - Style neutre) */}
                    {onRemove && (
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

                    {/* Bouton Supprimer définitivement (Trash2 - Style destructif rouge) */}
                    {onDelete && isAdmin && (
                        <button
                            onClick={(e) => { e.preventDefault(); e.stopPropagation(); onDelete(image.id); }}
                            className="p-2 bg-red-600/90 text-white rounded-full shadow-sm hover:bg-red-700 transition-transform hover:scale-110"
                            title="Permanently Delete Image"
                        >
                            <Trash2 size={14} />
                        </button>
                    )}
                </div>

                <div className={`transform transition-transform duration-300 text-white ${forceOverlay ? 'translate-y-0' : 'translate-y-4 group-hover:translate-y-0'}`}>
                    <div className="flex items-center justify-between">
                        <div className="min-w-0">
                            <span className="text-xs font-mono opacity-75">#{image.id}</span>
                        </div>

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
                    </div>
                </div>
            </div>
        </div>
    );
};

export default ImageCard;
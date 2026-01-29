import { ImageDto } from "../types";
import { Heart, Trash2, Edit2, FolderMinus } from "lucide-react";
import { Link } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { useState } from "react";
import api from "../services/api";

interface ImageCardProps {
    image: ImageDto;
    onDelete?: (id: number) => void; // Suppression définitive (Admin)
    onRemove?: (id: number) => void; // Retrait de la collection/album courant
    onEdit?: (image: ImageDto) => void;
}

const ImageCard = ({ image, onDelete, onRemove, onEdit }: ImageCardProps) => {
    const { user } = useAuth();
    const [isLiked, setIsLiked] = useState(!!image.likedAt);
    const [likesCount, setLikesCount] = useState(image.favorites);
    const [isLikeLoading, setIsLikeLoading] = useState(false);

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
            } else {
                await api.post(`/users/me/albums/favorites/images/${image.id}`);
            }
        } catch (error) {
            setIsLiked(prevLiked);
            setLikesCount(prev => prevLiked ? prev : prev - 1);
            console.error("Like failed", error);
        } finally {
            setIsLikeLoading(false);
        }
    };

    return (
        <div className="relative group overflow-hidden rounded-xl bg-card border border-border shadow-sm hover:shadow-md transition-all">
            <Link to={`/images/${image.id}`} className="block w-full">
                <img
                    src={image.url}
                    alt={`Img ${image.id}`}
                    loading="lazy"
                    className="w-full h-auto object-cover transition-transform duration-700 group-hover:scale-105"
                />
            </Link>

            <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-all duration-300 flex flex-col justify-between p-3 pointer-events-none">

                {/* Actions en haut à droite */}
                <div className="flex justify-end gap-2 pointer-events-auto">
                    {onEdit && (
                        <button
                            onClick={(e) => { e.preventDefault(); e.stopPropagation(); onEdit(image); }}
                            className="p-2 bg-accent/90 text-accent-foreground rounded-full shadow-sm hover:bg-accent transition-transform hover:scale-110"
                            title="Edit Image"
                        >
                            <Edit2 size={14} />
                        </button>
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

                    {/* Bouton Supprimer définitivement (Trash2 - Style destructif rouge) */}
                    {onDelete && (
                        <button
                            onClick={(e) => { e.preventDefault(); e.stopPropagation(); onDelete(image.id); }}
                            className="p-2 bg-red-600/90 text-white rounded-full shadow-sm hover:bg-red-700 transition-transform hover:scale-110"
                            title="Permanently Delete Image"
                        >
                            <Trash2 size={14} />
                        </button>
                    )}
                </div>

                <div className="transform translate-y-4 group-hover:translate-y-0 transition-transform duration-300 text-white">
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
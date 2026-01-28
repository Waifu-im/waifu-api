import { ImageDto } from "../types";
import { Heart, Trash2 } from "lucide-react";
import { Link } from "react-router-dom";
import { useAuth } from "../context/AuthContext";

interface ImageCardProps {
  image: ImageDto;
  onDelete?: (id: number) => void;
}

const ImageCard = ({ image, onDelete }: ImageCardProps) => {
  const { user } = useAuth();
  const isAdmin = user?.role === 3; // Assuming 3 is Admin role

  return (
    <div className="relative group overflow-hidden rounded-lg mb-4 break-inside-avoid">
      <Link to={`/images/${image.id}`}>
        <img src={image.url} alt={`Image ${image.id}`} className="w-full h-auto object-cover" />
      </Link>
      <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex flex-col justify-between p-4">
        <div className="text-white text-sm">
          <p>ID: {image.id}</p>
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-white">
            <Heart size={20} className={image.likedAt ? "text-red-500 fill-current" : ""} />
            <span>{image.favorites}</span>
          </div>
          {isAdmin && onDelete && (
            <button 
              onClick={() => onDelete(image.id)}
              className="p-2 rounded-full bg-red-500/80 hover:bg-red-500 text-white transition-colors"
              aria-label="Delete Image"
            >
              <Trash2 size={18} />
            </button>
          )}
        </div>
      </div>
    </div>
  );
};

export default ImageCard;

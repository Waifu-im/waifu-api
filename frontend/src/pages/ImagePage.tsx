import { useState, useEffect } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import api from '../services/api';
import { ImageDto, Role, ImageFormData } from '../types';
import { Heart, Trash2, Edit, X, AlertTriangle } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import ImageModal from '../components/modals/ImageModal';

const ImagePage = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();
  const [image, setImage] = useState<ImageDto | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showNsfwWarning, setShowNsfwWarning] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);

  const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);

  useEffect(() => {
    const fetchImage = async () => {
      try {
        setIsLoading(true);
        const userId = user?.id || 0;
        const { data } = await api.get<ImageDto>(`/images/${id}?userId=${userId}`);
        setImage(data);

        if (data.isNsfw) {
          const hasConsented = localStorage.getItem('nsfw-consent');
          if (!hasConsented) {
            setShowNsfwWarning(true);
          }
        }
      } catch (err) {
        console.error("Failed to fetch image", err);
        setError("Failed to load image.");
      } finally {
        setIsLoading(false);
      }
    };

    fetchImage();
  }, [id, user]);

  const handleNsfwConsent = () => {
    localStorage.setItem('nsfw-consent', 'true');
    setShowNsfwWarning(false);
  };

  const handleNsfwReject = () => {
    navigate('/gallery?isNsfw=false');
  };

  const handleDelete = async () => {
    if (!confirm('Are you sure you want to delete this image? This action cannot be undone.')) return;
    try {
      await api.delete(`/images/${id}`);
      navigate('/gallery');
    } catch (err) {
      console.error('Failed to delete image', err);
      alert('Failed to delete image.');
    }
  };

  const handleLike = async () => {
    if (!user) {
      alert('You must be logged in to like images.');
      navigate('/login');
      return;
    }

    if (!image) return;

    try {
      if (image.likedAt) {
        await api.delete(`/users/${user.id}/albums/favorites/images/${image.id}`);
        setImage(prev => prev ? { ...prev, likedAt: undefined, favorites: prev.favorites - 1 } : null);
      } else {
        await api.post(`/users/${user.id}/albums/favorites/images/${image.id}`);
        setImage(prev => prev ? { ...prev, likedAt: new Date().toISOString(), favorites: prev.favorites + 1 } : null);
      }
    } catch (err) {
      console.error('Failed to toggle like status', err);
      alert('Failed to update like status.');
    }
  };

  const handleSaveEdit = async (data: ImageFormData) => {
    if (!isAdminOrModerator || !image) return;
    try {
      const payload = {
        source: data.source || null,
        isNsfw: data.isNsfw,
        userId: data.userId || null,
      };
      const { data: updatedImage } = await api.patch<ImageDto>(`/images/${id}`, payload);
      setImage(updatedImage);
      setShowEditModal(false);
    } catch (err) {
      console.error('Failed to update image', err);
      alert('Failed to update image.');
    }
  };

  if (isLoading) {
    return (
        <div className="container mx-auto px-4 py-8 text-center">
          <div className="animate-pulse bg-card rounded-lg p-8 max-w-3xl mx-auto">
            <div className="h-96 bg-muted rounded-md mb-6"></div>
          </div>
        </div>
    );
  }

  if (error || !image) {
    return (
        <div className="container mx-auto px-4 py-8 text-center text-destructive">
          <p>{error || "Image not found."}</p>
        </div>
    );
  }

  return (
      <div className="container mx-auto px-4 py-8">
        <div className="bg-card rounded-lg shadow-lg overflow-hidden border border-border">
          <div className="relative">
            <img src={image.url} alt={`Image ${image.id}`} className="w-full h-auto object-contain max-h-[80vh]" />
            {image.isNsfw && showNsfwWarning && (
                <div className="absolute inset-0 bg-background/90 backdrop-blur-lg flex items-center justify-center p-4">
                  <div className="text-center">
                    <AlertTriangle size={48} className="text-destructive mx-auto mb-4" />
                    <h3 className="text-2xl font-bold mb-2">NSFW Content</h3>
                    <button onClick={handleNsfwConsent} className="bg-destructive text-destructive-foreground px-6 py-3 rounded-lg font-bold mr-4">I am 18 or older</button>
                    <button onClick={handleNsfwReject} className="bg-secondary text-secondary-foreground px-6 py-3 rounded-lg font-medium">Go Back</button>
                  </div>
                </div>
            )}
          </div>

          <div className="p-6">
            <h1 className="text-3xl font-bold mb-4">Image #{image.id}</h1>

            <div className="flex flex-wrap items-center gap-4 mb-4">
              <button
                  onClick={handleLike}
                  className="flex items-center gap-2 px-4 py-2 rounded-md bg-secondary hover:bg-secondary/80 text-secondary-foreground transition-colors"
              >
                <Heart size={20} className={image.likedAt ? "text-red-500 fill-current" : ""} />
                <span>{image.favorites} Likes</span>
              </button>
              {isAdminOrModerator && (
                  <>
                    <button onClick={() => setShowEditModal(true)} className="flex items-center gap-2 px-4 py-2 rounded-md bg-accent hover:bg-accent/80 text-accent-foreground transition-colors">
                      <Edit size={20} /> <span>Edit</span>
                    </button>
                    <button onClick={handleDelete} className="flex items-center gap-2 px-4 py-2 rounded-md bg-destructive hover:bg-destructive/90 text-destructive-foreground transition-colors">
                      <Trash2 size={20} /> <span>Delete</span>
                    </button>
                  </>
              )}
            </div>

            <p className="text-muted-foreground mb-2">Uploaded At: {new Date(image.uploadedAt).toLocaleDateString()}</p>

            {image.artist && (
                <p className="text-muted-foreground mb-2 flex gap-2">
                  Artist:
                  <Link to={`/gallery?artistId=${image.artist.id}`} className="text-primary hover:underline font-bold">
                    {image.artist.name}
                  </Link>
                </p>
            )}

            {image.source && (
                <p className="text-muted-foreground mb-2">Source: <a href={image.source} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">{image.source}</a></p>
            )}
            <p className="text-muted-foreground mb-2">Dimensions: {image.width}x{image.height}</p>
            <p className="text-muted-foreground mb-4">File Size: {(image.byteSize / (1024 * 1024)).toFixed(2)} MB</p>

            <div className="mb-4">
              <h3 className="text-lg font-semibold mb-2">Tags:</h3>
              <div className="flex flex-wrap gap-2">
                {image.tags.map(tag => (
                    <Link
                        key={tag.id}
                        to={`/gallery?includedTags=${encodeURIComponent(tag.name)}`}
                        className="bg-secondary text-secondary-foreground px-3 py-1 rounded-full text-sm hover:bg-primary hover:text-primary-foreground transition-colors"
                    >
                      {tag.name}
                    </Link>
                ))}
              </div>
            </div>
          </div>
        </div>

        <ImageModal
            isOpen={showEditModal}
            onClose={() => setShowEditModal(false)}
            initialData={image}
            onSubmit={handleSaveEdit}
        />
      </div>
  );
};

export default ImagePage;
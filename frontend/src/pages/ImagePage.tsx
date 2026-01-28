import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import api from '../services/api';
import { ImageDto, Role } from '../types';
import { Heart, Trash2, Edit, X, AlertTriangle } from 'lucide-react';
import { useAuth } from '../context/AuthContext';

const ImagePage = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();
  const [image, setImage] = useState<ImageDto | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showNsfwWarning, setShowNsfwWarning] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [editSource, setEditSource] = useState<string>('');
  const [editIsNsfw, setEditIsNsfw] = useState<boolean>(false);
  const [editUserId, setEditUserId] = useState<string>('');

  const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);

  useEffect(() => {
    const fetchImage = async () => {
      try {
        setIsLoading(true);
        const userId = user?.id || 0; // Pass user ID for liked status
        const { data } = await api.get<ImageDto>(`/images/${id}?userId=${userId}`);
        setImage(data);

        // Check NSFW status and consent
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
    navigate('/gallery?isNsfw=false'); // Redirect to safe gallery
  };

  const handleDelete = async () => {
    if (!confirm('Are you sure you want to delete this image? This action cannot be undone.')) return;
    try {
      await api.delete(`/images/${id}`);
      navigate('/gallery'); // Redirect to gallery after deletion
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
    try {
      if (image?.likedAt) {
        // Unlike
        await api.delete(`/users/${user.id}/albums/favorites/images/${image.id}`);
        setImage(prev => prev ? { ...prev, likedAt: undefined, favorites: prev.favorites - 1 } : null);
      } else {
        // Like
        await api.post(`/users/${user.id}/albums/favorites/images/${image.id}`);
        setImage(prev => prev ? { ...prev, likedAt: new Date().toISOString(), favorites: prev.favorites + 1 } : null);
      }
    } catch (err) {
      console.error('Failed to toggle like status', err);
      alert('Failed to update like status.');
    }
  };

  const handleEdit = () => {
    if (image) {
      setEditSource(image.source || '');
      setEditIsNsfw(image.isNsfw);
      setEditUserId(image.uploaderId.toString());
      setShowEditModal(true);
    }
  };

  const handleSaveEdit = async () => {
    if (!isAdminOrModerator) return;
    try {
      const payload = {
        source: editSource || null,
        isNsfw: editIsNsfw,
        userId: parseInt(editUserId) || null,
      };
      const { data } = await api.patch<ImageDto>(`/images/${id}`, payload);
      setImage(data); // Update image with new data
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
          <div className="h-8 bg-muted rounded-md w-3/4 mx-auto mb-4"></div>
          <div className="h-6 bg-muted rounded-md w-1/2 mx-auto"></div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="container mx-auto px-4 py-8 text-center text-destructive">
        <p>{error}</p>
      </div>
    );
  }

  if (!image) {
    return (
      <div className="container mx-auto px-4 py-8 text-center text-muted-foreground">
        <p>Image not found.</p>
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
                <p className="text-muted-foreground mb-6">This image contains NSFW content. Please verify your age.</p>
                <button 
                  onClick={handleNsfwConsent}
                  className="bg-destructive hover:bg-destructive/90 text-destructive-foreground px-6 py-3 rounded-lg font-bold mr-4"
                >
                  I am 18 or older
                </button>
                <button 
                  onClick={handleNsfwReject}
                  className="bg-secondary hover:bg-secondary/80 text-secondary-foreground px-6 py-3 rounded-lg font-medium"
                >
                  Go Back
                </button>
              </div>
            </div>
          )}
        </div>

        <div className="p-6">
          <h1 className="text-3xl font-bold mb-4">Image #{image.id}</h1>
          <div className="flex items-center gap-4 mb-4">
            <button 
              onClick={handleLike}
              className="flex items-center gap-2 px-4 py-2 rounded-md bg-secondary hover:bg-secondary/80 text-secondary-foreground transition-colors"
            >
              <Heart size={20} className={image.likedAt ? "text-red-500 fill-current" : ""} />
              <span>{image.favorites} Likes</span>
            </button>
            {isAdminOrModerator && (
              <>
                <button 
                  onClick={handleEdit}
                  className="flex items-center gap-2 px-4 py-2 rounded-md bg-accent hover:bg-accent/80 text-accent-foreground transition-colors"
                >
                  <Edit size={20} />
                  <span>Edit</span>
                </button>
                <button 
                  onClick={handleDelete}
                  className="flex items-center gap-2 px-4 py-2 rounded-md bg-destructive hover:bg-destructive/90 text-destructive-foreground transition-colors"
                >
                  <Trash2 size={20} />
                  <span>Delete</span>
                </button>
              </>
            )}
          </div>

          <p className="text-muted-foreground mb-2">Uploaded At: {new Date(image.uploadedAt).toLocaleDateString()}</p>
          {image.artist && (
            <p className="text-muted-foreground mb-2">Artist: {image.artist.name}</p>
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
                <span key={tag.id} className="bg-secondary text-secondary-foreground px-3 py-1 rounded-full text-sm">
                  {tag.name}
                </span>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Edit Image Modal */}
      {showEditModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm p-4">
          <div className="bg-card p-8 rounded-lg shadow-xl max-w-md w-full border border-border">
            <div className="flex justify-between items-center mb-6">
              <h2 className="text-2xl font-bold">Edit Image #{image.id}</h2>
              <button onClick={() => setShowEditModal(false)} className="p-2 rounded-full hover:bg-accent">
                <X size={24} />
              </button>
            </div>
            <div className="space-y-4">
              <div>
                <label htmlFor="editSource" className="block text-sm font-medium mb-1">Source URL</label>
                <input
                  type="text"
                  id="editSource"
                  value={editSource}
                  onChange={(e) => setEditSource(e.target.value)}
                  className="w-full p-2 border border-input rounded-md bg-background focus:ring-2 focus:ring-primary focus:border-transparent outline-none"
                />
              </div>
              <div className="flex items-center">
                <input
                  type="checkbox"
                  id="editIsNsfw"
                  checked={editIsNsfw}
                  onChange={(e) => setEditIsNsfw(e.target.checked)}
                  className="mr-2 h-4 w-4 text-primary focus:ring-primary border-border rounded"
                />
                <label htmlFor="editIsNsfw" className="text-sm font-medium">Is NSFW?</label>
              </div>
              <div>
                <label htmlFor="editUserId" className="block text-sm font-medium mb-1">Uploader User ID</label>
                <input
                  type="text"
                  id="editUserId"
                  value={editUserId}
                  onChange={(e) => setEditUserId(e.target.value)}
                  className="w-full p-2 border border-input rounded-md bg-background focus:ring-2 focus:ring-primary focus:border-transparent outline-none"
                />
              </div>
              <button
                onClick={handleSaveEdit}
                className="w-full bg-primary text-primary-foreground py-2 rounded-md hover:bg-primary/90 transition"
              >
                Save Changes
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default ImagePage;

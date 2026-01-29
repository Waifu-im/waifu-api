import { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import api from '../services/api';
import { ImageDto, Role, ImageFormData, AlbumDto, PaginatedList } from '../types';
import { Heart, Trash2, Edit, AlertTriangle, FolderPlus, ChevronDown, Check, Flag } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import ImageModal from '../components/modals/ImageModal';
import ConfirmModal from '../components/modals/ConfirmModal';
import ReportModal from '../components/modals/ReportModal';

const ImagePage = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();
  const { showNotification } = useNotification();

  const [image, setImage] = useState<ImageDto | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showNsfwWarning, setShowNsfwWarning] = useState(false);

  // Modals
  const [showEditModal, setShowEditModal] = useState(false);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [isReportModalOpen, setIsReportModalOpen] = useState(false);

  // Album Dropdown
  const [userAlbums, setUserAlbums] = useState<AlbumDto[]>([]);
  const [isAlbumMenuOpen, setIsAlbumMenuOpen] = useState(false);
  const albumMenuRef = useRef<HTMLDivElement>(null);

  const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);

  // Function to fetch image (can be called silently to refresh data)
  const fetchImage = async (silent = false) => {
    try {
      if (!silent) setIsLoading(true);
      const userId = user?.id || 0;
      const { data } = await api.get<ImageDto>(`/images/${id}?userId=${userId}`);
      setImage(data);

      if (data.isNsfw && !silent) {
        const hasConsented = localStorage.getItem('nsfw-consent');
        if (!hasConsented) setShowNsfwWarning(true);
      }
    } catch (err) {
      console.error("Failed to fetch image", err);
      if (!silent) setError("Failed to load image.");
    } finally {
      if (!silent) setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchImage();
  }, [id, user?.id]);

  // Close dropdown on outside click
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (albumMenuRef.current && !albumMenuRef.current.contains(event.target as Node)) {
        setIsAlbumMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleFetchAlbums = async () => {
    if (!user) {
      showNotification('warning', 'You must be logged in to manage albums.');
      navigate('/login');
      return;
    }

    if (userAlbums.length > 0) {
      setIsAlbumMenuOpen(!isAlbumMenuOpen);
      return;
    }
    try {
      const { data } = await api.get<PaginatedList<AlbumDto>>('/users/me/albums', { params: { pageSize: 100 } });
      setUserAlbums(data.items);
      setIsAlbumMenuOpen(true);
    } catch (e) {
      showNotification('error', 'Failed to load albums');
    }
  };

  const handleToggleAlbum = async (targetAlbum: AlbumDto) => {
    if (!image || !user) return;

    const isInAlbum = image.albums?.some(a => a.id === targetAlbum.id);
    // FIXED: Use isDefault to identify the Favorites album instead of name string
    const isFavorites = targetAlbum.isDefault;

    try {
      if (isInAlbum) {
        // REMOVE
        await api.delete(`/users/${user.id}/albums/${targetAlbum.id}/images/${image.id}`);
        showNotification('success', `Removed from ${targetAlbum.name}`);

        setImage(prev => {
          if (!prev) return null;
          return {
            ...prev,
            albums: prev.albums?.filter(a => a.id !== targetAlbum.id),
            // Sync Heart if removing from Favorites
            likedAt: isFavorites ? undefined : prev.likedAt,
            favorites: isFavorites ? Math.max(0, prev.favorites - 1) : prev.favorites
          };
        });
      } else {
        // ADD
        await api.post(`/users/${user.id}/albums/${targetAlbum.id}/images/${image.id}`);
        showNotification('success', `Added to ${targetAlbum.name}`);

        setImage(prev => {
          if (!prev) return null;
          return {
            ...prev,
            albums: [...(prev.albums || []), targetAlbum],
            // Sync Heart if adding to Favorites
            likedAt: isFavorites ? new Date().toISOString() : prev.likedAt,
            favorites: isFavorites ? prev.favorites + 1 : prev.favorites
          };
        });
      }
    } catch (e) {
      showNotification('error', `Failed to ${isInAlbum ? 'remove from' : 'add to'} album`);
    }
  };

  const handleNsfwConsent = () => {
    localStorage.setItem('nsfw-consent', 'true');
    setShowNsfwWarning(false);
  };

  const handleDelete = async () => {
    try {
      await api.delete(`/images/${id}`);
      navigate('/gallery');
    } catch (err) {
      alert('Failed to delete image.');
    }
  };

  const handleLike = async () => {
    if (!user) {
      showNotification('warning', 'You must be logged in to like images.');
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
      // Silently refresh to update the "Favorites" checkmark in the dropdown
      fetchImage(true);
    } catch (err) { console.error(err); }
  };

  const handleSaveEdit = async (data: ImageFormData) => {
    if (!isAdminOrModerator || !image) return;
    try {
      const payload = {
        source: data.source || null,
        isNsfw: data.isNsfw,
        userId: data.userId || null,
        tagIds: data.tagIds || [],
        artistIds: data.artistIds || []
      };
      const { data: updatedImage } = await api.put<ImageDto>(`/images/${id}`, payload);
      setImage(updatedImage);
      setShowEditModal(false);
      showNotification('success', 'Image updated successfully');
    } catch (err) {
      console.error('Failed to update image', err);
      showNotification('error', 'Failed to update image');
    }
  };

  const handleReport = async (description: string) => {
    if (!image) return;
    try {
      await api.post('/reports', { imageId: image.id, description });
      showNotification('success', 'Report submitted');
      setIsReportModalOpen(false);
    } catch (err) {
      showNotification('error', 'Failed to submit report');
    }
  };

  if (isLoading) return <div className="p-10 text-center">Loading...</div>;
  if (error || !image) return <div className="p-10 text-center text-destructive">{error || "Not Found"}</div>;

  return (
      <div className="container mx-auto px-4 py-6 md:py-8">
        <div className="bg-card rounded-lg shadow-lg border border-border">

          {/* Image Preview */}
          <div className="relative bg-black/5 rounded-t-lg overflow-hidden">
            <img
                src={image.url}
                alt={`Image ${image.id}`}
                className="w-full h-auto object-contain max-h-[60vh] md:max-h-[80vh] mx-auto"
            />

            {image.isNsfw && showNsfwWarning && (
                <div className="absolute inset-0 bg-background/95 backdrop-blur-xl flex items-center justify-center p-4 z-10">
                  <div className="text-center max-w-sm">
                    <AlertTriangle size={48} className="text-destructive mx-auto mb-4" />
                    <h3 className="text-2xl font-bold mb-2">NSFW Content</h3>
                    <p className="text-muted-foreground mb-6">This content is flagged as 18+.</p>
                    <div className="flex gap-3 justify-center">
                      <button onClick={handleNsfwConsent} className="bg-primary text-primary-foreground px-6 py-2.5 rounded-lg font-bold">Show Content</button>
                      <button onClick={() => navigate('/gallery?isNsfw=0')} className="bg-secondary px-6 py-2.5 rounded-lg font-bold">Go Back</button>
                    </div>
                  </div>
                </div>
            )}
          </div>

          <div className="p-4 md:p-6">
            <h1 className="text-2xl md:text-3xl font-bold mb-4">Image #{image.id}</h1>

            {/* Actions Bar */}
            <div className="flex flex-wrap items-center gap-2 md:gap-3 mb-6">
              <button
                  onClick={handleLike}
                  className="flex-1 sm:flex-none flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-secondary hover:bg-secondary/80 text-secondary-foreground transition-colors font-medium text-sm"
              >
                <Heart size={18} className={image.likedAt ? "text-red-500 fill-current" : ""} />
                <span>{image.favorites}</span>
              </button>

              {/* Album Dropdown */}
              <div className="relative flex-1 sm:flex-none" ref={albumMenuRef}>
                <button
                    onClick={handleFetchAlbums}
                    className="w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-colors font-medium text-sm"
                >
                  <FolderPlus size={18} className="shrink-0" />
                  <span className="truncate">Add to Album</span>
                  <ChevronDown size={16} className={`transition-transform shrink-0 ${isAlbumMenuOpen ? 'rotate-180' : ''}`} />
                </button>

                {isAlbumMenuOpen && (
                    <div className="absolute top-full right-0 mt-2 w-56 bg-popover border border-border rounded-xl shadow-xl py-1 z-50 animate-in fade-in zoom-in-95 origin-top-right">
                      {userAlbums.length === 0 ? (
                          <div className="px-4 py-3 text-sm text-muted-foreground text-center">No albums found.</div>
                      ) : (
                          <div className="max-h-60 overflow-y-auto">
                            {userAlbums.map(album => {
                              const isInAlbum = image.albums?.some(a => a.id === album.id);
                              return (
                                  <button
                                      key={album.id}
                                      onClick={() => handleToggleAlbum(album)}
                                      className="w-full text-left px-4 py-3 hover:bg-accent hover:text-accent-foreground text-sm flex items-center justify-between group transition-colors"
                                  >
                                    <span className="truncate pr-2 font-medium">{album.name}</span>
                                    {isInAlbum && <Check size={18} className="text-primary shrink-0" />}
                                  </button>
                              );
                            })}
                          </div>
                      )}
                      <div className="border-t border-border mt-1 pt-1">
                        <Link to="/albums" className="block w-full text-center px-4 py-3 text-xs font-bold text-primary hover:underline hover:bg-accent/50 rounded-b-xl">
                          + Create New Album
                        </Link>
                      </div>
                    </div>
                )}
              </div>

              {/* Report Button */}
              <button
                  onClick={() => {
                      if (!user) {
                          showNotification('warning', 'You must be logged in to report images.');
                          navigate('/login');
                          return;
                      }
                      setIsReportModalOpen(true);
                  }}
                  className="flex-1 sm:flex-none flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-secondary hover:bg-secondary/80 text-secondary-foreground transition-colors font-medium text-sm"
                  title="Report"
              >
                  <Flag size={18} />
                  <span className="hidden sm:inline">Report</span>
              </button>

              {isAdminOrModerator && (
                  <>
                    <div className="h-8 w-px bg-border mx-1 hidden sm:block"></div>
                    <button
                        onClick={() => setShowEditModal(true)}
                        className="flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-accent hover:bg-accent/80 text-accent-foreground transition-colors font-medium text-sm"
                        title="Edit"
                    >
                      <Edit size={18} /> <span className="hidden sm:inline">Edit</span>
                    </button>
                    <button
                        onClick={() => setIsDeleteModalOpen(true)}
                        className="flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-destructive hover:bg-destructive/90 text-white transition-colors font-medium text-sm"
                        title="Delete"
                    >
                      <Trash2 size={18} /> <span className="hidden sm:inline">Delete</span>
                    </button>
                  </>
              )}
            </div>

            <div className="space-y-3 text-sm text-muted-foreground">
              <div className="flex flex-wrap gap-x-6 gap-y-2">
                <p>Uploaded: <span className="text-foreground font-medium">{new Date(image.uploadedAt).toLocaleDateString()}</span></p>
                <p>Size: <span className="text-foreground font-medium">{(image.byteSize / (1024 * 1024)).toFixed(2)} MB</span></p>
                <p>Dims: <span className="text-foreground font-medium">{image.width}x{image.height}</span></p>
              </div>

              {image.artists && image.artists.length > 0 && (
                  <p className="flex items-center gap-2">
                    Artist{image.artists.length > 1 ? 's' : ''}:
                    {image.artists.map((artist, index) => (
                        <span key={artist.id}>
                            <Link to={`/gallery?includedArtists=${artist.id}`} className="text-primary hover:underline font-bold text-base">
                              {artist.name}
                            </Link>
                            {index < image.artists.length - 1 && ", "}
                        </span>
                    ))}
                  </p>
              )}

              {image.source && (
                  <p>Source: <a href={image.source} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline truncate inline-block max-w-[200px] align-bottom">{image.source}</a></p>
              )}
            </div>

            <div className="mt-6">
              <h3 className="text-xs font-bold uppercase text-muted-foreground mb-3 tracking-wider">Tags</h3>
              <div className="flex flex-wrap gap-2">
                {image.tags.map(tag => (
                    <Link
                        key={tag.id}
                        to={`/gallery?includedTags=${encodeURIComponent(tag.name)}`}
                        className="bg-secondary hover:bg-primary hover:text-primary-foreground text-secondary-foreground px-3 py-1.5 rounded-full text-sm font-medium transition-colors"
                    >
                      {tag.name}
                    </Link>
                ))}
              </div>
            </div>
          </div>
        </div>

        <ConfirmModal
            isOpen={isDeleteModalOpen}
            onClose={() => setIsDeleteModalOpen(false)}
            onConfirm={handleDelete}
            title="Delete Image"
            message="Permanently delete this image? This cannot be undone."
            confirmText="Delete"
            variant="destructive"
        />

        <ImageModal
            isOpen={showEditModal}
            onClose={() => setShowEditModal(false)}
            initialData={image}
            onSubmit={handleSaveEdit}
        />

        <ReportModal
            isOpen={isReportModalOpen}
            onClose={() => setIsReportModalOpen(false)}
            onSubmit={handleReport}
        />
      </div>
  );
};

export default ImagePage;
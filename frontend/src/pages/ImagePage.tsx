import { useState, useEffect} from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import api from '../services/api';
import { ImageDto, Role, ImageFormData, ReviewStatus } from '../types';
import { Heart, Trash2, Edit, FolderPlus, Flag, Image } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import { useNsfwConsent } from '../hooks/useNsfwConsent';
import { useImageUpdate } from '../hooks/useImageUpdate';
import { useAuthGuard } from '../hooks/useRequireAuth';
import ImageModal from '../components/modals/ImageModal';
import ConfirmModal from '../components/modals/ConfirmModal';
import ReasonModal from '../components/modals/ReasonModal';
import AlbumSelectionModal from '../components/modals/AlbumSelectionModal';
import NsfwWarning from '../components/NsfwWarning';

const ImagePage = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();
  const { showNotification } = useNotification();
  const checkAuth = useAuthGuard();

  const [image, setImage] = useState<ImageDto | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const { showWarning: showNsfwWarning, setShowWarning: setShowNsfwWarning, grantConsent, hasConsent } = useNsfwConsent(false);
  const { updateImage } = useImageUpdate();

  // Modals
  const [showEditModal, setShowEditModal] = useState(false);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [isReportModalOpen, setIsReportModalOpen] = useState(false);
  const [isAlbumModalOpen, setIsAlbumModalOpen] = useState(false);

  const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);

  // Function to fetch image (can be called silently to refresh data)
  const fetchImage = async (silent = false) => {
    try {
      if (!silent) setIsLoading(true);
      // Removed userId query param as it's handled by auth token now
      const params: Record<string, string> = {};
      if (isAdminOrModerator) {
        params.childrenReviewStatus = 'All';
      }
      const { data } = await api.get<ImageDto>(`/images/${id}`, { params, skipGlobalErrorHandler: true });
      setImage(data);

      if (data.isNsfw && !silent && !hasConsent()) {
        setShowNsfwWarning(true);
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
  }, [id, user?.id]); // Keep user.id dependency to refresh if user logs in/out

  const handleDelete = async () => {
    try {
      await api.delete(`/images/${id}`);
      showNotification('success', 'Image deleted successfully');
      navigate('/gallery');
    } catch (err) {
      showNotification('error', 'Failed to delete image');
    }
  };

  const handleLike = async () => {
    if (!checkAuth('You must be logged in to like images.')) return;
    if (!image) return;
    try {
      if (image.likedAt) {
        await api.delete(`/users/${user!.id}/albums/favorites/images/${image.id}`, { skipGlobalErrorHandler: true });
        setImage(prev => prev ? { ...prev, likedAt: undefined, favorites: prev.favorites - 1 } : null);
      } else {
        await api.post(`/users/${user!.id}/albums/favorites/images/${image.id}`, {}, { skipGlobalErrorHandler: true });
        setImage(prev => prev ? { ...prev, likedAt: new Date().toISOString(), favorites: prev.favorites + 1 } : null);
      }
      // Silently refresh to update the "Favorites" checkmark in the dropdown
      fetchImage(true);
    } catch (err) { console.error(err); }
  };

  const handleSaveEdit = async (data: ImageFormData) => {
    if (!isAdminOrModerator || !image) return;
    try {
      const updatedImage = await updateImage(image.id, data);
      setImage(updatedImage);
      setShowEditModal(false);
      showNotification('success', 'Image updated successfully');
      fetchImage(true);
    } catch (err) {
      console.error('Failed to update image', err);
    }
  };

  const handleReport = async (description: string) => {
    if (!image) return;
    try {
      await api.post('/reports', { imageId: image.id, description });
      showNotification('success', 'Report submitted');
      setIsReportModalOpen(false);
    } catch (err) {
      // showNotification('error', 'Failed to submit report'); // Handled globally
    }
  };

  const getReviewStatusLabel = (status: ReviewStatus) => {
      switch (status) {
          case ReviewStatus.Pending: return 'Pending';
          case ReviewStatus.Accepted: return 'Accepted';
          case ReviewStatus.Rejected: return 'Rejected';
          default: return 'Unknown';
      }
  };

  const getReviewStatusColor = (status: ReviewStatus) => {
      switch (status) {
          case ReviewStatus.Pending: return 'text-yellow-500';
          case ReviewStatus.Accepted: return 'text-green-500';
          case ReviewStatus.Rejected: return 'text-red-500';
          default: return 'text-muted-foreground';
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
                <NsfwWarning
                    variant="overlay"
                    onConsent={grantConsent}
                    onDecline={() => window.history.length > 1 ? navigate(-1) : navigate('/gallery')}
                />
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

              {/* Album Button */}
              <button
                  onClick={() => setIsAlbumModalOpen(true)}
                  className="flex-1 sm:flex-none flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-colors font-medium text-sm"
              >
                  <FolderPlus size={18} className="shrink-0" />
                  <span className="truncate">Add to Album</span>
              </button>

              {/* Report Button */}
              <button
                  onClick={() => {
                      if (!checkAuth('You must be logged in to report images.')) return;
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
                <p>Status: <span className={`font-medium ${getReviewStatusColor(image.reviewStatus)}`}>{getReviewStatusLabel(image.reviewStatus)}</span></p>
              </div>

              {image.source && (
                  <p>Source: <a href={image.source} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline truncate inline-block max-w-[200px] align-bottom">{image.source}</a></p>
              )}
            </div>

            <div className="mt-6">
              <h3 className="text-xs font-bold uppercase text-muted-foreground mb-3 tracking-wider">
                Artist{image.artists.length > 1 ? 's' : ''}
              </h3>
              <div className="flex flex-wrap gap-2">
                {image.artists.map(artist => (
                    <span key={artist.id} className="inline-flex items-stretch">
                      <Link
                          to={`/artists?includedIds=${artist.id}`}
                          className="flex items-center px-3 py-1.5 text-sm font-medium bg-secondary hover:bg-primary hover:text-primary-foreground text-secondary-foreground border border-border border-r-0 rounded-l-full transition-colors"
                          title="View artist details"
                      >
                        {artist.name}
                      </Link>
                      <Link
                          to={`/gallery?includedArtists=${artist.id}`}
                          className="flex items-center px-2.5 bg-secondary/60 hover:bg-primary hover:text-primary-foreground text-primary border border-border rounded-r-full transition-colors"
                          title="View images by this artist"
                      >
                        <Image size={14} />
                      </Link>
                    </span>
                ))}
              </div>
            </div>

            <div className="mt-6">
              <h3 className="text-xs font-bold uppercase text-muted-foreground mb-3 tracking-wider">Tags</h3>
              <div className="flex flex-wrap gap-2">
                {image.tags.map(tag => (
                    <span key={tag.id} className="inline-flex items-stretch">
                      <Link
                          to={`/tags?includedIds=${tag.id}`}
                          className="flex items-center px-3 py-1.5 text-sm font-medium bg-secondary hover:bg-primary hover:text-primary-foreground text-secondary-foreground border border-border border-r-0 rounded-l-full transition-colors"
                          title="View tag details"
                      >
                        {tag.name}
                      </Link>
                      <Link
                          to={`/gallery?includedTags=${encodeURIComponent(tag.slug)}`}
                          className="flex items-center px-2.5 bg-secondary/60 hover:bg-primary hover:text-primary-foreground text-primary border border-border rounded-r-full transition-colors"
                          title="View images with this tag"
                      >
                        <Image size={14} />
                      </Link>
                    </span>
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
            zIndex={70}
        />

        <ImageModal
            isOpen={showEditModal}
            onClose={() => setShowEditModal(false)}
            initialData={image}
            onSubmit={handleSaveEdit}
            onDelete={() => setIsDeleteModalOpen(true)}
        />

        <ReasonModal
            isOpen={isReportModalOpen}
            onClose={() => setIsReportModalOpen(false)}
            onSubmit={handleReport}
            title="Report Image"
            description="Please describe why you are reporting this image."
            placeholder="Reason for reporting..."
            submitText="Submit Report"
        />

        <AlbumSelectionModal
            isOpen={isAlbumModalOpen}
            onClose={() => setIsAlbumModalOpen(false)}
            image={image}
            onUpdate={(updated) => setImage(updated)}
        />
      </div>
  );
};

export default ImagePage;
import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useImages } from '../hooks/useImages';
import { SlidersHorizontal, RefreshCw, Search } from 'lucide-react';
import ImageCard from '../components/ImageCard';
import api from '../services/api';
import { ImageDto, Role, ImageFormData } from '../types';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import ImageModal from '../components/modals/ImageModal';
import FilterSidebar from '../components/FilterSidebar';
import ConfirmModal from '../components/modals/ConfirmModal';

const Gallery = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const [showFilters, setShowFilters] = useState(false);
  const [showNsfwWarning, setShowNsfwWarning] = useState(false);
  const { user } = useAuth();
  const { showNotification } = useNotification();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [imageToDelete, setImageToDelete] = useState<number | null>(null);
  const [editingImage, setEditingImage] = useState<ImageDto | null>(null);
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);

  // Extract Params
  const isNsfw = searchParams.get('isNsfw') || '0';
  const orderBy = searchParams.get('orderBy') || 'UPLOADED_AT';
  const orientation = searchParams.get('orientation') || '';
  const isAnimatedStr = searchParams.get('isAnimated');
  const height = searchParams.get('height') || '';
  const width = searchParams.get('width') || '';
  const byteSize = searchParams.get('byteSize') || '';
  const includedTags = searchParams.getAll('includedTags');
  const excludedTags = searchParams.getAll('excludedTags');

  const artistIdStr = searchParams.get('artistId');
  const artistId = artistIdStr ? parseInt(artistIdStr) : undefined;

  const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);
  const isAnimatedBool = isAnimatedStr === 'true' ? true : isAnimatedStr === 'false' ? false : undefined;

  const { data: paginatedImages, isLoading, error, refetch } = useImages({
    isNsfw: parseInt(isNsfw),
    orderBy,
    orientation,
    isAnimated: isAnimatedBool,
    width,
    height,
    byteSize,
    includedTags,
    excludedTags,
    artistId,
    page: 1,
    pageSize: 50
  });

  const images = paginatedImages?.items || [];

  useEffect(() => {
    if (isNsfw !== '0' && !localStorage.getItem('nsfw-consent')) {
      setShowNsfwWarning(true);
    }
  }, [isNsfw]);

  const confirmDelete = (id: number) => {
    setImageToDelete(id);
    setIsDeleteModalOpen(true);
  };

  const handleRealDelete = async () => {
    if (!imageToDelete) return;
    try {
      await api.delete(`/images/${imageToDelete}`);
      refetch();
      setIsDeleteModalOpen(false);
      setImageToDelete(null);
      showNotification('success', 'Image deleted');
    } catch(e) {
      showNotification('error', 'Failed to delete image');
    }
  };

  const handleSaveEdit = async (data: ImageFormData) => {
    if (!editingImage) return;
    try {
      const payload = {
        source: data.source || null,
        isNsfw: data.isNsfw,
        userId: data.userId || null,
      };
      await api.patch<ImageDto>(`/images/${editingImage.id}`, payload);
      showNotification('success', 'Image updated');
      refetch();
      setIsEditModalOpen(false);
      setEditingImage(null);
    } catch (err) {
      showNotification('error', 'Failed to update image');
    }
  };

  const sortOptions = [
    { id: 'UPLOADED_AT', name: 'Newest First' },
    { id: 'FAVORITES', name: 'Most Popular' },
    { id: 'RANDOM', name: 'Random Shuffle' }
  ];

  return (
      <div className="flex h-full relative overflow-hidden">
        <div className="flex-1 overflow-y-auto h-full p-4">
          <div className="flex justify-between items-center mb-6">
            <h2 className="text-2xl font-black tracking-tight text-foreground flex gap-2">
              Gallery
            </h2>
            <button onClick={() => setShowFilters(!showFilters)} className="flex items-center gap-2 px-4 py-2 bg-card border border-border rounded-lg text-sm font-medium shadow-sm hover:bg-secondary transition-colors">
              <SlidersHorizontal size={16} /> <span className="hidden sm:inline">{showFilters ? 'Hide' : 'Filters'}</span>
            </button>
          </div>

          {isLoading ? (
              <div className="columns-2 sm:columns-3 md:columns-4 gap-4 space-y-4">
                {[...Array(12)].map((_,i) => <div key={i} className="bg-muted h-64 rounded-xl animate-pulse break-inside-avoid"></div>)}
              </div>
          ) : error ? (
              <div className="flex flex-col items-center justify-center h-64 text-muted-foreground">
                <p className="mb-4">Unable to load images.</p>
                <button onClick={() => refetch()} className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg"><RefreshCw size={16}/> Retry</button>
              </div>
          ) : images.length > 0 ? (
              <div className="columns-2 sm:columns-3 md:columns-4 xl:columns-5 gap-4 space-y-4 pb-10">
                {images.map(img => (
                    <div key={img.id} className="break-inside-avoid relative group">
                      <ImageCard
                          image={img}
                          onDelete={confirmDelete}
                          onEdit={isAdminOrModerator ? (img) => { setEditingImage(img); setIsEditModalOpen(true); } : undefined}
                      />
                    </div>
                ))}
              </div>
          ) : (
              <div className="flex flex-col items-center justify-center h-[50vh] text-muted-foreground">
                <Search size={48} className="mb-4 opacity-20" />
                <p className="text-lg font-medium">No images found.</p>
                <button onClick={() => { setSearchParams({}); refetch(); }} className="mt-6 px-6 py-2 bg-secondary text-foreground rounded-lg font-bold">Clear All Filters</button>
              </div>
          )}
        </div>

        <FilterSidebar
            searchParams={searchParams}
            setSearchParams={setSearchParams}
            showFilters={showFilters}
            setShowFilters={setShowFilters}
            sortOptions={sortOptions}
        />

        {/* Updated Deletion Message to match ImagePage */}
        <ConfirmModal
            isOpen={isDeleteModalOpen}
            onClose={() => setIsDeleteModalOpen(false)}
            onConfirm={handleRealDelete}
            title="Delete Image"
            message="Permanently delete this image? This cannot be undone."
            confirmText="Delete"
            variant="destructive"
        />

        {editingImage && (
            <ImageModal
                isOpen={isEditModalOpen}
                onClose={() => { setIsEditModalOpen(false); setEditingImage(null); }}
                initialData={editingImage}
                onSubmit={handleSaveEdit}
            />
        )}

        {showNsfwWarning && (
            <div className="fixed inset-0 z-[60] bg-black/90 flex items-center justify-center p-4 backdrop-blur-md">
              <div className="bg-card p-8 rounded-2xl max-w-sm w-full text-center border border-border shadow-2xl">
                <h2 className="text-2xl font-black mb-2">Age Restricted</h2>
                <button onClick={() => { localStorage.setItem('nsfw-consent', 'true'); setShowNsfwWarning(false); }} className="w-full py-3 bg-primary text-primary-foreground rounded-xl font-bold mb-2">I am 18+</button>
                <button onClick={() => { setSearchParams(prev => { prev.set('isNsfw', '0'); return prev; }); setShowNsfwWarning(false); }} className="w-full py-3 bg-secondary rounded-xl font-bold">Go Back</button>
              </div>
            </div>
        )}
      </div>
  );
};

export default Gallery;
import { useState, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useNavigate, useLocation } from 'react-router-dom';
import api from '../services/api';
import SearchableSelect from '../components/SearchableSelect';
import { UploadCloud, Info, Loader2 } from 'lucide-react';
import { useNotification } from '../context/NotificationContext';
import { useAuth } from '../context/AuthContext';
import TagModal from '../components/modals/TagModal';
import ArtistModal from '../components/modals/ArtistModal';
import { useMetadata } from '../hooks/useMetadata';
import { ImageDto } from '../types';

interface UploadForm {
  file: FileList;
  source: string;
  isNsfw: boolean;
}

const Upload = () => {
  const { register, handleSubmit, watch, formState: { errors } } = useForm<UploadForm>();
  const navigate = useNavigate();
  const location = useLocation();
  const { showNotification } = useNotification();
  const { user } = useAuth();

  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [isUploading, setIsUploading] = useState(false);

  const isReviewMode = !(user?.role === 2 || user?.role === 3);

  const {
      selectedTags, setSelectedTags,
      selectedArtists, setSelectedArtists,
      showCreateTagModal, setShowCreateTagModal,
      newTagName, setNewTagName,
      showCreateArtistModal, setShowCreateArtistModal,
      newArtistName, setNewArtistName,
      loadTags, loadArtists,
      handleCreateTag, handleCreateArtist,
      slugify
  } = useMetadata(isReviewMode);

  const fileList = watch('file');

  useEffect(() => {
    if (fileList && fileList.length > 0) {
      const url = URL.createObjectURL(fileList[0]);
      setPreviewUrl(url);
      return () => URL.revokeObjectURL(url);
    } else {
      setPreviewUrl(null);
    }
  }, [fileList]);

  useEffect(() => {
    if (!user) {
      showNotification('warning', 'You must be logged in to upload images.');
      navigate('/login', { state: { from: location } });
      return;
    }
  }, [user, navigate, showNotification, location]);

  const onSubmit = async (data: UploadForm) => {
    setIsUploading(true);
    const formData = new FormData();
    formData.append('file', data.file[0]);

    // Updated keys: tags (slugs) and artists (ids)
    selectedTags.forEach(tag => formData.append('tags', tag.slug || slugify(tag.name.trim())));
    selectedArtists.forEach(artist => formData.append('artists', String(artist.id)));

    if (data.source) formData.append('source', data.source.trim());
    formData.append('isNsfw', String(data.isNsfw));

    try {
      const { data: uploadedImage } = await api.post<ImageDto>('/images/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } });
      showNotification(isReviewMode ? 'info' : 'success', isReviewMode ? 'Upload pending review.' : 'Image uploaded!');
      navigate(`/images/${uploadedImage.id}`);
    } catch (e) { 
        // showNotification('error', 'Upload failed'); // Handled globally
        setIsUploading(false);
    }
  };

  if (!user) return null;

  return (
      <div className="max-w-5xl mx-auto p-4 md:p-8">
        {/* Moderation Notice */}
        <div className="bg-blue-500/10 border border-blue-500/20 rounded-xl p-4 mb-8 flex items-start gap-3">
          <Info className="text-blue-500 shrink-0 mt-0.5" size={20} />
          <div>
            <h4 className="font-bold text-blue-600 dark:text-blue-400 text-sm">Review Process</h4>
            <p className="text-sm text-muted-foreground mt-1">
              New uploads require moderation approval before appearing publicly.
            </p>
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
          <div className="order-2 lg:order-1">
            <label className={`flex flex-col items-center justify-center w-full h-[500px] border-2 border-dashed rounded-2xl cursor-pointer transition-all duration-300 relative overflow-hidden group ${previewUrl ? 'border-primary bg-card' : 'border-border bg-secondary/30 hover:bg-secondary/50 hover:border-primary/50'}`}>
              {previewUrl ? (
                  <>
                    <img src={previewUrl} className="w-full h-full object-contain p-2" alt="Preview" />
                    <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center text-white font-medium">Click to change image</div>
                  </>
              ) : (
                  <div className="text-center p-6 text-muted-foreground">
                    <UploadCloud size={32} className="mx-auto mb-4" />
                    <p className="text-lg font-medium text-foreground">Click or drag image here</p>
                    <p className="text-sm mt-1">Supports PNG, JPG, GIF</p>
                  </div>
              )}
              <input type="file" accept="image/*" {...register('file', { required: true })} className="hidden" />
            </label>
            {errors.file && <p className="text-destructive text-center mt-2 font-medium">Image is required.</p>}
          </div>

          <div className="order-1 lg:order-2 space-y-6">
            <h1 className="text-3xl font-black mb-2">Upload Image</h1>
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
              <div className="space-y-4">
                <SearchableSelect
                    label="Artists"
                    placeholder="Search artists..."
                    loadOptions={loadArtists}
                    selectedOptions={selectedArtists}
                    onSelect={(o) => setSelectedArtists(p => p.some(a => a.id === o.id) ? p : [...p, o])}
                    onRemove={(o) => setSelectedArtists(p => p.filter(a => a.id !== o.id))}
                    onCreate={(name) => { setNewArtistName(name.trim()); setShowCreateArtistModal(true); }}
                    isMulti={true}
                />
                <SearchableSelect
                    label="Tags"
                    placeholder="Add tags..."
                    loadOptions={loadTags}
                    selectedOptions={selectedTags}
                    onSelect={(o) => setSelectedTags(p => p.some(t => t.id === o.id) ? p : [...p, o])}
                    onRemove={(o) => setSelectedTags(p => p.filter(t => t.id !== o.id))}
                    onCreate={(name) => { setNewTagName(name.trim()); setShowCreateTagModal(true); }}
                    isMulti={true}
                />
                <div>
                  <label className="block text-sm font-medium mb-1.5">Source URL</label>
                  <input type="url" {...register('source')} className="w-full p-3 rounded-lg bg-secondary border-transparent focus:ring-2 focus:ring-primary outline-none transition-all" placeholder="https://pixiv.net/..." />
                </div>
                <label className="flex items-center gap-3 p-4 border border-border rounded-xl cursor-pointer hover:bg-secondary/50 transition-colors">
                  <input type="checkbox" {...register('isNsfw')} className="w-5 h-5 rounded text-destructive focus:ring-destructive" />
                  <div><span className="block font-bold">NSFW Content</span><span className="text-xs text-muted-foreground">Contains adult material</span></div>
                </label>
              </div>
              <button 
                type="submit" 
                disabled={isUploading}
                className="w-full py-4 rounded-xl bg-foreground text-background font-bold text-lg hover:opacity-90 transition-all shadow-lg flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isUploading ? (
                    <>
                        <Loader2 size={20} className="animate-spin" />
                        Uploading...
                    </>
                ) : (
                    "Upload Now"
                )}
              </button>
            </form>
          </div>
        </div>

        <TagModal
            isOpen={showCreateTagModal}
            onClose={() => setShowCreateTagModal(false)}
            onSubmit={handleCreateTag}
            initialData={{ name: newTagName }}
            title="New Tag"
            isReviewMode={isReviewMode}
            submitLabel="Create"
        />

        <ArtistModal
            isOpen={showCreateArtistModal}
            onClose={() => setShowCreateArtistModal(false)}
            onSubmit={handleCreateArtist}
            initialData={{ name: newArtistName }}
            title="New Artist"
            isReviewMode={isReviewMode}
            submitLabel="Create"
        />
      </div>
  )
}

export default Upload;
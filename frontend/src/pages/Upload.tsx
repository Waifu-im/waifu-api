import { useState, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useNavigate, useLocation } from 'react-router-dom';
import api from '../services/api';
import SearchableSelect from '../components/SearchableSelect';
import { Tag, Artist, PaginatedList } from '../types';
import { UploadCloud, Info } from 'lucide-react';
import { useNotification } from '../context/NotificationContext';
import { useAuth } from '../context/AuthContext';
import TagModal, { TagFormData } from '../components/modals/TagModal';
import ArtistModal, { ArtistFormData } from '../components/modals/ArtistModal';
import { useRequireAuth } from '../hooks/useRequireAuth';

interface UploadForm {
  file: FileList;
  source: string;
  isNsfw: boolean;
}

interface Option {
  id: number | string;
  name: string;
  description?: string;
}

const Upload = () => {
  const user = useRequireAuth('/login', 'You must be logged in to upload images.');
  const { register, handleSubmit, watch, formState: { errors } } = useForm<UploadForm>();
  const navigate = useNavigate();
  const location = useLocation();
  const { showNotification } = useNotification();

  const [tags, setTags] = useState<Option[]>([]);
  const [artists, setArtists] = useState<Option[]>([]);
  const [selectedTags, setSelectedTags] = useState<Option[]>([]);
  const [selectedArtists, setSelectedArtists] = useState<Option[]>([]);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);

  // Modals
  const [showCreateTagModal, setShowCreateTagModal] = useState(false);
  const [newTagName, setNewTagName] = useState('');

  const [showCreateArtistModal, setShowCreateArtistModal] = useState(false);
  const [newArtistName, setNewArtistName] = useState('');

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
    if (!user) return;

    const fetchData = async () => {
      try {
        const [tagsRes, artistsRes] = await Promise.all([
          api.get<PaginatedList<Tag>>('/tags', { params: { pageSize: 1000 } }),
          api.get<PaginatedList<Artist>>('/artists?pageSize=1000')
        ]);

        setTags(tagsRes.data.items.map(t => ({ id: t.id, name: t.name, description: t.description })));
        setArtists(artistsRes.data.items.map(a => ({ id: a.id, name: a.name })));
      } catch (e) { console.error(e); }
    };
    fetchData();
  }, [user, navigate, showNotification, location]);

  const isReviewMode = !(user?.role === 2 || user?.role === 3);

  const handleCreateTag = async (data: TagFormData) => {
    try {
      const res = await api.post<Tag>('/tags', data);
      const newOpt = { id: res.data.id, name: res.data.name, description: res.data.description };
      // Check duplicate before adding
      setSelectedTags(p => p.some(t => t.id === newOpt.id) ? p : [...p, newOpt]);

      showNotification(isReviewMode ? 'info' : 'success', isReviewMode ? 'Tag submitted for review' : 'Tag created');
      setShowCreateTagModal(false);
    } catch (err: any) {
      if(err.response?.status === 409) {
        try {
          const existingRes = await api.get<Tag>(`/tags/by-name/${data.name}`);
          const existing = existingRes.data;
          const existingOpt = { id: existing.id, name: existing.name, description: existing.description };
          
          setSelectedTags(p => p.some(t => t.id === existingOpt.id) ? p : [...p, existingOpt]);
          showNotification('info', 'Tag already exists (possibly under review), added to selection.');
          setShowCreateTagModal(false);
        } catch (fetchErr) {
          showNotification('error', 'Tag exists but could not be retrieved.');
        }
      } else {
        showNotification('error', 'Failed to create tag.');
      }
    }
  };

  const handleCreateArtist = async (data: ArtistFormData) => {
    try {
      const res = await api.post<Artist>('/artists', data);
      const newOption = { id: res.data.id, name: res.data.name };
      setSelectedArtists(p => p.some(a => a.id === newOption.id) ? p : [...p, newOption]);

      showNotification(isReviewMode ? 'info' : 'success', isReviewMode ? 'Artist submitted for review' : 'Artist created');
      setShowCreateArtistModal(false);
    } catch (error: any) {
      if (error.response?.status === 409) {
        try {
          const existingRes = await api.get<Artist>(`/artists/by-name/${data.name}`);
          const existing = existingRes.data;
          const existingOpt = { id: existing.id, name: existing.name };
          
          setSelectedArtists(p => p.some(a => a.id === existingOpt.id) ? p : [...p, existingOpt]);
          showNotification('info', 'Artist already exists (possibly under review), selected.');
          setShowCreateArtistModal(false);
        } catch (fetchErr) {
          showNotification('error', 'Artist exists but could not be retrieved.');
        }
      } else {
        showNotification('error', 'Failed to create artist.');
      }
    }
  };

  const onSubmit = async (data: UploadForm) => {
    const formData = new FormData();
    formData.append('file', data.file[0]);
    
    // Use IDs instead of names
    selectedTags.forEach(tag => formData.append('tagIds', String(tag.id)));
    selectedArtists.forEach(artist => formData.append('artistIds', String(artist.id)));

    if (data.source) formData.append('source', data.source);
    formData.append('isNsfw', String(data.isNsfw));

    try {
      await api.post('/images/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } });
      showNotification(isReviewMode ? 'info' : 'success', isReviewMode ? 'Upload pending review.' : 'Image uploaded!');
      navigate('/gallery');
    } catch (e) { showNotification('error', 'Upload failed'); }
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
                    options={artists}
                    selectedOptions={selectedArtists}
                    onSelect={(o) => setSelectedArtists(p => p.some(a => a.id === o.id) ? p : [...p, o])}
                    onRemove={(o) => setSelectedArtists(p => p.filter(a => a.id !== o.id))}
                    onCreate={(name) => { setNewArtistName(name); setShowCreateArtistModal(true); }}
                    isMulti={true}
                />
                <SearchableSelect
                    label="Tags"
                    placeholder="Add tags..."
                    options={tags}
                    selectedOptions={selectedTags}
                    // FIX: Guard against duplicates here
                    onSelect={(o) => setSelectedTags(p => p.some(t => t.id === o.id) ? p : [...p, o])}
                    onRemove={(o) => setSelectedTags(p => p.filter(t => t.id !== o.id))}
                    onCreate={(name) => { setNewTagName(name); setShowCreateTagModal(true); }}
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
              <button type="submit" className="w-full py-4 rounded-xl bg-foreground text-background font-bold text-lg hover:opacity-90 transition-all shadow-lg">Upload Now</button>
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
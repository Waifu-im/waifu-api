import { useState, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useNavigate } from 'react-router-dom';
import api from '../services/api';
import SearchableSelect from '../components/SearchableSelect';
import { Tag, Artist, PaginatedList } from '../types'; // Import PaginatedList
import { UploadCloud } from 'lucide-react';
import { useNotification } from '../context/NotificationContext';
import { useAuth } from '../context/AuthContext';

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
  const { register, handleSubmit, watch, formState: { errors } } = useForm<UploadForm>();
  const navigate = useNavigate();
  const { showNotification } = useNotification();
  const { user } = useAuth();

  const [tags, setTags] = useState<Option[]>([]);
  const [artists, setArtists] = useState<Option[]>([]);
  const [selectedTags, setSelectedTags] = useState<Option[]>([]);
  const [selectedArtist, setSelectedArtist] = useState<Option | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);

  // Modals state
  const [showCreateTagModal, setShowCreateTagModal] = useState(false);
  const [newTagName, setNewTagName] = useState('');
  const [newTagDescription, setNewTagDescription] = useState('');

  const [showCreateArtistModal, setShowCreateArtistModal] = useState(false);
  const [newArtistName, setNewArtistName] = useState('');
  const [newArtistLinks, setNewArtistLinks] = useState({
    patreon: '', pixiv: '', twitter: '', deviantArt: ''
  });

  // Watch file input for preview
  const fileList = watch('file');

  useEffect(() => {
    if (fileList && fileList.length > 0) {
      const file = fileList[0];
      const url = URL.createObjectURL(file);
      setPreviewUrl(url);
      return () => URL.revokeObjectURL(url);
    } else {
      setPreviewUrl(null);
    }
  }, [fileList]);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [tagsRes, artistsRes] = await Promise.all([
          // Updated to handle PaginatedList response for tags
          // Fetching page 1 with large pageSize to get a good list for dropdown
          api.get<PaginatedList<Tag>>('/tags', { params: { pageSize: 1000 } }),
          api.get<PaginatedList<Artist>>('/artists?pageSize=1000')
        ]);

        // Map items from PaginatedList
        setTags(tagsRes.data.items.map(t => ({ id: t.id, name: t.name, description: t.description })));
        setArtists(artistsRes.data.items.map(a => ({ id: a.id, name: a.name })));
      } catch (e) { console.error(e); }
    };
    fetchData();
  }, []);

  const handleCreateTag = async () => {
    try {
      const { data } = await api.post<Tag>('/tags', { name: newTagName, description: newTagDescription });
      const newOpt = { id: data.id, name: data.name, description: data.description };
      setSelectedTags(p => [...p, newOpt]);
      showNotification('success', 'Tag created');
      setShowCreateTagModal(false);
      setNewTagName(''); setNewTagDescription('');
    } catch (err: any) {
      if(err.response?.status === 409) {
        const temp = { id: `temp-${Date.now()}`, name: newTagName, description: newTagDescription };
        setSelectedTags(p => [...p, temp]);
        setShowCreateTagModal(false);
      }
    }
  };

  const handleCreateArtist = async () => {
    try {
      const { data } = await api.post<Artist>('/artists', {
        name: newArtistName,
        ...newArtistLinks
      });
      const newOption = { id: data.id, name: data.name };
      setSelectedArtist(newOption);
      showNotification('success', 'Artist created');
      setShowCreateArtistModal(false);
      setNewArtistName('');
      setNewArtistLinks({ patreon: '', pixiv: '', twitter: '', deviantArt: '' });
    } catch (error: any) {
      if (error.response?.status === 409) {
        showNotification('info', 'Artist already exists, selected.');
        const existing = artists.find(a => a.name.toLowerCase() === newArtistName.toLowerCase())
            || { id: `temp-${Date.now()}`, name: newArtistName };
        setSelectedArtist(existing);
        setShowCreateArtistModal(false);
      } else {
        showNotification('error', 'Failed to create artist.');
      }
    }
  };

  const onSubmit = async (data: UploadForm) => {
    const formData = new FormData();
    formData.append('file', data.file[0]);
    selectedTags.forEach(tag => formData.append('tags', tag.name));
    if (selectedArtist) formData.append('artistName', selectedArtist.name);
    if (data.source) formData.append('source', data.source);
    formData.append('isNsfw', String(data.isNsfw));

    try {
      await api.post('/images/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } });
      showNotification('success', 'Uploaded successfully!');
      navigate('/gallery');
    } catch (e) { showNotification('error', 'Upload failed'); }
  };

  if (!user) return <div className="p-10 text-center">Please log in.</div>;

  return (
      <div className="max-w-5xl mx-auto p-4 md:p-8">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">

          {/* Left: Preview Area */}
          <div className="order-2 lg:order-1">
            <label className={`
            flex flex-col items-center justify-center w-full h-[500px] 
            border-2 border-dashed rounded-2xl cursor-pointer 
            transition-all duration-300 relative overflow-hidden group
            ${previewUrl ? 'border-primary bg-card' : 'border-border bg-secondary/30 hover:bg-secondary/50 hover:border-primary/50'}
          `}>
              {previewUrl ? (
                  <>
                    <img src={previewUrl} className="w-full h-full object-contain p-2" alt="Preview" />
                    <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center text-white font-medium">
                      Click to change image
                    </div>
                  </>
              ) : (
                  <div className="text-center p-6 text-muted-foreground">
                    <div className="w-16 h-16 bg-background rounded-full flex items-center justify-center mx-auto mb-4 shadow-sm">
                      <UploadCloud size={32} />
                    </div>
                    <p className="text-lg font-medium text-foreground">Click or drag image here</p>
                    <p className="text-sm mt-1">Supports PNG, JPG, GIF</p>
                  </div>
              )}
              <input type="file" accept="image/*" {...register('file', { required: true })} className="hidden" />
            </label>
            {errors.file && <p className="text-destructive text-center mt-2 font-medium">Image is required.</p>}
          </div>

          {/* Right: Form */}
          <div className="order-1 lg:order-2 space-y-6">
            <div>
              <h1 className="text-3xl font-black mb-2">Upload Image</h1>
              <p className="text-muted-foreground">Share your collection with the community.</p>
            </div>

            <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
              <div className="space-y-4">
                <SearchableSelect
                    label="Artist"
                    placeholder="Search artist..."
                    options={artists}
                    selectedOptions={selectedArtist ? [selectedArtist] : []}
                    onSelect={setSelectedArtist}
                    onRemove={() => setSelectedArtist(null)}
                    onCreate={(name) => { setNewArtistName(name); setShowCreateArtistModal(true); }}
                    isMulti={false}
                />

                <SearchableSelect
                    label="Tags"
                    placeholder="Add tags..."
                    options={tags}
                    selectedOptions={selectedTags}
                    onSelect={(o) => setSelectedTags(p => [...p, o])}
                    onRemove={(o) => setSelectedTags(p => p.filter(t => t.id !== o.id))}
                    onCreate={(name) => { setNewTagName(name); setShowCreateTagModal(true); }}
                    isMulti={true}
                />

                <div>
                  <label className="block text-sm font-medium mb-1.5">Source URL</label>
                  <input
                      type="url" {...register('source')}
                      className="w-full p-3 rounded-lg bg-secondary border-transparent focus:ring-2 focus:ring-primary outline-none transition-all"
                      placeholder="https://pixiv.net/..."
                  />
                </div>

                <label className="flex items-center gap-3 p-4 border border-border rounded-xl cursor-pointer hover:bg-secondary/50 transition-colors">
                  <input type="checkbox" {...register('isNsfw')} className="w-5 h-5 rounded text-destructive focus:ring-destructive" />
                  <div>
                    <span className="block font-bold">NSFW Content</span>
                    <span className="text-xs text-muted-foreground">Contains adult material</span>
                  </div>
                </label>
              </div>

              <button
                  type="submit"
                  className="w-full py-4 rounded-xl bg-foreground text-background font-bold text-lg hover:opacity-90 transition-all shadow-lg"
              >
                Upload Now
              </button>
            </form>
          </div>
        </div>

        {/* Modal Creating Tag */}
        {showCreateTagModal && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
              <div className="bg-card p-6 rounded-xl w-full max-w-sm border border-border shadow-xl">
                <h3 className="text-xl font-bold mb-4">New Tag</h3>
                <input value={newTagName} readOnly className="w-full p-2 mb-2 bg-secondary rounded" />
                <textarea
                    value={newTagDescription} onChange={e => setNewTagDescription(e.target.value)}
                    placeholder="Description..." className="w-full p-2 mb-4 bg-secondary rounded h-20"
                />
                <div className="flex gap-2">
                  <button onClick={handleCreateTag} className="flex-1 bg-primary text-primary-foreground py-2 rounded">Create</button>
                  <button onClick={() => setShowCreateTagModal(false)} className="px-4 py-2 bg-secondary rounded">Cancel</button>
                </div>
              </div>
            </div>
        )}

        {/* Modal Creating Artist */}
        {showCreateArtistModal && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
              <div className="bg-card p-6 rounded-xl w-full max-w-sm border border-border shadow-xl">
                <h3 className="text-xl font-bold mb-4">New Artist</h3>
                <div className="space-y-3">
                  <div>
                    <label className="text-xs font-bold text-muted-foreground uppercase">Name</label>
                    <input value={newArtistName} readOnly className="w-full p-2 bg-secondary rounded text-sm font-bold text-foreground mt-1" />
                  </div>

                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="text-xs font-bold text-muted-foreground uppercase">Twitter</label>
                      <input placeholder="@username" value={newArtistLinks.twitter} onChange={e => setNewArtistLinks({...newArtistLinks, twitter: e.target.value})} className="w-full p-2 bg-secondary rounded text-sm mt-1 focus:ring-1 focus:ring-primary outline-none"/>
                    </div>
                    <div>
                      <label className="text-xs font-bold text-muted-foreground uppercase">Pixiv</label>
                      <input placeholder="ID/Url" value={newArtistLinks.pixiv} onChange={e => setNewArtistLinks({...newArtistLinks, pixiv: e.target.value})} className="w-full p-2 bg-secondary rounded text-sm mt-1 focus:ring-1 focus:ring-primary outline-none"/>
                    </div>
                    <div>
                      <label className="text-xs font-bold text-muted-foreground uppercase">Patreon</label>
                      <input placeholder="Url" value={newArtistLinks.patreon} onChange={e => setNewArtistLinks({...newArtistLinks, patreon: e.target.value})} className="w-full p-2 bg-secondary rounded text-sm mt-1 focus:ring-1 focus:ring-primary outline-none"/>
                    </div>
                    <div>
                      <label className="text-xs font-bold text-muted-foreground uppercase">DeviantArt</label>
                      <input placeholder="Username" value={newArtistLinks.deviantArt} onChange={e => setNewArtistLinks({...newArtistLinks, deviantArt: e.target.value})} className="w-full p-2 bg-secondary rounded text-sm mt-1 focus:ring-1 focus:ring-primary outline-none"/>
                    </div>
                  </div>
                </div>
                <div className="flex gap-2 mt-6">
                  <button onClick={handleCreateArtist} className="flex-1 bg-primary text-primary-foreground py-2 rounded font-bold hover:opacity-90">Create</button>
                  <button onClick={() => setShowCreateArtistModal(false)} className="px-4 py-2 bg-secondary rounded font-bold hover:bg-secondary/80">Cancel</button>
                </div>
              </div>
            </div>
        )}
      </div>
  )
}

export default Upload
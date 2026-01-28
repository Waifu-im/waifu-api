import { useState, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useNavigate, useLocation } from 'react-router-dom';
import api from '../services/api';
import SearchableSelect from '../components/SearchableSelect';
import { Tag, Artist, PaginatedList } from '../types';
import { X } from 'lucide-react';
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
  const { register, handleSubmit, formState: { errors } } = useForm<UploadForm>();
  const navigate = useNavigate();
  const location = useLocation();
  const { showNotification } = useNotification();
  const { user } = useAuth();
  
  const [tags, setTags] = useState<Option[]>([]);
  const [artists, setArtists] = useState<Option[]>([]);
  const [selectedTags, setSelectedTags] = useState<Option[]>([]);
  const [selectedArtist, setSelectedArtist] = useState<Option | null>(null);
  
  const [showCreateTagModal, setShowCreateTagModal] = useState(false);
  const [newTagName, setNewTagName] = useState('');
  const [newTagDescription, setNewTagDescription] = useState('');
  
  const [showCreateArtistModal, setShowCreateArtistModal] = useState(false);
  const [newArtistName, setNewArtistName] = useState('');
  const [newArtistLinks, setNewArtistLinks] = useState({
    patreon: '',
    pixiv: '',
    twitter: '',
    deviantArt: ''
  });

  useEffect(() => {
    if (!user) {
      showNotification('warning', 'You must be logged in to upload images.');
      navigate('/login', { state: { from: location } });
      return;
    }

    const fetchData = async () => {
      try {
        const [tagsRes, artistsRes] = await Promise.all([
          api.get<Tag[]>('/tags'),
          api.get<PaginatedList<Artist>>('/artists?pageSize=1000') // Fetching a large batch for client-side filtering
        ]);
        
        setTags(tagsRes.data.map(t => ({ id: t.id, name: t.name, description: t.description })));
        setArtists(artistsRes.data.items.map(a => ({ id: a.id, name: a.name })));
      } catch (error) {
        console.error("Failed to fetch data", error);
        showNotification('error', 'Failed to load tags and artists.');
      }
    };
    fetchData();
  }, [user, navigate, showNotification, location]);

  const handleCreateTag = async () => {
    try {
      // Create tag via API
      const { data } = await api.post<Tag>('/tags', {
        name: newTagName,
        description: newTagDescription
      });
      
      const newOption = { id: data.id, name: data.name, description: data.description };
      
      // Add to selected tags
      setSelectedTags(prev => [...prev, newOption]);
      showNotification('success', `Tag "${data.name}" created successfully.`);
      
      // Close modal and reset
      setShowCreateTagModal(false);
      setNewTagName('');
      setNewTagDescription('');
    } catch (error: any) {
      if (error.response && error.response.status === 409) {
        showNotification('warning', `Tag "${newTagName}" already exists. Added to selection.`);
        
        // Check if it exists in loaded tags
        const existingTag = tags.find(t => t.name.toLowerCase() === newTagName.toLowerCase());
        if (existingTag) {
          setSelectedTags(prev => [...prev, existingTag]);
        } else {
          // If not in list (maybe pending), add as temporary option
          // We use name as ID or random ID since we don't have the real ID
          const tempOption = { id: `temp-${Date.now()}`, name: newTagName, description: newTagDescription };
          setSelectedTags(prev => [...prev, tempOption]);
        }
        
        setShowCreateTagModal(false);
        setNewTagName('');
        setNewTagDescription('');
      } else {
        console.error("Failed to create tag", error);
        showNotification('error', 'Failed to create tag.');
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
      
      // Set as selected artist
      setSelectedArtist(newOption);
      showNotification('success', `Artist "${data.name}" created successfully.`);
      
      // Close modal and reset
      setShowCreateArtistModal(false);
      setNewArtistName('');
      setNewArtistLinks({ patreon: '', pixiv: '', twitter: '', deviantArt: '' });
    } catch (error: any) {
      if (error.response && error.response.status === 409) {
        showNotification('warning', `Artist "${newArtistName}" already exists. Selected.`);
        
        const existingArtist = artists.find(a => a.name.toLowerCase() === newArtistName.toLowerCase());
        if (existingArtist) {
          setSelectedArtist(existingArtist);
        } else {
          const tempOption = { id: `temp-${Date.now()}`, name: newArtistName };
          setSelectedArtist(tempOption);
        }
        
        setShowCreateArtistModal(false);
        setNewArtistName('');
        setNewArtistLinks({ patreon: '', pixiv: '', twitter: '', deviantArt: '' });
      } else {
        console.error("Failed to create artist", error);
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
      await api.post('/images/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' }
      });
      showNotification('success', 'Image uploaded successfully!');
      navigate('/gallery');
    } catch (error: any) {
      console.error('Upload failed', error);
      if (error.response) {
        if (error.response.status === 401) {
          showNotification('error', 'Session expired. Please login again.');
          navigate('/login', { state: { from: location } });
        } else if (error.response.status === 409) {
          showNotification('error', 'This image has already been uploaded.');
        } else {
          showNotification('error', error.response.data?.detail || 'Upload failed. Please try again.');
        }
      } else {
        showNotification('error', 'Network error. Please check your connection.');
      }
    }
  };

  return (
    <div className="container mx-auto px-4 py-8">
      <div className="max-w-2xl mx-auto bg-card rounded-lg shadow-lg border border-border p-8">
        <h2 className="text-3xl font-bold mb-6 text-center">Upload Image</h2>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
          
          {/* File Input */}
          <div>
            <label className="block text-sm font-medium mb-2">Image File</label>
            <div className="flex items-center justify-center w-full">
              <label className="flex flex-col items-center justify-center w-full h-32 border-2 border-dashed border-input rounded-lg cursor-pointer bg-secondary/50 hover:bg-secondary transition-colors">
                <div className="flex flex-col items-center justify-center pt-5 pb-6">
                  <p className="mb-2 text-sm text-muted-foreground"><span className="font-semibold">Click to upload</span> or drag and drop</p>
                  <p className="text-xs text-muted-foreground">PNG, JPG, GIF up to 10MB</p>
                </div>
                <input 
                  type="file" 
                  accept="image/*"
                  {...register('file', { required: true })}
                  className="hidden" 
                />
              </label>
            </div>
            {errors.file && <span className="text-destructive text-sm mt-1">File is required</span>}
          </div>

          {/* Artist Selection */}
          <div>
            <SearchableSelect
              label="Artist (Optional)"
              placeholder="Search or create artist..."
              options={artists}
              selectedOptions={selectedArtist ? [selectedArtist] : []}
              onSelect={(option) => setSelectedArtist(option)}
              onRemove={() => setSelectedArtist(null)}
              onCreate={(name) => {
                setNewArtistName(name);
                setShowCreateArtistModal(true);
              }}
              isMulti={false}
            />
          </div>

          {/* Source URL */}
          <div>
            <label className="block text-sm font-medium mb-2">Source URL (Optional)</label>
            <input 
              type="url" 
              {...register('source')}
              className="w-full p-2 border border-input rounded-md bg-background focus:ring-2 focus:ring-primary focus:border-transparent outline-none"
              placeholder="https://..."
            />
          </div>

          {/* Tags Selection */}
          <div>
            <SearchableSelect
              label="Tags"
              placeholder="Search or create tags..."
              options={tags}
              selectedOptions={selectedTags}
              onSelect={(option) => setSelectedTags(prev => [...prev, option])}
              onRemove={(option) => setSelectedTags(prev => prev.filter(t => t.id !== option.id))}
              onCreate={(name) => {
                setNewTagName(name);
                setShowCreateTagModal(true);
              }}
              isMulti={true}
            />
          </div>

          {/* NSFW Checkbox */}
          <div className="flex items-center p-4 border border-input rounded-md bg-secondary/20">
            <input 
              type="checkbox" 
              id="nsfw" 
              {...register('isNsfw')}
              className="w-4 h-4 text-primary border-input rounded focus:ring-primary" 
            />
            <label htmlFor="nsfw" className="ml-2 block text-sm font-medium">
              This image contains NSFW content
            </label>
          </div>

          <button 
            type="submit" 
            className="w-full bg-primary text-primary-foreground py-3 rounded-md hover:bg-primary/90 transition font-bold text-lg shadow-md"
          >
            Upload Image
          </button>
        </form>
      </div>

      {/* Create Tag Modal */}
      {showCreateTagModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm p-4">
          <div className="bg-card p-6 rounded-lg shadow-xl max-w-md w-full border border-border">
            <div className="flex justify-between items-center mb-4">
              <h3 className="text-xl font-bold">Create New Tag</h3>
              <button onClick={() => setShowCreateTagModal(false)} className="p-1 hover:bg-accent rounded-full"><X size={20} /></button>
            </div>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-1">Name</label>
                <input 
                  type="text" 
                  value={newTagName} 
                  onChange={(e) => setNewTagName(e.target.value)}
                  className="w-full p-2 border border-input rounded-md bg-background"
                  disabled
                />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1">Description</label>
                <textarea 
                  value={newTagDescription} 
                  onChange={(e) => setNewTagDescription(e.target.value)}
                  className="w-full p-2 border border-input rounded-md bg-background h-24 resize-none"
                  placeholder="Describe this tag..."
                />
              </div>
              <button 
                onClick={handleCreateTag}
                className="w-full bg-primary text-primary-foreground py-2 rounded-md hover:bg-primary/90 transition"
              >
                Create Tag
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Create Artist Modal */}
      {showCreateArtistModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm p-4">
          <div className="bg-card p-6 rounded-lg shadow-xl max-w-md w-full border border-border">
            <div className="flex justify-between items-center mb-4">
              <h3 className="text-xl font-bold">Create New Artist</h3>
              <button onClick={() => setShowCreateArtistModal(false)} className="p-1 hover:bg-accent rounded-full"><X size={20} /></button>
            </div>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-1">Name</label>
                <input 
                  type="text" 
                  value={newArtistName} 
                  onChange={(e) => setNewArtistName(e.target.value)}
                  className="w-full p-2 border border-input rounded-md bg-background"
                  disabled
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium mb-1">Twitter</label>
                  <input 
                    type="text" 
                    value={newArtistLinks.twitter} 
                    onChange={(e) => setNewArtistLinks({...newArtistLinks, twitter: e.target.value})}
                    className="w-full p-2 border border-input rounded-md bg-background text-sm"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium mb-1">Pixiv</label>
                  <input 
                    type="text" 
                    value={newArtistLinks.pixiv} 
                    onChange={(e) => setNewArtistLinks({...newArtistLinks, pixiv: e.target.value})}
                    className="w-full p-2 border border-input rounded-md bg-background text-sm"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium mb-1">Patreon</label>
                  <input 
                    type="text" 
                    value={newArtistLinks.patreon} 
                    onChange={(e) => setNewArtistLinks({...newArtistLinks, patreon: e.target.value})}
                    className="w-full p-2 border border-input rounded-md bg-background text-sm"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium mb-1">DeviantArt</label>
                  <input 
                    type="text" 
                    value={newArtistLinks.deviantArt} 
                    onChange={(e) => setNewArtistLinks({...newArtistLinks, deviantArt: e.target.value})}
                    className="w-full p-2 border border-input rounded-md bg-background text-sm"
                  />
                </div>
              </div>
              <button 
                onClick={handleCreateArtist}
                className="w-full bg-primary text-primary-foreground py-2 rounded-md hover:bg-primary/90 transition"
              >
                Create Artist
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default Upload

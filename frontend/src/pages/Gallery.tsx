import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useImages } from '../hooks/useImages';
import { Filter, X, AlertTriangle } from 'lucide-react';
import ImageCard from '../components/ImageCard';
import api from '../services/api';
import { useAuth } from '../context/AuthContext';

const Gallery = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const [showNsfwWarning, setShowNsfwWarning] = useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const { user } = useAuth();

  // Filters state
  const isNsfw = searchParams.get('isNsfw') || 'false';
  const orderBy = searchParams.get('orderBy') || 'UPLOADED_AT';
  const includedTags = searchParams.getAll('includedTags');

  // Check NSFW consent
  useEffect(() => {
    if (isNsfw === 'true') {
      const hasConsented = localStorage.getItem('nsfw-consent');
      if (!hasConsented) {
        setShowNsfwWarning(true);
      }
    }
  }, [isNsfw]);

  const handleNsfwConsent = () => {
    localStorage.setItem('nsfw-consent', 'true');
    setShowNsfwWarning(false);
  };

  const handleNsfwReject = () => {
    setSearchParams(prev => {
      prev.set('isNsfw', 'false');
      return prev;
    });
    setShowNsfwWarning(false);
  };

  const { data: images, isLoading, error, refetch } = useImages({
    isNsfw: isNsfw === 'true' ? 1 : isNsfw === 'null' ? 2 : 0,
    orderBy,
    includedTags,
    limit: 30
  });

  const updateFilter = (key: string, value: string) => {
    setSearchParams(prev => {
      if (value) prev.set(key, value);
      else prev.delete(key);
      return prev;
    });
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to delete this image?')) return;
    try {
      await api.delete(`/images/${id}`);
      refetch();
    } catch (err) {
      console.error('Failed to delete image', err);
      alert('Failed to delete image');
    }
  };

  return (
    <div className="container mx-auto px-4 py-8">
      <div className="flex flex-col md:flex-row gap-6">
        {/* Mobile Filter Button */}
        <div className="md:hidden flex justify-end mb-4">
          <button 
            onClick={() => setIsSidebarOpen(true)}
            className="flex items-center gap-2 px-4 py-2 bg-card border border-border rounded-lg shadow-sm hover:bg-accent transition-colors"
          >
            <Filter size={20} /> Filters
          </button>
        </div>

        {/* Sidebar Filters */}
        <aside className={`
          fixed inset-y-0 left-0 z-40 w-72 bg-card border-r border-border shadow-lg transform transition-transform duration-300 ease-in-out
          md:relative md:translate-x-0 md:shadow-none md:bg-transparent md:w-64 md:block md:border-none
          ${isSidebarOpen ? 'translate-x-0' : '-translate-x-full'}
        `}>
          <div className="p-6 h-full overflow-y-auto md:p-0">
            <div className="flex justify-between items-center mb-6 md:hidden">
              <h2 className="text-xl font-bold">Filters</h2>
              <button onClick={() => setIsSidebarOpen(false)} className="p-2 hover:bg-accent rounded-full"><X size={24} /></button>
            </div>

            <div className="space-y-6 sticky top-24">
              {/* NSFW Toggle */}
              <div>
                <label className="block text-sm font-medium mb-2">Content Rating</label>
                <select 
                  value={isNsfw} 
                  onChange={(e) => updateFilter('isNsfw', e.target.value)}
                  className="w-full p-2 border border-input rounded-md bg-background focus:ring-2 focus:ring-primary focus:border-transparent outline-none"
                >
                  <option value="false">Safe</option>
                  <option value="true">NSFW (18+)</option>
                  <option value="null">All</option>
                </select>
              </div>

              {/* Order By */}
              <div>
                <label className="block text-sm font-medium mb-2">Sort By</label>
                <select 
                  value={orderBy} 
                  onChange={(e) => updateFilter('orderBy', e.target.value)}
                  className="w-full p-2 border border-input rounded-md bg-background focus:ring-2 focus:ring-primary focus:border-transparent outline-none"
                >
                  <option value="UPLOADED_AT">Recent</option>
                  <option value="FAVORITES">Top Favorites</option>
                  <option value="RANDOM">Random</option>
                </select>
              </div>

              {/* Tags (Simple Input for now - will be improved later) */}
              <div>
                <label className="block text-sm font-medium mb-2">Tags</label>
                <div className="flex flex-wrap gap-2 mb-2">
                  {includedTags.map(tag => (
                    <span key={tag} className="bg-secondary text-secondary-foreground px-2 py-1 rounded-full text-xs flex items-center gap-1">
                      {tag}
                      <button onClick={() => {
                        const newTags = includedTags.filter(t => t !== tag);
                        setSearchParams(prev => {
                          prev.delete('includedTags');
                          newTags.forEach(t => prev.append('includedTags', t));
                          return prev;
                        });
                      }} className="hover:text-destructive"><X size={12} /></button>
                    </span>
                  ))}
                </div>
                <input 
                  type="text" 
                  placeholder="Add tag..." 
                  className="w-full p-2 border border-input rounded-md bg-background focus:ring-2 focus:ring-primary focus:border-transparent outline-none"
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      const val = e.currentTarget.value.trim();
                      if (val && !includedTags.includes(val)) {
                        setSearchParams(prev => {
                          prev.append('includedTags', val);
                          return prev;
                        });
                        e.currentTarget.value = '';
                      }
                    }
                  }}
                />
              </div>
              
              <button 
                onClick={() => refetch()}
                className="w-full bg-primary text-primary-foreground py-2 rounded-md hover:bg-primary/90 transition shadow-sm"
              >
                Apply Filters
              </button>
            </div>
          </div>
        </aside>

        {/* Overlay for mobile sidebar */}
        {isSidebarOpen && (
          <div 
            className="fixed inset-0 bg-background/80 backdrop-blur-sm z-30 md:hidden"
            onClick={() => setIsSidebarOpen(false)}
          />
        )}

        {/* Image Grid */}
        <div className="flex-1">
          {isLoading ? (
            <div className="columns-1 sm:columns-2 lg:columns-3 xl:columns-4 gap-4 space-y-4">
              {[...Array(8)].map((_, i) => (
                <div key={i} className="bg-muted h-64 rounded-lg animate-pulse break-inside-avoid"></div>
              ))}
            </div>
          ) : error ? (
            <div className="text-center py-20 text-destructive bg-destructive/10 rounded-lg border border-destructive/20">
              <p className="font-medium">Failed to load images.</p>
              <button onClick={() => refetch()} className="mt-4 text-sm underline hover:text-destructive/80">Try Again</button>
            </div>
          ) : images && images.length > 0 ? (
            <div className="columns-1 sm:columns-2 lg:columns-3 xl:columns-4 gap-4 space-y-4">
              {images.map((image) => (
                <ImageCard key={image.id} image={image} onDelete={handleDelete} />
              ))}
            </div>
          ) : (
             <div className="text-center py-20 text-muted-foreground bg-card rounded-lg border border-border">
              <p className="text-lg">No images found matching your criteria.</p>
              <button 
                onClick={() => {
                  setSearchParams({});
                  refetch();
                }} 
                className="mt-4 text-primary hover:underline"
              >
                Clear Filters
              </button>
            </div>
          )}
        </div>
      </div>

      {/* NSFW Warning Modal */}
      {showNsfwWarning && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm p-4">
          <div className="bg-card p-8 rounded-2xl max-w-md w-full shadow-2xl border border-destructive/20 text-center">
            <div className="w-16 h-16 bg-destructive/10 rounded-full flex items-center justify-center text-destructive mx-auto mb-6">
              <AlertTriangle size={32} />
            </div>
            <h2 className="text-2xl font-bold mb-4">Age Verification</h2>
            <p className="text-muted-foreground mb-8">
              This page contains NSFW (Not Safe For Work) content. 
              You must be 18 years or older to view this content.
            </p>
            <div className="flex flex-col gap-3">
              <button 
                onClick={handleNsfwConsent}
                className="w-full bg-destructive hover:bg-destructive/90 text-destructive-foreground py-3 rounded-lg font-bold transition-colors"
              >
                I am 18 or older - Enter
              </button>
              <button 
                onClick={handleNsfwReject}
                className="w-full bg-secondary hover:bg-secondary/80 text-secondary-foreground py-3 rounded-lg font-medium transition-colors"
              >
                Go Back to Safe Mode
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default Gallery;

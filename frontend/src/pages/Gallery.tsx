import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useImages } from '../hooks/useImages';
import { X, SlidersHorizontal, RefreshCw, Trash2, Search } from 'lucide-react';
import ImageCard from '../components/ImageCard';
import api from '../services/api';
import SearchableSelect from '../components/SearchableSelect';
import Modal from '../components/Modal';
import { PaginatedList, Tag } from '../types';

const Gallery = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const [showFilters, setShowFilters] = useState(false);
  const [showNsfwWarning, setShowNsfwWarning] = useState(false);

  // Delete State
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [imageToDelete, setImageToDelete] = useState<number | null>(null);

  const isNsfw = searchParams.get('isNsfw') || 'false';
  const orderBy = searchParams.get('orderBy') || 'UPLOADED_AT';
  const includedTags = searchParams.getAll('includedTags');

  // Updated hook usage with pageSize
  const { data: paginatedImages, isLoading, error, refetch } = useImages({
    isNsfw: isNsfw === 'true' ? 1 : isNsfw === 'null' ? 2 : 0,
    orderBy,
    includedTags,
    page: 1,      // Defaulting to page 1 for now (infinite scroll/pagination UI can be added later)
    pageSize: 50  // Replaced limit with pageSize
  });

  const images = paginatedImages?.items || [];

  useEffect(() => {
    if (isNsfw === 'true' && !localStorage.getItem('nsfw-consent')) {
      setShowNsfwWarning(true);
    }
  }, [isNsfw]);

  const updateFilter = (key: string, value: string) => {
    setSearchParams(prev => {
      if (value) prev.set(key, value); else prev.delete(key);
      return prev;
    });
  };

  const handleTagInput = (name: string) => {
    if(!includedTags.includes(name)) {
      setSearchParams(prev => { prev.append('includedTags', name); return prev; });
    }
  };

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
    } catch(e) { alert("Failed to delete."); }
  };

  const sortOptions = [
    { id: 'UPLOADED_AT', name: 'Newest First' },
    { id: 'FAVORITES', name: 'Most Popular' },
    { id: 'RANDOM', name: 'Random Shuffle' }
  ];

  const ratingOptions = [
    { id: 'false', name: 'Safe' },
    { id: 'true', name: 'NSFW (18+)' },
    { id: 'null', name: 'All Content' }
  ];

  const [allTags, setAllTags] = useState<{id: number, name: string}[]>([]);

  useEffect(() => {
    // Fetch tags with high pageSize to get list for dropdown
    api.get<PaginatedList<Tag>>('/tags', { params: { pageSize: 1000 } })
        .then(res => setAllTags(res.data.items))
        .catch(console.error);
  }, []);

  return (
      <div className="flex h-full relative overflow-hidden">

        {/* CONTENT GRID */}
        <div className="flex-1 overflow-y-auto h-full p-4">
          <div className="flex justify-between items-center mb-6">
            <h2 className="text-2xl font-black tracking-tight text-foreground">Gallery</h2>
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
          ) : images && images.length > 0 ? (
              <div className="columns-2 sm:columns-3 md:columns-4 xl:columns-5 gap-4 space-y-4 pb-10">
                {images.map(img => (
                    <div key={img.id} className="break-inside-avoid">
                      <ImageCard image={img} onDelete={confirmDelete} />
                    </div>
                ))}
              </div>
          ) : (
              <div className="flex flex-col items-center justify-center h-[50vh] text-muted-foreground">
                <Search size={48} className="mb-4 opacity-20" />
                <p className="text-lg font-medium">No images found.</p>
                <p className="text-sm">Try adjusting your filters.</p>
                <button
                    onClick={() => { setSearchParams({}); refetch(); }}
                    className="mt-6 px-6 py-2 bg-secondary text-foreground rounded-lg font-bold hover:bg-secondary/80"
                >
                  Clear All Filters
                </button>
              </div>
          )}
        </div>

        {/* FILTER SIDEBAR - SAME AS BEFORE */}
        <aside className={`
        fixed inset-y-0 right-0 z-40 w-80 max-w-[85vw] bg-card border-l border-border shadow-2xl 
        transform transition-transform duration-300 ease-in-out h-full flex flex-col
        ${showFilters ? 'translate-x-0' : 'translate-x-full'}
        md:relative md:shadow-none md:translate-x-0
        ${!showFilters && 'hidden md:block md:w-0 md:border-none md:overflow-hidden'} 
      `}>
          <div className="p-5 border-b border-border flex justify-between items-center bg-card/95 backdrop-blur">
            <h3 className="font-bold">Filters</h3>
            <button onClick={() => setShowFilters(false)} className="md:hidden"><X size={20}/></button>
          </div>

          <div className="flex-1 p-5 overflow-y-auto space-y-6 w-80">
            <SearchableSelect
                label="Sort By"
                options={sortOptions}
                selectedOptions={sortOptions.filter(o => o.id === orderBy)}
                onSelect={(o) => updateFilter('orderBy', o.id as string)}
                onRemove={() => {}}
                isMulti={false}
                clearable={false}
            />

            <SearchableSelect
                label="Content Rating"
                options={ratingOptions}
                selectedOptions={ratingOptions.filter(o => o.id === isNsfw)}
                onSelect={(o) => updateFilter('isNsfw', o.id as string)}
                onRemove={() => {}}
                isMulti={false}
                clearable={false}
            />

            <SearchableSelect
                label="Filter by Tags"
                placeholder="Search tags..."
                options={allTags}
                selectedOptions={includedTags.map(t => ({ id: t, name: t }))}
                onSelect={(o) => handleTagInput(o.name)}
                onRemove={(o) => {
                  const newTags = includedTags.filter(t => t !== o.name);
                  setSearchParams(prev => { prev.delete('includedTags'); newTags.forEach(t => prev.append('includedTags', t)); return prev; });
                }}
                isMulti={true}
                clearable={true}
            />
          </div>

          <div className="p-4 border-t border-border md:hidden">
            <button onClick={() => setShowFilters(false)} className="w-full py-3 bg-primary text-primary-foreground rounded-xl font-bold">Done</button>
          </div>
        </aside>

        {/* Admin Delete Modal (Same as before) */}
        <Modal isOpen={isDeleteModalOpen} onClose={() => setIsDeleteModalOpen(false)} title="Delete Image">
          <div className="text-center space-y-4">
            <div className="w-16 h-16 bg-destructive/10 text-destructive rounded-full flex items-center justify-center mx-auto">
              <Trash2 size={32} />
            </div>
            <p className="text-muted-foreground">Are you sure you want to permanently delete this image?</p>
            <div className="flex gap-3 mt-6">
              <button onClick={() => setIsDeleteModalOpen(false)} className="flex-1 py-3 bg-secondary rounded-lg font-bold">Cancel</button>
              <button onClick={handleRealDelete} className="flex-1 py-3 bg-destructive text-destructive-foreground rounded-lg font-bold">Delete</button>
            </div>
          </div>
        </Modal>

        {/* NSFW Modal (Same as before) */}
        {showNsfwWarning && (
            <div className="fixed inset-0 z-[60] bg-black/90 flex items-center justify-center p-4 backdrop-blur-md">
              <div className="bg-card p-8 rounded-2xl max-w-sm w-full text-center border border-border shadow-2xl">
                <h2 className="text-2xl font-black mb-2">Age Restricted</h2>
                <p className="text-muted-foreground mb-6">This content is intended for adults only.</p>
                <div className="grid gap-3">
                  <button onClick={() => { localStorage.setItem('nsfw-consent', 'true'); setShowNsfwWarning(false); }} className="w-full py-3 bg-primary text-primary-foreground rounded-xl font-bold">I am 18+</button>
                  <button onClick={() => { updateFilter('isNsfw', 'false'); setShowNsfwWarning(false); }} className="w-full py-3 bg-secondary rounded-xl font-bold">Go Back</button>
                </div>
              </div>
            </div>
        )}
      </div>
  );
};

export default Gallery;
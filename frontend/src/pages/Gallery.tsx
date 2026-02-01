import { useState, useEffect } from 'react';
import { useImages } from '../hooks/useImages';
import { GalleryLayout } from '../components/layout/GalleryLayout';
import { useGalleryParams } from '../hooks/useGalleryParams';
import { useAlternativeSearch } from '../hooks/useAlternativeSearch';
import EmptyState from '../components/EmptyState';

const Gallery = () => {
    const { filters, page, setPage, searchParams, setSearchParams, isNsfw } = useGalleryParams();
    const [showNsfwWarning, setShowNsfwWarning] = useState(false);

    const { data: paginatedImages, isLoading, error, refetch } = useImages(filters);
    
    const { alternativeCount, isCheckingAlternatives } = useAlternativeSearch(
        isLoading,
        paginatedImages?.items,
        isNsfw,
        filters
    );

    useEffect(() => {
        if (isNsfw !== '0' && !localStorage.getItem('nsfw-consent')) setShowNsfwWarning(true);
    }, [isNsfw]);

    const handleEnableNsfw = () => {
        setSearchParams(prev => {
            prev.set('isNsfw', '2');
            return prev;
        });
    };

    return (
        <>
            <GalleryLayout
                images={paginatedImages?.items || []}
                isLoading={isLoading}
                error={error}
                refetch={refetch}
                page={page}
                totalPages={paginatedImages?.totalPages || 1}
                setPage={setPage}
                searchParams={searchParams}
                setSearchParams={setSearchParams}
                emptyState={
                    <EmptyState
                        isCheckingAlternatives={isCheckingAlternatives}
                        alternativeCount={alternativeCount}
                        onEnableNsfw={handleEnableNsfw}
                        onClearFilters={() => { setSearchParams(new URLSearchParams()); refetch(); }}
                    />
                }
            />

            {showNsfwWarning && (
                <div className="fixed inset-0 z-[60] bg-black/90 flex items-center justify-center p-4 backdrop-blur-md">
                    <div className="bg-card p-8 rounded-2xl max-w-sm w-full text-center border border-border shadow-2xl">
                        <h2 className="text-2xl font-black mb-2">Age Restricted</h2>
                        <button onClick={() => { localStorage.setItem('nsfw-consent', 'true'); setShowNsfwWarning(false); }} className="w-full py-3 bg-primary text-primary-foreground rounded-xl font-bold mb-2">I am 18+</button>
                        <button onClick={() => { setSearchParams(prev => { prev.set('isNsfw', '0'); return prev; }); setShowNsfwWarning(false); }} className="w-full py-3 bg-secondary rounded-xl font-bold">Go Back</button>
                    </div>
                </div>
            )}
        </>
    );
};

export default Gallery;
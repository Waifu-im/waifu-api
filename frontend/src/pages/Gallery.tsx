import { useEffect } from 'react';
import { useImages } from '../hooks/useImages';
import { GalleryLayout } from '../components/layout/GalleryLayout';
import { useGalleryParams } from '../hooks/useGalleryParams';
import { useAlternativeSearch } from '../hooks/useAlternativeSearch';
import { useNsfwConsent } from '../hooks/useNsfwConsent';
import EmptyState from '../components/EmptyState';
import NsfwWarning from '../components/NsfwWarning';

const Gallery = () => {
    const { filters, page, setPage, searchParams, setSearchParams, isNsfw } = useGalleryParams();
    const { showWarning: showNsfwWarning, setShowWarning: setShowNsfwWarning, grantConsent, hasConsent } = useNsfwConsent(false);

    const { data: paginatedImages, isLoading, error, refetch } = useImages(filters);

    const { alternativeCount, isCheckingAlternatives } = useAlternativeSearch(
        isLoading,
        paginatedImages?.items,
        isNsfw,
        filters
    );

    useEffect(() => {
        if (isNsfw !== '0' && !hasConsent()) setShowNsfwWarning(true);
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
                <NsfwWarning
                    variant="modal"
                    onConsent={grantConsent}
                    onDecline={() => {
                        setSearchParams(prev => { prev.set('isNsfw', '0'); return prev; });
                        setShowNsfwWarning(false);
                    }}
                />
            )}
        </>
    );
};

export default Gallery;
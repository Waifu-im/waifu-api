import { ReactNode } from 'react';
import { ImageDto, Role } from '../types';
import ImageCard from './ImageCard';
import { RefreshCw, ChevronLeft, ChevronRight } from 'lucide-react';
import { useAuth } from '../context/AuthContext';

export interface ImageGridProps {
    images: ImageDto[];
    isLoading: boolean;
    error?: any;
    onRetry?: () => void;
    page: number;
    totalPages: number;
    setPage: (page: number) => void;
    onEdit?: (image: ImageDto) => void;
    onDelete?: (id: number) => void;
    onRemove?: (id: number) => void;
    emptyState?: ReactNode;
    forceOverlay?: boolean;
}

const ImageGrid = ({ 
    images, isLoading, error, onRetry, 
    page, totalPages, setPage, 
    onEdit, onDelete, onRemove,
    emptyState, forceOverlay = false
}: ImageGridProps) => {
    const { user } = useAuth();
    const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);
    const isAdmin = user && user.role === Role.Admin;

    if (isLoading) {
        return (
            <div className="columns-2 sm:columns-3 md:columns-4 xl:columns-5 gap-4 space-y-4">
                {[...Array(12)].map((_,i) => <div key={i} className="bg-muted h-64 rounded-xl animate-pulse break-inside-avoid"></div>)}
            </div>
        );
    }

    if (error) {
        return (
            <div className="flex flex-col items-center justify-center h-64 text-muted-foreground">
                <p className="mb-4">Unable to load images.</p>
                {onRetry && <button onClick={onRetry} className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg"><RefreshCw size={16}/> Retry</button>}
            </div>
        );
    }

    if (images.length === 0) {
        return emptyState || (
            <div className="flex flex-col items-center justify-center h-64 text-muted-foreground">
                <p>No images found.</p>
            </div>
        );
    }

    return (
        <>
            <div className="columns-2 sm:columns-3 md:columns-4 xl:columns-5 gap-4 space-y-4 pb-10">
                {images.map(img => (
                    <div key={img.id} className="break-inside-avoid relative group">
                        <ImageCard
                            image={img}
                            onDelete={isAdmin ? onDelete : undefined}
                            onRemove={onRemove}
                            onEdit={isAdminOrModerator ? onEdit : undefined}
                            forceOverlay={forceOverlay}
                        />
                    </div>
                ))}
            </div>

            {totalPages > 1 && (
                <div className="flex justify-center items-center gap-4 mt-10 pb-10">
                    <button
                        disabled={page === 1}
                        onClick={() => setPage(Math.max(1, page - 1))}
                        className="p-2 rounded-full bg-secondary disabled:opacity-50 hover:bg-secondary/80"
                    >
                        <ChevronLeft size={20} />
                    </button>
                    <span className="text-sm font-bold">Page {page} of {totalPages}</span>
                    <button
                        disabled={page === totalPages}
                        onClick={() => setPage(Math.min(totalPages, page + 1))}
                        className="p-2 rounded-full bg-secondary disabled:opacity-50 hover:bg-secondary/80"
                    >
                        <ChevronRight size={20} />
                    </button>
                </div>
            )}
        </>
    );
};

export default ImageGrid;
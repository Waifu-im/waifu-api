import { ReactNode, useState, useEffect } from 'react';
import { ImageDto, Role } from '../types';
import ImageCard from './ImageCard';
import { RefreshCw } from 'lucide-react';
import { useAuth } from '../context/AuthContext';

export interface ImageGridProps {
    images: ImageDto[];
    isLoading: boolean;
    error?: any;
    onRetry?: () => void;
    onEdit?: (image: ImageDto) => void;
    onDelete?: (id: number) => void;
    onRemove?: (id: number) => void;
    emptyState?: ReactNode;
    forceOverlay?: boolean;
    readOnly?: boolean;
}

const getColumnCount = (width: number) => {
    if (width >= 1280) return 5;
    if (width >= 1024) return 4;
    if (width >= 768) return 3;
    if (width >= 640) return 2;
    return 1;
};

const useColumnCount = () => {
    const [count, setCount] = useState(() =>
        typeof window !== 'undefined' ? getColumnCount(window.innerWidth) : 5
    );
    useEffect(() => {
        const handler = () => setCount(getColumnCount(window.innerWidth));
        window.addEventListener('resize', handler);
        return () => window.removeEventListener('resize', handler);
    }, []);
    return count;
};

const ImageGrid = ({
                       images, isLoading, error, onRetry,
                       onEdit, onDelete, onRemove,
                       emptyState, forceOverlay = false, readOnly = false
                   }: ImageGridProps) => {
    const { user } = useAuth();
    const colCount = useColumnCount();
    const isAdminOrModerator = user && (user.role === Role.Admin || user.role === Role.Moderator);
    const isAdmin = user && user.role === Role.Admin;

    if (isLoading) {
        return (
            <div className="flex gap-4">
                {Array.from({ length: colCount }, (_, c) => (
                    <div key={c} className="flex-1 flex flex-col gap-4 min-w-0">
                        {[...Array(3)].map((_, i) => (
                            <div key={i} className="bg-muted h-64 rounded-xl animate-pulse"></div>
                        ))}
                    </div>
                ))}
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

    // Distribute images round-robin into N columns so reading order is row-by-row.
    // Item index i goes to column (i % colCount). With uniform heights this looks like a grid;
    // with varied heights items stack naturally with no balancing artifacts.
    const columns: ImageDto[][] = Array.from({ length: colCount }, () => []);
    images.forEach((img, i) => columns[i % colCount].push(img));

    return (
        <div className="flex gap-4 pb-10 items-start">
            {columns.map((column, c) => (
                <div key={c} className="flex-1 flex flex-col gap-4 min-w-0">
                    {column.map(img => (
                        <ImageCard
                            key={img.id}
                            image={img}
                            onDelete={isAdmin ? onDelete : undefined}
                            onRemove={onRemove}
                            onEdit={isAdminOrModerator ? onEdit : undefined}
                            forceOverlay={forceOverlay}
                            readOnly={readOnly}
                        />
                    ))}
                </div>
            ))}
        </div>
    );
};

export default ImageGrid;

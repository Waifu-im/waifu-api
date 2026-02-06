import { Search } from 'lucide-react';
import { ReactNode } from 'react';

interface EmptyStateProps {
    isCheckingAlternatives: boolean;
    alternativeCount: number | null;
    onEnableNsfw: () => void;
    onClearFilters: () => void;
    customEmptyMessage?: ReactNode;
}

const EmptyState = ({
                        isCheckingAlternatives,
                        alternativeCount,
                        onEnableNsfw,
                        onClearFilters,
                        customEmptyMessage
                    }: EmptyStateProps) => {
    return (
        <div className="flex flex-col items-center justify-center h-[50vh] text-muted-foreground">
            <Search size={48} className="mb-4 opacity-20" />
            <p className="text-lg font-medium">No images found.</p>

            {isCheckingAlternatives ? (
                <p className="text-sm mt-2 animate-pulse">Searching for alternatives...</p>
            ) : alternativeCount !== null && alternativeCount > 0 ? (
                <div className="mt-6 flex flex-col items-center animate-in fade-in slide-in-from-bottom-4 duration-500">
                    <p className="text-sm mb-3 max-w-xs text-center">
                        We found <span className="font-bold text-primary">{alternativeCount}</span> images that match your search but are hidden by the NSFW filter.
                    </p>
                    <button
                        onClick={onEnableNsfw}
                        className="px-6 py-2 bg-primary text-primary-foreground rounded-lg font-bold hover:opacity-90 transition-all shadow-lg shadow-primary/20"
                    >
                        Include NSFW Content
                    </button>
                </div>
            ) : (
                <>
                    {customEmptyMessage ? (
                        <div className="mt-4">{customEmptyMessage}</div>
                    ) : (
                        <button
                            onClick={onClearFilters}
                            className="mt-6 px-6 py-2 bg-secondary text-foreground rounded-lg font-bold"
                        >
                            Clear All Filters
                        </button>
                    )}
                </>
            )}
        </div>
    );
};

export default EmptyState;

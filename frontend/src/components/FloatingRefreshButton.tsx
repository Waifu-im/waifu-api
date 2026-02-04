import { RefreshCw } from 'lucide-react';

interface FloatingRefreshButtonProps {
    onRefresh: () => void;
}

const FloatingRefreshButton = ({ onRefresh }: FloatingRefreshButtonProps) => {
    return (
        <button
            onClick={onRefresh}
            className="absolute bottom-8 right-8 z-40 p-4 rounded-full shadow-2xl transition-all hover:scale-110 active:scale-95 border border-white dark:border-zinc-900 bg-zinc-900 text-white dark:bg-white dark:text-black"
            title="Refresh"
        >
            <RefreshCw size={28} />
        </button>
    );
};

export default FloatingRefreshButton;
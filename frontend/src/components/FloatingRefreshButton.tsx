import { useState, useEffect } from 'react';
import { RefreshCw } from 'lucide-react';

interface FloatingRefreshButtonProps {
    onRefresh: () => void;
}

const FloatingRefreshButton = ({ onRefresh }: FloatingRefreshButtonProps) => {
    const [bottomOffset, setBottomOffset] = useState(24);

    useEffect(() => {
        const vv = window.visualViewport;
        if (!vv) return;

        const update = () => {
            // The difference between layout viewport and visual viewport
            // gives us the height of the browser's bottom toolbar
            const toolbarHeight = window.innerHeight - vv.height - vv.offsetTop;
            setBottomOffset(Math.max(24, toolbarHeight + 24));
        };

        vv.addEventListener('resize', update);
        vv.addEventListener('scroll', update);
        update();

        return () => {
            vv.removeEventListener('resize', update);
            vv.removeEventListener('scroll', update);
        };
    }, []);

    return (
        <button
            onClick={onRefresh}
            className="fixed right-6 z-40 p-4 rounded-full shadow-2xl transition-all hover:scale-110 active:scale-95 border border-white dark:border-zinc-900 bg-zinc-900 text-white dark:bg-white dark:text-black"
            style={{ bottom: `${bottomOffset}px` }}
            title="Refresh"
        >
            <RefreshCw size={28} />
        </button>
    );
};

export default FloatingRefreshButton;

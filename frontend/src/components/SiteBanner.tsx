import { useState } from 'react';
import { Info, CheckCircle2, AlertTriangle, AlertOctagon, X } from 'lucide-react';
import { useActiveBanner } from '../hooks/useBanner';
import { BannerVariant } from '../types';

const DISMISSED_KEY = 'dismissedBanner';

const bannerStyles: Record<BannerVariant, { className: string; icon: typeof Info }> = {
    [BannerVariant.Info]: { className: 'bg-blue-500/10 border-blue-500/30 text-blue-700 dark:text-blue-300', icon: Info },
    [BannerVariant.Success]: { className: 'bg-emerald-500/10 border-emerald-500/30 text-emerald-700 dark:text-emerald-300', icon: CheckCircle2 },
    [BannerVariant.Warning]: { className: 'bg-amber-500/10 border-amber-500/30 text-amber-700 dark:text-amber-300', icon: AlertTriangle },
    [BannerVariant.Danger]: { className: 'bg-red-500/10 border-red-500/30 text-red-700 dark:text-red-300', icon: AlertOctagon },
};

interface BannerViewProps {
    message: string;
    variant: BannerVariant;
    onDismiss?: () => void;
}

/** Presentational banner, shared by the live site banner and the admin preview. Message is plain text. */
export const BannerView = ({ message, variant, onDismiss }: BannerViewProps) => {
    const { className, icon: Icon } = bannerStyles[variant] ?? bannerStyles[BannerVariant.Info];
    return (
        <div role="status" className={`flex items-start gap-3 px-4 lg:px-6 py-3 border-b text-sm font-medium ${className}`}>
            <Icon size={18} className="shrink-0 mt-0.5" />
            <p className="flex-1 whitespace-pre-line break-words">{message}</p>
            {onDismiss && (
                <button
                    onClick={onDismiss}
                    aria-label="Dismiss banner"
                    className="shrink-0 p-1 -m-1 rounded-md opacity-70 hover:opacity-100 hover:bg-black/5 dark:hover:bg-white/10 transition"
                >
                    <X size={16} />
                </button>
            )}
        </div>
    );
};

/**
 * Site-wide banner, editable from the banner settings page. Dismissal is remembered per browser and keyed on the
 * banner's last update, so editing the banner shows it again to everyone.
 */
const SiteBanner = () => {
    const { data: banner } = useActiveBanner();
    const [dismissed, setDismissed] = useState(() => localStorage.getItem(DISMISSED_KEY));

    if (!banner || dismissed === banner.updatedAt) return null;

    const dismiss = () => {
        localStorage.setItem(DISMISSED_KEY, banner.updatedAt);
        setDismissed(banner.updatedAt);
    };

    return <BannerView message={banner.message} variant={banner.variant} onDismiss={dismiss} />;
};

export default SiteBanner;

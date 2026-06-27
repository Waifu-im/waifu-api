import { Link } from 'react-router-dom';
import { ArrowRight, Book, Wifi, WifiOff, Image as ImageIcon, Tag as TagIcon, Users, Activity } from 'lucide-react';
import { useEffect, useState } from 'react';
import api from '../services/api';
import { ImageDto, PaginatedList, NsfwMode, Orientation, AnimatedMode } from '../types';
import { getEnv } from '../utils/env';
import Skeleton from '../components/Skeleton';
import { MetaTags } from '../hooks/useMetaTags';
import { imgPreview } from '../utils/cfImage';

interface PublicStats {
    totalRequests: number;
    totalImages: number;
    totalTags: number;
    totalArtists: number;
}

function formatCompactNumber(n: number): string {
    if (n >= 1_000_000_000) return `${+(n / 1_000_000_000).toFixed(1)}B`;
    if (n >= 1_000_000) return `${+(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${+(n / 1_000).toFixed(1)}K`;
    return n.toString();
}

const Home = () => {
    const [heroImage, setHeroImage] = useState<ImageDto | null>(null);
    const [imageLoaded, setImageLoaded] = useState(false);
    const [isApiOnline, setIsApiOnline] = useState<boolean>(false);
    const [isLoading, setIsLoading] = useState<boolean>(true);
    const [stats, setStats] = useState<PublicStats | null>(null);
    const [loadingStats, setLoadingStats] = useState<boolean>(true);

    const docsUrl = getEnv('VITE_DOCS_URL');

    useEffect(() => {
        const fetchHero = async () => {
            try {
                const { data } = await api.get<PaginatedList<ImageDto>>('/images', {
                    params: {
                        isNsfw: NsfwMode.False,
                        pageSize: 1,
                        orientation: Orientation.Landscape,
                        isAnimated: AnimatedMode.False,
                        width: '>=1000'
                    },
                    skipGlobalErrorHandler: true
                });
                if (data.items.length > 0) {
                    setHeroImage(data.items[0]);
                    setIsApiOnline(true);
                }
            } catch (e) {
                console.error(e);
                setIsApiOnline(false);
            } finally {
                setIsLoading(false);
            }
        };

        const fetchStats = async () => {
            try {
                const { data } = await api.get<PublicStats>('/stats/public', { skipGlobalErrorHandler: true });
                setStats(data);
            } catch (e) {
                console.error(e);
            } finally {
                setLoadingStats(false);
            }
        };

        fetchHero();
        fetchStats();
    }, []);

    return (
        <div className="relative flex flex-col items-center justify-start md:justify-center p-4 min-h-[calc(100vh-4rem)] pt-16 md:pt-4">
            <MetaTags />

            {/* Background Image */}
            {heroImage && (
                <div className="absolute inset-0 z-0 overflow-hidden select-none">
                    <img
                        src={imgPreview(heroImage.url)}
                        className={`w-full h-full object-cover scale-110 blur-md grayscale-[20%] transition-opacity duration-700 ${imageLoaded ? 'opacity-[.55] dark:opacity-20' : 'opacity-0'}`}
                        alt="Atmosphere"
                        onLoad={() => setImageLoaded(true)}
                        onError={() => setHeroImage(null)}
                    />
                    
                    <div className="absolute inset-0 bg-gradient-to-b from-transparent via-background/50 to-background"></div>
                </div>
            )}

            <div className="relative z-10 max-w-4xl w-full text-center space-y-8 animate-in fade-in zoom-in-95 duration-700">

                <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-secondary/50 border border-border text-xs font-semibold text-muted-foreground backdrop-blur-md min-h-[26px]">
                    {isLoading ? (
                        <div className="flex items-center gap-2">
                            <Skeleton className="w-3 h-3 rounded-full" />
                            <Skeleton className="w-16 h-3 rounded" />
                        </div>
                    ) : isApiOnline ? (
                        <>
                            <Wifi size={12} className="text-emerald-500" />
                            <span>API Online</span>
                        </>
                    ) : (
                        <>
                            <WifiOff size={12} className="text-destructive" />
                            <span>API Offline</span>
                        </>
                    )}
                </div>

                <h1 className="text-5xl md:text-7xl font-black tracking-tight leading-tight text-foreground">
                    The <span className="text-info">API</span> for your <span className="static-rainbow">Waifu</span> content
                </h1>

                <p className="text-xl md:text-2xl text-muted-foreground font-light max-w-2xl mx-auto">
                    Access thousands of categorized anime illustrations via our robust REST API or browse freely.
                </p>

                {/* Stats Grid */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 max-w-3xl mx-auto pt-4">
                   
                    <div className="bg-card/75 dark:bg-card/90 backdrop-blur-md border border-border p-4 rounded-xl flex flex-col items-center shadow-sm">
                        <Activity className="text-primary mb-2" size={24} />
                        {loadingStats ? (
                            <Skeleton className="h-8 w-20 mb-1" />
                        ) : (
                            <span className="text-2xl font-bold">{formatCompactNumber(stats?.totalRequests ?? 0)}</span>
                        )}
                        <span className="text-xs text-muted-foreground uppercase tracking-wider">Requests</span>
                    </div>
                    <div className="bg-card/75 dark:bg-card/90 backdrop-blur-md border border-border p-4 rounded-xl flex flex-col items-center shadow-sm">
                        <ImageIcon className="text-blue-500 mb-2" size={24} />
                        {loadingStats ? (
                            <Skeleton className="h-8 w-16 mb-1" />
                        ) : (
                            <span className="text-2xl font-bold">{formatCompactNumber(stats?.totalImages ?? 0)}</span>
                        )}
                        <span className="text-xs text-muted-foreground uppercase tracking-wider">Images</span>
                    </div>
                    <div className="bg-card/75 dark:bg-card/90 backdrop-blur-md border border-border p-4 rounded-xl flex flex-col items-center shadow-sm">
                        <TagIcon className="text-green-500 mb-2" size={24} />
                        {loadingStats ? (
                            <Skeleton className="h-8 w-12 mb-1" />
                        ) : (
                            <span className="text-2xl font-bold">{formatCompactNumber(stats?.totalTags ?? 0)}</span>
                        )}
                        <span className="text-xs text-muted-foreground uppercase tracking-wider">Tags</span>
                    </div>
                    <div className="bg-card/75 dark:bg-card/90 backdrop-blur-md border border-border p-4 rounded-xl flex flex-col items-center shadow-sm">
                        <Users className="text-purple-500 mb-2" size={24} />
                        {loadingStats ? (
                            <Skeleton className="h-8 w-12 mb-1" />
                        ) : (
                            <span className="text-2xl font-bold">{formatCompactNumber(stats?.totalArtists ?? 0)}</span>
                        )}
                        <span className="text-xs text-muted-foreground uppercase tracking-wider">Artists</span>
                    </div>
                </div>

                <div className="flex flex-col sm:flex-row items-center justify-center gap-4 pt-4">
                    <Link
                        to="/gallery"
                        className="w-full sm:w-auto px-8 py-4 rounded-xl bg-foreground text-background font-bold text-lg hover:opacity-90 transition-all flex items-center justify-center gap-2 shadow-xl hover:-translate-y-1"
                    >
                        Start Browsing <ArrowRight size={20} />
                    </Link>

                    {docsUrl && (
                        <a
                            href={docsUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="w-full sm:w-auto px-8 py-4 rounded-xl bg-card border border-border hover:bg-secondary text-foreground font-bold text-lg transition-all flex items-center justify-center gap-2 shadow-sm"
                        >
                            <Book size={20} /> API Docs
                        </a>
                    )}
                </div>
            </div>
        </div>
    );
};

export default Home;
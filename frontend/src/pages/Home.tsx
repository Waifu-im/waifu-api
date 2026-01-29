import { Link } from 'react-router-dom';
import { ArrowRight, Book, Wifi, WifiOff, Image as ImageIcon, Tag as TagIcon, Users, Activity } from 'lucide-react';
import { useEffect, useState } from 'react';
import api from '../services/api';
import { ImageDto, PaginatedList } from '../types';

interface PublicStats {
    totalRequests: number;
    totalImages: number;
    totalTags: number;
    totalArtists: number;
}

const Home = () => {
  const [heroImage, setHeroImage] = useState<ImageDto | null>(null);
  const [isApiOnline, setIsApiOnline] = useState<boolean>(false);
  const [stats, setStats] = useState<PublicStats | null>(null);

  useEffect(() => {
    const fetchHero = async () => {
      try {
        const { data } = await api.get<PaginatedList<ImageDto>>('/images', {
          params: { isNsfw: 0, pageSize: 1, orientation: 'LANDSCAPE' },
          skipGlobalErrorHandler: true
        });
        if (data.items.length > 0) {
            setHeroImage(data.items[0]);
            setIsApiOnline(true);
        }
      } catch (e) { 
          console.error(e);
          setIsApiOnline(false);
      }
    };

    const fetchStats = async () => {
        try {
            const { data } = await api.get<PublicStats>('/stats/public', { skipGlobalErrorHandler: true });
            setStats(data);
        } catch (e) {
            console.error(e);
        }
    };

    fetchHero();
    fetchStats();
  }, []);

  return (
      <div className="relative flex flex-col items-center justify-start md:justify-center p-4 min-h-[calc(100vh-4rem)] pt-16 md:pt-4">

        {/* Background Image - Heavily blurred to fix resolution issues */}
        {heroImage && (
            <div className="absolute inset-0 z-0 overflow-hidden select-none">
              <img
                  src={heroImage.url}
                  className="w-full h-full object-cover opacity-20 dark:opacity-[0.07] scale-110 blur-md grayscale-[20%]"
                  alt="Atmosphere"
                  onError={() => setHeroImage(null)}
              />
              <div className="absolute inset-0 bg-gradient-to-b from-background/80 via-transparent to-background"></div>
            </div>
        )}

        <div className="relative z-10 max-w-4xl w-full text-center space-y-8 animate-in fade-in zoom-in-95 duration-700">

          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-secondary/50 border border-border text-xs font-semibold text-muted-foreground backdrop-blur-md">
            {isApiOnline ? (
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
          {stats && (
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4 max-w-3xl mx-auto pt-4">
                  <div className="bg-card/50 backdrop-blur-sm border border-border p-4 rounded-xl flex flex-col items-center">
                      <Activity className="text-primary mb-2" size={24} />
                      <span className="text-2xl font-bold">{stats.totalRequests.toLocaleString()}</span>
                      <span className="text-xs text-muted-foreground uppercase tracking-wider">Requests</span>
                  </div>
                  <div className="bg-card/50 backdrop-blur-sm border border-border p-4 rounded-xl flex flex-col items-center">
                      <ImageIcon className="text-blue-500 mb-2" size={24} />
                      <span className="text-2xl font-bold">{stats.totalImages.toLocaleString()}</span>
                      <span className="text-xs text-muted-foreground uppercase tracking-wider">Images</span>
                  </div>
                  <div className="bg-card/50 backdrop-blur-sm border border-border p-4 rounded-xl flex flex-col items-center">
                      <TagIcon className="text-green-500 mb-2" size={24} />
                      <span className="text-2xl font-bold">{stats.totalTags.toLocaleString()}</span>
                      <span className="text-xs text-muted-foreground uppercase tracking-wider">Tags</span>
                  </div>
                  <div className="bg-card/50 backdrop-blur-sm border border-border p-4 rounded-xl flex flex-col items-center">
                      <Users className="text-purple-500 mb-2" size={24} />
                      <span className="text-2xl font-bold">{stats.totalArtists.toLocaleString()}</span>
                      <span className="text-xs text-muted-foreground uppercase tracking-wider">Artists</span>
                  </div>
              </div>
          )}

          <div className="flex flex-col sm:flex-row items-center justify-center gap-4 pt-4">
            <Link
                to="/gallery"
                className="w-full sm:w-auto px-8 py-4 rounded-xl bg-foreground text-background font-bold text-lg hover:opacity-90 transition-all flex items-center justify-center gap-2 shadow-xl hover:-translate-y-1"
            >
              Start Browsing <ArrowRight size={20} />
            </Link>

            <a
                href="https://docs.waifu.im"
                target="_blank"
                rel="noopener noreferrer"
                className="w-full sm:w-auto px-8 py-4 rounded-xl bg-card border border-border hover:bg-secondary text-foreground font-bold text-lg transition-all flex items-center justify-center gap-2 shadow-sm"
            >
              <Book size={20} /> API Docs
            </a>
          </div>
        </div>
      </div>
  );
};

export default Home;
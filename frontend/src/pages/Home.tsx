import { Link } from 'react-router-dom';
import { ArrowRight, Book, ShieldCheck } from 'lucide-react';
import { useEffect, useState } from 'react';
import api from '../services/api';
import { ImageDto, PaginatedList } from '../types';

const Home = () => {
  const [heroImage, setHeroImage] = useState<ImageDto | null>(null);

  useEffect(() => {
    const fetchHero = async () => {
      try {
        const { data } = await api.get<PaginatedList<ImageDto>>('/images', {
          params: { isNsfw: 0, pageSize: 1, orientation: 'LANDSCAPE' }
        });
        if (data.items.length > 0) setHeroImage(data.items[0]);
      } catch (e) { console.error(e); }
    };
    fetchHero();
  }, []);

  return (
      <div className="relative h-full flex flex-col items-center justify-center p-4 min-h-[calc(100vh-4rem)]">

        {/* Background Image - Heavily blurred to fix resolution issues */}
        {heroImage && (
            <div className="absolute inset-0 z-0 overflow-hidden select-none">
              <img
                  src={heroImage.url}
                  className="w-full h-full object-cover opacity-10 dark:opacity-[0.07] scale-110 blur-xl grayscale-[20%]"
                  alt="Atmosphere"
              />
              <div className="absolute inset-0 bg-gradient-to-b from-background/80 via-transparent to-background"></div>
            </div>
        )}

        <div className="relative z-10 max-w-4xl w-full text-center space-y-8 animate-in fade-in zoom-in-95 duration-700">

          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-secondary/50 border border-border text-xs font-semibold text-muted-foreground backdrop-blur-md">
            <ShieldCheck size={12} className="text-emerald-500" />
            <span>Secure & Verified Archive</span>
          </div>

          <h1 className="text-5xl md:text-7xl font-black tracking-tight leading-tight text-foreground">
            The <span className="text-info">API</span> for your <span className="static-rainbow">Waifu</span> content
          </h1>

          <p className="text-xl md:text-2xl text-muted-foreground font-light max-w-2xl mx-auto">
            Access thousands of categorized anime illustrations via our robust REST API or browse freely.
          </p>

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
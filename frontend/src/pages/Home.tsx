import { Link } from 'react-router-dom';
import { ArrowRight, Image as ImageIcon, Clock, AlertTriangle } from 'lucide-react';
import { useEffect, useState } from 'react';
import api from '../services/api';
import { ImageDto } from '../types';

const Home = () => {
  const [heroImage, setHeroImage] = useState<ImageDto | null>(null);

  useEffect(() => {
    const fetchHeroImage = async () => {
      try {
        const { data } = await api.get<ImageDto[]>('/images', {
          params: {
            isNsfw: 0, // 0 for Safe
            limit: 1,
          }
        });
        if (data && data.length > 0) {
          setHeroImage(data[0]);
        }
      } catch (error) {
        console.error("Failed to fetch hero image", error);
      }
    };

    fetchHeroImage();
  }, []);

  return (
    <div className="flex flex-col min-h-[calc(100vh-4rem)]">
      {/* Hero Section */}
      <section className="relative flex-1 flex items-center justify-center py-20 overflow-hidden">
        {/* Background Image */}
        {heroImage && (
          <div className="absolute inset-0 z-0">
            <img 
              src={heroImage.url} 
              alt="Hero Background" 
              className="w-full h-full object-cover opacity-20 dark:opacity-10 blur-sm scale-105"
            />
            <div className="absolute inset-0 bg-gradient-to-b from-background/80 via-background/50 to-background"></div>
          </div>
        )}

        <div className="container mx-auto px-4 text-center max-w-4xl relative z-10">
          <h1 className="text-5xl md:text-7xl font-extrabold mb-6 tracking-tight">
            The API for your Waifu images
          </h1>
          <p className="text-lg md:text-xl text-muted-foreground mb-8 leading-relaxed max-w-2xl mx-auto">
            WAIFU.IM is a powerful and easy-to-use API for retrieving high-quality waifu images. 
            Search, filter, and integrate anime art into your applications with ease.
          </p>
          
          <div className="flex flex-wrap justify-center gap-4">
            <Link 
              to="/gallery" 
              className="px-8 py-4 bg-primary text-primary-foreground hover:bg-primary/90 rounded-full font-bold text-lg transition-transform hover:scale-105 shadow-lg flex items-center gap-2"
            >
              Start Browsing <ArrowRight size={20} />
            </Link>
            <Link 
              to="/upload" 
              className="px-8 py-4 bg-secondary text-secondary-foreground hover:bg-secondary/80 rounded-full font-bold text-lg transition-all hover:shadow-md"
            >
              Upload Image
            </Link>
          </div>

          {heroImage && (
            <div className="mt-12 max-w-md mx-auto bg-card rounded-xl shadow-xl overflow-hidden border border-border transform rotate-2 hover:rotate-0 transition-transform duration-500">
              <img src={heroImage.url} alt="Waifu Preview" className="w-full h-auto" />
              <div className="p-3 text-xs text-muted-foreground flex justify-between">
                <span>ID: {heroImage.id}</span>
                {heroImage.artist && <span>Artist: {heroImage.artist.name}</span>}
              </div>
            </div>
          )}
        </div>
      </section>

      {/* Quick Links Section */}
      <section className="py-16 bg-secondary/30">
        <div className="container mx-auto px-4">
          <h2 className="text-3xl font-bold text-center mb-12">Quick Access</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-8 max-w-5xl mx-auto">
            
            {/* Top Images */}
            <Link to="/gallery?orderBy=FAVORITES" className="group">
              <div className="bg-card p-8 rounded-2xl border border-border hover:border-primary transition-all hover:shadow-xl h-full flex flex-col items-center text-center">
                <div className="w-16 h-16 bg-primary/10 rounded-full flex items-center justify-center text-primary mb-6 group-hover:scale-110 transition-transform">
                  <ImageIcon size={32} />
                </div>
                <h3 className="text-xl font-bold mb-2">Top Images</h3>
                <p className="text-muted-foreground">Browse the most favorited images by the community.</p>
              </div>
            </Link>

            {/* Recent Uploads */}
            <Link to="/gallery?orderBy=UPLOADED_AT" className="group">
              <div className="bg-card p-8 rounded-2xl border border-border hover:border-primary transition-all hover:shadow-xl h-full flex flex-col items-center text-center">
                <div className="w-16 h-16 bg-primary/10 rounded-full flex items-center justify-center text-primary mb-6 group-hover:scale-110 transition-transform">
                  <Clock size={32} />
                </div>
                <h3 className="text-xl font-bold mb-2">Recent Uploads</h3>
                <p className="text-muted-foreground">See the latest additions to the collection.</p>
              </div>
            </Link>

            {/* NSFW */}
            <Link to="/gallery?isNsfw=true" className="group">
              <div className="bg-card p-8 rounded-2xl border border-border hover:border-destructive transition-all hover:shadow-xl h-full flex flex-col items-center text-center">
                <div className="w-16 h-16 bg-destructive/10 rounded-full flex items-center justify-center text-destructive mb-6 group-hover:scale-110 transition-transform">
                  <AlertTriangle size={32} />
                </div>
                <h3 className="text-xl font-bold mb-2">NSFW Content</h3>
                <p className="text-muted-foreground">Browse adult content (18+ only).</p>
              </div>
            </Link>

          </div>
        </div>
      </section>
    </div>
  );
};

export default Home;

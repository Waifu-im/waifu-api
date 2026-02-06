import { Link } from 'react-router-dom';
import { Home, ArrowLeft } from 'lucide-react';

const NotFound = () => {
    return (
        <div className="flex flex-col items-center justify-center min-h-[80vh] text-center px-4">
            <div className="relative mb-8">
                <h1 className="text-9xl font-black text-primary/10 select-none">404</h1>
                <div className="absolute inset-0 flex items-center justify-center">
                    <span className="text-4xl font-bold text-foreground">Page Not Found</span>
                </div>
            </div>
            
            <p className="text-muted-foreground text-lg max-w-md mb-8">
                Oops! The page you are looking for might have been removed, had its name changed, or is temporarily unavailable.
            </p>

            <div className="flex flex-col sm:flex-row gap-4">
                <button 
                    onClick={() => window.history.back()}
                    className="flex items-center gap-2 px-6 py-3 rounded-xl bg-secondary text-secondary-foreground font-medium hover:bg-secondary/80 transition-colors"
                >
                    <ArrowLeft size={20} />
                    Go Back
                </button>
                
                <Link 
                    to="/"
                    className="flex items-center gap-2 px-6 py-3 rounded-xl bg-primary text-primary-foreground font-bold hover:opacity-90 transition-all shadow-lg shadow-primary/20"
                >
                    <Home size={20} />
                    Back to Home
                </Link>
            </div>
        </div>
    );
};

export default NotFound;

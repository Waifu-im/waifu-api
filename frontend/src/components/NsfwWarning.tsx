import { AlertTriangle } from 'lucide-react';

interface NsfwWarningProps {
    variant: 'overlay' | 'modal';
    onConsent: () => void;
    onDecline: () => void;
}

const NsfwWarning = ({ variant, onConsent, onDecline }: NsfwWarningProps) => {
    if (variant === 'overlay') {
        return (
            <div className="absolute inset-0 bg-background/95 backdrop-blur-3xl flex items-center justify-center p-4 z-10">
                <div className="text-center max-w-sm">
                    <AlertTriangle size={48} className="text-destructive mx-auto mb-4" />
                    <h3 className="text-2xl font-bold mb-2">NSFW Content</h3>
                    <p className="text-muted-foreground mb-6">This content is flagged as 18+.</p>
                    <div className="flex gap-3 justify-center">
                        <button
                            onClick={onConsent}
                            className="bg-primary text-primary-foreground px-6 py-2.5 rounded-lg font-bold hover:bg-primary/90 transition-colors"
                        >
                            Show Content
                        </button>
                        <button
                            onClick={onDecline}
                            className="bg-secondary px-6 py-2.5 rounded-lg font-bold hover:bg-secondary/80 transition-colors"
                        >
                            Go Back
                        </button>
                    </div>
                </div>
            </div>
        );
    }

    // Modal variant (fullscreen)
    return (
        <div className="fixed inset-0 z-[60] bg-black/90 flex items-center justify-center p-4 backdrop-blur-3xl">
            <div className="bg-card p-8 rounded-2xl max-w-sm w-full text-center border border-border shadow-2xl">
                <AlertTriangle size={48} className="text-destructive mx-auto mb-4" />
                <h2 className="text-2xl font-black mb-2">Age Restricted</h2>
                <p className="text-muted-foreground mb-6">This content is intended for adults only.</p>
                <button
                    onClick={onConsent}
                    className="w-full py-3 bg-primary text-primary-foreground rounded-xl font-bold mb-2 hover:bg-primary/90 transition-colors"
                >
                    I am 18+
                </button>
                <button
                    onClick={onDecline}
                    className="w-full py-3 bg-secondary rounded-xl font-bold hover:bg-secondary/80 transition-colors"
                >
                    Go Back
                </button>
            </div>
        </div>
    );
};

export default NsfwWarning;

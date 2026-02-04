import { getEnv } from '../utils/env';
import { Mail, MessageSquare } from 'lucide-react';

const Contact = () => {
    const contactEmail = getEnv('VITE_CONTACT_EMAIL');
    const discordServerUrl = getEnv('VITE_DISCORD_SERVER_URL');

    if (!contactEmail && !discordServerUrl) {
        return (
            <div className="flex flex-col items-center justify-center min-h-[60vh] p-4 text-center">
                <h1 className="text-2xl font-bold mb-2">Contact Information Unavailable</h1>
                <p className="text-muted-foreground">No contact details have been configured for this instance.</p>
            </div>
        );
    }

    return (
        <div className="container mx-auto px-4 py-12 max-w-4xl">
            <div className="text-center mb-12">
                <h1 className="text-4xl font-black mb-4 bg-gradient-to-r from-primary to-purple-600 bg-clip-text text-transparent">
                    Get in Touch
                </h1>
                <p className="text-xl text-muted-foreground">
                    Have questions, suggestions, or need support? We're here to help!
                </p>
            </div>

            <div className="grid md:grid-cols-2 gap-6 max-w-2xl mx-auto">
                {contactEmail && (
                    <a 
                        href={`mailto:${contactEmail}`}
                        className="group flex flex-col items-center p-8 bg-card border border-border rounded-2xl hover:border-primary/50 hover:shadow-lg hover:shadow-primary/5 transition-all duration-300"
                    >
                        <div className="w-16 h-16 rounded-full bg-primary/10 flex items-center justify-center mb-6 group-hover:scale-110 transition-transform duration-300">
                            <Mail size={32} className="text-primary" />
                        </div>
                        <h2 className="text-xl font-bold mb-2">Email Support</h2>
                        <p className="text-muted-foreground text-center mb-4">
                            Send us an email and we'll get back to you as soon as possible.
                        </p>
                        <span className="text-primary font-medium group-hover:underline">
                            {contactEmail}
                        </span>
                    </a>
                )}

                {discordServerUrl && (
                    <a 
                        href={discordServerUrl}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="group flex flex-col items-center p-8 bg-card border border-border rounded-2xl hover:border-[#5865F2]/50 hover:shadow-lg hover:shadow-[#5865F2]/5 transition-all duration-300"
                    >
                        <div className="w-16 h-16 rounded-full bg-[#5865F2]/10 flex items-center justify-center mb-6 group-hover:scale-110 transition-transform duration-300">
                            <MessageSquare size={32} className="text-[#5865F2]" />
                        </div>
                        <h2 className="text-xl font-bold mb-2">Discord Community</h2>
                        <p className="text-muted-foreground text-center mb-4">
                            Join our community server for real-time support and discussions.
                        </p>
                        <span className="text-[#5865F2] font-medium group-hover:underline">
                            Join Server
                        </span>
                    </a>
                )}
            </div>
        </div>
    );
};

export default Contact;

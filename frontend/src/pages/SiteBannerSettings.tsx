import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Megaphone, Save, Eye, EyeOff } from 'lucide-react';
import api from '../services/api';
import { useRequireAuth } from '../hooks/useRequireAuth';
import { usePermissions, invalidateBanner } from '../hooks/useBanner';
import { useNotification } from '../context/NotificationContext';
import { BannerView } from '../components/SiteBanner';
import { BannerSettings, BannerVariant } from '../types';

const MAX_MESSAGE_LENGTH = 500;

const variants: { value: BannerVariant; label: string; dot: string }[] = [
    { value: BannerVariant.Info, label: 'Info', dot: 'bg-blue-500' },
    { value: BannerVariant.Success, label: 'Success', dot: 'bg-emerald-500' },
    { value: BannerVariant.Warning, label: 'Warning', dot: 'bg-amber-500' },
    { value: BannerVariant.Danger, label: 'Danger', dot: 'bg-red-500' },
];

/**
 * Site banner settings. The API enforces Permissions__BannerManagementMinRole (Moderator by default) on
 * /banner/settings; the permission check here only avoids rendering a form the API would reject.
 */
const SiteBannerSettings = () => {
    const user = useRequireAuth();
    const { canManageBanner, isLoading: permissionsLoading } = usePermissions();
    const queryClient = useQueryClient();
    const { showNotification } = useNotification();

    const settingsQuery = useQuery({
        queryKey: ['banner', 'settings'],
        enabled: canManageBanner,
        queryFn: async () => (await api.get<BannerSettings>('/banner/settings')).data,
    });

    const [message, setMessage] = useState('');
    const [variant, setVariant] = useState<BannerVariant>(BannerVariant.Info);
    const [isEnabled, setIsEnabled] = useState(false);
    const [saving, setSaving] = useState(false);

    useEffect(() => {
        if (!settingsQuery.data) return;
        setMessage(settingsQuery.data.message);
        setVariant(settingsQuery.data.variant);
        setIsEnabled(settingsQuery.data.isEnabled);
    }, [settingsQuery.data]);

    if (!user || permissionsLoading) return <div className="p-10 text-center">Loading...</div>;
    if (!canManageBanner) return <div className="p-10 text-center">Access Denied</div>;

    const saved = settingsQuery.data;
    const isDirty = !!saved && (message !== saved.message || variant !== saved.variant || isEnabled !== saved.isEnabled);
    const canSave = isDirty && !saving && (!isEnabled || message.trim().length > 0);

    const handleSave = async () => {
        setSaving(true);
        try {
            const { data } = await api.put<BannerSettings>('/banner/settings', { message, variant, isEnabled });
            queryClient.setQueryData(['banner', 'settings'], data);
            await invalidateBanner(queryClient);
            showNotification('success', data.isEnabled ? 'Banner published.' : 'Banner saved (hidden).');
        } catch {
            // Errors are surfaced by the global API error handler.
        } finally {
            setSaving(false);
        }
    };

    return (
        <div className="container mx-auto p-6 md:p-10 max-w-4xl">
            <div className="flex justify-between items-center gap-4 mb-10">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground"><Megaphone className="text-primary" size={32} /> Site Banner</h1>
                    <p className="text-muted-foreground mt-1">Shown at the top of every page. Visitors can dismiss it; saving a change shows it again.</p>
                </div>
                <button
                    type="button"
                    onClick={() => setIsEnabled(!isEnabled)}
                    disabled={settingsQuery.isLoading}
                    className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-bold whitespace-nowrap transition-colors ${
                        isEnabled ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : 'bg-secondary text-muted-foreground'
                    }`}
                >
                    {isEnabled ? <><Eye size={16} /> Visible</> : <><EyeOff size={16} /> Hidden</>}
                </button>
            </div>

            <section className="bg-card border border-border rounded-xl p-6 space-y-6">
                {settingsQuery.isLoading ? (
                    <div className="h-40 bg-muted rounded-xl animate-pulse" />
                ) : (
                    <>
                        <div>
                            <label htmlFor="banner-message" className="block text-sm font-bold mb-2">Message</label>
                            <textarea
                                id="banner-message"
                                value={message}
                                onChange={e => setMessage(e.target.value)}
                                maxLength={MAX_MESSAGE_LENGTH}
                                rows={3}
                                placeholder="e.g. Scheduled maintenance tonight from 22:00 UTC."
                                className="w-full p-3 bg-background border border-input rounded-xl focus:outline-none focus:ring-2 focus:ring-primary resize-y"
                            />
                            <p className="text-xs text-muted-foreground text-right mt-1">{message.length}/{MAX_MESSAGE_LENGTH}</p>
                        </div>

                        <div>
                            <span className="block text-sm font-bold mb-2">Style</span>
                            <div className="flex flex-wrap gap-2">
                                {variants.map(v => (
                                    <button
                                        key={v.value}
                                        type="button"
                                        onClick={() => setVariant(v.value)}
                                        className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-bold border transition-colors ${
                                            variant === v.value ? 'border-primary bg-primary/10 text-foreground' : 'border-border text-muted-foreground hover:bg-secondary'
                                        }`}
                                    >
                                        <span className={`w-2.5 h-2.5 rounded-full ${v.dot}`} /> {v.label}
                                    </button>
                                ))}
                            </div>
                        </div>

                        <div>
                            <span className="block text-sm font-bold mb-2">Preview</span>
                            <div className="rounded-xl overflow-hidden border border-border">
                                {message.trim()
                                    ? <BannerView message={message} variant={variant} onDismiss={() => {}} />
                                    : <p className="p-4 text-sm text-muted-foreground italic">Write a message to preview the banner.</p>}
                            </div>
                        </div>

                        <div className="flex flex-col-reverse sm:flex-row sm:items-center justify-between gap-4 pt-2">
                            <p className="text-xs text-muted-foreground">
                                {saved?.updatedAt
                                    ? <>Last updated {new Date(saved.updatedAt).toLocaleString()}{saved.updatedBy && <> by <span className="font-semibold">{saved.updatedBy.name}</span></>}</>
                                    : 'Never configured.'}
                            </p>
                            <button
                                onClick={handleSave}
                                disabled={!canSave}
                                className="flex items-center justify-center gap-2 px-6 py-3 bg-primary text-primary-foreground rounded-xl font-bold hover:opacity-90 shadow-lg transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                                <Save size={18} /> {saving ? 'Saving...' : 'Save'}
                            </button>
                        </div>
                    </>
                )}
            </section>
        </div>
    );
};

export default SiteBannerSettings;

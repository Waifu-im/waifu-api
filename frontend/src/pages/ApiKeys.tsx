import { useState, useEffect } from 'react';
import api from '../services/api';
import { ApiKeyDto } from '../types';
import { useNotification } from '../context/NotificationContext';
import Modal from '../components/Modal';
import ConfirmModal from '../components/modals/ConfirmModal';
import { Key, Plus, Trash2, Copy, Calendar, Shield, Clock, Edit2 } from 'lucide-react';

const ApiKeys = () => {
    const { showNotification } = useNotification();
    const [keys, setKeys] = useState<ApiKeyDto[]>([]);
    const [loading, setLoading] = useState(true);

    // Create/Edit State
    const [isCreateOpen, setIsCreateOpen] = useState(false);
    const [isEditOpen, setIsEditOpen] = useState(false);
    const [keyFormData, setKeyFormData] = useState({ description: '', expirationDate: '' });
    const [createdKey, setCreatedKey] = useState<string | null>(null);

    // Revoke State
    const [isRevokeOpen, setIsRevokeOpen] = useState(false);
    const [selectedKey, setSelectedKey] = useState<ApiKeyDto | null>(null);

    const fetchKeys = async () => {
        setLoading(true);
        try {
            const { data } = await api.get<ApiKeyDto[]>('/auth/api-keys');
            setKeys(data);
        } catch (err: any) { 
            // showNotification('error', err.message); // Handled globally
        } finally { setLoading(false); }
    };

    useEffect(() => { fetchKeys(); }, []);

    const handleCreate = async () => {
        try {
            const payload: any = { description: keyFormData.description };
            if (keyFormData.expirationDate) payload.expirationDate = new Date(keyFormData.expirationDate).toISOString();

            const { data } = await api.post<ApiKeyDto & { key?: string, token?: string }>('/auth/api-keys', payload);
            setCreatedKey(data.key || data.token || "Error: Token not received");
            fetchKeys();
            setKeyFormData({ description: '', expirationDate: '' });
        } catch (err: any) { 
            // showNotification('error', err.message); // Handled globally
        }
    };

    const handleEdit = async () => {
        if (!selectedKey) return;
        try {
            const payload: any = { description: keyFormData.description };
            // Handle expiration: empty string means no change or remove? 
            // Typically null sends "remove", string sends "date".
            if (keyFormData.expirationDate) payload.expirationDate = new Date(keyFormData.expirationDate).toISOString();
            else payload.expirationDate = null;

            await api.put(`/auth/api-keys/${selectedKey.id}`, payload);
            showNotification('success', 'API Key updated');
            setIsEditOpen(false);
            fetchKeys();
        } catch (err: any) { 
            // showNotification('error', err.message); // Handled globally
        }
    };

    const handleRevoke = async () => {
        if (!selectedKey) return;
        try {
            await api.delete(`/auth/api-keys/${selectedKey.id}`);
            showNotification('success', 'API Key revoked');
            setIsRevokeOpen(false);
            fetchKeys();
        } catch (err: any) { 
            // showNotification('error', err.message); // Handled globally
        }
    };

    const copyToClipboard = (text: string) => {
        if (!text) return;
        navigator.clipboard.writeText(text);
        showNotification('success', 'Copied to clipboard');
    };

    const openEdit = (key: ApiKeyDto) => {
        setSelectedKey(key);
        // Format date to YYYY-MM-DD for input[type="date"]
        const dateStr = key.expirationDate ? new Date(key.expirationDate).toISOString().split('T')[0] : '';
        setKeyFormData({ description: key.description, expirationDate: dateStr });
        setIsEditOpen(true);
    };

    return (
        <div className="container mx-auto p-6 md:p-10">
            <div className="flex justify-between items-center mb-10">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground"><Key className="text-primary" size={32}/> API Keys</h1>
                    <p className="text-muted-foreground mt-1">Manage access tokens for external applications.</p>
                </div>
                <button onClick={() => { setCreatedKey(null); setKeyFormData({description:'', expirationDate:''}); setIsCreateOpen(true); }} className="flex items-center gap-2 px-6 py-3 bg-primary text-primary-foreground rounded-xl font-bold hover:opacity-90 shadow-lg transition-all whitespace-nowrap"><Plus size={20} /> <span className="hidden sm:inline">Generate Key</span></button>
            </div>

            <div className="grid gap-4">
                {loading ? [...Array(3)].map((_,i) => <div key={i} className="h-24 bg-muted rounded-xl animate-pulse"/>) :
                    keys.length === 0 ? (
                            <div className="text-center p-12 border-2 border-dashed border-border rounded-xl text-muted-foreground">
                                <Key size={48} className="mx-auto mb-4 opacity-20"/>
                                <p>No API keys found.</p>
                            </div>
                        ) :
                        keys.map(key => (
                            <div key={key.id} className="bg-card border border-border rounded-xl p-6 flex flex-col md:flex-row justify-between items-start md:items-center gap-4 hover:shadow-md transition-all">
                                <div>
                                    <div className="flex items-center gap-3 mb-2">
                                        <span className="font-bold text-lg">{key.description}</span>
                                        <span className="text-xs bg-secondary border border-border px-2 py-0.5 rounded font-mono text-muted-foreground tracking-wider">{key.keyPrefix}••••••</span>
                                    </div>

                                    <div className="flex flex-wrap gap-4 text-xs text-muted-foreground">
                                        <span className="flex items-center gap-1.5"><Calendar size={14}/> {new Date(key.createdAt).toLocaleDateString()}</span>
                                        <span className="flex items-center gap-1.5"><Shield size={14}/> {key.lastUsedAt ? new Date(key.lastUsedAt).toLocaleDateString() : "Never used"}</span>
                                        <span className="flex items-center gap-1.5"><Clock size={14}/> {key.expirationDate ? new Date(key.expirationDate).toLocaleDateString() : "No expiration"}</span>
                                    </div>
                                </div>
                                <div className="flex items-center gap-2">
                                    <button onClick={() => openEdit(key)} className="px-3 py-2 bg-secondary hover:bg-secondary/80 rounded-lg font-bold transition-colors text-sm flex items-center gap-2">
                                        <Edit2 size={16} /> Edit
                                    </button>
                                    <button onClick={() => { setSelectedKey(key); setIsRevokeOpen(true); }} className="px-3 py-2 text-destructive bg-destructive/10 hover:bg-destructive hover:text-destructive-foreground rounded-lg font-bold transition-colors text-sm flex items-center gap-2">
                                        <Trash2 size={16} /> Revoke
                                    </button>
                                </div>
                            </div>
                        ))}
            </div>

            {/* Create Modal */}
            <Modal isOpen={isCreateOpen} onClose={() => { setIsCreateOpen(false); setCreatedKey(null); }} title="Generate API Key">
                {!createdKey ? (
                    <div className="space-y-4">
                        <div>
                            <label className="block text-sm font-bold mb-1">Description</label>
                            <input value={keyFormData.description} onChange={e => setKeyFormData({...keyFormData, description: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground" placeholder="e.g. My Bot" autoFocus/>
                        </div>
                        <div>
                            <label className="block text-sm font-bold mb-1">Expiration (Optional)</label>
                            <input type="date" value={keyFormData.expirationDate} onChange={e => setKeyFormData({...keyFormData, expirationDate: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground"/>
                        </div>
                        <button onClick={handleCreate} disabled={!keyFormData.description} className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg mt-2 disabled:opacity-50">Generate</button>
                    </div>
                ) : (
                    <div className="space-y-4 text-center">
                        <div className="bg-green-500/10 text-green-600 p-4 rounded-xl mb-4 border border-green-500/20">
                            <p className="font-bold">Key Generated Successfully</p>
                            <p className="text-xs mt-1">Copy it now. You won't be able to see it again.</p>
                        </div>
                        <div onClick={() => copyToClipboard(createdKey)} className="group relative flex items-center gap-2 bg-secondary p-4 rounded-xl border border-border cursor-pointer hover:border-primary/50 transition-colors">
                            <code className="flex-1 text-sm break-all font-mono text-left">{createdKey}</code>
                            <Copy size={18} className="text-muted-foreground group-hover:text-foreground"/>
                        </div>
                        <button onClick={() => { setIsCreateOpen(false); setCreatedKey(null); }} className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg">Done</button>
                    </div>
                )}
            </Modal>

            {/* Edit Modal */}
            <Modal isOpen={isEditOpen} onClose={() => setIsEditOpen(false)} title="Edit API Key">
                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-bold mb-1">Description</label>
                        <input value={keyFormData.description} onChange={e => setKeyFormData({...keyFormData, description: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground" />
                    </div>
                    <div>
                        <label className="block text-sm font-bold mb-1">Expiration (Optional)</label>
                        <input type="date" value={keyFormData.expirationDate} onChange={e => setKeyFormData({...keyFormData, expirationDate: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground"/>
                        <p className="text-xs text-muted-foreground mt-1">Clear date to make it non-expiring.</p>
                    </div>
                    <button onClick={handleEdit} className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg mt-2">Save Changes</button>
                </div>
            </Modal>

            <ConfirmModal isOpen={isRevokeOpen} onClose={() => setIsRevokeOpen(false)} onConfirm={handleRevoke} title="Revoke API Key" message="Are you sure? This action cannot be undone." confirmText="Revoke" />
        </div>
    );
};

export default ApiKeys;
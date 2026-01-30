import { useState, useEffect } from 'react';
import api from '../services/api';
import { User, PaginatedList, Role } from '../types';
import { useNotification } from '../context/NotificationContext';
import { Users as UsersIcon, Search, Shield, Ban, CheckCircle, ChevronLeft, ChevronRight } from 'lucide-react';
import ConfirmModal from '../components/modals/ConfirmModal';
import SearchableSelect from '../components/SearchableSelect'; // Reusing your nice component

const Users = () => {
    const { showNotification } = useNotification();
    const [users, setUsers] = useState<User[]>([]);
    const [loading, setLoading] = useState(true);

    // Search & Pagination
    const [search, setSearch] = useState('');
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const [searchTerm, setSearchTerm] = useState('');

    // Actions
    const [selectedUser, setSelectedUser] = useState<User | null>(null);
    const [isBanModalOpen, setIsBanModalOpen] = useState(false);

    const fetchUsers = async () => {
        setLoading(true);
        try {
            const params = new URLSearchParams({ page: page.toString(), pageSize: '20' });
            if (searchTerm) params.append('search', searchTerm);

            const { data } = await api.get<PaginatedList<User>>('/users', { params });
            setUsers(data.items);
            setTotalPages(data.totalPages);
        } catch { 
            // showNotification('error', 'Failed to load users'); // Handled globally
        } finally { setLoading(false); }
    };

    useEffect(() => { fetchUsers(); }, [page, searchTerm]);

    const handleSearch = (e: React.FormEvent) => {
        e.preventDefault();
        setSearchTerm(search);
        setPage(1);
    };

    const handleRoleChange = async (userId: number, newRole: string) => {
        const roleEnum = parseInt(newRole);
        try {
            await api.put(`/users/${userId}/role`, { role: roleEnum });
            showNotification('success', 'User role updated');
            fetchUsers(); // Refresh to ensure state sync
        } catch { 
            // showNotification('error', 'Failed to update role'); // Handled globally
        }
    };

    const toggleBan = async () => {
        if (!selectedUser) return;
        try {
            await api.put(`/users/${selectedUser.id}/ban`, { isBlacklisted: !selectedUser.isBlacklisted });
            showNotification('success', `User ${selectedUser.isBlacklisted ? 'unbanned' : 'banned'}`);
            setIsBanModalOpen(false);
            fetchUsers();
        } catch { 
            // showNotification('error', 'Failed to update ban status'); // Handled globally
        }
    };

    const openBanModal = (user: User) => {
        setSelectedUser(user);
        setIsBanModalOpen(true);
    };

    const roleOptions = [
        { id: Role.User.toString(), name: 'User' },
        { id: Role.TrustedUser.toString(), name: 'Trusted' },
        { id: Role.Moderator.toString(), name: 'Moderator' },
        { id: Role.Admin.toString(), name: 'Admin' }
    ];

    return (
        <div className="container mx-auto p-6 md:p-10">
            <div className="flex flex-col md:flex-row justify-between items-center mb-8 gap-4">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground"><UsersIcon className="text-primary" size={32}/> User Management</h1>
                    <p className="text-muted-foreground mt-1">Manage user roles and access.</p>
                </div>
                <form onSubmit={handleSearch} className="relative w-full md:w-80">
                    <Search className="absolute left-3 top-3 text-muted-foreground" size={18} />
                    <input
                        type="text"
                        placeholder="Search by name or ID..."
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        className="w-full pl-10 p-3 bg-card border border-border rounded-xl outline-none focus:ring-2 focus:ring-primary"
                    />
                </form>
            </div>

            <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
                <div className="overflow-x-auto min-h-[400px]"> {/* Min height ensures dropdowns aren't cut off if few rows */}
                    <table className="w-full text-left border-collapse">
                        <thead>
                        <tr className="bg-secondary/50 border-b border-border">
                            <th className="p-4 font-bold text-sm text-muted-foreground">ID</th>
                            <th className="p-4 font-bold text-sm text-muted-foreground">User</th>
                            <th className="p-4 font-bold text-sm text-muted-foreground">Discord ID</th>
                            <th className="p-4 font-bold text-sm text-muted-foreground">Total</th>
                            <th className="p-4 font-bold text-sm text-muted-foreground">API Key</th>
                            <th className="p-4 font-bold text-sm text-muted-foreground">Web</th>
                            <th className="p-4 font-bold text-sm text-muted-foreground w-48">Role</th>
                            <th className="p-4 font-bold text-sm text-muted-foreground">Status</th>
                            <th className="p-4 font-bold text-sm text-muted-foreground text-right">Actions</th>
                        </tr>
                        </thead>
                        <tbody>
                        {loading ? (
                            [...Array(5)].map((_, i) => <tr key={i}><td colSpan={9} className="p-4"><div className="h-10 bg-muted rounded animate-pulse"/></td></tr>)
                        ) : users.map(user => (
                            <tr key={user.id} className="border-b border-border last:border-0 hover:bg-secondary/30 transition-colors">
                                <td className="p-4 font-mono text-sm">{user.id}</td>
                                <td className="p-4 font-bold">{user.name}</td>
                                <td className="p-4 font-mono text-xs text-muted-foreground">{user.discordId}</td>
                                <td className="p-4 font-mono text-sm">{user.requestCount?.toLocaleString() || 0}</td>
                                <td className="p-4 font-mono text-sm text-blue-500">{user.apiKeyRequestCount?.toLocaleString() || 0}</td>
                                <td className="p-4 font-mono text-sm text-purple-500">{user.jwtRequestCount?.toLocaleString() || 0}</td>
                                <td className="p-4">
                                    {/* Using your SearchableSelect for consistent look */}
                                    <div className="w-36">
                                        <SearchableSelect
                                            options={roleOptions}
                                            selectedOptions={roleOptions.filter(r => r.id === user.role.toString())}
                                            onSelect={(opt) => handleRoleChange(user.id, opt.id as string)}
                                            onRemove={() => {}} // Single select, can't remove
                                            isMulti={false}
                                            clearable={false}
                                            placeholder="Select Role"
                                        />
                                    </div>
                                </td>
                                <td className="p-4">
                                    {user.isBlacklisted ?
                                        <span className="inline-flex items-center gap-1 bg-destructive/10 text-destructive px-2 py-1 rounded text-xs font-bold"><Ban size={12}/> Banned</span> :
                                        <span className="inline-flex items-center gap-1 bg-green-500/10 text-green-600 px-2 py-1 rounded text-xs font-bold"><CheckCircle size={12}/> Active</span>
                                    }
                                </td>
                                <td className="p-4 text-right">
                                    <button
                                        onClick={() => openBanModal(user)}
                                        className={`p-2 rounded-lg transition-colors ${user.isBlacklisted ? 'bg-green-500/10 text-green-600 hover:bg-green-500/20' : 'bg-secondary text-red-600 hover:bg-red-500/10'}`}
                                        title={user.isBlacklisted ? "Unban" : "Ban"}
                                    >
                                        {user.isBlacklisted ? <Shield size={16} /> : <Ban size={16} />}
                                    </button>
                                </td>
                            </tr>
                        ))}
                        </tbody>
                    </table>
                </div>
            </div>

            {totalPages > 1 && (
                <div className="flex justify-center items-center gap-4 mt-8">
                    <button disabled={page === 1} onClick={() => setPage(p => p - 1)} className="p-2 rounded-full bg-secondary disabled:opacity-50 hover:bg-secondary/80"><ChevronLeft size={20}/></button>
                    <span className="text-sm font-bold">Page {page} of {totalPages}</span>
                    <button disabled={page === totalPages} onClick={() => setPage(p => p + 1)} className="p-2 rounded-full bg-secondary disabled:opacity-50 hover:bg-secondary/80"><ChevronRight size={20}/></button>
                </div>
            )}

            <ConfirmModal
                isOpen={isBanModalOpen}
                onClose={() => setIsBanModalOpen(false)}
                onConfirm={toggleBan}
                title={selectedUser?.isBlacklisted ? "Unban User" : "Ban User"}
                message={<>Are you sure you want to {selectedUser?.isBlacklisted ? "unban" : "ban"} <strong>{selectedUser?.name}</strong>?</>}
                confirmText={selectedUser?.isBlacklisted ? "Unban" : "Ban"}
                variant={selectedUser?.isBlacklisted ? "warning" : "destructive"}
            />
        </div>
    );
};

export default Users;
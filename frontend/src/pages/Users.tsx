import { useState } from 'react';
import api from '../services/api';
import { User, Role } from '../types';
import { useNotification } from '../context/NotificationContext';
import { Users as UsersIcon, Shield, Ban, CheckCircle } from 'lucide-react';
import ConfirmModal from '../components/modals/ConfirmModal';
import SearchableSelect from '../components/SearchableSelect';
import SearchInput from '../components/SearchInput';
import Pagination from '../components/Pagination'; // Import Pagination
import { useResource } from '../hooks/useResource';

const Users = () => {
    const { showNotification } = useNotification();

    const {
        items: users,
        loading,
        page,
        setPage,
        totalPages,
        search,
        setSearch,
        searchType,
        setSearchType,
        refresh
    } = useResource<User>('/users', 20);

    const [selectedUser, setSelectedUser] = useState<User | null>(null);
    const [isBanModalOpen, setIsBanModalOpen] = useState(false);

    const handleRoleChange = async (userId: number, newRole: string) => {
        const roleEnum = parseInt(newRole);
        try {
            await api.put(`/users/${userId}/role`, { role: roleEnum });
            showNotification('success', 'User role updated');
            refresh();
        } catch {
            // handled globally
        }
    };

    const toggleBan = async () => {
        if (!selectedUser) return;
        try {
            await api.put(`/users/${selectedUser.id}/ban`, { isBlacklisted: !selectedUser.isBlacklisted });
            showNotification('success', `User ${selectedUser.isBlacklisted ? 'unbanned' : 'banned'}`);
            setIsBanModalOpen(false);
            refresh();
        } catch {
            // handled globally
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

                <SearchInput
                    value={search}
                    onChange={setSearch}
                    searchType={searchType}
                    onSearchTypeChange={setSearchType}
                    className="w-full md:w-80"
                />
            </div>

            <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
                <div className="overflow-x-auto min-h-[400px]">
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
                                    <div className="w-36">
                                        <SearchableSelect
                                            options={roleOptions}
                                            selectedOptions={roleOptions.filter(r => r.id === user.role.toString())}
                                            onSelect={(opt) => handleRoleChange(user.id, opt.id as string)}
                                            onRemove={() => {}}
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

            {!loading && (
                <Pagination currentPage={page} totalPages={totalPages} setPage={setPage} />
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
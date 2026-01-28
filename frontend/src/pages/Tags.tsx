import { useState, useEffect } from 'react';
import api from '../services/api';
import { Tag, PaginatedList } from '../types'; // Import PaginatedList
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import Modal from '../components/Modal';
import { Plus, Edit2, Trash2, Tag as TagIcon, ChevronLeft, ChevronRight } from 'lucide-react';

const Tags = () => {
    const { user } = useAuth();
    const { showNotification } = useNotification();

    const [tags, setTags] = useState<Tag[]>([]);
    const [loading, setLoading] = useState(true);

    // Pagination state
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const pageSize = 50;

    // Modals state
    const [isCreateOpen, setIsCreateOpen] = useState(false);
    const [isEditOpen, setIsEditOpen] = useState(false);
    const [isDeleteOpen, setIsDeleteOpen] = useState(false);

    const [selectedTag, setSelectedTag] = useState<Tag | null>(null);
    const [formData, setFormData] = useState({ name: '', description: '' });

    const fetchTags = async () => {
        setLoading(true);
        try {
            const { data } = await api.get<PaginatedList<Tag>>('/tags', {
                params: { page, pageSize }
            });
            setTags(data.items);
            setTotalPages(data.totalPages);
        } catch (error) {
            console.error(error);
            showNotification('error', 'Failed to load tags');
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchTags();
    }, [page]);

    const handleCreate = async () => {
        try {
            await api.post('/tags', formData);
            showNotification('success', 'Tag created');
            setIsCreateOpen(false);
            setFormData({ name: '', description: '' });
            fetchTags();
        } catch (e: any) {
            if (e.response?.status === 409) showNotification('error', 'Tag already exists');
            else showNotification('error', 'Failed to create tag');
        }
    };

    const handleEdit = async () => {
        if (!selectedTag) return;
        try {
            await api.put(`/tags/${selectedTag.id}`, formData);
            showNotification('success', 'Tag updated');
            setIsEditOpen(false);
            fetchTags();
        } catch (e) {
            showNotification('error', 'Failed to update tag');
        }
    };

    const handleDelete = async () => {
        if (!selectedTag) return;
        try {
            await api.delete(`/tags/${selectedTag.id}`);
            showNotification('success', 'Tag deleted');
            setIsDeleteOpen(false);
            fetchTags();
        } catch (e) {
            showNotification('error', 'Failed to delete tag');
        }
    };

    const openEdit = (tag: Tag) => {
        setSelectedTag(tag);
        setFormData({ name: tag.name, description: tag.description });
        setIsEditOpen(true);
    };

    const openDelete = (tag: Tag) => {
        setSelectedTag(tag);
        setIsDeleteOpen(true);
    };

    const isAdmin = user?.role === 3;

    return (
        <div className="container mx-auto p-6 md:p-10">
            <div className="flex justify-between items-center mb-10">
                <div>
                    <h1 className="text-3xl font-black flex items-center gap-3 text-foreground">
                        <TagIcon className="text-primary" size={32}/> Tags
                    </h1>
                    <p className="text-muted-foreground mt-1">Browse and manage image tags.</p>
                </div>
                <button
                    onClick={() => { setFormData({name:'', description:''}); setIsCreateOpen(true); }}
                    className="flex items-center gap-2 px-6 py-3 bg-primary text-primary-foreground rounded-xl font-bold hover:opacity-90 shadow-lg transition-all"
                >
                    <Plus size={20} /> New Tag
                </button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                {loading ? (
                    [...Array(8)].map((_,i) => <div key={i} className="h-32 bg-muted rounded-xl animate-pulse"/>)
                ) : (
                    tags.map(tag => (
                        <div key={tag.id} className="bg-card border border-border rounded-xl p-6 hover:shadow-md transition-all group relative">
                            <h3 className="font-bold text-lg mb-2">{tag.name}</h3>
                            <p className="text-sm text-muted-foreground line-clamp-2">{tag.description || "No description"}</p>

                            {isAdmin && (
                                <div className="absolute top-4 right-4 opacity-0 group-hover:opacity-100 transition-opacity flex gap-2">
                                    <button onClick={() => openEdit(tag)} className="p-1.5 hover:bg-secondary rounded text-muted-foreground hover:text-foreground"><Edit2 size={16}/></button>
                                    <button onClick={() => openDelete(tag)} className="p-1.5 hover:bg-destructive/10 rounded text-muted-foreground hover:text-destructive"><Trash2 size={16}/></button>
                                </div>
                            )}
                        </div>
                    ))
                )}
            </div>

            {/* Pagination Controls */}
            {!loading && totalPages > 1 && (
                <div className="flex justify-center items-center gap-4 mt-10">
                    <button
                        disabled={page === 1}
                        onClick={() => setPage(p => Math.max(1, p - 1))}
                        className="p-2 rounded-full bg-secondary disabled:opacity-50 hover:bg-secondary/80"
                    >
                        <ChevronLeft size={20} />
                    </button>
                    <span className="text-sm font-bold">Page {page} of {totalPages}</span>
                    <button
                        disabled={page === totalPages}
                        onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                        className="p-2 rounded-full bg-secondary disabled:opacity-50 hover:bg-secondary/80"
                    >
                        <ChevronRight size={20} />
                    </button>
                </div>
            )}

            {/* Modals reuse generic Modal component */}
            <Modal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} title="New Tag">
                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-bold mb-1">Name</label>
                        <input value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground"/>
                    </div>
                    <div>
                        <label className="block text-sm font-bold mb-1">Description</label>
                        <textarea value={formData.description} onChange={e => setFormData({...formData, description: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none h-24 resize-none text-foreground"/>
                    </div>
                    <button onClick={handleCreate} className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg mt-2">Create</button>
                </div>
            </Modal>

            <Modal isOpen={isEditOpen} onClose={() => setIsEditOpen(false)} title="Edit Tag">
                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-bold mb-1">Name</label>
                        <input value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none focus:ring-1 focus:ring-primary text-foreground"/>
                    </div>
                    <div>
                        <label className="block text-sm font-bold mb-1">Description</label>
                        <textarea value={formData.description} onChange={e => setFormData({...formData, description: e.target.value})} className="w-full p-3 bg-secondary rounded-lg outline-none h-24 resize-none text-foreground"/>
                    </div>
                    <button onClick={handleEdit} className="w-full py-3 bg-primary text-primary-foreground font-bold rounded-lg mt-2">Save</button>
                </div>
            </Modal>

            <Modal isOpen={isDeleteOpen} onClose={() => setIsDeleteOpen(false)} title="Delete Tag">
                <div className="text-center space-y-4">
                    <div className="w-16 h-16 bg-destructive/10 text-destructive rounded-full flex items-center justify-center mx-auto"><Trash2 size={32}/></div>
                    <p className="text-muted-foreground">Delete tag <strong>{selectedTag?.name}</strong>?</p>
                    <div className="flex gap-3 mt-6">
                        <button onClick={() => setIsDeleteOpen(false)} className="flex-1 py-3 bg-secondary rounded-lg font-bold">Cancel</button>
                        <button onClick={handleDelete} className="flex-1 py-3 bg-destructive text-destructive-foreground rounded-lg font-bold">Delete</button>
                    </div>
                </div>
            </Modal>
        </div>
    );
};

export default Tags;
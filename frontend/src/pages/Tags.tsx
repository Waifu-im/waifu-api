import { useState, useEffect } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import api from '../services/api';
import { Tag, PaginatedList, Role } from '../types';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import Modal from '../components/Modal';
import TagModal, { TagFormData } from '../components/modals/TagModal';
import { Plus, Edit2, Trash2, Tag as TagIcon, ChevronLeft, ChevronRight, ExternalLink } from 'lucide-react';

const Tags = () => {
    const { user } = useAuth();
    const { showNotification } = useNotification();
    const navigate = useNavigate();
    const location = useLocation();

    const [tags, setTags] = useState<Tag[]>([]);
    const [loading, setLoading] = useState(true);

    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const pageSize = 50;

    const [isCreateOpen, setIsCreateOpen] = useState(false);
    const [isEditOpen, setIsEditOpen] = useState(false);
    const [isDeleteOpen, setIsDeleteOpen] = useState(false);

    const [selectedTag, setSelectedTag] = useState<Tag | null>(null);
    const [formData, setFormData] = useState<TagFormData>({ name: '', slug: '', description: '' });

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

    const handleOpenCreate = () => {
        if (!user) {
            showNotification('warning', 'Log in to create tags.');
            navigate('/login', { state: { from: location } });
            return;
        }
        setFormData({ name: '', slug: '', description: '' });
        setIsCreateOpen(true);
    };

    const handleCreate = async (data: TagFormData) => {
        try {
            await api.post('/tags', data);
            showNotification('success', 'Tag created');
            setIsCreateOpen(false);
            setFormData({ name: '', slug: '', description: '' });
            fetchTags();
        } catch (e: any) {
            if (e.response?.status === 409) showNotification('error', 'Tag name or slug already exists');
            else showNotification('error', 'Failed to create tag');
        }
    };

    const handleEdit = async (data: TagFormData) => {
        if (!selectedTag) return;
        try {
            await api.put(`/tags/${selectedTag.id}`, data);
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

    const openEdit = (e: React.MouseEvent, tag: Tag) => {
        e.preventDefault();
        e.stopPropagation();
        setSelectedTag(tag);
        setFormData({ name: tag.name, slug: tag.slug, description: tag.description });
        setIsEditOpen(true);
    };

    const openDelete = (e: React.MouseEvent, tag: Tag) => {
        e.preventDefault();
        e.stopPropagation();
        setSelectedTag(tag);
        setIsDeleteOpen(true);
    };

    const isAdmin = user?.role === Role.Admin;
    const isModerator = user?.role === Role.Moderator;
    const canEdit = isAdmin || isModerator;

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
                    onClick={handleOpenCreate}
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
                        <div key={tag.id} className="group bg-card border border-border rounded-xl hover:shadow-md hover:border-primary/50 transition-all overflow-hidden flex flex-col p-6">
                            <div className="flex justify-between items-start mb-2">
                                <div className="flex flex-col gap-1">
                                    <h3 className="font-bold text-lg text-foreground group-hover:text-primary transition-colors">{tag.name}</h3>
                                    <div className="flex items-center gap-2">
                                        <span className="text-xs font-mono bg-secondary px-2 py-1 rounded text-muted-foreground select-text">#{tag.id}</span>
                                        <code className="text-xs text-primary/80 bg-primary/5 px-1.5 py-0.5 rounded select-text" title="API Slug">{tag.slug}</code>
                                    </div>
                                </div>

                                <div className="flex gap-2">
                                    <Link
                                        to={`/gallery?includedTags=${encodeURIComponent(tag.slug)}`}
                                        className="p-1.5 bg-secondary hover:bg-primary hover:text-primary-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                        title="View in Gallery"
                                    >
                                        <ExternalLink size={16} />
                                    </Link>
                                    
                                    {canEdit && (
                                        <>
                                            <button
                                                onClick={(e) => openEdit(e, tag)}
                                                className="p-1.5 bg-secondary hover:bg-primary hover:text-primary-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                                title="Edit"
                                            >
                                                <Edit2 size={16}/>
                                            </button>
                                            {isAdmin && (
                                                <button
                                                    onClick={(e) => openDelete(e, tag)}
                                                    className="p-1.5 bg-secondary hover:bg-destructive hover:text-destructive-foreground rounded text-muted-foreground transition-colors shadow-sm"
                                                    title="Delete"
                                                >
                                                    <Trash2 size={16}/>
                                                </button>
                                            )}
                                        </>
                                    )}
                                </div>
                            </div>

                            <p className="text-sm text-muted-foreground line-clamp-3 mt-2">{tag.description || "No description"}</p>
                        </div>
                    ))
                )}
            </div>

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

            <TagModal
                isOpen={isCreateOpen}
                onClose={() => setIsCreateOpen(false)}
                onSubmit={handleCreate}
                initialData={{ name: '', slug: '', description: '' }}
                title="New Tag"
                submitLabel="Create"
            />

            <TagModal
                isOpen={isEditOpen}
                onClose={() => setIsEditOpen(false)}
                onSubmit={handleEdit}
                initialData={formData}
                title="Edit Tag"
                submitLabel="Save"
            />

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
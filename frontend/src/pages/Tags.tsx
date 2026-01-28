import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import api from '../services/api';
import { Tag, PaginatedList, Role } from '../types';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';
import Modal from '../components/Modal';
import { Plus, Edit2, Trash2, Tag as TagIcon, ChevronLeft, ChevronRight } from 'lucide-react';

const Tags = () => {
    const { user } = useAuth();
    const { showNotification } = useNotification();
    const [tags, setTags] = useState<Tag[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const pageSize = 50;

    const [isCreateOpen, setIsCreateOpen] = useState(false);
    const [isEditOpen, setIsEditOpen] = useState(false);
    const [isDeleteOpen, setIsDeleteOpen] = useState(false);
    const [selectedTag, setSelectedTag] = useState<Tag | null>(null);
    const [formData, setFormData] = useState({ name: '', description: '' });

    const fetchTags = async () => {
        setLoading(true);
        try {
            const { data } = await api.get<PaginatedList<Tag>>('/tags', { params: { page, pageSize } });
            setTags(data.items);
            setTotalPages(data.totalPages);
        } catch (error) { console.error(error); showNotification('error', 'Failed to load tags'); } finally { setLoading(false); }
    };

    useEffect(() => { fetchTags(); }, [page]);

    const handleCreate = async () => { /* ... same as before ... */ };
    const handleEdit = async () => { /* ... same as before ... */ };
    const handleDelete = async () => { /* ... same as before ... */ };

    const openEdit = (e: React.MouseEvent, tag: Tag) => {
        e.preventDefault(); e.stopPropagation();
        setSelectedTag(tag); setFormData({ name: tag.name, description: tag.description }); setIsEditOpen(true);
    };

    const openDelete = (e: React.MouseEvent, tag: Tag) => {
        e.preventDefault(); e.stopPropagation();
        setSelectedTag(tag); setIsDeleteOpen(true);
    };

    const isAdmin = user?.role === Role.Admin;
    const isModerator = user?.role === Role.Moderator;
    const canEdit = isAdmin || isModerator;

    return (
        <div className="container mx-auto p-6 md:p-10">
            {/* ... Header ... */}
            <div className="flex justify-between items-center mb-10">
                <div><h1 className="text-3xl font-black flex items-center gap-3"><TagIcon className="text-primary"/> Tags</h1></div>
                <button onClick={() => setIsCreateOpen(true)} className="flex items-center gap-2 px-6 py-3 bg-primary text-primary-foreground rounded-xl font-bold"><Plus size={20}/> New Tag</button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                {loading ? [...Array(8)].map((_,i) => <div key={i} className="h-32 bg-muted rounded-xl animate-pulse"/>) : (
                    tags.map(tag => (
                        <div key={tag.id} className="relative group">
                            {/* Link to Gallery */}
                            <Link to={`/gallery?includedTags=${encodeURIComponent(tag.name)}`} className="block h-full bg-card border border-border rounded-xl p-6 hover:shadow-md transition-all hover:border-primary/50">
                                <h3 className="font-bold text-lg mb-2 text-foreground group-hover:text-primary transition-colors">{tag.name}</h3>
                                <p className="text-sm text-muted-foreground line-clamp-2">{tag.description || "No description"}</p>
                            </Link>
                            {/* Actions always visible for admins */}
                            {canEdit && (
                                <div className="absolute top-4 right-4 flex gap-2 z-10">
                                    <button onClick={(e) => openEdit(e, tag)} className="p-2 bg-secondary text-secondary-foreground rounded-md hover:bg-primary hover:text-primary-foreground"><Edit2 size={14}/></button>
                                    {isAdmin && <button onClick={(e) => openDelete(e, tag)} className="p-2 bg-secondary text-destructive rounded-md hover:bg-destructive hover:text-destructive-foreground"><Trash2 size={14}/></button>}
                                </div>
                            )}
                        </div>
                    ))
                )}
            </div>
            {/* ... Pagination & Modals ... */}
        </div>
    );
};
export default Tags;
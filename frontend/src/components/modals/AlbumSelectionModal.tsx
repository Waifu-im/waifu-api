import { useState, useEffect } from 'react';
import { Check, Plus } from 'lucide-react';
import api from '../../services/api';
import { AlbumDto, ImageDto, PaginatedList } from '../../types';
import { useNotification } from '../../context/NotificationContext';
import { useAuth } from '../../context/AuthContext';
import Modal from '../Modal';
import AlbumModal, { AlbumFormData } from './AlbumModal';

interface AlbumSelectionModalProps {
    isOpen: boolean;
    onClose: () => void;
    image: ImageDto;
    onUpdate?: (updatedImage: ImageDto) => void;
}

const AlbumSelectionModal = ({ isOpen, onClose, image, onUpdate }: AlbumSelectionModalProps) => {
    const { user } = useAuth();
    const { showNotification } = useNotification();
    const [userAlbums, setUserAlbums] = useState<AlbumDto[]>([]);
    const [isAlbumsLoaded, setIsAlbumsLoaded] = useState(false);
    const [isCreateAlbumOpen, setIsCreateAlbumOpen] = useState(false);

    const loadAlbums = async () => {
        if (isAlbumsLoaded || !user) return;
        try {
            const { data } = await api.get<PaginatedList<AlbumDto>>('/users/me/albums', { params: { pageSize: 100 }, skipGlobalErrorHandler: true });
            setUserAlbums(data.items);
            setIsAlbumsLoaded(true);
        } catch (e) {
            console.error("Failed to load albums", e);
        }
    };

    useEffect(() => {
        if (isOpen) {
            loadAlbums();
        }
    }, [isOpen]);

    const handleToggleAlbum = async (e: React.MouseEvent, targetAlbum: AlbumDto) => {
        e.preventDefault();
        e.stopPropagation();

        if (!user) return;
        const isInAlbum = image.albums?.some(a => a.id === targetAlbum.id);
        const isFavorites = targetAlbum.isDefault;

        try {
            let updatedImage = { ...image };
            
            if (isInAlbum) {
                await api.delete(`/users/${user.id}/albums/${targetAlbum.id}/images/${image.id}`);
                showNotification('success', `Removed from ${targetAlbum.name}`);
                
                updatedImage.albums = image.albums?.filter(a => a.id !== targetAlbum.id) || [];
                
                if (isFavorites) {
                    updatedImage.likedAt = undefined;
                    updatedImage.favorites = Math.max(0, (updatedImage.favorites || 0) - 1);
                }
            } else {
                await api.post(`/users/${user.id}/albums/${targetAlbum.id}/images/${image.id}`);
                showNotification('success', `Added to ${targetAlbum.name}`);
                
                updatedImage.albums = [...(image.albums || []), targetAlbum];

                if (isFavorites) {
                    updatedImage.likedAt = new Date().toISOString();
                    updatedImage.favorites = (updatedImage.favorites || 0) + 1;
                }
            }

            if (onUpdate) {
                onUpdate(updatedImage);
            }
            
            // Update local list to reflect changes if needed
             setUserAlbums([...userAlbums]);
        } catch (e) {
            showNotification('error', `Failed to update album`);
        }
    };

    const handleCreateAlbum = async (data: AlbumFormData) => {
        try {
            const { data: newAlbum } = await api.post<AlbumDto>(`/users/me/albums`, data);
            showNotification('success', 'Album created');
            setIsCreateAlbumOpen(false);
            setUserAlbums(prev => [...prev, newAlbum]);
        } catch { 
            // Error handled globally
        }
    };

    if (!user) return null;

    return (
        <>
            <Modal
                isOpen={isOpen}
                onClose={onClose}
                title="Add to Album"
                maxWidth="max-w-sm"
            >
                <div className="space-y-2">
                    {isAlbumsLoaded ? (
                        userAlbums.length === 0 ? (
                            <div className="text-center py-8 text-muted-foreground">
                                <p>No albums found.</p>
                                <button 
                                    onClick={() => setIsCreateAlbumOpen(true)}
                                    className="mt-4 text-primary font-bold hover:underline"
                                >
                                    Create your first album
                                </button>
                            </div>
                        ) : (
                            <div className="space-y-1 max-h-[60vh] overflow-y-auto -mx-2 px-2">
                                {userAlbums.map(album => {
                                    const isInAlbum = image.albums?.some(a => a.id === album.id);
                                    return (
                                        <div
                                            key={album.id}
                                            onClick={(e) => handleToggleAlbum(e, album)}
                                            className={`flex items-center justify-between p-3 rounded-xl cursor-pointer transition-all border ${
                                                isInAlbum 
                                                    ? 'bg-primary/10 border-primary/20 text-primary' 
                                                    : 'bg-secondary/50 border-transparent hover:bg-secondary hover:border-border'
                                            }`}
                                        >
                                            <span className="font-medium truncate pr-4">{album.name}</span>
                                            {isInAlbum && <Check size={18} className="shrink-0" />}
                                        </div>
                                    );
                                })}
                            </div>
                        )
                    ) : (
                        <div className="py-8 text-center text-muted-foreground">Loading albums...</div>
                    )}

                    {userAlbums.length > 0 && (
                        <button
                            onClick={() => setIsCreateAlbumOpen(true)}
                            className="w-full mt-4 py-3 flex items-center justify-center gap-2 rounded-xl border-2 border-dashed border-border hover:border-primary/50 hover:bg-secondary/50 text-muted-foreground hover:text-foreground transition-all font-medium"
                        >
                            <Plus size={18} /> Create New Album
                        </button>
                    )}
                </div>
            </Modal>

            <AlbumModal 
                isOpen={isCreateAlbumOpen} 
                onClose={() => setIsCreateAlbumOpen(false)} 
                onSubmit={handleCreateAlbum} 
                title="Create New Album" 
                submitLabel="Create" 
            />
        </>
    );
};

export default AlbumSelectionModal;
import { useState, ReactNode } from 'react';
import { Check, Plus } from 'lucide-react';
import api from '../services/api';
import { AlbumDto, ImageDto, PaginatedList } from '../types';
import { Dropdown, DropdownItem, DropdownLabel, DropdownSeparator } from './Dropdown';
import { useNotification } from '../context/NotificationContext';
import { useAuth } from '../context/AuthContext';
import AlbumModal, { AlbumFormData } from './modals/AlbumModal';

interface AlbumDropdownProps {
    image: ImageDto;
    trigger: ReactNode;
    onUpdate?: (updatedImage: ImageDto) => void;
    align?: 'left' | 'right';
    width?: string;
}

const AlbumDropdown = ({ image, trigger, onUpdate, align = 'right', width = 'w-56' }: AlbumDropdownProps) => {
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

    const handleToggleAlbum = async (e: React.MouseEvent, targetAlbum: AlbumDto) => {
        e.preventDefault();
        e.stopPropagation(); // Prevent dropdown from closing

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
            } else {
                // If no onUpdate provided, we mutate the prop object to reflect changes locally 
                // (fallback for ImageCard where we don't easily update parent list state)
                image.albums = updatedImage.albums;
                image.likedAt = updatedImage.likedAt;
                image.favorites = updatedImage.favorites;
                // Force re-render of this component to show checkmark update
                setUserAlbums([...userAlbums]);
            }
        } catch (e) {
            showNotification('error', `Failed to update album`);
        }
    };

    const handleCreateAlbum = async (data: AlbumFormData) => {
        try {
            const { data: newAlbum } = await api.post<AlbumDto>(`/users/me/albums`, data);
            showNotification('success', 'Album created');
            setIsCreateAlbumOpen(false);
            
            // Add new album to list and select it immediately
            setUserAlbums(prev => [...prev, newAlbum]);
            
            // Optionally add image to new album immediately
            // We need to construct a synthetic event or call logic directly
            // For simplicity, let's just add it to the list so user can click it
        } catch { 
            // showNotification('error', 'Failed to create album'); // Handled globally
        }
    };

    if (!user) return null;

    return (
        <>
            <div onMouseEnter={loadAlbums}>
                <Dropdown width={width} align={align} trigger={trigger}>
                    <DropdownLabel>Add to Album</DropdownLabel>
                    {isAlbumsLoaded ? (
                        userAlbums.length === 0 ? (
                            <div className="px-3 py-2 text-xs text-muted-foreground">No albums found.</div>
                        ) : (
                            <div className="max-h-48 overflow-y-auto">
                                {userAlbums.map(album => {
                                    const isInAlbum = image.albums?.some(a => a.id === album.id);
                                    return (
                                        <DropdownItem
                                            key={album.id}
                                            onClick={(e: any) => handleToggleAlbum(e, album)}
                                            icon={isInAlbum ? <Check size={14} className="text-primary"/> : <div className="w-3.5"/>}
                                            active={isInAlbum}
                                        >
                                            <span className="truncate">{album.name}</span>
                                        </DropdownItem>
                                    );
                                })}
                            </div>
                        )
                    ) : (
                        <div className="px-3 py-2 text-xs text-muted-foreground">Loading...</div>
                    )}
                    <DropdownSeparator />
                    <DropdownItem 
                        icon={<Plus size={14} />} 
                        onClick={() => setIsCreateAlbumOpen(true)}
                    >
                        Create New Album
                    </DropdownItem>
                </Dropdown>
            </div>

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

export default AlbumDropdown;
import { useState } from 'react';
import api from '../services/api';
import { Tag, Artist, PaginatedList } from '../types';
import { Option } from '../components/SearchableSelect';
import { useNotification } from '../context/NotificationContext';
import { TagFormData } from '../components/modals/TagModal';
import { ArtistFormData } from '../components/modals/ArtistModal';

const slugify = (text: string) => {
  return text
      .toString()
      .toLowerCase()
      .normalize('NFD')
      .replace(/[\u0300-\u036f]/g, '')
      .replace(/\s+/g, '-')
      .replace(/[^\w-]+/g, '')
      .replace(/--+/g, '-');
};

export const useMetadata = (isReviewMode: boolean = false) => {
    const { showNotification } = useNotification();

    // Tags State
    const [selectedTags, setSelectedTags] = useState<Option[]>([]);
    const [showCreateTagModal, setShowCreateTagModal] = useState(false);
    const [newTagName, setNewTagName] = useState('');

    // Artists State
    const [selectedArtists, setSelectedArtists] = useState<Option[]>([]);
    const [showCreateArtistModal, setShowCreateArtistModal] = useState(false);
    const [newArtistName, setNewArtistName] = useState('');

    // Loaders
    const loadTags = async (query: string) => {
        const { data } = await api.get<PaginatedList<Tag>>('/tags', { 
            params: { search: query, pageSize: 20, reviewStatus: 1 }, 
            skipGlobalErrorHandler: true 
        });
        return data.items.map(t => ({ id: t.id, name: t.name, slug: t.slug, description: t.description }));
    };

    const loadArtists = async (query: string) => {
        const { data } = await api.get<PaginatedList<Artist>>('/artists', { 
            params: { search: query, pageSize: 20, reviewStatus: 1 }, 
            skipGlobalErrorHandler: true 
        });
        return data.items.map(a => ({ id: a.id, name: a.name }));
    };

    // Creators
    const handleCreateTag = async (data: TagFormData) => {
        try {
            // Ensure description is not undefined/null to avoid validation errors if backend requires it
            const payload = { ...data, description: data.description || '' };
            const res = await api.post<Tag>('/tags', payload); // Removed skipGlobalErrorHandler: true
            const newOpt = { id: res.data.id, name: res.data.name, slug: res.data.slug, description: res.data.description };

            setSelectedTags(p => p.some(t => t.id === newOpt.id) ? p : [...p, newOpt]);
            showNotification(isReviewMode ? 'info' : 'success', isReviewMode ? 'Tag submitted for review' : 'Tag created');
            setShowCreateTagModal(false);
        } catch (err: any) {
            if(err.response?.status === 409) {
                try {
                    const targetSlug = data.slug || slugify(data.name);
                    const existingRes = await api.get<Tag>(`/tags/by-slug/${targetSlug}`, { skipGlobalErrorHandler: true });
                    const existing = existingRes.data;
                    const existingOpt = { id: existing.id, name: existing.name, slug: existing.slug, description: existing.description };

                    setSelectedTags(p => p.some(t => t.id === existingOpt.id) ? p : [...p, existingOpt]);
                    showNotification('info', 'Tag already exists, added to selection.');
                    setShowCreateTagModal(false);
                } catch (fetchErr) {
                    showNotification('error', 'Tag already exists but could not be retrieved.');
                }
            } 
            // Other errors are now handled by the global error handler because we removed skipGlobalErrorHandler: true
        }
    };

    const handleCreateArtist = async (data: ArtistFormData) => {
        try {
            const res = await api.post<Artist>('/artists', data); // Removed skipGlobalErrorHandler: true
            const newOption = { id: res.data.id, name: res.data.name };
            setSelectedArtists(p => p.some(a => a.id === newOption.id) ? p : [...p, newOption]);
            showNotification(isReviewMode ? 'info' : 'success', isReviewMode ? 'Artist submitted for review' : 'Artist created');
            setShowCreateArtistModal(false);
        } catch (error: any) {
            if (error.response?.status === 409) {
                try {
                    const existingRes = await api.get<Artist>(`/artists/by-name/${data.name}`, { skipGlobalErrorHandler: true });
                    const existing = existingRes.data;
                    const existingOpt = { id: existing.id, name: existing.name };

                    setSelectedArtists(p => p.some(a => a.id === existingOpt.id) ? p : [...p, existingOpt]);
                    showNotification('info', 'Artist already exists, selected.');
                    setShowCreateArtistModal(false);
                } catch (fetchErr) {
                    showNotification('error', 'Artist exists but could not be retrieved.');
                }
            }
            // Other errors are now handled by the global error handler because we removed skipGlobalErrorHandler: true
        }
    };

    return {
        selectedTags, setSelectedTags,
        selectedArtists, setSelectedArtists,
        showCreateTagModal, setShowCreateTagModal,
        newTagName, setNewTagName,
        showCreateArtistModal, setShowCreateArtistModal,
        newArtistName, setNewArtistName,
        loadTags, loadArtists,
        handleCreateTag, handleCreateArtist,
        slugify
    };
};
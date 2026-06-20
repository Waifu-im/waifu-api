import { useState } from 'react';
import api from '../services/api';
import { Tag, Artist, PaginatedList, ReviewStatusFilter } from '../types';
import { Option } from '../components/SearchableSelect';
import { useNotification } from '../context/NotificationContext';
import { useAuth } from '../context/AuthContext';
import { canModerate } from '../utils/roles';
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

interface UseMetadataOptions {
    isReviewMode?: boolean;
    reviewStatus?: ReviewStatusFilter;
}

export const useMetadata = ({
    isReviewMode = false,
    reviewStatus = ReviewStatusFilter.Accepted,
}: UseMetadataOptions = {}) => {
    const tagsPageSize = 30;
    const artistsPageSize = 30;
    const { showNotification } = useNotification();
    const { user } = useAuth();
    // Regular users get their own pending tags/artists mixed in; moderators use the reviewStatus filter instead.
    const includeMine = user && !canModerate(user) ? true : undefined;

    // Tags State
    const [selectedTags, setSelectedTags] = useState<Option[]>([]);
    const [showCreateTagModal, setShowCreateTagModal] = useState(false);
    const [newTagName, setNewTagName] = useState('');

    // Artists State
    const [selectedArtists, setSelectedArtists] = useState<Option[]>([]);
    const [showCreateArtistModal, setShowCreateArtistModal] = useState(false);
    const [newArtistName, setNewArtistName] = useState('');

    // Loaders
    const loadTags = async (query: string, page: number = 1) => {
        const { data } = await api.get<PaginatedList<Tag>>('/tags', {
            // Authenticated users also get their own pending tags mixed in (so they can attach one they just created).
            params: { name: query, pageSize: tagsPageSize, page, reviewStatus, includeMyPending: includeMine },
            skipGlobalErrorHandler: true
        });
        return data.items.map(t => ({ id: t.id, name: t.name, slug: t.slug, description: t.description, reviewStatus: t.reviewStatus }));
    };

    const loadArtists = async (query: string, page: number = 1) => {
        const { data } = await api.get<PaginatedList<Artist>>('/artists', {
            params: { name: query, pageSize: artistsPageSize, page, reviewStatus, includeMyPending: includeMine },
            skipGlobalErrorHandler: true
        });
        return data.items.map(a => ({ id: a.id, name: a.name, reviewStatus: a.reviewStatus }));
    };

    // Creators
    const handleCreateTag = async (data: TagFormData) => {
        try {
            // Ensure description is not undefined/null to avoid validation errors if backend requires it
            const payload = { ...data, description: data.description || '' };
            const res = await api.post<Tag>('/tags', payload); // Removed skipGlobalErrorHandler: true
            const newOpt = { id: res.data.id, name: res.data.name, slug: res.data.slug, description: res.data.description, reviewStatus: res.data.reviewStatus };

            setSelectedTags(p => p.some(t => t.id === newOpt.id) ? p : [...p, newOpt]);
            showNotification(isReviewMode ? 'info' : 'success', isReviewMode ? 'Tag submitted for review' : 'Tag created');
            setShowCreateTagModal(false);
        } catch (err: any) {
            if(err.response?.status === 409) {
                try {
                    const targetSlug = data.slug || slugify(data.name);
                    const existingRes = await api.get<Tag>(`/tags/by-slug/${targetSlug}`, { skipGlobalErrorHandler: true });
                    const existing = existingRes.data;
                    const existingOpt = { id: existing.id, name: existing.name, slug: existing.slug, description: existing.description, reviewStatus: existing.reviewStatus };

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
            const newOption = { id: res.data.id, name: res.data.name, reviewStatus: res.data.reviewStatus };
            setSelectedArtists(p => p.some(a => a.id === newOption.id) ? p : [...p, newOption]);
            showNotification(isReviewMode ? 'info' : 'success', isReviewMode ? 'Artist submitted for review' : 'Artist created');
            setShowCreateArtistModal(false);
        } catch (error: any) {
            if (error.response?.status === 409) {
                try {
                    const existingRes = await api.get<Artist>(`/artists/by-name/${encodeURIComponent(data.name)}`, { skipGlobalErrorHandler: true });
                    const existing = existingRes.data;
                    const existingOpt = { id: existing.id, name: existing.name, reviewStatus: existing.reviewStatus };

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
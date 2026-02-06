import { useState } from 'react';
import api from '../services/api';
import { ImageDto, ImageFormData } from '../types';

export const useImageUpdate = () => {
    const [isUpdating, setIsUpdating] = useState(false);

    const updateImage = async (imageId: number, data: ImageFormData): Promise<ImageDto> => {
        setIsUpdating(true);
        try {
            const formData = new FormData();
            if (data.file) formData.append('file', data.file);
            if (data.source) formData.append('source', data.source);
            formData.append('isNsfw', String(data.isNsfw));
            if (data.userId) formData.append('userId', String(data.userId));
            data.tags?.forEach(tag => formData.append('tags', tag));
            data.artists?.forEach(artist => formData.append('artists', String(artist)));
            if (data.reviewStatus !== undefined) formData.append('reviewStatus', String(data.reviewStatus));

            const { data: updatedImage } = await api.patch<ImageDto>(`/images/${imageId}`, formData, {
                headers: { 'Content-Type': 'multipart/form-data' }
            });

            return updatedImage;
        } finally {
            setIsUpdating(false);
        }
    };

    return { updateImage, isUpdating };
};

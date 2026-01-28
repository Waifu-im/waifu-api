import { useForm } from 'react-hook-form';
import { useNavigate } from 'react-router-dom';
import api from '../services/api';

interface UploadForm {
  file: FileList;
  tags: string;
  artistName: string;
  source: string;
  isNsfw: boolean;
}

const Upload = () => {
  const { register, handleSubmit, formState: { errors } } = useForm<UploadForm>();
  const navigate = useNavigate();

  const onSubmit = async (data: UploadForm) => {
    const formData = new FormData();
    formData.append('file', data.file[0]);
    
    // Split tags by comma and trim
    const tagList = data.tags.split(',').map(t => t.trim()).filter(t => t);
    tagList.forEach(tag => formData.append('tags', tag));
    
    if (data.artistName) formData.append('artistName', data.artistName);
    if (data.source) formData.append('source', data.source);
    formData.append('isNsfw', String(data.isNsfw));

    try {
      await api.post('/images/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' }
      });
      navigate('/');
    } catch (error) {
      console.error('Upload failed', error);
      alert('Upload failed');
    }
  };

  return (
    <div className="max-w-lg mx-auto mt-10 p-6 bg-white dark:bg-gray-800 rounded-lg shadow-md">
      <h2 className="text-2xl font-bold mb-4">Upload Image</h2>
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div>
          <label className="block text-sm font-medium mb-1">Image File</label>
          <input 
            type="file" 
            accept="image/*"
            {...register('file', { required: true })}
            className="w-full border rounded p-2 dark:bg-gray-700 dark:border-gray-600" 
          />
          {errors.file && <span className="text-red-500 text-sm">File is required</span>}
        </div>
        
        <div>
          <label className="block text-sm font-medium mb-1">Artist Name (Optional)</label>
          <input 
            type="text" 
            {...register('artistName')}
            className="w-full border rounded p-2 dark:bg-gray-700 dark:border-gray-600" 
          />
        </div>

        <div>
          <label className="block text-sm font-medium mb-1">Source URL (Optional)</label>
          <input 
            type="url" 
            {...register('source')}
            className="w-full border rounded p-2 dark:bg-gray-700 dark:border-gray-600" 
          />
        </div>

        <div>
          <label className="block text-sm font-medium mb-1">Tags (comma separated)</label>
          <input 
            type="text" 
            placeholder="tag1, tag2" 
            {...register('tags', { required: true })}
            className="w-full border rounded p-2 dark:bg-gray-700 dark:border-gray-600" 
          />
          {errors.tags && <span className="text-red-500 text-sm">Tags are required</span>}
        </div>

        <div className="flex items-center">
          <input 
            type="checkbox" 
            id="nsfw" 
            {...register('isNsfw')}
            className="mr-2" 
          />
          <label htmlFor="nsfw">NSFW</label>
        </div>

        <button type="submit" className="w-full bg-indigo-600 text-white py-2 px-4 rounded hover:bg-indigo-700 transition">
          Upload
        </button>
      </form>
    </div>
  )
}

export default Upload

import { Link } from 'react-router-dom';
import { useTags } from '../hooks/useTags';
import { Tag as TagIcon } from 'lucide-react';

const Tags = () => {
  const { data: tags, isLoading, error } = useTags();

  if (isLoading) return <div className="text-center p-10">Loading tags...</div>;
  if (error) return <div className="text-center p-10 text-red-500">Failed to load tags.</div>;

  // Group tags by first letter? Or just list them.
  // Let's just list them in a grid for now.

  return (
    <div className="container mx-auto px-4 py-8">
      <h1 className="text-3xl font-bold mb-8 flex items-center gap-3">
        <TagIcon className="text-indigo-600" /> Browse Tags
      </h1>
      
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
        {tags?.map((tag) => (
          <Link 
            key={tag.id} 
            to={`/gallery?includedTags=${tag.name}`}
            className="bg-white dark:bg-gray-800 p-4 rounded-lg shadow-sm hover:shadow-md hover:bg-indigo-50 dark:hover:bg-gray-700 transition-all border border-gray-100 dark:border-gray-700 flex flex-col"
          >
            <span className="font-bold text-lg text-indigo-600 dark:text-indigo-400">{tag.name}</span>
            <span className="text-xs text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">
              {tag.description || 'No description'}
            </span>
            {tag.isNsfw && (
              <span className="mt-2 self-start text-[10px] bg-red-100 text-red-600 px-2 py-0.5 rounded-full font-bold">
                NSFW
              </span>
            )}
          </Link>
        ))}
      </div>
    </div>
  );
};

export default Tags;

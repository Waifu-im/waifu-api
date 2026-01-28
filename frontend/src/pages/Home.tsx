import { useImages } from '../hooks/useImages';

const Home = () => {
  const { data: images, isLoading, error } = useImages();

  if (isLoading) return <div className="text-center p-4">Loading...</div>;
  if (error) return <div className="text-center p-4 text-red-500">Error loading images</div>;

  return (
    <div>
      <h1 className="text-3xl font-bold mb-6">Latest Images</h1>
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
        {images?.map((image) => (
          <div key={image.id} className="bg-white dark:bg-gray-800 rounded-lg overflow-hidden shadow hover:shadow-lg transition">
            <img 
              src={image.url}
              alt={`Image ${image.id}`} 
              className="w-full h-64 object-cover"
              loading="lazy"
            />
            <div className="p-4">
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-500">ID: {image.id}</span>
                <span className="text-sm text-gray-500">❤️ {image.favorites}</span>
              </div>
              <div className="mt-2 flex flex-wrap gap-1">
                {image.tags.slice(0, 3).map(tag => (
                  <span key={tag.id} className="text-xs bg-gray-200 dark:bg-gray-700 px-2 py-1 rounded">
                    {tag.name}
                  </span>
                ))}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

export default Home

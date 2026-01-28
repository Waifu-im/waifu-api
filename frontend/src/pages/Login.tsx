import { useLocation } from 'react-router-dom';

const Login = () => {
  const location = useLocation();

  const handleLogin = () => {
    // Store the redirect path if present
    const from = location.state?.from?.pathname || '/';
    localStorage.setItem('auth_redirect', from);

    const clientId = import.meta.env.VITE_DISCORD_CLIENT_ID;
    const redirectUri = encodeURIComponent(import.meta.env.VITE_DISCORD_REDIRECT_URI);
    const scope = encodeURIComponent('identify email');
    const url = `https://discord.com/api/oauth2/authorize?client_id=${clientId}&redirect_uri=${redirectUri}&response_type=code&scope=${scope}`;
    window.location.href = url;
  };

  return (
    <div className="max-w-md mx-auto mt-10 p-6 bg-white dark:bg-gray-800 rounded-lg shadow-md">
      <h2 className="text-2xl font-bold mb-4">Login</h2>
      <p className="mb-4">Login with Discord to upload images and manage your favorites.</p>
      <button 
        onClick={handleLogin}
        className="w-full bg-indigo-600 text-white py-2 px-4 rounded hover:bg-indigo-700 transition"
      >
        Login with Discord
      </button>
    </div>
  )
}

export default Login

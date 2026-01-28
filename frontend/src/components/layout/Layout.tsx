import { Outlet, Link } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'

const Layout = () => {
  const { isAuthenticated, user, logout } = useAuth();

  return (
    <div className="min-h-screen bg-gray-100 dark:bg-gray-900 text-gray-900 dark:text-gray-100">
      <nav className="bg-white dark:bg-gray-800 shadow p-4">
        <div className="container mx-auto flex justify-between items-center">
          <Link to="/" className="text-xl font-bold text-indigo-600">Waifu API</Link>
          <div className="space-x-4 flex items-center">
            <Link to="/" className="hover:text-indigo-500">Home</Link>
            <Link to="/upload" className="hover:text-indigo-500">Upload</Link>
            
            {isAuthenticated ? (
              <>
                <span className="text-sm font-medium">Hello, {user?.name}</span>
                <button onClick={logout} className="hover:text-red-500">Logout</button>
              </>
            ) : (
              <Link to="/login" className="hover:text-indigo-500">Login</Link>
            )}
          </div>
        </div>
      </nav>
      <main className="container mx-auto p-4">
        <Outlet />
      </main>
    </div>
  )
}

export default Layout

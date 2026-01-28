import { Routes, Route } from 'react-router-dom'
import Home from './pages/Home'
import Login from './pages/Login'
import Upload from './pages/Upload'
import Callback from './pages/Callback'
import Gallery from './pages/Gallery'
import Tags from './pages/Tags'
import Albums from './pages/Albums'
import AlbumPage from './pages/AlbumPage' // Import new page
import ImagePage from './pages/ImagePage'
import Layout from './components/layout/Layout'

function App() {
    return (
        <Routes>
            <Route path="/" element={<Layout />}>
                <Route index element={<Home />} />
                <Route path="gallery" element={<Gallery />} />
                <Route path="albums" element={<Albums />} />
                <Route path="albums/:id" element={<AlbumPage />} /> {/* Route added */}
                <Route path="images/:id" element={<ImagePage />} />
                <Route path="tags" element={<Tags />} />
                <Route path="login" element={<Login />} />
                <Route path="upload" element={<Upload />} />
                <Route path="auth/callback" element={<Callback />} />
            </Route>
        </Routes>
    )
}

export default App
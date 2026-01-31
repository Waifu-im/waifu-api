import { Routes, Route } from 'react-router-dom'
import Home from './pages/Home'
import Login from './pages/Login'
import Upload from './pages/Upload'
import Callback from './pages/Callback'
import Gallery from './pages/Gallery'
import Tags from './pages/Tags'
import Artists from './pages/Artists'
import Albums from './pages/Albums'
import AlbumPage from './pages/AlbumPage'
import ImagePage from './pages/ImagePage'
import Layout from './components/layout/Layout'
import Review from './pages/Review'
import Users from './pages/Users'
import ApiKeys from './pages/ApiKeys'
import Reports from "@/pages/Reports.tsx";
import AdminStats from "@/pages/AdminStats.tsx";
import Contact from './pages/Contact';

function App() {
    return (
        <Routes>
            <Route path="/" element={<Layout />}>
                <Route index element={<Home />} />
                <Route path="gallery" element={<Gallery />} />
                <Route path="albums" element={<Albums />} />
                <Route path="albums/:id" element={<AlbumPage />} />
                <Route path="images/:id" element={<ImagePage />} />
                <Route path="tags" element={<Tags />} />
                <Route path="artists" element={<Artists />} />
                <Route path="login" element={<Login />} />
                <Route path="upload" element={<Upload />} />
                <Route path="review" element={<Review />} />
                <Route path="reports" element={<Reports />} />
                <Route path="users" element={<Users />} />
                <Route path="api-keys" element={<ApiKeys />} />
                <Route path="stats" element={<AdminStats />} />
                <Route path="contact" element={<Contact />} />
                <Route path="auth/callback" element={<Callback />} />
            </Route>
        </Routes>
    )
}

export default App

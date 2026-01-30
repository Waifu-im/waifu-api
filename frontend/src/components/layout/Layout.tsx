import { Outlet, Link, useLocation } from 'react-router-dom';
import { useTheme } from '../../context/ThemeContext';
import { useAuth } from '../../context/AuthContext';
import {
    Sun, Moon, Menu, X, LogOut, Upload as UploadIcon,
    Home, Image as ImageIcon, Tag as TagIcon, ChevronRight, PanelLeft,
    User as UserIcon, Library, ChevronDown, Palette, Key, FileCheck, Users as UsersIcon, Shield, Flag, BarChart, Monitor
} from 'lucide-react';
import { useState } from 'react';
import GlobalErrorHandler from '../GlobalErrorHandler';
import { Dropdown, DropdownItem, DropdownLabel, DropdownSeparator } from '../Dropdown';

const Layout = () => {
    const { theme, setTheme, resolvedTheme } = useTheme();
    const { user, logout } = useAuth();
    const location = useLocation();

    const [isSidebarCollapsed, setIsSidebarCollapsed] = useState(false);
    const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

    const isActive = (path: string) => location.pathname === path;

    const handleLogout = () => {
        logout();
        window.location.href = "/";
    };

    const navItems = [
        { name: 'Home', path: '/', icon: Home },
        { name: 'Gallery', path: '/gallery', icon: ImageIcon },
        { name: 'Albums', path: '/albums', icon: Library },
        { name: 'Tags', path: '/tags', icon: TagIcon },
        { name: 'Artists', path: '/artists', icon: Palette },
    ];

    const isModOrAdmin = user && (user.role === 2 || user.role === 3);
    const isAdmin = user && user.role === 3;

    return (
        <div className="min-h-screen bg-background text-foreground flex flex-col">
            <GlobalErrorHandler />

            {/* 1. TOP NAVBAR */}
            <header className="sticky top-0 z-50 w-full h-16 border-b border-border bg-card/95 backdrop-blur flex items-center justify-between px-4 lg:px-6 shadow-sm">
                <div className="flex items-center gap-4">
                    <button
                        className="lg:hidden p-2 rounded-md hover:bg-secondary"
                        onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
                    >
                        <Menu size={24} />
                    </button>

                    <Link to="/" className="flex items-center gap-2 group">
                        <img src="/favicon.png" alt="Logo" className="w-8 h-8 rounded-lg shadow-lg group-hover:scale-105 transition-transform" />
                        <span className="text-xl font-bold tracking-tight hidden sm:block">WAIFU.IM</span>
                    </Link>
                </div>

                <div className="flex items-center gap-2 sm:gap-4">
                    {/* Theme Toggle Dropdown */}
                    <Dropdown
                        width="w-36"
                        trigger={
                            <button className="p-2.5 rounded-full hover:bg-secondary text-muted-foreground hover:text-foreground transition-colors">
                                {resolvedTheme === 'light' ? <Sun size={20} /> : <Moon size={20} />}
                            </button>
                        }
                    >
                        <DropdownItem onClick={() => setTheme('light')} active={theme === 'light'} icon={<Sun size={16} />}>Light</DropdownItem>
                        <DropdownItem onClick={() => setTheme('dark')} active={theme === 'dark'} icon={<Moon size={16} />}>Dark</DropdownItem>
                        <DropdownItem onClick={() => setTheme('system')} active={theme === 'system'} icon={<Monitor size={16} />}>System</DropdownItem>
                    </Dropdown>

                    {user ? (
                        <Dropdown
                            width="w-64"
                            trigger={
                                <button className="flex items-center gap-3 pl-2 pr-3 py-1.5 rounded-full hover:bg-secondary transition-colors border border-transparent hover:border-border max-w-[200px]">
                                    {user.avatarUrl ? (
                                        <img src={user.avatarUrl} alt={user.name} className="w-8 h-8 shrink-0 rounded-full object-cover border border-primary/20" />
                                    ) : (
                                        <div className="w-8 h-8 shrink-0 rounded-full bg-primary/10 flex items-center justify-center text-primary border border-primary/20">
                                            <UserIcon size={16} />
                                        </div>
                                    )}
                                    <div className="text-right hidden md:block overflow-hidden">
                                        <p className="text-xs font-bold leading-none truncate max-w-[100px]">{user.name}</p>
                                        <p className="text-[10px] text-muted-foreground mt-0.5 uppercase tracking-wide font-semibold">
                                            {user.role === 3 ? 'Admin' : user.role === 2 ? 'Mod' : 'User'}
                                        </p>
                                    </div>
                                    <ChevronDown size={14} className="text-muted-foreground shrink-0" />
                                </button>
                            }
                        >
                            {/* Header Mobile */}
                            <div className="px-4 py-3 border-b border-border mb-1 md:hidden bg-secondary/30">
                                <p className="font-bold truncate">{user.name}</p>
                                <p className="text-xs text-muted-foreground">{user.role === 3 ? 'Administrator' : 'Member'}</p>
                            </div>

                            <DropdownLabel>Personal</DropdownLabel>
                            <Link to="/upload"><DropdownItem icon={<UploadIcon size={16} />}>Upload Image</DropdownItem></Link>
                            <Link to="/albums"><DropdownItem icon={<Library size={16} />}>My Albums</DropdownItem></Link>
                            <Link to="/api-keys"><DropdownItem icon={<Key size={16} />}>API Keys</DropdownItem></Link>

                            {isModOrAdmin && (
                                <>
                                    <DropdownSeparator />
                                    <DropdownLabel>Administration</DropdownLabel>
                                    <Link to="/review"><DropdownItem icon={<FileCheck size={16} className="text-orange-500" />}>Moderation Queue</DropdownItem></Link>
                                    <Link to="/reports"><DropdownItem icon={<Flag size={16} className="text-red-500" />}>Reports</DropdownItem></Link>
                                    {isAdmin && (
                                        <>
                                            <Link to="/users"><DropdownItem icon={<UsersIcon size={16} className="text-blue-500" />}>User Management</DropdownItem></Link>
                                            <Link to="/stats"><DropdownItem icon={<BarChart size={16} className="text-purple-500" />}>Statistics</DropdownItem></Link>
                                        </>
                                    )}
                                </>
                            )}

                            <DropdownSeparator />
                            <DropdownItem onClick={handleLogout} icon={<LogOut size={16} />} danger>Log out</DropdownItem>
                        </Dropdown>
                    ) : (
                        <Link
                            to="/login"
                            className="px-5 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-bold hover:opacity-90 transition-all shadow-sm"
                        >
                            Login
                        </Link>
                    )}
                </div>
            </header>

            {/* 2. MAIN LAYOUT */}
            <div className="flex flex-1 overflow-hidden relative">

                {/* DESKTOP SIDEBAR - Cleaned up */}
                <aside
                    className={`hidden lg:flex flex-col border-r border-border bg-card transition-all duration-300 ease-in-out ${
                        isSidebarCollapsed ? 'w-20' : 'w-64'
                    }`}
                >
                    <nav className="flex-1 p-4 space-y-2 overflow-y-auto">
                        {navItems.map((item) => (
                            <Link
                                key={item.path}
                                to={item.path}
                                className={`flex items-center gap-3 px-3 py-3 rounded-xl transition-all group relative overflow-hidden ${
                                    isActive(item.path)
                                        ? 'bg-primary text-primary-foreground font-bold shadow-md'
                                        : 'text-muted-foreground hover:bg-secondary hover:text-foreground'
                                } ${isSidebarCollapsed ? 'justify-center' : ''}`}
                                title={isSidebarCollapsed ? item.name : ''}
                            >
                                <item.icon size={22} className="relative z-10 shrink-0" />
                                {!isSidebarCollapsed && <span className="relative z-10 whitespace-nowrap">{item.name}</span>}
                            </Link>
                        ))}

                        <div className="my-4 border-t border-border mx-2 opacity-50"></div>

                        <Link
                            to="/upload"
                            className={`flex items-center gap-3 px-3 py-3 rounded-xl transition-all ${
                                isActive('/upload')
                                    ? 'bg-secondary text-foreground font-bold'
                                    : 'bg-secondary/30 text-foreground hover:bg-secondary font-medium'
                            } ${isSidebarCollapsed ? 'justify-center' : ''}`}
                        >
                            <UploadIcon size={22} className="shrink-0" />
                            {!isSidebarCollapsed && <span>Upload</span>}
                        </Link>
                    </nav>

                    <div className="p-4 border-t border-border mt-auto">
                        <button
                            onClick={() => setIsSidebarCollapsed(!isSidebarCollapsed)}
                            className="w-full flex items-center justify-center p-2 rounded-lg text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors"
                        >
                            {isSidebarCollapsed ? <ChevronRight size={20} /> : <PanelLeft size={20} />}
                        </button>
                    </div>
                </aside>

                {/* MOBILE DRAWER */}
                {isMobileMenuOpen && (
                    <div className="fixed inset-0 z-50 lg:hidden">
                        <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setIsMobileMenuOpen(false)} />
                        <div className="absolute inset-y-0 left-0 w-full sm:w-80 bg-background border-r border-border shadow-2xl flex flex-col p-4 animate-in slide-in-from-left">
                            <div className="flex justify-between items-center mb-8">
                                <span className="font-bold text-xl">Menu</span>
                                <button onClick={() => setIsMobileMenuOpen(false)} className="p-2 hover:bg-secondary rounded-lg"><X size={24} /></button>
                            </div>
                            <nav className="space-y-2 flex-1 overflow-y-auto">
                                {navItems.map((item) => (
                                    <Link
                                        key={item.path}
                                        to={item.path}
                                        onClick={() => setIsMobileMenuOpen(false)}
                                        className={`flex items-center gap-3 px-4 py-3 rounded-xl font-medium ${
                                            isActive(item.path) ? 'bg-secondary text-foreground' : 'text-muted-foreground'
                                        }`}
                                    >
                                        <item.icon size={20} /> {item.name}
                                    </Link>
                                ))}

                                <div className="my-2 border-t border-border opacity-50"></div>

                                {/* ADDED: Upload Button for Mobile */}
                                <Link
                                    to="/upload"
                                    onClick={() => setIsMobileMenuOpen(false)}
                                    className={`flex items-center gap-3 px-4 py-3 rounded-xl font-medium ${
                                        isActive('/upload') ? 'bg-secondary text-foreground' : 'text-muted-foreground'
                                    }`}
                                >
                                    <UploadIcon size={20} /> Upload
                                </Link>
                            </nav>
                        </div>
                    </div>
                )}

                {/* CONTENT AREA */}
                <main className="flex-1 overflow-x-hidden overflow-y-auto bg-muted/20 h-[calc(100vh-4rem)] relative">
                    <Outlet />
                </main>
            </div>
        </div>
    );
};

export default Layout;
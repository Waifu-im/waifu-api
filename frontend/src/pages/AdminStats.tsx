import { useState, useEffect } from 'react';
import api from '../services/api';
import { BarChart, Activity, Calendar, Image as ImageIcon, Tag as TagIcon, Users, Trophy, ShieldCheck, UserX, Key, Globe, Scale } from 'lucide-react';
import { useRequireAuth } from '../hooks/useRequireAuth';
import { Role } from '../types';

interface DailyStat {
    date: string;
    count: number;
}

interface UserStat {
    id: number;
    name: string;
    requestCount: number;
    apiKeyRequestCount: number;
    jwtRequestCount: number;
}

interface AdminStats {
    requestsToday: number;
    history: DailyStat[];
    topUsers: UserStat[];
    topApiKeyUsers: UserStat[];
    topJwtUsers: UserStat[];
    totalRequests: number;
    totalAuthenticatedRequests: number;
    totalUnauthenticatedRequests: number;
    totalApiKeyRequests: number;
    totalJwtRequests: number;
    totalImages: number;
    totalTags: number;
    totalArtists: number;
}

const AdminStats = () => {
    const user = useRequireAuth();
    const [stats, setStats] = useState<AdminStats | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        if (user && user.role === Role.Admin) {
            api.get<AdminStats>('/stats/admin')
                .then(res => setStats(res.data))
                .catch(console.error)
                .finally(() => setLoading(false));
        }
    }, [user]);

    if (!user || user.role !== Role.Admin) return <div className="p-10 text-center">Access Denied</div>;
    if (loading) return <div className="p-10 text-center">Loading stats...</div>;

    const history = stats?.history || [];
    
    // Round max count to nearest nice number (e.g. 448 -> 500, 671 -> 700)
    const rawMax = history.length ? Math.max(...history.map(h => h.count)) : 1;
    const magnitude = Math.pow(10, Math.floor(Math.log10(rawMax)));
    const maxCount = Math.ceil(rawMax / magnitude) * magnitude;
    
    const totalRequestsYear = history.reduce((acc, curr) => acc + curr.count, 0);

    // Calculate difference
    const diffAuth = Math.abs((stats?.totalAuthenticatedRequests || 0) - (stats?.totalUnauthenticatedRequests || 0));
    const isAuthMore = (stats?.totalAuthenticatedRequests || 0) >= (stats?.totalUnauthenticatedRequests || 0);

    // Generate SVG Points for Line Chart
    const points = history.map((h, i) => {
        const x = ((i + 0.5) / history.length) * 100;
        const y = 100 - ((h.count / maxCount) * 100);
        return `${x},${y}`;
    }).join(' ');

    const firstX = history.length ? (0.5 / history.length) * 100 : 0;
    const lastX = history.length ? ((history.length - 0.5) / history.length) * 100 : 100;
    const areaPoints = `${points} ${lastX},100 ${firstX},100`;

    return (
        <div className="container mx-auto p-6 md:p-10">
            <div className="flex items-center gap-3 mb-8">
                <BarChart className="text-primary" size={32} />
                <h1 className="text-3xl font-black text-foreground">Admin Dashboard</h1>
            </div>

            {/* SUMMARY CARDS */}
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6 mb-8">
                <div className="bg-card border border-border p-6 rounded-xl shadow-sm flex items-center gap-4">
                    <div className="p-3 bg-primary/10 text-primary rounded-full shrink-0">
                        <Activity size={32} />
                    </div>
                    <div className="min-w-0">
                        <p className="text-sm text-muted-foreground font-bold uppercase tracking-wider">Requests Today</p>
                        <p className="text-2xl xl:text-3xl font-black truncate" title={stats?.requestsToday.toLocaleString()}>{stats?.requestsToday.toLocaleString()}</p>
                    </div>
                </div>

                <div className="bg-card border border-border p-6 rounded-xl shadow-sm flex items-center gap-4">
                    <div className="p-3 bg-blue-500/10 text-blue-500 rounded-full shrink-0">
                        <ImageIcon size={32} />
                    </div>
                    <div className="min-w-0">
                        <p className="text-sm text-muted-foreground font-bold uppercase tracking-wider">Total Images</p>
                        <p className="text-2xl xl:text-3xl font-black truncate" title={stats?.totalImages.toLocaleString()}>{stats?.totalImages.toLocaleString()}</p>
                    </div>
                </div>

                <div className="bg-card border border-border p-6 rounded-xl shadow-sm flex items-center gap-4">
                    <div className="p-3 bg-green-500/10 text-green-500 rounded-full shrink-0">
                        <TagIcon size={32} />
                    </div>
                    <div className="min-w-0">
                        <p className="text-sm text-muted-foreground font-bold uppercase tracking-wider">Total Tags</p>
                        <p className="text-2xl xl:text-3xl font-black truncate" title={stats?.totalTags.toLocaleString()}>{stats?.totalTags.toLocaleString()}</p>
                    </div>
                </div>

                <div className="bg-card border border-border p-6 rounded-xl shadow-sm flex items-center gap-4">
                    <div className="p-3 bg-purple-500/10 text-purple-500 rounded-full shrink-0">
                        <Users size={32} />
                    </div>
                    <div className="min-w-0">
                        <p className="text-sm text-muted-foreground font-bold uppercase tracking-wider">Total Artists</p>
                        <p className="text-2xl xl:text-3xl font-black truncate" title={stats?.totalArtists.toLocaleString()}>{stats?.totalArtists.toLocaleString()}</p>
                    </div>
                </div>
            </div>

            {/* CHART SECTION - Full Width */}
            <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm p-6 mb-8">
                <div className="mb-6 flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
                    <h2 className="text-xl font-bold flex items-center gap-2">
                        <Calendar size={20} className="text-muted-foreground" /> Request History (Last Year)
                    </h2>
                    <div className="flex gap-6 text-sm">
                        <div>
                            <span className="text-muted-foreground">This Year: </span>
                            <span className="font-bold text-foreground">{totalRequestsYear.toLocaleString()}</span>
                        </div>
                        <div>
                            <span className="text-muted-foreground">All Time: </span>
                            <span className="font-bold text-foreground">{stats?.totalRequests.toLocaleString()}</span>
                        </div>
                    </div>
                </div>
                
                <div className="relative h-[400px] w-full pt-10 pb-6 pl-12 pr-2">
                    {/* Y-Axis Labels (Regular Intervals) */}
                    <div className="absolute left-0 top-10 bottom-6 w-10 flex flex-col justify-between text-[10px] text-muted-foreground font-mono text-right pr-2 select-none pointer-events-none">
                        <span>{maxCount.toLocaleString()}</span>
                        <span>{Math.round(maxCount * 0.75).toLocaleString()}</span>
                        <span>{Math.round(maxCount * 0.5).toLocaleString()}</span>
                        <span>{Math.round(maxCount * 0.25).toLocaleString()}</span>
                        <span>0</span>
                    </div>

                    {/* Horizontal Grid Lines */}
                    <div className="absolute left-12 right-2 top-10 bottom-6 flex flex-col justify-between pointer-events-none">
                        <div className="border-t border-border/50 w-full h-0"></div>
                        <div className="border-t border-border/50 w-full h-0"></div>
                        <div className="border-t border-border/50 w-full h-0"></div>
                        <div className="border-t border-border/50 w-full h-0"></div>
                        <div className="border-t border-border/50 w-full h-0"></div>
                    </div>

                    {history.length > 0 ? (
                        <>
                            <svg className="absolute inset-0 w-full h-full pl-12 pr-2 pt-10 pb-6 text-primary" viewBox="0 0 100 100" preserveAspectRatio="none">
                                <defs>
                                    <linearGradient id="chartGradient" x1="0" y1="0" x2="0" y2="1">
                                        <stop offset="0%" stopColor="currentColor" stopOpacity="0.3" />
                                        <stop offset="100%" stopColor="currentColor" stopOpacity="0" />
                                    </linearGradient>
                                </defs>
                                <polygon points={areaPoints} fill="url(#chartGradient)" />
                                <polyline points={points} fill="none" stroke="currentColor" strokeWidth="2" vectorEffect="non-scaling-stroke" strokeLinecap="round" strokeLinejoin="round" />
                            </svg>

                            <div className="absolute inset-0 flex items-end w-full h-full pl-12 pr-2 pt-10 pb-6">
                                {history.map((day, i) => {
                                    const date = new Date(day.date);
                                    const isMonthStart = date.getDate() === 1;
                                    
                                    return (
                                        <div key={i} className="flex-1 h-full group relative flex flex-col justify-end">
                                            <div className="absolute top-0 bottom-0 left-1/2 w-[1px] bg-foreground/20 opacity-0 group-hover:opacity-100 transition-opacity"></div>
                                            <div 
                                                className="absolute left-1/2 -translate-x-1/2 w-3 h-3 bg-primary rounded-full border-2 border-background opacity-0 group-hover:opacity-100 transition-opacity z-10"
                                                style={{ bottom: `${(day.count / maxCount) * 100}%`, marginBottom: '-6px' }}
                                            ></div>
                                            <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 hidden group-hover:block z-20 bg-popover text-popover-foreground text-xs p-2 rounded shadow-lg border border-border whitespace-nowrap pointer-events-none">
                                                <p className="font-bold">{date.toLocaleDateString()}</p>
                                                <p>{day.count.toLocaleString()} requests</p>
                                            </div>
                                            {isMonthStart && (
                                                <div className="absolute top-full mt-2 text-[10px] text-muted-foreground font-bold -translate-x-1/2 left-1/2 pointer-events-none">
                                                    {date.toLocaleDateString(undefined, { month: 'short' })}
                                                </div>
                                            )}
                                        </div>
                                    );
                                })}
                            </div>
                        </>
                    ) : (
                        <div className="h-full flex items-center justify-center text-muted-foreground">
                            No history data available.
                        </div>
                    )}
                </div>
            </div>

            {/* DETAILED STATS GRID */}
            <div className="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-4 gap-6">
                
                {/* AUTH STATS */}
                <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm p-6">
                    <h2 className="text-xl font-bold flex items-center gap-2 mb-6">
                        <Activity size={20} className="text-muted-foreground" /> Traffic Source
                    </h2>
                    <div className="space-y-4">
                        <div className="flex items-center justify-between p-3 bg-secondary/30 rounded-lg">
                            <div className="flex items-center gap-3 min-w-0">
                                <div className="p-2 bg-green-500/10 text-green-600 rounded-lg shrink-0">
                                    <ShieldCheck size={18} />
                                </div>
                                <span className="font-medium truncate">Authenticated</span>
                            </div>
                            <span className="font-mono font-bold shrink-0">{stats?.totalAuthenticatedRequests.toLocaleString()}</span>
                        </div>
                        
                        <div className="pl-4 space-y-2 border-l-2 border-border ml-4">
                            <div className="flex items-center justify-between text-sm">
                                <div className="flex items-center gap-2 text-muted-foreground min-w-0">
                                    <Key size={14} className="shrink-0"/>
                                    <span className="truncate">API Key</span>
                                </div>
                                <span className="font-mono shrink-0">{stats?.totalApiKeyRequests.toLocaleString()}</span>
                            </div>
                            <div className="flex items-center justify-between text-sm">
                                <div className="flex items-center gap-2 text-muted-foreground min-w-0">
                                    <Globe size={14} className="shrink-0"/>
                                    <span className="truncate">Web (JWT)</span>
                                </div>
                                <span className="font-mono shrink-0">{stats?.totalJwtRequests.toLocaleString()}</span>
                            </div>
                        </div>

                        <div className="flex items-center justify-between p-3 bg-secondary/30 rounded-lg">
                            <div className="flex items-center gap-3 min-w-0">
                                <div className="p-2 bg-orange-500/10 text-orange-600 rounded-lg shrink-0">
                                    <UserX size={18} />
                                </div>
                                <span className="font-medium truncate">Anonymous</span>
                            </div>
                            <span className="font-mono font-bold shrink-0">{stats?.totalUnauthenticatedRequests.toLocaleString()}</span>
                        </div>

                        <div className="flex items-center justify-between p-3 bg-secondary/30 rounded-lg">
                            <div className="flex items-center gap-3 min-w-0">
                                <div className="p-2 bg-gray-500/10 text-gray-600 rounded-lg shrink-0">
                                    <Scale size={18} />
                                </div>
                                <span className="font-medium truncate">Difference ({isAuthMore ? 'Auth' : 'Anon'})</span>
                            </div>
                            <span className="font-mono font-bold shrink-0">{diffAuth.toLocaleString()}</span>
                        </div>
                    </div>
                </div>

                {/* TOP USERS */}
                <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm p-6">
                    <h2 className="text-xl font-bold flex items-center gap-2 mb-6">
                        <Trophy size={20} className="text-yellow-500" /> Top Users
                    </h2>
                    <div className="space-y-4">
                        {stats?.topUsers.map((u, i) => (
                            <div key={u.id} className="flex flex-col p-3 bg-secondary/30 rounded-lg gap-2">
                                <div className="flex items-center justify-between">
                                    <div className="flex items-center gap-3 min-w-0">
                                        <div className={`w-6 h-6 rounded-full flex items-center justify-center font-bold text-xs shrink-0 ${i === 0 ? 'bg-yellow-500/20 text-yellow-600' : i === 1 ? 'bg-gray-400/20 text-gray-500' : i === 2 ? 'bg-orange-500/20 text-orange-600' : 'bg-secondary text-muted-foreground'}`}>
                                            {i + 1}
                                        </div>
                                        <span className="font-medium truncate" title={u.name}>{u.name}</span>
                                    </div>
                                    <span className="font-mono text-sm font-bold shrink-0">{u.requestCount.toLocaleString()}</span>
                                </div>
                                <div className="flex gap-3 text-[10px] text-muted-foreground pl-9">
                                    <span title="API Key Requests" className="flex items-center gap-1"><Key size={10}/> {u.apiKeyRequestCount.toLocaleString()}</span>
                                    <span title="Web Requests" className="flex items-center gap-1"><Globe size={10}/> {u.jwtRequestCount.toLocaleString()}</span>
                                </div>
                            </div>
                        ))}
                        {(!stats?.topUsers || stats.topUsers.length === 0) && (
                            <div className="text-center text-muted-foreground py-4">No user data yet.</div>
                        )}
                    </div>
                </div>

                {/* TOP API KEY USERS */}
                <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm p-6">
                    <h2 className="text-xl font-bold flex items-center gap-2 mb-6">
                        <Key size={20} className="text-blue-500" /> Top API Key Users
                    </h2>
                    <div className="space-y-4">
                        {stats?.topApiKeyUsers.map((u, i) => (
                            <div key={u.id} className="flex items-center justify-between p-3 bg-secondary/30 rounded-lg">
                                <div className="flex items-center gap-3 min-w-0">
                                    <div className={`w-6 h-6 rounded-full flex items-center justify-center font-bold text-xs shrink-0 ${i === 0 ? 'bg-blue-500/20 text-blue-600' : 'bg-secondary text-muted-foreground'}`}>
                                        {i + 1}
                                    </div>
                                    <span className="font-medium truncate" title={u.name}>{u.name}</span>
                                </div>
                                <span className="font-mono text-sm font-bold shrink-0">{u.apiKeyRequestCount.toLocaleString()}</span>
                            </div>
                        ))}
                        {(!stats?.topApiKeyUsers || stats.topApiKeyUsers.length === 0) && (
                            <div className="text-center text-muted-foreground py-4">No data yet.</div>
                        )}
                    </div>
                </div>

                {/* TOP JWT USERS */}
                <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm p-6">
                    <h2 className="text-xl font-bold flex items-center gap-2 mb-6">
                        <Globe size={20} className="text-purple-500" /> Top Web Users
                    </h2>
                    <div className="space-y-4">
                        {stats?.topJwtUsers.map((u, i) => (
                            <div key={u.id} className="flex items-center justify-between p-3 bg-secondary/30 rounded-lg">
                                <div className="flex items-center gap-3 min-w-0">
                                    <div className={`w-6 h-6 rounded-full flex items-center justify-center font-bold text-xs shrink-0 ${i === 0 ? 'bg-purple-500/20 text-purple-600' : 'bg-secondary text-muted-foreground'}`}>
                                        {i + 1}
                                    </div>
                                    <span className="font-medium truncate" title={u.name}>{u.name}</span>
                                </div>
                                <span className="font-mono text-sm font-bold shrink-0">{u.jwtRequestCount.toLocaleString()}</span>
                            </div>
                        ))}
                        {(!stats?.topJwtUsers || stats.topJwtUsers.length === 0) && (
                            <div className="text-center text-muted-foreground py-4">No data yet.</div>
                        )}
                    </div>
                </div>

            </div>
        </div>
    );
};

export default AdminStats;
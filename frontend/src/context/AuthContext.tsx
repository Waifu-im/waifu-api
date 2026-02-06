import React, { createContext, useContext, useState, useEffect, useMemo } from 'react';
import { Role, User } from '../types';
import { useUser } from '../hooks/useUser';

/**
 * Decode a JWT token and extract the payload.
 * Returns null if the token is invalid.
 */
function decodeJwtPayload(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    const payload = atob(parts[1].replace(/-/g, '+').replace(/_/g, '/'));
    return JSON.parse(payload);
  } catch {
    return null;
  }
}

/**
 * Extract the role from a JWT token.
 * The role claim is stored as "http://schemas.microsoft.com/ws/2008/06/identity/claims/role"
 */
function getRoleFromToken(token: string): Role | null {
  const payload = decodeJwtPayload(token);
  if (!payload) return null;

  const roleClaim = payload['http://schemas.microsoft.com/ws/2008/06/identity/claims/role'];
  if (typeof roleClaim === 'string' && Object.values(Role).includes(roleClaim as Role)) {
    return roleClaim as Role;
  }
  return null;
}

interface AuthContextType {
  token: string | null;
  user: User | null;
  login: (token: string) => void;
  logout: () => void;
  isAuthenticated: boolean;
  isLoading: boolean;
  /** True when the role in the JWT token differs from the role returned by the API */
  roleOutOfSync: boolean;
}

const AuthContext = createContext<AuthContextType | null>(null);

export const AuthProvider = ({ children }: { children: React.ReactNode }) => {
  const [token, setToken] = useState<string | null>(localStorage.getItem('token'));

  // Use React Query to fetch user
  const { data: user, isLoading: userLoading, isError } = useUser(!!token);

  // If no token, we are not loading (we are logged out).
  // If token exists, we are loading until userLoading is false.
  const isLoading = !!token && userLoading;

  // Check if the role in the token differs from the role in the API response
  const roleOutOfSync = useMemo(() => {
    if (!token || !user) return false;
    const tokenRole = getRoleFromToken(token);
    return tokenRole !== null && tokenRole !== user.role;
  }, [token, user]);

  useEffect(() => {
    if (token) {
      localStorage.setItem('token', token);
    } else {
      localStorage.removeItem('token');
    }
  }, [token]);

  // Handle token expiration / invalid token
  useEffect(() => {
    if (isError) {
      console.log('User fetch failed, logging out.');
      logout();
    }
  }, [isError]);

  const login = (newToken: string) => {
    setToken(newToken);
  };

  const logout = () => {
    setToken(null);
    localStorage.removeItem('token');
    // Clear React Query cache if possible or rely on page reload in UI component
  };

  return (
      <AuthContext.Provider value={{
        token,
        user: user || null,
        login,
        logout,
        isAuthenticated: !!user,
        isLoading,
        roleOutOfSync
      }}>
        {children}
      </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

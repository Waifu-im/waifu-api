import React, { createContext, useContext, useState, useEffect } from 'react';
import { User } from '../types';
import { useUser } from '../hooks/useUser';

interface AuthContextType {
  token: string | null;
  user: User | null;
  login: (token: string) => void;
  logout: () => void;
  isAuthenticated: boolean;
  isLoading: boolean;
}

const AuthContext = createContext<AuthContextType | null>(null);

export const AuthProvider = ({ children }: { children: React.ReactNode }) => {
  const [token, setToken] = useState<string | null>(localStorage.getItem('token'));

  // Use React Query to fetch user
  const { data: user, isLoading, isError } = useUser(!!token);

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
        isLoading
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
import { useEffect, useCallback } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';

/**
 * Hook for protecting entire pages - redirects automatically if not authenticated
 */
export const useRequireAuth = (redirectUrl = '/login', message = 'You must be logged in to access this page.') => {
  const { user, isLoading } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const { showNotification } = useNotification();

  useEffect(() => {
    // Only redirect if we are sure the user is not authenticated (isLoading is false)
    if (!isLoading && !user) {
      showNotification('warning', message);
      // Use replace to avoid building up history stack with redirects
      navigate(redirectUrl, { state: { from: location }, replace: true });
    }
  }, [user, isLoading, navigate, location, showNotification, redirectUrl, message]);

  return user;
};

/**
 * Hook for protecting actions (buttons, etc.) - returns a function to check auth on demand
 * Usage: const checkAuth = useAuthGuard();
 *        if (!checkAuth('You must be logged in to like images.')) return;
 */
export const useAuthGuard = () => {
  const { user } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const { showNotification } = useNotification();

  const checkAuth = useCallback((message = 'You must be logged in to perform this action.') => {
    if (!user) {
      showNotification('warning', message);
      navigate('/login', { state: { from: location } });
      return false;
    }
    return true;
  }, [user, navigate, location, showNotification]);

  return checkAuth;
};

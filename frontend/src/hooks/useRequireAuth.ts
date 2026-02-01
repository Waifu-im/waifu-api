import { useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';

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

import { useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';

export const useRequireAuth = (redirectUrl = '/login', message = 'You must be logged in to access this page.') => {
  const { user } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const { showNotification } = useNotification();

  useEffect(() => {
    if (!user) {
      showNotification('warning', message);
      navigate(redirectUrl, { state: { from: location } });
    }
  }, [user, navigate, location, showNotification, redirectUrl, message]);

  return user;
};
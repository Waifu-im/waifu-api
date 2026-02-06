import { useEffect, useRef } from 'react';
import { useAuth } from '../context/AuthContext';
import { useNotification } from '../context/NotificationContext';

/**
 * Component that shows a warning when the user's role in the JWT token
 * differs from the role returned by the API (e.g., after an admin changed
 * the user's role in the database).
 */
export const RoleSyncWarning = () => {
  const { roleOutOfSync } = useAuth();
  const { showNotification } = useNotification();
  const hasShownWarning = useRef(false);

  useEffect(() => {
    if (roleOutOfSync && !hasShownWarning.current) {
      hasShownWarning.current = true;
      showNotification(
        'warning',
        'Your role has been updated. Please log out and log back in to apply the changes.'
      );
    }

    // Reset when sync is restored (e.g., after re-login)
    if (!roleOutOfSync) {
      hasShownWarning.current = false;
    }
  }, [roleOutOfSync, showNotification]);

  return null;
};

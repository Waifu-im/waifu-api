import { useEffect } from 'react';
import { useNotification } from '../context/NotificationContext';
import { globalErrorBus } from '../services/eventBus';

const GlobalErrorHandler = () => {
  const { showNotification } = useNotification();

  useEffect(() => {
    const unsubscribe = globalErrorBus.subscribe((event) => {
      showNotification(event.type, event.message);
    });

    return () => {
      unsubscribe();
    };
  }, [showNotification]);

  return null;
};

export default GlobalErrorHandler;

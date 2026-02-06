import { createContext, useContext, useState, ReactNode, useCallback } from 'react';
import Notification, { NotificationType } from '../components/Notification';

interface NotificationItem {
  id: string;
  type: NotificationType;
  message: string;
}

interface NotificationContextType {
  showNotification: (type: NotificationType, message: string) => void;
}

const NotificationContext = createContext<NotificationContextType | undefined>(undefined);

export const NotificationProvider = ({ children }: { children: ReactNode }) => {
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);

  const showNotification = useCallback((type: NotificationType, message: string) => {
    const id = Math.random().toString(36).substring(2, 9);
    setNotifications(prev => [...prev, { id, type, message }]);
  }, []);

  const removeNotification = useCallback((id: string) => {
    setNotifications(prev => prev.filter(n => n.id !== id));
  }, []);

  return (
      <NotificationContext.Provider value={{ showNotification }}>
        {children}
        {/* Correction: z-[100] pour être au-dessus des Modals (z-50/z-60).
          Ajout de pointer-events-none sur le conteneur pour ne pas bloquer les clics en dessous,
          et pointer-events-auto sur les notifications elles-mêmes.
      */}
        <div className="fixed bottom-4 right-4 z-[100] flex flex-col items-end space-y-2 pointer-events-none">
          {notifications.map(notification => (
              <div key={notification.id} className="pointer-events-auto">
                <Notification
                    id={notification.id}
                    type={notification.type}
                    message={notification.message}
                    onClose={removeNotification}
                />
              </div>
          ))}
        </div>
      </NotificationContext.Provider>
  );
};

export const useNotification = () => {
  const context = useContext(NotificationContext);
  if (context === undefined) {
    throw new Error('useNotification must be used within a NotificationProvider');
  }
  return context;
};
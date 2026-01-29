import { useEffect } from 'react';
import { X, CheckCircle, AlertCircle, Info, AlertTriangle } from 'lucide-react';

export type NotificationType = 'success' | 'error' | 'info' | 'warning';

export interface NotificationProps {
  id: string;
  type: NotificationType;
  message: string;
  onClose: (id: string) => void;
  duration?: number;
}

const Notification = ({ id, type, message, onClose, duration = 5000 }: NotificationProps) => {
  useEffect(() => {
    const timer = setTimeout(() => {
      onClose(id);
    }, duration);

    return () => clearTimeout(timer);
  }, [id, duration, onClose]);

  const icons = {
    success: <CheckCircle className="w-5 h-5 text-green-500" />,
    error: <AlertCircle className="w-5 h-5 text-red-500" />,
    warning: <AlertTriangle className="w-5 h-5 text-orange-500" />,
    info: <Info className="w-5 h-5 text-blue-500" />
  };

  const bgColors = {
    success: 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-900',
    error: 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-900',
    warning: 'bg-orange-50 dark:bg-orange-900/20 border-orange-200 dark:border-orange-900',
    info: 'bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-900'
  };

  return (
    <div className={`flex items-start p-4 mb-3 rounded-lg border shadow-sm w-80 transition-all animate-in slide-in-from-right ${bgColors[type]}`}>
      <div className="flex-shrink-0 mr-3 mt-0.5">
        {icons[type]}
      </div>
      <div className="flex-1 mr-2">
        <p className="text-sm font-medium text-gray-800 dark:text-gray-200">{message}</p>
      </div>
      <button 
        onClick={() => onClose(id)}
        className="flex-shrink-0 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
      >
        <X size={16} />
      </button>
    </div>
  );
};

export default Notification;

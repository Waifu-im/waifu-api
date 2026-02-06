type ErrorEvent = {
  message: string;
  type: 'error' | 'warning';
};

type Listener = (event: ErrorEvent) => void;

class EventBus {
  private listeners: Listener[] = [];

  subscribe(listener: Listener) {
    this.listeners.push(listener);
    return () => {
      this.listeners = this.listeners.filter((l) => l !== listener);
    };
  }

  emit(event: ErrorEvent) {
    this.listeners.forEach((listener) => listener(event));
  }
}

export const globalErrorBus = new EventBus();

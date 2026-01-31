import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios';
import { globalErrorBus } from './eventBus';
import { getEnv } from '../utils/env';

// Extend AxiosRequestConfig to include our custom property
declare module 'axios' {
    export interface AxiosRequestConfig {
        skipGlobalErrorHandler?: boolean;
    }
}

const api = axios.create({
    baseURL: getEnv('VITE_API_URL') || '/api',
    headers: {
        'Content-Type': 'application/json',
    },
});

// Add a request interceptor to attach the token
api.interceptors.request.use(
    (config) => {
        const token = localStorage.getItem('token');
        if (token) {
            config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
    },
    (error) => Promise.reject(error)
);

// Enhanced Response Interceptor for Error Handling
api.interceptors.response.use(
    (response) => response,
    (error: AxiosError<any>) => {
        let errorMessage = "An unexpected error occurred";
        let errorType: 'error' | 'warning' = 'error';

        if (error.response) {
            const { data, status } = error.response;

            // 1. Handle 401 Unauthorized globally
            if (status === 401) {
                localStorage.removeItem('token');
                if (!window.location.pathname.includes('/login')) {
                    window.location.href = '/login';
                }
                return Promise.reject(new Error("Session expired. Please login again."));
            }

            // 2. Handle ASP.NET Core Validation ProblemDetails (400)
            if (data?.errors) {
                // validation errors are usually { "Field": ["Error1", "Error2"] }
                const validationErrors = Object.values(data.errors).flat();
                if (validationErrors.length > 0) {
                    errorMessage = String(validationErrors[0]); // Return the first validation error
                    errorType = 'warning'; // Validation errors are often warnings/user errors
                }
            }
            // 3. Handle generic "detail" from ProblemDetails
            else if (data?.detail) {
                errorMessage = data.detail;
            }
            // 4. Handle generic "title" from ProblemDetails
            else if (data?.title) {
                errorMessage = data.title;
            }
            // 5. Handle generic "message" field
            else if (data?.message) {
                errorMessage = data.message;
            }
            // 6. Fallback to status text
            else if (error.response.statusText) {
                errorMessage = `${status}: ${error.response.statusText}`;
            }
        } else if (error.request) {
            errorMessage = "Server unreachable. Please check your internet connection.";
        }

        // Attach the formatted message to the error object so components can use e.message directly
        error.message = errorMessage;

        // Check if the request explicitly asked to skip global error handling
        // We need to cast config because AxiosError.config is InternalAxiosRequestConfig which doesn't have our custom prop by default in types
        const config = error.config as InternalAxiosRequestConfig & { skipGlobalErrorHandler?: boolean };
        
        if (!config?.skipGlobalErrorHandler) {
            globalErrorBus.emit({ message: errorMessage, type: errorType });
        }

        return Promise.reject(error);
    }
);

export default api;

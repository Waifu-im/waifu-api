import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios';
import { globalErrorBus } from './eventBus';
import { getEnv } from '../utils/env';
import { ProblemDetails } from '../types';

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
    paramsSerializer: {
        indexes: null, // Use repeated keys (key=1&key=2) instead of brackets (key[]=1&key[]=2)
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

// Extract error info from an AxiosError into a message and type for toast display
export function formatApiError(error: AxiosError<ProblemDetails>): { message: string; type: 'error' | 'warning' } {
    let message = "An unexpected error occurred";
    let type: 'error' | 'warning' = 'error';

    if (error.response) {
        const { data, status } = error.response;

        if (data?.errors) {
            const validationErrors = Object.values(data.errors).flat();
            if (validationErrors.length > 0) {
                message = String(validationErrors[0]);
                type = 'warning';
            }
        } else if (data?.detail) {
            message = data.detail;
        } else if (data?.title) {
            message = data.title;
        } else if (data?.message) {
            message = data.message;
        } else if (error.response.statusText) {
            message = `${status}: ${error.response.statusText}`;
        }
    } else if (error.request) {
        message = "Server unreachable. Please check your internet connection.";
    }

    return { message, type };
}

// Show a toast notification for an API error
export function showApiError(error: AxiosError<ProblemDetails>) {
    const { message, type } = formatApiError(error);
    globalErrorBus.emit({ message, type });
}

// Enhanced Response Interceptor for Error Handling
api.interceptors.response.use(
    (response) => response,
    (error: AxiosError<ProblemDetails>) => {
        if (error.response) {
            const { status } = error.response;

            // Handle 401 Unauthorized globally
            if (status === 401) {
                localStorage.removeItem('token');
                return Promise.reject(new Error("Session expired. Please login again."));
            }
        }

        const { message } = formatApiError(error);
        error.message = message;

        const config = error.config as InternalAxiosRequestConfig & { skipGlobalErrorHandler?: boolean };

        if (!config?.skipGlobalErrorHandler) {
            showApiError(error);
        }

        return Promise.reject(error);
    }
);

export default api;

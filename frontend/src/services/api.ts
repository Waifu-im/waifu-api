import axios, { AxiosError } from 'axios';

const api = axios.create({
    baseURL: import.meta.env.VITE_API_URL || '/api',
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
                }
            }
            // 3. Handle generic "title" or "detail" from ProblemDetails
            else if (data?.title || data?.detail) {
                errorMessage = data.detail || data.title;
            }
            // 4. Handle generic "message" field
            else if (data?.message) {
                errorMessage = data.message;
            }
            // 5. Fallback to status text
            else if (error.response.statusText) {
                errorMessage = `${status}: ${error.response.statusText}`;
            }
        } else if (error.request) {
            errorMessage = "Server unreachable. Please check your internet connection.";
        }

        // Attach the formatted message to the error object so components can use e.message directly
        error.message = errorMessage;
        return Promise.reject(error);
    }
);

export default api;
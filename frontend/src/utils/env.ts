export const getEnv = (key: keyof Window['env']) => {
    if (window.env && window.env[key]) {
        return window.env[key];
    }
    return import.meta.env[key];
};

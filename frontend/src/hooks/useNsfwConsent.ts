import { useState, useEffect, useCallback } from 'react';

const NSFW_CONSENT_KEY = 'nsfw-consent';

export const useNsfwConsent = (shouldCheck: boolean = true) => {
    const [showWarning, setShowWarning] = useState(false);

    useEffect(() => {
        if (shouldCheck && !localStorage.getItem(NSFW_CONSENT_KEY)) {
            setShowWarning(true);
        }
    }, [shouldCheck]);

    const grantConsent = useCallback(() => {
        localStorage.setItem(NSFW_CONSENT_KEY, 'true');
        setShowWarning(false);
    }, []);

    const dismissWarning = useCallback(() => {
        setShowWarning(false);
    }, []);

    const hasConsent = useCallback(() => {
        return !!localStorage.getItem(NSFW_CONSENT_KEY);
    }, []);

    return {
        showWarning,
        setShowWarning,
        grantConsent,
        dismissWarning,
        hasConsent
    };
};

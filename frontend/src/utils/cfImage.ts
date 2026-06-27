// On-the-fly image resizing for thumbnails (e.g. Cloudflare Image Resizing). See .env.example for vars.

import { getEnv } from './env';

const DEFAULT_CARD_WIDTH = 640;
const DEFAULT_PREVIEW_WIDTH = 1280;
const DEFAULT_WIDTH_PARAM = 'width';

const isLocalHost = (host: string) => host === 'localhost' || host === '127.0.0.1' || host.endsWith('.local');

const transformEnabled = () => getEnv('VITE_IMG_TRANSFORM_ENABLED') === 'true';
const transformPath = () => getEnv('VITE_IMG_TRANSFORM_PATH') || '';
const widthParam = () => getEnv('VITE_IMG_WIDTH_PARAM') || DEFAULT_WIDTH_PARAM;
const cardWidth = () => Number(getEnv('VITE_IMG_CARD_WIDTH')) || DEFAULT_CARD_WIDTH;
const previewWidth = () => Number(getEnv('VITE_IMG_PREVIEW_WIDTH')) || DEFAULT_PREVIEW_WIDTH;

function resize(url: string, width: number): string {
    if (!transformEnabled()) return url;
    const path = transformPath();
    if (!path) return url;
    let u: URL;
    try {
        u = new URL(url);
    } catch {
        return url;
    }
    if (isLocalHost(u.hostname)) return url;
    // Insert the separator between the path and `width=` so a trailing comma/slash is optional.
    const last = path.slice(-1);
    const sep = last === ',' || last === '/' ? '' : path.includes('=') ? ',' : '/';
    return `${u.origin}${path}${sep}${widthParam()}=${width}${u.pathname}`;
}

/** Grid/card thumbnails: gallery, review banners, report cards. */
export const imgCard = (url: string) => resize(url, cardWidth());

/** Larger single-image displays that aren't the detail page: home hero, edit-modal reference. */
export const imgPreview = (url: string) => resize(url, previewWidth());

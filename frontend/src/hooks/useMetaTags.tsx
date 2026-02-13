import { Helmet } from 'react-helmet-async';
import { getEnv } from '../utils/env';

interface MetaTagsProps {
  title?: string;
  description?: string;
  image?: string;
  imageWidth?: number;
  imageHeight?: number;
  url?: string;
  type?: string;
  isNsfw?: boolean;
}

const defaultDescription = 'The API for your Waifu content. Access thousands of categorized anime illustrations via our robust REST API or browse freely.';

export function MetaTags({
  title,
  description,
  image,
  imageWidth,
  imageHeight,
  url,
  type = 'website',
  isNsfw = false,
}: MetaTagsProps) {
  const appTitle = getEnv('VITE_APP_TITLE') || 'Waifu.im';
  const fullTitle = title ? `${title} - ${appTitle}` : appTitle;
  const desc = description || defaultDescription;
  const canonicalUrl = url || window.location.href;
  const ogImage = image || '/android-chrome-512x512.png';
  const twitterCard = image ? 'summary_large_image' : 'summary';

  return (
    <Helmet>
      <title>{fullTitle}</title>
      <meta name="description" content={desc} />

      {/* OpenGraph */}
      <meta property="og:title" content={fullTitle} />
      <meta property="og:description" content={desc} />
      <meta property="og:image" content={ogImage} />
      {imageWidth && <meta property="og:image:width" content={String(imageWidth)} />}
      {imageHeight && <meta property="og:image:height" content={String(imageHeight)} />}
      <meta property="og:type" content={type} />
      <meta property="og:url" content={canonicalUrl} />

      {/* Twitter */}
      <meta name="twitter:card" content={twitterCard} />
      <meta name="twitter:title" content={fullTitle} />
      <meta name="twitter:description" content={desc} />
      <meta name="twitter:image" content={ogImage} />

      {/* NSFW rating */}
      {isNsfw && <meta name="rating" content="adult" />}
    </Helmet>
  );
}

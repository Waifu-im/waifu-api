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
  noindex?: boolean;
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
  noindex = false,
}: MetaTagsProps) {
  const appTitle = getEnv('VITE_APP_TITLE') || 'Waifu API';
  const fullTitle = title ? `${title} - ${appTitle}` : appTitle;
  const desc = description || defaultDescription;
  const origin = window.location.origin;
  // Query params and the hash are per-visit state, not a separate page: never
  // let a filtered gallery define its own canonical URL.
  const canonicalUrl = url || `${origin}${window.location.pathname}`;
  const siteIcon = `${origin}/android-chrome-512x512.png`;
  const ogImage = (image && !isNsfw) ? image : siteIcon;
  const twitterCard = (image && !isNsfw) ? 'summary_large_image' : 'summary';

  return (
    <Helmet>
      <title>{fullTitle}</title>
      <meta name="description" content={desc} />
      <link rel="canonical" href={canonicalUrl} />

      {/* OpenGraph */}
      <meta property="og:site_name" content={appTitle} />
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

      {noindex && <meta name="robots" content="noindex, nofollow" />}
      {isNsfw && <meta name="rating" content="adult" />}
    </Helmet>
  );
}

/// <reference types="vite/client" />

interface Window {
  env: {
    VITE_API_URL: string;
    VITE_DISCORD_CLIENT_ID: string;
    VITE_DISCORD_REDIRECT_URI: string;
    VITE_APP_TITLE?: string;
    VITE_DOCS_URL?: string;
    VITE_CONTACT_EMAIL?: string;
    VITE_DISCORD_SERVER_URL?: string;
    VITE_STATUS_URL?: string;
    VITE_GITHUB_URL?: string;
    VITE_TOS_URL?: string;
    VITE_IMG_TRANSFORM_ENABLED?: string;
    VITE_IMG_TRANSFORM_PATH?: string;
    VITE_IMG_WIDTH_PARAM?: string;
    VITE_IMG_CARD_WIDTH?: string;
    VITE_IMG_PREVIEW_WIDTH?: string;
  }
}

interface ImportMetaEnv {
  readonly VITE_API_URL: string
  readonly VITE_DISCORD_CLIENT_ID: string
  readonly VITE_DISCORD_REDIRECT_URI: string
  readonly VITE_APP_TITLE?: string
  readonly VITE_DOCS_URL?: string
  readonly VITE_CONTACT_EMAIL?: string
  readonly VITE_DISCORD_SERVER_URL?: string
  readonly VITE_STATUS_URL?: string
  readonly VITE_GITHUB_URL?: string
  readonly VITE_TOS_URL?: string
  readonly VITE_IMG_TRANSFORM_ENABLED?: string
  readonly VITE_IMG_TRANSFORM_PATH?: string
  readonly VITE_IMG_WIDTH_PARAM?: string
  readonly VITE_IMG_CARD_WIDTH?: string
  readonly VITE_IMG_PREVIEW_WIDTH?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

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
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

---
sidebar_position: 3
---

# Authentication

Basic image browsing is available without authentication. User-specific features like favorites, albums, and reports require an API key or a JWT token.

## Authentication Methods

The API supports two authentication methods:

### API Key

Generate an API key from your dashboard and include it in the `X-Api-Key` header:

```bash
curl -H "X-Api-Key: YOUR_API_KEY" https://api.waifu.im/users/me/albums
```

API keys are the recommended method for programmatic access. You can create and manage your keys via the `/auth/api-keys` endpoint once authenticated.

### JWT Token (Discord OAuth2)

For browser-based flows, authenticate via Discord OAuth2:

1. Redirect the user to the Discord OAuth2 authorization URL.
2. The user authorizes the application.
3. Discord redirects back with a `code` parameter.
4. Exchange the code for a JWT token via `POST /auth/discord`.

Include the JWT in the `Authorization` header:

```bash
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" https://api.waifu.im/users/me/albums
```

## Getting an API Key

1. Visit [waifu.im](https://waifu.im) and log in with your Discord account.
2. Navigate to your dashboard.
3. Generate an API key and store it securely -- the full key is only shown once.


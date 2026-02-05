---
sidebar_position: 3
---

# Authentication

While basic image search is available without authentication, user-specific features like favorites and albums require an API key.

## API Key Authentication

Include your API key in the `Authorization` header:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" https://api.waifu.im/search
```

## Getting an API Key

1. Visit [waifu.im](https://waifu.im) and log in with your Discord account.
2. Navigate to your account settings.
3. Generate an API key.

## Authenticated Endpoints

The following features require authentication:

- **Favorites** - Add/remove images from your personal favorites.
- **Albums** - Create and manage image collections.
- **Reports** - Report inappropriate content.

## Rate Limiting

The API implements rate limiting to ensure fair usage. If you exceed the rate limit, you will receive a `429 Too Many Requests` response. Implement exponential backoff in your application to handle rate limits gracefully.

## Next Steps

Browse the [API Reference](/docs/category/api) for detailed endpoint documentation.

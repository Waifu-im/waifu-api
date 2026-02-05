---
sidebar_position: 2
---

# Getting Started

This guide walks you through making your first requests to the Waifu.im API.

## Making Your First Request

The simplest way to use the API is to fetch random images. No authentication is required for basic usage.

### Fetch a Random Image

```bash
curl https://api.waifu.im/search
```

This returns a JSON response with image data:

```json
{
  "images": [
    {
      "signature": "abc123",
      "url": "https://cdn.waifu.im/example.jpg",
      "extension": ".jpg",
      "favorites": 42,
      "dominant_color": "#a1b2c3",
      "width": 1920,
      "height": 1080,
      "tags": [
        {
          "tag_id": 1,
          "name": "waifu",
          "description": "A female anime/manga character",
          "is_nsfw": false
        }
      ]
    }
  ]
}
```

### Filter by Tags

You can filter images by tags using the `included_tags` parameter:

```bash
curl "https://api.waifu.im/search?included_tags=waifu"
```

You can also exclude specific tags:

```bash
curl "https://api.waifu.im/search?excluded_tags=nsfw"
```

### Multiple Images

Request multiple images at once using the `limit` parameter:

```bash
curl "https://api.waifu.im/search?limit=5"
```

## Available Tags

To see all available tags:

```bash
curl https://api.waifu.im/tags
```

## Using the API with JavaScript

```javascript
const response = await fetch("https://api.waifu.im/search?included_tags=waifu");
const data = await response.json();
console.log(data.images[0].url);
```

## Using the API with Python

```python
import requests

response = requests.get("https://api.waifu.im/search", params={"included_tags": "waifu"})
data = response.json()
print(data["images"][0]["url"])
```

## Next Steps

- Set up [Authentication](./authentication.md) to access user-specific features like favorites and albums.
- Explore the full [API Reference](/docs/category/api) for all available endpoints and parameters.

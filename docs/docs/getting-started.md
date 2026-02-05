---
sidebar_position: 2
---

# Getting Started

This guide walks you through making your first requests to the Waifu.im API.

## Making Your First Request

The simplest way to use the API is to fetch random images. No authentication is required for basic usage.

### Fetch a Random Image

```bash
curl https://api.waifu.im/images
```

This returns a JSON response with paginated image data:

```json
{
  "items": [
    {
      "id": 8008,
      "signature": "abc123def456",
      "url": "https://cdn.waifu.im/example.jpg",
      "extension": ".jpg",
      "favorites": 42,
      "dominantColor": "#a1b2c3",
      "width": 1920,
      "height": 1080,
      "tags": [
        {
          "id": 1,
          "name": "waifu",
          "description": "A female anime/manga character"
        }
      ]
    }
  ],
  "pageNumber": 1,
  "totalPages": 1,
  "totalCount": 1,
  "hasPreviousPage": false,
  "hasNextPage": false
}
```

### Filter by Tags

Include images matching specific tags using `IncludedTags` (AND logic -- all tags must match):

```bash
curl "https://api.waifu.im/images?IncludedTags=waifu"
```

Exclude images with certain tags using `ExcludedTags` (OR logic -- any match is excluded):

```bash
curl "https://api.waifu.im/images?ExcludedTags=maid"
```

### Pagination

Control pagination with `Page` and `PageSize`:

```bash
curl "https://api.waifu.im/images?PageSize=10&Page=1"
```

### NSFW Content

By default, only SFW images are returned (`IsNsfw=false`). To include NSFW content:

```bash
curl "https://api.waifu.im/images?IsNsfw=true"
```

See the [Tags](./tags.md) page for more details on how tags and NSFW filtering interact.

## Available Tags

To see all available tags:

```bash
curl https://api.waifu.im/tags
```

## Code Examples

### JavaScript

```javascript
const response = await fetch("https://api.waifu.im/images?IncludedTags=waifu");
const data = await response.json();
console.log(data.items[0].url);
```

### Python

```python
import requests

response = requests.get(
    "https://api.waifu.im/images",
    params={"IncludedTags": "waifu"}
)
data = response.json()
print(data["items"][0]["url"])
```

## Next Steps

- Set up [Authentication](./authentication.md) to access features like favorites and albums.
- Read about [Tags](./tags.md) to understand the tagging system.
- Explore the full [API Reference](/docs/category/api) for all endpoints and parameters.

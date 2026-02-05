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
      "perceptualHash": "abc123def456",
      "extension": ".jpg",
      "dominantColor": "#a1b2c3",
      "source": "https://example.com/original",
      "artists": [
        {
          "id": 1,
          "name": "artist_name",
          "patreon": null,
          "pixiv": "https://pixiv.net/users/12345",
          "twitter": null,
          "deviantArt": null
        }
      ],
      "uploaderId": null,
      "uploadedAt": "2024-06-01T12:00:00Z",
      "isNsfw": false,
      "isAnimated": false,
      "width": 1920,
      "height": 1080,
      "byteSize": 512000,
      "url": "https://cdn.waifu.im/example.jpg",
      "tags": [
        {
          "id": 1,
          "name": "waifu",
          "slug": "waifu",
          "description": "A female anime/manga character",
          "imageCount": 1500
        }
      ],
      "favorites": 42,
      "likedAt": null,
      "addedToAlbumAt": null,
      "albums": []
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

By default, only SFW images are returned (`IsNsfw=false`). The parameter accepts three values: `false`, `true`, and `all`.

```bash
# NSFW images only
curl "https://api.waifu.im/images?IsNsfw=true"

# Both SFW and NSFW images
curl "https://api.waifu.im/images?IsNsfw=all"
```

See the [Tags](./tags.md) page for more details on how tags and NSFW filtering interact.

### Filter by Artist

You can filter images by artist ID using `IncludedArtists`:

```bash
curl "https://api.waifu.im/images?IncludedArtists=123"
```

Consult the [API Reference](/docs/category/api) for the full list of available filters (orientation, resolution, file size, animation, and more).

## Available Tags

To see all available tags:

```bash
curl https://api.waifu.im/tags
```

## The `me` and `favorites` Aliases

When authenticated, you can use `me` as a user ID alias to refer to yourself, and `favorites` as an album ID alias to refer to your default favorites album. These aliases simplify common operations:

```bash
# Get your own profile
curl -H "X-Api-Key: YOUR_API_KEY" https://api.waifu.im/users/me

# List your favorites
curl -H "X-Api-Key: YOUR_API_KEY" https://api.waifu.im/users/me/albums/favorites

# Add an image to your favorites
curl -X POST -H "X-Api-Key: YOUR_API_KEY" \
  "https://api.waifu.im/users/me/albums/favorites?ImageId=8008"
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


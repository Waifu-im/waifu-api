---
sidebar_position: 1
slug: /intro
---

# Introduction

Waifu.im is a versatile REST API that provides access to a curated archive of over 4000 anime-style images with powerful filtering capabilities.

## Features

- **Extensive Archive** - Over 4000 high-quality images.
- **Tag-Based Search** - Filter by specific character tags, styles, or themes.
- **Albums** - Organize and share collections of images.
- **Advanced Filtering** - Filter by orientation, resolution, file size, NSFW content, GIFs, artists, and more.
- **User Accounts** - Discord-based authentication to manage favorites and albums.
- **Statistics** - Track API usage and popular tags.
- **Flexible Sorting** - Sort by date, popularity, or get random results.

## Base URL

```
https://api.waifu.im
```

## Quick Example

Fetch a random SFW image:

```bash
curl https://api.waifu.im/search
```

Fetch images with a specific tag:

```bash
curl "https://api.waifu.im/search?included_tags=waifu"
```

## Next Steps

- Check out the [Getting Started](./getting-started.md) guide to set up your first integration.
- Learn about [Authentication](./authentication.md) to access user-specific features.
- Browse the [API Reference](/docs/category/api) for a full list of endpoints.

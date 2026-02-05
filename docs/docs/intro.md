---
sidebar_position: 1
slug: /intro
---

# Introduction

Waifu.im is a REST API that provides access to a curated archive of over 4000 anime-style images with powerful filtering capabilities.

## Features

- **Extensive Archive** -- Over 4000 high-quality images, continuously growing.
- **Tag-Based Search** -- Filter images by tags such as character types, styles, or themes.
- **Albums** -- Organize and share collections of images.
- **Advanced Filtering** -- Filter by resolution, file size, NSFW content, animated images, artists, and more.
- **User Accounts** -- Discord-based authentication to manage favorites and albums.
- **Statistics** -- Track popular tags and API usage.
- **Flexible Sorting** -- Sort results by date, popularity, or get random results.

## Base URL

```
https://api.waifu.im
```

## Quick Example

Fetch a random SFW image:

```bash
curl https://api.waifu.im/images
```

Fetch images with a specific tag:

```bash
curl "https://api.waifu.im/images?IncludedTags=waifu"
```


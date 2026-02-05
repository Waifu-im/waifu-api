---
sidebar_position: 6
---

# Tags

Tags are used to categorize images in the Waifu.im archive. Each image can have one or more tags describing its content.

## How Tags Work

A tag is simply a label attached to images. There is no distinction between "versatile" and "NSFW" tag categories -- every tag works the same way.

However, some tags only categorize explicit (NSFW) images. If you search for one of these tags without adjusting the `IsNsfw` parameter, you will get no results because `IsNsfw` defaults to `false` (SFW only).

### Example

If the tag `hentai` only has explicit images associated with it:

```bash
# Returns no results because IsNsfw defaults to false
curl "https://api.waifu.im/images?IncludedTags=hentai"

# Returns results
curl "https://api.waifu.im/images?IncludedTags=hentai&IsNsfw=true"
```

Tags like `waifu` have both SFW and NSFW images, so they return results regardless:

```bash
# Returns SFW waifu images (default)
curl "https://api.waifu.im/images?IncludedTags=waifu"

# Returns NSFW waifu images
curl "https://api.waifu.im/images?IncludedTags=waifu&IsNsfw=true"
```

## The `IsNsfw` Parameter

| Value   | Behavior                       |
| ------- | ------------------------------ |
| `false` | SFW images only **(default)**  |
| `true`  | NSFW images only               |
| `null`  | Both SFW and NSFW images       |

## Listing Available Tags

Fetch all available tags from the API:

```bash
curl https://api.waifu.im/tags
```

## Filtering with Tags

### Include Tags (AND logic)

Only return images that have **all** of the specified tags:

```bash
curl "https://api.waifu.im/images?IncludedTags=waifu&IncludedTags=blonde-hair"
```

### Exclude Tags (OR logic)

Exclude images that have **any** of the specified tags:

```bash
curl "https://api.waifu.im/images?ExcludedTags=maid&ExcludedTags=school-uniform"
```

### Combining Include and Exclude

```bash
curl "https://api.waifu.im/images?IncludedTags=waifu&ExcludedTags=maid"
```

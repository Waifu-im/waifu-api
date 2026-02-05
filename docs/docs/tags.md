---
sidebar_position: 6
---

# Tags

Tags are used to categorize images in the Waifu.im archive. Each image can have one or more tags describing its content.

## How Tags Work

A tag is simply a label attached to images. Every tag works the same way, but some tags may only have NSFW images associated with them. If you search for such a tag without adjusting the `IsNsfw` parameter, you will get no results because `IsNsfw` defaults to `False` (SFW only).

### Example

If the tag `hentai` only has NSFW images:

```bash
# Returns no results because IsNsfw defaults to False
curl "https://api.waifu.im/images?IncludedTags=hentai"

# Returns results
curl "https://api.waifu.im/images?IncludedTags=hentai&IsNsfw=True"
```

Tags like `waifu` have both SFW and NSFW images. Using `IsNsfw=All` lets you get both at once:

```bash
# Returns SFW waifu images only (default)
curl "https://api.waifu.im/images?IncludedTags=waifu"

# Returns NSFW waifu images only
curl "https://api.waifu.im/images?IncludedTags=waifu&IsNsfw=True"

# Returns both SFW and NSFW waifu images
curl "https://api.waifu.im/images?IncludedTags=waifu&IsNsfw=All"
```

## The `IsNsfw` Parameter

| Value   | Behavior                       |
| ------- | ------------------------------ |
| `False` | SFW images only **(default)**  |
| `True`  | NSFW images only               |
| `All`   | Both SFW and NSFW images       |

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

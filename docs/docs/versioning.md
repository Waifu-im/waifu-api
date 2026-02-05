---
sidebar_position: 5
---

# Versioning

The API supports versioning through the `Accept-Version` header. Pinning your requests to a specific version protects your integration from breaking changes.

## Current Version

The latest API version is **v1**.

## Specifying a Version

Include the `Accept-Version` header in your requests:

```bash
curl -H "Accept-Version: v1" https://api.waifu.im/images
```

### Python

```python
import requests

response = requests.get(
    "https://api.waifu.im/images",
    headers={"Accept-Version": "v1"}
)
```

### JavaScript

```javascript
const response = await fetch("https://api.waifu.im/images", {
  headers: { "Accept-Version": "v1" },
});
```

## Default Behavior

If you do not specify a version, the API uses the latest available version. This can break your application if a new version introduces breaking changes.

**Best practice**: always include the `Accept-Version` header in production code.

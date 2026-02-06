---
sidebar_position: 4
sidebar_label: Rate Limiting
---

# Rate Limiting

The API enforces rate limits to ensure fair usage for all consumers.

## Limits

- **Rate**: 20 requests per second (1 request every 50 milliseconds).
- **Burst**: Up to 30 excess requests are accepted beyond the base rate. The first 15 are served immediately; the remaining 15 are queued and processed at the base rate (one every 50 ms).
- **Rejection**: When the burst buffer is full, the API responds with HTTP `429 Too Many Requests`.
- **`Retry-After` header**: Included in every `429` response. It indicates how many seconds to wait before retrying.

## Handling Rate Limits

When you receive a `429` response, check the `Retry-After` header. It indicates how many seconds to wait before retrying.

### Example: Exponential Backoff

```python
import time
import requests

def fetch_with_backoff(url, max_retries=5):
    for attempt in range(max_retries):
        response = requests.get(url)
        if response.status_code != 429:
            return response
        wait = float(response.headers.get("Retry-After", 2 ** attempt))
        time.sleep(wait)
    return response
```

```javascript
async function fetchWithBackoff(url, maxRetries = 5) {
  for (let attempt = 0; attempt < maxRetries; attempt++) {
    const response = await fetch(url);
    if (response.status !== 429) return response;
    const wait = response.headers.get("Retry-After") ?? 2 ** attempt;
    await new Promise((r) => setTimeout(r, Number(wait) * 1000));
  }
}
```

## Abuse Policy

Excessive and abusive use of the API can result in being blacklisted. Reaching that threshold requires persistent and excessive violations -- normal usage and occasional traffic spikes will not cause issues.

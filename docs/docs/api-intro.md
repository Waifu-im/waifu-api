---
id: api-intro
sidebar_position: 0
slug: /api
---

# API Reference

This section contains the full API reference for Waifu.im, auto-generated from the [OpenAPI specification](https://api.waifu.im/openapi/v1.json).

## Base URL

```
https://api.waifu.im
```

## Authentication

The API supports two authentication methods:

- **JWT Token** -- Obtained via Discord OAuth2 (`POST /auth/discord`). Passed as `Authorization: Bearer <token>`.
- **API Key** -- Passed in the `X-Api-Key` header.

Some endpoints require authentication. Check the endpoint details below or the [Authentication](./authentication.md) page for more information.

## Response Format

All responses are JSON. Paginated endpoints return:

```json
{
  "items": [],
  "pageNumber": 1,
  "totalPages": 10,
  "totalCount": 100,
  "hasPreviousPage": false,
  "hasNextPage": true
}
```

## Error Handling

Errors follow the [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) Problem Details format:

```json
{
  "type": "https://tools.ietf.org/html/rfc9110#section-15.5.1",
  "title": "Bad Request",
  "status": 400,
  "detail": "One or more validation errors occurred.",
  "errors": {
    "fieldName": ["Error message"]
  }
}
```

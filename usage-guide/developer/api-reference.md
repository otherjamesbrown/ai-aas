# API Reference

## Overview

Complete API reference documentation for the AIaaS platform.

## Base Information

### Base URL

- Production: `https://api.example.com/v1`
- API version: v1
- Content-Type: `application/json`

### Authentication

All requests require API key authentication:

```
Authorization: Bearer <your-api-key>
```

## Endpoints

### Inference Endpoint

**POST** `/inference`

Make inference requests to AI models.

**Request Body:**
```json
{
  "model": "gpt-4",
  "prompt": "Your prompt here",
  "max_tokens": 100,
  "temperature": 0.7
}
```

**Response:**
```json
{
  "id": "req_123",
  "model": "gpt-4",
  "choices": [...],
  "usage": {...}
}
```

### Usage Endpoint

**GET** `/usage`

Retrieve usage statistics.

**Query Parameters:**
- `start_date`: Start date (ISO 8601)
- `end_date`: End date (ISO 8601)
- `model`: Filter by model

**Response:**
```json
{
  "usage": [...],
  "total_tokens": 1000,
  "total_cost": 0.05
}
```

## Response Codes

- **200 OK**: Request successful
- **400 Bad Request**: Invalid request
- **401 Unauthorized**: Invalid API key
- **403 Forbidden**: Insufficient permissions
- **429 Too Many Requests**: Rate limit exceeded
- **500 Internal Server Error**: Server error

## Rate Limits

- Default: 100 requests per minute
- Burst: 200 requests per minute
- Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`

## Error Handling

See [Error Handling](./error-handling.md) for detailed error handling guide.

## Related Documentation

- [Making API Requests](./making-api-requests.md)
- [Error Handling](./error-handling.md)
- [SDK Usage](./sdk-usage.md)


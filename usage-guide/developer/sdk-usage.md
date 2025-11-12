# SDK Usage

## Overview

This guide covers using platform SDKs to integrate with the AIaaS platform.

## Available SDKs

### Python SDK

```python
from ai_aas import Client

client = Client(api_key="your-api-key")

response = client.inference.create(
    model="gpt-4",
    prompt="Hello, world!",
    max_tokens=100
)
```

### JavaScript/TypeScript SDK

```javascript
import { Client } from '@ai-aas/sdk';

const client = new Client({ apiKey: 'your-api-key' });

const response = await client.inference.create({
  model: 'gpt-4',
  prompt: 'Hello, world!',
  maxTokens: 100
});
```

### Go SDK

```go
import "github.com/ai-aas/go-sdk"

client := sdk.NewClient("your-api-key")

response, err := client.Inference.Create(context.Background(), &sdk.InferenceRequest{
    Model: "gpt-4",
    Prompt: "Hello, world!",
    MaxTokens: 100,
})
```

## SDK Installation

### Python

```bash
pip install ai-aas
```

### JavaScript/TypeScript

```bash
npm install @ai-aas/sdk
```

### Go

```bash
go get github.com/ai-aas/go-sdk
```

## SDK Features

### Authentication

- Automatic API key handling
- Secure credential storage
- Token refresh support

### Error Handling

- Typed error responses
- Retry logic
- Error recovery

### Utilities

- Request/response logging
- Metrics collection
- Connection pooling

## Best Practices

### SDK Usage

- Use latest SDK version
- Handle errors properly
- Implement retry logic
- Monitor SDK usage

### Performance

- Reuse client instances
- Use connection pooling
- Implement caching
- Optimize requests

## Related Documentation

- [Making API Requests](./making-api-requests.md)
- [Error Handling](./error-handling.md)
- [Best Practices](./best-practices.md)


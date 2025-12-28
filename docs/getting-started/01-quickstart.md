# Quickstart: Your First API Call

Make your first AI inference request in under 5 minutes.

## Prerequisites

- An API key (get one from your organization admin or see [CLI Setup](./02-cli-setup.md))
- `curl` or any HTTP client

## Step 1: Set Your API Key

```bash
export AI_AAS_API_KEY="your-api-key-here"
```

## Step 2: Make a Request

```bash
curl -X POST https://api.dev.otherjamesbrown.com/v1/chat/completions \
  -H "X-API-Key: $AI_AAS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ],
    "max_tokens": 50
  }'
```

## Step 3: See the Response

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1732147200,
  "model": "gpt-oss-20b",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "The capital of France is Paris."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 14,
    "completion_tokens": 8,
    "total_tokens": 22
  }
}
```

## That's It!

You've made your first AI inference request. The response includes:

| Field | Description |
|-------|-------------|
| `choices[0].message.content` | The AI's response |
| `usage.prompt_tokens` | Tokens in your input |
| `usage.completion_tokens` | Tokens generated |
| `usage.total_tokens` | Total tokens (for billing) |

## Next Steps

- [Available Models](./03-available-models.md) - See what models you can use
- [Making Requests](./04-making-requests.md) - More examples with Python and Node.js
- [Usage & Limits](./05-usage-and-limits.md) - Understand rate limits and track usage

## Using with OpenAI SDK

The API is OpenAI-compatible, so you can use the official OpenAI SDK:

### Python

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-api-key-here",
    base_url="https://api.dev.otherjamesbrown.com/v1"
)

response = client.chat.completions.create(
    model="gpt-oss-20b",
    messages=[{"role": "user", "content": "Hello!"}]
)

print(response.choices[0].message.content)
```

### Node.js

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
    apiKey: 'your-api-key-here',
    baseURL: 'https://api.dev.otherjamesbrown.com/v1'
});

const response = await client.chat.completions.create({
    model: 'gpt-oss-20b',
    messages: [{ role: 'user', content: 'Hello!' }]
});

console.log(response.choices[0].message.content);
```

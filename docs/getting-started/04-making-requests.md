# Making API Requests

Complete examples for calling the AI-AAS API in different languages.

## API Endpoint

| Environment | Base URL |
|-------------|----------|
| Development | `https://api.dev.otherjamesbrown.com` |

## Authentication

All requests require the `X-API-Key` header:

```
X-API-Key: your-api-key-here
```

## Chat Completions

The primary endpoint for conversational AI.

**Endpoint**: `POST /v1/chat/completions`

### curl

```bash
curl -X POST https://api.dev.otherjamesbrown.com/v1/chat/completions \
  -H "X-API-Key: $AI_AAS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Write a haiku about programming."}
    ],
    "max_tokens": 100,
    "temperature": 0.7
  }'
```

### Python (OpenAI SDK)

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-api-key-here",
    base_url="https://api.dev.otherjamesbrown.com/v1"
)

response = client.chat.completions.create(
    model="gpt-oss-20b",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Write a haiku about programming."}
    ],
    max_tokens=100,
    temperature=0.7
)

print(response.choices[0].message.content)
print(f"Tokens used: {response.usage.total_tokens}")
```

### Python (requests)

```python
import requests

response = requests.post(
    "https://api.dev.otherjamesbrown.com/v1/chat/completions",
    headers={
        "X-API-Key": "your-api-key-here",
        "Content-Type": "application/json"
    },
    json={
        "model": "gpt-oss-20b",
        "messages": [
            {"role": "user", "content": "Hello!"}
        ],
        "max_tokens": 50
    }
)

data = response.json()
print(data["choices"][0]["message"]["content"])
```

### Node.js (OpenAI SDK)

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
    apiKey: 'your-api-key-here',
    baseURL: 'https://api.dev.otherjamesbrown.com/v1'
});

async function main() {
    const response = await client.chat.completions.create({
        model: 'gpt-oss-20b',
        messages: [
            { role: 'system', content: 'You are a helpful assistant.' },
            { role: 'user', content: 'Write a haiku about programming.' }
        ],
        max_tokens: 100,
        temperature: 0.7
    });

    console.log(response.choices[0].message.content);
    console.log(`Tokens used: ${response.usage.total_tokens}`);
}

main();
```

### Node.js (fetch)

```javascript
const response = await fetch('https://api.dev.otherjamesbrown.com/v1/chat/completions', {
    method: 'POST',
    headers: {
        'X-API-Key': 'your-api-key-here',
        'Content-Type': 'application/json'
    },
    body: JSON.stringify({
        model: 'gpt-oss-20b',
        messages: [
            { role: 'user', content: 'Hello!' }
        ],
        max_tokens: 50
    })
});

const data = await response.json();
console.log(data.choices[0].message.content);
```

## Message Roles

| Role | Purpose | Example |
|------|---------|---------|
| `system` | Set behavior/persona | "You are a helpful coding assistant." |
| `user` | User's input | "How do I sort a list in Python?" |
| `assistant` | Previous AI responses | Used for conversation history |

### Multi-Turn Conversation

```python
messages = [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What is Python?"},
    {"role": "assistant", "content": "Python is a programming language..."},
    {"role": "user", "content": "How do I install it?"}
]

response = client.chat.completions.create(
    model="gpt-oss-20b",
    messages=messages
)
```

## Text Completions (Legacy)

For simple text completion without chat format.

**Endpoint**: `POST /v1/completions`

```bash
curl -X POST https://api.dev.otherjamesbrown.com/v1/completions \
  -H "X-API-Key: $AI_AAS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "prompt": "Once upon a time",
    "max_tokens": 100
  }'
```

## Streaming Responses

For real-time token streaming:

```python
response = client.chat.completions.create(
    model="gpt-oss-20b",
    messages=[{"role": "user", "content": "Tell me a story."}],
    stream=True
)

for chunk in response:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
```

## Request Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `model` | string | required | Model ID |
| `messages` | array | required | Chat messages |
| `max_tokens` | integer | model max | Max tokens to generate |
| `temperature` | float | 1.0 | Randomness (0.0-2.0) |
| `top_p` | float | 1.0 | Nucleus sampling |
| `n` | integer | 1 | Number of completions |
| `stop` | string/array | null | Stop sequences |
| `stream` | boolean | false | Enable streaming |

## Response Format

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
        "content": "The response text..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 50,
    "total_tokens": 75
  }
}
```

## Error Handling

```python
from openai import OpenAI, RateLimitError, APIError

client = OpenAI(
    api_key="your-api-key",
    base_url="https://api.dev.otherjamesbrown.com/v1"
)

try:
    response = client.chat.completions.create(
        model="gpt-oss-20b",
        messages=[{"role": "user", "content": "Hello"}]
    )
except RateLimitError:
    print("Rate limit exceeded. Please wait and retry.")
except APIError as e:
    print(f"API error: {e}")
```

See [Troubleshooting](./06-troubleshooting.md) for common errors and solutions.

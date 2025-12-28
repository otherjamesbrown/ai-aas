# Usage & Rate Limits

Monitor your token consumption and understand rate limits.

## Understanding Tokens

Tokens are the basic units for measuring API usage:

- **1 token** ≈ 4 characters or ¾ of a word
- "Hello, world!" ≈ 4 tokens
- A typical paragraph ≈ 50-100 tokens

Every request consumes tokens for:
- **Prompt tokens**: Your input (messages, system prompt)
- **Completion tokens**: The model's response

## Checking Usage

### Via API Response

Every API response includes token usage:

```json
{
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 50,
    "total_tokens": 75
  }
}
```

### Via CLI

```bash
# Query usage for last 24 hours
ai-aas-cli usage query --org-id your-org --last-24h

# Query usage for last 7 days
ai-aas-cli usage query --org-id your-org --last-7d

# Query specific date range
ai-aas-cli usage query --org-id your-org --from 2025-01-01 --to 2025-01-31

# Filter by model
ai-aas-cli usage query --org-id your-org --last-7d --model gpt-oss-20b

# Get hourly breakdown
ai-aas-cli usage query --org-id your-org --last-24h --granularity hour

# Output as JSON
ai-aas-cli usage query --org-id your-org --last-7d --format json
```

### Usage Summary

```bash
ai-aas-cli usage summary --org-id your-org
```

## Rate Limits

Rate limits protect the platform and ensure fair access for all users.

### Default Limits

| Limit Type | Default |
|------------|---------|
| Requests per second | 100 RPS |
| Burst capacity | 200 requests |

Limits are enforced per API key.

### Rate Limit Headers

Every response includes rate limit information:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1732147260
```

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests per window |
| `X-RateLimit-Remaining` | Requests left in current window |
| `X-RateLimit-Reset` | Unix timestamp when limit resets |

### Handling Rate Limits

When you exceed the rate limit, you'll receive a `429 Too Many Requests` response:

```json
{
  "error": {
    "message": "Rate limit exceeded",
    "type": "rate_limit_error",
    "code": "rate_limit_exceeded"
  }
}
```

**Best practice**: Implement exponential backoff:

```python
import time
from openai import OpenAI, RateLimitError

client = OpenAI(
    api_key="your-api-key",
    base_url="https://api.dev.otherjamesbrown.com/v1"
)

def make_request_with_retry(messages, max_retries=3):
    for attempt in range(max_retries):
        try:
            return client.chat.completions.create(
                model="gpt-oss-20b",
                messages=messages
            )
        except RateLimitError:
            if attempt < max_retries - 1:
                wait_time = 2 ** attempt  # 1, 2, 4 seconds
                print(f"Rate limited. Waiting {wait_time}s...")
                time.sleep(wait_time)
            else:
                raise

# Usage
response = make_request_with_retry([
    {"role": "user", "content": "Hello"}
])
```

## Optimizing Token Usage

### 1. Use Concise Prompts

```python
# Less efficient
messages = [
    {"role": "user", "content": "I would really appreciate it if you could please tell me what the capital city of France is. Thank you very much!"}
]

# More efficient
messages = [
    {"role": "user", "content": "What is the capital of France?"}
]
```

### 2. Set Appropriate max_tokens

```python
# Don't use more than you need
response = client.chat.completions.create(
    model="gpt-oss-20b",
    messages=[{"role": "user", "content": "Yes or no: Is Paris in France?"}],
    max_tokens=5  # Only need a short answer
)
```

### 3. Use Stop Sequences

```python
response = client.chat.completions.create(
    model="gpt-oss-20b",
    messages=[{"role": "user", "content": "List 3 colors:"}],
    stop=["\n4."]  # Stop before generating more
)
```

### 4. Truncate Conversation History

For multi-turn conversations, keep only recent messages:

```python
def truncate_messages(messages, max_messages=10):
    # Always keep system message
    system = [m for m in messages if m["role"] == "system"]
    others = [m for m in messages if m["role"] != "system"]

    # Keep last N messages
    return system + others[-max_messages:]
```

## Usage Reports

Organization admins can generate usage reports:

```bash
# CSV export for billing
ai-aas-cli usage query --org-id your-org --last-30d --format csv > usage.csv

# JSON for programmatic access
ai-aas-cli usage query --org-id your-org --last-30d --format json > usage.json
```

## Requesting Limit Increases

Contact your platform administrator to request:
- Higher rate limits
- Increased token quotas
- Priority access

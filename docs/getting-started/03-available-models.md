# Available Models

This page lists the AI models available on the platform.

## Current Models

| Model ID | Description | Context Length | Best For |
|----------|-------------|----------------|----------|
| `gpt-oss-20b` | Unsloth GPT-OSS 20B parameter model | 4K tokens | General chat, Q&A, content generation |

## Checking Available Models

### Via API

```bash
curl https://api.dev.otherjamesbrown.com/v1/models \
  -H "X-API-Key: $AI_AAS_API_KEY"
```

### Via CLI

```bash
ai-aas-cli model registry list
```

## Model Capabilities

### gpt-oss-20b

The primary model available on the platform. Suitable for:

- **Chat completions** - Conversational AI
- **Text generation** - Content creation
- **Question answering** - Knowledge queries
- **Summarization** - Document summarization
- **Code assistance** - Basic code help

**Limitations**:
- Context window: 4,096 tokens
- No image/multimodal support
- No function calling

## Using Models

### Specify Model in Request

```bash
curl -X POST https://api.dev.otherjamesbrown.com/v1/chat/completions \
  -H "X-API-Key: $AI_AAS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Explain quantum computing simply."}
    ],
    "max_tokens": 200,
    "temperature": 0.7
  }'
```

### Model Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `model` | string | required | Model ID to use |
| `messages` | array | required | Chat messages |
| `max_tokens` | integer | model max | Maximum tokens to generate |
| `temperature` | float | 1.0 | Randomness (0.0-2.0) |
| `top_p` | float | 1.0 | Nucleus sampling |
| `stop` | string/array | null | Stop sequences |
| `stream` | boolean | false | Stream responses |

## Organization Model Access

Organizations may have access to specific models. Check with your administrator or use:

```bash
ai-aas-cli model library list --org-id your-org
```

## Requesting New Models

Contact your platform administrator to request additional models be deployed.

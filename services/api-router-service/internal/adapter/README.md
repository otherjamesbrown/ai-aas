# Protocol Adapters

This directory contains protocol translation adapters for the API Router Service.

## KServe Adapter

The `kserve` package provides bidirectional translation between:
- **OpenAI Chat Completions API** (client-facing)
- **KServe V2 Inference Protocol** (backend-facing)

### Usage

```go
import "github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/kserve"

translator := kserve.NewTranslator()

// Translate OpenAI request to KServe V2
openaiReq := &kserve.OpenAIChatCompletionRequest{
    Model: "llama-2-7b",
    Messages: []kserve.ChatMessage{
        {Role: "user", Content: "Hello!"},
    },
    Temperature: float64Ptr(0.7),
    MaxTokens:   intPtr(50),
}

kserveReq, err := translator.TranslateOpenAIToKServe(openaiReq)
if err != nil {
    // Handle error
}

// Forward kserveReq to KServe backend...

// Translate KServe response back to OpenAI format
kserveResp := &kserve.InferResponse{
    ID: "response-123",
    ModelName: "llama-2-7b",
    Outputs: []kserve.InferOutputTensor{
        {
            Name:     "text",
            Shape:    []int64{1},
            Datatype: "BYTES",
            Data:     []interface{}{"Hello! How can I help you?"},
        },
    },
}

openaiResp, err := translator.TranslateKServeToOpenAI(kserveResp, openaiReq)
if err != nil {
    // Handle error
}

// Return openaiResp to client
```

### Protocol Mapping

#### Request Translation

| OpenAI Field | KServe V2 Field | Notes |
|--------------|-----------------|-------|
| `messages` | `inputs[0].data[0]` | Serialized as prompt string |
| `temperature` | `parameters.temperature` | Direct mapping |
| `max_tokens` | `parameters.max_tokens` | Direct mapping |
| `top_p` | `parameters.top_p` | Direct mapping |
| `stop` | `parameters.stop` | Direct mapping |

#### Response Translation

| KServe V2 Field | OpenAI Field | Notes |
|-----------------|--------------|-------|
| `outputs[0].data[0]` | `choices[0].message.content` | Extracted completion text |
| `id` | `id` | Direct mapping |
| Token counts | `usage.*` | Estimated (rough approximation) |

### Testing

Run unit tests:

```bash
cd services/api-router-service
go test ./internal/adapter/kserve/...
```

Expected output:
```
ok      github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/kserve
```

### Future Improvements

1. **Token Counting**: Use tiktoken or actual model tokenizer for accurate counts
2. **Chat Templates**: Support model-specific chat templates (Llama 2, Mistral, etc.)
3. **Streaming**: Implement streaming support for both protocols
4. **Error Handling**: Enhanced error translation and retry logic
5. **Metadata**: Preserve additional metadata fields

### References

- [KServe V2 Protocol Spec](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md)
- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat)
- [vLLM OpenAI-Compatible Server](https://docs.vllm.ai/en/latest/serving/openai_compatible_server.html)

# RAG Infrastructure Specification

## Overview

Infrastructure support for Retrieval-Augmented Generation (RAG) pipelines, including embedding models, rerankers, and vector database integration. This spec extends the Model Recipes system (spec 025) to support the complete RAG workflow.

## RAG Pipeline Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           RAG Pipeline                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Query ──► [Embedding Model] ──► Vector Search ──► [Reranker] ──► LLM   │
│              (TEI)                (Qdrant)          (TEI)       (vLLM)   │
│                                                                          │
│  Documents ──► [Chunking] ──► [Embedding Model] ──► Vector DB            │
│                (App Layer)        (TEI)             (Qdrant)             │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Components

### 1. Embedding Models (TEI Runtime)

Text Embeddings Inference (TEI) from HuggingFace provides optimized inference for embedding models.

**Why TEI?**
- Purpose-built for embeddings (not adapted from LLM serving)
- Optimized batching and tokenization
- Lower latency than vLLM for embeddings
- Supports both embeddings and rerankers

### 2. Reranker Models (TEI Runtime)

Cross-encoder models that score query-document pairs for improved retrieval quality.

**Why Rerankers?**
- Embedding similarity is approximate; rerankers are more accurate
- Typical improvement: 5-15% on retrieval metrics
- Applied to top-k results (typically 20-100) from initial retrieval

### 3. Vector Database

Storage and similarity search for embeddings. Qdrant is recommended for production use.

**Why Qdrant?**
- Purpose-built vector database (not an extension)
- Excellent performance and scalability
- Rich filtering and payload support
- Active development and good Kubernetes support

## TEI Runtime Support

### Embedding Model Recipe

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: ModelRecipe
metadata:
  name: bge-large-en-v1.5
  namespace: ai-model-system
  labels:
    ai.ai-aas.io/model-family: bge
    ai.ai-aas.io/task: embedding
    ai.ai-aas.io/runtime: tei
spec:
  modelID: BAAI/bge-large-en-v1.5
  displayName: "BGE Large English v1.5"
  description: |
    BAAI's BGE embedding model for English text.
    Produces 1024-dimensional embeddings for semantic search.

  runtime: tei
  image: ghcr.io/huggingface/text-embeddings-inference:1.5

  resources:
    gpu:
      type: nvidia
      count: 1
      minMemoryGB: 4
    cpu:
      requests: "2"
      limits: "4"
    memory:
      requests: "4Gi"
      limits: "8Gi"

  runtimeArgs:
    tei:
      # Model type determines API behavior
      modelType: embedding  # embedding | reranker

      # Batching configuration
      maxBatchTokens: 16384
      maxConcurrentRequests: 512

      # Precision
      dtype: float16

      # Pooling strategy for embeddings
      pooling: cls  # cls | mean | splade

      # Additional arguments
      extraArgs:
        - --auto-truncate

  scheduling:
    tolerations:
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
      - key: gpu-workload
        operator: Equal
        value: "true"
        effect: NoSchedule

  healthCheck:
    startupProbeSeconds: 60
    livenessPath: /health
    readinessPath: /health

  metadata:
    parameters: "335M"
    dimensions: 1024
    maxSequenceLength: 512
    architecture: "BertModel"
    license: "MIT"
    sourceURL: "https://huggingface.co/BAAI/bge-large-en-v1.5"
```

### Reranker Model Recipe

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: ModelRecipe
metadata:
  name: bge-reranker-v2-m3
  namespace: ai-model-system
  labels:
    ai.ai-aas.io/model-family: bge
    ai.ai-aas.io/task: reranking
    ai.ai-aas.io/runtime: tei
spec:
  modelID: BAAI/bge-reranker-v2-m3
  displayName: "BGE Reranker v2 M3"
  description: |
    Cross-encoder reranker for improving retrieval quality.
    Supports 100+ languages.

  runtime: tei
  image: ghcr.io/huggingface/text-embeddings-inference:1.5

  resources:
    gpu:
      type: nvidia
      count: 1
      minMemoryGB: 4
    cpu:
      requests: "2"
      limits: "4"
    memory:
      requests: "4Gi"
      limits: "8Gi"

  runtimeArgs:
    tei:
      # IMPORTANT: modelType must be "reranker" for cross-encoder scoring
      modelType: reranker
      maxBatchTokens: 8192
      maxConcurrentRequests: 128
      dtype: float16
      extraArgs:
        - --auto-truncate

  healthCheck:
    startupProbeSeconds: 60
    livenessPath: /health
    readinessPath: /health

  metadata:
    parameters: "568M"
    maxSequenceLength: 8192
    license: "MIT"
    sourceURL: "https://huggingface.co/BAAI/bge-reranker-v2-m3"
```

## Vector Database: Qdrant

### Deployment Options

#### Option 1: Managed Qdrant Cloud
Best for production - handles scaling, backups, and HA automatically.

#### Option 2: Self-Hosted Qdrant (Kubernetes)

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: VectorDatabase
metadata:
  name: qdrant-production
  namespace: rag-infrastructure
spec:
  type: qdrant

  # Cluster configuration
  replicas: 3  # For HA
  shards: 2    # Data distribution

  # Storage
  storage:
    size: 100Gi
    storageClass: fast-ssd

  # Resources per replica
  resources:
    cpu:
      requests: "2"
      limits: "4"
    memory:
      requests: "8Gi"
      limits: "16Gi"

  # Qdrant-specific config
  config:
    # Write-ahead log for durability
    walCapacityMb: 256
    # HNSW index parameters
    hnswConfig:
      m: 16
      efConstruct: 100
    # Optimizers
    optimizers:
      defaultSegmentNumber: 2
      indexingThreshold: 20000
```

### Qdrant Helm Deployment

For initial deployment, we'll use the official Qdrant Helm chart:

```bash
helm repo add qdrant https://qdrant.github.io/qdrant-helm
helm install qdrant qdrant/qdrant \
  --namespace rag-infrastructure \
  --set replicaCount=1 \
  --set persistence.size=50Gi \
  --set resources.requests.memory=4Gi \
  --set resources.limits.memory=8Gi
```

## Recipe Library Structure

Embedding and reranker recipes are stored in the model-recipes library:

```
infra/model-recipes/
├── embedding/
│   ├── bge-large-en-v1.5.yaml
│   ├── bge-m3.yaml
│   └── e5-mistral-7b-instruct.yaml
├── reranker/
│   ├── bge-reranker-v2-m3.yaml
│   ├── bge-reranker-v2-gemma.yaml
│   └── ms-marco-minilm.yaml
└── ...
```

## API Integration

### Embedding API (OpenAI-compatible)

TEI provides an OpenAI-compatible embeddings endpoint:

```bash
POST /v1/embeddings
{
  "model": "bge-large-en-v1.5",
  "input": ["Hello world", "How are you?"]
}

Response:
{
  "object": "list",
  "data": [
    {"embedding": [0.1, 0.2, ...], "index": 0},
    {"embedding": [0.3, 0.4, ...], "index": 1}
  ],
  "model": "bge-large-en-v1.5",
  "usage": {"prompt_tokens": 8, "total_tokens": 8}
}
```

### Reranking API

```bash
POST /rerank
{
  "query": "What is machine learning?",
  "texts": [
    "Machine learning is a subset of AI...",
    "The weather is nice today...",
    "Deep learning uses neural networks..."
  ]
}

Response:
[
  {"index": 0, "score": 0.95},
  {"index": 2, "score": 0.82},
  {"index": 1, "score": 0.12}
]
```

### API Router Integration

The API Router will need to be extended to route embedding and reranking requests:

```yaml
# api-router-service values.yaml
backends:
  # LLM backends (existing)
  - name: mistral-7b
    type: vllm
    ...

  # Embedding backends (new)
  - name: bge-large-en
    type: tei
    modelType: embedding
    serviceName: bge-large-en-tei
    namespace: rag-infrastructure
    port: 80
    path: /v1/embeddings

  # Reranker backends (new)
  - name: bge-reranker
    type: tei
    modelType: reranker
    serviceName: bge-reranker-tei
    namespace: rag-infrastructure
    port: 80
    path: /rerank
```

## CLI Integration

```bash
# List embedding models
ai-aas-cli model recipe list --task embedding

# Deploy embedding model
ai-aas-cli model deploy create --recipe bge-large-en-v1.5 -e production

# List reranker models
ai-aas-cli model recipe list --task reranking

# Deploy reranker
ai-aas-cli model deploy create --recipe bge-reranker-v2-m3 -e production

# Check RAG infrastructure status
ai-aas-cli rag status

# Test embedding endpoint
ai-aas-cli rag test-embedding "Hello world"

# Test reranking
ai-aas-cli rag test-rerank "query" "doc1" "doc2" "doc3"
```

## Implementation Phases

### Phase 1: TEI Runtime Support
- Add TEI container builder to AI Model Operator
- Create embedding model recipes (BGE-large, BGE-M3)
- Create reranker model recipes (BGE-reranker-v2-m3)
- OpenAI-compatible embedding endpoint in API Router

### Phase 2: Vector Database Integration
- Deploy Qdrant via Helm chart
- Add VectorDatabase CRD (optional, for managed deployments)
- CLI commands for vector DB status

### Phase 3: RAG Pipeline Helpers
- Document chunking utilities (SDK/library)
- Example RAG application
- Monitoring and metrics for RAG components

## Recommended Model Choices

### Embedding Models

| Model | Size | Dimensions | Languages | Best For |
|-------|------|------------|-----------|----------|
| BGE-large-en-v1.5 | 335M | 1024 | English | English semantic search |
| BGE-M3 | 568M | 1024 | 100+ | Multilingual, long docs (8K) |
| E5-mistral-7b | 7B | 4096 | English | Highest quality, resource-heavy |

### Reranker Models

| Model | Size | Speed | Languages | Best For |
|-------|------|-------|-----------|----------|
| MS-MARCO-MiniLM-L6 | 22M | Fast | English | Low latency |
| BGE-reranker-v2-m3 | 568M | Medium | 100+ | Multilingual |
| BGE-reranker-v2-gemma | 2B | Slow | 100+ | Highest accuracy |

## Related Specs

- **[025-model-recipes](../025-model-recipes/spec.md)**: Core recipe system (vLLM, Triton, TGI)

## Open Questions

1. Should vector database be managed via CRD or just Helm/ArgoCD?
2. How to handle embedding model versioning (affects stored vectors)?
3. Should we provide a unified RAG SDK or just infrastructure?
4. Hybrid search (dense + sparse) support?

# Model Recipes Library

This directory contains ModelRecipe YAML files for known-good AI model configurations.

## Directory Structure

- `llm/` - Large Language Models
  - `mistral/` - Mistral AI models
  - `llama/` - Meta Llama models
  - `openai/` - OpenAI-compatible models (e.g., gpt-oss-20b)
- `vision/` - Vision models
  - `florence/` - Microsoft Florence models

## Usage

Recipes are synced to the cluster via ArgoCD. To deploy a model using a recipe:

```bash
ai-aas-cli model deploy <model-name> -e <environment> --recipe <recipe-name>
```

## Adding New Recipes

1. Create a new YAML file following the ModelRecipe CRD schema
2. Place it in the appropriate category directory
3. Commit and push - ArgoCD will sync automatically

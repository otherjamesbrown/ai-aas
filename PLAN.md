# CLI Refactoring Plan: Nested Model Subcommands

## Overview

Refactor the `ai-aas model` CLI commands from a flat structure (27 commands) to a docker/gh-style nested structure with world-class help output and workflow guidance.

## Design Decisions

- **Breaking change**: Yes - old command syntax will not work
- **Structure**: Full docker-style nesting (`model registry add`, `model cache pull`, etc.)
- **Help quality**: Professional formatting with examples, related commands, and next-step suggestions

---

## New Command Structure

```
ai-aas model
├── registry        # Model registry management
│   ├── add         # Register a model from HuggingFace
│   ├── list        # List registered models
│   ├── info        # Show model details
│   ├── remove      # Remove model from registry
│   └── status      # Show model status
├── cache           # Object storage cache management
│   ├── pull        # Download model to cache
│   ├── list        # List cached models
│   ├── info        # Show cache details
│   └── delete      # Remove from cache
├── deploy          # Deployment management
│   ├── create      # Deploy model to Kubernetes
│   ├── delete      # Remove deployment
│   ├── restart     # Rolling restart
│   ├── scale       # Scale replicas
│   └── status      # Show deployment status
├── troubleshoot    # Debugging and validation
│   ├── logs        # Stream pod logs
│   ├── events      # Show K8s events
│   ├── describe    # Show full deployment details
│   ├── test        # Run inference test
│   └── validate    # Full-stack validation
├── version         # Version management
│   ├── check       # Check for updates on HuggingFace
│   ├── update      # Update to new version
│   ├── pin         # Pin to specific version
│   └── unpin       # Unpin version
└── library         # Quick enable/disable workflow
    ├── list        # Show library overview
    ├── enable      # Quick deploy from library
    ├── disable     # Quick undeploy (keep cache)
    ├── swap        # Swap model versions
    ├── history     # Show deployment history
    └── alias       # Manage model aliases
```

---

## Help Text Standards

### Parent Command Format (e.g., `model registry --help`)

```
Manage the model registry - add, list, and remove models from HuggingFace Hub.

The registry stores metadata about models available to the platform. Models must
be registered before they can be cached or deployed.

Usage:
  ai-aas model registry <command> [flags]

Available Commands:
  add         Register a model from HuggingFace Hub
  list        List all registered models
  info        Show detailed model information
  remove      Remove a model from the registry
  status      Show model status across environments

Examples:
  # Register a new model
  ai-aas model registry add mistralai/Mistral-7B-v0.1 --name mistral-7b

  # List all models with their status
  ai-aas model registry list

  # Get detailed info about a model
  ai-aas model registry info mistral-7b

Workflow:
  1. Register model    →  ai-aas model registry add <hf-id> --name <name>
  2. Cache model       →  ai-aas model cache pull <name>
  3. Deploy model      →  ai-aas model deploy create <name> -e <env>
  4. Test inference    →  ai-aas model troubleshoot test <name> -e <env>

Use "ai-aas model registry <command> --help" for more information about a command.
```

### Leaf Command Format (e.g., `model registry add --help`)

```
Register a model from HuggingFace Hub in the platform registry.

This command fetches model metadata from HuggingFace and creates a registry entry.
For gated models (like Llama), you must first accept the license on HuggingFace.

Usage:
  ai-aas model registry add <hf-model-id> --name <name> [flags]

Flags:
  -n, --name string        Internal name for the model (required)
      --accept-license     Accept gated model license terms
      --gpu-memory int     Recommended GPU memory in GB
      --cpu-memory int     Recommended CPU memory in GB
      --license string     License type (auto-detected if not specified)
      --requires-auth      Model requires HuggingFace authentication

Examples:
  # Add a public model
  ai-aas model registry add mistralai/Mistral-7B-v0.1 --name mistral-7b

  # Add a gated model (must accept license on HuggingFace first)
  ai-aas model registry add meta-llama/Llama-3-8B-Instruct \
    --name llama-3-8b --accept-license

  # Add with resource recommendations
  ai-aas model registry add mistralai/Mixtral-8x7B-v0.1 \
    --name mixtral-8x7b --gpu-memory 80 --cpu-memory 128

See Also:
  ai-aas model registry list    List registered models
  ai-aas model cache pull       Download model to cache
  ai-aas model deploy create    Deploy model to cluster
```

---

## Next Steps Guidance

After each command completes, show contextual next steps:

### After `model registry add`:
```
✓ Model registered: mistral-7b

Next steps:
  ai-aas model cache pull mistral-7b           # Download to object storage
  ai-aas model deploy create mistral-7b -e dev # Deploy directly from HuggingFace
  ai-aas model registry info mistral-7b        # View model details
```

### After `model cache pull`:
```
✓ Model cached: mistral-7b (14.5 GB)

Next steps:
  ai-aas model deploy create mistral-7b -e development   # Deploy to dev
  ai-aas model cache list                                # View all cached models
```

### After `model deploy create`:
```
✓ Deployed: mistral-7b-development

The model is starting up. Large models may take 5-10 minutes to load.

Next steps:
  ai-aas model deploy status mistral-7b -e development   # Check deployment status
  ai-aas model troubleshoot logs mistral-7b -e dev       # Watch startup logs
  ai-aas model troubleshoot test mistral-7b -e dev       # Test inference (when ready)
```

### After `model deploy delete`:
```
✓ Removed deployment: mistral-7b-development

Note: Model cache is preserved. To fully remove:
  ai-aas model cache delete mistral-7b         # Remove cached files
  ai-aas model registry remove mistral-7b      # Remove from registry

To re-deploy:
  ai-aas model deploy create mistral-7b -e development
```

---

## Implementation Steps

### Phase 1: Create Help Utilities
1. Create `internal/cli/help.go` with:
   - `FormatHelp()` - Consistent help text formatting
   - `PrintNextSteps()` - Contextual next-step suggestions
   - `WorkflowSection()` - Generate workflow guidance
   - ANSI color support (with fallback for non-TTY)

### Phase 2: Create Parent Commands
2. Create new parent command files:
   - `cmd/model/registry.go` - `NewRegistryCommand()`
   - `cmd/model/cache.go` - Rename/refactor existing
   - `cmd/model/deploy.go` - Refactor to parent command
   - `cmd/model/troubleshoot.go` - Refactor to parent command
   - `cmd/model/version.go` - `NewVersionCommand()`
   - `cmd/model/library.go` - `NewLibraryCommand()`

### Phase 3: Refactor Existing Commands
3. Move existing commands into nested structure:
   - `add.go` → becomes subcommand of `registry`
   - `list.go` → becomes subcommand of `registry`
   - `info.go` → becomes subcommand of `registry`
   - `remove.go` → becomes subcommand of `registry`
   - `status.go` → split between `registry` and `deploy`
   - `pull.go` → becomes subcommand of `cache`
   - `deploy.go` functions → become subcommands of `deploy`
   - `troubleshoot.go` functions → become subcommands of `troubleshoot`
   - `enable.go` functions → become subcommands of `library`

### Phase 4: Update Root Command
4. Update `cmd/ai-aas-cli/root.go`:
   - Simplify `newModelCommand()` to only add 6 parent commands
   - Remove command groups (no longer needed with nesting)
   - Update root help text

### Phase 5: Add Next-Steps to All Commands
5. Add contextual next-step output to every command:
   - Success path suggestions
   - Failure path suggestions (what to do on error)
   - Related command references

### Phase 6: Testing and Documentation
6. Update tests and docs:
   - Update any CLI tests
   - Update README/docs with new command structure
   - Test all commands work as expected

---

## File Changes Summary

### New Files:
- `internal/cli/help.go` - Help formatting utilities
- `internal/cli/workflow.go` - Workflow and next-step logic

### Modified Files:
- `cmd/ai-aas-cli/root.go` - Simplified model command registration
- `cmd/model/add.go` - Add to registry parent, update help
- `cmd/model/list.go` - Add to registry parent, update help
- `cmd/model/info.go` - Add to registry parent, update help
- `cmd/model/remove.go` - Add to registry parent, update help
- `cmd/model/status.go` - Split into registry/deploy status
- `cmd/model/pull.go` - Add to cache parent, update help
- `cmd/model/cache.go` - Refactor as parent command
- `cmd/model/deploy.go` - Refactor as parent with create/delete/restart/scale/status
- `cmd/model/troubleshoot.go` - Refactor as parent with logs/events/describe/test/validate
- `cmd/model/validate.go` - Move under troubleshoot
- `cmd/model/enable.go` - Move under library
- `cmd/model/update.go` - Move under version
- `cmd/model/swap.go` - Move under library
- `cmd/model/alias.go` - Move under library

### Deleted Files (content merged):
- Individual command files may be consolidated into parent files

---

## Command Mapping (Old → New)

| Old Command | New Command |
|-------------|-------------|
| `model add` | `model registry add` |
| `model list` | `model registry list` |
| `model info` | `model registry info` |
| `model remove` | `model registry remove` |
| `model status` | `model registry status` / `model deploy status` |
| `model pull` | `model cache pull` |
| `model cache` | `model cache list` / `model cache info` / `model cache delete` |
| `model deploy` | `model deploy create` |
| `model undeploy` | `model deploy delete` |
| `model restart` | `model deploy restart` |
| `model scale` | `model deploy scale` |
| `model logs` | `model troubleshoot logs` |
| `model events` | `model troubleshoot events` |
| `model describe` | `model troubleshoot describe` |
| `model test` | `model troubleshoot test` |
| `model validate` | `model troubleshoot validate` |
| `model check-updates` | `model version check` |
| `model update` | `model version update` |
| `model pin` | `model version pin` |
| `model unpin` | `model version unpin` |
| `model enable` | `model library enable` |
| `model disable` | `model library disable` |
| `model library` | `model library list` |
| `model swap` | `model library swap` |
| `model history` | `model library history` |
| `model alias` | `model library alias` |

---

## Acceptance Criteria

1. All commands work with new nested syntax
2. `--help` on every command shows professional, consistent output
3. Every command that modifies state shows relevant next steps
4. `model --help` shows clear overview of all subcommand groups
5. Parent commands (`model registry`, `model cache`, etc.) show workflow guidance
6. Error messages suggest corrective actions

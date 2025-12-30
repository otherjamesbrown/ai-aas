# Developer Onboarding Guide

Welcome to the AI-AAS platform! This guide helps new developers get set up and productive.

## Prerequisites

Before starting, ensure you have:

| Tool | Version | Installation |
|------|---------|--------------|
| Go | 1.21+ | `brew install go` or [golang.org](https://golang.org/dl/) |
| kubectl | 1.28+ | `brew install kubectl` |
| Helm | 3.x | `brew install helm` |
| git-crypt | - | `brew install git-crypt` |
| Docker | - | [docker.com](https://www.docker.com/get-started) |
| make | - | Pre-installed on macOS/Linux |

## Step 1: Clone and Setup

```bash
# Clone the repository
git clone git@github.com:otherjamesbrown/ai-aas.git
cd ai-aas

# Unlock encrypted secrets (you'll need the key from an admin)
git-crypt unlock path/to/git-crypt-key

# Install dependencies
make bootstrap

# Verify setup
make check
```

## Step 2: Environment Access

### Kubeconfig

Get kubeconfig files from an admin and place them in `~/kubeconfigs/`:

```bash
mkdir -p ~/kubeconfigs
# Copy kubeconfig-development.yaml, kubeconfig-staging.yaml, etc.

# Test access
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl get nodes
```

### CLI Configuration

Configure the platform CLI:

```bash
# Build and install CLI
./scripts/build-clis.sh --install

# Configure (values from secrets/env/.env)
ai-aas-cli config set --api-endpoint=https://admin-api.dev.otherjamesbrown.com
ai-aas-cli config set --api-key=$MASTER_ADMIN_API_KEY

# Verify
ai-aas-cli status
```

### Full environment access details: [docs/platform/environment-access.md](platform/environment-access.md)

## Step 3: Understand the Codebase

### Key Directories

```
ai-aas/
├── services/              # Go microservices
│   ├── admin-api-service/ # Admin operations API
│   ├── api-router-service/# Main API gateway
│   ├── user-org-service/  # User and org management
│   └── ai-aas-cli/        # Platform CLI
├── operators/             # Kubernetes operators
│   └── ai-model-operator/ # AIModel CR controller
├── infra/                 # Infrastructure (Terraform, base K8s)
├── gitops/                # ArgoCD applications
├── specs/                 # Feature specifications
├── context/               # AI agent context docs
└── docs/                  # All documentation
```

### Core Documentation

1. **Start here**: [AI_ASSISTANT_GUIDE.md](../AI_ASSISTANT_GUIDE.md) - Platform overview
2. **Architecture**: [ARCHITECTURE.md](../ARCHITECTURE.md) - System design
3. **Feature specs**: `specs/` directory - Detailed designs
4. **Runbooks**: `docs/runbooks/` - Operational procedures

## Step 4: Development Workflow

### Daily Development

```bash
# Pull latest changes
git checkout develop
git pull origin develop

# Create feature branch (using workspace helper if available)
git checkout -b feature/my-feature develop

# Make changes, then:
make fmt        # Format code
make lint       # Check style
make test       # Run tests
make check      # All checks

# Commit and push
git add .
git commit -m "feat: add my feature"
git push origin feature/my-feature
```

### Running Services Locally

```bash
# Run all services with local dependencies
make up

# Run a specific service
make run SERVICE=admin-api-service

# Run tests for a service
make test SERVICE=admin-api-service
```

### Using Git Worktrees

For parallel work on multiple branches, use worktrees:

```bash
# Create worktree for a feature
git worktree add ~/worktrees/my-feature -b feature/my-feature develop

# Work in the worktree
cd ~/worktrees/my-feature
# ... make changes ...

# Clean up when done
git worktree remove ~/worktrees/my-feature
```

## Step 5: Key Concepts

### GitOps

**All deployments happen via Git commits**, not kubectl:

1. Edit manifests in `gitops/clusters/<env>/apps/`
2. Commit and push
3. ArgoCD syncs changes to cluster

### CLI-First Operations

**Always prefer CLI over direct API/kubectl**:

```bash
# Deploy a model
ai-aas-cli model registry add mistralai/Mistral-7B-v0.1 --name mistral-7b
ai-aas-cli model deploy create mistral-7b -e development

# Manage organizations
ai-aas-cli org create --name "Test Org"
```

### Beads Issue Tracking

We use beads for issue tracking:

```bash
bd list --status open     # List open issues
bd show aas-xxxx          # Show issue details
bd create "Title" --type bug  # Create new issue
bd close aas-xxxx         # Close issue
```

## Getting Help

| Topic | Resource |
|-------|----------|
| Platform architecture | `ARCHITECTURE.md` |
| Feature specs | `specs/` directory |
| CLI commands | `ai-aas-cli --help` |
| Environment access | `docs/platform/environment-access.md` |
| Debugging | `docs/runbooks/ai-debugging-workflow.md` |
| Deployment | `docs/runbooks/deploy-to-environments.md` |

## Common Issues

### git-crypt unlock fails

Ensure you have the correct key file. Contact an admin if needed.

### kubectl connection refused

Check your kubeconfig is set correctly:
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl config current-context
```

### CLI returns "unauthorized"

Verify your API key is set correctly:
```bash
ai-aas-cli config show
ai-aas-cli config test
```

### Tests fail with module errors

For E2E tests, disable go.work:
```bash
cd tests/e2e
GOWORK=off go test ./suites/... -tags="smoke,e2e_tier"
```

## Next Steps

1. Read the feature spec for your first task
2. Explore the relevant service code
3. Check `bd ready` for unblocked issues
4. Ask questions in team chat!

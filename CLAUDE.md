# Claude Rules for AI-AAS Platform

This document provides a set of rules and guidelines for interacting with the AI-AAS Platform repository.

## Core Principles

1.  **Spec-Driven Development**: This is a spec-driven repository. Always refer to the `specs/` directory for the relevant feature specification before making any changes.
2.  **GitOps-First Deployment**: ALWAYS use GitOps for infrastructure and deployment changes. Never make direct changes to Kubernetes clusters. All changes must go through: edit → commit → push → ArgoCD sync.
3.  **Makefile is the Entry Point**: All automation is handled through the root `Makefile`. Use `make help` to see a list of available commands.
4.  **Microservices Architecture**: The platform is composed of multiple Go-based microservices located in the `services/` directory. The `api-router-service` is the central gateway.
5.  **Shared Libraries**: Common code is located in the `shared/` directory.

## Key Files and Directories

*   `ARCHITECTURE.md`: A high-level overview of the system architecture.
*   `CONTRIBUTING.md`: Guidelines for contributing to the project.
*   `Makefile`: The main entry point for all automation.
*   `services/`: The source code for each of the microservices.
*   `shared/`: Shared libraries used by multiple services.
*   `docs/`: Detailed documentation, including runbooks and setup guides.
*   `specs/`: The feature specifications and design documents.

## Environment Access & Credentials

**CRITICAL**: Before searching for credentials or environment access information, ALWAYS check this document first:

📖 **[docs/platform/environment-access.md](docs/platform/environment-access.md)** - Complete environment access guide

This document contains:
- Kubernetes cluster access (kubeconfigs, contexts)
- ArgoCD URLs and credentials
- Database connection strings
- API endpoints and ingress IPs
- API keys and authentication tokens
- Admin CLI configuration
- SSH keys and infrastructure tokens
- Port-forwarding commands
- Troubleshooting common access issues

**Quick Access Examples:**
- Kubernetes: `kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml`
- Database: Connection string in `secrets/env/.env` as `DATABASE_URL`
- API Router: `https://api.172.232.58.222.nip.io` or `https://api.dev.ai-aas.local`
- Master Admin API Key: Found in `secrets/env/.env` as `MASTER_ADMIN_API_KEY`
- ArgoCD: `https://argocd.dev.ai-aas.local` (password retrieved from k8s secret)

## Development Workflow

1.  **Bootstrap the environment**: `make bootstrap`
2.  **Start the local stack**: `make up`
3.  **Run checks**: `make check`
4.  **Run tests**: `make test SERVICE=<service-name>`

## GitOps Deployment Workflow

**CRITICAL**: All infrastructure and Kubernetes resource changes MUST follow this workflow. Never use `kubectl apply`, `kubectl edit`, or `kubectl patch` for permanent changes.

### Correct Workflow:

1.  **Make changes locally**: Edit Helm charts, Kubernetes manifests, or ArgoCD applications in the git repository
2.  **Test locally** (optional): Validate with `helm template`, `kubectl diff`, or `make check`
3.  **Commit changes**: `git add . && git commit -m "description"`
4.  **Push to repository**: `git push origin main` (or feature branch)
5.  **ArgoCD syncs automatically** (development) or **manually sync** (production): `argocd app sync <app-name>`
6.  **Verify deployment**: Check application status with `kubectl get pods` or `argocd app get <app-name>`

### What Gets Managed by GitOps:

- ✅ Kubernetes Deployments, Services, ConfigMaps, Secrets
- ✅ Helm chart values and releases
- ✅ Ingress configurations
- ✅ Resource limits and scaling
- ✅ Any infrastructure-as-code

### What Can Use Direct Tools:

- ✅ **Application runtime data**: Routing policies, user records (use `admin-cli` or APIs)
- ✅ **Debugging**: `kubectl logs`, `kubectl describe`, `kubectl port-forward`
- ✅ **Temporary testing**: Quick validation before committing (must be followed by git commit)

### Reference Documentation:

- `docs/runbooks/deploy-to-environments.md`: Complete deployment runbook
- ArgoCD endpoints: `argocd.dev.ai-aas.local`, `argocd.prod.ai-aas.local`

## Important Commands

*   `make help`: List all available commands.
*   `make check`: Run all checks (format, lint, security, test).
*   `make build`: Build the services.
*   `make test`: Run unit tests.
*   `make up`: Start the local development environment.
*   `make stop`: Stop the local development environment.

# Terraform Infrastructure

This directory contains Terraform configuration for provisioning Akamai Linode infrastructure across the development, staging, production, and system environments.

## Directory Structure

- `backend.hcl` – Remote state configuration targeting Linode Object Storage.
- `environments/` – Per-environment stacks that compose Terraform modules for the platform.
- `environments/_shared/` – Shared configuration including node pool definitions.
- `modules/` – Reusable modules (cluster, network, observability, secrets, data services).
- `Makefile` – Developer entry points for `plan`, `apply`, `destroy`, `drift`, and `state` commands.
- `state-policy.json` – Bucket policy enforcing least-privilege access to Terraform state.

Use the Makefile targets instead of invoking `terraform` directly to ensure linting, security scans, and approvals run consistently.

## LKE Node Pools

Node pools are defined in `environments/_shared/locals.tf` under `environment_defaults`. Each environment has specific node pool configurations optimized for its workload.

### Linode Instance Types

| Type | vCPUs | RAM | GPU | Use Case |
|------|-------|-----|-----|----------|
| `g6-standard-4` | 4 | 8GB | - | General workloads |
| `g6-standard-6` | 6 | 16GB | - | Medium workloads |
| `g6-standard-8` | 8 | 32GB | - | Production workloads |
| `g7-highmem-16` | 16 | 128GB | - | High-memory workloads |
| `g1-gpu-rtx6000` | 8 | 32GB | RTX 6000 24GB | Large GPU models |
| `g2-gpu-rtx4000a1-m` | 8 | 32GB | RTX 4000 Ada 20GB | Medium GPU models (7B) |
| `g2-gpu-rtx4000a1-l` | 8 | 64GB | RTX 4000 Ada 20GB | GPU + high CPU memory |

### Current Node Pool Configuration

#### Development
```hcl
node_pools = [
  { type = "g6-standard-4", count = 2, autoscaler = { min = 2, max = 4 } },
  { type = "g1-gpu-rtx6000", count = 1, autoscaler = { min = 1, max = 1 } }
]
```

#### Staging (Singapore)
```hcl
node_pools = [
  { type = "g6-standard-6", count = 3, autoscaler = { min = 3, max = 6 } },
  { type = "g2-gpu-rtx4000a1-m", count = 1, autoscaler = { min = 1, max = 2 } },
  { type = "g2-gpu-rtx4000a1-l", count = 1, autoscaler = { min = 1, max = 1 } }
]
```

#### Production
```hcl
node_pools = [
  { type = "g6-standard-8", count = 3, autoscaler = { min = 3, max = 6 } },
  { type = "g7-highmem-16", count = 1, autoscaler = { min = 1, max = 2 } }
]
```

### Adding a New Node Pool

1. Edit `environments/_shared/locals.tf`
2. Add the pool definition to the appropriate environment:
   ```hcl
   {
     type  = "g2-gpu-rtx4000a1-l"
     count = 1
     autoscaler = {
       min = 1
       max = 2
     }
     labels = {}  # Applied via script (see below)
     taints = []
   }
   ```
3. Run `make plan ENV=<environment>` to preview changes
4. Run `make apply ENV=<environment>` to apply
5. Apply GPU node labels (if GPU pool): `./scripts/infra/apply-gpu-node-labels.sh`

### GPU Node Labels

**Important**: The Linode Terraform provider does not support node labels/taints. After creating GPU node pools, run:

```bash
./scripts/infra/apply-gpu-node-labels.sh
```

This applies:
- `nvidia.com/gpu.product=<gpu-type>` - GPU product label
- `ai-aas.io/gpu-class=<class>` - Custom classification for scheduling

### Modifying Node Pool Size

To scale a node pool:

1. Update `count` and `autoscaler.min/max` in `_shared/locals.tf`
2. `make plan ENV=<environment>`
3. `make apply ENV=<environment>`

**Warning**: Reducing `count` below current running nodes will terminate workloads.

## Commands

```bash
# Preview changes
make plan ENV=development

# Apply changes (requires approval)
make apply ENV=development

# Check for configuration drift
make drift ENV=development

# Destroy environment (dangerous!)
make destroy ENV=development
```

## Related Documentation

- [GPU Workload Troubleshooting](../docs/runbooks/gpu-workload-troubleshooting.md)
- [Infrastructure Prerequisites](../context/infra-ops-manager/agents.md#infrastructure-prerequisites)
- [Environment Access](../docs/platform/environment-access.md)

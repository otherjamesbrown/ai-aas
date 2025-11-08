# Linode Access Setup

Automation requires API access to Akamai Linode services:

- **Linode Kubernetes Engine (LKE)**: Cluster provisioning (`/lke/clusters`)
- **Linode Instances**: VM provisioning (`/linode/instances`)
- **Object Storage**: Metrics bucket management (`/object-storage/buckets`)

## Create Personal Access Token

1. Log into the Akamai Control Panel.
2. Navigate to **My Profile → API Tokens → Add a Personal Access Token**.
3. Grant the following permissions:
   - `linodes:read_write`
   - `lke:read_write`
   - `object-storage:read_write`
4. Copy the generated token and store it securely (e.g., 1Password).

## Configure Environment

```bash
export LINODE_TOKEN=<token>
export LINODE_DEFAULT_REGION=us-east
```

Store tokens in `.envrc`, `.bash_profile`, or your preferred secret manager. Do **not** commit tokens to Git.

## CLI Tools

- Official Linode CLI: https://www.linode.com/docs/products/tools/cli/
- Terraform & Helm charts should use environment variables or secret stores to access tokens.

Refer to `docs/runbooks/linode-setup.md` for full provisioning workflows.


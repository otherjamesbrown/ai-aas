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
export LINODE_DEFAULT_REGION=fr-par
```

Store tokens in `.envrc`, `.bash_profile`, or your preferred secret manager. Do **not** commit tokens to Git.

### Object Storage Credentials

Terraform (and any tooling that uses the S3-compatible backend) needs access/secret keys for Linode Object Storage:

```bash
export LINODE_OBJECT_STORAGE_ACCESS_KEY=<access-key>
export LINODE_OBJECT_STORAGE_SECRET_KEY=<secret-key>
export AWS_ACCESS_KEY_ID=$LINODE_OBJECT_STORAGE_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=$LINODE_OBJECT_STORAGE_SECRET_KEY
```

We keep these values in `.env` as `LINODE_OBJECT_STORAGE_ACCESS_KEY` / `LINODE_OBJECT_STORAGE_SECRET_KEY`. The Terraform CLI expects the `AWS_*` variables, so export them (or `source .env`) before running `terraform init/plan/apply`.

## CLI Tools

- Official Linode CLI: https://www.linode.com/docs/products/tools/cli/
- Terraform & Helm charts should use environment variables or secret stores to access tokens.
- Object Storage commands require a cluster flag (e.g., `--cluster fr-par-1`):  
  `linode-cli obj ls --cluster fr-par-1 ai-aas/terraform/environments/production`

Refer to `docs/runbooks/linode-setup.md` for full provisioning workflows.


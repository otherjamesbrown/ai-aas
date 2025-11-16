# Dev Workspace Terraform Module

Terraform module for provisioning ephemeral Linode development workspaces with Docker Compose stack, Vector agent, and systemd orchestration.

## Usage

```hcl
module "dev_workspace" {
  source = "../../modules/dev-workspace"

  workspace_name = "dev-jab"
  region         = "us-east"
  owner          = "jab"
  ttl_hours      = 24
  instance_type  = "g6-nanode-1"
  
  ssh_keys = ["12345", "67890"]
  tags     = ["environment:development"]
}
```

## Features

- **Ephemeral instances**: TTL-based lifecycle with automatic cleanup tags
- **StackScript bootstrap**: Automated Docker Compose and Vector agent installation
- **Private VLAN support**: Secure networking with private VLAN attachment
- **Systemd orchestration**: Resilient service management for dev stack
- **Observability**: Vector agent pre-configured for log shipping
- **Security**: Tag-based access control and audit logging

## Inputs

See `variables.tf` for complete list. Key variables:

- `workspace_name` - Unique identifier (required)
- `region` - Linode region (required)
- `owner` - Workspace owner (required)
- `ttl_hours` - Time-to-live in hours (default: 24)
- `instance_type` - Linode instance type (default: g6-nanode-1)
- `vlan_id` - Private VLAN ID (optional)
- `ssh_keys` - Linode SSH key IDs (optional)
- `authorized_keys` - SSH public keys (optional)

## Outputs

- `instance_id` - Linode instance ID
- `public_ip` - Public IP address
- `private_ip` - Private IP address (if VLAN configured)
- `ssh_command` - SSH command to connect
- `ttl_timestamp` - TTL expiration timestamp
- `tags` - Instance tags

## StackScript

The bootstrap StackScript (`files/bootstrap.sh`) installs:

1. Docker Engine and Compose v2
2. Vector agent for log shipping
3. Systemd services for dev stack orchestration
4. Log rotation and cleanup scripts
5. Health check utilities

## Lifecycle

Workspaces are designed to be ephemeral with TTL enforcement:

- Provision: `terraform apply`
- Check status: `make remote-status`
- Destroy: `terraform destroy` or `make remote-destroy`

## Security

- Instances tagged with owner, TTL, and workspace metadata
- Private VLAN attachment for internal networking
- Audit logging via Vector agent
- 24-hour TTL default (configurable)

## Dependencies

- Linode Terraform provider ~> 2.12
- Random provider ~> 3.5 (for password generation)
- Linode API token with instance/StackScript permissions

## See Also

- `specs/002-local-dev-environment/` - Specification and design
- `scripts/dev/remote_provision.sh` - Provisioning wrapper
- `scripts/dev/remote_lifecycle.sh` - Lifecycle operations


# Infrastructure Management

## Overview

This guide covers managing infrastructure components for the AIaaS platform, including Kubernetes clusters, databases, and supporting services.

## Infrastructure Components

### Kubernetes Clusters

The platform runs on Kubernetes clusters managed via:
- Terraform for infrastructure provisioning
- ArgoCD for GitOps deployments
- Helm charts for service packaging

### Databases

- **Operational Database**: PostgreSQL for core entities (organizations, users, API keys)
- **Analytics Database**: PostgreSQL for usage analytics and reporting

### Supporting Services

- Redis for caching and session management
- Kafka for event streaming
- Object storage for artifacts and backups

## Common Tasks

### Provisioning New Environments

1. Configure Terraform variables for the environment
2. Initialize Terraform backend
3. Plan and apply infrastructure changes

```bash
cd infra/terraform/environments/<environment>
terraform init
terraform plan
terraform apply
```

### Updating Infrastructure

1. Modify Terraform configuration
2. Review changes with `terraform plan`
3. Apply changes incrementally

### Scaling Resources

- **Compute**: Adjust node pool sizes in Kubernetes
- **Database**: Scale PostgreSQL instances
- **Storage**: Increase volume sizes or add storage classes

## Monitoring Infrastructure Health

- Check cluster status via `kubectl`
- Review infrastructure metrics in Grafana
- Monitor resource utilization and alerts

## Disaster Recovery

- Regular backups of databases and configurations
- Documented recovery procedures
- Tested restore processes

See [Disaster Recovery](./disaster-recovery.md) for detailed procedures.

## Security Considerations

- Network policies and firewall rules
- Secret management and rotation
- Access control and RBAC

See [Security Hardening](./security-hardening.md) for best practices.

## Related Documentation

- [Infrastructure Overview](../../docs/platform/infrastructure-overview.md)
- [Linode Access Guide](../../docs/platform/linode-access.md)
- [Terraform Documentation](../../infra/terraform/README.md)


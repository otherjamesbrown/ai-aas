# Bootstrap and Initial Setup

## Overview

This guide covers the initial setup and bootstrap procedures for a new AIaaS platform instance.

## Prerequisites

- Access to infrastructure provider (Akamai Linode)
- Administrative access to Kubernetes cluster
- Database credentials and connection strings
- Required environment variables configured

## Bootstrap Process

### 1. Initial Platform Bootstrap

Run the bootstrap script to create the first system administrator account:

```bash
./scripts/setup/bootstrap.sh
```

This creates:
- First system admin account
- Initial database schemas
- Core service configurations

### 2. Database Setup

Initialize database schemas:

```bash
make db-migrate-status
make db-migrate-up
```

### 3. Service Deployment

Deploy core services:

```bash
# Deploy user-org service
make deploy SERVICE=user-org-service

# Deploy API router service
make deploy SERVICE=api-router-service

# Deploy analytics service
make deploy SERVICE=analytics-service
```

### 4. Verify Installation

Check service health:

```bash
make health-check
```

## Post-Bootstrap Configuration

### Configure Monitoring

Set up observability stack:

1. Deploy Grafana dashboards
2. Configure Prometheus scraping
3. Set up alerting rules

See [Monitoring and Observability](./monitoring-observability.md) for details.

### Configure Network Access

Set up ingress and TLS:

1. Configure ingress controller
2. Set up TLS certificates
3. Configure firewall rules

See [Network Configuration](./network-configuration.md) for details.

## Troubleshooting

Common issues during bootstrap:

- **Database connection failures**: Verify connection strings and network access
- **Service startup errors**: Check logs and resource limits
- **Authentication issues**: Verify credential configuration

See the [troubleshooting guide](../troubleshooting.md) for more details.

## Next Steps

- [Infrastructure Management](./infrastructure-management.md)
- [Service Deployment](./service-deployment.md)
- [Monitoring and Observability](./monitoring-observability.md)


# Service Deployment

## Overview

This guide covers deploying and updating services in the AIaaS platform using GitOps workflows and Helm charts.

## Deployment Methods

### GitOps with ArgoCD

Services are deployed via ArgoCD using GitOps principles:

1. Update service manifests in `gitops/clusters/<environment>/`
2. ArgoCD automatically syncs changes
3. Monitor deployment status in ArgoCD UI

### Manual Deployment

For emergency deployments or testing:

```bash
make deploy SERVICE=<service-name> ENV=<environment>
```

## Service Lifecycle

### Deploying a New Service

1. Create service manifests in GitOps repository
2. Configure ArgoCD application
3. Set up monitoring and alerting
4. Deploy to development first, then staging, then production

### Updating an Existing Service

1. Update service image or configuration
2. Commit changes to GitOps repository
3. ArgoCD syncs automatically (or trigger manual sync)
4. Monitor deployment rollout

### Rolling Back a Service

1. Identify the previous working version
2. Update GitOps manifests to previous version
3. Trigger ArgoCD sync
4. Verify service health

## Service Configuration

### Environment Variables

Configure service-specific environment variables in:
- Helm values files
- Kubernetes ConfigMaps and Secrets
- ArgoCD application parameters

### Resource Limits

Set appropriate resource requests and limits:
- CPU requests and limits
- Memory requests and limits
- Storage requirements

### Health Checks

Configure liveness and readiness probes:
- HTTP health endpoints
- Startup and liveness probe intervals
- Timeout and failure thresholds

## Monitoring Deployments

- Check deployment status via `kubectl`
- Review ArgoCD sync status
- Monitor service metrics and logs
- Verify health endpoints

## Troubleshooting

Common deployment issues:

- **Image pull errors**: Check image registry access
- **Resource constraints**: Review resource limits
- **Health check failures**: Verify service endpoints
- **Configuration errors**: Validate manifests

## Related Documentation

- [ArgoCD Bootstrap](../../docs/runbooks/argocd-bootstrap.md)
- [Service Template](../../samples/service-template/README.md)
- [GitOps Documentation](../../gitops/README.md)


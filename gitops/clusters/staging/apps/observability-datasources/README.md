# Grafana Datasources for Observability Stack

This directory contains Grafana datasource configurations that are auto-discovered by the Grafana sidecar in kube-prometheus-stack.

## How It Works

The Grafana deployment includes a sidecar container that watches for ConfigMaps with the label `grafana_datasource: "1"`. When found, the sidecar automatically provisions these datasources in Grafana.

## Datasources

### Loki (`loki-datasource.yaml`)

**Status**: Active and operational

- **URL**: `http://loki-gateway.observability.svc.cluster.local`
- **Purpose**: Log aggregation and querying
- **Features**:
  - LogQL query interface
  - Derived field to extract `trace_id` and link to Tempo
  - Max 1000 lines per query

### Tempo (`tempo-datasource.yaml`)

**Status**: Configuration ready, but Tempo not yet deployed in staging

- **URL**: `http://tempo.observability.svc.cluster.local:3200`
- **Purpose**: Distributed tracing
- **Tracked in**: aas-8fg3p (Deploy Tempo to staging)
- **Features**:
  - Trace-to-logs correlation with Loki
  - Node graph for service topology
  - Tag mapping for trace_id correlation

**Note**: This datasource will appear as "unavailable" in Grafana until Tempo is deployed. Once aas-8fg3p is complete, this datasource will automatically become active.

## Deployment

Deployed via ArgoCD application: `grafana-datasources-staging`

```bash
# Apply the ArgoCD application
kubectl apply -f gitops/clusters/staging/apps/grafana-datasources.yaml

# Verify datasources are created
kubectl get configmap -n monitoring -l grafana_datasource=1

# Check Grafana has loaded them (after ~30 seconds)
# Login to Grafana UI and go to Configuration > Data Sources
```

## Related Documentation

- [Observability Architecture](../../../../docs/architecture/observability-architecture.md) - Full observability stack
- [Environment Access](../../../../docs/platform/environment-access.md) - Grafana credentials
- [Observability Guide](../../../../docs/platform/observability-guide.md) - Monitoring and logging

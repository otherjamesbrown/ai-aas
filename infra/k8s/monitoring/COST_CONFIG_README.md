# Cost Configuration for Usage Tracking

This directory contains the cost configuration used by Grafana dashboards to calculate infrastructure costs and organization cost allocation.

## Overview

The `cost-config.yaml` ConfigMap provides cost parameters that enable:
- **Org Usage Dashboard**: Track token consumption and costs per organization
- **Cost Efficiency Dashboard**: Analyze tokens per GPU-hour and cost efficiency
- **Per-Model Performance**: Calculate model-level cost metrics

## Cost Configuration File

**Location**: `infra/k8s/monitoring/cost-config.yaml`

### Key Configuration Parameters

| Parameter | Description | Default Value |
|-----------|-------------|---------------|
| `cost_per_million_input_tokens` | Cost per 1M input tokens (USD) | $1.50 |
| `cost_per_million_output_tokens` | Cost per 1M output tokens (USD) | $3.00 |
| `gpu_hourly_costs` | GPU type to hourly cost mapping | See file |
| `instance_type_to_gpu` | Linode instance type to GPU type mapping | See file |
| `org_allocation_method` | Organization cost allocation method | proportional |
| `monthly_infrastructure_overhead` | Fixed monthly overhead (USD) | $500 |
| `markup_percentage` | Markup on infrastructure costs | 20% |

### GPU Types and Costs

Based on Linode GPU instance pricing:

| GPU Type | Hourly Cost (USD) | Instance Type |
|----------|-------------------|---------------|
| RTX 4000 Ada (20GB) | $1.50 | g2-gpu-rtx4000a1-m |
| A100 (40GB) | $2.50 | - |
| A100 (80GB) | $3.50 | - |
| V100 (16GB) | $1.20 | - |
| L40S (48GB) | $2.00 | - |
| Default (unknown) | $1.50 | - |

### Token Costs

Token costs are based on typical LLM provider pricing:
- **Input tokens**: $1.50 per 1M tokens (reading/processing)
- **Output tokens**: $3.00 per 1M tokens (generation)

These costs reflect the infrastructure overhead of:
- GPU compute time
- Memory bandwidth
- Power consumption
- Cooling and infrastructure
- Platform margin (20% markup)

## Cost Efficiency Thresholds

Used for dashboard alerting and visualization:

| Metric | Excellent | Good | Acceptable | Poor |
|--------|-----------|------|------------|------|
| **Tokens/GPU-hour** | >1M | 500K-1M | 250K-500K | <250K |
| **Cost/Request** | <$0.001 | $0.001-$0.005 | $0.005-$0.010 | >$0.010 |

## Organization Cost Allocation

### Proportional Method (Default)

Costs are distributed based on actual token consumption:
```
org_cost = (org_tokens / total_tokens) × total_infrastructure_cost
```

### Fixed Overhead

The platform charges a fixed monthly overhead of $500 for:
- Control plane infrastructure
- Monitoring and observability
- ArgoCD and GitOps tooling
- Shared networking and ingress
- Database and storage

This overhead is distributed proportionally across organizations.

## Using Cost Config in Dashboards

### Accessing Cost Parameters

Dashboards can reference cost config via ConfigMap mount or direct queries:

```yaml
# In a pod/deployment
envFrom:
  - configMapRef:
      name: cost-config
```

### PromQL Cost Calculations

The ConfigMap includes template queries for common calculations:

**Total cost by model (tokens + GPU)**:
```promql
(
  sum(increase(vllm:prompt_tokens_total{model_name="$model"}[1h])) * 1.50 / 1000000
  +
  sum(increase(vllm:generation_tokens_total{model_name="$model"}[1h])) * 3.00 / 1000000
  +
  sum(count_over_time(up{job="vllm",model_name="$model"}[1h]) / 60) * 1.50
)
```

**Efficiency (tokens per GPU-hour)**:
```promql
(
  sum(increase(vllm:prompt_tokens_total{model_name="$model"}[1h]))
  +
  sum(increase(vllm:generation_tokens_total{model_name="$model"}[1h]))
) / (count_over_time(up{job="vllm",model_name="$model"}[1h]) / 60)
```

**Cost per request**:
```promql
(total_cost / request_count)
```

## Updating Cost Configuration

### Local Development

Edit `cost-config.yaml` and apply manually:
```bash
kubectl apply -f infra/k8s/monitoring/cost-config.yaml
```

### Production (GitOps)

1. **Edit the ConfigMap**: Modify `infra/k8s/monitoring/cost-config.yaml`
2. **Commit changes**:
   ```bash
   git add infra/k8s/monitoring/cost-config.yaml
   git commit -m "chore(monitoring): update cost configuration"
   ```
3. **Push to repository**:
   ```bash
   git push origin develop  # or staging/main
   ```
4. **ArgoCD syncs automatically**: Changes propagate within 3 minutes
5. **Grafana picks up changes**: Dashboards refresh automatically (no restart needed)

### Verification

Check that the ConfigMap is deployed:
```bash
kubectl get configmap cost-config -n monitoring -o yaml
```

View specific cost parameters:
```bash
kubectl get configmap cost-config -n monitoring -o jsonpath='{.data.cost_per_million_input_tokens}'
```

## Cost Calculation Examples

### Example 1: Small Model (7B parameters)
- **Tokens processed**: 10M input + 5M output in 1 hour
- **GPU usage**: 1x RTX 4000 Ada for 1 hour
- **Cost calculation**:
  - Input tokens: 10M × $1.50/1M = $15.00
  - Output tokens: 5M × $3.00/1M = $15.00
  - GPU time: 1 hour × $1.50/hour = $1.50
  - **Total**: $31.50/hour
- **Efficiency**: 15M tokens / 1 GPU-hour = 15M tokens/GPU-hour (Excellent)

### Example 2: Large Model (20B parameters)
- **Tokens processed**: 5M input + 2M output in 1 hour
- **GPU usage**: 1x RTX 4000 Ada for 1 hour
- **Cost calculation**:
  - Input tokens: 5M × $1.50/1M = $7.50
  - Output tokens: 2M × $3.00/1M = $6.00
  - GPU time: 1 hour × $1.50/hour = $1.50
  - **Total**: $15.00/hour
- **Efficiency**: 7M tokens / 1 GPU-hour = 7M tokens/GPU-hour (Excellent)

### Example 3: Multi-GPU Deployment
- **Tokens processed**: 20M input + 10M output in 1 hour
- **GPU usage**: 4x A100 (40GB) for 1 hour
- **Cost calculation**:
  - Input tokens: 20M × $1.50/1M = $30.00
  - Output tokens: 10M × $3.00/1M = $30.00
  - GPU time: 4 GPUs × 1 hour × $2.50/hour = $10.00
  - **Total**: $70.00/hour
- **Efficiency**: 30M tokens / 4 GPU-hours = 7.5M tokens/GPU-hour (Excellent)

## Environment-Specific Considerations

### Development
- Uses RTX 4000 Ada GPUs primarily
- Cost multiplier: 1.0 (no adjustment)
- Focus on cost efficiency metrics for optimization

### Staging
- May use different GPU types
- Cost multiplier: 1.0 (same as production pricing)
- Pre-production cost validation

### Production
- Mixed GPU types based on workload
- Cost multiplier: 1.0 (actual customer billing rates)
- Strict cost tracking and allocation

## Cost Optimization Strategies

Based on efficiency thresholds:

### Excellent Efficiency (>1M tokens/GPU-hour)
- Continue current configuration
- Model is well-optimized for hardware
- Consider scaling up workload

### Good Efficiency (500K-1M tokens/GPU-hour)
- Monitor for optimization opportunities
- Review batch size and concurrency settings
- Check for underutilization periods

### Acceptable Efficiency (250K-500K tokens/GPU-hour)
- Investigate model configuration
- Review `--max-num-seqs` and `--max-model-len` parameters
- Consider different GPU types

### Poor Efficiency (<250K tokens/GPU-hour)
- Immediate optimization required
- Check for idle GPU time
- Review vLLM configuration parameters
- Consider model quantization or pruning
- Evaluate alternative inference engines

## Related Documentation

- **Observability Guide**: `docs/platform/observability-guide.md`
- **vLLM Best Practices**: `docs/platform/vllm-best-practices.md`
- **Cost Dashboard Spec**: Created in bead ai-aas-6eq7
- **Org Usage Dashboard Spec**: Created in bead ai-aas-s0pj

## Troubleshooting

### Cost Metrics Not Appearing in Dashboards

1. **Verify ConfigMap exists**:
   ```bash
   kubectl get configmap cost-config -n monitoring
   ```

2. **Check ArgoCD sync status**:
   ```bash
   argocd app get monitoring-config-development
   ```

3. **Verify Grafana can access ConfigMap**:
   ```bash
   kubectl describe configmap cost-config -n monitoring
   ```

### Cost Calculations Seem Incorrect

1. **Verify token metrics are being collected**:
   ```bash
   kubectl exec -n monitoring prometheus-xyz -- \
     promtool query instant 'vllm:prompt_tokens_total'
   ```

2. **Check GPU type mapping**:
   - Ensure Kubernetes node labels include GPU type
   - Verify instance type mapping in cost-config.yaml

3. **Review cost parameters**:
   - Confirm token costs match expected values
   - Verify GPU hourly costs are correct for your provider

### Cost Config Not Updating

1. **Force ArgoCD sync**:
   ```bash
   argocd app sync monitoring-config-development --force
   ```

2. **Check for ConfigMap immutability issues**:
   - Delete and recreate if ConfigMap is immutable
   ```bash
   kubectl delete configmap cost-config -n monitoring
   argocd app sync monitoring-config-development
   ```

## Maintenance

### Regular Review Schedule

- **Monthly**: Review cost parameters against actual cloud provider bills
- **Quarterly**: Audit cost allocation across organizations
- **Annually**: Update token costs based on market rates and infrastructure changes

### Cost Config Audit Checklist

- [ ] Token costs reflect actual infrastructure costs + margin
- [ ] GPU hourly costs match cloud provider pricing
- [ ] Instance type mappings are complete and accurate
- [ ] Efficiency thresholds align with business goals
- [ ] Markup percentage is appropriate
- [ ] Fixed overhead is distributed fairly

## Contact

For questions about cost configuration:
- Platform team: Review `docs/platform/observability-guide.md`
- Bead tracking: Check ai-aas-mciz for cost config bead
- Dashboard issues: Reference org usage (ai-aas-s0pj) or cost efficiency (ai-aas-6eq7) beads

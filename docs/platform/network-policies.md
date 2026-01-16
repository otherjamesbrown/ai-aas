# Network Policies

---
last_updated: 2025-12-30
document_type: guide
---

## Overview

NetworkPolicies provide network segmentation and security for Kubernetes services. This document covers requirements for services deployed with NetworkPolicies.

## Key Requirement: Explicit Egress Rules

**CRITICAL**: Services with NetworkPolicies require explicit egress rules for ALL external dependencies.

By default, NetworkPolicies deny all traffic not explicitly allowed. If your service connects to external services (databases, APIs, etc.), you MUST add egress rules.

## Common External Dependencies

| Dependency | Egress Rule Required |
|------------|---------------------|
| PostgreSQL database | Yes - port 5432 to database CIDR |
| Redis cache | Yes - port 6379 to Redis service |
| S3/Object storage | Yes - port 443 to S3 endpoint |
| External APIs | Yes - port 443 to API CIDR |
| DNS | Yes - port 53 to kube-dns |
| Kubernetes API | Yes - port 443 to API server |

## NetworkPolicy Template

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: my-service-network-policy
  namespace: development
spec:
  podSelector:
    matchLabels:
      app: my-service
  policyTypes:
    - Ingress
    - Egress

  # Allow incoming traffic
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: development
      ports:
        - protocol: TCP
          port: 8080

  # Allow outgoing traffic (MUST list all dependencies)
  egress:
    # DNS resolution (required for almost all services)
    - to:
        - namespaceSelector:
            matchLabels:
              name: kube-system
      ports:
        - protocol: UDP
          port: 53

    # PostgreSQL database
    - to:
        - ipBlock:
            cidr: 10.0.0.0/8  # Adjust to your database CIDR
      ports:
        - protocol: TCP
          port: 5432

    # External HTTPS (S3, external APIs)
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
            except:
              - 10.0.0.0/8
              - 172.16.0.0/12
              - 192.168.0.0/16
      ports:
        - protocol: TCP
          port: 443
```

## Debugging Connectivity Issues

If a service suddenly can't reach an external dependency:

```bash
# Check if NetworkPolicy exists
kubectl get networkpolicy -n <namespace>

# Describe the policy
kubectl describe networkpolicy <name> -n <namespace>

# Test connectivity from pod
kubectl exec -it <pod> -n <namespace> -- nc -zv <host> <port>

# Check if egress rule exists for the target
kubectl get networkpolicy <name> -n <namespace> -o yaml | grep -A20 "egress:"
```

## Deployment Checklist

Before deploying a service with NetworkPolicy:

- [ ] List all external dependencies (databases, caches, APIs)
- [ ] Add egress rule for each dependency
- [ ] Include DNS egress (port 53 to kube-system)
- [ ] Test connectivity after deployment
- [ ] Document dependencies in service's DEPLOYMENT.md

## Anti-patterns

```yaml
# WRONG: No egress rules = no outbound traffic!
spec:
  policyTypes:
    - Egress
  egress: []  # Blocks ALL outbound traffic

# WRONG: Forgetting DNS
egress:
  - to:
      - ipBlock:
          cidr: 10.0.0.0/8
    ports:
      - port: 5432
  # Missing DNS rule - service can't resolve hostnames!

# CORRECT: Always include DNS
egress:
  - to:
      - namespaceSelector:
          matchLabels:
            name: kube-system
    ports:
      - protocol: UDP
        port: 53
  # ... other rules
```

## Related Documentation

- [Infrastructure Troubleshooting](../runbooks/infrastructure-troubleshooting.md)
- [Service Checklist](../go-services/service-checklist.md)

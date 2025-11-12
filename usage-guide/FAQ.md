# Frequently Asked Questions (FAQ)

## Overview

This FAQ addresses common questions about the AIaaS platform. Questions are organized by topic to help you quickly find answers.

## Table of Contents

- [Getting Started](#getting-started)
- [Authentication & Access](#authentication--access)
- [API Usage](#api-usage)
- [Billing & Budgets](#billing--budgets)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [Platform Features](#platform-features)
- [Technical Questions](#technical-questions)

---

## Getting Started

### How do I get access to the platform?

Contact your organization administrator to request access. They will send you an invitation email. Once you accept the invitation, you can set up your account and configure your profile.

### What roles are available in the platform?

The platform supports multiple personas:
- **Architects**: System architecture and design
- **Security**: Security policies and compliance
- **System Administrators**: Platform infrastructure
- **Operations**: Day-to-day operations
- **Organization Administrators**: Organization management
- **Finance**: Budget and billing management
- **Developers**: API integration
- **Managers**: Team oversight
- **Analysts**: Usage analytics

### How do I set up my account?

1. Accept the invitation email from your organization administrator
2. Complete your profile setup
3. Set up authentication (MFA recommended)
4. Review organization settings
5. Configure your preferences

### Where can I find documentation?

- [Getting Started Guide](./getting-started.md) - First-time setup
- [Persona Guides](./README.md) - Role-specific documentation
- [Glossary](./glossary.md) - Key terms and concepts
- [Troubleshooting Guide](./troubleshooting.md) - Common issues

---

## Authentication & Access

### How do I authenticate API requests?

Use API keys for authentication. Include the API key in the `X-API-Key` header:

```bash
curl -X POST https://api.example.com/v1/inference \
  -H "X-API-Key: your-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o", "messages": [...]}'
```

See [API Authentication](./developer/api-authentication.md) for details.

### How do I get an API key?

Contact your organization administrator to create an API key. They can create keys with appropriate scopes and permissions for your use case.

### What is Multi-Factor Authentication (MFA)?

MFA adds an extra layer of security by requiring a second authentication factor (like a TOTP code from an authenticator app) in addition to your password. MFA is recommended for all users and required for sensitive roles.

### How do I enable MFA?

1. Log in to the web portal
2. Go to your profile settings
3. Navigate to Security settings
4. Follow the MFA setup instructions
5. Scan the QR code with your authenticator app

### I forgot my password. How do I reset it?

Use the "Forgot Password" link on the login page. You'll receive an email with password reset instructions. If you have MFA enabled, you'll need to complete MFA verification.

### My API key was compromised. What should I do?

1. Immediately revoke the compromised API key in the web portal
2. Create a new API key with the same scopes
3. Update your applications to use the new key
4. Review access logs for unauthorized usage
5. Report the incident to your security team

See [API Key Security](./org-admin/api-key-security.md) for details.

### Why am I getting 401 Unauthorized errors?

Common causes:
- Invalid or expired API key
- Missing `X-API-Key` header
- API key has been revoked
- Account has been suspended

Verify your API key is correct and hasn't been revoked. Contact your organization administrator if issues persist.

### Why am I getting 403 Forbidden errors?

This means your API key or user account doesn't have permission for the requested action. Common causes:
- Insufficient API key scopes
- User role doesn't have required permissions
- Organization-level restrictions

Contact your organization administrator to review your permissions.

---

## API Usage

### What models are available?

Available models depend on your organization's configuration and backend setup. Common models include GPT-4o, GPT-3.5-turbo, and other OpenAI-compatible models. Check with your organization administrator for available models.

### How do I make an inference request?

Use the `/v1/inference` endpoint (OpenAI-compatible):

```bash
curl -X POST https://api.example.com/v1/inference \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

See [Making API Requests](./developer/making-api-requests.md) for details.

### What are rate limits?

Rate limits control how many requests you can make within a time period. Limits are configured per API key and organization. When exceeded, you'll receive a `429 Too Many Requests` response.

See [Rate Limiting](./developer/rate-limiting.md) for details.

### How do I handle rate limit errors?

Implement exponential backoff:
1. Wait before retrying (start with 1 second)
2. Double the wait time on each retry
3. Respect the `Retry-After` header if provided
4. Reduce request frequency if needed

### What is the maximum request size?

The maximum request payload size is 64 KB. For larger inputs, consider:
- Splitting requests
- Using streaming responses
- Optimizing prompt size

### How do I use streaming responses?

Include `"stream": true` in your request:

```json
{
  "model": "gpt-4o",
  "messages": [...],
  "stream": true
}
```

The response will use Server-Sent Events (SSE) format.

### What SDKs are available?

Check the [SDK Usage](./developer/sdk-usage.md) guide for available SDKs and examples. SDKs are available for popular languages including Python, JavaScript, Go, and more.

### How do I handle errors?

All errors follow RFC7807 format with:
- `type`: Error type URI
- `title`: Human-readable error title
- `status`: HTTP status code
- `detail`: Detailed error message

See [Error Handling](./developer/error-handling.md) for details.

---

## Billing & Budgets

### How does billing work?

Billing is based on token usage. Each model has a cost per token (input and output tokens may have different rates). Costs are tracked per organization and can be viewed in the analytics dashboard.

### What is a budget?

A budget is a spending limit configured for your organization to control costs and prevent overage. Budgets can be set at the organization level with alert thresholds.

### How do I set up a budget?

Organization administrators can set budgets:
1. Go to Organization Settings
2. Navigate to Budgets
3. Create a new budget
4. Set spending limits and alert thresholds
5. Configure enforcement actions

See [Setting Budgets](./org-admin/setting-budgets.md) for details.

### What happens when I exceed my budget?

When a budget is exceeded:
- Requests may be blocked (depending on budget policy)
- Alerts are sent to configured recipients
- Usage continues to be tracked
- You can increase the budget limit

### How do I view my usage and costs?

- **Analytics Dashboard**: View usage analytics and costs
- **Usage Reports**: Generate detailed usage reports
- **Budget Monitoring**: Track budget usage and alerts

See [Viewing Usage](./org-admin/viewing-usage.md) for details.

### How are costs calculated?

Costs are calculated based on:
- Tokens consumed (input + output)
- Model pricing (varies by model)
- Time period (hourly, daily, monthly)

See [Cost Analysis](./finance/cost-analysis.md) for details.

### Can I get invoices?

Yes, invoices are available through the Finance dashboard. Organization administrators can configure billing settings and payment methods.

See [Invoice Management](./finance/invoice-management.md) for details.

---

## Security

### How secure is the platform?

The platform implements multiple security layers:
- **Authentication**: API keys, OAuth2, MFA
- **Authorization**: RBAC, API key scoping
- **Network Security**: TLS encryption, network policies
- **Data Protection**: Encryption at rest and in transit
- **Security Monitoring**: Continuous threat detection

See [Security Architecture](./security/security-architecture.md) for details.

### Are API keys encrypted?

API keys are stored as SHA-256 hashes in the database. The original key is never stored and cannot be retrieved once created.

### How often should I rotate API keys?

Recommended rotation schedule:
- **Production keys**: Every 90 days
- **Development keys**: Every 180 days
- **After security incident**: Immediately

See [API Key Security](./org-admin/api-key-security.md) for details.

### What data is logged?

The platform logs:
- Authentication events (successful and failed)
- Authorization decisions
- API usage (requests, responses, errors)
- Administrative actions
- Security events

All logs are stored securely and retained according to compliance requirements.

### How do I report a security issue?

Report security issues to your organization's security team or platform administrators. For critical security vulnerabilities, follow your organization's security incident response procedures.

See [Security Incident Response](./security/security-incident-response.md) for details.

### Is my data encrypted?

Yes:
- **In Transit**: TLS 1.3 encryption for all API communications
- **At Rest**: Database encryption, object storage encryption
- **Secrets**: Encrypted secret storage (Linode Secret Manager)

---

## Troubleshooting

### Why are my requests slow?

Common causes:
- High request volume
- Large payloads
- Network latency
- Service load

Solutions:
- Optimize request patterns
- Reduce payload size
- Implement caching
- Check service status

See [Performance Optimization](./developer/performance-optimization.md) for details.

### Why am I getting timeouts?

Timeouts can occur due to:
- Slow model responses
- Network issues
- Service overload
- Request size

Solutions:
- Increase timeout settings
- Optimize requests
- Check service status
- Contact support if persistent

### Why isn't my usage showing up in analytics?

Usage data may take time to appear:
- **Real-time**: Available within minutes
- **Aggregated**: Available within hours
- **Reports**: Available within 24 hours

If data doesn't appear after 24 hours, contact support.

### Why are my budget alerts not working?

Check:
- Alert configuration is correct
- Email address is valid
- Alert thresholds are set
- Notification settings are enabled

See [Budget Monitoring](./org-admin/budget-monitoring.md) for details.

### How do I check service status?

- **Health Endpoints**: `/healthz` and `/readyz` endpoints
- **Status Page**: Public status page (if available)
- **Monitoring Dashboard**: Operations dashboard
- **Support**: Contact support for status updates

---

## Platform Features

### What is declarative configuration?

Declarative configuration uses Git as the source of truth. System state is defined in version control and automatically reconciled by the platform. This enables:
- Version-controlled configuration
- Drift detection
- Automated reconciliation
- Audit trail

See [Declarative Configuration](./system-admin/declarative-configuration.md) for details.

### Can I export my data?

Yes, data exports are available:
- **Usage Data**: CSV exports via Analytics Service
- **Audit Logs**: Audit log exports
- **Financial Data**: Financial reports and exports

See [Data Export](./analyst/data-export.md) for details.

### How do I manage team members?

Organization administrators can:
- Invite new members
- Assign roles
- Manage permissions
- Remove members

See [Inviting Members](./org-admin/inviting-members.md) and [Role Management](./org-admin/role-management.md) for details.

### What roles and permissions are available?

Roles include:
- **Organization Admin**: Full organization management
- **Developer**: API access
- **Manager**: Team oversight
- **Analyst**: Analytics access
- **Finance**: Budget and billing management

See [Access Control](./org-admin/access-control.md) for details.

### Can I use the platform for multiple projects?

Yes, you can organize usage by:
- **API Keys**: Create separate keys per project
- **Scopes**: Use API key scopes to limit access
- **Budgets**: Set budgets per project
- **Analytics**: Filter analytics by project

---

## Technical Questions

### What is the API versioning strategy?

The platform uses URL-based versioning (`/v1/`, `/v2/`). When breaking changes are introduced, a new version is released. Previous versions are maintained for backward compatibility.

See [Architectural Principles](./architect/architectural-principles.md#versioning-strategy) for details.

### How do I migrate between API versions?

1. Review the version changelog
2. Update your code to use the new version
3. Test thoroughly
4. Deploy gradually
5. Monitor for issues

### What is the platform architecture?

The platform consists of three core microservices:
- **API Router Service**: Routes inference requests
- **User & Organization Service**: Manages identity and access
- **Analytics Service**: Processes usage analytics

See [System Components](./architect/system-components.md) for details.

### How does the platform scale?

The platform scales horizontally:
- Stateless services scale independently
- Load balancing via Kubernetes
- Auto-scaling based on demand
- Database connection pooling

See [Architecture Overview](./architect/architecture-overview.md) for details.

### What databases does the platform use?

- **PostgreSQL**: Primary data store (per service)
- **TimescaleDB**: Time-series analytics data
- **Redis**: Caching and rate limiting
- **RabbitMQ**: Message queue for async processing

### How do I integrate with CI/CD?

The platform supports:
- API-based configuration
- Declarative GitOps workflows
- Webhook integrations (future)
- Event-driven integrations

See [CI/CD Architecture](./architect/ci-cd-architecture.md) for details.

### What monitoring and observability tools are available?

- **Metrics**: Prometheus/Grafana dashboards
- **Logs**: Loki for log aggregation
- **Traces**: OpenTelemetry/Tempo for distributed tracing
- **Alerts**: Alertmanager for alerting

See [Monitoring and Observability](./system-admin/monitoring-observability.md) for details.

---

## Still Have Questions?

### Where can I get help?

1. **Documentation**: Review relevant persona guides
2. **Troubleshooting**: Check the [Troubleshooting Guide](./troubleshooting.md)
3. **Organization Administrator**: Contact your org admin
4. **Support**: Submit a support ticket

### How do I report a bug?

Report bugs through:
- Your organization administrator
- Support ticket system
- GitHub issues (if applicable)

Include:
- Description of the issue
- Steps to reproduce
- Error messages
- Relevant logs
- Environment details

### How do I request a feature?

Feature requests can be submitted through:
- Your organization administrator
- Support ticket system
- Product feedback channels

---

## Related Documentation

- [Getting Started](./getting-started.md) - First-time setup
- [Troubleshooting Guide](./troubleshooting.md) - Common issues
- [Glossary](./glossary.md) - Key terms
- [Persona Guides](./README.md) - Role-specific documentation


# Cache Salt Security

---
last_updated: 2025-12-08
document_type: guide
---

This document explains the `cache_salt` parameter - a security feature designed to prevent prompt leakage in LLM caching systems.

## 1. The Fundamental Problem: Statelessness and the Prefill Bottleneck

All major Large Language Models (LLMs) built on the Transformer architecture are **stateless** by default. This means the model retains no memory of past interactions; the only context that matters is the **current input**.

To enable conversational continuity (the illusion of "memory"), the external application layer must act as a repeater, sending the **entire conversation history** (the context window) as a **prefix** along with the new user prompt for every single turn.

This re-submission requires the LLM serving system to perform a costly **"prefill" computation** on the entire history to calculate the Key (K) and Value (V) tensors—collectively known as the **KV Cache**. This redundancy is computationally expensive and is the primary cost driver and latency source in multi-turn interactions.

## 2. The Solution Layer: Automatic Prefix Caching (APC)

To mitigate the expense of re-computation, modern inference engines like **vLLM** utilize **Automatic Prefix Caching (APC)**.

*   **KV Cache Persistence:** APC evolves the KV Cache from a single-request buffer into a **shared, inter-request state store**.

*   **Mechanism:** When a new request arrives, the engine identifies if the prefix of the new prompt matches any existing tokens already cached in GPU High Bandwidth Memory (HBM). If a match (a cache hit) is found, the engine retrieves the stored tensors, **skipping the expensive prefill computation** for the shared portion.

*   **Performance Impact:** This drastically reduces the **Time to First Token (TTFT)** and increases system throughput, especially in workloads like conversational AI and agentic workflows where the prompt prefix (history/instructions) is massive compared to the new input (suffix).

## 3. The Security Problem: Timing Attacks

While efficient, sharing the KV Cache across multiple users in a multi-tenant environment creates a severe security vulnerability known as a **Timing Attack**.

*   **The Threat:** An attacker (User B) can infer the contents of a victim's sensitive prompt (User A) by systematically measuring the **latency** (specifically TTFT) of their own probe requests.

*   **Inference:** If a guessed prompt returns with significantly **lower latency**, the attacker deduces that this specific sequence of tokens was already resident in the shared cache, thereby confirming that the victim had recently submitted that sensitive text.

*   **Mitigation Need:** To reconcile the trade-off between the performance benefits of shared caching and the need for strong privacy isolation, a security mechanism is required.

## 4. The Solution: Cache Salting via the `cache_salt` Parameter

**Cache Salting** is the mitigation strategy designed to enforce isolation while still utilizing prefix caching. It protects prompt data by ensuring that KV cache entries containing sensitive content are never reused across user boundaries.

### Mechanism and Injection

The `cache_salt` parameter is the vehicle used to implement this security partitioning:

1.  **Identity Injection:** The application layer must supply a unique value—the **salt**—derived from a user's ID, Tenant ID, or API Key.

2.  **Hashing:** This unique salt is **injected into the hash calculation** of the KV blocks. The hash for a KV block is calculated using the Token IDs, the Parent Block Hash (ensuring order sensitivity), and crucial **Extra Hashes** like the Cache Salt.

3.  **Isolation:** If two users submit identical prompts but provide different `cache_salt` values, the resulting KV block hashes will differ. This forces the system to treat the prompts as completely distinct, thus **preventing cache reuse** and negating the timing attack vector between the salted groups.

### API Implementation

In frameworks like the vLLM OpenAI-compatible API, the `cache_salt` is delivered by the client (Host) as a specific field in the request:

*   The parameter is typically passed in the **`extra_body`** field of the API call.

*   The salt itself is a string, ideally a base64-encoded 256-bit string, which is only used internally for block hashing and is **not passed to the model**.

### Isolation Levels and Trade-offs

The choice of the salt value determines the security-versus-efficiency trade-off.

| Isolation Level | Salt Value | Description | Implication |
| :--- | :--- | :--- | :--- |
| **Organization-Level Salt** | Derived from a shared Tenant ID or Organization ID. | Allows all users within a single trusted entity (e.g., employees of a company) to share the cached system prompts and common context. | **Optimal balance** for B2B SaaS, maximizing efficiency while isolating the organization from external tenants. |
| **User-Level Salt** | A unique ID for every individual user. | Provides **strict isolation**. User A cannot share cache even with User B from the same team, eliminating the timing attack completely for that user. | **Reduces efficiency**, as APC gains are negated for commonly shared preambles across users. |

### Advanced Salting (Multi-Barrier Design)

For maximum flexibility, a **Multi-barrier design** (Option B) has been proposed, which uses a `cache_salt_map`.

*   This approach allows an arbitrary number of salts to be assigned to specific **messages** within the conversation history (tracked by message index).

*   This enables **hierarchical cache reuse**: a long common document could be protected by an `org-salt` (allowing sharing across the group), while the individual user's specific query is protected by a separate `user-salt` (protecting the prompt even from others within the organization). This design allows for cache reuse and prompt protection simultaneously.

## 5. Architectural Context: State Management Tiers

The `cache_salt` parameter operates within the **Hot Layer** of a multi-tiered memory architecture, addressing real-time security risks associated with immediate, high-speed context handling:

| Architectural Layer | Memory Technology | Context Role | Security/State Management |
| :--- | :--- | :--- | :--- |
| **Hot Layer** (GPU HBM) | vLLM with **APC** | Holds active KV tensors for instant reuse (seconds to minutes). | **Cache Salting** isolates the cache by user/tenant, ensuring privacy boundary enforcement. |
| **Warm Layer** (Host DRAM/NVMe) | **LMCache** / Dynamo | Offloads evicted KV blocks from the GPU to cheaper, slower memory, extending session context. | Prefix-Aware Scheduling (e.g., llm-d Router) ensures follow-up requests are routed to the node holding the user's cached state. |
| **Cold Layer** (Database) | LangGraph **Checkpointers** / Redis / Vector DBs | Persists conversation history or facts indefinitely across sessions. | The application framework retrieves the history and injects it back into the prompt, fulfilling the LLM's stateless requirement. |

## 6. Implementation in AI-AAS Platform

In the AI-AAS platform, `cache_salt` is configured and validated through the load testing harness. The platform supports:

*   **Organization-level isolation**: Using `org_id` as the cache salt value, allowing cache sharing within an organization while maintaining isolation between organizations.

*   **User-level isolation**: Using `user_id` as the cache salt value, providing strict per-user isolation for maximum security.

*   **Custom salt values**: For specialized testing scenarios or advanced use cases.

*   **Isolation validation**: The load testing harness validates that cache hits only occur within the same salt boundary, detecting and reporting any cross-tenant cache leakage as security violations.

### Configuration Example

```yaml
cache_salt:
  enabled: true
  strategy: "org_id"  # Options: "org_id", "user_id", "custom"
  custom_salt_value: null  # Only used when strategy is "custom"
  validation:
    enabled: true  # Validate cache isolation
    report_violations: true  # Report security violations
```

The `cache_salt` is passed via the `extra_body` parameter in API requests to the vLLM backend:

```json
{
  "model": "llama-3-8b",
  "messages": [...],
  "extra_body": {
    "cache_salt": "org_abc123"
  }
}
```

## 7. Security Considerations

*   **Timing Attack Mitigation**: Cache salting prevents attackers from inferring sensitive prompt content through latency measurements.

*   **Multi-Tenant Isolation**: Ensures that cache entries from one tenant cannot be reused by another tenant, even if they submit identical prompts.

*   **Performance Trade-offs**: Organization-level salting provides a balance between security and performance, while user-level salting maximizes security at the cost of reduced cache efficiency.

*   **Validation**: The platform includes validation mechanisms to detect and report any cache salt isolation violations, ensuring the security mechanism is functioning correctly.

## Related Documentation

- [API Router vLLM Integration](../API_ROUTER_VLLM_INTEGRATION.md) - Platform integration details
- [vLLM Documentation](https://docs.vllm.ai/) - vLLM Automatic Prefix Caching


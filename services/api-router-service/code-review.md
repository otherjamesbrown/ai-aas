# Code Review: API Router Service

**Date:** 2025-11-23
**Service:** `api-router-service`
**Reviewer:** Antigravity AI Assistant

## Executive Summary

The `api-router-service` demonstrates a **sound architectural foundation** with a clear separation of concerns and a well-designed two-tier routing strategy. However, the review identified **critical issues** related to concurrency and security that require immediate remediation to ensure service stability and data protection.

## 1. Architecture & Design

### Strengths
*   **Two-Tier Routing**: The use of a main router (for health/metrics) and a sub-router (for application logic) is a robust pattern. It effectively handles `chi`'s middleware ordering constraints while keeping operational endpoints accessible.
*   **Separation of Concerns**: The codebase is well-structured with clear boundaries between:
    *   `internal/auth`: Authentication logic
    *   `internal/routing`: Backend selection and failover
    *   `internal/limiter`: Rate limiting
    *   `internal/api`: HTTP handlers
*   **Dependency Injection**: Components are loosely coupled and dependencies are injected via constructors (e.g., `NewHandler`, `NewAuthenticator`), facilitating testability.

### Weaknesses
*   **Main Function Complexity**: The `main` function in `cmd/router/main.go` is overly complex (~400 lines), handling configuration, dependency wiring, and server startup. This makes it hard to read and maintain.

## 2. Critical Issues (Must Fix)

### 🔴 Concurrency Bug: Race Condition in Authenticator
*   **Severity**: **CRITICAL**
*   **Location**: `internal/auth/authenticator.go`
*   **Description**: The `validationCache` map is accessed for both reading and writing in `validateAPIKey` without any synchronization mechanism (e.g., `sync.Mutex` or `sync.RWMutex`).
*   **Impact**: Under concurrent load, this **will cause the service to panic** and crash due to concurrent map writes.
*   **Recommendation**: Protect the map with a `sync.RWMutex`.

### 🔴 Security Vulnerability: Hardcoded Secret
*   **Severity**: **CRITICAL**
*   **Location**: `internal/auth/authenticator.go`
*   **Description**: The `verifyHMAC` function uses a hardcoded secret: `[]byte("stub-secret")`.
*   **Impact**: This allows anyone with knowledge of this string to forge valid HMAC signatures and bypass integrity checks.
*   **Recommendation**: Retrieve the actual secret from the `user-org-service` or secure configuration.

## 3. Code Quality & Maintainability

*   **Readability**: The code is generally well-written, idiomatic, and includes helpful comments explaining the "why" behind design decisions (e.g., the middleware ordering comments in `main.go`).
*   **Error Handling**: Error handling is consistent, using a centralized error builder and typed errors.
*   **Testing Gaps**:
    *   **Missing Unit Tests**: There are **no unit tests** for critical logic in `internal/auth` or `internal/routing`.
    *   **Reliance on Integration Tests**: While `test/integration` provides good end-to-end coverage, the lack of unit tests makes it difficult to verify edge cases and internal logic (like weighted routing distribution) in isolation.

## 4. Specific Observations

*   **`internal/routing/engine.go`**: The `recordDecision` function uses a simple slice append for a circular buffer. While functional, a ring buffer implementation might be more efficient for high-throughput scenarios.
*   **`internal/api/public/handler.go`**: The `fallbackRouting` method iterates through backends sequentially, ignoring the configured weights. This differs from the main routing logic and could lead to uneven load distribution during fallbacks.

## 5. Recommendations

1.  **Immediate Remediation**:
    *   Fix the race condition in `authenticator.go` by adding a mutex.
    *   Remove the hardcoded secret in `authenticator.go` and implement proper secret retrieval.

2.  **Short-Term Improvements**:
    *   Add unit tests for `Authenticator` and `RoutingEngine` to cover edge cases and concurrency.
    *   Refactor `cmd/router/main.go` to move dependency wiring into a dedicated `Bootstrap` or `Server` struct.

3.  **Long-Term Goals**:
    *   Implement a proper ring buffer for decision tracking.
    *   Align `fallbackRouting` logic with the weighted selection strategy.

# Shared TypeScript Unit Test Harness

This workspace drives unit tests for the shared TypeScript libraries.

- Depends on the local `@ai-aas/shared` package via `file:` reference.
- Use `npm test` (triggered by `make shared-ts-test`) to run Vitest suites.
- Extend `src/` with additional `.test.ts` files as features are implemented.


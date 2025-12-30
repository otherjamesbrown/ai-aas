# Investigation Report

**Bead**: aas-hjat
**Date**: 2025-12-30
**Investigator**: debugger agent
**Commit SHA**: ae08009d3f82244bb1a69dd6da371125323fbc6d

## Symptom

User invite endpoint returns HTTP 409 (Conflict) when the user with the same email already exists in the organization. The bead description suggests this needs "better handling."

## Reproduction

When calling `POST /v1/orgs/{orgId}/invites` with an email that already exists:
1. API returns HTTP 409 with message: "user with this email already exists"
2. CLI error message: "failed to invite user: invite user failed: status 409, body: ..."

## Evidence Gathered

| Source | Finding |
|--------|---------|
| `services/user-org-service/internal/httpapi/users/handlers.go:188-193` | InviteUser explicitly checks for existing email and returns 409 |
| `usecases/users.yaml:127-134` | UC-USR-002/AC-03 **requires** exit code 6 (409) for duplicate email |
| `specs/009-admin-cli/spec.md:61` | Spec explicitly states 409 behavior with --upsert for idempotency |
| `services/ai-aas-cli/internal/admin/user.go:605-616` | CLI already implements --upsert flag to handle duplicates |
| `services/ai-aas-cli/internal/admin/user.go:626-631` | CLI error handling doesn't suggest --upsert flag |

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| 409 is incorrect, should return 200 | ❌ Ruled out | UC-USR-002/AC-03 explicitly requires 409 |
| API should auto-upsert (idempotent) | ❌ Ruled out | Spec requires explicit failure, idempotency via CLI --upsert flag |
| --upsert flag is missing | ❌ Ruled out | Already implemented in user.go:605-616 |
| Error message is unclear | ✅ CONFIRMED | Generic "failed to invite user" doesn't mention --upsert |
| CLI doesn't parse 409 specially | ✅ CONFIRMED | All API errors handled the same way (line 628-631) |

## Root Cause

**Category**: `missing_context` + `logging_gap`

**Explanation**: The behavior is **working as designed** according to the use case specification. The 409 response is correct per UC-USR-002/AC-03. However, the issue is that:

1. **CLI error message is not actionable** - When the CLI receives a 409 from the invite endpoint, it shows a generic error message that doesn't inform the user about the `--upsert` flag
2. **The bead itself represents a context gap** - The person who created this bead didn't know that:
   - 409 is the required behavior per use case spec
   - The `--upsert` flag already exists to handle this scenario
   - This is the correct implementation of idempotent operations

**Evidence**:
- UC-USR-002/AC-03 states: "Command fails with exit code 6 (conflict)" and "Suggestion to use a different email"
- Spec 009 US-002 states: "operation is idempotent (re-running with `--upsert` succeeds)"
- Current error message (user.go:629): `"Verify your API key is valid and you have permission to invite users."` - doesn't suggest --upsert

## Context Gap Check

- [x] Was this caused by missing context? **YES**

**Context file**: N/A - This bead itself represents a context issue

**What was missing**:
- User who created bead didn't know 409 is the designed behavior
- User didn't know --upsert flag exists
- CLI error message doesn't guide users to the solution

**Suggested fix**:
1. Update CLI error handling to detect 409 and suggest --upsert flag
2. Add documentation/help text explaining --upsert usage
3. Consider renaming bead to reflect actual issue: "CLI doesn't suggest --upsert flag for duplicate users"

## Proposed Fix

**NOT a bug fix - this is an enhancement to error messaging.**

**Affected files**:
- `services/ai-aas-cli/internal/admin/user.go` - Error handling in runInviteUserCreate (lines 626-631)
- `services/ai-aas-cli/internal/client/userorg/client.go` - InviteUser error response parsing (lines 224-228)

**Approach**:

1. **Detect 409 specifically in CLI client**:
   - Parse 409 status code in `InviteUser()` method
   - Return a specific error type (e.g., `ErrUserAlreadyExists`)

2. **Improve error message in user command**:
   - Check for `ErrUserAlreadyExists` error type
   - Return actionable error: `"User with email {email} already exists. Use --upsert flag to skip creation if user exists."`

3. **Add help text to --upsert flag**:
   - Document that --upsert makes the operation idempotent
   - Explain it returns existing user instead of failing

**Example improved error message**:
```
Error: User with email user@example.com already exists in organization 'acme'

To return the existing user instead of failing, use:
  ai-aas-org user create --user-email user@example.com --upsert

Alternatively, use a different email address.
```

**Estimated complexity**: Low (error message improvement only, no logic changes)

## Prevention

How to prevent this class of issue in future:

| Type | Action |
|------|--------|
| Documentation | Add "Common Errors" section to CLI docs explaining --upsert |
| Context | Add anti-pattern: "Don't assume 409 is a bug - check use case spec first" |
| Testing | Add E2E test that verifies 409 error message suggests --upsert |
| CLI Help | Enhance --upsert flag description with example |

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| (to be created) | enhancement | cli-developer | Improve CLI error message for 409 to suggest --upsert |
| (to be created) | task | cli-developer | Add E2E test verifying --upsert suggestion in error |
| (to be created) | documentation | cli-developer | Document --upsert in CLI usage guide |

## Summary

**This is NOT a bug.** The 409 response is working exactly as specified in UC-USR-002/AC-03. The actual issue is that the CLI error message doesn't guide users to the existing `--upsert` flag solution. This is a user experience enhancement, not a functional fix.

The bead should be closed as "MISDIAGNOSED - behavior is correct per spec, needs error message enhancement" and replaced with a new enhancement bead for improving CLI error messaging.

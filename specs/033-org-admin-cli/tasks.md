# Tasks: Org Admin CLI (ai-aas-org)

**Spec Number:** 033
**Epic Bead:** aas-spec033
**Implementation Bead:** aas-2816
**Created:** 2025-12-28

---

## Task Summary

| Phase | Tasks | Priority |
|-------|-------|----------|
| Phase 1: Shared Components | 5 tasks | P1 (blocking) |
| Phase 2: Core CLI | 4 tasks | P1 |
| Phase 3: User & Key Management | 4 tasks | P2 |
| Phase 4: Reporting & Audit | 4 tasks | P2 |
| Phase 5: UX Polish | 4 tasks | P3 |
| Phase 6: Platform Integration | 3 tasks | P2 |
| **Total** | **24 tasks** | |

---

## Phase 1: Shared Components Extraction (P1 - Blocking)

These tasks must complete first as they unblock all other phases.

### T1.1: Create cli-shared module structure
**Bead:** `TBD`
**Priority:** P1
**Depends on:** None

- Create `services/cli-shared/` directory
- Initialize go.mod with module path
- Add required dependencies (tablewriter, color, viper)
- Create package structure: output/, errors/, config/, prompt/

**Acceptance:**
- [ ] `go mod tidy` succeeds
- [ ] Directory structure matches plan

---

### T1.2: Extract output package to cli-shared
**Bead:** `TBD`
**Priority:** P1
**Depends on:** T1.1

- Move table.go, json.go, color.go, duration.go, progress.go
- Remove model-specific formatters (keep in ai-aas-cli)
- Create generic format.go with FormatBool, FormatBytes, FormatDuration
- Add unit tests

**Acceptance:**
- [ ] All output functions work
- [ ] Tests pass

---

### T1.3: Extract errors package to cli-shared
**Bead:** `TBD`
**Priority:** P1
**Depends on:** T1.1

- Move errors.go with CLIError struct
- Move error factory functions
- Keep exit codes consistent
- Add unit tests

**Acceptance:**
- [ ] CLIError works correctly
- [ ] Exit codes match spec

---

### T1.4: Create prompt package in cli-shared
**Bead:** `TBD`
**Priority:** P1
**Depends on:** T1.1

- Implement Input() for string prompts
- Implement Password() for hidden input
- Implement Confirm() for yes/no
- Implement ConfirmDestructive() for typing confirmation
- Implement Select() for option selection
- Add unit tests with mock stdin

**Acceptance:**
- [ ] All prompt functions work
- [ ] Handles Ctrl+C gracefully

---

### T1.5: Update ai-aas-cli to use cli-shared
**Bead:** `TBD`
**Priority:** P1
**Depends on:** T1.2, T1.3, T1.4

- Add cli-shared to go.mod with replace directive
- Update all imports from internal/output to cli-shared/output
- Update all imports from internal/errors to cli-shared/errors
- Remove old internal/output and internal/errors packages
- Run full test suite
- Smoke test CLI commands

**Acceptance:**
- [ ] `make test` passes
- [ ] `make build` succeeds
- [ ] `ai-aas-cli model list` works
- [ ] No duplicate code

---

## Phase 2: Core CLI Structure (P1)

### T2.1: Create ai-aas-org project scaffold
**Bead:** `TBD`
**Priority:** P1
**Depends on:** T1.5

- Create `services/ai-aas-org/` directory structure
- Initialize go.mod with dependencies
- Create main.go entry point
- Create root.go with global flags (--json, --quiet, --yes, --guided, --verbose)
- Create Makefile with build, test, install targets

**Acceptance:**
- [ ] `go build` produces binary
- [ ] `ai-aas-org --help` shows usage

---

### T2.2: Implement config package for ai-aas-org
**Bead:** `TBD`
**Priority:** P1
**Depends on:** T2.1

- Create Config struct with APIEndpoint, APIKey, OrgID, OutputFormat
- Implement Load() using cli-shared/config base
- Implement Save() with proper file permissions (600)
- Implement config show command (mask API key)
- Implement config set command

**Acceptance:**
- [ ] Config loads from ~/.ai-aas-org.yaml
- [ ] Environment variables work (AI_AAS_ORG_ prefix)
- [ ] API key is masked in output

---

### T2.3: Implement init command
**Bead:** `TBD`
**Priority:** P1
**Depends on:** T2.2

- Display welcome message
- Prompt for API endpoint (with default)
- Prompt for bootstrap key (hidden input)
- Validate bootstrap key against API
- Prompt for admin name and email
- Redeem bootstrap key, get API key
- Save config file
- Display quick-start guide

**Acceptance:**
- [ ] Full init flow works
- [ ] Config file created with correct permissions
- [ ] Quick-start guide displays

---

### T2.4: Add version command and build metadata
**Bead:** `TBD`
**Priority:** P2
**Depends on:** T2.1

- Add version command
- Inject version, commit, build time via ldflags
- Display in `ai-aas-org version` and `ai-aas-org --version`

**Acceptance:**
- [ ] Version displays correctly
- [ ] Build metadata injected

---

## Phase 3: User & Key Management (P2)

### T3.1: Implement user commands
**Bead:** `TBD`
**Priority:** P2
**Depends on:** T2.3

- Implement `user list` with table/JSON output
- Implement `user create` with --name, --email, --role flags
- Implement `user show` with user details
- Implement `user update` with optional flags
- Implement `user delete` with confirmation prompt
- Add --guided mode to create/update

**Acceptance:**
- [ ] All user CRUD operations work
- [ ] Guided mode prompts correctly
- [ ] Confirmation required for delete

---

### T3.2: Implement user models commands
**Bead:** `TBD`
**Priority:** P2
**Depends on:** T3.1

- Implement `user models list` showing accessible models
- Implement `user models grant` to add access
- Implement `user models revoke` to remove access
- Use new API endpoint (or mock if not ready)

**Acceptance:**
- [ ] Model access changes immediately
- [ ] Cannot grant access to non-org models

---

### T3.3: Implement apikey commands
**Bead:** `TBD`
**Priority:** P2
**Depends on:** T2.3

- Implement `apikey list` with optional --user filter
- Implement `apikey create` with --user, --name, --expires flags
- Implement `apikey show` with key metadata (not secret)
- Implement `apikey rotate` with confirmation
- Implement `apikey delete` with confirmation
- Show created key once only with warning

**Acceptance:**
- [ ] All API key operations work
- [ ] Secret shown only on create
- [ ] Rotate creates new and revokes old

---

### T3.4: Add unit tests for user and apikey commands
**Bead:** `TBD`
**Priority:** P2
**Depends on:** T3.1, T3.2, T3.3

- Test flag parsing and validation
- Test output formatting
- Test confirmation prompts with mock stdin
- Test error handling

**Acceptance:**
- [ ] >80% coverage on command files
- [ ] All tests pass

---

## Phase 4: Reporting & Audit (P2)

### T4.1: Implement model commands
**Bead:** `TBD`
**Priority:** P2
**Depends on:** T2.3

- Implement `model list` showing org-available models
- Implement `model show` with model details and user access
- Reuse existing registry client with org filtering

**Acceptance:**
- [ ] Only org models displayed
- [ ] User access info shown in model show

---

### T4.2: Implement usage commands
**Bead:** `TBD`
**Priority:** P2
**Depends on:** T2.3

- Implement `usage summary` with org totals
- Implement `usage by-user` with per-user breakdown
- Implement `usage by-model` with per-model breakdown
- Implement `usage export` with CSV/JSON formats
- Implement period parsing (1d, 7d, 30d, this-month, last-month)

**Acceptance:**
- [ ] All usage queries work
- [ ] Export produces valid CSV/JSON
- [ ] Period filtering works

---

### T4.3: Implement audit commands
**Bead:** `TBD`
**Priority:** P2
**Depends on:** T2.3

- Implement `audit list` with --user, --action, --since filters
- Implement `audit export` with CSV/JSON formats
- Handle pagination for large result sets
- Use new API endpoint (or mock if not ready)

**Acceptance:**
- [ ] All audit event types displayed
- [ ] Filtering works correctly
- [ ] Export produces valid output

---

### T4.4: Implement org commands
**Bead:** `TBD`
**Priority:** P2
**Depends on:** T2.3

- Implement `org show` with org details, limits, user count
- Implement `org update` with --name flag

**Acceptance:**
- [ ] Org details display correctly
- [ ] Name update works

---

## Phase 5: UX Polish (P3)

### T5.1: Add guided mode to all applicable commands
**Bead:** `TBD`
**Priority:** P3
**Depends on:** T3.1, T3.3

- Add --guided flag handling to create/update commands
- Implement preview before execution
- Allow abort at any prompt

**Acceptance:**
- [ ] All create commands support --guided
- [ ] Preview shows before execution
- [ ] Ctrl+C aborts cleanly

---

### T5.2: Create help topics
**Bead:** `TBD`
**Priority:** P3
**Depends on:** T2.1

- Create internal/guides/ directory
- Write onboarding.md - Getting started guide
- Write users.md - User management walkthrough
- Write api-keys.md - API key best practices
- Write usage.md - Understanding usage reports
- Write troubleshooting.md - Common issues
- Implement `help [topic]` command with embedded files

**Acceptance:**
- [ ] All 5 help topics written
- [ ] `ai-aas-org help` lists topics
- [ ] `ai-aas-org help onboarding` displays content

---

### T5.3: Add shell completions
**Bead:** `TBD`
**Priority:** P3
**Depends on:** T4.4

- Add completion command using Cobra's built-in support
- Generate bash completion
- Generate zsh completion
- Generate fish completion
- Document installation in help topic

**Acceptance:**
- [ ] All three shells supported
- [ ] Completions work correctly

---

### T5.4: Create install script
**Bead:** `TBD`
**Priority:** P3
**Depends on:** T2.1

- Create scripts/install.sh
- Detect OS (linux, darwin)
- Detect architecture (amd64, arm64)
- Download correct binary from releases
- Install to /usr/local/bin or ~/.local/bin
- Display next steps

**Acceptance:**
- [ ] Works on Linux amd64
- [ ] Works on macOS arm64
- [ ] Handles permission errors gracefully

---

## Phase 6: Platform Admin Integration (P2)

### T6.1: Add bootstrap-key commands to ai-aas-cli
**Bead:** `TBD`
**Priority:** P2
**Depends on:** T1.5

- Implement `org bootstrap-key create` with --org, --expires, --admin-email
- Implement `org bootstrap-key list` with --org filter
- Implement `org bootstrap-key revoke`
- Add to existing org command group

**Acceptance:**
- [ ] All bootstrap-key commands work
- [ ] Key displayed once on create
- [ ] Expiry defaults to 7 days

---

### T6.2: Implement bootstrap key API endpoints
**Bead:** `TBD`
**Priority:** P2
**Depends on:** None (backend work)

- POST /v1/org/bootstrap-keys - Create key
- GET /v1/org/bootstrap-keys - List keys for org
- DELETE /v1/org/bootstrap-keys/{id} - Revoke key
- POST /v1/org/bootstrap - Redeem key
- Add database table for bootstrap keys
- Implement key hashing and validation

**Acceptance:**
- [ ] All endpoints work
- [ ] Keys are single-use
- [ ] Expired keys rejected

---

### T6.3: E2E testing of full onboarding flow
**Bead:** `TBD`
**Priority:** P2
**Depends on:** T6.1, T6.2, T2.3

- Test: Platform admin creates org
- Test: Platform admin generates bootstrap key
- Test: Org admin runs install script
- Test: Org admin runs init with bootstrap key
- Test: Org admin creates first user
- Document the full flow

**Acceptance:**
- [ ] Full flow works end-to-end
- [ ] Documentation matches reality

---

## Dependency Graph

```
T1.1 ──┬── T1.2 ──┐
       ├── T1.3 ──┼── T1.5 ── T2.1 ── T2.2 ── T2.3 ──┬── T3.1 ── T3.2
       └── T1.4 ──┘                     │            ├── T3.3
                                        │            ├── T4.1
                                        │            ├── T4.2
                                        │            ├── T4.3
                                        │            └── T4.4
                                        │
                                        ├── T2.4
                                        ├── T5.2
                                        └── T5.4

T3.1 ── T3.4
T3.3 ──┘

T3.1 ── T5.1
T3.3 ──┘

T4.4 ── T5.3

T1.5 ── T6.1 ──┐
               ├── T6.3
T6.2 ──────────┘
T2.3 ──────────┘
```

---

## Task Beads

| Task ID | Bead ID | Title | Priority | Status |
|---------|---------|-------|----------|--------|
| T1.1 | aas-efdn | Create cli-shared module structure | P1 | open |
| T1.2 | aas-3xgv | Extract output package to cli-shared | P1 | open |
| T1.3 | aas-vspv | Extract errors package to cli-shared | P1 | open |
| T1.4 | aas-paqa | Create prompt package in cli-shared | P1 | open |
| T1.5 | aas-crai | Update ai-aas-cli to use cli-shared | P1 | open |
| T2.1 | aas-kq9r | Create ai-aas-org project scaffold | P1 | open |
| T2.2 | aas-7v4l | Implement config package for ai-aas-org | P1 | open |
| T2.3 | aas-aqwn | Implement init command | P1 | open |
| T2.4 | aas-yg9x | Add version command and build metadata | P2 | open |
| T3.1 | aas-yvzb | Implement user commands | P2 | open |
| T3.2 | aas-6hnc | Implement user models commands | P2 | open |
| T3.3 | aas-29fi | Implement apikey commands | P2 | open |
| T3.4 | aas-8v31 | Add unit tests for user and apikey commands | P2 | open |
| T4.1 | aas-bf98 | Implement model commands | P2 | open |
| T4.2 | aas-dpcc | Implement usage commands | P2 | open |
| T4.3 | aas-7ecu | Implement audit commands | P2 | open |
| T4.4 | aas-4jlv | Implement org commands | P2 | open |
| T5.1 | aas-u8xd | Add guided mode to all applicable commands | P3 | open |
| T5.2 | aas-35jr | Create help topics | P3 | open |
| T5.3 | aas-eb48 | Add shell completions | P3 | open |
| T5.4 | aas-q9b6 | Create install script | P3 | open |
| T6.1 | aas-ty9c | Add bootstrap-key commands to ai-aas-cli | P2 | open |
| T6.2 | aas-mmjd | Implement bootstrap key API endpoints | P2 | open |
| T6.3 | aas-tjel | E2E testing of full onboarding flow | P2 | open |

---

## Critical Path

The minimum tasks required to have a working MVP:

1. **T1.1** → **T1.2** → **T1.3** → **T1.4** → **T1.5** (shared components)
2. **T2.1** → **T2.2** → **T2.3** (core CLI with init)
3. **T3.1** → **T3.3** (user and API key management)
4. **T6.1** → **T6.2** → **T6.3** (bootstrap key flow)

MVP = 12 tasks on critical path

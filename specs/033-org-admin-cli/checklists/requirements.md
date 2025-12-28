# Requirements Checklist - Spec 033: Org Admin CLI

## Functional Requirements

### CLI Structure (FR-1)
- [ ] `init` command implemented
- [ ] `user` command group (list, create, show, update, delete)
- [ ] `user models` subcommand group (list, grant, revoke)
- [ ] `apikey` command group (list, create, show, rotate, delete)
- [ ] `model` command group (list, show)
- [ ] `usage` command group (summary, by-user, by-model, export)
- [ ] `audit` command group (list, export)
- [ ] `org` command group (show, update)
- [ ] `config` command group (show, set)
- [ ] `help` command with topic support

### Global Flags (FR-2)
- [ ] `--json` flag on all list/show commands
- [ ] `--quiet` flag on all commands
- [ ] `--yes` flag on destructive commands
- [ ] `--guided` flag on create/update commands
- [ ] `--help` with examples on all commands

### Init Command (FR-3)
- [ ] Prompts for API endpoint
- [ ] Prompts for bootstrap key
- [ ] Validates bootstrap key against API
- [ ] Prompts for admin name and email
- [ ] Creates admin user account
- [ ] Saves API key to ~/.ai-aas-org.yaml
- [ ] Displays quick-start guide
- [ ] Bootstrap key format: `org_boot_<32-char>`

### User Commands (FR-4)
- [ ] `user list` with table output
- [ ] `user create` with --name and --email required
- [ ] `user create` with optional --role (admin|member)
- [ ] `user show` displays user details
- [ ] `user update` allows changing name, email, role
- [ ] `user delete` requires confirmation
- [ ] Default role is `member`

### User Model Access (FR-5)
- [ ] `user models list` shows accessible models
- [ ] `user models grant` adds model access
- [ ] `user models revoke` removes model access
- [ ] New users get all org models by default
- [ ] Cannot grant access to models not in org

### API Key Commands (FR-6)
- [ ] `apikey list` with optional --user filter
- [ ] `apikey create` requires --user
- [ ] `apikey create` optional --name and --expires
- [ ] `apikey show` displays key metadata (not secret)
- [ ] `apikey rotate` creates new, revokes old
- [ ] `apikey delete` requires confirmation
- [ ] Created keys displayed once only

### Model Commands (FR-7)
- [ ] `model list` shows org-available models
- [ ] `model show` includes user access info
- [ ] Read-only (no deployment ops)

### Usage Commands (FR-8)
- [ ] `usage summary` shows org totals
- [ ] `usage by-user` shows per-user breakdown
- [ ] `usage by-model` shows per-model breakdown
- [ ] `usage export` supports csv and json
- [ ] Period flags: 1d, 7d, 30d, 90d, this-month, last-month
- [ ] Default period is current month

### Audit Commands (FR-9)
- [ ] `audit list` shows recent events
- [ ] `audit list --user` filters by user
- [ ] `audit list --action` filters by action type
- [ ] `audit list --since` filters by time
- [ ] `audit export` supports csv and json
- [ ] Captures: user CRUD, key lifecycle, model access, org updates, auth

### Org Commands (FR-10)
- [ ] `org show` displays org details and limits
- [ ] `org update --name` changes display name

### Config Commands (FR-11)
- [ ] `config show` displays current config
- [ ] `config set` updates config values
- [ ] Supports: api_endpoint, output_format

### Help System (FR-12)
- [ ] `help onboarding` topic
- [ ] `help users` topic
- [ ] `help api-keys` topic
- [ ] `help usage` topic
- [ ] `help troubleshooting` topic

---

## Non-Functional Requirements

### Security (NFR-1)
- [ ] Config file permissions 600
- [ ] API keys never logged after creation
- [ ] Bootstrap keys single-use
- [ ] Bootstrap keys time-limited (7 days default)
- [ ] HTTPS for all API calls
- [ ] Org-scoping enforced server-side

### Performance (NFR-2)
- [ ] CLI startup < 200ms
- [ ] API timeout 30s (configurable)
- [ ] Pagination for > 100 items

### Usability (NFR-3)
- [ ] 2-3 examples per command in --help
- [ ] Error messages include suggestions
- [ ] Confirmation prompts on destructive ops
- [ ] Progress indicators for long ops
- [ ] Color output (green/red/yellow)
- [ ] Respects NO_COLOR env var

### Compatibility (NFR-4)
- [ ] Linux amd64 binary
- [ ] Linux arm64 binary
- [ ] macOS amd64 binary
- [ ] macOS arm64 binary
- [ ] Bash completion script
- [ ] Zsh completion script
- [ ] Fish completion script

---

## User Scenarios

- [ ] US-1: Platform admin generates bootstrap key
- [ ] US-2: Org admin first-time setup
- [ ] US-3: Org admin creates team member
- [ ] US-4: Org admin uses guided mode
- [ ] US-5: Org admin restricts user model access
- [ ] US-6: Org admin views usage reports
- [ ] US-7: Org admin reviews audit log
- [ ] US-8: Org admin deletes user with confirmation

---

## Shared Components

- [ ] Extract `output` package to cli-shared
- [ ] Extract `errors` package to cli-shared
- [ ] Extract `config` base to cli-shared
- [ ] Create `prompt` package in cli-shared
- [ ] Create `help` package in cli-shared
- [ ] Update ai-aas-cli to use cli-shared
- [ ] Verify no regressions in ai-aas-cli

---

## Platform Admin CLI Additions

- [ ] `org bootstrap-key create` command
- [ ] `org bootstrap-key list` command
- [ ] `org bootstrap-key revoke` command

---

## API Endpoints Required

- [ ] POST `/v1/org/bootstrap` - Redeem bootstrap key
- [ ] GET/POST/DELETE `/v1/org/bootstrap-keys` - Manage keys
- [ ] GET `/v1/org/{id}/audit` - Fetch audit events
- [ ] GET/PUT `/v1/users/{id}/model-access` - Per-user permissions

---

## Validation

### Unit Tests
- [ ] Command parsing tests
- [ ] Config file tests
- [ ] Error formatting tests
- [ ] Output formatting tests

### Integration Tests
- [ ] Init flow with mock API
- [ ] User CRUD operations
- [ ] API key lifecycle
- [ ] Model access operations
- [ ] Usage data retrieval

### E2E Tests
- [ ] Full onboarding flow
- [ ] User creation and key issuance
- [ ] Guided mode walkthrough
- [ ] Destructive operation confirmations

### Manual Validation
- [ ] Install script on Linux amd64
- [ ] Install script on macOS arm64
- [ ] Help text readability
- [ ] Error message clarity
- [ ] Shell completion (bash, zsh)

---

## Documentation

- [ ] README.md for ai-aas-org
- [ ] Help topic: onboarding.md
- [ ] Help topic: users.md
- [ ] Help topic: api-keys.md
- [ ] Help topic: usage.md
- [ ] Help topic: troubleshooting.md
- [ ] Install script documentation

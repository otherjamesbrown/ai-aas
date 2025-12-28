# Spec: Org Admin CLI (ai-aas-org)

**Spec Number:** 033
**Epic Bead:** aas-spec033
**Status:** Draft
**Created:** 2025-12-28

## Overview

A simplified, user-friendly CLI for organization administrators to manage their org's users, API keys, model access, and usage without access to platform-level operations. This CLI (`ai-aas-org`) is separate from the platform admin CLI (`ai-aas-cli`) but shares common components.

### Goals

1. **Simplified Experience** - Only expose commands relevant to org management
2. **Smooth Onboarding** - Bootstrap key flow for easy first-time setup
3. **Excellent UX** - Guided modes, helpful errors, comprehensive examples
4. **Shared Architecture** - Extract common code to `cli-shared` package

### Non-Goals

- Model deployment/caching (platform admin only)
- Org creation/deletion (platform admin only)
- Infrastructure credentials (platform admin only)
- Cross-org access
- Budget enforcement (future phase)

---

## User Scenarios

### US-1: Platform Admin Onboards New Org Admin

**Actor:** Platform Administrator
**Precondition:** Organization exists in the system

1. Platform admin runs `ai-aas-cli org bootstrap-key create --org acme-corp --admin-email jane@acme.com`
2. System generates a time-limited bootstrap key (expires in 7 days)
3. Platform admin emails Jane: install script URL + bootstrap key
4. Jane can now initialize her org admin account

**Acceptance Criteria:**
- Bootstrap key is single-use
- Key expires after 7 days (configurable)
- Email is recorded for audit trail

### US-2: Org Admin First-Time Setup

**Actor:** Organization Administrator (new)
**Precondition:** Has received bootstrap key from platform admin

1. Org admin runs install script: `curl -fsSL https://api.example.com/install-org-cli.sh | bash`
2. CLI binary is downloaded and installed
3. Org admin runs `ai-aas-org init`
4. CLI prompts for: API endpoint, bootstrap key, name, email
5. System validates bootstrap key, creates admin account
6. CLI saves API key to `~/.ai-aas-org.yaml`
7. CLI displays quick-start guide

**Acceptance Criteria:**
- Install script detects OS/arch automatically
- Init validates bootstrap key before prompting for details
- Config file has secure permissions (600)
- Quick-start shows most common next commands

### US-3: Org Admin Creates Team Member

**Actor:** Organization Administrator
**Precondition:** Logged in with valid API key

1. Org admin runs `ai-aas-org user create --name "Bob Smith" --email bob@acme.com`
2. System creates user with access to all org models (default)
3. CLI displays user ID and confirmation
4. Org admin runs `ai-aas-org apikey create --user usr_xxx --name "Bob's key"`
5. System generates API key for Bob
6. CLI displays API key (shown once)

**Acceptance Criteria:**
- User is created in org admin's organization only
- New users get access to all org models by default
- API key is displayed only once with warning

### US-4: Org Admin Uses Guided Mode

**Actor:** Organization Administrator
**Precondition:** Logged in with valid API key

1. Org admin runs `ai-aas-org user create --guided`
2. CLI prompts: "User's full name:"
3. CLI prompts: "User's email:"
4. CLI prompts: "Grant access to all models? [Y/n]:"
5. CLI shows preview: "Will create user Bob Smith (bob@acme.com) with access to 5 models"
6. CLI prompts: "Proceed? [Y/n]:"
7. User is created

**Acceptance Criteria:**
- All create/update commands support `--guided`
- Guided mode shows preview before execution
- Can abort at any prompt with Ctrl+C

### US-5: Org Admin Restricts User Model Access

**Actor:** Organization Administrator
**Precondition:** User exists in org

1. Org admin runs `ai-aas-org user models list usr_xxx`
2. CLI displays models user can access
3. Org admin runs `ai-aas-org user models revoke usr_xxx gpt-4`
4. System removes user's access to gpt-4
5. Audit log records the change

**Acceptance Criteria:**
- Changes are immediate
- User cannot access revoked models
- Audit log includes who made the change

### US-6: Org Admin Views Usage Reports

**Actor:** Organization Administrator
**Precondition:** Org has usage data

1. Org admin runs `ai-aas-org usage summary`
2. CLI displays: total tokens, total requests, period
3. Org admin runs `ai-aas-org usage by-user --period 7d`
4. CLI displays per-user breakdown in table format
5. Org admin runs `ai-aas-org usage export --format csv > usage.csv`
6. CLI exports data to CSV

**Acceptance Criteria:**
- Default period is current month
- Supports day, week, month periods
- Export includes all fields

### US-7: Org Admin Reviews Audit Log

**Actor:** Organization Administrator
**Precondition:** Audit events exist

1. Org admin runs `ai-aas-org audit list --since 7d`
2. CLI displays recent audit events
3. Org admin runs `ai-aas-org audit list --user usr_xxx --action user.created`
4. CLI filters by user and action type

**Acceptance Criteria:**
- Audit events include: user CRUD, API key lifecycle, model access changes, login/init
- Can filter by user, action, time range
- Export to CSV/JSON supported

### US-8: Org Admin Deletes User (Confirmation Required)

**Actor:** Organization Administrator
**Precondition:** User exists in org

1. Org admin runs `ai-aas-org user delete usr_xxx`
2. CLI displays: "Delete user 'Bob Smith' (bob@acme.com)? This will also revoke 2 API keys."
3. CLI prompts: "Type 'delete' to confirm:"
4. Org admin types "delete"
5. User and associated keys are deleted

**Acceptance Criteria:**
- Destructive operations require typing confirmation word
- Shows what will be affected (keys, access, etc.)
- Can bypass with `--yes` for scripting

---

## Functional Requirements

### FR-1: CLI Structure

| Command Group | Commands | Description |
|---------------|----------|-------------|
| `init` | - | First-time setup with bootstrap key |
| `user` | list, create, show, update, delete | User management |
| `user models` | list, grant, revoke | Per-user model access |
| `apikey` | list, create, show, rotate, delete | API key management |
| `model` | list, show | View available models |
| `usage` | summary, by-user, by-model, export | Usage reporting |
| `audit` | list, export | Audit log access |
| `org` | show, update | Organization details |
| `config` | show, set | CLI configuration |
| `help` | [topic] | Topic-based help guides |

### FR-2: Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |
| `--quiet` | Minimal output (IDs only) |
| `--yes` | Skip confirmation prompts |
| `--guided` | Interactive mode with prompts |
| `--help` | Show help with examples |

### FR-3: Init Command

```
ai-aas-org init [--endpoint URL] [--key KEY]
```

- Prompts for API endpoint if not provided
- Prompts for bootstrap key if not provided
- Validates bootstrap key against API
- Prompts for admin name and email
- Creates admin user account
- Saves permanent API key to config file
- Displays quick-start guide

**Bootstrap Key Format:** `org_boot_<32-char-random>`

### FR-4: User Commands

```
ai-aas-org user list [--json]
ai-aas-org user create --name NAME --email EMAIL [--role admin|member] [--guided]
ai-aas-org user show USER_ID [--json]
ai-aas-org user update USER_ID [--name] [--email] [--role]
ai-aas-org user delete USER_ID [--yes]
```

**Roles:**
- `admin` - Can manage users and keys
- `member` - Can only use API (default for new users created by org admin)

### FR-5: User Model Access Commands

```
ai-aas-org user models list USER_ID
ai-aas-org user models grant USER_ID MODEL_NAME
ai-aas-org user models revoke USER_ID MODEL_NAME
```

- New users get access to all org models by default
- Revoked access is immediate
- Cannot grant access to models not available to org

### FR-6: API Key Commands

```
ai-aas-org apikey list [--user USER_ID] [--json]
ai-aas-org apikey create --user USER_ID [--name NAME] [--expires DURATION]
ai-aas-org apikey show KEY_ID [--json]
ai-aas-org apikey rotate KEY_ID [--yes]
ai-aas-org apikey delete KEY_ID [--yes]
```

- Created keys are shown once only
- Rotate creates new key and revokes old
- List shows: ID, name, user, created, last used, status

### FR-7: Model Commands

```
ai-aas-org model list [--json]
ai-aas-org model show MODEL_NAME [--json]
```

- Lists models available to organization
- Show includes: model details, which users have access
- Read-only (no deployment operations)

### FR-8: Usage Commands

```
ai-aas-org usage summary [--period PERIOD]
ai-aas-org usage by-user [--period PERIOD] [--json]
ai-aas-org usage by-model [--period PERIOD] [--json]
ai-aas-org usage export [--period PERIOD] [--format csv|json]
```

**Periods:** `1d`, `7d`, `30d`, `90d`, `this-month`, `last-month`

**Metrics:**
- Total tokens (input + output)
- Total requests
- Per-user breakdown
- Per-model breakdown

### FR-9: Audit Commands

```
ai-aas-org audit list [--user USER_ID] [--action ACTION] [--since DURATION] [--json]
ai-aas-org audit export [--since DURATION] [--format csv|json]
```

**Audit Events:**
- `user.created`, `user.updated`, `user.deleted`
- `apikey.created`, `apikey.rotated`, `apikey.deleted`
- `model_access.granted`, `model_access.revoked`
- `org.updated`
- `auth.init`, `auth.login`

### FR-10: Org Commands

```
ai-aas-org org show [--json]
ai-aas-org org update [--name NAME]
```

- Show displays: org name, ID, created date, limits, user count
- Update allows changing org display name only

### FR-11: Config Commands

```
ai-aas-org config show
ai-aas-org config set KEY VALUE
```

**Config Keys:**
- `api_endpoint` - API URL
- `output_format` - default output format (table|json)

### FR-12: Help System

```
ai-aas-org help [TOPIC]
```

**Topics:**
- `onboarding` - Getting started guide
- `users` - User management walkthrough
- `api-keys` - API key best practices
- `usage` - Understanding usage reports
- `troubleshooting` - Common issues and solutions

---

## Non-Functional Requirements

### NFR-1: Security

- Config file permissions: 600 (owner read/write only)
- API keys never logged or displayed after creation
- Bootstrap keys are single-use and time-limited
- All API calls use HTTPS
- Org-scoping enforced server-side (CLI cannot access other orgs)

### NFR-2: Performance

- CLI startup time: < 200ms
- API call timeout: 30 seconds (configurable)
- Pagination for large result sets (> 100 items)

### NFR-3: Usability

- Every command has 2-3 examples in `--help`
- Error messages include recovery suggestions
- Confirmation prompts for destructive operations
- Progress indicators for long operations
- Colors for status (green=success, red=error, yellow=warning)
- Respects `NO_COLOR` environment variable

### NFR-4: Compatibility

- Supported platforms: Linux (amd64, arm64), macOS (amd64, arm64)
- Shell completion: bash, zsh, fish
- Config location: `~/.ai-aas-org.yaml`

---

## Technical Design

### Shared Components (cli-shared)

Extract from `ai-aas-cli/internal/` to `services/cli-shared/`:

| Package | Contents |
|---------|----------|
| `output` | Table, JSON, CSV formatting; colors; progress bars |
| `errors` | CLIError type, exit codes, suggestions |
| `config` | Base config loading, file handling, env vars |
| `prompt` | Interactive prompts, confirmations, guided mode |
| `help` | Help text formatting, topic-based guides |

### ai-aas-org Structure

```
services/ai-aas-org/
├── cmd/
│   └── ai-aas-org/
│       ├── main.go
│       └── root.go
├── internal/
│   ├── cmd/           # Command implementations
│   │   ├── init.go
│   │   ├── user.go
│   │   ├── apikey.go
│   │   ├── model.go
│   │   ├── usage.go
│   │   ├── audit.go
│   │   ├── org.go
│   │   └── config.go
│   └── guides/        # Help topic content
│       ├── onboarding.md
│       ├── users.md
│       └── ...
├── scripts/
│   └── install.sh     # Install script
├── go.mod
├── go.sum
└── Makefile
```

### API Dependencies

Uses existing clients from `ai-aas-cli`:
- `internal/client/userorg` - User/org operations
- `internal/client/analytics` - Usage data
- `internal/api` - Base HTTP client

**New API Endpoints Required:**

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/org/bootstrap` | POST | Redeem bootstrap key, create admin |
| `/v1/org/bootstrap-keys` | GET/POST/DELETE | Manage bootstrap keys (platform admin) |
| `/v1/org/{id}/audit` | GET | Fetch audit events |
| `/v1/users/{id}/model-access` | GET/PUT | Per-user model permissions |

### Platform Admin CLI Additions

Add to `ai-aas-cli`:

```
ai-aas-cli org bootstrap-key create --org ORG_ID [--expires 7d] [--admin-email EMAIL]
ai-aas-cli org bootstrap-key list --org ORG_ID
ai-aas-cli org bootstrap-key revoke KEY_ID
```

---

## Validation

### V-1: Unit Tests

- [ ] All command parsing and flag validation
- [ ] Config file loading and saving
- [ ] Error message formatting
- [ ] Output formatting (table, JSON, CSV)

### V-2: Integration Tests

- [ ] Init flow with mock API
- [ ] User CRUD operations
- [ ] API key lifecycle
- [ ] Model access grant/revoke
- [ ] Usage data retrieval

### V-3: E2E Tests

- [ ] Full onboarding flow (bootstrap key → init → first command)
- [ ] User creation and key issuance
- [ ] Guided mode walkthrough
- [ ] Destructive operation confirmations

### V-4: Manual Validation

- [ ] Install script on Linux amd64
- [ ] Install script on macOS arm64
- [ ] Help text readability and examples
- [ ] Error message clarity
- [ ] Shell completion (bash, zsh)

---

## Dependencies

### Requires Before Starting

- User-org service with org-scoped endpoints
- Analytics service with usage aggregation
- Bootstrap key API endpoints

### Blocks

- None (standalone deliverable)

---

## Milestones

### M1: Shared Components Extraction

- Extract `output`, `errors`, `config`, `prompt` to `cli-shared`
- Update `ai-aas-cli` to use shared packages
- Verify no regressions

### M2: Core CLI Structure

- New `ai-aas-org` project skeleton
- Root command with global flags
- Config and init commands

### M3: User & Key Management

- User CRUD commands
- API key commands
- Model access commands

### M4: Reporting & Audit

- Usage commands
- Audit commands
- Export functionality

### M5: UX Polish

- Guided mode for all applicable commands
- Help topics
- Shell completions
- Install script

### M6: Platform Admin Integration

- Bootstrap key commands in `ai-aas-cli`
- API endpoint implementation
- E2E testing

---

## Open Questions

1. **API Endpoints** - Need to design bootstrap key redemption flow with backend team
2. **Audit Storage** - Where are audit events stored? Existing analytics service or new?

---

## References

- Idea document: `specs/033-org-admin-cli/idea.md`
- Existing CLI: `services/ai-aas-cli/`
- User-org service: `services/user-org-service/`

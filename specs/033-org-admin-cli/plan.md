# Implementation Plan: Org Admin CLI (ai-aas-org)

**Spec Number:** 033
**Epic Bead:** aas-spec033
**Created:** 2025-12-28

---

## Executive Summary

This plan describes the implementation of `ai-aas-org`, a simplified CLI for organization administrators. The implementation is structured in 6 phases, with Phase 1 (shared components extraction) being a prerequisite for both CLIs going forward.

**Total Estimated Components:** 15 packages
**New API Endpoints Required:** 4
**Shared Code to Extract:** ~500 LOC

---

## Architecture Overview

```
services/
├── cli-shared/                    # NEW: Shared CLI components
│   ├── output/                    # Table, JSON, CSV, colors
│   ├── errors/                    # CLIError, exit codes
│   ├── config/                    # Base config loading
│   ├── prompt/                    # Interactive prompts
│   └── version/                   # Version info injection
│
├── ai-aas-cli/                    # EXISTING: Platform admin CLI
│   ├── internal/
│   │   ├── config/               # MODIFY: Use cli-shared/config base
│   │   ├── output/               # REMOVE: Use cli-shared/output
│   │   ├── errors/               # REMOVE: Use cli-shared/errors
│   │   └── ...                   # Keep platform-specific code
│   └── cmd/
│
└── ai-aas-org/                    # NEW: Org admin CLI
    ├── cmd/
    │   └── ai-aas-org/
    │       └── main.go
    ├── internal/
    │   ├── cmd/                  # Command implementations
    │   ├── config/               # Org-CLI specific config
    │   └── guides/               # Help topic content
    ├── scripts/
    │   └── install.sh
    └── Makefile
```

---

## Phase 1: Shared Components Extraction

**Goal:** Extract reusable packages from ai-aas-cli into cli-shared

### 1.1 Create cli-shared Module

```bash
mkdir -p services/cli-shared
cd services/cli-shared
go mod init github.com/otherjamesbrown/ai-aas/services/cli-shared
```

**go.mod dependencies:**
- `github.com/olekukonko/tablewriter`
- `github.com/fatih/color`
- `github.com/spf13/viper`

### 1.2 Extract output Package

**Source:** `services/ai-aas-cli/internal/output/`
**Target:** `services/cli-shared/output/`

**Files to move:**
| File | Changes |
|------|---------|
| `table.go` | Remove model-specific formatters (keep generic TableWriter) |
| `json.go` | Move as-is |
| `color.go` | Move as-is |
| `duration.go` | Move as-is |
| `progress.go` | Move as-is |

**New file:** `output/format.go`
```go
// Generic formatters
func FormatBool(b bool) string
func FormatBytes(bytes int64) string
func FormatDuration(d time.Duration) string
```

### 1.3 Extract errors Package

**Source:** `services/ai-aas-cli/internal/errors/`
**Target:** `services/cli-shared/errors/`

**Move as-is:** `errors.go` (110 LOC)
- `CLIError` struct
- Error factory functions
- Exit codes

### 1.4 Extract config Base Package

**Source:** `services/ai-aas-cli/internal/config/`
**Target:** `services/cli-shared/config/`

**Extract base functionality:**
```go
// services/cli-shared/config/loader.go
package config

// Loader provides base config loading functionality
type Loader struct {
    ConfigName string   // e.g., ".ai-aas-org"
    EnvPrefix  string   // e.g., "AI_AAS_ORG"
    Paths      []string
}

func (l *Loader) Load(cfg interface{}) error
func (l *Loader) Save(cfg interface{}) error
func (l *Loader) ConfigPath() string
```

**Keep in ai-aas-cli:** CLI-specific `Config` struct with all fields

### 1.5 Create prompt Package

**Target:** `services/cli-shared/prompt/`

**New implementation:**
```go
// services/cli-shared/prompt/prompt.go
package prompt

// Input prompts for a string input
func Input(label string, defaultVal string) (string, error)

// Password prompts for hidden input
func Password(label string) (string, error)

// Confirm prompts for yes/no confirmation
func Confirm(label string, defaultYes bool) (bool, error)

// ConfirmDestructive requires typing a specific word
func ConfirmDestructive(label, confirmWord string) (bool, error)

// Select prompts user to select from options
func Select(label string, options []string) (int, error)
```

### 1.6 Update ai-aas-cli Imports

**Files to update:**
- All files importing `internal/output` → `cli-shared/output`
- All files importing `internal/errors` → `cli-shared/errors`
- Config files to use base loader

**go.mod addition:**
```go
require github.com/otherjamesbrown/ai-aas/services/cli-shared v0.0.0
replace github.com/otherjamesbrown/ai-aas/services/cli-shared => ../cli-shared
```

### 1.7 Verification

- [ ] `make test` passes in ai-aas-cli
- [ ] `make build` produces working binary
- [ ] Smoke test: `ai-aas-cli model list` works
- [ ] No duplicate code between cli-shared and ai-aas-cli

---

## Phase 2: Core CLI Structure

**Goal:** Create ai-aas-org project skeleton with config and init

### 2.1 Project Scaffold

```
services/ai-aas-org/
├── cmd/
│   └── ai-aas-org/
│       ├── main.go           # Entry point
│       └── root.go           # Root command + global flags
├── internal/
│   ├── cmd/
│   │   ├── init.go           # init command
│   │   └── config.go         # config show/set
│   └── config/
│       └── config.go         # Org-CLI specific config struct
├── go.mod
├── go.sum
└── Makefile
```

### 2.2 Config Structure

```go
// services/ai-aas-org/internal/config/config.go
package config

type Config struct {
    APIEndpoint  string `mapstructure:"api_endpoint"`
    APIKey       string `mapstructure:"api_key"`
    OrgID        string `mapstructure:"org_id"`
    OutputFormat string `mapstructure:"output_format"`
    Verbose      bool   `mapstructure:"verbose"`
}

const (
    ConfigFileName = ".ai-aas-org"
    EnvPrefix      = "AI_AAS_ORG"
)
```

### 2.3 Root Command

```go
// Global flags
var rootCmd = &cobra.Command{
    Use:   "ai-aas-org",
    Short: "AI-AAS Organization Administration CLI",
}

func init() {
    rootCmd.PersistentFlags().BoolP("json", "j", false, "Output in JSON format")
    rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Minimal output")
    rootCmd.PersistentFlags().Bool("yes", false, "Skip confirmation prompts")
    rootCmd.PersistentFlags().Bool("guided", false, "Interactive guided mode")
    rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")
}
```

### 2.4 Init Command Implementation

```go
// services/ai-aas-org/internal/cmd/init.go

func runInit(cmd *cobra.Command, args []string) error {
    // 1. Display welcome
    fmt.Println("Welcome to AI-AAS Org CLI")
    fmt.Println(strings.Repeat("─", 25))

    // 2. Prompt for endpoint (or use flag)
    endpoint, _ := prompt.Input("API Endpoint", "https://api.ai-aas.example.com")

    // 3. Prompt for bootstrap key
    bootstrapKey, _ := prompt.Password("Bootstrap Key")

    // 4. Validate bootstrap key against API
    client := api.NewClient(endpoint, "")
    orgInfo, err := client.ValidateBootstrapKey(ctx, bootstrapKey)
    if err != nil {
        return errors.NewAuthenticationError("Invalid or expired bootstrap key")
    }

    fmt.Printf("Organization: %s\n", orgInfo.Name)

    // 5. Prompt for admin details
    name, _ := prompt.Input("Your Name", "")
    email, _ := prompt.Input("Your Email", "")

    // 6. Create admin account
    admin, apiKey, err := client.RedeemBootstrapKey(ctx, bootstrapKey, name, email)

    // 7. Save config
    cfg := &config.Config{
        APIEndpoint: endpoint,
        APIKey:      apiKey,
        OrgID:       orgInfo.ID,
    }
    config.Save(cfg)

    // 8. Set file permissions
    os.Chmod(config.Path(), 0600)

    // 9. Display quick start
    displayQuickStart()

    return nil
}
```

### 2.5 Config Commands

```go
// config show - display current config (mask API key)
// config set KEY VALUE - update config value
```

### 2.6 Makefile

```makefile
BINARY_NAME=ai-aas-org
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build build-all test clean install

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY_NAME) ./cmd/ai-aas-org

build-all:
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/ai-aas-org
	GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/ai-aas-org
	GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/ai-aas-org
	GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/ai-aas-org

test:
	go test -race ./...

install: build
	cp bin/$(BINARY_NAME) $(GOPATH)/bin/
```

---

## Phase 3: User & Key Management

**Goal:** Implement user, user models, and apikey commands

### 3.1 API Client Setup

Reuse existing clients:
```go
import (
    "github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/userorg"
)
```

### 3.2 User Commands

| Command | Implementation |
|---------|----------------|
| `user list` | Call `userorg.Client.ListUsers(orgID)` |
| `user create` | Call `userorg.Client.CreateUser(orgID, name, email, role)` |
| `user show` | Call `userorg.Client.GetUser(userID)` |
| `user update` | Call `userorg.Client.UpdateUser(userID, updates)` |
| `user delete` | Confirm, then `userorg.Client.DeleteUser(userID)` |

**Guided mode for create:**
```go
if guided {
    name, _ = prompt.Input("User's full name", "")
    email, _ = prompt.Input("User's email", "")
    grantAll, _ = prompt.Confirm("Grant access to all models?", true)

    fmt.Printf("Will create user %s (%s)\n", name, email)
    if ok, _ := prompt.Confirm("Proceed?", true); !ok {
        return nil
    }
}
```

### 3.3 User Models Commands

**New API endpoint required:** `GET/PUT /v1/users/{id}/model-access`

| Command | Implementation |
|---------|----------------|
| `user models list` | Call new endpoint |
| `user models grant` | Call new endpoint |
| `user models revoke` | Call new endpoint |

### 3.4 API Key Commands

| Command | Implementation |
|---------|----------------|
| `apikey list` | Call `userorg.Client.ListAPIKeys(orgID, userID?)` |
| `apikey create` | Call `userorg.Client.IssueUserAPIKey(userID, name)` |
| `apikey show` | Call `userorg.Client.GetAPIKey(keyID)` |
| `apikey rotate` | Confirm, create new, revoke old |
| `apikey delete` | Confirm, then `userorg.Client.DeleteAPIKey(keyID)` |

**Key display (once only):**
```go
fmt.Println("API Key created successfully!")
fmt.Println()
fmt.Printf("  Key ID:  %s\n", key.ID)
fmt.Printf("  Secret:  %s\n", key.Secret)
fmt.Println()
fmt.Println("⚠️  Save this key now - it won't be shown again!")
```

---

## Phase 4: Reporting & Audit

**Goal:** Implement model, usage, audit, and org commands

### 4.1 Model Commands

| Command | Implementation |
|---------|----------------|
| `model list` | List models available to org |
| `model show` | Show model details + user access |

Uses existing registry client with org filtering.

### 4.2 Usage Commands

Reuse analytics client:
```go
import (
    "github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/analytics"
)
```

| Command | Implementation |
|---------|----------------|
| `usage summary` | Aggregate tokens/requests for org |
| `usage by-user` | Breakdown by user |
| `usage by-model` | Breakdown by model |
| `usage export` | CSV/JSON export |

**Period parsing:**
```go
func parsePeriod(s string) (start, end time.Time, error) {
    switch s {
    case "1d": return time.Now().AddDate(0,0,-1), time.Now(), nil
    case "7d": return time.Now().AddDate(0,0,-7), time.Now(), nil
    case "30d": return time.Now().AddDate(0,0,-30), time.Now(), nil
    case "this-month": return startOfMonth(), time.Now(), nil
    case "last-month": return startOfLastMonth(), endOfLastMonth(), nil
    }
}
```

### 4.3 Audit Commands

**New API endpoint required:** `GET /v1/org/{id}/audit`

| Command | Implementation |
|---------|----------------|
| `audit list` | Query audit events with filters |
| `audit export` | CSV/JSON export |

**Audit event types:**
- `user.created`, `user.updated`, `user.deleted`
- `apikey.created`, `apikey.rotated`, `apikey.deleted`
- `model_access.granted`, `model_access.revoked`
- `org.updated`
- `auth.init`

### 4.4 Org Commands

| Command | Implementation |
|---------|----------------|
| `org show` | Display org details, limits, user count |
| `org update` | Update org name |

---

## Phase 5: UX Polish

**Goal:** Add guided mode, help topics, shell completions, install script

### 5.1 Guided Mode

Add to all create/update commands:
```go
func addGuidedFlag(cmd *cobra.Command) {
    cmd.Flags().Bool("guided", false, "Interactive guided mode")
}

func isGuided(cmd *cobra.Command) bool {
    g, _ := cmd.Flags().GetBool("guided")
    return g
}
```

### 5.2 Help Topics

**Directory:** `services/ai-aas-org/internal/guides/`

| Topic | File | Content |
|-------|------|---------|
| `onboarding` | `onboarding.md` | Getting started guide |
| `users` | `users.md` | User management walkthrough |
| `api-keys` | `api-keys.md` | API key best practices |
| `usage` | `usage.md` | Understanding usage reports |
| `troubleshooting` | `troubleshooting.md` | Common issues |

**Help command implementation:**
```go
//go:embed guides/*.md
var guidesFS embed.FS

func runHelp(cmd *cobra.Command, args []string) error {
    if len(args) == 0 {
        // List available topics
        return listTopics()
    }

    topic := args[0]
    content, err := guidesFS.ReadFile(fmt.Sprintf("guides/%s.md", topic))
    if err != nil {
        return fmt.Errorf("unknown topic: %s", topic)
    }

    // Render markdown
    renderMarkdown(content)
    return nil
}
```

### 5.3 Shell Completions

```go
// Add completion command
var completionCmd = &cobra.Command{
    Use:   "completion [bash|zsh|fish]",
    Short: "Generate shell completion script",
}

func init() {
    rootCmd.AddCommand(completionCmd)
}
```

### 5.4 Install Script

**File:** `services/ai-aas-org/scripts/install.sh`

```bash
#!/bin/bash
set -e

REPO="otherjamesbrown/ai-aas"
BINARY="ai-aas-org"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS/Arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
esac

# Download
VERSION=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep tag_name | cut -d'"' -f4)
URL="https://github.com/$REPO/releases/download/$VERSION/${BINARY}-${OS}-${ARCH}"

echo "Downloading $BINARY $VERSION for $OS/$ARCH..."
curl -sL "$URL" -o "/tmp/$BINARY"
chmod +x "/tmp/$BINARY"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "/tmp/$BINARY" "$INSTALL_DIR/$BINARY"
else
    sudo mv "/tmp/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "Installed $BINARY to $INSTALL_DIR/$BINARY"
echo ""
echo "Next steps:"
echo "  $BINARY init    # Set up your organization"
echo "  $BINARY --help  # See all commands"
```

### 5.5 Example-Rich Help Text

Every command should have examples:
```go
var userCreateCmd = &cobra.Command{
    Use:   "create",
    Short: "Create a new user in your organization",
    Long: `Create a new user in your organization.

New users are granted access to all organization models by default.
Use 'ai-aas-org user models revoke' to restrict access.`,
    Example: `  # Create a new team member
  ai-aas-org user create --name "Bob Smith" --email bob@acme.com

  # Create an admin user
  ai-aas-org user create --name "Jane Admin" --email jane@acme.com --role admin

  # Use guided mode for interactive creation
  ai-aas-org user create --guided`,
}
```

---

## Phase 6: Platform Admin Integration

**Goal:** Add bootstrap key commands to ai-aas-cli and implement API endpoints

### 6.1 Bootstrap Key Commands (ai-aas-cli)

**Add to:** `services/ai-aas-cli/cmd/org/bootstrap_key.go`

```go
// ai-aas-cli org bootstrap-key create --org ORG_ID [--expires 7d] [--admin-email EMAIL]
// ai-aas-cli org bootstrap-key list --org ORG_ID
// ai-aas-cli org bootstrap-key revoke KEY_ID
```

### 6.2 API Endpoints Required

| Endpoint | Method | Request | Response |
|----------|--------|---------|----------|
| `/v1/org/bootstrap-keys` | POST | `{org_id, expires, admin_email}` | `{key_id, key, expires_at}` |
| `/v1/org/bootstrap-keys` | GET | `?org_id=X` | `[{key_id, org_id, expires_at, used}]` |
| `/v1/org/bootstrap-keys/{id}` | DELETE | - | `204` |
| `/v1/org/bootstrap` | POST | `{key, name, email}` | `{user, api_key, org}` |

### 6.3 Backend Implementation Notes

**Bootstrap key storage:**
- Store in database with: `id`, `org_id`, `key_hash`, `admin_email`, `expires_at`, `used_at`
- Key format: `org_boot_<32-char-random>`
- Single-use: mark `used_at` on redemption
- Expiry: default 7 days

**Redemption flow:**
1. Validate key exists and not expired
2. Validate key not already used
3. Create user with `role=admin` in organization
4. Issue API key for user
5. Mark bootstrap key as used
6. Return user + API key + org info

---

## Dependencies & Risks

### External Dependencies

| Dependency | Required By | Risk |
|------------|-------------|------|
| User-org service | Phase 3 | Low - exists |
| Analytics service | Phase 4 | Low - exists |
| Bootstrap key API | Phase 6 | Medium - new endpoints |
| Audit event API | Phase 4 | Medium - new endpoint |

### Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Shared extraction breaks ai-aas-cli | High | Thorough testing, feature flag rollback |
| Bootstrap key security issues | High | Security review, key rotation, short expiry |
| API endpoint delays | Medium | Mock API for CLI development |

---

## Testing Strategy

### Unit Tests

- [ ] All command flag parsing
- [ ] Config loading/saving
- [ ] Period parsing (usage commands)
- [ ] Output formatting

### Integration Tests

- [ ] Init flow with mock API
- [ ] User CRUD operations
- [ ] API key lifecycle
- [ ] Usage data retrieval

### E2E Tests

- [ ] Full bootstrap → init → create user flow
- [ ] Guided mode walkthrough
- [ ] Confirmation prompts

### Manual Testing

- [ ] Install script on Linux amd64
- [ ] Install script on macOS arm64
- [ ] Help text readability
- [ ] Shell completions

---

## File Summary

### New Files

| Path | Purpose |
|------|---------|
| `services/cli-shared/` | Shared CLI components (new module) |
| `services/ai-aas-org/` | Org admin CLI (new module) |
| `services/ai-aas-org/scripts/install.sh` | Install script |

### Modified Files

| Path | Changes |
|------|---------|
| `services/ai-aas-cli/go.mod` | Add cli-shared dependency |
| `services/ai-aas-cli/internal/output/` | Move to cli-shared |
| `services/ai-aas-cli/internal/errors/` | Move to cli-shared |
| `services/ai-aas-cli/cmd/org/*.go` | Add bootstrap-key commands |

---

## Success Criteria

1. **Shared components work for both CLIs** - No duplication, clean imports
2. **Init flow is smooth** - < 2 minutes from install to first command
3. **Help is excellent** - Every command has examples, topics are comprehensive
4. **Errors are helpful** - Clear messages with recovery suggestions
5. **Tests pass** - Unit, integration, and E2E tests all green
6. **Install script works** - Linux and macOS, amd64 and arm64

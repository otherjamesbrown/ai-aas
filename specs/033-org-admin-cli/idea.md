# Idea: Org Admin CLI (ai-aas-org)

**Spec Number:** 033
**Created:** 2025-12-28
**Status:** Draft

## Problem

Organization administrators need a simplified, user-friendly CLI to manage their org without access to platform-level operations. Currently, the full `ai-aas-cli` is designed for platform administrators and exposes too many commands (model deployment, infrastructure, credentials) that org admins shouldn't access.

We need:
- A cut-down CLI focused on org management tasks
- Smooth onboarding experience for new org admins
- Excellent documentation and walkthroughs
- Clear separation from platform admin capabilities

## Discussion

### Target Users
- **Org admins only** - regular org users will not use the CLI
- Org admins are colleagues who receive an email with CLI + bootstrap key

### Onboarding Flow
1. Platform admin creates org via `ai-aas-cli org create`
2. Platform admin generates bootstrap key via `ai-aas-cli org bootstrap-key create`
3. Platform admin emails org admin: install script URL + bootstrap key
4. Org admin runs install script, then `ai-aas-org init`
5. Init wizard prompts for endpoint, bootstrap key, name, email
6. Creates admin account, saves API key locally
7. Shows quick-start guide

### Core Capabilities

| Area | Commands |
|------|----------|
| **Users** | list, create, show, update, delete |
| **User Model Access** | list, grant, revoke (per-user model permissions) |
| **API Keys** | list, create, show, rotate, delete |
| **Models** | list, show (view available models) |
| **Usage** | summary, by-user, by-model, export |
| **Audit** | list, export (who did what) |
| **Org** | show, update |
| **Config** | show, set |

### Key Decisions
- **CLI name**: `ai-aas-org`
- **Distribution**: Install script (`curl ... | bash`)
- **Bootstrap key expiry**: 7 days default
- **Key re-issue**: Platform admin can generate new bootstrap key if needed
- **Model access default**: New users get access to all org models by default

### User Experience Requirements
- Excellent `--help` text with examples for every command
- Interactive walkthroughs for common tasks
- Clear error messages with recovery suggestions
- Quick-start guide after init
- Consistent command structure (noun-verb pattern)

### UX Patterns (Confirmed)

**Guided Mode**
- Commands support `--guided` flag for interactive walkthrough
- Prompts for each required field with sensible defaults
- Shows what will happen before executing

**Help System**
- Every command has 2-3 real-world examples in `--help`
- Topic-based guides via `ai-aas-org help <topic>`:
  - `help onboarding` - Getting started guide
  - `help users` - User management walkthrough
  - `help api-keys` - API key best practices
  - `help usage` - Understanding usage reports

**Output Formats**
- Table format for humans (default)
- `--json` for scripting/automation
- `--quiet` for minimal output (just IDs)

**Confirmation Prompts**
- Destructive operations require confirmation (type 'delete' to confirm)
- Bypass with `--yes` flag for scripting
- Show what will be affected before confirming

## Proposed Approach

### New CLI: ai-aas-org

```
ai-aas-org
├── init                           # First-time setup with bootstrap key
├── user
│   ├── list
│   ├── create --name --email
│   ├── show <user-id>
│   ├── update <user-id> [--name] [--email] [--role]
│   ├── delete <user-id>
│   └── models                     # Manage user's model access
│       ├── list <user-id>
│       ├── grant <user-id> <model>
│       └── revoke <user-id> <model>
├── apikey
│   ├── list [--user <user-id>]
│   ├── create --user <user-id> [--name "CI key"]
│   ├── show <key-id>
│   ├── rotate <key-id>
│   └── delete <key-id>
├── model
│   ├── list                       # Models available to org
│   └── show <model>               # Details, who has access
├── usage
│   ├── summary                    # Org totals
│   ├── by-user [--period 7d]
│   ├── by-model [--period 7d]
│   └── export [--format csv|json]
├── audit
│   ├── list [--user] [--action] [--since]
│   └── export [--format csv|json]
├── org
│   ├── show                       # Org details + limits
│   └── update [--name]
└── config
    ├── show
    └── set <key> <value>
```

### Platform Admin Side (additions to ai-aas-cli)

```
ai-aas-cli org bootstrap-key
├── create --org <org-id> [--expires 7d] [--admin-email]
├── list --org <org-id>
└── revoke <key-id>
```

### Install Script

```bash
curl -fsSL https://ai-aas.example.com/install-org-cli.sh | bash
```

- Detects OS/arch
- Downloads appropriate binary
- Installs to /usr/local/bin or ~/.local/bin
- Prints next steps

### Init Experience

```
$ ai-aas-org init

Welcome to AI-AAS Org CLI
─────────────────────────

API Endpoint: https://api.ai-aas.example.com
Bootstrap Key: ▪▪▪▪▪▪▪▪▪▪▪▪▪▪▪▪

Connecting... ✓

Organization: Acme Corp
Your Name: Jane Smith
Your Email: jane@acme.com

Creating your admin account... ✓
Generating API key... ✓

Setup complete!

Configuration saved to ~/.ai-aas-org.yaml

┌─────────────────────────────────────────┐
│ Quick Start                             │
├─────────────────────────────────────────┤
│ ai-aas-org model list    # See models   │
│ ai-aas-org user create   # Add member   │
│ ai-aas-org usage summary # View usage   │
│ ai-aas-org --help        # All commands │
└─────────────────────────────────────────┘
```

## Open Questions

- What API endpoints need to be added/modified to support bootstrap key flow?

## Resolved Questions

- ~~Should we add budget management commands now or defer?~~ → Defer to future
- ~~Should there be a `--guided` mode for commands that walks through interactively?~~ → Yes
- ~~Config file location?~~ → `~/.ai-aas-org.yaml` (matches ai-aas-cli pattern)
- ~~What audit events to capture?~~ → All: user CRUD, API key lifecycle, model access changes, login/init

## Out of Scope

- Model deployment/undeployment (platform admin only)
- Model caching/registry management (platform admin only)
- Org creation/deletion (platform admin only)
- Infrastructure credentials (HuggingFace tokens, S3) (platform admin only)
- Routing configuration (platform admin only)
- Cross-org access
- Budget enforcement (deferred to future)

## Notes

- Can reuse much of the existing client code from ai-aas-cli
- Need to ensure API enforces org-scoping (users can only see/modify their own org)
- Consider generating shell completions (bash/zsh/fish) for better UX
- Help text should include real examples, not just flag descriptions

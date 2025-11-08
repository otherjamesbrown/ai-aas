# Tooling Version Management

This repository centralizes all automation tool versions in `configs/tool-versions.mk`.  
Teams MUST update that manifest whenever bumping the supported toolchain.

## Version Bump Process

1. Edit `configs/tool-versions.mk`, updating the desired version constants with inline comments if needed.
2. Run affected Make targets locally to ensure compatibility (e.g., `make check`, `make ci-local`).
3. Update quickstart or troubleshooting docs if workflows change.
4. Capture the change in release notes (or project changelog) along with rationale.
5. Submit PR including:
   - Manifest change
   - Any lockfile or dependency updates (e.g., `go.sum`)
   - Verification logs from Step 2
6. After merge, notify contributors in the communication channel so they can upgrade locally.

Automation tasks (e.g., `make check`) must read versions from the manifest—hardcoding versions in scripts is prohibited.


# Next Spec with Gaps - Analysis

Generated: 2025-11-15

## Summary

**Recommended Next Spec: Spec 002 - Local Dev Environment**

This spec has the most actionable foundational gaps that enable other development work.

## Detailed Analysis

### Spec 002 - Local Dev Environment ⭐ **BEST CANDIDATE**

**Status**: 0% complete (0/19+ tasks)
**Gaps**: Phase 1-2 foundational infrastructure
**Priority**: High - enables other development

**Why it's next**:
- Clear, actionable foundational tasks
- Enables local development for all services
- No dependencies on other incomplete specs
- Quick wins possible (bootstrap script updates, compose files)

**Gaps to fill**:
- Phase 1: Update bootstrap.sh, tooling versions, README
- Phase 2: Docker Compose base config, port mappings, helper scripts
- Phase 3+: Remote workspace provisioning (can be done incrementally)

**Estimated effort**: 2-3 days for Phase 1-2

---

### Spec 005 - User-Org Service 🔄

**Status**: 14% complete (7/50 tasks)
**Gaps**: Phase 2+ implementation (auth, user/org lifecycle, API keys, policies)
**Priority**: High - core platform service

**Why it might be next**:
- Critical for platform functionality
- Phase 1 complete (scaffolding, migrations)
- Clear next steps in Phase 2

**Gaps to fill**:
- Phase 2: Authentication flows (login/refresh/logout), user/org lifecycle, API key management
- Phase 3: Policy engine, budget enforcement
- Phase 4+: Declarative configuration, reconciliation

**Estimated effort**: 2-3 weeks for Phase 2

---

### Spec 006 - API Router Service ⚠️

**Status**: Implementation complete but tasks.md shows 0%
**Gaps**: Tasks.md sync needed (not actual implementation gaps)
**Priority**: Medium - documentation gap

**Why it might be next**:
- Implementation exists per HANDOFF.md
- Just needs tasks.md updated to reflect completion
- Could mark all tasks as complete based on existing code

**Action needed**:
- Review implementation against tasks.md
- Mark completed tasks
- Verify any remaining gaps

**Estimated effort**: 1-2 hours (documentation update)

---

### Spec 004 - Shared Libraries ✅

**Status**: 97% complete (45/46 tasks)
**Gaps**: Only pilot measurement remaining
**Priority**: Low - blocked on pilot services

**Why it's not next**:
- Blocked on pilot service deployments (`billing-api`, `content-ingest`)
- Cannot proceed until pilots complete

---

### Spec 008 - Web Portal

**Status**: 0% complete (0 tasks done)
**Gaps**: Phase 1 setup (workspace, build config, dependencies)
**Priority**: Medium - depends on backend services

**Why it's not next**:
- Depends on backend services (005, 006) being more complete
- Frontend work can wait until APIs are stable

---

## Recommendation

**Start with Spec 002 - Local Dev Environment** because:

1. ✅ **Actionable gaps**: Clear foundational tasks with no blockers
2. ✅ **Enables others**: Makes local development easier for all services
3. ✅ **Quick wins**: Phase 1-2 can be completed in days, not weeks
4. ✅ **Independent**: No dependencies on incomplete specs
5. ✅ **High value**: Improves developer experience across the platform

**First tasks to tackle**:
1. T001: Update tooling versions in `configs/tool-versions.mk`
2. T002: Extend `scripts/setup/bootstrap.sh` for new tools
3. T003: Refresh README prerequisites
4. T004: Create Docker Compose base config
5. T005-T007: Port mappings, log redaction, helper scripts


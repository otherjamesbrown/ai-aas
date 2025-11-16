# Admin CLI - Next Steps

**Date**: 2025-11-15  
**Current Status**: Foundation complete, bootstrap command implemented, integration tests ready

## ✅ Completed Work

1. **Phase 1: Setup** - Complete
   - Project structure created
   - Go module initialized with dependencies
   - Makefile and README in place
   - CLI entrypoint with command registration

2. **Phase 2: Foundational** - Complete
   - Configuration management (Viper-based)
   - Health checks
   - Output formatters (table, JSON, CSV)
   - Audit logging
   - Progress indicators
   - API client packages (structure)
   - Error handling utilities
   - Retry logic

3. **Phase 3: Bootstrap** - Partially Complete
   - Bootstrap command structure implemented
   - Health checks integrated
   - Audit logging integrated
   - API client placeholder created
   - Integration tests created

4. **Integration Testing** - Ready
   - Test framework in place
   - Health check tests
   - Bootstrap flow tests
   - Can run against live services

## 🎯 Immediate Next Steps (Priority Order)

### 1. Complete Bootstrap Functionality (Phase 3 - High Priority)

**Goal**: Make bootstrap command fully functional end-to-end

**Tasks**:
- [ ] **Implement actual user-org-service bootstrap endpoint** (if it doesn't exist, may need service-side work)
  - Check if `/v1/bootstrap` endpoint exists in user-org-service
  - If not, may need to use existing org/user creation endpoints
  - File: `services/admin-cli/internal/client/userorg/client.go`

- [ ] **Test bootstrap against running service**
  - Start user-org-service locally
  - Run integration tests
  - Verify admin account creation works
  - File: `services/admin-cli/test/integration/bootstrap_test.go`

- [ ] **Complete credential rotation command**
  - Implement rotate command
  - Add break-glass recovery mode
  - File: `services/admin-cli/internal/commands/credentials.go`

### 2. Implement Org/User Commands (Phase 4 - Medium Priority)

**Goal**: Enable day-2 management operations

**Tasks**:
- [ ] **Complete user-org-service API client for org operations**
  - Implement `ListOrgs()`, `CreateOrg()`, `UpdateOrg()`, `DeleteOrg()` methods
  - Add proper request/response types
  - Files: `services/admin-cli/internal/client/userorg/client.go`, `types.go`

- [ ] **Implement org commands**
  - Complete `org list`, `create`, `update`, `delete` commands
  - Add flags (--dry-run, --confirm, --format)
  - Add structured output support
  - File: `services/admin-cli/internal/commands/org.go`

- [ ] **Implement user commands**
  - Complete `user list`, `create`, `update`, `delete` commands
  - Add idempotent support (--upsert flag)
  - File: `services/admin-cli/internal/commands/user.go`

- [ ] **Add integration tests for org/user lifecycle**
  - Test CRUD operations
  - Test idempotent behavior
  - File: `services/admin-cli/test/integration/org_user_test.go`

### 3. Verify Against Running Services (Testing - High Priority)

**Goal**: Ensure everything works with actual services

**Tasks**:
- [ ] **Set up local development environment**
  ```bash
  # Start user-org-service
  cd services/user-org-service
  make run  # or docker-compose up
  ```

- [ ] **Run integration tests**
  ```bash
  cd services/admin-cli
  make integration-test
  ```

- [ ] **Manual testing**
  - Test bootstrap command manually
  - Verify JSON output format
  - Test health checks
  - Verify audit logs

### 4. Complete Remaining Features (Lower Priority)

**Phase 5: Exports**
- [ ] Implement analytics-service API client
- [ ] Implement export commands (usage, memberships)
- [ ] Add reconciliation verification
- [ ] Add streaming support for large datasets

**Phase 6: Polish**
- [ ] Command completion (bash/zsh)
- [ ] Performance benchmarks
- [ ] Additional error handling improvements
- [ ] Documentation examples

## 📋 Recommended Workflow

### Step 1: Make Bootstrap Work End-to-End (1-2 days)

1. **Check user-org-service API**:
   ```bash
   # Check what endpoints are available
   curl http://localhost:8081/healthz
   curl http://localhost:8081/v1/orgs  # Check if this exists
   ```

2. **Update API client** to use actual endpoints:
   - If bootstrap endpoint doesn't exist, use org/user creation endpoints
   - Implement proper request/response handling
   - Add error handling

3. **Test manually**:
   ```bash
   # Build and test
   cd services/admin-cli
   make build
   ./bin/admin-cli bootstrap --dry-run --user-org-endpoint=http://localhost:8081
   ```

4. **Run integration tests**:
   ```bash
   make integration-test
   ```

### Step 2: Implement Org Commands (2-3 days)

1. **Extend API client** with org methods:
   - Look at existing user-org-service API handlers
   - Implement matching client methods
   - Add proper types

2. **Implement org commands**:
   - Complete command implementations
   - Add flags and validation
   - Add structured output

3. **Test**:
   - Manual testing
   - Integration tests

### Step 3: Implement User Commands (2-3 days)

Similar to org commands, implement user lifecycle operations.

## 🔍 Quick Wins

Before diving into full implementation, consider these quick validation steps:

1. **Verify service endpoints exist**:
   ```bash
   # Check user-org-service endpoints
   curl http://localhost:8081/healthz
   curl http://localhost:8081/v1/orgs  # If service is running
   ```

2. **Test CLI help works**:
   ```bash
   cd services/admin-cli
   make build
   ./bin/admin-cli --help
   ./bin/admin-cli bootstrap --help
   ```

3. **Run unit tests**:
   ```bash
   make test
   ```

## 🚧 Known Issues / Blockers

1. **Bootstrap endpoint**: May need to verify if `/v1/bootstrap` exists in user-org-service, or use alternative approach
2. **API client endpoints**: Placeholder endpoints need to be replaced with actual service endpoints
3. **Command implementations**: Many commands return "not implemented" - need to fill in actual logic

## 📚 Resources

- **Spec**: `specs/009-admin-cli/spec.md`
- **Plan**: `specs/009-admin-cli/plan.md`
- **Tasks**: `specs/009-admin-cli/tasks.md`
- **Quickstart**: `specs/009-admin-cli/quickstart.md`
- **Integration Tests**: `services/admin-cli/test/integration/README.md`

## 🎯 Success Criteria

Bootstrap command is "done" when:
- ✅ Can bootstrap against running user-org-service
- ✅ Integration tests pass
- ✅ Audit logs are emitted correctly
- ✅ Credentials are masked in logs
- ✅ JSON output format works

Org/User commands are "done" when:
- ✅ All CRUD operations work
- ✅ Dry-run mode works
- ✅ Structured output (JSON/CSV/table) works
- ✅ Integration tests pass
- ✅ Idempotent operations work


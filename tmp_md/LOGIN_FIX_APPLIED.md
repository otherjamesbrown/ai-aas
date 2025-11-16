# Login Fix Applied - Using user-org-service Directly

## Issue
The UI was trying to connect to `http://localhost:8080/api/auth/login` but the API router service isn't running (port 8080 is closed). The login endpoint works directly on user-org-service.

## Solution Applied
Temporarily configured the UI to use user-org-service directly for authentication:

- **Changed**: `VITE_API_BASE_URL` from `http://localhost:8080/api` to `http://localhost:8081/v1`
- **Updated**: Environment profile `local-dev.yaml`
- **Updated**: `.env.local` file

## Current Configuration

### UI Configuration
- **Base URL**: `http://localhost:8081/v1` (user-org-service)
- **Login Endpoint**: `http://localhost:8081/v1/auth/login` ✓

### Services Status
- ✅ **user-org-service**: Running on port 8081, login endpoint working
- ⚠️ **api-router-service**: Not running (needs investigation)

## Next Steps

1. **Restart UI** (if running): The UI needs to pick up the new `VITE_API_BASE_URL` environment variable
   ```bash
   # If UI is running, restart it to pick up new env var
   # Or set it in the shell before starting:
   export VITE_API_BASE_URL=http://localhost:8081/v1
   cd web/portal && pnpm dev
   ```

2. **Test Login**: 
   - URL: `http://localhost:5173/auth/login`
   - Email: `admin@example.com`
   - Password: `nubipwdkryfmtaho123!`

3. **Fix API Router** (later): Once the API router is fixed and running, we can switch back to using it as the gateway.

## Note
This is a temporary workaround. The proper architecture is:
- UI → API Router (port 8080) → user-org-service (port 8081)

But for now, we're using:
- UI → user-org-service (port 8081) directly

This allows login to work while we investigate why the API router isn't starting.


# Services Status - Final Check

## Service Status Summary

### ✅ Docker Services (All Running)
- **PostgreSQL** (dev-postgres) - Port 5433
- **Redis** (dev-redis) - Port 6379  
- **NATS** (dev-nats) - Ports 4222/8222
- **MinIO** (dev-minio) - Ports 9000/9001
- **Mock Inference** (dev-mock-inference) - Port 8000

### ✅ Backend Services
- **user-org-service** - Port 8081
  - Health: `http://localhost:8081/healthz` ✓
  - Login: `http://localhost:8081/v1/auth/login` ✓
  - Status: Running and healthy

- ⚠️ **api-router-service** - Port 8080
  - Status: Not running (needs investigation)
  - Note: UI temporarily configured to use user-org-service directly

### ✅ Frontend
- **Web Portal UI** - Port 5173
  - URL: `http://localhost:5173`
  - Configuration: Using `VITE_API_BASE_URL=http://localhost:8081/v1`
  - Status: Starting/Checking...

## Configuration

### Environment Variables
- `VITE_API_BASE_URL=http://localhost:8081/v1` (user-org-service)
- `VITE_USE_HTTPS=false` (HTTP for local-dev)

### Database
- Connection: `postgres://postgres:postgres@localhost:5433/ai_aas?sslmode=disable`
- Migrations: Applied (version 3)
- Seeded: Test user created

### Login Credentials
- Email: `admin@example.com`
- Password: `nubipwdkryfmtaho123!`
- Org Slug: `demo`

## Next Steps

1. **Verify UI is running**: Check `http://localhost:5173`
2. **Test Login**: Navigate to `http://localhost:5173/auth/login`
3. **Check Logs**: 
   - UI: `tail -f /tmp/web-portal.log`
   - user-org-service: `tail -f /tmp/user-org-service.log`

## Commands to Check Status

```bash
# Check all services
make env-component-status

# Check Docker services
docker ps | grep -E "postgres|redis|nats|minio|mock"

# Check backend services
curl http://localhost:8081/healthz
ps aux | grep admin-api

# Check UI
curl http://localhost:5173
ps aux | grep vite
```


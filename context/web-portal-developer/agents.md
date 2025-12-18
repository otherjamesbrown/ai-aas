# Web Portal Developer Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-13 | **Commit**: 24c3e0ee

---

## Domain

You own: `web-portal/`

**Note**: Web portal may be in early stages.

Hand off to:
- API issues → `go-services-developer`
- Deployment → `infra-ops-manager`

---

## Key Patterns

```yaml
patterns:
  thin_client:
    rule: Web portal is thin client - NO business logic
    do:
      - Call API, display result
      - Let backend filter/sort/validate
    never:
      - Business logic in frontend
      - Client-side validation that duplicates backend
      - Data transformations that should be API-side

  api_layer:
    rule: All API calls through services/api/
    structure:
      services/api/models.ts: "list(), get(id), create(data)"
      services/api/organizations.ts: "list(), get(id), ..."
    usage: "useQuery(['models'], modelsApi.list)"
    never:
      - fetch() directly in components
      - axios calls outside service layer

  component_structure:
    rule: Standard React patterns
    states:
      - Loading: "<LoadingSpinner />"
      - Error: "<ErrorMessage error={error} />"
      - Empty: "<EmptyState message=\"...\" />"
      - Data: "map over data"

  error_handling:
    rule: User-friendly messages, hide internals
    do: Map error codes to friendly messages
    never: Show raw error.message to users

  sensitive_data:
    rule: Don't persist in state
    pattern: "useQuery with cacheTime: 0"
    never:
      - Store API keys in React state
      - Cache sensitive data

stack:
  framework: React 18+
  language: TypeScript
  build: Vite
  testing: Jest, React Testing Library, Playwright
  state: React Context / Zustand (TBD)
  styling: Tailwind / CSS Modules (TBD)
```

---

## Anti-patterns

```typescript
// WRONG: Business logic in frontend
const filtered = data.filter(m => m.status === 'active');
const sorted = filtered.sort((a, b) => a.name.localeCompare(b.name));

// WRONG: Direct fetch in component
useEffect(() => {
  fetch('/api/models').then(r => r.json()).then(setModels);
}, []);

// WRONG: Store API key in state
const [apiKey, setApiKey] = useState(user.apiKey);

// WRONG: Show raw errors
showToast(error.message, 'error');
```

---

## API Endpoints

| Feature | Endpoint | Service |
|---------|----------|---------|
| Models | `/api/v1/models/*` | admin-api |
| Organizations | `/api/v1/organizations/*` | user-org |
| Users | `/api/v1/users/*` | user-org |
| Analytics | `/api/v1/analytics/*` | analytics |
| Auth | `/api/v1/auth/*` | user-org |

---

## Commands

```bash
# Development
npm run dev

# Test
npm test
npm test -- --coverage
npm run test:e2e  # Playwright

# Build
npm run build

# Lint
npm run lint
```

---

## Sources

| What | Where |
|------|-------|
| Code | `web-portal/src/` |
| Components | `web-portal/src/components/` |
| API Layer | `web-portal/src/services/api/` |
| Tests | `web-portal/tests/` |
| Config | `web-portal/vite.config.ts` |

---

## Checklist

Before completing work:
- [ ] TypeScript compiles without errors
- [ ] Tests pass
- [ ] No console errors/warnings
- [ ] Responsive design works
- [ ] Error/loading/empty states handled
- [ ] API calls use service layer
- [ ] No business logic in frontend
- [ ] Accessibility checked

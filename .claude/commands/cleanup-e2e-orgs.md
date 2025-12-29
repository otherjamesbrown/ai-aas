# Cleanup E2E Test Organizations

Delete all E2E test organizations from the platform.

## Instructions

### Step 1: List E2E Organizations

Run the following command to get all organizations with `e2e-` prefix:

```bash
ai-aas-cli org list --format json 2>/dev/null | jq -r '.[] | select(.slug | startswith("e2e-")) | "\(.id)\t\(.name)"'
```

Count them:
```bash
ai-aas-cli org list --format json 2>/dev/null | jq '[.[] | select(.slug | startswith("e2e-"))] | length'
```

### Step 2: Confirm with User

**Use AskUserQuestion tool:**

Question (header: "Confirm"):
- "Delete all" - Delete all E2E test organizations
- "Cancel" - Abort cleanup

Show the count of organizations to be deleted before asking.

### Step 3: Delete Organizations

If user confirms, delete each organization:

```bash
# Get list of E2E org IDs
E2E_ORGS=$(ai-aas-cli org list --format json 2>/dev/null | jq -r '.[] | select(.slug | startswith("e2e-")) | .id')

# Delete each one
for org_id in $E2E_ORGS; do
  echo "Deleting org: $org_id"
  ai-aas-cli org delete "$org_id" --force
done
```

**Important:** Run deletions in batches of 10-20 to avoid rate limiting. Use parallel execution where possible.

### Step 4: Verify Cleanup

```bash
ai-aas-cli org list --format json 2>/dev/null | jq '[.[] | select(.slug | startswith("e2e-"))] | length'
```

### Step 5: Report Results

Present results:

---

## E2E Organization Cleanup Results

| Metric | Value |
|--------|-------|
| Organizations found | X |
| Successfully deleted | X |
| Failed to delete | X |

### Errors (if any)
| Org ID | Error |
|--------|-------|
| ... | ... |

---

## Notes

- E2E tests should clean up after themselves via `t.Cleanup()` or deferred cleanup
- If E2E orgs keep accumulating, check test cleanup logic in `tests/e2e/fixtures/`
- This command is safe to run - it only deletes orgs with `e2e-` prefix in their slug

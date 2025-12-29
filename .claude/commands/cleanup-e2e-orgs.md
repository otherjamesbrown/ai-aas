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

If user confirms, delete each organization using parallel execution:

```bash
# Delete E2E orgs in parallel (10 concurrent deletions)
ai-aas-cli org list --format json 2>/dev/null | \
  jq -r '.[] | select(.slug | startswith("e2e-")) | .id' | \
  xargs -P 10 -I {} sh -c 'echo "Deleting org: {}"; ai-aas-cli org delete "{}" --force'
```

**Note:** Uses `xargs -P 10` for parallel execution to speed up cleanup.

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

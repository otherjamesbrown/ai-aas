#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo ">> Running shared library verification from ${ROOT_DIR}"

pushd "${ROOT_DIR}" >/dev/null

echo ">> make shared-check"
make shared-check

if [[ "${SKIP_SAMPLE_TESTS:-false}" != "true" ]]; then
  echo ">> Go sample service tests"
  go test ./samples/service-template/go/...

  pushd "${ROOT_DIR}/samples/service-template/ts" >/dev/null
  echo ">> npm ci (sample ts)"
  npm ci >/dev/null
  echo ">> npm run build (sample ts)"
  npm run build >/dev/null
  echo ">> npm test (sample ts)"
  npm test >/dev/null
  popd >/dev/null
else
  echo ">> Skipping sample service tests (SKIP_SAMPLE_TESTS=${SKIP_SAMPLE_TESTS})"
fi

pushd "${ROOT_DIR}/tests/go/integration" >/dev/null
echo ">> Go integration tests"
go test ./... >/dev/null
popd >/dev/null

pushd "${ROOT_DIR}/tests/ts/integration" >/dev/null
echo ">> npm ci (ts integration)"
npm ci >/dev/null
echo ">> npm test (ts integration)"
npm test >/dev/null
popd >/dev/null

pushd "${ROOT_DIR}/tests/ts/contract" >/dev/null
echo ">> npm ci (ts contract)"
npm ci >/dev/null
echo ">> npm test (ts contract)"
npm test >/dev/null
popd >/dev/null

pushd "${ROOT_DIR}/tests/go/contract" >/dev/null
echo ">> Go contract tests"
go test ./... >/dev/null
popd >/dev/null

cat <<'EOT'
Verification complete. Recommended follow-up:
- Run samples/service-template/scripts/smoke.sh against both sample services.
- Update release notes and run the shared-libraries-release workflow with the new version.
EOT


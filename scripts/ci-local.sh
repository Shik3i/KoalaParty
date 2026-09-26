#!/usr/bin/env bash
# Runs every check of .github/workflows/ci.yml and codeql.yml locally, so a push
# never has to be the first test. Usage (from anywhere, Git Bash/Linux/macOS):
#
#   scripts/ci-local.sh            all steps
#   scripts/ci-local.sh --quick    skip npm ci, the Docker image and CodeQL
#   scripts/ci-local.sh --only e2e run a single step (backend, scripts, frontend,
#                                  e2e, docker, codeql)
#
# Browser tests run inside the official Playwright Linux image (all three
# browsers, like CI), because Windows may block Playwright's own Firefox.
# Requirements: Go, Node/npm, Docker (for e2e and docker), and the global
# `codeql-scan` command (%USERPROFILE%\.local\bin) for the CodeQL step.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
QUICK=0
ONLY=""
while [ $# -gt 0 ]; do
  case "$1" in
    --quick) QUICK=1 ;;
    --only) ONLY="$2"; shift ;;
    *) echo "unknown option $1" >&2; exit 2 ;;
  esac
  shift
done

RESULTS=()
FAILED=0
step() {
  local name="$1"; shift
  if [ -n "$ONLY" ] && [ "$ONLY" != "$name" ]; then return; fi
  echo
  echo "=== $name ==="
  local started=$SECONDS
  if "$@"; then
    RESULTS+=("PASS  $name ($((SECONDS - started))s)")
  else
    RESULTS+=("FAIL  $name ($((SECONDS - started))s)")
    FAILED=1
  fi
}

backend() {
  cd "$ROOT/backend" || return 1
  local unformatted
  unformatted="$(gofmt -l .)"
  if [ -n "$unformatted" ]; then echo "gofmt needed: $unformatted"; return 1; fi
  go vet ./... && go test -race -count=1 ./... && go run golang.org/x/vuln/cmd/govulncheck@latest ./...
}

scripts() {
  cd "$ROOT" && node --test scripts/*.test.mjs
}

frontend() {
  cd "$ROOT/frontend" || return 1
  if [ "$QUICK" = 0 ]; then npm ci || return 1; fi
  npm run check && npm run lint && npm test -- --run && npm run build && npm audit --audit-level=high
}

e2e() {
  command -v docker >/dev/null || { echo "docker is required for browser tests"; return 1; }
  cd "$ROOT/frontend" || return 1
  [ -d build ] || npm run build || return 1
  local version
  version="$(node -p "require('./node_modules/@playwright/test/package.json').version")" || return 1
  # The E2E server runs inside the Linux container; SQLite is pure Go, so a
  # cross-compiled binary needs no C toolchain.
  (cd "$ROOT/backend" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$ROOT/.cache/koalaparty-e2e-server" ./cmd/server) || return 1
  local mount="$ROOT"
  if command -v cygpath >/dev/null; then mount="$(cygpath -w "$ROOT")"; fi
  MSYS_NO_PATHCONV=1 docker run --rm --ipc=host \
    -v "$mount:/work" \
    -v koalaparty-e2e-node-modules:/work/frontend/node_modules \
    -e KOALAPARTY_E2E_DB=/tmp/koalaparty-e2e.db \
    -e CI=1 \
    -w /work/frontend \
    "mcr.microsoft.com/playwright:v${version}-noble" \
    bash -c "npm ci --ignore-scripts --no-audit --no-fund >/dev/null && npx playwright test"
}

docker_image() {
  command -v docker >/dev/null || { echo "docker is required"; return 1; }
  cd "$ROOT" || return 1
  docker build --build-arg VERSION=ci --build-arg COMMIT="$(git rev-parse HEAD)" -t koalaparty:ci . || return 1
  docker rm -f koalaparty-ci >/dev/null 2>&1
  docker run -d --name koalaparty-ci -e KOALAPARTY_TRUSTED_ORIGINS=https://smoke.koalaparty.test \
    -p 127.0.0.1:18080:8080 koalaparty:ci >/dev/null || return 1
  local ok=1
  for _ in $(seq 1 30); do
    if curl --fail --silent http://127.0.0.1:18080/api/ready >/dev/null; then
      curl --fail --silent http://127.0.0.1:18080/api/version && echo && ok=0
      break
    fi
    sleep 1
  done
  [ "$ok" = 0 ] || docker logs koalaparty-ci
  docker rm -f koalaparty-ci >/dev/null
  return "$ok"
}

codeql_step() {
  command -v codeql-scan >/dev/null || { echo "codeql-scan is not installed (see %USERPROFILE%\\.local\\bin)"; return 1; }
  codeql-scan "$ROOT"
}

step backend backend
step scripts scripts
step frontend frontend
step e2e e2e
if [ "$QUICK" = 0 ] || [ -n "$ONLY" ]; then
  step docker docker_image
  step codeql codeql_step
fi

echo
echo "=== summary ==="
printf '%s\n' "${RESULTS[@]}"
exit "$FAILED"

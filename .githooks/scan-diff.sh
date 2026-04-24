#!/usr/bin/env bash
# Credential-pattern scanner for pre-commit hook and CI.
#
# Usage:
#   scan-diff.sh            - scan all tracked files (CI mode)
#   scan-diff.sh --staged   - scan files in the current staged diff
#
# Exits non-zero when anything matches. The patterns are intentionally
# conservative — a tripwire for the obvious cases, not a real secret scanner.
# Real scanners (trufflehog, gitleaks) go in a later milestone if needed.

set -euo pipefail

MODE="${1:-all}"
case "$MODE" in
  --staged)
    FILES=$(git diff --cached --name-only --diff-filter=ACMR)
    ;;
  all)
    FILES=$(git ls-files)
    ;;
  *)
    echo "Usage: scan-diff.sh [--staged]" >&2
    exit 2
    ;;
esac

if [[ -z "$FILES" ]]; then
  echo "scan-diff: no files to scan"
  exit 0
fi

# Files we never scan: fixtures, documentation prose, example configs,
# generated output, the hook script itself, the pre-commit hook.
is_skipped() {
  case "$1" in
    *.env.example|*.md|*.svg|*.png|*.ico) return 0 ;;
    *_test.go|*_test.py|*.test.ts|*.test.tsx|*.spec.ts|*.spec.tsx) return 0 ;;
    *testdata/*|*/fixtures/*) return 0 ;;
    *.githooks/scan-diff.sh|*.githooks/pre-commit) return 0 ;;
    web/package-lock.json) return 0 ;;
    *.go.sum) return 0 ;;
  esac
  return 1
}

# Patterns. Anchored so a bare `password` mention in source code doesn't fire;
# we look for assignments to non-trivial-looking values.
#
# Each pattern is (regex, human-readable-label).
PATTERNS=(
  'SMTP_PASSWORD=[^[:space:]$]{4,}|SMTP PASSWORD set'
  'SMTP_USER=[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}|SMTP USER set'
  'PUBMED_API_KEY=[a-fA-F0-9]{16,}|PubMed API key'
  'api_key[[:space:]]*[:=][[:space:]]*["'"'"'][a-zA-Z0-9]{20,}["'"'"']|api_key literal'
  'API_KEY[[:space:]]*[:=][[:space:]]*["'"'"'][a-zA-Z0-9]{20,}["'"'"']|API_KEY literal'
  'password[[:space:]]*[:=][[:space:]]*["'"'"'][^"'"'"'$[:space:]]{8,}["'"'"']|password literal'
  'AKIA[0-9A-Z]{16}|AWS access key id'
  'ghp_[A-Za-z0-9]{36}|GitHub personal access token'
  'xox[baprs]-[A-Za-z0-9-]+|Slack token'
)

FAIL=0
MATCHES=""

for file in $FILES; do
  if [[ ! -f "$file" ]]; then continue; fi
  if is_skipped "$file"; then continue; fi

  for p in "${PATTERNS[@]}"; do
    pattern="${p%|*}"
    label="${p##*|}"
    if matches=$(grep -nE "$pattern" "$file" 2>/dev/null); then
      FAIL=1
      while IFS= read -r line; do
        MATCHES="${MATCHES}${file}:${line}   [${label}]"$'\n'
      done <<< "$matches"
    fi
  done
done

if [[ $FAIL -eq 1 ]]; then
  echo "scan-diff: credential patterns detected:" >&2
  printf '%s' "$MATCHES" >&2
  echo "" >&2
  echo "If this is a false positive, extend the skip list in .githooks/scan-diff.sh" >&2
  echo "or move the value to .env (gitignored) and keep .env.example as the template." >&2
  exit 1
fi

echo "scan-diff: $(echo "$FILES" | wc -l) files scanned, no credential patterns detected."
exit 0

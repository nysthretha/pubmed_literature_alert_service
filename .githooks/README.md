# Git hooks

Local hooks tracked in the repo. Opt in after cloning:

```bash
make install-hooks   # wraps `git config core.hooksPath .githooks`
```

To uninstall:

```bash
make uninstall-hooks
```

## What's here

- `pre-commit` — runs `scan-diff.sh --staged` before every commit. Aborts the
  commit if any staged change looks like a credential (SMTP password, PubMed
  API key, AWS access key, GitHub token, Slack token, `password=<literal>`).
- `scan-diff.sh` — the shared scanner. Used by both the local hook and the
  CI `secret-scan` job (`.github/workflows/ci.yml`). Single source of
  truth for which patterns we block.

## When the scan yells at you

1. If the match is a real credential, move the value to `.env` (gitignored)
   and keep a placeholder in `.env.example` for the same key.
2. If it's a false positive (unusual source code that happens to match a
   regex), extend the skip list or the anti-pattern in `scan-diff.sh`.

The scanner is a tripwire, not a real secret scanner. Bypassing it with
`git commit --no-verify` is allowed when you know what you're doing — CI still
runs the same scanner against the push/PR diff, so anything that would have
mattered gets caught before merge.

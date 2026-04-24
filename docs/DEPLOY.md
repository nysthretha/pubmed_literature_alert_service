# Deployment runbook

Production target: Railway.  Domain: `<service>.up.railway.app` (custom domain: future).

This doc is a runbook. Copy-paste the commands, don't pause to understand
them mid-incident.

---

## First-time Railway setup

### 1. Create the project and services

In Railway's dashboard:

1. New Project → Deploy from GitHub repo (this repo, `main` branch).
2. Add services in order:
   - **scheduler** — Deploy from repo root. Dockerfile path: `Dockerfile`.
     Railway autodetects `railway.json`.
   - **worker** — Deploy from repo, Dockerfile path: `worker/Dockerfile`.
     Settings → Networking → **Disable** public networking (background worker).
   - **digest_worker** — same as worker but Dockerfile path:
     `worker/Dockerfile.digest`.
   - **Postgres** — Add from Railway's managed service catalog.
   - **RabbitMQ** — Deploy template or use image `rabbitmq:3-management-alpine`.
     Disable public networking.

### 2. Environment variables

Set these under each service's **Variables** tab. Placeholders are dev
defaults — see `.env.example` for the full list.

**All services** (scheduler + both workers):

| Var | Value |
|---|---|
| `POSTGRES_URL` | `${{Postgres.DATABASE_URL}}?sslmode=require` |
| `RABBITMQ_URL` | `amqp://${{RabbitMQ.RABBITMQ_DEFAULT_USER}}:${{RabbitMQ.RABBITMQ_DEFAULT_PASS}}@rabbitmq.railway.internal:5672/` |
| `PUBMED_EMAIL` | your real email — NCBI requires it |
| `PUBMED_TOOL_NAME` | `pubmed-alerts` |
| `PUBMED_API_KEY` | your NCBI API key (optional, raises rate limit) |

Use Railway's variable-reference syntax (`${{Postgres.DATABASE_URL}}`) so
credentials rotate automatically when the managed service rotates them.

**scheduler only**:

| Var | Value | Notes |
|---|---|---|
| `AUTH_COOKIE_SECURE` | `true` | Production is HTTPS |
| `SCHEDULER_TICK_SECONDS` | `300` | 5 min |
| `HEALTHCHECK_URL` | from Healthchecks.io (see below) | Optional but recommended |
| `PORT` | set by Railway automatically | |

**digest_worker only** (SMTP via Brevo):

| Var | Value |
|---|---|
| `DIGEST_MODE` | `smtp` |
| `DIGEST_SEND_HOUR` | `7` |
| `DIGEST_TIMEZONE` | `Europe/Istanbul` |
| `DIGEST_RECIPIENT` | your real email |
| `DIGEST_SUBJECT_PREFIX` | `PubMed Alerts` |
| `SMTP_HOST` | `smtp-relay.brevo.com` |
| `SMTP_PORT` | `587` |
| `SMTP_USER` | your Brevo SMTP login |
| `SMTP_PASSWORD` | your Brevo SMTP key |
| `SMTP_FROM` | sender address (verified in Brevo) |

### 3. Bootstrap the admin user (first deploy only)

Migration `00006_scope_queries_digests_to_users.sql` aborts with an
actionable error if no admin user exists. First deploy will fail at
migration — this is expected. Do:

```bash
# From your laptop, with Railway CLI installed and linked to the project:
railway run --service scheduler ./scheduler create-admin \
    --username <you> --email <you>@example.com
# Prompts twice for a password (min 8 chars).
```

Redeploy the scheduler (or wait for Railway's restart). Migration 00006
succeeds; the app comes up.

### 4. Healthchecks.io liveness monitor

1. Free account at https://healthchecks.io
2. Create a check:
   - **Schedule**: period **10 min**, grace **2 min**
   - **Name**: `pubmed-scheduler`
3. Copy the ping URL (format: `https://hc-ping.com/<uuid>`)
4. Set `HEALTHCHECK_URL` env var on the scheduler service to that URL.
5. Redeploy. Scheduler logs `"msg":"healthcheck enabled"` and pings every
   5 min.

If pings stop for >12 minutes, Healthchecks.io emails you. This catches
the "container is up but the Go process is deadlocked" case Railway's own
monitoring can't see.

---

## Deploying changes

`git push origin main` — Railway autodeploys. CI gates the push: all of
secret-scan, go-tests, python-check, frontend must be green.

To trigger a redeploy without a new commit (env var change, for example):
Railway dashboard → service → Deployments → Redeploy.

---

## Checking logs

```bash
railway logs --service scheduler         # tail
railway logs --service scheduler -n 200  # last N lines
railway logs --service digest_worker
railway logs --service worker
```

Or the Deployments → Logs tab in the dashboard.

---

## Rollback

```bash
# Find the last-known-good deployment in the Railway dashboard and click
# "Redeploy" on it. No automated rollback — if a deploy is bad, revert the
# commit on main and push:
git revert <bad-sha>
git push origin main
```

---

## Resetting admin password in production

```bash
railway run --service scheduler ./scheduler reset-password --username <user>
```

---

## Adding a user in production

Either the CLI:

```bash
railway run --service scheduler ./scheduler create-user --username <x> --email <y>
```

or via the app UI under Account → Users (admin only).

---

## Monitoring

- **Deployments, crashes, restarts**: Railway dashboard
- **Liveness (silent hang)**: Healthchecks.io alerts to email
- **Logs**: `railway logs` or dashboard
- **Readiness**: `curl https://<service>.up.railway.app/healthz` — returns
  200 `{"status":"ok","checks":{"postgres":"ok","rabbitmq":"ok"}}` when
  the scheduler can reach both dependencies; 503 otherwise with per-check
  results.

---

## Resource tier note

Scheduler + 2 Python workers + Postgres + RabbitMQ = 5 services. On
Railway's hobby plan the per-service memory cap can bite, especially
Postgres and RabbitMQ. If services start OOMing (log message: "Killed"
with exit code 137), upgrade the plan or shrink RabbitMQ by switching
to `rabbitmq:3-alpine` without the management UI.

---

## Secret hygiene

- `.env` is gitignored. Local dev only.
- `.env.example` has placeholders, never real values.
- Railway secrets live in the dashboard, never committed.
- `.githooks/pre-commit` scans staged changes for credential patterns.
  Install via `make install-hooks`. CI runs the same scanner on every
  push; bypass with `--no-verify` is allowed but CI still catches leaks
  that would reach main.

---

## Future (not in 5e)

- Custom domain: Settings → Domains → Add custom domain, point CNAME to
  `<service>.up.railway.app`. Railway provisions the Let's Encrypt cert.
- Staging environment: duplicate project, point at a staging branch, use
  Railway's environment variables for per-env overrides.
- Database backups: Railway managed Postgres does daily backups on Pro;
  hobby tier is less clear — verify what you get before you need it.

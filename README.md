# PubMed Literature Alert Service

A self-hosted service that watches PubMed for new papers matching saved queries
and delivers them as a daily email digest. Built as a backend developer capstone
project covering Go, Python, RabbitMQ, PostgreSQL, Docker, and a React frontend.

**Live instance:** [scheduler-production-e532.up.railway.app](https://scheduler-production-e532.up.railway.app)
(invite-only — admin creates accounts via CLI)

## Motivation

I'm an emergency physician in Turkey. ED shifts have stretches of downtime
between cases, and I wanted a way to passively keep up with literature relevant
to my practice — chest pain risk stratification, sepsis recognition, renal
colic, the things I see most days — without having to remember to log into
PubMed and run searches manually.

The existing options didn't fit. PubMed's own email alerts are clunky to manage
and don't filter the way I want (commentary, errata, and veterinary studies
slip through). Aggregator services are aimed at researchers writing reviews,
not clinicians scanning for something useful between patients. Building my own
gave me the filters I wanted (publication-type allow/blocklists, minimum
abstract length, structured-abstract section extraction) and doubled as a
project to consolidate everything I'd been learning across the boot.dev
backend curriculum.

## Quick Start

If you have an account on the live instance:

1. Open [the app](https://scheduler-production-e532.up.railway.app) and log in.
2. Go to **Queries** → **Create query**. Use PubMed's standard search syntax,
   e.g. `("HEART score"[tiab] OR "HEART pathway"[tiab]) AND humans[mh]`. The
   `[tiab]` field tag limits matches to title/abstract; `humans[mh]` excludes
   veterinary papers.
3. Wait a few hours. The scheduler polls PubMed every 6 hours by default, so
   the first batch lands within 6 hours of creating the query.
4. Articles arrive in the **Articles** tab. Each row shows title, journal, an
   abstract snippet, and the queries it matched.
5. The next morning at your configured digest time (default 07:00 in your
   configured timezone), you receive an email summarizing new articles since
   the last digest.

Don't have an account? Open an issue or contact me — accounts are
admin-created. The instance is sized for a handful of users.

## Usage

### Queries

Each query is a PubMed search you want to track. Per-query configuration:

- **Poll interval:** how often to check PubMed for new matches (default 6
  hours, minimum 1 hour to be polite to NCBI).
- **Min abstract length:** filter out conference abstracts, letters, etc. that
  have abstracts under N characters. Default 200.
- **Publication type allowlist:** if set, only matching types pass (e.g.
  `["Journal Article", "Review"]`). If unset, all types pass.
- **Publication type blocklist:** types to exclude. Default blocklist filters
  out `Comment`, `Retraction of Publication`, and `Published Erratum`.
- **Notes:** free-form text for remembering why you added the query.

You can edit, deactivate, or re-poll a query (force re-evaluation against
current PubMed indexing) at any time. Deleting a query removes it and its
match records, but keeps the underlying articles in case other queries
matched them.

### Articles

The Articles tab shows everything matched by your active queries. Filter by
query, search abstracts and titles, and click any row to open a detail drawer
with the full abstract, authors, publication types, and a link to the
canonical PubMed entry.

### Digests

A daily email summarizing new articles since the last digest, grouped by
query. Configurable send time (defaults to 07:00 in your local timezone) and
"send test digest" button for on-demand previews.

If a query is too noisy, tighten its filters and re-poll. If it's too narrow,
broaden the search string. The Articles tab plus the digest history make it
easy to iterate.

### Account

Each user has their own queries, articles, and digest history — no
cross-user leakage. Admins can create users, reset passwords, and toggle
admin status. There's no public registration; accounts are admin-created
to keep the instance focused on a small known group.

## Local Development

If you want to run your own instance or contribute:

### Prerequisites

- Docker 24+ and Docker Compose v2
- Node.js 20+ (for frontend dev)

### Setup

```bash
git clone https://github.com/nysthretha/pubmed_literature_alert_service.git
cd pubmed_literature_alert_service
cp .env.example .env
```

Edit `.env`:
- Set `PUBMED_EMAIL` (required by NCBI).
- Optionally set `PUBMED_API_KEY` (raises rate limit from 3 → 10 req/sec).
- Set `DIGEST_RECIPIENT`, `DIGEST_TIMEZONE`, `DIGEST_SEND_HOUR`.
- `DIGEST_MODE` defaults to `file` (safe — no email sent). Switch to `mailpit`
  or `smtp` when ready.

### Run the backend stack

```bash
docker compose up --build -d
```

Services:
- `scheduler` — Go; polls PubMed, runs migrations, serves `http://localhost:8080`
- `worker` — Python; enrichment (efetch + parse + filter + insert)
- `digest_worker` — Python; renders/sends the daily digest
- `postgres` — schema + data
- `rabbitmq` — queues: `pmid.fetch` (durable), `digest.manual_trigger` (ephemeral)
- `mailpit` — fake SMTP + web UI at `http://localhost:8025` (dev only)

First-time setup requires creating an admin via CLI before migrations can
finish — see [docs/DEPLOY.md](docs/DEPLOY.md) for the bootstrap sequence.

### Frontend dev (M5c onwards)

Two terminals:

```bash
# Terminal 1: backend stack
docker compose up -d

# Terminal 2: Vite dev server on :5173
cd web
npm install         # first time only
npm run dev
```

Open `http://localhost:5173`. The Vite proxy (`vite.config.ts`) forwards
`/api/*` calls to the Go scheduler while keeping the browser's origin as
`localhost:5173` (`changeOrigin: false`) so the SameSite=Strict session
cookie works across both sides.

Production build: `cd web && npm run build` → `web/dist/`. M5e embeds this
into the Go binary via `go:embed` and serves it as static assets.

### Digest modes

Set `DIGEST_MODE` in `.env`:

| mode | behavior |
|---|---|
| `file` *(default)* | Render HTML to `./previews/<timestamp>.html`. No SMTP, no DB writes. Articles stay pending so you can iterate on the template. |
| `mailpit` | Send to the local Mailpit container on port 1025 (no auth). Full DB flow (`digests` row, `digest_articles` inserts). View at <http://localhost:8025>. |
| `smtp` | Send for real via `SMTP_HOST:SMTP_PORT` with STARTTLS. |

For SMTP, [Brevo](https://www.brevo.com)'s free tier (300 emails/day) is a
good fit. Gmail SMTP also works with an app password but has lower deliverability.

### Inspect

```bash
docker compose exec postgres psql -U pubmed -d pubmed -c \
  "SELECT pmid, title, publication_date FROM articles ORDER BY fetched_at DESC LIMIT 10;"

docker compose exec postgres psql -U pubmed -d pubmed -c \
  "SELECT id, sent_at, sent_local_date, articles_included, status, manual FROM digests ORDER BY id DESC LIMIT 10;"
```

Queue status: <http://localhost:15672> (guest / guest in dev).

### Tear down

```bash
docker compose down          # keep data
docker compose down -v       # drop Postgres volume
```

## Architecture

Three services, one queue, one database:

- **Scheduler (Go)** — periodic PubMed polling, HTTP API, embedded React SPA,
  session-based auth (argon2id, HttpOnly+SameSite=Strict cookies).
- **Worker (Python)** — consumes `pmid.fetch`, calls PubMed efetch, parses XML,
  applies filters, inserts articles. Dedup happens before the network call to
  stay polite to NCBI.
- **Digest worker (Python)** — separate service that consumes
  `digest.manual_trigger` and runs a sleep-loop scheduled-time check. Idempotency
  via a partial unique index on `(user_id, sent_local_date)`.

Postgres holds users, queries, articles, query-match join records, sessions,
digests, and digest-article join records. Migrations are embedded via goose.
RabbitMQ separates the fetch lifecycle from the polling lifecycle and lets the
two services scale independently.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for ownership rules,
data-access conventions, and the "no service layer" call.

## Deployment

Production deployment to Railway, with GitHub Actions CI gating main:

- Multi-stage Dockerfile (Node builds frontend, Go embeds + compiles, distroless
  runtime).
- `/healthz` verifies Postgres + RabbitMQ before Railway routes traffic.
- Healthchecks.io liveness ping from the scheduler's main loop catches
  silent-failure cases Railway's own monitoring misses.
- Pre-commit and CI credential scanner shared via `.githooks/scan-diff.sh`.

See [docs/DEPLOY.md](docs/DEPLOY.md) for the full runbook including the
admin-bootstrap sequence required for fresh deployments.

## Project History

Built as the boot.dev backend developer path capstone. Topics covered: Python
(OOP, decorators, data structures), Go (HTTP, concurrency, generics), Docker,
and RabbitMQ pub/sub. Topics added beyond the curriculum: PostgreSQL advanced
features, authentication and session management, React + TanStack + Tailwind v4
frontend, testcontainers for integration tests, GitHub Actions CI, and Railway
deployment.

Milestone log:

- **M1** — walking skeleton (Go publishes heartbeats, Python consumes)
- **M2** — real PubMed ingestion: esearch + efetch, articles stored with dedup
- **M3** — multi-query support: per-query poll intervals, filters, inspection endpoint
- **M4** — daily email digest with scheduled + manual triggers, three delivery modes
- **M5a** — auth backend: argon2id, sessions, login/logout/me, admin CLI
- **M5b** — CRUD/read endpoints scoped per user, admin user management
- **M5c** — frontend scaffolding (Vite + React 19 + TanStack + Tailwind v4 + shadcn)
- **M5d** — full CRUD UI with forms, filters, infinite scroll, and dark mode
- **M5e** — production deploy: multi-stage Dockerfile, CI, /healthz, secret scanner

## Contributing

(coming soon)

## License

MIT

# PubMed Literature Alert Service

Personal capstone project: a service that subscribes to PubMed literature alerts,
enriches the results, and delivers periodic email digests. Stack: Go (scheduler +
HTTP API), Python (enrichment worker + digest worker), RabbitMQ, PostgreSQL.

## Milestones

- **M1** — walking skeleton (Go publishes heartbeats, Python consumes) ✓
- **M2** — real PubMed ingestion: esearch + efetch, articles stored with dedup ✓
- **M3** — multi-query support: per-query poll intervals and filters (min abstract
  length, publication-type allow/blocklists), `GET /articles/recent` endpoint ✓
- **M4** — daily email digest: scheduled + manual-trigger delivery with Jinja HTML
  templates, SMTP with STARTTLS, Mailpit for dev, `POST /digest/trigger` ✓
- **M5a** — auth backend: user accounts, argon2id password hashing, Postgres-backed
  sessions with HttpOnly/SameSite=Strict cookies, login/logout/me endpoints,
  CLI admin bootstrap ✓
- **M5b** — CRUD/read endpoints for queries/articles/digests scoped by user,
  admin endpoints for user management. Ownership enforced via Pattern A
  (userID as mandatory repo arg) — see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) ✓
- **M5c** — frontend scaffolding: Vite + React 19 + TypeScript + TanStack
  Router/Query + Tailwind v4 + shadcn/ui under [web/](web/). Login, authed
  shell with placeholder pages, dark mode default, Sonner toasts ✓
- **M5d** — real CRUD UI. Queries: create/edit/delete/repoll with a pub-type
  combobox. Articles: 2-line table with abstract snippet, search + filters +
  detail drawer. Digests: history list + detail + send-test button. Account:
  profile / change-password / admin Users tab. Backend added
  `POST /api/auth/change-password`. ✓
- **M5e** — production deploy on Railway: multi-stage Dockerfile with embedded
  SPA (build-tag split), `/healthz` with PG+RabbitMQ checks, GitHub Actions
  CI (go-tests / python-check / frontend / secret-scan), pre-commit credential
  scanner, Healthchecks.io liveness ping. See [docs/DEPLOY.md](docs/DEPLOY.md). ✓

## Seed query (applied by migrations)

| name | query_string |
|---|---|
| HEART score | `("HEART score"[tiab] OR "HEART pathway"[tiab]) AND humans[mh]` |

Filters applied: `min_abstract_length=200`; default publication-type blocklist of
`{Comment, Retraction of Publication, Published Erratum}`.

Add more queries manually via `psql` — the schema fields (`poll_interval_seconds`,
`is_active`, filter columns, `notes`) all have sensible defaults.

## Prerequisites

- Docker 24+ and Docker Compose v2

## Setup

```bash
cp .env.example .env
```

Edit `.env`:
- Set `PUBMED_EMAIL` (required by NCBI).
- Optionally set `PUBMED_API_KEY` (raises rate limit from 3 → 10 req/sec).
- Set `DIGEST_RECIPIENT`, `DIGEST_TIMEZONE`, `DIGEST_SEND_HOUR`.
- `DIGEST_MODE` defaults to `file` (safe — no email sent). Switch to `mailpit` or
  `smtp` when ready.

## Run

```bash
docker compose up --build -d
```

Services:
- `scheduler` — Go; polls PubMed, runs migrations, serves `http://localhost:8080`
- `worker` — Python; enrichment (efetch + parse + filter + insert)
- `digest_worker` — Python; renders/sends the daily digest
- `postgres` — schema + data
- `rabbitmq` — queues: `pmid.fetch` (durable), `digest.manual_trigger` (ephemeral)
- `mailpit` — fake SMTP + web UI at `http://localhost:8025` (for dev only)

## Digest modes

Set `DIGEST_MODE` in `.env`:

| mode | behavior |
|---|---|
| `file` *(default)* | Render HTML to `./previews/<timestamp>.html`. No SMTP, no DB writes. Articles stay pending so you can iterate on the template. |
| `mailpit` | Send to the local Mailpit container on port 1025 (no auth). Full DB flow (`digests` row, `digest_articles` inserts). View sent messages at <http://localhost:8025>. |
| `smtp` | Send for real via `SMTP_HOST:SMTP_PORT` with STARTTLS. For Gmail, set `SMTP_USER=<your@gmail>`, `SMTP_PASSWORD=<app password>`. |

### Gmail app password

1. Enable 2-Step Verification on your Google account.
2. Go to <https://myaccount.google.com/apppasswords>, create a new app password
   ("Mail" / custom name). Copy the 16-character string.
3. In `.env`, set:
   - `SMTP_HOST=smtp.gmail.com`
   - `SMTP_PORT=587`
   - `SMTP_USER=your-address@gmail.com`
   - `SMTP_PASSWORD=<16-char app password>`
   - `SMTP_FROM=your-address@gmail.com`
   - `DIGEST_MODE=smtp`

## HTTP endpoints (bound to 127.0.0.1:8080)

Auth (public):
- `POST /api/auth/login` — body `{"username":"...","password":"..."}` → sets session cookie
- `POST /api/auth/logout` — clears session
- `GET /api/auth/me` — returns `{"user":{...}}` for the current session, 401 otherwise

Queries (auth required — scoped to current user):
- `GET /api/queries` — list own queries with `article_count`
- `POST /api/queries` — create; body `{name, query_string, ...}` (see [queries/handlers.go](scheduler/internal/queries/handlers.go) for full shape)
- `GET /api/queries/{id}` · `PATCH /api/queries/{id}` · `DELETE /api/queries/{id}`
- `POST /api/queries/{id}/repoll` — clear `last_polled_at` to trigger a re-poll next tick

Articles (auth required — scoped via `query_matches`):
- `GET /api/articles?limit=50&offset=0&query_id=N&since=<rfc3339>&search=<text>`
- `GET /api/articles/{pmid}`

Digests (auth required — scoped to current user):
- `GET /api/digests` · `GET /api/digests/{id}` (detail includes per-article `matched_queries`)

Admin (auth + admin required):
- `GET /api/admin/users` · `POST /api/admin/users` · `PATCH /api/admin/users/{id}`
- `DELETE /api/admin/users/{id}` (400 if self)
- `POST /api/admin/users/{id}/reset-password`

Operational:
- `POST /digest/trigger` — enqueue a manual digest run

**First-time setup** requires creating an admin via CLI — see
[docs/DEPLOY.md](docs/DEPLOY.md) for the bootstrap sequence.

```bash
curl -s 'http://localhost:8080/articles/recent?limit=10' | jq '.articles[] | {pmid, title, matched: [.matched_queries[].name]}'
curl -X POST http://localhost:8080/digest/trigger
```

Manual triggers bypass the scheduled-hour check but still respect "already sent
today" (scheduled-path only) and "has pending articles". In `file` mode neither
is checked — the render always happens.

## Inspect

```bash
docker compose exec postgres psql -U pubmed -d pubmed -c \
  "SELECT pmid, title, publication_date FROM articles ORDER BY fetched_at DESC LIMIT 10;"

docker compose exec postgres psql -U pubmed -d pubmed -c \
  "SELECT id, sent_at, sent_local_date, articles_included, status, manual FROM digests ORDER BY id DESC LIMIT 10;"
```

Queue status: <http://localhost:15672> (guest / guest).

## Tear down

```bash
docker compose down          # keep data
docker compose down -v       # drop Postgres volume
```

## Frontend dev (M5c onwards)

Two terminals:

```bash
# Terminal 1: backend stack (Postgres, RabbitMQ, Go scheduler on :8080, workers)
docker compose up -d

# Terminal 2: Vite dev server on :5173, proxies /api/* to :8080
cd web
npm install         # first time only
npm run dev
```

Open `http://localhost:5173`. The Vite proxy (`vite.config.ts`) forwards
`/api/*` calls to the Go scheduler while keeping the browser's origin as
`localhost:5173` (`changeOrigin: false`) so the SameSite=Strict session
cookie works across both sides.

Dark mode is the default; toggle in the header. Theme persists in
`localStorage['pubmed-theme']`.

Production build: `cd web && npm run build` → `web/dist/`. M5e will embed
this into the Go binary via `go:embed` and serve it as static assets.

## Layout

```
pubmed_literature_alert_service/
├── docker-compose.yml
├── .env.example
├── previews/                        # (gitignored) digest HTML in file mode
├── scheduler/                       # Go
│   ├── main.go
│   ├── migrations/*.sql             # goose migrations (embedded)
│   ├── internal/
│   │   ├── pubmed/                  # esearch client + rate limiter
│   │   ├── store/                   # queries / migrations
│   │   ├── publisher/               # pmid.fetch + digest.manual_trigger
│   │   ├── poller/                  # per-tick orchestration
│   │   └── httpapi/                 # GET /articles/recent, POST /digest/trigger
│   └── Dockerfile
└── worker/
    ├── pyproject.toml
    ├── Dockerfile                   # enrichment worker
    ├── Dockerfile.digest            # digest worker
    └── src/worker/
        ├── __main__.py              # enrichment entry
        ├── consumer.py / pipeline.py / parser.py / pubmed_client.py / filters.py
        ├── db.py / logging_setup.py
        └── digest/
            ├── __main__.py          # digest entry
            ├── scheduler.py         # sleep loop + manual trigger consumer
            ├── builder.py           # group articles by query
            ├── renderer.py          # Jinja2
            ├── sender.py            # FileSender / SMTPSender
            ├── db.py                # digest-specific SQL
            └── templates/digest.html.j2
```

## Notes

**Rate limits.** Each service has its own limiter (3 or 10 req/s depending on
`PUBMED_API_KEY`). With one worker we're fine; a parallel-worker deployment
would need a shared limiter.

**Horizontal scaling.** `DueQueries()` uses a single-scheduler pattern. To scale
to multiple schedulers, switch to `SELECT ... FOR UPDATE SKIP LOCKED` and update
`last_polled_at` in the same transaction to prevent double-polling (comment in
[scheduler/internal/store/store.go](scheduler/internal/store/store.go)).

**Digest idempotency.** The partial unique index `digests_one_sent_per_day`
(on `sent_local_date WHERE status='sent'`) guarantees at most one successful
scheduled digest per local date. Manual triggers respect the same constraint.

**RabbitMQ state.** The current compose does not mount a RabbitMQ volume — if
RabbitMQ restarts, in-flight `pmid.fetch` messages are lost. In practice the
scheduler re-fetches on its next due-poll; the worker's dedup path handles
already-stored PMIDs. Not ideal but acceptable for a single-user deployment.

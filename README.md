# PubMed Literature Alert Service

Personal capstone project: a service that subscribes to PubMed literature alerts,
enriches the results, and delivers periodic digests. Stack: Go (scheduler + HTTP API),
Python (enrichment / digest workers), RabbitMQ, PostgreSQL.

## Milestone 2 — Real PubMed Ingestion

The walking skeleton has been replaced with a real polling pipeline:

- **scheduler** (Go) runs migrations on startup (via embedded [goose](https://github.com/pressly/goose)), then on each tick reads all rows from `queries`, calls PubMed **esearch** for each, and publishes every returned PMID to the `pmid.fetch` queue.
- **worker** (Python) consumes `pmid.fetch`. For each PMID: if already in `articles`, just inserts a `query_matches` row (dedup fast-path); otherwise calls **efetch**, parses the XML with `lxml`, and inserts into `articles` + `query_matches` in a single transaction.
- **postgres** holds the schema (`queries`, `articles`, `query_matches`) managed by goose migrations under [scheduler/migrations/](scheduler/migrations/).

No email digest yet — that's a later milestone.

### Seed query (applied by migration)

| name | query_string |
|---|---|
| HEART score | `("HEART score"[tiab] OR "HEART pathway"[tiab])` |

## Prerequisites

- Docker 24+ and Docker Compose v2

## Setup

```bash
cp .env.example .env
```

**You must set `PUBMED_EMAIL`** — NCBI requires contact info on every E-utilities request and may block traffic without it.
Optionally request an API key at <https://www.ncbi.nlm.nih.gov/account/> and set `PUBMED_API_KEY` to raise the rate limit from 3 req/sec to 10 req/sec.

## Run

```bash
docker compose up --build
```

First run polls the last 30 days of results for the seed query; subsequent runs poll from `last_polled_at` to now (EDAT — entrez index date). Default poll interval is 6 hours.

## Verify

Within ~1 minute of startup, logs should include:

```
scheduler | {"level":"INFO","msg":"migrations applied",...}
scheduler | {"level":"INFO","msg":"poll start","query_id":1,"name":"HEART score","mindate":"2026/03/24","maxdate":"2026/04/23"}
scheduler | {"level":"INFO","msg":"poll end","query_id":1,"name":"HEART score","total_reported":N,"published":N,...}
worker    | {"level":"INFO","msg":"efetch ok","pmid":"...","duration_ms":...}
worker    | {"level":"INFO","msg":"stored","pmid":"...","new":true,"title":"..."}
```

Re-running (`docker compose restart scheduler`) should show `dedup` instead of `stored` for previously-seen PMIDs.

Inspect the data:

```bash
docker compose exec postgres psql -U pubmed -d pubmed -c "SELECT pmid, title, publication_date FROM articles ORDER BY fetched_at DESC LIMIT 10;"
docker compose exec postgres psql -U pubmed -d pubmed -c "SELECT q.name, COUNT(*) FROM query_matches qm JOIN queries q ON q.id = qm.query_id GROUP BY q.name;"
```

RabbitMQ management UI: <http://localhost:15672> (guest / guest). Expect to see the `pmid.fetch` queue.

## Tear down

```bash
docker compose down -v
```

The `-v` flag drops the Postgres volume; omit to keep state.

## Layout

```
pubmed_literature_alert_service/
├── docker-compose.yml
├── .env.example
├── scheduler/                    # Go
│   ├── main.go                   # wiring + ticker loop
│   ├── migrations/               # goose .sql files (embedded)
│   │   └── 00001_init.sql
│   ├── internal/
│   │   ├── pubmed/               # esearch client + rate limiter
│   │   ├── store/                # queries table, goose runner
│   │   ├── publisher/            # RabbitMQ publish wrapper
│   │   └── poller/               # per-tick orchestration
│   ├── go.mod / go.sum
│   └── Dockerfile
└── worker/                       # Python
    ├── pyproject.toml            # deps + entry point
    └── src/
        └── pubmed_worker/
            ├── main.py           # consumer loop
            ├── pubmed.py         # efetch + lxml parse
            ├── db.py             # SQL inserts
            ├── ratelimit.py
            └── logging_setup.py
```

## Rate limiting note

Each service maintains its own rate limiter (3 req/sec without key, 10 req/sec with). If you add parallel workers later, the aggregate rate against NCBI will exceed the per-service cap. At that point, move the limiter behind a shared gate (e.g. Redis token bucket) or a dedicated fetch-proxy service. For M2 with a single worker, per-service is fine.

## What's next

- **M3**: HTTP API on the Go side — CRUD for queries, browsing articles, triggering a one-off poll.
- **M4**: Email digest — scheduled delivery of new matches per query, with templating.
- **M5**: Dead-letter queue + bounded retries; observability (OpenTelemetry or at least metrics).

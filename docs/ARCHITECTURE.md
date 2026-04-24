# Architecture

Short reference for the conventions enforced across the scheduler code.

## Ownership enforcement (M5b onwards)

Every repository method that reads or writes a user-scoped table
(`queries`, `query_matches`, `digests`, `digest_articles`, and any future
per-user tables) **must** take `userID int64` as the first parameter
after `ctx`, and its SQL must include `AND user_id = $N` (or an
equivalent `user_id`-constrained JOIN).

Admin-bypass methods — those that legitimately read or write across
users — are prefixed `Admin*` and colocated in a dedicated file within
the owning package (e.g. `auth/admin.go`). This makes the scoping
bypass visible at every call site:

```go
queries.List(ctx, db, user.ID)         // scoped — always safe
queries.AdminListAll(ctx, db)          // bypass — caller takes
                                       // responsibility for proving
                                       // the request is admin-authorised
```

Never mix scoped and bypass logic in the same function. Never accept a
`userID` parameter you don't use — if the function can legitimately
ignore it, it's an `Admin*` method.

## Test obligation

Every user-scoped endpoint needs an **ownership isolation test**: user
A calling `GET`/`PATCH`/`DELETE` on user B's resource must return 404.
Not 403 (that leaks existence). Not 500 (unhandled). Not 200 with B's
data (catastrophic). These tests catch the Pattern A failure mode — a
repository function that accepts `userID` but forgets to use it in the
WHERE clause.

## Error responses

Standard shape for all 4xx/5xx JSON responses:

```json
{
  "error": {
    "code": "validation_error",
    "message": "request has 2 validation errors",
    "fields": [
      {"field": "name", "message": "is required"},
      {"field": "poll_interval_seconds", "message": "must be >= 3600"}
    ]
  }
}
```

`fields` is omitted for non-validation errors. Codes in use:
`validation_error`, `not_found`, `conflict`, `forbidden`, `unauthorized`,
`internal_error`.

## Layering

`Handler → Store → Postgres`. No service layer. If business logic
accretes beyond what fits into a handler or store method, surface it in
review — don't pre-emptively add an intermediate layer.

## Validation

Hand-rolled `Validator` helper at `internal/validation/validator.go`.
Per-request input structs expose a `Validate() *ValidationErrors`
method that uses the helper to collect all errors in one pass.
Database-dependent checks (unique name, existence of referenced ID)
happen in the handler **after** `Validate()` returns nil — catch the
unique-violation or foreign-key error from the DB and map to 409 or
400 as appropriate.

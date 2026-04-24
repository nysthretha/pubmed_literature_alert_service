# Deployment & Bootstrap

## First-time setup (fresh database or upgrading from M4 → M5a)

Milestone 5a splits authentication migrations into two phases because the
second migration (`00006_scope_queries_digests_to_users`) needs to know which
user to assign existing queries/digests to — it aborts with an actionable
error if no admin exists. So the sequence is:

1. **Bring up infrastructure.**
   ```bash
   docker compose up -d postgres rabbitmq
   ```

2. **Let the scheduler apply migrations up through `00005_add_users_sessions.sql`.**
   The next migration (`00006`) will fail with:
   > ERROR: No admin user exists. Run `./scheduler create-admin --username X
   > --email Y` before applying this migration.

   You'll see this log from the scheduler container (it will exit / restart
   until an admin exists). That's expected.

   ```bash
   docker compose up -d scheduler
   docker compose logs scheduler   # confirm you see the "no admin user" message
   ```

3. **Create the admin user.**
   ```bash
   docker compose run --rm scheduler create-admin \
       --username ahmet \
       --email your-email@example.com
   # prompts twice for password (min 8 chars)
   ```

4. **Start everything.**
   ```bash
   docker compose up -d
   ```

   The scheduler's next restart attempt finds the admin, applies migration
   `00006` (which backfills `queries.user_id` and `digests.user_id` from the
   admin), and the app runs normally.

## Useful CLI commands

All CLI subcommands of `./scheduler` connect to Postgres via `POSTGRES_URL`
and prompt interactively for passwords. They **do not** run migrations —
they only touch the `users` table.

```bash
# Create an admin user (same command as bootstrap; can add more admins later)
docker compose run --rm scheduler create-admin --username X --email Y

# Create a non-admin user
docker compose run --rm scheduler create-user --username X --email Y

# Reset a user's password
docker compose run --rm scheduler reset-password --username X

# Show available commands
docker compose run --rm scheduler help
```

## Auth-related environment variables

| var | default | notes |
|---|---|---|
| `AUTH_COOKIE_SECURE` | `true` | Set to `false` for local development without HTTPS. Logs a startup WARN when disabled. **Always `true` in production.** |

Session and idle timeouts are compile-time constants in `internal/auth/auth.go`
(`SessionTTL=30d`, `SessionIdleTimeout=7d`).

## API smoke test (via curl)

```bash
# Login
curl -i -c cookies.txt -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"ahmet","password":"..."}'

# Authenticated endpoint
curl -i -b cookies.txt http://localhost:8080/api/auth/me

# Logout
curl -i -b cookies.txt -c cookies.txt -X POST http://localhost:8080/api/auth/logout

# After logout, /me should 401
curl -i -b cookies.txt http://localhost:8080/api/auth/me
```

The cookie jar pattern (`-c cookies.txt -b cookies.txt`) is what lets curl
behave like a browser across requests.

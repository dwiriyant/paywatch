# PayWatch

[![CI](https://github.com/dwiriyant/paywatch/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/dwiriyant/paywatch/actions/workflows/ci-cd.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Open-source multi-tenant payment watcher. Register each **qrisgate app → GoBiz account** via admin API (DB-backed). Polls providers and notifies qrisgate `Claim(app_id)` so that app’s webhooks fire.

```
Admin API ─→ tenants (Postgres)
                ↓
         paywatch poller ─→ qrisgate Claim(app_id) ─→ app webhooks
```

Go 1.26+. **One replica.** `POLL_INTERVAL_MS >= 5000`.

> [!WARNING]
> Unofficial GoBiz APIs. Aggressive polling can get accounts banned.
> On auth failure (refresh/login), that tenant is **auto-disabled**. Re-verify with password, then `PATCH` `"enabled": true`.

**Passwords are never stored.** Create/verify with email+password exchanges them for `access_token` + `refresh_token` (saved). After a successful verify, you have **60 seconds** to create/update the tenant with the same email+password without hitting GoBiz again. Polling uses tokens; expired access tokens are refreshed automatically.

## Quick start

```bash
cp .env.example .env
make docker-up
export ADMIN_TOKEN=dev-admin-token

# 1) Verify GoBiz credentials (caches tokens for 60s — save next without a second login)
curl -s -X POST http://localhost:8081/v1/tenants/verify \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "gobiz",
    "gobiz": {
      "login_method": "password",
      "email": "merchant@example.com",
      "password": "secret"
    }
  }'

# 2) Register tenant within 60s using the same email+password (reuses verify; no second GoBiz login)
curl -s -X POST http://localhost:8081/v1/tenants \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "YOUR_QRISGATE_APP_ID",
    "name": "Shop A",
    "provider": "gobiz",
    "gobiz": {
      "login_method": "password",
      "email": "merchant@example.com",
      "password": "secret"
    }
  }'

# 3) Optional: re-check stored tokens (refreshes if needed)
# curl -s -X POST http://localhost:8081/v1/tenants/$TENANT_ID/verify -H "Authorization: Bearer $ADMIN_TOKEN"

# 4) Enable polling (right away is fine — no extra GoBiz login; uses tokens saved in step 2)
curl -s -X PATCH http://localhost:8081/v1/tenants/$TENANT_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled":true}'
```

## API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/healthz` | — | Liveness |
| POST | `/v1/tenants/verify` | Bearer admin | Live-check GoBiz email/password (or token); rate-limited |
| POST | `/v1/tenants` | Bearer admin | Register tenant (**disabled by default**; password → tokens) |
| GET | `/v1/tenants` | Bearer admin | List (secrets masked) |
| GET | `/v1/tenants/:id` | Bearer admin | Get one |
| PATCH | `/v1/tenants/:id` | Bearer admin | Update / enable / disable |
| POST | `/v1/tenants/:id/verify` | Bearer admin | Validate/refresh stored tokens; rate-limited |
| DELETE | `/v1/tenants/:id` | Bearer admin | Delete |

New tenants and credential updates stay **disabled** until you set `"enabled": true`. Enabled tenants are picked up on the next poll cycle (no restart).

Verify → save: complete create/update with the same password within **60s** of verify (`save_window_sec` in the verify response). After that window, save performs a fresh GoBiz login.

Enable polling anytime after create — it only sets `enabled: true` in the DB and does not count toward the verify rate limit.

Verify endpoints share a per-IP rate limit (`VERIFY_RATE_LIMIT_PER_MIN`, default **1**/min) and return `429` when exceeded. Create/update are not rate-limited by that counter.

## Docker Hub

Image: `dwiriyant/paywatch` — see Releases. Secrets: `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`.

```bash
export PAYWATCH_TAG=0.1.5 ADMIN_TOKEN=… QRISGATE_ADMIN_TOKEN=…
make docker-hub-up
```

## Development

```bash
make test
make lint
make run   # needs DATABASE_URL + migrations
```

## License

[MIT](LICENSE)

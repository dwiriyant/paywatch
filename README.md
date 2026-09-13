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
> On login failure (wrong password / temp lockout), that tenant is **auto-disabled** so polls stop. Fix credentials, then `PATCH /v1/tenants/:id` with `"enabled": true`.

## Quick start

```bash
cp .env.example .env
make docker-up
export ADMIN_TOKEN=dev-admin-token

# 1) Verify GoBiz credentials (no tenant created, no polling)
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

# 2) Register tenant (saved disabled — will not poll yet)
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

# 3) Optional: re-check stored credentials
# curl -s -X POST http://localhost:8081/v1/tenants/$TENANT_ID/verify -H "Authorization: Bearer $ADMIN_TOKEN"

# 4) Enable polling
curl -s -X PATCH http://localhost:8081/v1/tenants/$TENANT_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled":true}'
```

## API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/healthz` | — | Liveness |
| POST | `/v1/tenants/verify` | Bearer admin | Live-check GoBiz email/password (or token) |
| POST | `/v1/tenants` | Bearer admin | Register tenant (**disabled by default**) |
| GET | `/v1/tenants` | Bearer admin | List (secrets masked) |
| GET | `/v1/tenants/:id` | Bearer admin | Get one |
| PATCH | `/v1/tenants/:id` | Bearer admin | Update / enable / disable |
| POST | `/v1/tenants/:id/verify` | Bearer admin | Live-check stored credentials |
| DELETE | `/v1/tenants/:id` | Bearer admin | Delete |

New tenants and credential updates stay **disabled** until you set `"enabled": true`. Enabled tenants are picked up on the next poll cycle (no restart).

## Docker Hub

Image: `dwiriyant/paywatch` — see Releases. Secrets: `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`.

```bash
export PAYWATCH_TAG=0.1.0 ADMIN_TOKEN=… QRISGATE_ADMIN_TOKEN=…
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

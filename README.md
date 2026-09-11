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

## Quick start

```bash
cp .env.example .env
make docker-up
export ADMIN_TOKEN=dev-admin-token

# Register tenant (qrisgate app_id + GoBiz login)
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
```

## API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/healthz` | — | Liveness |
| POST | `/v1/tenants` | Bearer admin | Register tenant |
| GET | `/v1/tenants` | Bearer admin | List (secrets masked) |
| GET | `/v1/tenants/:id` | Bearer admin | Get one |
| PATCH | `/v1/tenants/:id` | Bearer admin | Update / disable |
| DELETE | `/v1/tenants/:id` | Bearer admin | Delete |

Enabled tenants are picked up on the next poll cycle (no restart).

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

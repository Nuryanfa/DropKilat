# Backend — DropKilat

## 1. Module layout

Each package under `internal/` owns its tables and exposes a small service. Packages talk to each other through these services, never through each other's SQL.

| Package | Owns | Exposes |
| --- | --- | --- |
| `seller` | `sellers`, `seller_sessions`, `otp_challenges` (seller_login) | `Login`, `VerifyOTP`, `Session`, `UpdateProfile`, `SetPaymentDestination` |
| `trust` | `seller_social_links`, `seller_stats`, `reports`, verification fields on `sellers` | `Badges(sellerID)`, `CanOpenDrop`, `LinkSocial`, `Report`, `RecordOutcome` |
| `drop` | `drops`, `drop_variants`, `drop_photos` | `Create`, `Update`, `Publish`, `Close`, `PublicView`, `LiveCounters` |
| `buyer` | `buyers`, `buyer_sessions`, `buyer_verifications`, `otp_challenges` (buyer_verify) | `StartVerification`, `ResolveInboundCode`, `Session` |
| `order` | `orders` | `Claim`, `SubmitProof`, `Confirm`, `Reject`, `Cancel`, `SetAddress`, `Ship`, `MarkReceived`, `ExpireDue` |
| `waitlist` | `waitlist_entries` | `Enqueue`, `Position`, `Leave`, `PromoteNext(tx, variantID)` |
| `payment` | `payments`, `webhook_events` (xendit) | `Provider` interface; `HandleWebhook` |
| `notify` | `outbox`, `webhook_events` (whatsapp) | `Enqueue(tx, Message)`, `Sender` interface, outbox worker |
| `platform/*` | — | `db` (pool, `WithSellerTx`, `WithSystemTx`, `WithPublicTx`), `httpx` (middleware, errors), `config`, `clock` |

Rules:
- Every state change runs inside a transaction obtained from `platform/db`; the helper sets the RLS context.
- `order.Claim` calls `waitlist.Enqueue` and `notify.Enqueue` with the **same** `pgx.Tx`.
- Time comes from `clock.Clock` (fake in tests). Never call `time.Now()` in domain code.
- Money is `int64` rupiah; no `float64` in money paths (lint rule).

## 2. Key interfaces

```go
// payment
type Provider interface {
    // CreateCharge returns what the buyer page needs (QRIS string or manual instructions).
    CreateCharge(ctx context.Context, o Order, expiresAt time.Time) (Charge, error)
    // ParseWebhook verifies authenticity and returns a normalized event.
    ParseWebhook(r *http.Request) (Event, error)
    Refund(ctx context.Context, p Payment, reason string) error
}

// notify
type Sender interface {
    Send(ctx context.Context, to Phone, msg Message) (providerID string, err error)
}

// clock
type Clock interface{ Now() time.Time }
```

Implementations: `payment/manual`, `payment/xendit`, `payment/fake`; `notify/whatsapp`, `notify/fake`.

## 3. HTTP API

JSON APIs return `application/json`; errors use RFC 9457 `application/problem+json` with a stable `type` slug. HTML pages are server-rendered with templ.

### Public (buyer)

| Method | Path | Purpose | Auth |
| --- | --- | --- | --- |
| GET | `/d/{shop}/{drop}` | Drop page (HTML) | — |
| GET | `/api/drops/{dropID}/stream` | SSE: `stock`, `status`, `order` events | — (order events need buyer session) |
| POST | `/api/buyer/verify/start` | Returns `{code, wa_link, nonce}` | — (rate limited) |
| GET | `/api/buyer/verify/{nonce}` | Poll status; sets buyer cookie when verified | nonce |
| POST | `/api/buyer/verify/otp` | Fallback: send OTP / check OTP | — |
| POST | `/api/drops/{dropID}/claim` | Body `{variant_id, qty}` → `{result: "held"|"waitlisted", order?, position?}` | buyer |
| GET | `/o/{code}` | Order page (HTML) | buyer (owner) |
| POST | `/api/orders/{code}/proof` | Multipart upload (manual mode) | buyer (owner) |
| POST | `/api/orders/{code}/cancel` | Cancel while `held` | buyer (owner) |
| PUT | `/api/orders/{code}/address` | Shipping address | buyer (owner), status `paid` |
| POST | `/api/orders/{code}/received` | Confirm receipt | buyer (owner), status `shipped` |
| POST | `/api/orders/{code}/report` | Report a problem | buyer (owner) |
| DELETE | `/api/waitlist/{entryID}` | Leave waitlist | buyer (owner) |

### Seller dashboard (`/seller/*`, cookie session + CSRF)

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/seller/login` · `/seller/login/verify` | WA OTP login |
| GET/PUT | `/seller/profile` | Shop name, slug, timezone, payment destination |
| POST | `/seller/social-links` · `/seller/social-links/{id}/check` | Link IG/TikTok |
| GET/POST | `/seller/drops` | List / create |
| GET/PUT | `/seller/drops/{id}` | Edit (draft only, except stock increase on live) |
| POST | `/seller/drops/{id}/publish` · `/unpublish` · `/close` | Lifecycle |
| GET | `/seller/drops/{id}/live` | Live monitor (HTML + SSE `/seller/drops/{id}/stream`) |
| GET | `/seller/orders?status=&drop=` | Orders list |
| POST | `/seller/orders/{id}/confirm` · `/reject` | Manual payment decision |
| POST | `/seller/orders/{id}/ship` · `/seller/orders/ship-bulk` | Tracking numbers |
| GET | `/seller/drops/{id}/export.csv` · `/labels` | Export, print labels |

### Webhooks

| Method | Path | Verification |
| --- | --- | --- |
| POST | `/webhooks/xendit` | `x-callback-token` header, constant-time compare |
| GET/POST | `/webhooks/whatsapp` | GET: verify token challenge. POST: `X-Hub-Signature-256` HMAC of body with app secret |

Webhook handlers only persist the event and enqueue processing in the same request; they answer 200 within 2 s.

### Operations

`/healthz`, `/readyz`, `/metrics` (bound to the internal interface only).

## 4. Error types

| `type` | HTTP | Meaning |
| --- | --- | --- |
| `drop-not-live` | 409 | Drop not open yet or closed |
| `sold-out` | 200 | Not an error: response is `waitlisted` |
| `already-claimed` | 409 | Max per buyer reached |
| `not-verified` | 401 | Buyer must verify WhatsApp |
| `rate-limited` | 429 | With `Retry-After` |
| `hold-expired` | 409 | Proof uploaded after hold ended |
| `invalid-transition` | 409 | Order not in a status that allows the action |
| `seller-not-verified` | 403 | Seller cannot publish before identity verification |
| `new-seller-limit` | 422 | Stock or value above new-seller limits |
| `validation` | 422 | Field errors in `errors[]` |

## 5. Background workers

| Worker | Interval | Work | Concurrency |
| --- | --- | --- | --- |
| `expirer` | 2 s | Expire due holds, promote waitlist (`docs/database-schema.md` §6) | 2 goroutines, `SKIP LOCKED` |
| `scheduler` | 5 s | `scheduled → live` at `opens_at`; `live → closed` at `closes_at`; auto-complete shipped orders after 7 days | 1 |
| `webhooks` | 1 s | Process unprocessed `webhook_events` | 2, `SKIP LOCKED` |
| `outbox` | 1 s | Deliver due messages; backoff 5 s, 30 s, 2 min, 10 min, 1 h; dead after 6 attempts | 4, `SKIP LOCKED` |
| `retention` | daily 03:00 WIB | Privacy deletes (`docs/database-schema.md` §8) | 1 |

All workers stop gracefully on SIGTERM (finish the current transaction, then exit).

## 6. SSE events

| Stream | Event | Data |
| --- | --- | --- |
| `/api/drops/{id}/stream` | `stock` | `{variant_id, available, total}` (coalesced, max 1 per second per variant) |
| | `status` | `{drop_status}` |
| | `order` | `{code, status, hold_expires_at}` (only for the session's buyer) |
| `/seller/drops/{id}/stream` | `counters` | `{available, held, proof_submitted, paid, waiting}` |
| | `event` | Feed line for the live monitor |

Implementation: PostgreSQL `LISTEN/NOTIFY` on channel `drop_<id>` fan-out to SSE clients in-process. Clients reconnect with `Last-Event-ID`.

## 7. WhatsApp messages (templates)

| Kind | When | Window |
| --- | --- | --- |
| `wa.verify_ok` | Buyer code received | Inside service window (free) |
| `wa.payment_link` | Hold created | Inside window if buyer verified in last 24 h, else utility template |
| `wa.payment_reminder` | 5 min before hold end | Same |
| `wa.waitlist_turn` | Promoted from waitlist | Same |
| `wa.paid` / `wa.rejected` | Payment decided | Same |
| `wa.shipped` | Tracking number entered | Utility template (often > 24 h) |
| `wa.seller_otp` | Seller login | Authentication template |

`notify` decides template vs free-form by checking the buyer's last inbound message time.

## 8. Configuration (`DK_*`)

| Variable | Example | Notes |
| --- | --- | --- |
| `DK_ENV` | `dev` / `prod` | |
| `DK_HTTP_ADDR` | `:8080` | |
| `DK_BASE_URL` | `https://dropkilat.id` | Used in links |
| `DK_DATABASE_URL` | `postgres://dropkilat_app:...@db:5432/dropkilat` | App role, RLS applies |
| `DK_MIGRATE_DATABASE_URL` | `postgres://dropkilat:...@db:5432/dropkilat` | Owner role, migrations only |
| `DK_ENCRYPTION_KEY` | 32-byte base64 | AES-256-GCM for PII |
| `DK_HASH_KEY` | 32-byte base64 | HMAC for `*_hash` columns |
| `DK_UPLOAD_DIR` | `/data/uploads` | |
| `DK_FEATURE_XENDIT` | `false` | |
| `DK_XENDIT_SECRET_KEY`, `DK_XENDIT_CALLBACK_TOKEN`, `DK_XENDIT_SPLIT_RULE_ID` | | Required if feature on |
| `DK_FEATURE_WA_SEND` | `false` | false → fake sender logs messages |
| `DK_WA_PHONE_NUMBER_ID`, `DK_WA_ACCESS_TOKEN`, `DK_WA_APP_SECRET`, `DK_WA_VERIFY_TOKEN`, `DK_WA_BUSINESS_NUMBER` | | |
| `DK_OWNER_PHONES` | `+62812...` | Owner admin access |

Startup fails fast if a required variable is missing or a key has the wrong length.

## 9. Code conventions

- Go 1.27, `gofmt`, `go vet`, `staticcheck`, `golangci-lint` (config in repo).
- SQL in `internal/<pkg>/queries.sql`, generated with `sqlc`; no string-built SQL.
- Errors: wrap with `%w`; domain errors are typed (`order.ErrSoldOut`, ...) and mapped to problem types in `httpx`.
- Logging: `slog` with `request_id`, `seller_id`, `order_code`; phones masked (`+62812****789`).
- Tests next to code (`_test.go`); integration tests use testcontainers PostgreSQL and the real migrations.

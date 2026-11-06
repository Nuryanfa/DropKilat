# App flow — DropKilat

Diagrams are Mermaid; they render on GitHub. State names here are the exact values stored in the database.

## 1. Seller onboarding

```mermaid
flowchart TD
  A[Enter WA number] --> B[OTP sent via WA]
  B --> C{OTP correct?}
  C -- no, < 5 tries --> B
  C -- yes --> D[Level 1: phone_verified]
  D --> E[Fill shop name and slug]
  E --> F{Payment mode?}
  F -- manual --> G[Upload QRIS image / bank account + KTP name]
  G --> H[Owner reviews: name on KTP = account name]
  F -- xendit --> I[Create sub-account via API]
  I --> J[Seller completes KYC at Xendit]
  J --> K[Webhook: sub-account verified]
  H --> L[Level 2: identity_verified]
  K --> L
  L --> M[Optional: link IG/TikTok with bio code]
  M --> N[Level 3: social_linked]
  L --> O[Can open drops]
```

## 2. Drop lifecycle

```mermaid
stateDiagram-v2
  [*] --> draft
  draft --> scheduled: publish (seller verified)
  scheduled --> draft: unpublish (before opens_at)
  scheduled --> live: scheduler at opens_at
  live --> closed: closes_at reached / all paid and waitlist empty / seller closes
  scheduled --> cancelled: seller cancels
  closed --> [*]
  cancelled --> [*]
```

Rules: a `live` drop can only gain stock or be closed; variants and prices are frozen once `scheduled`.

## 3. Buyer verification (buyer-initiated WhatsApp)

```mermaid
sequenceDiagram
  participant P as Buyer page
  participant A as DropKilat API
  participant WA as Buyer's WhatsApp
  participant M as Meta Cloud API
  P->>A: POST /api/buyer/verify/start
  A-->>P: code DK-7Q2M, wa.me link
  P->>WA: open wa.me/<number>?text=DK-7Q2M
  WA->>M: buyer sends "DK-7Q2M"
  M->>A: POST /webhooks/whatsapp (from, text)
  A->>A: match code, bind phone to pending verification
  A->>M: reply "Nomor terverifikasi" (inside 24h window)
  P->>A: poll / SSE: verification status
  A-->>P: verified, set buyer session cookie
```

Fallback when the buyer cannot open WA on this device: OTP template sent to the typed number.

## 4. Claim

```mermaid
flowchart TD
  A[Buyer taps Ambil] --> B{Drop live and buyer verified?}
  B -- no --> X[Show reason]
  B -- yes --> C{Rate limit ok?}
  C -- no --> X
  C -- yes --> D[TX: advisory lock drop+buyer]
  D --> E{Active orders < max_per_buyer?}
  E -- no --> X2[You already have this item]
  E -- yes --> F{UPDATE available >= qty}
  F -- 1 row --> G[INSERT order held, hold_expires_at]
  G --> H[Outbox: WA payment link]
  H --> I[Return hold + payment instructions]
  F -- 0 rows --> J{Buyer already waiting?}
  J -- yes --> K[Return position]
  J -- no --> L[INSERT waitlist_entry waiting]
  L --> K
```

## 5. Order state machine

```mermaid
stateDiagram-v2
  [*] --> held: claim / waitlist offer
  held --> proof_submitted: buyer uploads proof (manual)
  held --> paid: webhook paid (xendit)
  held --> expired: hold_expires_at + grace passed
  held --> cancelled: buyer cancels
  proof_submitted --> paid: seller confirms
  proof_submitted --> rejected: seller rejects
  proof_submitted --> cancelled: review window passed and buyer cancels
  expired --> paid: late payment AND stock re-reserved
  expired --> refund_pending: late payment AND no stock
  paid --> shipped: seller enters tracking number
  paid --> refund_pending: seller cancels
  shipped --> completed: buyer confirms / 7 days auto
  shipped --> disputed: buyer reports
  disputed --> completed: resolved
  disputed --> refund_pending: resolved with refund
  refund_pending --> refunded: refund done
  completed --> [*]
  refunded --> [*]
  expired --> [*]
  cancelled --> [*]
  rejected --> [*]
```

Stock effects:
- Entering `held` decrements `available` (inside the claim or promotion transaction).
- Leaving to `expired`, `cancelled`, `rejected` increments `available` and triggers promotion.
- `proof_submitted` keeps the stock locked (no timer) until the seller decides.
- `refund_pending` from `paid` (seller cancel) does **not** return stock automatically; the seller decides.

## 6. Hold expiry and waitlist promotion

```mermaid
sequenceDiagram
  participant W as expirer worker
  participant DB as PostgreSQL
  participant O as outbox worker
  loop every 2 s
    W->>DB: SELECT held orders WHERE hold_expires_at + grace < now() FOR UPDATE SKIP LOCKED LIMIT 100
    W->>DB: UPDATE order to expired, available plus qty
    W->>DB: SELECT first waiting entry for variant FOR UPDATE SKIP LOCKED
    alt entry found
      W->>DB: available minus qty, INSERT held order (source waitlist), entry offered
      W->>DB: INSERT outbox "Giliranmu, bayar dalam 15 menit"
    end
    W->>DB: COMMIT (one transaction per expired order)
  end
  O->>O: deliver WA messages with retry/backoff
```

If an offered buyer does not pay, their order expires like any other and the next entry is promoted. Entries are marked `skipped` if the buyer already has an active order for that variant.

## 7. Manual payment

```mermaid
sequenceDiagram
  participant B as Buyer
  participant A as API
  participant S as Seller dashboard
  B->>A: POST /api/orders/{code}/proof (image)
  A->>A: validate, store file, order -> proof_submitted
  A-->>S: SSE: new proof in queue
  S->>A: POST /seller/orders/{id}/confirm
  A->>A: order -> paid, audit_log, outbox "Lunas"
  Note over S,A: Reject → order rejected, stock released, waitlist promoted, buyer notified with reason
```

## 8. Automatic payment (Xendit)

```mermaid
sequenceDiagram
  participant B as Buyer
  participant A as API
  participant X as Xendit
  A->>X: create QRIS (for-user-id=sub-account, with-split-rule, expires=hold end)
  X-->>A: qr_string, payment id
  A-->>B: render QRIS
  B->>X: scan and pay
  X->>A: POST /webhooks/xendit (callback token)
  A->>A: INSERT webhook_events (unique event id) — duplicate → 200 and stop
  A->>A: order held/expired → paid (or refund_pending), outbox "Lunas"
  A-->>B: SSE: status paid
```

## 9. Fulfilment

1. Paid → buyer fills address (required before seller can ship).
2. Seller enters tracking number (single or CSV bulk) → `shipped`, WA to buyer.
3. Buyer taps "Barang sudah diterima" → `completed`, or auto after 7 days.
4. Buyer reports → `disputed`; owner and seller see it; resolution recorded in `audit_log`.

## 10. Seller emergency close

Seller taps **Tutup drop** → confirm dialog → drop `closed`; active holds keep running until they expire or are paid; waitlist entries become `closed` and buyers are notified "Drop ditutup oleh seller".

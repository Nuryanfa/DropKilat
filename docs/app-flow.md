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
  A-->>P: code DK-7Q2M (valid 5 min, single use), wa.me link
  P->>WA: open wa.me/<number>?text=DK-7Q2M (kode login DropKilat)
  WA->>M: buyer sends "DK-7Q2M (kode login DropKilat)"
  M->>A: POST /webhooks/whatsapp (from, text)
  A->>A: extract DK-XXXX, match a pending unexpired code, bind phone to it
  A->>M: reply wa.verify_ok: what the code is for + "never send a code someone gave you" (inside 24h window)
  P->>A: poll / SSE: verification status
  A-->>P: verified, set buyer session cookie
```

Fallback when the buyer cannot open WA on this device: OTP template sent to the typed number.

Phishing mitigations (ADR 0003 amendment):
- Codes expire 5 minutes after start, bind one phone, and cannot be reused.
- The `wa.verify_ok` reply says the number was just logged in to DropKilat on the device that requested the code, names the shop, and warns never to send a DropKilat code from someone else (copy in `docs/design.md` §9).
- The recipient name, phone and address of earlier orders are masked on buyer pages. Showing them needs a step-up: a fresh code or OTP on this session in the last 10 minutes.

## 4. Claim

```mermaid
flowchart TD
  A[Buyer taps Ambil] --> B{Drop live and buyer verified?}
  B -- no --> X[Show reason]
  B -- yes --> C{Rate limit ok?}
  C -- no --> X
  C -- yes --> D[TX system: advisory lock drop+buyer]
  D --> E{Active qty + qty <= max_per_buyer?}
  E -- no --> X2[You already have this item]
  E -- yes --> V[Lock variant row FOR UPDATE]
  V --> F{Nobody waiting AND available >= qty?}
  F -- yes --> G[available - qty, INSERT order held, hold_expires_at = now + hold]
  G --> H[Outbox: WA payment link]
  H --> I[Return hold + payment instructions]
  F -- no --> L[INSERT waitlist_entry waiting, ON CONFLICT do nothing]
  L --> P[PromoteFit: may serve this buyer at once]
  P --> Q{This buyer offered?}
  Q -- yes --> I
  Q -- no --> K[Return position]
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

Stock effects (complete list of stock paths: `docs/database-schema.md` §6a; accounting rule: ADR 0005):
- Entering `held` decrements `available` and sets `holds_stock = true` (inside the claim or promotion transaction).
- Leaving to `expired`, `cancelled`, `rejected` sets `holds_stock = false`, increments `available` and runs promotion, all in one transaction.
- `proof_submitted` keeps the stock locked (no hold timer). The seller has a review deadline instead (§7).
- `expired → paid` re-reserves stock (`holds_stock = true`, `available − qty`). Without stock the order goes to `refund_pending` and no stock moves.
- `refund_pending` from `paid` (seller cancel) returns stock only if the seller chooses "kembalikan stok"; otherwise `holds_stock` stays true.

## 6. Hold expiry and waitlist promotion

```mermaid
sequenceDiagram
  participant W as expirer worker
  participant DB as PostgreSQL
  participant O as outbox worker
  loop every 2 s, until no due hold is left
    W->>DB: BEGIN; pick one held order WHERE hold_expires_at + grace < $now FOR UPDATE SKIP LOCKED
    W->>DB: lock variant row; order expired, holds_stock false, available plus qty
    loop PromoteFit while available > 0 and drop live
      W->>DB: oldest waiting entry with qty <= available, FOR UPDATE SKIP LOCKED
      W->>DB: available minus qty, INSERT held order (source waitlist), entry offered
      W->>DB: INSERT outbox "Giliranmu, bayar dalam 15 menit"
    end
    W->>DB: COMMIT (one transaction per expired order)
  end
  O->>O: deliver WA messages with retry/backoff
```

If an offered buyer does not pay, their order expires like any other and the next entry is promoted.

Promotion rules (ADR 0005):
- **First fit in FIFO order.** An entry whose qty does not fit keeps its place while smaller entries behind it are served.
- **Skips.** An entry is `skipped` if its buyer is blocked in this shop or would exceed `max_per_buyer`. It stays `waiting` (and is tried next time) if the buyer is claiming at that moment.
- **Triggers.** The same `PromoteFit` runs after every release (expiry, cancel, reject, review-window cancel, seller cancel with stock return), after a stock increase, and after every enqueue.
- **Queue first.** A new claim never takes stock while someone is waiting.

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

Review deadline (PO decision 2026-11-11, task T-406):

| Proof submitted while… | `review_due_at` | At the deadline | Buyer may cancel |
| --- | --- | --- | --- |
| the drop is `live` | `proof_submitted_at + 60 min` | order flagged **Terlambat direview** (`review_late_at`), warning on the dashboard and by WA to the seller. No auto-reject. | from `proof_submitted_at + 6 h` |
| the drop is not live | `proof_submitted_at + 6 h` | same flag and warning | from `proof_submitted_at + 6 h` |

The `reviewer` worker sets the flags (`docs/backend.md` §5). A buyer cancel after 6 h is stock path 6.

**Tolak dan blokir pembeli di toko ini.** Next to Reject, the seller can reject *and* block the buyer in one transaction:
- The order is `rejected` and its stock released and promoted (path 5).
- A row is added to `shop_buyer_blocks`, and the buyer's `waiting` entries in this shop become `skipped`.
- One `audit_log` row is written.
- From then on, claims and promotions for that buyer in this shop fail with `buyer-blocked`.

The block is per shop only (PRD non-goal: no cross-shop blacklist). The seller can lift it in the shop profile.

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

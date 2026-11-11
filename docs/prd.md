# PRD — DropKilat MVP

Owner: Nur Yanfa (product) · Tech lead: Claude · Status: draft v1.1 · Updated: 2026-11-11

## 1. Problem

UMKM sellers who sell limited stock in short "drops" or pre-orders lose money at two points:

1. **When the drop opens.** Hundreds of "mau kak" comments arrive in minutes. Admins copy the order of buyers into Excel by hand, make mistakes, and sometimes sell the same item twice (oversell).
2. **After the drop.** Buyers who booked do not pay ("hit-and-run"). Stock stays blocked for days, the seller has to re-post, and the drop loses momentum. Payment proofs are checked one by one and can be fake.

All numbers about how often this happens are **hypotheses** until the seller interviews are done (see `docs/tasks/` and the interview script).

## 2. Users

| Persona | Description | Main job |
| --- | --- | --- |
| **Seller** (primary) | UMKM selling limited items: modest fashion, preloved, merchandise, PO food, rare plants. 1–4 drops per week, 20–300 buyers per drop. Sells through IG/TikTok/WA. | Run a drop that sells out fairly, get paid, ship. |
| **Seller admin** | Helps the seller during the drop. | Watch orders, confirm payments, input tracking numbers. |
| **Buyer** | Follower of the seller, on a phone, often on mobile data. | Get the item before it sells out, pay quickly, trust the seller. |

Not a target for the MVP: shops with large always-in-stock catalogues (they fit a web store or marketplace better).

## 3. Goals and success metrics

| Goal | Metric | MVP target |
| --- | --- | --- |
| No oversell, ever | Orders created beyond stock | 0 |
| Fewer hit-and-runs | % of holds that expire without payment, vs the seller's old way | Lower than the seller's own baseline |
| Drops sell out | Sell-through within 1 hour of opening | ≥ 80% |
| Less admin work | Admin minutes per drop for recap | < 5 |
| Sellers come back | Sellers who run a 2nd drop within 30 days | ≥ 60% |
| Engine holds the load | 500 claims in 10 s on 50 stock, one 2 vCPU / 4 GB VM | Exactly 50 holds, p95 < 300 ms |

North-star metric: **repeat drops per seller per month**.

## 4. Scope

### In MVP

1. Seller sign-up and login with WhatsApp OTP.
2. Seller verification level 1 (WA number) and level 2 (identity, required before receiving money).
3. Create a drop: title, photos, variants (name, price, stock), open time, close time, hold minutes (default 15), max items per buyer (default 1), payment mode.
4. Public drop page (mobile web, no app install): countdown, waiting room, remaining stock in real time, **Ambil** button.
5. Buyer verification by WhatsApp (buyer sends a code from their own WA; fallback OTP).
6. Claim engine: atomic stock decrement, 15-minute hold, automatic expiry, FIFO waitlist with automatic promotion.
7. **Payment mode Manual** (default, free): buyer pays to the seller's own QRIS/bank account shown on the page, uploads proof, seller confirms or rejects.
8. **Payment mode Otomatis** (behind a feature flag): dynamic QRIS through Xendit xenPlatform sub-account owned by the seller, automatic confirmation by webhook, 2% platform fee via split. Only for sellers with ≥ 15 orders/month; variant price ≥ Rp20.000 (ADR 0006).
9. Buyer address form after payment, seller enters tracking number, buyer confirms receipt.
10. Order list, CSV export, shipping label print view.
11. WhatsApp notifications: payment link, reminder 5 minutes before the hold ends, paid, waitlist turn, shipped.
12. Trust signals on the drop page: verification badges, linked IG/TikTok handle, track record (drops, orders, % confirmed received), "Seller baru" label and limits for new sellers.
13. Report button for buyers (fake account, item not received).

### Out of MVP (later phases)

- Subscriptions (Pro plan), broadcast to past buyers, priority access, raffle mode.
- DP/pelunasan, multi-admin roles, courier API integration, custom subdomain.
- Group order, packing evidence, booking slots, reseller, rental modes.
- Native mobile apps.

### Explicit non-goals

- DropKilat never holds buyers' money in its own account. Money goes to the seller's own account (Manual) or the seller's sub-account (Otomatis).
- No extra fee charged to buyers (BI prohibits passing QRIS MDR to consumers).
- No cross-shop blacklist of buyers. Reliability scores are per shop only.
- Verification badges are never sold.

## 5. User stories and acceptance criteria

### Seller

| ID | Story | Acceptance criteria |
| --- | --- | --- |
| S-1 | As a seller I sign up with my WhatsApp number. | OTP verified; session cookie set; level 1 badge granted. |
| S-2 | As a seller I complete identity verification. | Otomatis mode: sub-account KYC status `verified` from the PG. Manual mode: KTP name matches bank account name, reviewed by the owner. Until verified, drops cannot open. |
| S-3 | As a seller I link my Instagram or TikTok. | Seller puts a one-time code in their bio; we confirm it manually (MVP) and show "Link resmi dari @handle". |
| S-4 | As a seller I create a drop with variants. | Validation: price ≥ Rp1.000, stock 1–10.000, open time in the future, hold 5–30 minutes. Draft can be edited; a live drop can only add stock or close early. |
| S-5 | As a seller I share one link. | URL `/d/{shop_slug}/{drop_slug}`; OG image and title for WA/IG previews. |
| S-6 | As a seller I watch the drop live. | Dashboard shows available / held / paid / waitlist counts updating within 2 s. |
| S-7 | As a seller I confirm a manual payment. | One tap Confirm or Reject (with reason). Reject releases stock to the waitlist. "Tolak dan blokir pembeli di toko ini" also blocks that buyer in this shop only. Proofs not reviewed in time are flagged "Terlambat direview" and the seller is warned (§6). |
| S-8 | As a seller I ship. | Enter tracking number per order or bulk via CSV; buyer is notified. |
| S-9 | As a seller I export orders. | CSV with order code, buyer name, phone (masked unless paid), address, variant, qty, amount, status. |

### Buyer

| ID | Story | Acceptance criteria |
| --- | --- | --- |
| B-1 | As a buyer I open the link and see whether the seller is real. | Badges, linked handle, track record and "Seller baru" label shown above the fold. |
| B-2 | As a buyer I verify my WhatsApp once. | Tap "Verifikasi lewat WhatsApp" → WA opens with a prefilled code → after sending, page continues within 5 s. Remembered on this device for 90 days. Code valid 5 minutes; the WA reply explains the code is a DropKilat login; saved addresses stay masked until a fresh verification (ADR 0003). |
| B-3 | As a buyer I claim an item. | If stock: hold created, timer shown, payment instructions shown, WA message sent. If no stock: placed in waitlist with position. Never both. |
| B-4 | As a buyer I pay within the hold. | Manual: upload proof (JPG/PNG/PDF ≤ 5 MB) before the timer ends; status becomes "Menunggu konfirmasi seller". Otomatis: QRIS on the page; status becomes "Lunas" within 10 s of payment. |
| B-5 | As a buyer in the waitlist I get my turn. | When stock is released I get a WA message and a fresh hold of the same length. |
| B-6 | As a buyer I fill my address and track my order. | Address form after payment; status page shows timeline; I can confirm "Barang sudah diterima". |
| B-7 | As a buyer I report a problem. | Report form on the order page; seller and owner see it. |

## 6. Business rules

| Rule | Value |
| --- | --- |
| Default hold | 15 minutes (seller can set 5–30) |
| Grace before release | 2 minutes after hold end (late payments in flight) |
| Manual proof review window | During a live drop: 60 minutes, then flagged "Terlambat direview" and the seller is warned; no auto-reject. Outside a live drop: 6 hours, then flagged and warned. In both cases the buyer can cancel 6 hours after submitting. |
| Block a buyer | Seller can reject a proof and block that buyer **in this shop only** |
| Max per buyer per drop | Default 1, seller can set up to 10 |
| New seller limits | First 3 drops: max 30 total stock and Rp5.000.000 total value per drop, until ≥ 90% of paid orders are confirmed received |
| Platform fee (Otomatis) | 2% of net amount after PG fee, min Rp1.000, max Rp7.500 per order |
| Otomatis eligibility | Seller has ≥ 15 orders per month; every variant price ≥ Rp20.000 |
| Platform fee (Manual) | Free during the pilot. After Gate D: 50 confirmed orders per seller per calendar month free, then Rp1.000 per confirmed order, billed on a monthly invoice. Orders are never blocked. A refund caused by the seller or a system error is credited on the next invoice; a buyer cancel is not. An invoice more than 14 days overdue stops the seller from starting new drops (ADR 0006 amendment). |
| Auto-complete | Order becomes "Diterima" 7 days after "Dikirim" if the buyer does not report |

## 7. Pricing (decided 2026-11-11 by PO, ADR 0006; amounts to validate in the pilot)

| Plan | Price | Includes |
| --- | --- | --- |
| Pilot | Rp0 | Everything, until Gate D |
| Manual | 50 confirmed orders/month free, then Rp1.000 per confirmed order (monthly invoice) | Manual payment mode, hold and waitlist, "Powered by DropKilat" |
| Otomatis | 2% per paid order (min Rp1.000, max Rp7.500) | Sellers with ≥ 15 orders/month, items ≥ Rp20.000. Dynamic QRIS, automatic confirmation, WA notifications |
| Pro (later) | Rp79–149k / month + 2% | Customer list, priority access, broadcast, raffle, multi-admin, analytics, custom subdomain |

## 8. Risks (summary)

| Risk | Mitigation |
| --- | --- |
| System fails during a seller's first drop | Load test gate before pilot; limit concurrent live drops; emergency "close drop" button |
| Fake or fraudulent sellers | Required identity verification, new-seller limits, track record, reports |
| WhatsApp cost per order eats the margin | Buyer-initiated verification opens the 24-hour service window |
| Payment gateway terms not as assumed | Manual mode ships first; Otomatis is behind a flag until Xendit confirms QRIS + split + settlement |
| Regulatory: holding funds | Never route money through DropKilat's account |

## 9. Open questions

1. Xendit: QRIS with split on sub-accounts supported? Entity requirement (PT perorangan?) for xenPlatform? Settlement time to sub-account?
2. Meta Cloud API: confirm that utility templates inside an open customer service window are free for our category and region.
3. How many orders per month does a typical small seller really have? (from interviews)
4. Is 15 minutes right for buyers who pay by bank transfer in Manual mode?
5. Billing details: how "≥ 15 orders/month" is measured (default in ADR 0006, confirm in T-806). Refunds and unpaid invoices decided 2026-11-12; still open: credit after a dispute refund, invoice due date (ADR 0006 amendment).
6. Tax and legal check before the first invoice (invoice format, PPN).

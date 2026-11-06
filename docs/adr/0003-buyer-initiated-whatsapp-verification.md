# ADR 0003 — Buyer-initiated WhatsApp verification

Status: accepted (pricing assumption to re-check) · Date: 2026-11-06

## Context
Business-initiated template messages cost about Rp367 each (utility/authentication, BSP list price Oct 2026). Three messages per order would cost about Rp1.100, a large share of a Rp2.000–3.000 fee. Messages sent while a 24-hour customer service window is open (opened when the user messages the business) are not charged for service replies and, per Meta's current pricing, utility templates inside the window are free.

## Decision
Buyers verify by sending a short code (`DK-XXXX`) from their own WhatsApp via a `wa.me` link. This proves phone ownership and opens the service window, so the payment link, reminder and waitlist messages during a drop are sent inside it. OTP by authentication template is the fallback.

## Consequences
- Verification needs WhatsApp on the same device, or the fallback.
- `notify` must track the last inbound time per buyer and choose template vs free-form.
- Re-check Meta pricing rules before launch (task in P5) and record the result here.

# ADR 0003 — Buyer-initiated WhatsApp verification

Status: accepted (pricing assumption to re-check; amended 2026-11-11) · Date: 2026-11-06

## Context
Business-initiated template messages cost about Rp367 each (utility/authentication, BSP list price Oct 2026). Three messages per order would cost about Rp1.100, a large share of a Rp2.000–3.000 fee. Messages sent while a 24-hour customer service window is open (opened when the user messages the business) are not charged for service replies and, per Meta's current pricing, utility templates inside the window are free.

## Decision
Buyers verify by sending a short code (`DK-XXXX`) from their own WhatsApp via a `wa.me` link. This proves phone ownership and opens the service window, so the payment link, reminder and waitlist messages during a drop are sent inside it. OTP by authentication template is the fallback.

## Consequences
- Verification needs WhatsApp on the same device, or the fallback.
- `notify` must track the last inbound time per buyer and choose template vs free-form.
- Re-check Meta pricing rules before launch (task in P5) and record the result here.

## Amendment 2026-11-11 — phishing mitigations (decision by Nur)

Risk: an attacker opens a drop page, gets a code, and persuades a victim to send it ("kirim DK-7Q2M ke nomor ini untuk promo"). The attacker's browser is then bound to the victim's phone and can see the victim's orders and addresses.

Decision, all four in T-503:

1. **Short-lived, single-use codes.** A code expires **5 minutes** after `verify/start` (was 10), binds at most one phone, and cannot be reused after `verified` or `expired`.
2. **The WhatsApp reply explains the code.** It says the code logs this number in to DropKilat on the device that requested it, names the shop page that requested it, and warns never to send a DropKilat code someone else gave you. Copy: `docs/design.md` §9.
3. **Masked personal data until step-up.** On buyer order pages, the recipient name, phone and address of earlier orders are masked (`docs/design.md` §9). Full values appear only after a fresh verification (new WA code or OTP) on this session in the last 10 minutes. Filling a *new* address for the current order needs no step-up.
4. **The wa.me prefill names the purpose.** The prefilled text is `DK-7Q2M (kode login DropKilat)`, so the sender sees what they send. The webhook matches only the `DK-XXXX` token.

Considered, not decided: a "reply BATAL to log that device out" command. If the pilot shows phishing attempts, it becomes a follow-up card.

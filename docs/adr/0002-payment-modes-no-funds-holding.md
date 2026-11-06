# ADR 0002 — Two payment modes; DropKilat never holds buyer funds

Status: accepted · Date: 2026-11-06

## Context
Options considered:
1. **Single master account + internal ledger**: all payments land in one DropKilat merchant account; seller balances tracked in our database and paid out. Cheapest on paper (no per-sub-account fee).
2. **Xendit xenPlatform sub-accounts**: each seller has a sub-account; platform fee via split rule. Cost: Rp25.000 per active sub-account per month, 0.5% of the split amount.
3. **Manual**: buyer pays the seller's own QRIS/bank account; seller confirms the proof.

Option 1 makes DropKilat collect and hold money for third parties, which is close to payment-service activity that needs a Bank Indonesia licence, likely conflicts with payment-gateway terms, books the sellers' gross sales as our turnover for tax, shows "DropKilat" instead of the shop name on the QRIS, and makes us liable for seller fraud.

## Decision
- Ship **Manual** mode first (free tier, no gateway dependency).
- Add **Xendit sub-account** mode behind a feature flag after Xendit confirms QRIS + split support, entity requirements and settlement times (task T-801).
- **Reject option 1.** Revisit only with legal advice and a licensed partner.

## Consequences
- Small sellers cost nothing in gateway fees; the per-sub-account fee applies only to sellers who opt into automatic mode.
- Manual mode needs a seller confirmation step and a proof-review window; fraud risk is mitigated by identity verification and track record.

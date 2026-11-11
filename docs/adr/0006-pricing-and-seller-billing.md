# ADR 0006 — Pricing after the pilot and monthly seller billing

Status: accepted (decision by Nur, product owner) · Date: 2026-11-11 · Amends: PRD §6–7

## Context
In PRD v1, Manual mode was free and capped at 50 orders/month, and the only revenue was the 2% Otomatis fee. Otomatis comes last (P8, waiting on Xendit), so revenue was zero until then. In Otomatis, each seller sub-account costs Rp25.000/month. A minimum fee of Rp1.500 is a 15% take on a Rp10.000 item.

Break-even per sub-account (corrected; the earlier "~4 orders" assumed every order paid the Rp7.500 maximum fee). Take a Rp100.000 order with an assumed QRIS MDR of 0.7%:

- net amount Rp99.300; 2% fee ≈ Rp1.986
- minus Xendit's 0.5% fee on the split ≈ Rp1.976 to us
- Rp25.000 / Rp1.976 ≈ **13 orders/month**

The MDR and split fee are unconfirmed until T-801.

## Decision
1. **Pilot is free** (P7, until Gate D) for every seller and every feature.
2. **Manual mode after Gate D:** each seller gets **50 confirmed orders free per calendar month (WIB)**. Each confirmed order above that costs **Rp1.000** and is billed on a **monthly invoice** to the seller. A *confirmed order* is a Manual order that reached `paid` in that month. Orders are never blocked for exceeding the quota.
3. **Otomatis mode:**
   - Fee stays at 2% of the net amount after the PG fee, with the **minimum lowered to Rp1.000** and the maximum at Rp7.500 per paid order. It is collected by split rule.
   - Only for sellers with **at least 15 orders per month**.
   - **Variant price ≥ Rp20.000** in Otomatis drops.
4. Billing does not touch buyer money. The invoice is DropKilat's own fee, which the seller pays to DropKilat. Buyer payments still go only to the seller's account or sub-account (ADR 0002 unchanged). Buyers pay no fee.

Interpretations by the tech lead, to be confirmed by Nur in the named cards:

| Question | Default until confirmed | Card |
| --- | --- | --- |
| How is "≥ 15 orders/month" measured? | ≥ 15 `paid` orders (any mode) in the 30 days before opting in; no automatic downgrade in MVP | T-806 |
| Does an order refunded later still count as confirmed? | Yes if refunded in a later month; no if refunded in the same month before the invoice | T-901 |
| What happens to an unpaid invoice? | Reminder at +7 and +14 days; no blocking in MVP | T-901 |

## Consequences
- T-407 (P4) meters confirmed Manual orders per seller per month and shows the meter on the dashboard. Pilot sellers are marked exempt. Nothing blocks.
- T-901 (new phase P9, after Gate D) turns usage into monthly invoices. Before the first real invoice, Nur gets a tax and legal check (invoice format; PPN if DropKilat becomes PKP). This is listed as an escalation in `CLAUDE.md`.
- T-201 validates variant price ≥ Rp20.000 when `payment_mode = 'xendit'`. T-806 enforces Otomatis eligibility.
- Sellers below 15 orders/month stay on Manual, which keeps the per-sub-account loss from appearing.

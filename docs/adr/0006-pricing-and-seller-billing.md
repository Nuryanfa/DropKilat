# ADR 0006 — Pricing after the pilot and monthly seller billing

Status: accepted (decision by Nur, product owner) · Date: 2026-11-11 · Amends: PRD §6–7 · Amended 2026-11-12 (refunds, unpaid invoices, issuer; see the end)

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
| Does an order refunded later still count as confirmed? | **Decided 2026-11-12**, see amendment | T-901 |
| What happens to an unpaid invoice? | **Decided 2026-11-12**, see amendment | T-901 |

## Consequences
- T-407 (P4) meters confirmed Manual orders per seller per month and shows the meter on the dashboard. Pilot sellers are marked exempt. Nothing blocks.
- T-901 (new phase P9, after Gate D) turns usage into monthly invoices. Before the first real invoice, Nur gets a tax and legal check (invoice format; PPN if DropKilat becomes PKP). This is listed as an escalation in `CLAUDE.md`.
- T-201 validates variant price ≥ Rp20.000 when `payment_mode = 'xendit'`. T-806 enforces Otomatis eligibility.
- Sellers below 15 orders/month stay on Manual, which keeps the per-sub-account loss from appearing.

## Amendment 2026-11-12 — refunds, unpaid invoices, issuer (decision by Nur)

### Decision
5. **Refunds.** An order counts in the month it reaches `paid` (confirmed), whatever happens later. If it is later refunded because **the seller cancelled** or because of **a system error**, the fee for that order is **credited on the next invoice**. If **the buyer cancelled** after paying, the fee stays.
6. **Unpaid invoices.** WhatsApp reminder on the due date and 7 days later. When an invoice is **more than 14 days overdue**, the seller **cannot create or publish new drops**. Live and scheduled drops, orders and data are untouched, and buyers are never affected. During the first period after Gate D, Nur handles overdue cases manually.
7. **Issuer.** Invoices are issued in Nur's personal name until a legal entity exists. The issuer name and bank account come from configuration (env/secret), never from the repository. Nur provides them before Gate D. The tax/legal check before the first invoice still applies.

### How the tech lead implements it (T-901)
- **Refund cause.** Every `paid → refund_pending` transition records `orders.refund_cause`:
  - `seller` — seller cancelled for their own reason (out of stock, cannot ship). **Credited.**
  - `buyer_request` — seller cancelled because the buyer asked. **Not credited.** There is no buyer-side cancel after `paid` in the MVP; a buyer who wants out asks the seller, who picks this cause.
  - `system_error` — set only by the owner (Nur) from the owner screen, e.g. a double charge or a bug. **Credited.**
  - `dispute` — `disputed → refund_pending`. **Credited** (confirmed by Nur 2026-11-12).
  - `late_payment` — `expired → refund_pending`. Never confirmed, so never billed; no credit.
- **Credit amount.** A credit only returns a fee that was billed. For a month M: credits for M = min(orders billed for M, credited refunds of orders confirmed in M). Each credit is Rp1.000 and tied to one order, so an order is never credited twice.
- **Where credits go.** The invoicer applies unapplied credits, oldest first, to the next invoice it issues. A credit larger than that invoice's charges carries over to the following invoice. An invoice amount is never negative. A refund recorded before the invoice for its own month is issued is credited on that invoice.
- **Due date.** The 10th of the month the invoice is issued (WIB), i.e. 9 days after issue. Reminders at `due_at` and `due_at + 7 days`. Overdue block from `due_at + 14 days`.
- **Overdue block switch.** The block is behind `DK_BILLING_ENFORCE_OVERDUE` (default `false`), so Nur handles overdue cases manually until it is turned on. The owner screen lists overdue invoices either way. Marking an invoice paid or void lifts the block immediately.
- **Issuer details.** `DK_INVOICE_ISSUER_NAME`, `DK_INVOICE_BANK_NAME`, `DK_INVOICE_BANK_ACCOUNT`, `DK_INVOICE_BANK_HOLDER`, required when `DK_FEATURE_BILLING=true`. Each invoice stores an encrypted snapshot of them at issue time, so a later change does not rewrite issued invoices. The bank account is never logged.

### Open points for Nur
- None. Both defaults were confirmed by Nur on 2026-11-12: refunds after a dispute are credited, and the due date is the 10th of the issue month.

### Consequences
- New problem type `invoice-overdue` (403) on drop create and publish, only when the switch is on.
- `orders.refund_cause` and `billing_credits` join the T-901 migration. The seller cancel endpoint takes a required `cause` (`seller` or `buyer_request`).
- The rule in decision 2 still holds: buyer orders are never blocked. The overdue block only stops a seller from starting new drops.

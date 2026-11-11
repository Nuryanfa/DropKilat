# Architecture decision records

| ADR | Title | Status |
| --- | --- | --- |
| [0001](0001-modular-monolith-go-postgres.md) | Modular monolith in Go with PostgreSQL as the only stateful service | accepted, fan-out amended by 0007 |
| [0002](0002-payment-modes-no-funds-holding.md) | Two payment modes; DropKilat never holds buyer funds | accepted |
| [0003](0003-buyer-initiated-whatsapp-verification.md) | Buyer-initiated WhatsApp verification | accepted, amended 2026-11-11 (phishing mitigations) |
| [0004](0004-rls-contexts-public-buyer.md) | RLS contexts: `system`, `seller`, `buyer`, `public` | accepted |
| [0005](0005-fifo-waitlist-and-stock-accounting.md) | FIFO waitlist, stock accounting and time in SQL | accepted |
| [0006](0006-pricing-and-seller-billing.md) | Pricing after the pilot and monthly seller billing | accepted (PO) |
| [0007](0007-in-process-sse-fan-out.md) | In-process SSE fan-out, no NOTIFY in the claim path | accepted |

New ADR: copy the shape of an existing one (Context, Decision, Consequences), next number, add a row here. Only the tech lead marks an ADR accepted; the product owner approves ADRs that touch money, legal or scope.

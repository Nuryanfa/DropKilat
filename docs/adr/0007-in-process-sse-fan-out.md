# ADR 0007 — In-process SSE fan-out, no NOTIFY in the claim path

Status: accepted · Date: 2026-11-11 · Amends: ADR 0001 (real-time fan-out)

## Context
ADR 0001 and `docs/backend.md` §6 planned `LISTEN/NOTIFY` on channel `drop_<id>` for stock updates. A `NOTIFY` inside the claim transaction makes every committing transaction that notified take PostgreSQL's global notify-queue lock at commit. During a burst (500 claims in 10 s), commits on all drops would serialise on that lock, putting NFR-2 (p95 < 300 ms) at risk. The MVP runs one binary, so cross-process fan-out is not needed.

## Decision
- An in-process `drop` hub fans updates out to SSE clients.
- After a transaction **commits**, the service that changed stock or an order calls `hub.Touch(dropID)` (stock or counters) or `hub.OrderChanged(buyerID, code)` (the buyer's own order).
- Every 500 ms the hub reads counters for the touched drops in one query and broadcasts them: `stock` to buyers, `counters` to the seller monitor. This coalescing replaces "max 1 event per second per variant".
- No `NOTIFY` in any transaction in the MVP.

## Consequences
- NFR-4 (stock change visible within 2 s) is met with margin: at most 500 ms plus network.
- Running two app instances would need cross-process fan-out. Revisit then with a coalesced publisher (one `NOTIFY` per drop per tick, outside request transactions). This ADR is superseded at that point.
- A missed `Touch` (for example a code path that forgets to call it) shows up as stale counters, not wrong stock. The hub also refreshes every live drop every 5 s as a safety net.

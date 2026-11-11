# ADR 0005 — FIFO waitlist, stock accounting and time in SQL

Status: accepted · Date: 2026-11-11

## Context
The v1 claim and promotion SQL (`docs/database-schema.md` §5–6) had four gaps:

1. **Queue jumping.** A claim only checked `available >= qty`. When stock was free while people were waiting, a new buyer got the hold ahead of them.
2. **One promotion per release.** An expired hold of qty 2, or a skipped head of the queue, left stock idle while buyers waited.
3. **Rules in prose only.** "Only if the drop is still live" and "skip buyers who already hold" were comments. A stock increase on a live drop did not promote at all.
4. **Inconsistent invariant.** `paid → refund_pending` keeps the stock (`docs/app-flow.md` §5), but the invariant in `GEMINI.md` and T-205 left `refund_pending`/`refunded` out of the sum, so correct behaviour would fail the tests. Late-payment re-reservation and stock increases were missing from the stock paths list.

Also, the SQL used `now()` for `hold_expires_at` and the expiry check, which contradicts "time via `clock.Clock`" and makes the fake-clock tests of T-204/NFR-5 impossible.

## Decision

### Queue first, first fit
- **Every change to a variant's stock or queue first locks the variant row** (`SELECT … FROM drop_variants WHERE id = $v FOR UPDATE`). One row lock serialises claims, enqueues, releases and promotions on that variant. Claims already serialised on this row through the `UPDATE`, so the extra cost is the queue check (an index probe).
- **Claim:** if the variant has no `waiting` entry and `available >= qty`, create the hold. Otherwise enqueue the buyer, then run promotion. The new entry may be promoted at once, in which case the result is `held`.
- **Promotion** (`waitlist.PromoteFit`) loops while stock is available. It takes the oldest `waiting` entry whose `qty <= available` (first fit in FIFO order), so a large head-of-queue request does not block smaller ones. For each candidate:
  - `pg_try_advisory_xact_lock(drop, buyer)`. If that buyer is claiming at this moment, leave the entry `waiting` and try the next one. This avoids a lock-order deadlock with the claim (advisory → variant).
  - Mark the entry `skipped` if the buyer is blocked in this shop (T-406) or would exceed `max_per_buyer`.
  - Otherwise decrement stock, insert a `held` order (`source = 'waitlist'`), mark the entry `offered` and queue `wa.waitlist_turn`.
- Promotion runs only while the drop is `live` and `$now < closes_at`. It is called after **every** release (expiry, buyer cancel, reject, review-window cancel, seller cancel with stock return), after every stock increase, and after every enqueue.
- **Lock order** everywhere: existing order row → variant row → waitlist entries. The claim takes the advisory lock (drop, buyer) before the variant row, and promotion only *tries* advisory locks. No cycle.
- **Closing a drop** races with at most one promotion or claim that already read the drop as live. The resulting hold is valid and runs like any other active hold (`docs/app-flow.md` §10).

### Stock accounting
- New column `orders.holds_stock boolean NOT NULL DEFAULT true`. It is true while the order consumes a unit of stock. A CHECK forbids `holds_stock` on `expired`, `cancelled` and `rejected`. Migration `000003_orders_holds_stock` (task T-203):

  ```sql
  ALTER TABLE orders
    ADD COLUMN holds_stock boolean NOT NULL DEFAULT true,
    ADD CONSTRAINT orders_released_status_frees_stock
        CHECK (status NOT IN ('expired', 'cancelled', 'rejected') OR NOT holds_stock);
  ```

- **Invariant**, per variant at every commit:
  `available + Σ qty (orders WHERE holds_stock) = stock_total`, and `0 ≤ available ≤ stock_total`.
- Releasing stock means `holds_stock = false` **and** `available + qty` in the same transaction. Late-payment re-reservation sets `holds_stock = true` again with `available - qty`. A seller cancelling a paid order chooses whether stock returns. `refund_pending`/`refunded` after shipping keep `holds_stock = true`.
- The only stock paths are the nine listed in `docs/database-schema.md` §6a.

### Time
- Every deadline and every time comparison in SQL takes `$now` from `clock.Clock`. `DEFAULT now()` remains only on bookkeeping columns (`created_at`, `updated_at`).
- The drop-live check in the claim is a plain `SELECT`, not `FOR SHARE`. Multixact share locks on one hot `drops` row cost more than they protect.

## Verification
The reference statements in `docs/database-schema.md` §5–6 were run on PostgreSQL 16 against migrations 000001 + 000002 + 000003, using a test-only plpgsql rendering:

| Scenario | Result |
| --- | --- |
| 500 buyers × 2 claims each, 40 parallel connections, 50 stock, `max_per_buyer` 1 | 50 `held`, 450 `waiting`, 0 duplicate entries, invariant true |
| 10 holds expired by 3 parallel expirers while 30 new buyers claim | waitlist positions #1–#10 `offered` (strict FIFO), all 30 newcomers `waitlisted`, `available` 0, invariant true |
| Head of the queue wants 2, the next wants 1, 1 unit released | the second buyer is promoted; the head is promoted after a later +1 stock increase |

## Consequences
- T-203/T-204 implement exactly this. The concurrency suite (T-205) checks the new invariant and FIFO order.
- First fit means a buyer wanting qty 2 can wait while later buyers wanting 1 are served. With `max_per_buyer` defaulting to 1 this is rare; the waitlist card (B6) does not promise an exact position for multi-quantity entries.
- Hot-row cost grows slightly because waitlist inserts now also take the variant lock. NFR-2 is checked in T-701; the fallback (stock shards) is unchanged from ADR 0001.

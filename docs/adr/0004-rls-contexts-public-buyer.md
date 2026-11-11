# ADR 0004 — RLS contexts: `system`, `seller`, `buyer`, `public`

Status: accepted · Date: 2026-11-11 · Supersedes: the `app.role = 'public'` row of `docs/database-schema.md` §4 (v1)

## Context
Migration `000001_init` defines `app_is_system()` as `app.role IN ('system', 'public')`. Every buyer-facing transaction runs as `public`, so for those requests the tenant policy `app_is_system() OR seller_id = app_seller_id()` is always true: a buyer-facing query can read and write every seller's orders, waitlist entries and payments. RLS, our second line of defence, is off exactly on the surface exposed to anonymous traffic.

Options considered:
1. Keep one `public` context and rely on explicit `WHERE` clauses. Rejected: that is the first line; RLS exists for when it fails.
2. Give buyers write policies on `drop_variants` and `orders` so the claim can run as the buyer. Rejected: RLS limits rows, not columns. A buyer policy that allows `UPDATE drop_variants` for a live drop also allows a buggy handler to change `price`.
3. Split the contexts: a read-only `public`, a `buyer` context limited to the buyer's own rows, and run every stock-moving transaction as `system` inside the `order`/`waitlist`/`drop` packages only.

## Decision
Option 3.

| Context | Set by | Can read | Can write |
| --- | --- | --- | --- |
| `system` | `WithSystemTx` | everything | everything |
| seller | `WithSellerTx(sellerID)` | own tenant rows | own tenant rows |
| `buyer` | `WithBuyerTx(buyerID)` | published catalog, own orders, own waitlist entries, payments of own orders | `UPDATE` own orders and own waitlist entries; `INSERT` reports on own orders |
| `public` | `WithPublicTx` | published catalog (`drops` in `scheduled`/`live`/`closed` and their variants and photos), verified social links | nothing |

- `app.buyer_id` is only honoured when `app.role = 'buyer'`; setting it under `public` returns nothing.
- **Every transaction that changes stock or the queue** (claim, promotion, expiry, cancel, reject, late payment, stock increase; list in `docs/database-schema.md` §6a) runs under `system`, after the handler has authorised the actor. Buyer and seller IDs come from the session, never from the request body.
- Buyer writes that do not move stock (proof upload, address, received, report, leave waitlist) run under `buyer`.
- `sellers` and `buyers` stay without RLS: they are not tenant tables, and their PII is encrypted.

Migration `000002_rls_contexts` (task T-102). Verified on PostgreSQL 16 with migration 000001 applied, up → down → up clean:

```sql
CREATE OR REPLACE FUNCTION app_is_system() RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT COALESCE(current_setting('app.role', true), '') = 'system'
$$;
CREATE FUNCTION app_is_public() RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT COALESCE(current_setting('app.role', true), '') IN ('public', 'buyer')
$$;
CREATE FUNCTION app_buyer_id() RETURNS uuid LANGUAGE sql STABLE AS $$
  SELECT CASE WHEN current_setting('app.role', true) = 'buyer'
              THEN NULLIF(current_setting('app.buyer_id', true), '')::uuid END
$$;

CREATE POLICY drops_public_read ON drops FOR SELECT
  USING (app_is_public() AND status IN ('scheduled', 'live', 'closed'));
CREATE POLICY drop_variants_public_read ON drop_variants FOR SELECT
  USING (app_is_public() AND EXISTS (SELECT 1 FROM drops d WHERE d.id = drop_id
                                      AND d.status IN ('scheduled', 'live', 'closed')));
CREATE POLICY drop_photos_public_read ON drop_photos FOR SELECT
  USING (app_is_public() AND EXISTS (SELECT 1 FROM drops d WHERE d.id = drop_id
                                      AND d.status IN ('scheduled', 'live', 'closed')));
CREATE POLICY seller_social_links_public_read ON seller_social_links FOR SELECT
  USING (app_is_public() AND status = 'verified');

CREATE POLICY orders_buyer_read ON orders FOR SELECT USING (buyer_id = app_buyer_id());
CREATE POLICY orders_buyer_update ON orders FOR UPDATE
  USING (buyer_id = app_buyer_id()) WITH CHECK (buyer_id = app_buyer_id());
CREATE POLICY waitlist_entries_buyer_read ON waitlist_entries FOR SELECT USING (buyer_id = app_buyer_id());
CREATE POLICY waitlist_entries_buyer_update ON waitlist_entries FOR UPDATE
  USING (buyer_id = app_buyer_id()) WITH CHECK (buyer_id = app_buyer_id());
CREATE POLICY payments_buyer_read ON payments FOR SELECT
  USING (EXISTS (SELECT 1 FROM orders o WHERE o.id = order_id AND o.buyer_id = app_buyer_id()));
CREATE POLICY reports_buyer_insert ON reports FOR INSERT
  WITH CHECK (buyer_id = app_buyer_id()
              AND EXISTS (SELECT 1 FROM orders o WHERE o.id = order_id AND o.buyer_id = app_buyer_id()));
```

The `down` drops the ten policies and the two new functions, and restores the old `app_is_system()`.

Results observed as `dropkilat_app` (two sellers, one live and one draft drop, two buyers with one order each):

| Context | Result |
| --- | --- |
| `public` | sees 1 drop (the live one), 1 variant, 0 orders; `UPDATE drop_variants` touches 0 rows; `INSERT INTO orders` → RLS error |
| `buyer` b1 | sees 1 order (own); `UPDATE orders` touches 1 row; `UPDATE drop_variants` touches 0; changing `buyer_id` → RLS error |
| `public` + `app.buyer_id` | 0 orders |
| seller B | 0 orders, 1 drop (own draft) |
| seller A / `system` | 2 orders |
| nothing set | 0 orders, 0 drops |

## Consequences
- `platform/db` gains `WithBuyerTx(ctx, buyerID, fn)`. `WithPublicTx` is read-only by policy; T-102 also sets `SET LOCAL transaction_read_only = on` in it.
- The claim runs under `system`, so RLS does not protect it. That is acceptable only because the claim is one reviewed function in `order` with concurrency tests (Gate B). New stock paths need a card and TL review (`AGENTS.md` hard rule).
- The IDOR tests in T-306 must also run with the RLS context alone: no `WHERE buyer_id`, and the result must still be empty.

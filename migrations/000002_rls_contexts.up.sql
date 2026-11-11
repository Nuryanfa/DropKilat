BEGIN;
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
COMMIT;

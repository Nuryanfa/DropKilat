BEGIN;
DROP POLICY drops_public_read ON drops;
DROP POLICY drop_variants_public_read ON drop_variants;
DROP POLICY drop_photos_public_read ON drop_photos;
DROP POLICY seller_social_links_public_read ON seller_social_links;
DROP POLICY orders_buyer_read ON orders;
DROP POLICY orders_buyer_update ON orders;
DROP POLICY waitlist_entries_buyer_read ON waitlist_entries;
DROP POLICY waitlist_entries_buyer_update ON waitlist_entries;
DROP POLICY payments_buyer_read ON payments;
DROP POLICY reports_buyer_insert ON reports;
DROP FUNCTION app_is_public(), app_buyer_id();
CREATE OR REPLACE FUNCTION app_is_system() RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT COALESCE(current_setting('app.role', true), '') IN ('system', 'public')
$$;
COMMIT;

BEGIN;
DROP TABLE IF EXISTS audit_log, reports, outbox, webhook_events, payments, waitlist_entries, orders,
  buyer_verifications, buyer_sessions, buyers, drop_variants, drop_photos, drops,
  seller_stats, seller_social_links, otp_challenges, seller_sessions, sellers CASCADE;
DROP FUNCTION IF EXISTS app_is_system(), app_seller_id(), set_updated_at();
COMMIT;

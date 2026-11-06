-- Runs once when the database volume is created.
-- Owner role `dropkilat` runs migrations; the app connects as `dropkilat_app` so RLS applies.
CREATE ROLE dropkilat_app LOGIN PASSWORD 'dropkilat_app';   -- change in production
GRANT CONNECT ON DATABASE dropkilat TO dropkilat_app;
GRANT USAGE ON SCHEMA public TO dropkilat_app;
ALTER DEFAULT PRIVILEGES FOR ROLE dropkilat IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO dropkilat_app;
ALTER DEFAULT PRIVILEGES FOR ROLE dropkilat IN SCHEMA public
  GRANT USAGE ON SEQUENCES TO dropkilat_app;

-- Indexes for the read endpoints, which carry most of the traffic.
-- In production, run each CREATE INDEX with CONCURRENTLY (outside a transaction) to avoid locking writes.

-- Trigram index: makes the title search (ILIKE '%bike%') use an index instead of scanning every ad.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX ads_title_trgm_idx ON ads USING gin (title gin_trgm_ops);

-- Default listing order (newest first) with a stable tie-breaker, read without sorting.
CREATE INDEX ads_created_at_id_idx ON ads (created_at DESC, id DESC);

-- "Ads of a seller", newest first.
CREATE INDEX ads_user_id_created_at_idx ON ads (user_id, created_at DESC, id DESC);

-- Price range filters.
CREATE INDEX ads_price_cents_idx ON ads (price_cents);
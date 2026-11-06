-- Remembers bulk imports by idempotency key, so that a client retrying an import
-- (after a timeout or a lost response) gets the original result instead of duplicates.
CREATE TABLE ad_imports (
                            user_id         UUID        NOT NULL,
                            idempotency_key TEXT        NOT NULL,
                            request_hash    TEXT        NOT NULL,
                            ad_ids          JSONB,
                            created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
                            PRIMARY KEY (user_id, idempotency_key)
);
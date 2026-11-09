-- Log of ads published through POST /ads, used to enforce the publication quota.
-- It is separate from the ads table on purpose: deleting an ad must not give a slot back,
-- and bulk imports, which are not subject to the quota, never write here.
CREATE TABLE ad_publications (
                                 user_id      UUID        NOT NULL,
                                 ad_id        BIGINT      NOT NULL,
                                 published_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Serves the quota query: the publications of one user over the rolling window.
CREATE INDEX ad_publications_user_published_at_idx ON ad_publications (user_id, published_at);
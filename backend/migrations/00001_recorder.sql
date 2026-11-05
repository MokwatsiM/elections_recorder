-- +goose Up

-- Bodies are stored once per distinct content. content_sha hashes the file with
-- its ReportDate values removed, because the IEC rewrites files every cycle.
CREATE TABLE raw_blobs (
    content_sha bytea PRIMARY KEY,
    gz          bytea NOT NULL,
    raw_size    integer NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
-- Already gzipped, so don't let TOAST try to compress it again.
ALTER TABLE raw_blobs ALTER COLUMN gz SET STORAGE EXTERNAL;

-- One row each time a path's content changed. This is the replayable history.
CREATE TABLE raw_versions (
    id            bigserial PRIMARY KEY,
    path          text NOT NULL,
    content_sha   bytea NOT NULL REFERENCES raw_blobs (content_sha),
    report_date   timestamptz,
    upstream_modified text NOT NULL DEFAULT '',
    fetched_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX raw_versions_path_idx ON raw_versions (path, id DESC);
CREATE INDEX raw_versions_fetched_idx ON raw_versions (fetched_at);

-- Latest known validators per path, so a restart can keep using 304s.
CREATE TABLE fetch_state (
    path          text PRIMARY KEY,
    etag          text NOT NULL DEFAULT '',
    last_modified text NOT NULL DEFAULT '',
    content_sha   bytea,
    last_status   integer NOT NULL,
    checked_at    timestamptz NOT NULL DEFAULT now(),
    changed_at    timestamptz
);

-- +goose Down
DROP TABLE fetch_state;
DROP TABLE raw_versions;
DROP TABLE raw_blobs;

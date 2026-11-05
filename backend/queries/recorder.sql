-- name: GetFetchState :one
SELECT etag, last_modified, content_sha, last_status FROM fetch_state WHERE path = $1;

-- name: InsertBlob :exec
INSERT INTO raw_blobs (content_sha, gz, raw_size) VALUES ($1, $2, $3)
ON CONFLICT (content_sha) DO NOTHING;

-- name: InsertVersion :one
INSERT INTO raw_versions (path, content_sha, report_date, upstream_modified)
VALUES ($1, $2, $3, $4) RETURNING id;

-- name: UpsertChanged :exec
INSERT INTO fetch_state (path, etag, last_modified, content_sha, last_status, changed_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (path) DO UPDATE SET
    etag = EXCLUDED.etag, last_modified = EXCLUDED.last_modified,
    content_sha = EXCLUDED.content_sha, last_status = EXCLUDED.last_status,
    checked_at = now(), changed_at = now();

-- name: UpsertUnchanged :exec
INSERT INTO fetch_state (path, etag, last_modified, last_status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (path) DO UPDATE SET
    etag = EXCLUDED.etag, last_modified = EXCLUDED.last_modified,
    last_status = EXCLUDED.last_status, checked_at = now();

-- name: LatestBody :one
SELECT b.gz FROM fetch_state s JOIN raw_blobs b USING (content_sha) WHERE s.path = $1;

-- name: RecorderStats :one
SELECT
    (SELECT count(*) FROM raw_versions)::bigint AS versions,
    (SELECT count(*) FROM fetch_state)::bigint AS paths,
    (SELECT coalesce(sum(length(gz)), 0) FROM raw_blobs)::bigint AS stored_bytes,
    (SELECT max(fetched_at) FROM raw_versions)::timestamptz AS last_change;

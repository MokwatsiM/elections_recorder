-- name: LoadEngineState :one
SELECT cursor, seq FROM engine_state WHERE id = 1;

-- name: SaveEngineState :exec
UPDATE engine_state SET cursor = $1, seq = $2 WHERE id = 1;

-- name: InsertEvent :exec
INSERT INTO events (seq, type, at, topics, data) VALUES ($1, $2, $3, $4, $5);

-- name: InsertSnapshot :exec
INSERT INTO area_snapshots (level, area_id, at, data) VALUES ($1, $2, $3, $4);

-- name: FeedBefore :many
SELECT seq, type, at, data FROM events WHERE seq < $1 ORDER BY seq DESC LIMIT $2;

-- name: Timeline :many
SELECT at, data FROM area_snapshots WHERE level = $1 AND area_id = $2 ORDER BY at DESC LIMIT $3;

-- name: HydrationVersions :many
SELECT DISTINCT ON (v.path) v.id, v.path, v.fetched_at, b.gz
FROM raw_versions v JOIN raw_blobs b USING (content_sha)
WHERE v.id <= $1 AND v.path LIKE $2
ORDER BY v.path, v.id DESC;

-- name: VersionsAfter :many
SELECT v.id, v.path, v.fetched_at, b.gz
FROM raw_versions v JOIN raw_blobs b USING (content_sha)
WHERE v.id > $1 AND v.path LIKE $2
ORDER BY v.id LIMIT $3;

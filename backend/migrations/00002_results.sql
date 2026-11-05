-- +goose Up

-- The engine's position in raw_versions, saved with the output of each version.
CREATE TABLE engine_state (
    id     smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    cursor bigint NOT NULL DEFAULT 0,
    seq    bigint NOT NULL DEFAULT 0
);
INSERT INTO engine_state DEFAULT VALUES;

CREATE TABLE events (
    seq    bigint PRIMARY KEY,
    type   text NOT NULL,
    at     timestamptz NOT NULL,
    topics text[] NOT NULL,
    data   jsonb NOT NULL
);

-- National to municipality only. Powers race-over-time charts.
CREATE TABLE area_snapshots (
    level   text NOT NULL,
    area_id integer NOT NULL,
    at      timestamptz NOT NULL,
    data    jsonb NOT NULL
);
CREATE INDEX area_snapshots_idx ON area_snapshots (level, area_id, at DESC);

-- +goose Down
DROP TABLE area_snapshots;
DROP TABLE events;
DROP TABLE engine_state;

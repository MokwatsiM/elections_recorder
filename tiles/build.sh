#!/usr/bin/env bash
# Builds tiles/sa.pmtiles from recorded IEC boundaries. Needs Postgres running.
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p raw
(cd ../backend && go run ./cmd/exportgeo ../tiles/raw)
docker build -q -t lge-tippecanoe . >/dev/null
run() { docker run --rm -v "$PWD:/w" -w /w lge-tippecanoe "$@"; }
common=(--force --no-tile-compression --coalesce-densest-as-needed --detect-shared-borders --no-tile-size-limit --extend-zooms-if-still-dropping)
run tippecanoe "${common[@]}" -o raw/p.mbtiles -l provinces      -Z0 -z8  raw/provinces.geojson
run tippecanoe "${common[@]}" -o raw/m.mbtiles -l municipalities -Z3 -z10 raw/municipalities.geojson
run tippecanoe "${common[@]}" -o raw/w.mbtiles -l wards          -Z5 -z12 raw/wards.geojson
run tippecanoe "${common[@]}" -o raw/v.mbtiles -l vds            -Z7 -z13 raw/vds.geojson
run tile-join --force --no-tile-size-limit -o sa.pmtiles raw/p.mbtiles raw/m.mbtiles raw/w.mbtiles raw/v.mbtiles
ls -lh sa.pmtiles
python3 -I bounds.py

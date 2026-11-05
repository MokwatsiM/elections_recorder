# 2026 Local Elections: live results

An unofficial, live dashboard for South Africa's 2026 Local Government Elections. It covers the country, provinces, municipalities, wards and voting districts.

A Go service watches the IEC's published results files, records every change to Postgres and streams updates to a Next.js frontend over Server-Sent Events. Browsers never contact the IEC, so audience size adds no load upstream.

The recorder always runs. Browsers only open the stream when the viewer turns on "Live updates", which is off by default and remembered per browser. Otherwise figures change only on Refresh or a page load.

## Run it

Requires Docker, Node 20.9+, pnpm and Go 1.26.

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

This starts Postgres and the backend on port 8080, and recording begins straight away. The first run also downloads boundaries, party data and past elections, which takes about 10 minutes at the polite request rate.

The backend expects `tiles/sa.pmtiles`, which isn't in git. Build it once the first run has finished downloading:

```bash
./tiles/build.sh
```

Then start the frontend at http://localhost:3000:

```bash
pnpm --dir frontend install && pnpm --dir frontend dev
```

## Replay a recorded count

This reads recorded data and writes nothing, so it's safe beside a live recorder. The UI labels it as a replay. Turn on "Live updates" to watch it.

```bash
cd backend && REPLAY_SPEED=30 LISTEN_ADDR=:8081 SSE_MAX_PER_IP=100000 TILES_FILE=../tiles/sa.pmtiles go run ./cmd/server
```

Point a second frontend at it:

```bash
cd frontend && NEXT_DIST_DIR=.next-replay BACKEND_URL=http://localhost:8081 pnpm exec next dev -p 3001
```

## Checks

Backend tests:

```bash
cd backend && go test -race ./...
```

Compare the API with the IEC's files at every level:

```bash
python3 scripts/accuracy-check.py
```

Load test against a replay server, with raised file limits:

```bash
cd backend && go run ./cmd/loadtest -base http://localhost:8081 -conns 10000 -for 1m
```

## Deploy

Copy `deploy/.env.example` to `deploy/.env` and fill it in, then:

```bash
docker compose -f deploy/docker-compose.yml --profile public up -d --build
```

This adds the frontend and Caddy. Before going public:

- Check that the IEC's terms allow republishing, and set `IEC_USER_AGENT` so they can contact you.
- Put Cloudflare in front, turn off buffering for `/api/v1/stream`, and set `TRUST_PROXY=true`.
- Run only one public replica of the backend: a second one would wait on the poller lock and hold no results of its own.

## Recorder on a VPS

`scripts/move-recorder-to-vps.sh user@host` moves the recorder and its data to `/opt/lge`, reachable only from the server on port 18080. Only one recorder should run at a time. To use a local frontend against it:

```bash
ssh -N -L 8080:localhost:18080 user@host
```

### Private access

The frontend also runs on the VPS, with no public port. It's reached only through a Cloudflare Tunnel to `http://frontend:3000`, with a Cloudflare Access login in front. Set up the Access policy before the tunnel's public hostname, and keep `TUNNEL_TOKEN` in `/opt/lge/deploy/.env` on the server. A new Access policy can take a few minutes to apply, so confirm that a logged-out request gets a 302 to the login page before connecting the tunnel. Compose commands on the server need `--env-file deploy/.env` and both compose files, plus `--profile private --profile tunnel`.

Back up the recording to this machine:

```bash
scripts/backup-from-vps.sh root@187.127.231.242
```

## CI/CD

Pushing to `main` runs GitHub Actions, which builds Docker images into the GitHub container registry. The server pulls images and never builds them.

- **Frontend:** tests, builds and deploys automatically.
- **Backend:** tests and builds automatically, but deploys only when you run the "Deploy backend" workflow, because that restarts the recorder.
- **Rollback:** a deploy that fails its health check goes back to the previous image by itself. To go back later, run the workflow again with an older commit SHA, or re-run an older frontend run.

The deploy key can only run `deploy/lge-deploy.sh` on the server. Run `scripts/install-deploy-key.sh root@187.127.231.242` to set it up or replace it, and again after changing that script. `.env` and `tiles/sa.pmtiles` stay on the server and aren't in git.

## How the data works

There's no IEC results API. The official dashboard reads static JSON files through Cloudflare:

| Level | File, under `dashboards/lge/MapsJason/2026/` |
|---|---|
| Country | `EEID1887LiteMapsNational.js` |
| Province | `EEID1887Province{1-9}.js` |
| Municipality, ward, voting district | `EEID1887Munic{id}.js` |

Boundaries are in `ElectionMap/MapData/2026/`. Things that aren't obvious from the files:

- **Timing:** files regenerate about every 6 minutes, municipalities first and the national file about 4 minutes later. So the poller watches one municipality file as an early signal.
- **Ballots, not voters:** each voter casts several ballots. `TotalVotesCast / RegisteredVoters` exceeds 100% on finished councils, so it isn't turnout, and the files contain no turnout figure.
- **Largest isn't control:** `IsCouncilWinner` and `CouncilsWon` mean the most seats, even in a hung council. Only `MajorityWon` means control.
- **Files disagree briefly:** each level is generated separately, so we publish each as reported and never re-sum them.
- **Empty files:** some files are served as an empty 200, and they must not be treated as a block.

# go-inventory-chat

Inventory backend module host for the launcher-first `go-go-os` runtime.

## Run (from repo root)

Build and run the single launcher binary (embedded frontend + backend modules):

```bash
npm run launcher:binary:build
./build/go-go-os-launcher go-go-os-launcher \
  --addr :8091 \
  --inventory-db ./go-inventory-chat/data/inventory.db \
  --timeline-db ./go-inventory-chat/data/webchat-timeline.db \
  --turns-db ./go-inventory-chat/data/webchat-turns.db
```

Run directly without prebuilding the binary:

```bash
go run ./go-inventory-chat/cmd/go-go-os-launcher go-go-os-launcher \
  --addr :8091 \
  --inventory-db ./go-inventory-chat/data/inventory.db \
  --timeline-db ./go-inventory-chat/data/webchat-timeline.db \
  --turns-db ./go-inventory-chat/data/webchat-turns.db
```

## Key routes

- `GET /` launcher shell (embedded `apps/os-launcher` build)
- `GET /api/os/apps` backend module manifest + health
- `GET /api/os/apps/<app-id>/reflection` module reflection payload (if implemented)
- `POST /api/apps/inventory/chat`
- `GET /api/apps/inventory/ws?conv_id=<id>`
- `GET /api/apps/inventory/api/timeline?conv_id=<id>`
- `GET /api/apps/inventory/api/chat/profiles`
- `GET /api/apps/gepa/scripts`
- `POST /api/apps/gepa/runs`
- `GET /api/apps/gepa/runs/<run-id>`
- `POST /api/apps/gepa/runs/<run-id>/cancel`
- `GET /api/apps/gepa/schemas/<schema-id>`

Hard-cut route policy:

- legacy aliases are intentionally blocked: `/chat`, `/ws`, `/api/timeline`

## Validation

```bash
cd go-inventory-chat
go test ./...
```

```bash
cd ..
npm run launcher:smoke
```

## GEPA module config and curl runbook

The launcher now mounts a non-required internal GEPA backend module under
`/api/apps/gepa/*`.

Script catalog roots are controlled via:

```bash
--gepa-scripts-root "/path/to/scripts,/another/path"
--gepa-run-timeout-seconds 30
--gepa-max-concurrent-runs 4
```

List scripts:

```bash
curl -s localhost:8091/api/apps/gepa/scripts | jq
```

Start a run:

```bash
curl -s -X POST localhost:8091/api/apps/gepa/runs \
  -H 'Content-Type: application/json' \
  -d '{"script_id":"example.js","arguments":["--dry-run"]}' | jq
```

Get run status:

```bash
curl -s localhost:8091/api/apps/gepa/runs/<run-id> | jq
```

Cancel a run:

```bash
curl -s -X POST localhost:8091/api/apps/gepa/runs/<run-id>/cancel | jq
```

Inspect reflection and schemas:

```bash
curl -s localhost:8091/api/os/apps/gepa/reflection | jq
curl -s localhost:8091/api/apps/gepa/schemas/gepa.runs.start.request.v1 | jq
```

## Notes

- Runtime key is locked to `inventory`.
- Runtime overrides are rejected in the request resolver.
- Model/provider selection uses Geppetto/Glazed CLI sections.

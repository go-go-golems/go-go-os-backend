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
- `POST /api/apps/inventory/chat`
- `GET /api/apps/inventory/ws?conv_id=<id>`
- `GET /api/apps/inventory/api/timeline?conv_id=<id>`
- `GET /api/apps/inventory/api/chat/profiles`

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

## Notes

- Runtime key is locked to `inventory`.
- Runtime overrides are rejected in the request resolver.
- Model/provider selection uses Geppetto/Glazed CLI sections.

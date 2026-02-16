# go-inventory-chat

Inventory chat backend using Glazed + Pinocchio webchat.

## Run

From repository root:

```bash
go run ./2026-02-12--hypercard-react/go-inventory-chat/cmd/hypercard-inventory-server hypercard-inventory-server \
  --addr :8091 \
  --timeline-db ./2026-02-12--hypercard-react/go-inventory-chat/data/webchat-timeline.db \
  --turns-db ./2026-02-12--hypercard-react/go-inventory-chat/data/webchat-turns.db
```

## Key routes

- `POST /chat`
- `GET /ws?conv_id=<id>`
- `GET /api/timeline?conv_id=<id>`

## Notes

- Runtime key is locked to `inventory`.
- Runtime overrides are rejected in the request resolver.
- Model/provider selection uses Geppetto/Glazed CLI sections.


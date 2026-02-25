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

## End-to-end runbook

Backend tmux session:

```bash
tmux new-session -d -s hc033-backend 'cd /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/2026-02-12--hypercard-react/go-inventory-chat && go run ./cmd/hypercard-inventory-server hypercard-inventory-server --addr :8091 --timeline-db ./data/webchat-timeline.db --turns-db ./data/webchat-turns.db'
```

Frontend tmux session:

```bash
tmux new-session -d -s hc033-frontend 'cd /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/2026-02-12--hypercard-react && INVENTORY_CHAT_BACKEND=http://127.0.0.1:8091 pnpm dev -- --host 127.0.0.1 --port 5173'
```

Round-trip smoke:

```bash
cd /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/2026-02-12--hypercard-react
node ttmp/2026/02/16/HC-033-ADD-WEBCHAT-INTEGRATION--add-webchat-integration/scripts/smoke-roundtrip-playwright.mjs
```

Reload/hydration smoke:

```bash
cd /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/2026-02-12--hypercard-react
node ttmp/2026/02/16/HC-033-ADD-WEBCHAT-INTEGRATION--add-webchat-integration/scripts/smoke-reload-hydration-playwright.mjs
```

Focused validation:

```bash
cd /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/2026-02-12--hypercard-react/go-inventory-chat
go test ./...
```

```bash
cd /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/2026-02-12--hypercard-react
pnpm -C apps/inventory exec tsc --noEmit
npm exec vitest run apps/inventory/src/features/chat/chatSlice.test.ts apps/inventory/src/features/chat/artifactsSlice.test.ts apps/inventory/src/features/chat/artifactRuntime.test.ts apps/inventory/src/features/chat/InventoryChatWindow.timeline.test.ts
```

## Key routes

- `POST /api/apps/inventory/chat`
- `GET /api/apps/inventory/ws?conv_id=<id>`
- `GET /api/apps/inventory/api/timeline?conv_id=<id>`
- `GET /api/os/apps`

## Notes

- Runtime key is locked to `inventory`.
- Runtime overrides are rejected in the request resolver.
- Model/provider selection uses Geppetto/Glazed CLI sections.

## Known limitations

- Structured card/widget/suggestion blocks are model-authored and optional; no fallback synthesis path exists.
- Some repository-wide Storybook/Vite worker build issues are tracked outside this ticket and can still affect full production/story builds.

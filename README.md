# go-go-os-backend

Backend host contracts and lifecycle for launcher app modules.

## Overview

This repository owns the `backendhost` package extracted from `go-go-os`.
It provides shared contracts and wiring helpers used by launcher-composition repos.

Primary package:

- `pkg/backendhost`

## Package Highlights

- App backend module contract (`AppBackendModule`)
- Lifecycle manager (`Init` / `Start` / `Health` / `Stop` sequencing)
- Namespaced route mounting under `/api/apps/<app-id>`
- Manifest and reflection endpoint registration (`/api/os/apps`)
- Legacy alias guardrails for hard-cut route policy

## Install

```bash
go get github.com/go-go-golems/go-go-os-backend@latest
```

## Usage

```go
import "github.com/go-go-golems/go-go-os-backend/pkg/backendhost"
```

## Development

```bash
make lint
make test
make build
```

## License

MIT. See `LICENSE`.

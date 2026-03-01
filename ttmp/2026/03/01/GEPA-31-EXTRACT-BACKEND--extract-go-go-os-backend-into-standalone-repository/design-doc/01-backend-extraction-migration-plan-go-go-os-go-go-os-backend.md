---
Title: Backend Extraction Migration Plan (go-go-os -> go-go-os-backend)
Ticket: GEPA-31-EXTRACT-BACKEND
Status: active
Topics:
    - go-go-os
    - go
    - migration
    - architecture
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: workspaces/2026-02-22/add-gepa-optimizer/go-go-os-backend/.goreleaser.yaml
      Note: Destination release scaffold currently placeholder
    - Path: workspaces/2026-02-22/add-gepa-optimizer/go-go-os-backend/go.mod
      Note: |-
        Destination module currently placeholder and must be normalized
        Normalized module identity after extraction
    - Path: workspaces/2026-02-22/add-gepa-optimizer/go-go-os-backend/pkg/backendhost/module.go
      Note: Extracted backendhost contract now owned in destination repo
    - Path: workspaces/2026-02-22/add-gepa-optimizer/go-go-os/.github/workflows/launcher-ci.yml
      Note: |-
        Source repo CI currently runs nested backend tests
        Source CI decoupled from removed nested backend module
    - Path: workspaces/2026-02-22/add-gepa-optimizer/go-go-os/README.md
      Note: Source ownership boundary updated after extraction
    - Path: workspaces/2026-02-22/add-gepa-optimizer/go-go-os/go-go-os/go.mod
      Note: Current backend module path and extraction boundary
    - Path: workspaces/2026-02-22/add-gepa-optimizer/go-go-os/go-go-os/pkg/backendhost/module.go
      Note: Primary backendhost API contract to extract
    - Path: workspaces/2026-02-22/add-gepa-optimizer/wesen-os/cmd/wesen-os-launcher/main.go
      Note: Concrete downstream import rewrite target
    - Path: workspaces/2026-02-22/add-gepa-optimizer/wesen-os/go.mod
      Note: |-
        Downstream dependency and replace rewiring required
        Downstream dependency rewiring to go-go-os-backend
ExternalSources: []
Summary: Prescriptive migration plan to extract go-go-os backendhost Go module into go-go-os-backend with history preserved and downstream imports updated.
LastUpdated: 2026-03-01T11:18:00-05:00
WhatFor: Execute a low-risk repository split with commit history preservation and deterministic downstream rewiring.
WhenToUse: Use when implementing GEPA-31 or reviewing the extraction rollout.
---



# Backend Extraction Migration Plan (go-go-os -> go-go-os-backend)

## Executive Summary

Extract the backend Go module currently under `go-go-os/go-go-os/*` into the standalone repository `go-go-os-backend` while preserving history with `git filter-repo`, then rewire all consumers from `github.com/go-go-golems/go-go-os/pkg/backendhost` to `github.com/go-go-golems/go-go-os-backend/pkg/backendhost`.

The safest approach is:

1. Build a filtered history branch from `go-go-os` including both historical paths (`go-inventory-chat/` and `go-go-os/`).
2. Merge that branch into `go-go-os-backend` (unrelated histories allowed), then normalize module/scaffold files.
3. Update downstream consumers (`wesen-os`) and source repo references (`go-go-os` README + CI).
4. Validate each repo independently and cut a release from `go-go-os-backend` before removing local `replace` directives.

## Implementation Result (2026-03-01)

Migration tasks `I1` through `I6` were executed end-to-end.

### Completed Changes

1. History extraction and import into `go-go-os-backend` completed:
   - `go-go-os-backend` commit `e0ca8bf` (`merge: import backendhost history from go-go-os`)
2. Destination scaffold/module normalization completed:
   - `go-go-os-backend` commit `4c73c42` (`chore: normalize go-go-os-backend scaffold after history import`)
   - module path now `github.com/go-go-golems/go-go-os-backend`
3. Downstream consumer rewiring completed:
   - `wesen-os` commit `a5bd49a` (`refactor: consume go-go-os-backend backendhost module`)
4. Source repo cleanup completed:
   - `go-go-os` commit `0798467` (`refactor: extract backendhost module ownership to go-go-os-backend`)
   - removed nested `go-go-os/` module and backendhost CI job from `go-go-os`

### Validation Results

1. `go-go-os-backend`
   - `go test ./... -count=1` passed
   - `make lint` passed
2. `wesen-os`
   - `go test ./... -count=1` passed
3. `go-go-os`
   - `npm run build` passed
   - `npm run test` passed
4. Import sanity check
   - no live code matches for `github.com/go-go-golems/go-go-os/pkg/backendhost`

### Notable Deviations From Planned Draft

1. A minimal `cmd/go-go-os-backend` entrypoint was kept so scaffold smoke/release wiring remains valid.
2. The merge path used an unrelated-history merge in the target repo rather than force-rewriting target branch history.

## Problem Statement And Scope

### Goal

Move the Go backend host component from:

- `go-go-os/go-go-os/*`

into:

- `go-go-os-backend/*`

while preserving meaningful history and leaving `go-go-os` as a frontend/platform-focused repository.

### In Scope

1. History-preserving extraction of backend Go files.
2. Module path migration to `github.com/go-go-golems/go-go-os-backend`.
3. Downstream import and `go.mod` rewiring.
4. CI/readme cleanup in `go-go-os`.
5. Alignment of `go-go-os-backend` with standard go-go-golems scaffold conventions.

### Out Of Scope

1. Feature changes to backendhost behavior.
2. API contract changes under `pkg/backendhost`.
3. Non-Go frontend architecture changes.

## Current-State Analysis (Evidence-Backed)

### Source module currently nested in go-go-os

1. The current Go module is nested under `go-go-os/go-go-os` and declares module path `github.com/go-go-golems/go-go-os`.
2. Backend host package files live at `go-go-os/go-go-os/pkg/backendhost/*.go`.
3. The top-level `go-go-os` README still documents this nested module location.

### Downstream consumers depend on old module path

`wesen-os` imports `backendhost` from the old module path in these files:

1. `wesen-os/cmd/wesen-os-launcher/main.go`
2. `wesen-os/cmd/wesen-os-launcher/main_integration_test.go`
3. `wesen-os/cmd/wesen-os-launcher/inventory_backend_module.go`
4. `wesen-os/pkg/gepa/module.go`
5. `wesen-os/pkg/arcagi/module.go`

`wesen-os/go.mod` currently requires and locally replaces `github.com/go-go-golems/go-go-os`.

### Source repo CI still couples to nested backend module

`go-go-os/.github/workflows/launcher-ci.yml` includes `backendhost-go-tests` and executes `go test ./...` inside `go-go-os/` (nested Go module directory).

### Destination repo exists but is still placeholder scaffold

`go-go-os-backend` currently has scaffold placeholders (`XXX`) in:

1. `go.mod` module path (`github.com/go-go-golems/XXX`)
2. `Makefile` binary/install targets
3. `.goreleaser.yaml` project/binary/homepage fields
4. `cmd/XXX/main.go`

### History detail that affects extraction strategy

The backend subtree was renamed from `go-inventory-chat/` to `go-go-os/` in commit `6d4302a`.

Implication: extraction must include both path roots to preserve full ancestry.

## Gap Analysis

1. Desired module path (`github.com/go-go-golems/go-go-os-backend`) does not exist in code yet.
2. Consumer imports still target `github.com/go-go-golems/go-go-os/pkg/backendhost`.
3. `go-go-os-backend` scaffolding is not yet filled with real module metadata.
4. `go-go-os` docs and CI still assume backend Go code is inside that repo.
5. Without path-aware history extraction (`go-inventory-chat/` + `go-go-os/`), early backend history is lost.

## Proposed Target State

1. `go-go-os-backend` becomes the canonical backendhost module repo.
2. `go.mod` in target repo declares:

```go
module github.com/go-go-golems/go-go-os-backend
```

3. Public import path becomes:

```go
import "github.com/go-go-golems/go-go-os-backend/pkg/backendhost"
```

4. `go-go-os` no longer contains `go-go-os/` nested Go module, and no longer runs backendhost Go tests in its CI.
5. `wesen-os` depends on `go-go-os-backend` instead of `go-go-os` for backendhost contracts.

## Migration Strategy (Prescriptive)

### Phase 0: Preconditions And Freeze Window

1. Announce short migration freeze on backendhost touching PRs in both repos.
2. Confirm clean working trees:

```bash
git -C /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-os status --porcelain
git -C /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-os-backend status --porcelain
git -C /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/wesen-os status --porcelain
```

3. Create working branches:

```bash
git -C /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-os switch -c gepa31/source-cleanup
git -C /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-os-backend switch -c gepa31/extract-backend
git -C /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/wesen-os switch -c gepa31/import-rewire
```

### Phase 1: Extract History With git filter-repo

Use a temporary clone so source repo is never rewritten in place.

```bash
ROOT=/home/manuel/workspaces/2026-02-22/add-gepa-optimizer
TMP=$(mktemp -d /tmp/gepa31-extract.XXXXXX)

git clone --no-local "$ROOT/go-go-os" "$TMP/go-go-os-filtered"
cd "$TMP/go-go-os-filtered"
git switch -c gepa31/extract-backend

git filter-repo --force \
  --path go-inventory-chat/ \
  --path go-go-os/ \
  --path-rename go-inventory-chat/: \
  --path-rename go-go-os/:
```

Verification after filtering:

```bash
git log --oneline --decorate -n 20
git ls-tree --name-only -r HEAD | sed -n '1,80p'
test -f go.mod && test -d pkg/backendhost
```

### Phase 2: Merge Filtered History Into go-go-os-backend

Recommended approach: merge unrelated histories (avoid force-pushing over existing default branch history).

```bash
cd "$ROOT/go-go-os-backend"
git remote add gepa31-filter "$TMP/go-go-os-filtered"
git fetch gepa31-filter gepa31/extract-backend

git merge --allow-unrelated-histories gepa31-filter/gepa31/extract-backend
```

If conflicts occur on `go.mod`, `README.md`, `Makefile`, `.gitignore`, `.golangci.yml`:

1. Take extracted content first (ensures backend code lands exactly as historical state).
2. Re-apply/normalize scaffold in next phase.

Example conflict resolution command set:

```bash
git checkout --theirs go.mod go.sum README.md Makefile .gitignore .golangci.yml pkg/backendhost
git add go.mod go.sum README.md Makefile .gitignore .golangci.yml pkg/backendhost
git commit -m "merge: import backendhost history from go-go-os"
```

### Phase 3: Normalize go-go-os-backend To Standard Scaffold

Run the standard scaffold tool from `go-go-golems-project-setup` with actual identifiers.

```bash
python3 /home/manuel/.codex/skills/go-go-golems-project-setup/scripts/scaffold.py \
  --module github.com/go-go-golems/go-go-os-backend \
  --binary go-go-os-backend \
  --project-name go-go-os-backend \
  --description "Backend host contracts and lifecycle for go-go-os app modules" \
  --force
```

Then enforce library-first adjustments (because this repo is package-focused):

1. Remove template placeholder CLI if not needed.
2. Remove/adjust `.goreleaser.yaml` binary-oriented sections (`builds`, `brews`, `nfpms`, `publishers`) unless a real CLI is introduced.
3. Keep CI/lint/security workflows and Go quality gates.

Normalize module metadata:

```bash
cd "$ROOT/go-go-os-backend"
go mod edit -module github.com/go-go-golems/go-go-os-backend
go mod tidy
```

### Phase 4: Downstream Import Rewire (wesen-os)

#### A) Rewrite imports

Old -> new import path mapping:

- `github.com/go-go-golems/go-go-os/pkg/backendhost`
- `github.com/go-go-golems/go-go-os-backend/pkg/backendhost`

Apply rewrites to:

1. `wesen-os/cmd/wesen-os-launcher/main.go`
2. `wesen-os/cmd/wesen-os-launcher/main_integration_test.go`
3. `wesen-os/cmd/wesen-os-launcher/inventory_backend_module.go`
4. `wesen-os/pkg/gepa/module.go`
5. `wesen-os/pkg/arcagi/module.go`

Command:

```bash
cd "$ROOT/wesen-os"
rg -l 'github.com/go-go-golems/go-go-os/pkg/backendhost' --glob '*.go' | \
  xargs -I{} perl -0pi -e 's#github\.com/go-go-golems/go-go-os/pkg/backendhost#github.com/go-go-golems/go-go-os-backend/pkg/backendhost#g' {}
```

#### B) Update module dependencies

```bash
cd "$ROOT/wesen-os"
go mod edit -droprequire=github.com/go-go-golems/go-go-os || true
go mod edit -require=github.com/go-go-golems/go-go-os-backend@v0.0.0-00010101000000-000000000000
go mod edit -dropreplace=github.com/go-go-golems/go-go-os || true
go mod edit -replace=github.com/go-go-golems/go-go-os-backend=../go-go-os-backend
go mod tidy
```

### Phase 5: Source Repo Cleanup (go-go-os)

1. Remove extracted Go subtree from source repo:

```bash
cd "$ROOT/go-go-os"
git rm -r go-go-os
```

2. Update `README.md` to remove backend module ownership references.
3. Update `.github/workflows/launcher-ci.yml` to remove `backendhost-go-tests` job.

### Phase 6: Validation Matrix

Run in this order.

#### go-go-os-backend

```bash
cd "$ROOT/go-go-os-backend"
GOWORK=off go test ./... -count=1
GOWORK=off make lint
```

#### wesen-os

```bash
cd "$ROOT/wesen-os"
go test ./... -count=1
```

#### go-go-os

```bash
cd "$ROOT/go-go-os"
npm run build
npm run test
```

#### Cross-repo sanity checks

```bash
cd "$ROOT"
rg -n 'github.com/go-go-golems/go-go-os/pkg/backendhost' wesen-os go-go-os go-go-os-backend -S
rg -n '^module github.com/go-go-golems/go-go-os-backend$' go-go-os-backend/go.mod -S
```

Expected result: first `rg` returns no code hits outside historical docs.

### Phase 7: PR And Release Sequencing

Merge in dependency-safe order:

1. `go-go-os-backend` extraction + scaffold normalization.
2. Tag and release `go-go-os-backend` (for example `v0.1.0`).
3. `wesen-os` import/go.mod update to released version (remove local `replace` when done).
4. `go-go-os` source cleanup (remove nested backend code + CI job + docs).

## Risks, Mitigations, And Rollback

### Risk 1: History loss due incomplete path filter

Mitigation: include both `go-inventory-chat/` and `go-go-os/` in filter command.

Rollback: discard temp filtered repo and rerun extraction.

### Risk 2: Conflict-heavy unrelated-history merge

Mitigation: merge in isolated branch; resolve by taking extracted backend files first, then scaffold normalization commit.

Rollback: reset branch to pre-merge commit; rerun with clean strategy.

### Risk 3: Downstream breakage in wesen-os

Mitigation: import rewrite + `go mod tidy` + full `go test ./...` before merge.

Rollback: temporarily keep `replace github.com/go-go-golems/go-go-os => ../go-go-os/go-go-os` until backend repo release is stable.

## Alternatives Considered

### 1) `git subtree split`

Pros: built into git, no extra tool.

Cons: path rename handling across `go-inventory-chat/` -> `go-go-os/` is less explicit; harder to produce deterministic root rewrite in one pass.

### 2) Copy files without history

Pros: fastest.

Cons: loses authorship and regression archaeology; rejected.

### 3) Force-push filtered history directly onto go-go-os-backend/main

Pros: simplest DAG for extracted files.

Cons: rewrites destination default branch; acceptable only if branch policy allows and team signs off.

## Open Questions

1. Keep `go-go-os-backend` as library-only (no CLI binary) or retain a tiny CLI for release automation compatibility?
2. Should `go-go-os` retain a compatibility README pointer to `go-go-os-backend` for one release cycle?
3. Is branch protection configured to permit unrelated-history merges on `go-go-os-backend`?

## References

1. `/home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-os/go-go-os/go.mod`
2. `/home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-os/go-go-os/pkg/backendhost/module.go`
3. `/home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-os/.github/workflows/launcher-ci.yml`
4. `/home/manuel/workspaces/2026-02-22/add-gepa-optimizer/wesen-os/go.mod`
5. `/home/manuel/workspaces/2026-02-22/add-gepa-optimizer/wesen-os/cmd/wesen-os-launcher/main.go`
6. `/home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-os-backend/go.mod`
7. `/home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-os-backend/.goreleaser.yaml`

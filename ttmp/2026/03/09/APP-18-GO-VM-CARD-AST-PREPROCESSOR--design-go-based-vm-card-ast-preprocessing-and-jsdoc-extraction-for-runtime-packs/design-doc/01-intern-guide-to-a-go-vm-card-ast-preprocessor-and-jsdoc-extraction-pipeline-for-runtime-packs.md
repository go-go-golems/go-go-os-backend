---
Title: Intern guide to a Go VM card AST preprocessor and jsdoc extraction pipeline for runtime packs
Ticket: APP-18-GO-VM-CARD-AST-PREPROCESSOR
Status: active
Topics:
    - architecture
    - frontend
    - hypercard
    - wesen-os
    - goja
    - tooling
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../go-go-goja/cmd/goja-jsdoc/doc/01-jsdoc-system.md
      Note: |-
        Existing user guide for jsdoc extraction that should inform the new preprocessor design
        Existing intern-facing jsdoc extraction guide that informs the new pipeline
    - Path: ../../../../../../../go-go-goja/pkg/jsdoc/batch/batch.go
      Note: |-
        Existing multi-input orchestration seam for pack documentation batches
        Reusable multi-file batch builder for pack and card docs extraction
    - Path: ../../../../../../../go-go-goja/pkg/jsdoc/extract/extract.go
      Note: |-
        Existing jsdoc extraction pipeline to reuse for pack and card documentation
        Existing jsdocex-derived extraction engine for package
    - Path: ../../../../../../../go-go-goja/pkg/jsdoc/model/model.go
      Note: |-
        Current doc model that informs whether card docs are represented as symbols or need extension
        Current doc model used to reason about representing card docs as symbols in v1
    - Path: ../../../../../../../go-go-goja/pkg/jsparse/treesitter.go
      Note: |-
        Existing tree-sitter parser wrapper to reuse for VM card structural analysis
        Existing tree-sitter parsing wrapper to reuse for structural VM card metadata extraction
    - Path: ../../../../../../../wesen-os/apps/os-launcher/src/domain/pluginBundle.authoring.d.ts
      Note: |-
        Current VM authoring API declaration that the generated docs and metadata should complement
        Current VM authoring API seam to complement with generated metadata and docs
    - Path: ../../../../../../../wesen-os/apps/os-launcher/src/domain/pluginBundle.vm.js
      Note: |-
        Current inline VM card bundle that should be split into preprocessable card source files
        Current inline VM bundle that motivates per-card file splitting
    - Path: ../../../../../../../wesen-os/workspace-links/go-go-os-backend/cmd/go-go-os-backend/main.go
      Note: |-
        Backend CLI home where the new preprocessor command should be added
        Backend CLI home where the new vmmeta command should live
    - Path: ../../../../../../../wesen-os/workspace-links/go-go-os-backend/go.mod
      Note: Backend module that should import github.com/go-go-golems/go-go-goja without modifying it
    - Path: ../../../../../../../wesen-os/workspace-links/go-go-os-frontend/packages/hypercard-runtime/src/runtime-packs/kanbanV1Pack.tsx
      Note: |-
        Concrete first runtime pack whose DSL surface should be documented by the generated docs
        Concrete first runtime pack whose DSL and renderer docs should become extractable
    - Path: ../../../../../../../wesen-os/workspace-links/go-go-os-frontend/packages/hypercard-runtime/src/runtime-packs/runtimePackRegistry.tsx
      Note: |-
        Current runtime-pack registry that would later consume generated metadata
        Runtime-pack consumer seam for generated metadata later on
ExternalSources: []
Summary: Detailed design and implementation guide for a Go-native preprocessing pipeline whose CLI lives in go-go-os-backend, whose parsing/docs libraries come from github.com/go-go-golems/go-go-goja, and whose outputs are generated manifests for runtime-pack builds.
LastUpdated: 2026-03-10T12:05:00-04:00
WhatFor: Use this guide before implementing the Go-based VM card preprocessor so the authoring format, command layout, generated outputs, and build integration points are explicit and reviewable.
WhenToUse: Use when implementing APP-18, onboarding an intern to the preprocessor architecture, or deciding how runtime packs should publish structural metadata and documentation from JS-authored sources.
---



# Intern guide to a Go VM card AST preprocessor and jsdoc extraction pipeline for runtime packs

## Executive Summary

We want runtime packs such as `kanban.v1` to stay authored in JavaScript, but we do not want humans to keep structural metadata and documentation in sync by hand. The next architecture step is therefore a build-time preprocessor that reads VM card sources, extracts machine-usable metadata, extracts human-usable documentation, and emits deterministic generated artifacts.

This ticket deliberately chooses Go as the implementation language. The main reason is not ideology. It is reuse. The local `go-go-goja` repo already contains two critical library building blocks:

- `github.com/go-go-golems/go-go-goja/pkg/jsparse`, which wraps JavaScript tree-sitter parsing in Go
- `github.com/go-go-golems/go-go-goja/pkg/jsdoc`, which already ports `jsdocex` extraction and export behavior into Go

The proposed system is a Go command owned by `go-go-os-backend` that scans runtime-pack source trees, extracts exact source and structural metadata from VM card files, extracts pack-level and per-card documentation from jsdoc-style sentinel docs, and writes generated outputs that `go-go-os-frontend` and `hypercard-runtime` can consume. The generator should run before frontend build steps and in CI, so build artifacts stay deterministic and reviewable.

This document explains the current system, the desired authoring conventions, the recommended command/package layout, the generated artifact formats, and the implementation phases in enough detail for a new intern to execute later.

## Problem Statement

Today, VM cards and pack definitions have several problems:

1. Structural metadata is implicit.
   - Card IDs, pack IDs, handler names, and sometimes even source ownership are embedded in handwritten JS, but not extracted into a manifest.
2. Source is difficult to inspect programmatically.
   - The runtime can execute code, but built-in stack cards do not automatically preserve exact per-card source in a structured way.
3. Pack documentation is not first-class.
   - We have runtime-pack code such as `kanbanV1Pack.tsx`, but no canonical extracted docs for the DSL or API surface exposed to VM authors.
4. Per-card documentation is also not first-class.
   - Demo cards and authored cards can carry intent and guidance, but we do not yet have a standard way to extract that prose into a browsable or exportable corpus.
5. The current authoring format is hard to preprocess.
   - Files like `apps/os-launcher/src/domain/pluginBundle.vm.js` define multiple cards inline in one large file. That makes exact per-card source extraction, documentation extraction, and build determinism much harder.

The goal of APP-18 is to design a system that solves these problems without introducing a Node-only preprocessing pipeline.

## Current System

### 1. The current VM card authoring surface

Right now, `os-launcher` and several other apps author VM stacks in a single `pluginBundle.vm.js` file.

Example seam:

- `apps/os-launcher/src/domain/pluginBundle.vm.js`
- `apps/os-launcher/src/domain/pluginBundle.authoring.d.ts`

That file currently mixes:

- stack bootstrap
- default state
- helper functions
- multiple `defineCard(...)` calls
- card handlers

This format is executable, but it is not preprocessing-friendly.

### 2. The current runtime-pack consumption surface

The runtime-pack registry is currently in:

- `packages/hypercard-runtime/src/runtime-packs/runtimePackRegistry.tsx`

And a concrete pack implementation is in:

- `packages/hypercard-runtime/src/runtime-packs/kanbanV1Pack.tsx`

Those files define:

- pack IDs
- tree validation
- host rendering

What they do not define is a build-time extraction story for:

- pack DSL/API docs
- per-card docs
- per-card exact source manifests

### 3. The current Go parser and docs foundation

The relevant local building blocks already exist in `go-go-goja`, and should be imported as libraries from `go-go-os-backend`:

#### `pkg/jsparse`

File:

- `go-go-goja/pkg/jsparse/treesitter.go`

What it gives us:

- a JavaScript tree-sitter parser in Go
- an owning `TSNode` snapshot type
- position information
- traversal helpers such as `NodeAtPosition`

This is the structural extraction foundation.

#### `pkg/jsdoc`

Files:

- `go-go-goja/pkg/jsdoc/extract/extract.go`
- `go-go-goja/pkg/jsdoc/model/model.go`
- `go-go-goja/pkg/jsdoc/batch/batch.go`
- `go-go-goja/cmd/goja-jsdoc/doc/01-jsdoc-system.md`

What they give us:

- jsdocex-style extraction of sentinel docs such as:
  - `__package__(...)`
  - `__doc__(...)`
  - `__example__(...)`
  - `doc\`...\``
- a current doc model with package, symbol, and example docs
- a batch builder that supports path and inline content parsing

This is the documentation extraction foundation.

### 4. The missing piece

What we do not yet have is a dedicated Go command in `go-go-os-backend` that combines both sides into one VM card preprocessing pipeline.

Current reality:

```text
VM cards execute
  but
no generated structural manifest exists
  and
no generated docs corpus exists for packs/cards
```

Desired future:

```text
VM source files
  ->
Go preprocessor
  |- tree-sitter structural extraction
  |- jsdoc documentation extraction
  ->
generated manifests + docs outputs
  ->
runtime/debugger/build systems consume generated artifacts
```

## Proposed Solution

### High-level architecture

The design has three layers:

1. Authoring layer
   - runtime pack sources and VM card files written in JS
   - pack docs and card docs written using jsdoc sentinel conventions
2. Preprocessing layer
   - a Go command in `go-go-os-backend` scans those files
   - that command imports `github.com/go-go-golems/go-go-goja/pkg/jsparse`
   - and `github.com/go-go-golems/go-go-goja/pkg/jsdoc/...`
   - tree-sitter extracts structure
   - jsdoc extraction collects docs
3. Consumption layer
   - generated JSON/TS artifacts are consumed by `wesen-os`, `hypercard-runtime`, debuggers, and later docs tooling

### Proposed data flow

```text
pack src/
  |- cards/*.vm.js
  |- docs/pack-api.js
  |- docs/examples/*.js
  |- manifest roots / config
        |
        v
go-go-os-backend vmmeta (new Go command)
  |- imported go-go-goja/pkg/jsparse -> structural metadata
  |- imported go-go-goja/pkg/jsdoc   -> docs extraction
        |
        +--> generated/vm_card_manifest.json
        +--> generated/vm_card_manifest.ts
        +--> generated/pack_docs.json
        +--> generated/source_index.json
        |
        v
frontend/runtime/debugger/build scripts
```

### Recommended command ownership

I recommend putting the new command in `go-go-os-backend`, while importing parser and docs libraries from `github.com/go-go-golems/go-go-goja` and not modifying that upstream repo.

Suggested layout:

```text
go-go-os-backend/
  cmd/
    go-go-os-backend/
      main.go
  pkg/
    vmmeta/
      model/
      scan/
      generate/
      docs/

imports:
  github.com/go-go-golems/go-go-goja/pkg/jsparse
  github.com/go-go-golems/go-go-goja/pkg/jsdoc/...
```

Why:

- the CLI should be project-owned by the backend repo that already ships local tooling
- the reusable parsing/docs libraries already exist in `go-go-goja`
- this preserves a clean dependency boundary: import the library, do not fork or patch it

### Recommended consumer integration

The frontend consumer repos should invoke the backend-owned command from build scripts.

Example:

```text
wesen-os/apps/os-launcher
  ->
npm/pnpm script or Make target
  ->
go run ../workspace-links/go-go-os-backend/cmd/go-go-os-backend vmmeta generate ...
  ->
generated manifests written into src/generated/
  ->
vite build consumes generated outputs
```

This preserves Go ownership of parsing while letting frontend builds stay frontend-native once the files are generated. It also keeps the executable in the backend repo that owns operational tooling, while still reusing `go-go-goja` as a library.

## Authoring Conventions

The preprocessor will only stay maintainable if the authoring format becomes more regular.

### 1. Split cards into per-card files

Do not keep a large pack worth of cards inline in one `pluginBundle.vm.js`.

Recommended structure:

```text
src/domain/
  cards/
    sprint-board.vm.js
    bug-triage.vm.js
    personal-planner.vm.js
  docs/
    pack-api.js
    examples.js
  generated/
    vm_card_manifest.ts
    pack_docs.json
  pluginBundle.vm.js
```

`pluginBundle.vm.js` should become an assembly file, not the source-of-truth location for each card definition.

### 2. Structural card metadata should be explicit

I recommend making each card file export explicit metadata in addition to executable content.

Suggested shape:

```js
export const meta = {
  cardId: "kanbanSprintBoard",
  packId: "kanban.v1",
  title: "Sprint Board",
  tags: ["demo", "kanban"],
};

export default ({ widgets }) => ({
  render({ state }) {
    return widgets.kanban.board(...);
  },
  handlers: {
    saveTask(context, args) { ... },
  },
});
```

Why explicit `meta`:

- easier for a Go AST walker than scraping arbitrary `defineCard(...)` calls
- easier to validate
- easier to keep deterministic
- makes the emitted manifest simpler

### 3. Pack docs should use jsdoc sentinel patterns

For pack-level docs and DSL/API docs, reuse the current jsdoc conventions.

Recommended pack docs file:

```js
__package__({
  name: "kanban.v1",
  title: "Kanban runtime pack",
  category: "hypercard-runtime-pack",
  description: "Declarative kanban board DSL for HyperCard VM cards."
});

__doc__("widgets.kanban.board", {
  summary: "Render a kanban board tree node.",
  tags: ["dsl", "kanban", "render"],
  concepts: ["runtime-pack", "ui-dsl"],
});

doc`
---
symbol: widgets.kanban.board
---

Accepts columns, tasks, editing state, filters, and event refs.
Use this as the top-level render entry for `kanban.v1` cards.
`;
```

### 4. Per-card docs should also use jsdoc conventions

The current `pkg/jsdoc/model` does not have a dedicated `CardDoc` type. The simplest first design is to treat cards as symbols.

Recommended convention:

```js
__doc__("kanbanSprintBoard", {
  summary: "Sprint planning demo board",
  tags: ["card", "demo", "kanban"],
  concepts: ["sprint-planning"],
});

doc`
---
symbol: kanbanSprintBoard
---

This card demonstrates a typical sprint planning layout with backlog,
ready, doing, and done columns.
`;
```

This avoids changing `pkg/jsdoc/model` in v1.

Later, if we need true card-specific docs fields, we can add a higher-level adapter or model extension.

## Detailed Output Artifacts

I recommend emitting four artifacts.

### 1. Structural manifest

File:

- `generated/vm_card_manifest.json`

Purpose:

- machine-readable source of truth for runtime/debugger/build code

Suggested shape:

```json
{
  "packId": "kanban.v1",
  "cards": [
    {
      "cardId": "kanbanSprintBoard",
      "title": "Sprint Board",
      "sourcePath": "src/domain/cards/sprint-board.vm.js",
      "source": "export const meta = ...",
      "handlers": ["openTaskEditor", "saveTask", "deleteTask"],
      "tags": ["demo", "kanban"]
    }
  ]
}
```

### 2. TypeScript manifest wrapper

File:

- `generated/vm_card_manifest.ts`

Purpose:

- ergonomic frontend import without JSON loader quirks

Suggested shape:

```ts
import manifest from './vm_card_manifest.json';
export const VM_CARD_MANIFEST = manifest;
```

Or generated directly as typed TS if preferred.

### 3. Docs corpus

File:

- `generated/pack_docs.json`

Purpose:

- browsable/exportable docs for pack DSL and card docs

Suggested shape:

```json
{
  "package": { "...": "..." },
  "symbols": [
    { "name": "widgets.kanban.board", "...": "..." },
    { "name": "kanbanSprintBoard", "...": "..." }
  ],
  "examples": []
}
```

### 4. Source index

File:

- `generated/source_index.json`

Purpose:

- lightweight mapping for debuggers and source openers

Suggested shape:

```json
{
  "kanbanSprintBoard": {
    "path": "src/domain/cards/sprint-board.vm.js",
    "packId": "kanban.v1",
    "hash": "..."
  }
}
```

## API References

### `pkg/jsparse`

File:

- `go-go-goja/pkg/jsparse/treesitter.go`

Useful APIs:

- `NewTSParser()`
- `(*TSParser).Parse(source []byte) *TSNode`
- `TSNode.Kind`
- `TSNode.Text`
- `TSNode.Children`
- `(*TSNode).NodeAtPosition(...)`

Limitations to note:

- current wrapper snapshots a CST-like tree, not a rich semantic AST
- there is no domain-specific query API yet
- for metadata extraction we will likely add our own walker over `TSNode`

### `pkg/jsdoc/extract`

File:

- `go-go-goja/pkg/jsdoc/extract/extract.go`

Useful APIs:

- `ParseSource(path string, src []byte) (*model.FileDoc, error)`
- `ParseFSFile(fsys fs.FS, path string) (*model.FileDoc, error)`
- `ParseDir(dir string) ([]*model.FileDoc, error)`

Important behavior:

- recognizes `__package__`, `__doc__`, `__example__`, and `doc\`...\``
- uses tree-sitter parsing already
- attaches prose to symbols/packages using frontmatter keys such as `symbol:` and `package:`

### `pkg/jsdoc/batch`

File:

- `go-go-goja/pkg/jsdoc/batch/batch.go`

Useful APIs:

- `BuildStore(ctx, inputs, opts)`
- `InputFile`
- `BatchOptions{ ContinueOnError, ParsePath }`

This matters because runtime-pack docs are likely multi-file and should be built in batches.

## Design Decisions

### Decision 1: the preprocessor is a Go command in `go-go-os-backend`

Reason:

- the user explicitly wants the parser in Go
- `github.com/go-go-golems/go-go-goja` already contains the key parser/docs packages
- the user explicitly wants us to import that library rather than modify it
- `go-go-os-backend` is the correct project-owned executable home
- it creates one authoritative implementation instead of duplicating logic in Vite plugins and frontend scripts

### Decision 2: split cards into per-card files

Reason:

- exact source preservation is easier
- docs attachment is easier
- structural extraction is simpler and less fragile
- debugger/source tooling benefits immediately

### Decision 3: reuse jsdoc sentinel docs instead of inventing a separate docs DSL

Reason:

- the extraction tooling already exists
- there is already local documentation and tests for it
- it avoids parallel doc formats

### Decision 4: represent card docs as symbols in v1

Reason:

- avoids extending `pkg/jsdoc/model` too early
- keeps the first iteration small
- still supports summary, tags, concepts, examples, and long-form prose

### Decision 5: emit generated artifacts into consumer repos, not only stdout

Reason:

- frontend builds want importable files
- CI wants deterministic outputs to diff and cache
- local debugging wants concrete generated artifacts to inspect

## Detailed Implementation Plan

### Phase 1: standardize authoring layout

Target repos/files:

- `wesen-os/apps/os-launcher/src/domain/pluginBundle.vm.js`
- future pack package directories after APP-16

Work:

- split inline cards into `cards/*.vm.js`
- add one or more `docs/*.js` files with jsdoc sentinel metadata
- keep the existing bundle file as assembly glue only

Review checkpoint:

- no large inline multi-card definitions remain in the first migrated pack

### Phase 2: create Go scanning/model packages

Target:

- `go-go-os-backend/pkg/vmmeta/model`
- `go-go-os-backend/pkg/vmmeta/scan`

Suggested core types:

```go
type CardMeta struct {
    CardID     string
    PackID     string
    Title      string
    Tags       []string
    SourcePath string
    Source     string
    Handlers   []string
}

type PackManifest struct {
    PackID string
    Cards  []CardMeta
}
```

Work:

- walk JS CST from `pkg/jsparse`
- find exported `meta` object
- find default export shape
- collect handler names
- preserve exact source bytes

Review checkpoint:

- the scanner works on one `*.vm.js` file and emits deterministic metadata

### Phase 3: wire jsdoc extraction for pack/card docs

Target:

- `go-go-os-backend/pkg/vmmeta/docs`

Work:

- decide which files to feed into `pkg/jsdoc/batch`
- build a docs corpus from `docs/*.js` and optionally card files
- normalize symbol naming conventions for cards and pack DSL symbols

Pseudocode:

```go
inputs := []batch.InputFile{
    {Path: "src/domain/docs/pack-api.js"},
    {Path: "src/domain/cards/sprint-board.vm.js"},
}

result, err := batch.BuildStore(ctx, inputs, batch.BatchOptions{
    ContinueOnError: false,
    ParsePath: scopedParser,
})
```

Review checkpoint:

- pack docs and one card doc are extractable in one batch run

### Phase 4: generate output files

Target:

- `go-go-os-backend/pkg/vmmeta/generate`
- `go-go-os-backend/cmd/go-go-os-backend`

Work:

- serialize manifest JSON
- serialize docs JSON
- optionally emit TS wrappers
- include deterministic ordering

Important rule:

- generation must be deterministic
- sort by path, then card ID, then symbol/example ID

Review checkpoint:

- running the generator twice on unchanged input produces byte-identical output

### Phase 5: integrate with builds

Target consumers:

- `wesen-os/apps/os-launcher/package.json`
- later dedicated pack packages after APP-16

Suggested scripts:

```json
{
  "scripts": {
    "vmmeta:generate": "go run ../../../../workspace-links/go-go-os-backend/cmd/go-go-os-backend vmmeta generate --config vmmeta.yaml",
    "dev": "npm run vmmeta:generate && vite",
    "build": "npm run vmmeta:generate && vite build"
  }
}
```

Possible config:

```yaml
pack_id: kanban.v1
root: src/domain
cards_glob:
  - cards/*.vm.js
docs_glob:
  - docs/*.js
out_dir: src/generated
```

Review checkpoint:

- a clean build fails if generated outputs are missing or stale

### Phase 6: later debugger/runtime consumption

Target consumers:

- Stacks & Cards debugger
- runtime-pack registry
- future pack registration tooling

Possible uses:

- show exact built-in card source in debugger
- populate pack/card doc viewers
- validate that runtime bundle cards match generated manifests

This phase should remain a follow-up, not part of the first generator delivery.

## Pseudocode

### End-to-end generation

```pseudo
load config
resolve root + globs

cardFiles = find all cards/*.vm.js
docFiles = find all docs/*.js plus optionally card files with docs

manifest = new PackManifest

for file in cardFiles:
  src = read bytes
  tree = jsparse.Parse(src)
  meta = extract card meta from CST
  meta.source = src
  manifest.cards.append(meta)

docStore = jsdoc.batch.BuildStore(docFiles)

write generated/vm_card_manifest.json
write generated/pack_docs.json
write generated/source_index.json
write generated/vm_card_manifest.ts
```

### Card scanner logic

```pseudo
parse file to TSNode root
find exported const meta object
extract:
  cardId
  packId
  title
  tags

find export default object/function
find handlers object keys

return CardMeta{
  source: exact file contents,
  handlers: sorted handler names,
}
```

## Alternatives Considered

### Alternative A: Node/Vite-only preprocessor

Rejected because:

- the user explicitly wants Go
- it duplicates local parser/docs investment already made in `go-go-goja`
- it creates another toolchain to maintain

### Alternative B: use `function.toString()` at runtime

Rejected because:

- source capture becomes lossy and runtime-dependent
- build outputs are not deterministic
- docs extraction becomes impossible or awkward

### Alternative C: scrape `defineCard(...)` calls out of monolithic bundle files forever

Rejected because:

- fragile
- poor authoring ergonomics
- hard to attribute docs to individual cards cleanly

### Alternative D: extend `pkg/jsdoc/model` immediately with first-class `CardDoc`

Deferred because:

- it is not necessary for v1
- cards can be represented as symbols initially
- the simpler design reduces the scope of the first implementation

## Risks And Review Notes

### Risk 1: CST extraction becomes brittle

Tree-sitter gives syntax trees, not semantic JS execution.

Mitigation:

- constrain authoring format
- use explicit `export const meta`
- do not support arbitrarily dynamic metadata expressions in v1

### Risk 2: docs and structure drift apart

If card metadata and docs live in unrelated files with no naming discipline, drift will occur.

Mitigation:

- enforce naming conventions
- optionally validate that `__doc__("cardId")` matches a known `cardId`

### Risk 3: build complexity spreads too early

If too many repos wire the generator at once, failures will be noisy.

Mitigation:

- first implement on one pack
- first integrate into one app or package
- expand only after deterministic output and error reporting are solid

## Rollback Strategy

If the full plan becomes too large, the safe rollback is:

1. keep the Go scanner
2. emit only structural manifest and source index first
3. defer jsdoc extraction integration by one slice

This still delivers major value for debugger/source workflows without blocking on documentation modeling.

## Testing Plan

### Unit tests

- malformed `meta` export
- missing `cardId`
- unsupported dynamic expressions
- handler extraction
- exact source preservation
- deterministic ordering

### Integration tests

- scan a fixture pack directory
- batch extract docs
- generate all outputs
- compare against golden files

### Consumer checks

- frontend imports generated TS manifest successfully
- debugger can open built-in card source later
- docs JSON can be exported or browsed through a thin viewer later

## Open Questions

1. Should v1 emit only JSON plus a thin TS wrapper, or direct TS-only output?
2. Should card docs live in the card files themselves, sidecar docs files, or both?
3. When APP-16 lands, should the generator move with the dedicated pack package or stay app-local?
4. Do we want an explicit `vmmeta.yaml` config file per pack/app, or just CLI flags in v1?

My recommendations:

- emit JSON plus TS wrapper
- allow card docs in the card files or a pack docs file, but prefer one docs file per pack plus optional per-card docs in card files
- use an explicit config file
- keep cards-as-symbols in the docs model for v1

## Intern Checklist

Before coding:

- read `pkg/jsparse/treesitter.go`
- read `pkg/jsdoc/extract/extract.go`
- read `pkg/jsdoc/batch/batch.go`
- read `cmd/goja-jsdoc/doc/01-jsdoc-system.md`
- read `apps/os-launcher/src/domain/pluginBundle.vm.js`

When coding:

- standardize authoring format first
- build the scanner second
- build docs integration third
- wire build scripts last

When reviewing:

- check determinism
- check exact source preservation
- check docs extraction conventions
- check that the system does not rely on executing the VM source to discover metadata

## References

- `../index.md`
- `../tasks.md`
- `../reference/01-implementation-diary.md`

---
Title: Implementation diary
Ticket: APP-18-GO-VM-CARD-AST-PREPROCESSOR
Status: completed
Topics:
    - architecture
    - frontend
    - hypercard
    - wesen-os
    - goja
    - tooling
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../go-go-goja/cmd/goja-jsdoc/doc/01-jsdoc-system.md
      Note: |-
        Reviewed as the existing intern guide for jsdoc extraction behavior and conventions
        Reviewed as the existing intern guide for jsdoc extraction behavior
    - Path: ../../../../../../../go-go-goja/pkg/jsdoc/batch/batch.go
      Note: |-
        Reviewed to confirm the reusable multi-input batch builder and ParsePath override seam
        Reviewed to confirm the multi-input batch builder and ParsePath override seam
    - Path: ../../../../../../../go-go-goja/pkg/jsdoc/extract/extract.go
      Note: |-
        Reviewed to confirm the sentinel extraction rules and reusable ParseSource/ParseFSFile entrypoints
        Reviewed to confirm reusable ParseSource and ParseFSFile entrypoints
    - Path: ../../../../../../../go-go-goja/pkg/jsdoc/model/model.go
      Note: |-
        Reviewed to determine how per-card docs can fit the existing package/symbol/example model
        Reviewed to determine how card docs fit the current package/symbol/example model
    - Path: ../../../../../../../go-go-goja/pkg/jsparse/treesitter.go
      Note: Reviewed to confirm the available tree-sitter wrapper and its current node snapshot API
    - Path: ../../../../../../../wesen-os/apps/os-launcher/package.json
      Note: App build scripts now invoke the backend-owned vmmeta generator
    - Path: ../../../../../../../wesen-os/apps/os-launcher/src/domain/generated/kanbanVmmeta.generated.ts
      Note: Generated TypeScript artifact consumed by os-launcher
    - Path: ../../../../../../../wesen-os/apps/os-launcher/src/domain/pluginBundle.ts
      Note: Runtime bundle now assembled from ordered raw source fragments
    - Path: ../../../../../../../wesen-os/apps/os-launcher/src/domain/pluginBundle.vm.js
      Note: |-
        Reviewed as the current inline VM card authoring seam that motivates per-card source files
        Reviewed as the current inline VM authoring seam that motivates per-card files
    - Path: ../../../../../../../wesen-os/apps/os-launcher/src/domain/vm/cards/kanbanSprintBoard.vm.js
      Note: Representative per-card VM source file with __card__ and __doc__ sentinels
    - Path: ../../../../../../../wesen-os/apps/os-launcher/src/domain/vm/docs/kanban-pack.docs.vm.js
      Note: Pack-level DSL docs source consumed by the jsdoc pipeline
    - Path: ../../../../../../../wesen-os/workspace-links/go-go-os-backend/cmd/go-go-os-backend/main.go
      Note: Reviewed as the backend CLI home where the new preprocessor command should live
    - Path: ../../../../../../../wesen-os/workspace-links/go-go-os-backend/cmd/go-go-os-backend/vmmeta_command.go
      Note: Backend Cobra subcommand that exposes vmmeta generate
    - Path: ../../../../../../../wesen-os/workspace-links/go-go-os-backend/go.mod
      Note: Reviewed as the backend module that should import github.com/go-go-golems/go-go-goja without modifying it
    - Path: ../../../../../../../wesen-os/workspace-links/go-go-os-backend/pkg/vmmeta/vmmeta.go
      Note: Core generator that scans card/docs sources and emits deterministic artifacts
    - Path: ../../../../../../../wesen-os/workspace-links/go-go-os-backend/pkg/vmmeta/vmmeta_test.go
      Note: Backend unit tests covering valid extraction
    - Path: ../../../../../../../wesen-os/workspace-links/go-go-os-frontend/packages/hypercard-runtime/src/runtime-packs/kanbanV1Pack.tsx
      Note: |-
        Reviewed as the first concrete pack whose DSL/API docs should eventually be extracted
        Reviewed as the first pack whose DSL/API docs should later be extracted
ExternalSources: []
Summary: Diary for the APP-18 design work that defines the Go-native VM card preprocessing pipeline with a CLI in go-go-os-backend and parsing/docs libraries imported from github.com/go-go-golems/go-go-goja.
LastUpdated: 2026-03-10T23:25:00-04:00
WhatFor: Record the evidence, reasoning, and concrete API findings behind APP-18 so later implementation can proceed without redoing the local parser/jsdoc audit.
WhenToUse: Use when continuing APP-18, reviewing why the preprocessor should be in Go, or checking the exact local `go-go-goja` APIs the design depends on.
---



# Implementation diary

## Goal

Capture the research and design reasoning for a Go-based VM card AST preprocessor whose CLI lives in `go-go-os-backend` and whose structural/docs extraction libraries are imported from `github.com/go-go-golems/go-go-goja`.

## Step 1: Define The Go-Native VM Metadata Ticket

The user rejected a Node-side preprocessor and explicitly pointed to `go-go-goja` as the correct home for parsing and documentation extraction. That changed the design question from “what sort of build step should we add?” to “which exact local Go packages already solve enough of the parsing problem that the new system can be a thin composition layer instead of a fresh parser implementation?”

The answer, after reading the local repos, was that most of the foundation already exists. `pkg/jsparse` provides a practical tree-sitter wrapper for JavaScript syntax trees, and `pkg/jsdoc` already ports the `jsdocex` extraction model into Go. The design work in this step therefore focused on defining the missing glue: authoring conventions, a scanner/generator command, generated output shapes, and where consumer builds should invoke the command.

### Prompt Context

**User prompt (verbatim):**

> no it should be in go. Let's create a second ticket for that. We are going to use go-go-goja (see ../go-go-goja) which provides a tree-sitter based parser we can use. I also want to use jsdocex fromthere to extract documentation from the DSL/API documentation of packs, as well as the doc for individual cards. Do a detailed implementation research in that new ticket, keeping a detailed diary.
>
> Create a detailed analysis / design / implementation guide that is very detailed for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file
>   references.
>   It should be very clear and detailed. Store in the ticket and the nupload to remarkable.

**Assistant interpretation:** Create a new research/design ticket for a Go-native VM card metadata preprocessor based on local `go-go-goja` parser and jsdoc packages, document it in intern-friendly detail, keep a proper diary, and upload the resulting ticket bundle to reMarkable.

**Inferred user intent:** Make the VM metadata/docs pipeline a first-class Go subsystem with clear architecture and handoff-quality docs, rather than an ad hoc frontend-only script.

**Commit (code):** N/A

### What I did

- Created APP-18 in the `openai-app-server` ticket workspace.
- Audited the relevant local `go-go-goja` APIs:
  - `pkg/jsparse/treesitter.go`
  - `pkg/jsdoc/extract/extract.go`
  - `pkg/jsdoc/batch/batch.go`
  - `pkg/jsdoc/model/model.go`
  - `cmd/goja-jsdoc/doc/01-jsdoc-system.md`
- Audited the current frontend/runtime-pack seams:
  - `apps/os-launcher/src/domain/pluginBundle.vm.js`
  - `apps/os-launcher/src/domain/pluginBundle.authoring.d.ts`
  - `packages/hypercard-runtime/src/runtime-packs/kanbanV1Pack.tsx`
  - `packages/hypercard-runtime/src/runtime-packs/runtimePackRegistry.tsx`
- Wrote the design guide to define:
  - the Go command and package layout
  - the authoring format for card files and docs files
  - the generated output artifacts
  - build invocation points in consumer repos
  - phased implementation slices and review checkpoints

### Why

- The user explicitly wants the parser and build process in Go.
- The local repo already contains most of the hard pieces, so the correct design is composition and standardization, not a greenfield parser.
- This architecture spans `go-go-goja`, `wesen-os`, and runtime-pack code, so a detailed cross-repo ticket is the right place to capture the plan.

### What worked

- The local audit found enough reusable APIs that the proposed implementation does not need speculative external dependencies.
- The existing `pkg/jsdoc` model is flexible enough that per-card docs can likely be represented as symbol docs in v1, which keeps the first implementation smaller.
- The current inline `pluginBundle.vm.js` shape made the need for per-card file splitting obvious and concrete.

### What didn't work

- `docmgr ticket create-ticket` again created the workspace under `2026/03/09` rather than the effective session date of March 10, 2026. This did not block the work, but it is worth noting for later lookup.

### What I learned

- `go-go-goja/pkg/jsparse` is intentionally small. It gives a CST-like snapshot and positions, not a high-level semantic query API. That means APP-18 should plan a narrow authoring format instead of assuming the parser can infer arbitrary dynamic JavaScript constructs.
- `go-go-goja/pkg/jsdoc` is already strong enough to support the documentation half of the problem with very little new parsing logic, especially if pack docs and card docs adopt the current sentinel conventions.
- The main architectural risk is not parsing. It is keeping authoring conventions disciplined enough that the Go scanner remains deterministic and maintainable.

### What was tricky to build

- The tricky part was choosing a docs model for “individual cards” without over-designing a new `CardDoc` subsystem. The current `pkg/jsdoc/model` only knows packages, symbols, and examples, so the design had to decide whether to add a new model immediately or map cards into an existing concept.
- I chose the smaller and more defensible path for v1: represent card docs as symbol docs, using the card ID as the symbol name. That keeps the extraction system close to the current jsdocex behavior and avoids premature model expansion.

### What warrants a second pair of eyes

- The proposed authoring format deserves review, especially the explicit `export const meta` recommendation. It is the key simplification that makes Go-side structural extraction realistic without a much more sophisticated JS semantic analyzer.
- The output artifact set should also be reviewed. There are tradeoffs between emitting only JSON, JSON plus TS wrappers, or direct TS generation.
- The eventual package location of generated files should be reviewed again once APP-16 lands and pack code moves into external packages.

### What should be done in the future

- Implement APP-18 beginning with one pack and one app, not the whole workspace at once.
- Standardize the VM authoring layout before building the scanner, so the parser targets are stable.
- Keep built-in debugger/runtime consumers as a later slice rather than making them part of the first generator milestone.

### Code review instructions

- Start with the APP-18 design doc and verify each major claim against the concrete files in `RelatedFiles`.
- Pay special attention to:
  - the `pkg/jsparse` API limitations
  - the existing `pkg/jsdoc` sentinel contract
  - the proposed mapping of card docs to symbol docs
  - the recommendation to split `pluginBundle.vm.js` into per-card files

### Technical details

- Ticket creation commands:

```bash
docmgr ticket create-ticket \
  --ticket APP-18-GO-VM-CARD-AST-PREPROCESSOR \
  --title "Design Go-based VM card AST preprocessing and jsdoc extraction for runtime packs" \
  --topics architecture,frontend,hypercard,wesen-os,goja,tooling

docmgr doc add \
  --ticket APP-18-GO-VM-CARD-AST-PREPROCESSOR \
  --doc-type design-doc \
  --title "Intern guide to a Go VM card AST preprocessor and jsdoc extraction pipeline for runtime packs"

docmgr doc add \
  --ticket APP-18-GO-VM-CARD-AST-PREPROCESSOR \
  --doc-type reference \
  --title "Implementation diary"
```

- Key local APIs reviewed:

```go
// pkg/jsparse
func NewTSParser() (*TSParser, error)
func (tp *TSParser) Parse(source []byte) *TSNode

// pkg/jsdoc/extract
func ParseSource(path string, src []byte) (*model.FileDoc, error)
func ParseFSFile(fsys fs.FS, path string) (*model.FileDoc, error)

// pkg/jsdoc/batch
func BuildStore(ctx context.Context, inputs []InputFile, opts BatchOptions) (*BatchResult, error)
```

## Step 2: Move CLI Ownership To go-go-os-backend

After the initial ticket draft, the user tightened the ownership boundary. The parser and jsdoc libraries should still come from `github.com/go-go-golems/go-go-goja`, but the executable itself should live in `go-go-os-backend` and be invoked from `go-go-os-frontend`. That is a better boundary anyway, because it keeps the project-owned CLI close to the backend repo that already owns operational tooling while leaving `go-go-goja` as an imported dependency rather than something we patch locally.

This refinement changed several important parts of the design: the command/package layout, the related files, the implementation slices, and the consumer invocation examples. The core parsing approach did not change. The ownership and integration path did.

### Prompt Context

**User prompt (verbatim):**

> The CLI tool should be inside go-go-os-backend and you can then use it from go-go-os-frontend. Updat ethe document. We import github.com/go-go-golems/go-go-goja but don't modify it

**Assistant interpretation:** Update APP-18 so the CLI belongs to `go-go-os-backend`, while `go-go-goja` remains a library dependency that we import rather than modify.

**Inferred user intent:** Keep the executable in the project-owned backend repo, preserve a clean dependency boundary to `go-go-goja`, and avoid turning the library repo into an application-specific tooling home.

**Commit (code):** N/A

### What I did

- Audited the current `go-go-os-backend` executable home:
  - `cmd/go-go-os-backend/main.go`
  - `go.mod`
- Updated APP-18 index, task list, changelog, design guide, and diary to reflect:
  - CLI home in `go-go-os-backend`
  - parser/docs imports from `github.com/go-go-golems/go-go-goja`
  - frontend invocation from `go-go-os-frontend`
- Added backend files to the ticket’s related files so the later implementation starts from the correct repo boundary.

### Why

- The user explicitly requested this boundary.
- It is also the cleaner architecture. `go-go-goja` should stay a reusable library/tooling dependency, not become an application-specific command host.

### What worked

- `go-go-os-backend` is currently sparse, so the ticket can define the new CLI ownership cleanly without working around existing command complexity.
- The rest of the design remains valid. Only the ownership and invocation seams changed.

### What didn't work

- N/A

### What I learned

- The original “put the command in `go-go-goja`” recommendation was pragmatic but not the best ownership boundary for this workspace. The user’s correction improves the separation of concerns.

### What was tricky to build

- The tricky part was making sure the docs reflected the new ownership boundary everywhere, not just in one section. The command location affects:
  - package layout
  - example scripts
  - future implementation slices
  - related files
  - the rationale for why `go-go-goja` is a library dependency rather than the command home

### What warrants a second pair of eyes

- The eventual command wiring in `go-go-os-backend/cmd/go-go-os-backend/main.go` deserves review once implementation starts, because that file is currently a stub and the team may want a broader CLI structure at the same time.

### What should be done in the future

- Implement the CLI in `go-go-os-backend`.
- Keep all parsing/docs logic on the library side through imports from `github.com/go-go-golems/go-go-goja`.
- Avoid local changes to the `go-go-goja` repo as part of APP-18 implementation.

### Code review instructions

- Re-read APP-18 design sections:
  - Executive Summary
  - Recommended command ownership
  - Detailed implementation phases
- Confirm the docs now clearly state:
  - CLI in `go-go-os-backend`
  - imports from `github.com/go-go-golems/go-go-goja`
  - frontend invocation from `go-go-os-frontend`

### Technical details

- Backend executable home reviewed:

```text
wesen-os/workspace-links/go-go-os-backend/cmd/go-go-os-backend/main.go
wesen-os/workspace-links/go-go-os-backend/go.mod
```

## Step 3: Validate And Publish The Corrected APP-18 Bundle

After the backend-ownership correction, the remaining work was publication hygiene rather than more research. The important thing here was to make sure the ticket state matched reality before uploading anything: the task list still showed two unchecked setup items that had in fact been completed already, and the final reMarkable bundle needed to reflect the corrected architecture, not the earlier draft.

I treated this as a final documentation pass. I checked the completed setup items, reran `docmgr doctor` to confirm the ticket was clean, uploaded the five-document APP-18 bundle to reMarkable, and verified the remote directory. That closes the research side of the ticket cleanly and leaves a reviewable handoff artifact for later implementation.

### Prompt Context

**User prompt (verbatim):** (see Step 2)

**Assistant interpretation:** Finalize the corrected APP-18 ticket around the backend-owned CLI and publish the resulting bundle.

**Inferred user intent:** Make the ticket authoritative and portable so the team can implement the Go preprocessor later without ambiguity about ownership or tool boundaries.

**Commit (code):** N/A

### What I did

- Checked off the already-completed setup tasks in `tasks.md`:
  - related files attached
  - `docmgr doctor` run
- Reran:

```bash
docmgr doctor --ticket APP-18-GO-VM-CARD-AST-PREPROCESSOR --stale-after 30
```

- Verified reMarkable tooling/account:

```bash
remarquee status
remarquee cloud account --non-interactive
```

- Dry-ran and uploaded the final bundle:

```bash
remarquee upload bundle \
  "/home/manuel/workspaces/2026-03-02/os-openai-app-server/openai-app-server/ttmp/2026/03/09/APP-18-GO-VM-CARD-AST-PREPROCESSOR--design-go-based-vm-card-ast-preprocessing-and-jsdoc-extraction-for-runtime-packs/index.md" \
  "/home/manuel/workspaces/2026-03-02/os-openai-app-server/openai-app-server/ttmp/2026/03/09/APP-18-GO-VM-CARD-AST-PREPROCESSOR--design-go-based-vm-card-ast-preprocessing-and-jsdoc-extraction-for-runtime-packs/design-doc/01-intern-guide-to-a-go-vm-card-ast-preprocessor-and-jsdoc-extraction-pipeline-for-runtime-packs.md" \
  "/home/manuel/workspaces/2026-03-02/os-openai-app-server/openai-app-server/ttmp/2026/03/09/APP-18-GO-VM-CARD-AST-PREPROCESSOR--design-go-based-vm-card-ast-preprocessing-and-jsdoc-extraction-for-runtime-packs/tasks.md" \
  "/home/manuel/workspaces/2026-03-02/os-openai-app-server/openai-app-server/ttmp/2026/03/09/APP-18-GO-VM-CARD-AST-PREPROCESSOR--design-go-based-vm-card-ast-preprocessing-and-jsdoc-extraction-for-runtime-packs/changelog.md" \
  "/home/manuel/workspaces/2026-03-02/os-openai-app-server/openai-app-server/ttmp/2026/03/09/APP-18-GO-VM-CARD-AST-PREPROCESSOR--design-go-based-vm-card-ast-preprocessing-and-jsdoc-extraction-for-runtime-packs/reference/01-implementation-diary.md" \
  --name "APP-18 Go VM Card AST Preprocessor" \
  --remote-dir "/ai/2026/03/10/APP-18-GO-VM-CARD-AST-PREPROCESSOR" \
  --toc-depth 2

remarquee cloud ls /ai/2026/03/10/APP-18-GO-VM-CARD-AST-PREPROCESSOR --long --non-interactive
```

### Why

- The corrected backend-owned CLI boundary needed to be the published version, not just the local draft.
- The task list should not drift from the actual ticket state, especially on a handoff/research ticket where later readers will use the checklist as a status indicator.

### What worked

- `docmgr doctor` passed cleanly after the correction.
- The reMarkable upload succeeded and the remote directory verified correctly.
- The bundle includes the full implementation guide, tasks, changelog, and diary, so the ticket is portable as a single artifact.

### What didn't work

- N/A

### What I learned

- The smallest documentation cleanup steps matter before publication. Leaving already-completed setup items unchecked would have made the ticket look less complete than it actually is.

### What was tricky to build

- The only subtle part was making sure the publication step reflected the corrected architecture everywhere. Once the backend-ownership change landed in the design doc, the remaining risk was administrative inconsistency rather than technical inconsistency.

### What warrants a second pair of eyes

- The uploaded bundle itself is fine, but once implementation starts, the first reviewer should confirm that `go-go-os-backend` is still the right command home and that no team-level CLI conventions have changed by then.

### What should be done in the future

- Use APP-18 as the research/design baseline when creating the actual implementation ticket.
- Keep `go-go-goja` usage import-only unless there is a separately justified upstream change.

### Code review instructions

- Review `tasks.md` and `changelog.md` for publication/bookkeeping status.
- Review the APP-18 design doc for the final backend-owned CLI architecture.
- Verify the reMarkable remote directory if needed:

```bash
remarquee cloud ls /ai/2026/03/10/APP-18-GO-VM-CARD-AST-PREPROCESSOR --long --non-interactive
```

### Technical details

- Verified remote location:

```text
/ai/2026/03/10/APP-18-GO-VM-CARD-AST-PREPROCESSOR
```

## Step 4: Expand The Ticket Tasks And Split `os-launcher` Into Real VM Source Files

The user moved APP-18 from research into implementation. That required two immediate changes before writing the backend generator itself: the task list needed to become a concrete engineering checklist rather than a research backlog, and `os-launcher` needed an actual preprocessable source layout so the generator would not have to scrape one giant monolithic bundle file.

I handled those together. The task list was expanded into explicit implementation phases, then the `os-launcher` Kanban cards were split out into dedicated VM source files with runtime-safe no-op sentinels for `__card__`, `__doc__`, `__package__`, and `doc`. The runtime bundle is now assembled from ordered raw source fragments in `pluginBundle.ts`, which means the same authored files can feed both QuickJS runtime execution and the Go metadata pipeline.

### Prompt Context

**User prompt (verbatim):**

> alright, create detailed tasks in the ticket. then work on them one by one, checking thing soff as you go, committing at appropriate intervals and keeping a diary

**Assistant interpretation:** Turn APP-18 into an implementation ticket, expand the tasks into detailed checkpoints, implement the slices incrementally, commit them in focused groups, and record the whole execution path in the diary.

**Inferred user intent:** Move from architecture discussion to working code while keeping the ticket useful as an execution log and future onboarding artifact.

**Commit (code):** `5bb60d8` — `Refactor os-launcher kanban vm sources for vmmeta`

### What I did

- Expanded `tasks.md` from high-level slices into concrete implementation phases.
- Replaced the single `apps/os-launcher/src/domain/pluginBundle.vm.js` source with a fragment-based authored layout:
  - `apps/os-launcher/src/domain/vm/00-runtimePrelude.vm.js`
  - `apps/os-launcher/src/domain/vm/10-home.vm.js`
  - `apps/os-launcher/src/domain/vm/docs/kanban-pack.docs.vm.js`
  - `apps/os-launcher/src/domain/vm/cards/kanbanSprintBoard.vm.js`
  - `apps/os-launcher/src/domain/vm/cards/kanbanBugTriage.vm.js`
  - `apps/os-launcher/src/domain/vm/cards/kanbanPersonalPlanner.vm.js`
- Added no-op runtime-safe sentinel declarations in the prelude so docs metadata can coexist with QuickJS execution.
- Updated `pluginBundle.ts` to concatenate the authored fragments into the runtime bundle string.
- Added `vmmeta.ts` plus generated artifact consumption in `stack.ts`.

### Why

- The Go generator needed deterministic inputs. A monolithic `pluginBundle.vm.js` was the wrong shape.
- Keeping runtime execution on the same authored source files avoids a parallel metadata-only source tree and reduces drift.

### What worked

- The fragment-based bundle assembly keeps the runtime contract intact while making the Kanban cards individually inspectable and preprocessable.
- The `__card__` sentinel format ended up being a clean boundary between “authored metadata” and “extracted metadata”.

### What didn't work

- `npm test -- --run src/domain/pluginBundle.test.ts` did not provide a clean validation signal because it is currently blocked by an existing `hypercard-runtime` JS/TS import gap:

```text
Error: Cannot find module '/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-os-frontend/packages/hypercard-runtime/src/app/createAppStore'
```

- `npm run typecheck` in `apps/os-launcher` is still blocked by unrelated `rich-widgets` type errors from linked workspace sources.

### What I learned

- The cleanest implementation path was not a parallel “metadata source tree”. It was making the authored VM files themselves the common source for runtime execution and preprocessing.

### What was tricky to build

- Tagged-template docs required a stricter authoring discipline than normal JS prose. Inline backticks inside a `doc` template caused tree-sitter noise and incomplete extraction, so the prose needed to avoid nested backticks entirely.

### What warrants a second pair of eyes

- The raw fragment assembly order in `pluginBundle.ts` is now semantically important. Reviewers should confirm that prelude/docs/cards ordering stays stable if more packs are added later.

### What should be done in the future

- Generalize the fragment-based source layout once a second runtime pack is implemented.
- Decide whether the final system should also generate the runtime bundle string instead of only metadata/docs artifacts.

### Code review instructions

- Start with:
  - `apps/os-launcher/src/domain/pluginBundle.ts`
  - `apps/os-launcher/src/domain/vm/00-runtimePrelude.vm.js`
  - `apps/os-launcher/src/domain/vm/cards/*.vm.js`
  - `apps/os-launcher/src/domain/vm/docs/kanban-pack.docs.vm.js`
- Confirm the runtime bundle still includes:
  - one stack definition
  - three `kanban.v1` cards
  - runtime-safe sentinel no-ops

### Technical details

- New constrained metadata sentinel:

```js
__card__({
  id: 'kanbanSprintBoard',
  packId: 'kanban.v1',
  title: 'Sprint Board',
  icon: '🏁',
});
```

## Step 5: Implement The Backend `vmmeta` Generator And Wire It Into `os-launcher`

With the source layout in place, I built the backend-owned generator in `go-go-os-backend` and then used it from `apps/os-launcher` to emit real artifacts. The command is intentionally narrow in v1: it scans one cards directory and one docs directory, extracts card metadata and handler names with `pkg/jsparse`, extracts pack/card docs with `pkg/jsdoc`, and writes both JSON and TypeScript outputs.

The generator also surfaced two important authoring constraints that are now explicit in the ticket. First, the metadata object converter had to normalize trailing commas to deal with normal JS object literals. Second, doc templates must avoid inline backticks inside the tagged template body, because those destabilize the parser and the doc extractor. I normalized the Kanban prose accordingly and reran the generator against the live `os-launcher` sources.

### Prompt Context

**User prompt (verbatim):** (see Step 4)

**Assistant interpretation:** Continue the APP-18 implementation slices until the backend command works and the frontend can consume its outputs.

**Inferred user intent:** Prove the design with a working vertical slice, not just scaffolding.

**Commit (code):** `0772da6` — `Add vmmeta generator command`

### What I did

- Added `github.com/go-go-golems/go-go-goja` and `github.com/spf13/cobra` to `go-go-os-backend`.
- Added a Cobra-based `vmmeta generate` command in:
  - `cmd/go-go-os-backend/main.go`
  - `cmd/go-go-os-backend/vmmeta_command.go`
- Implemented the generator in:
  - `pkg/vmmeta/vmmeta.go`
  - `pkg/vmmeta/vmmeta_test.go`
- Reused:
  - `pkg/jsparse` for tree-sitter-backed scanning
  - `pkg/jsdoc/batch` and `pkg/jsdoc/model` for docs extraction and aggregation
- Generated:
  - `apps/os-launcher/src/domain/generated/kanban.vmmeta.json`
  - `apps/os-launcher/src/domain/generated/kanbanVmmeta.generated.ts`
- Wired the app package scripts so `os-launcher` can regenerate metadata before `dev`, `build`, `typecheck`, and `test`.

### Why

- The ticket required the CLI to live in `go-go-os-backend`, not in `go-go-goja`, and the easiest way to validate that decision was a real end-to-end slice against `os-launcher`.

### What worked

- `go test ./pkg/vmmeta ./cmd/go-go-os-backend` passed.
- `npm run vmmeta:generate` in `apps/os-launcher` passed.
- The generated artifacts include exact card source, handler names, pack docs, and card docs.

### What didn't work

- Full `go test ./...` in `go-go-os-backend` still hits a pre-existing unrelated failure in `pkg/backendhost`:

```text
TestMountNamespacedRoutes_PrefixRootRedirectStaysNamespaced
expected: 301
actual  : 307
```

- Full frontend validation is still blocked by the existing workspace issues noted in Step 4.

### What I learned

- `pkg/jsparse` is sufficient for this problem once the authoring format is constrained and the extractor works from source slices instead of trying to infer arbitrary semantics.
- `pkg/jsdoc` is already good enough to carry both pack docs and per-card docs without any upstream model changes.

### What was tricky to build

- The first attempt treated `root.HasError()` from `pkg/jsparse` as a fatal file-level error. That was too coarse for the real authored sources. The practical rule turned out to be: fail on missing `__card__`, missing `defineCard`, or undecodable metadata, not on every generic error node in the full CST.
- Trailing commas in normal JS object literals also needed a normalization pass before unmarshalling into Go structs.

### What warrants a second pair of eyes

- Review the decision to tolerate generic CST error nodes while still enforcing the required sentinels and object decode rules. It is pragmatic, but it is a deliberate boundary.

### What should be done in the future

- If the `jsparse` wrapper grows richer source-node helpers later, tighten the current extraction code and revisit the relaxed `HasError()` handling.
- Consider splitting generated “runtime metadata” and “docs corpus” outputs if bundle size becomes a concern for frontend imports.

### Code review instructions

- Start with:
  - `workspace-links/go-go-os-backend/pkg/vmmeta/vmmeta.go`
  - `workspace-links/go-go-os-backend/pkg/vmmeta/vmmeta_test.go`
  - `workspace-links/go-go-os-backend/cmd/go-go-os-backend/vmmeta_command.go`
- Then review:
  - `apps/os-launcher/src/domain/generated/kanban.vmmeta.json`
  - `apps/os-launcher/src/domain/generated/kanbanVmmeta.generated.ts`
  - `apps/os-launcher/package.json`

### Technical details

- Successful command:

```bash
go run ../../workspace-links/go-go-os-backend/cmd/go-go-os-backend vmmeta generate \
  --pack-id kanban.v1 \
  --cards-dir src/domain/vm/cards \
  --docs-dir src/domain/vm/docs \
  --output-json src/domain/generated/kanban.vmmeta.json \
  --output-ts src/domain/generated/kanbanVmmeta.generated.ts
```

## Step 6: Close APP-18 And Defer Debugger Source Display

At this point APP-18 had reached the intended boundary: generated source and generated docs exist, the backend command works, and `os-launcher` consumes the generated metadata. The next thing the user wants is not more preprocessing work inside this ticket. It is moving `Stacks & Cards` into `hypercard-runtime` and then showing the generated source there.

That means APP-18 should close here instead of absorbing debugger work. I updated the ticket to make that handoff explicit: this ticket owns generation, while `APP-17-HYPERCARD-RUNTIME-DEBUG-BOOTSTRAP` will own the debugger relocation, and only after that should we wire the generated built-in card source/docs into the visible runtime debugger.

### Prompt Context

**User prompt (verbatim):**

> ok, let's move on to moving Staack & Cards to hypercard-runtime. You already made a ticket. Close this one. 
>
> Once we have moved Stack & Cards around, we'll add showing the source

**Assistant interpretation:** Close APP-18 now that the generation slice is complete, and treat visible source display as a later follow-up after APP-17 moves the debugger.

**Inferred user intent:** Keep ticket scope disciplined and avoid mixing the generator work with the runtime-debug relocation work.

**Commit (code):** N/A

### What I did

- Marked APP-18 as completed in the ticket frontmatter.
- Updated the index, tasks, changelog, and diary to say:
  - APP-18 stops at generated source/docs artifacts
  - `Stacks & Cards` relocation is handled by `APP-17-HYPERCARD-RUNTIME-DEBUG-BOOTSTRAP`
  - generated source display inside the debugger is deferred until after APP-17
- Added a `completed` status vocabulary entry so the ticket can be closed cleanly in docmgr metadata.

### Why

- The preprocessing slice is done enough to hand off.
- The next work item has a different architectural center of gravity: runtime-debug ownership, not metadata extraction.

### What worked

- The ticket boundary is now explicit and reviewable.
- Future source-display work has a named dependency instead of becoming an implicit loose end.

### What didn't work

- N/A

### What I learned

- This split is the right place to stop. The generator is useful on its own, and trying to continue directly into debugger relocation would make the review story much worse.

### What was tricky to build

- The only subtle part was making the closure explicit in both prose and task state so later readers do not mistake the deferred debugger work for incomplete generator work.

### What warrants a second pair of eyes

- Reviewers should confirm that `APP-17` is indeed the right dependency for visible source display and that no other runtime-debug ticket superseded it.

### What should be done in the future

- Execute `APP-17-HYPERCARD-RUNTIME-DEBUG-BOOTSTRAP`.
- After APP-17 lands, add built-in VM source/docs display in `Stacks & Cards` using the generated artifacts from APP-18.

### Code review instructions

- Read APP-18 `index.md` and `tasks.md` to confirm the closure boundary.
- Then continue in:
  - `APP-17-HYPERCARD-RUNTIME-DEBUG-BOOTSTRAP`

### Technical details

- Deferred follow-up target:

```text
APP-17-HYPERCARD-RUNTIME-DEBUG-BOOTSTRAP
```

## Related

- `../index.md`
- `../tasks.md`
- `../changelog.md`
- `../design-doc/01-intern-guide-to-a-go-vm-card-ast-preprocessor-and-jsdoc-extraction-pipeline-for-runtime-packs.md`

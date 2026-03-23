# Changelog

## 2026-03-09

- Initial workspace created
- Added the APP-18 design scope, implementation guide, task plan, and diary for a Go-native VM card preprocessing pipeline based on `go-go-goja/pkg/jsparse` and `pkg/jsdoc`.
- Refined APP-18 so the CLI/tooling home is `go-go-os-backend`, while `github.com/go-go-golems/go-go-goja` remains an imported library dependency rather than a repo we modify.

## 2026-03-10

- Revalidated APP-18 with `docmgr doctor`, checked off the ticket setup tasks that were already completed, and uploaded the corrected bundle to reMarkable at `/ai/2026/03/10/APP-18-GO-VM-CARD-AST-PREPROCESSOR`.
- Implemented the first APP-18 code slice in `go-go-os-backend` as commit `0772da6` (`Add vmmeta generator command`): added a Cobra-based `vmmeta generate` command, deterministic JSON/TS artifact generation, jsparse-based card metadata extraction, jsdoc-based docs extraction, and backend unit tests.
- Implemented the `os-launcher` source and wiring slice in `wesen-os` as commit `5bb60d8` (`Refactor os-launcher kanban vm sources for vmmeta`): split the Kanban demo cards into raw source files with `__card__` / `__doc__` sentinels, assembled the runtime bundle from source fragments, checked in generated artifacts, and wired `os-launcher` scripts plus runtime metadata helpers.
- Validation for the completed slices:
  - `go test ./pkg/vmmeta ./cmd/go-go-os-backend` passed
  - `npm run vmmeta:generate` in `apps/os-launcher` passed
  - `npm test -- --run src/domain/pluginBundle.test.ts` remains blocked by an existing `hypercard-runtime` JS/TS import gap
  - `npm run typecheck` in `apps/os-launcher` remains blocked by pre-existing `rich-widgets` type errors
- Closed APP-18 as the generation slice. Remaining debugger/UI consumption work is deferred to `APP-17-HYPERCARD-RUNTIME-DEBUG-BOOTSTRAP`, and source display inside `Stacks & Cards` should be added only after that ticket lands.

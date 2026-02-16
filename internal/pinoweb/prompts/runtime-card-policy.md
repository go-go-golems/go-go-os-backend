When the user's request calls for a visual card (a drilldown view, a report,
a detail panel, an interactive form, etc.), emit exactly one
<hypercard:card:v2> block containing a YAML payload.

╔══════════════════════════════════════════════════════════════════════╗
║                        PAYLOAD SHAPE                               ║
╚══════════════════════════════════════════════════════════════════════╝

<hypercard:card:v2>
```yaml
name: Short Display Name          # shown in timeline while code streams
title: Longer Window Title         # used as the opened window title
artifact:
  id: kebab-case-artifact-id       # unique, stable across regenerations
  data: {}                         # optional context data for the card
card:
  id: lowerCamelCardId             # JS identifier, unique per card
  code: |-
    ({ ui }) => ({
      render({ cardState, sessionState, globalState }) {
        return ui.panel([
          ui.text("Hello from a runtime card")
        ]);
      }
    })
```
</hypercard:card:v2>

Field rules:
  • name        — 1-5 words. Shown in the sidebar/timeline immediately.
  • title       — Sentence-length. Used as the window title bar.
  • artifact.id — kebab-case, globally unique. Same artifact re-uses the id.
  • card.id     — lowerCamelCase. Must be a valid JS identifier.
  • card.code   — A JS expression (see DSL Reference below).

╔══════════════════════════════════════════════════════════════════════╗
║                     card.code DSL REFERENCE                        ║
╚══════════════════════════════════════════════════════════════════════╝

card.code must be a single JS expression that returns a card definition
object. Two forms are accepted:

  FACTORY FORM (recommended — gives you the ui helper):

    ({ ui }) => ({
      render({ cardState, sessionState, globalState }) {
        return ui.panel([ ... ]);
      },
      handlers: { ... }
    })

  OBJECT FORM (raw UI nodes, no ui helper):

    ({
      render({ cardState, sessionState, globalState }) {
        return { kind: "panel", children: [ { kind: "text", text: "hi" } ] };
      }
    })

The card definition object MUST have:
  • render(ctx) — returns a UI node tree.

The card definition object MAY have:
  • handlers    — an object mapping handler names to functions.

────────────────────────────────────────────────────────────────────

UI HELPER API  (available as ui.* in the factory form)

  ui.text(content)
      Returns { kind: "text", text: String(content) }
      Use for headings, labels, paragraphs.

  ui.badge(text)
      Returns { kind: "badge", text: String(text) }
      Use for status indicators, counts, tags.

  ui.button(label, props?)
      Returns { kind: "button", props: { label, ...props } }
      props.onClick = { handler: "handlerName", args: { ... } }
      to wire a handler.

  ui.input(value, props?)
      Returns { kind: "input", props: { value, ...props } }
      props.onChange = { handler: "handlerName" }
      props.placeholder = "hint text"

  ui.table(rows, props?)
      Returns { kind: "table", props: { rows, headers: props.headers } }
      rows is an array of arrays:  [["A", "B"], ["C", "D"]]
      props.headers is a string array: ["Col1", "Col2"]

  ui.row(children)
      Horizontal layout container.

  ui.column(children)
      Vertical layout container.

  ui.panel(children)
      Top-level card body container. Use as the root return value.

────────────────────────────────────────────────────────────────────

RENDER CONTEXT

  render({ cardState, sessionState, globalState })

  • cardState      — per-card key/value store (initially {}).
  • sessionState   — per-session key/value store (shared across cards).
  • globalState    — read-only snapshot of the full app state:
      globalState.domains.inventory.items  — array of inventory Item objects
          Each item: { sku, name, qty, price, cost, category, tags }
      globalState.domains.sales.log        — array of SaleEntry objects
          Each entry: { id, sku, qty, total, date }
      globalState.nav.cardId               — currently displayed card ID
      globalState.nav.param                — navigation parameter

────────────────────────────────────────────────────────────────────

HANDLER CONTEXT

  handlerName(ctx, args)

  ctx provides:
    ctx.cardState, ctx.sessionState, ctx.globalState  (read-only)

    ctx.dispatchCardAction(actionType, payload)
        actionType: "patch" | "set" | "reset"
        Example: ctx.dispatchCardAction("patch", { filter: "shoes" })

    ctx.dispatchSessionAction(actionType, payload)
        Same action types as card actions, but session-scoped.

    ctx.dispatchDomainAction(domain, actionType, payload)
        Dispatches to a domain reducer.
        Example: ctx.dispatchDomainAction("inventory", "updateQty",
                   { sku: "A-1002", qty: 10 })

    ctx.dispatchSystemCommand(command, payload)
        "nav.go"       — { cardId: "someCard", param: "optional" }
        "nav.back"     — no payload
        "notify"       — { message: "Done!" }
        "window.close" — no payload

────────────────────────────────────────────────────────────────────

WIRING BUTTONS TO HANDLERS

  ui.button("Click me", {
    onClick: { handler: "myHandler", args: { foo: 42 } }
  })

  Then define:
    handlers: {
      myHandler(ctx, args) {
        // args.foo === 42
        ctx.dispatchCardAction("patch", { clicked: true });
      }
    }

╔══════════════════════════════════════════════════════════════════════╗
║                          EXAMPLES                                  ║
╚══════════════════════════════════════════════════════════════════════╝

EXAMPLE 1 — Simple read-only table card

<hypercard:card:v2>
```yaml
name: Low Stock Items
title: Items Below Reorder Threshold
artifact:
  id: low-stock-drilldown
  data:
    threshold: 5
card:
  id: lowStockDrilldown
  code: |-
    ({ ui }) => ({
      render({ globalState }) {
        const threshold = 5;
        const items = (globalState?.domains?.inventory?.items ?? [])
          .filter(item => Number(item?.qty ?? 0) <= threshold);
        return ui.panel([
          ui.text("Low Stock Items (qty ≤ " + threshold + ")"),
          items.length === 0
            ? ui.text("No items below threshold.")
            : ui.table(
                items.map(item => [
                  String(item?.sku ?? ""),
                  String(item?.name ?? ""),
                  String(item?.qty ?? 0),
                  "$" + Number(item?.price ?? 0).toFixed(2)
                ]),
                { headers: ["SKU", "Name", "Qty", "Price"] }
              ),
          ui.button("Close", { onClick: { handler: "close" } })
        ]);
      },
      handlers: {
        close(ctx) {
          ctx.dispatchSystemCommand("window.close");
        }
      }
    })
```
</hypercard:card:v2>


EXAMPLE 2 — Interactive filter card with state

<hypercard:card:v2>
```yaml
name: Category Browser
title: Browse Inventory by Category
artifact:
  id: category-browser
  data: {}
card:
  id: categoryBrowser
  code: |-
    ({ ui }) => ({
      render({ cardState, globalState }) {
        const filter = String(cardState?.category ?? "");
        const allItems = globalState?.domains?.inventory?.items ?? [];
        const categories = [...new Set(allItems.map(i => String(i?.category ?? "Other")))];
        const filtered = filter
          ? allItems.filter(i => String(i?.category ?? "") === filter)
          : allItems;
        return ui.panel([
          ui.text("Inventory by Category"),
          ui.row(
            categories.map(cat =>
              ui.button(cat, {
                onClick: { handler: "setCategory", args: { category: cat } }
              })
            )
          ),
          filter ? ui.badge("Showing: " + filter) : ui.badge("All items"),
          ui.table(
            filtered.map(item => [
              String(item?.sku ?? ""),
              String(item?.name ?? ""),
              String(item?.category ?? ""),
              String(item?.qty ?? 0)
            ]),
            { headers: ["SKU", "Name", "Category", "Qty"] }
          ),
          filter
            ? ui.button("Show All", { onClick: { handler: "clearFilter" } })
            : null
        ].filter(Boolean));
      },
      handlers: {
        setCategory(ctx, args) {
          ctx.dispatchCardAction("patch", { category: args?.category ?? "" });
        },
        clearFilter(ctx) {
          ctx.dispatchCardAction("patch", { category: "" });
        }
      }
    })
```
</hypercard:card:v2>


EXAMPLE 3 — Summary stats card

<hypercard:card:v2>
```yaml
name: Inventory Summary
title: Current Inventory Summary Report
artifact:
  id: inventory-summary-report
  data: {}
card:
  id: inventorySummary
  code: |-
    ({ ui }) => ({
      render({ globalState }) {
        const items = globalState?.domains?.inventory?.items ?? [];
        const sales = globalState?.domains?.sales?.log ?? [];
        const totalUnits = items.reduce((sum, i) => sum + Number(i?.qty ?? 0), 0);
        const totalValue = items.reduce(
          (sum, i) => sum + Number(i?.qty ?? 0) * Number(i?.price ?? 0), 0
        );
        const outOfStock = items.filter(i => Number(i?.qty ?? 0) === 0).length;
        const salesTotal = sales.reduce((sum, s) => sum + Number(s?.total ?? 0), 0);
        return ui.panel([
          ui.text("Inventory Summary"),
          ui.table(
            [
              ["Total SKUs", String(items.length)],
              ["Total Units", String(totalUnits)],
              ["Retail Value", "$" + totalValue.toFixed(2)],
              ["Out of Stock", String(outOfStock)],
              ["Recent Sales Total", "$" + salesTotal.toFixed(2)]
            ],
            { headers: ["Metric", "Value"] }
          ),
          ui.button("Close", { onClick: { handler: "close" } })
        ]);
      },
      handlers: {
        close(ctx) {
          ctx.dispatchSystemCommand("window.close");
        }
      }
    })
```
</hypercard:card:v2>

╔══════════════════════════════════════════════════════════════════════╗
║                       SAFETY RULES                                 ║
╚══════════════════════════════════════════════════════════════════════╝

card.code MUST:
  ✓ Be a single JS expression (no statements, no semicolons at top level).
  ✓ Return an object with a render() function.
  ✓ Use only the ui.* helpers or raw { kind: ... } node literals.
  ✓ Be deterministic and bounded (no infinite loops, no recursion).

card.code MUST NOT:
  ✗ Use import, export, or require.
  ✗ Use eval, Function, or new Function.
  ✗ Access window, document, fetch, XMLHttpRequest, or any browser API.
  ✗ Use setTimeout, setInterval, or Promise.
  ✗ Mutate globalState (read-only). Use dispatch* to request changes.
  ✗ Use arrow functions with implicit object returns at the top level
    without wrapping in parentheses.

╔══════════════════════════════════════════════════════════════════════╗
║                     GENERAL RULES                                  ║
╚══════════════════════════════════════════════════════════════════════╝

1. First output a short plain-language summary sentence BEFORE the tag.
2. Do not emit a <hypercard:card:v2> block if you cannot produce valid YAML.
3. Always provide non-empty name, title, artifact.id, card.id, card.code.
4. card.code should handle missing/undefined data gracefully with defaults.
5. Use only YAML inside the tag (no JSON, no raw JS outside card.code).
6. The card.code |- YAML block preserves newlines. Indent code by 4 spaces.

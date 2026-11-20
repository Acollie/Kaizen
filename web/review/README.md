# Kaizen review UI

The browser UI for `kaizen review <PR>`: the PR's diff, Kaizen's risk analysis, a call
flow diagram and the AI review streaming in live. Vite + React 18 + TypeScript (strict)
+ zustand.

It is built into `pkg/reviewui/static/` and embedded in the Go binary with
`//go:embed static`, so a `kaizen` binary needs no Node at runtime. The build output is
**committed**: run `make ui` after changing anything here and commit
`pkg/reviewui/static`. CI rebuilds it and fails if the committed copy is stale.

## Dev loop

```bash
make ui-dev                     # or: cd web/review && npm ci && npm run dev
```

The Vite dev server proxies `/api` to a running Kaizen server (see `vite.config.ts`),
`http://127.0.0.1:7799` by default:

```bash
kaizen review-demo --port 7799 --open=false   # scripted fixture PR, no AI cost
# or a real PR; a cached review replays instantly
kaizen review 42 --port 7799 --open=false
make ui-dev KAIZEN_URL=http://127.0.0.1:7799
```

Open the Vite URL with the same `?t=<token>` that kaizen printed. Every API call needs
that session token.

| Command | What it does |
|---|---|
| `npm run dev` | Vite dev server with hot reload |
| `npm run typecheck` | `tsc -b` (strict) |
| `npm test` | vitest (jsdom) |
| `npm run build` | typecheck, then build into `../../pkg/reviewui/static` |
| `make ui` / `make ui-test` | the same from the repo root (`npm ci` first if needed) |

Build file names are stable (no content hashes), so a rebuild gives a small, reviewable
diff. The server sends `Cache-Control: no-cache` for these assets.

## Architecture

```
src/
  main.tsx, Boot.tsx       load /api/context, create the store, mount <App>
  App.tsx                  header + Workspace(sidebar | code | context) + overlays
  types.ts                 wire types, mirroring pkg/review/types.go
  state/
    store.ts               the zustand store: context, findings, selection, history, UI
    StoreProvider.tsx      useReview / useReviewShallow / useFindings / useSidebarGroups
    commands.ts            useCommands(): components register keyboard actions they own
    indexing.ts            lookup maps built once from the context
  keys/keymap.ts           every key binding + a pure key-sequence parser
  hooks/useKeymap.ts       installs the keymap; handles store-level actions
  hooks/useEventStream.ts  SSE /api/events -> store.applyEvent
  lib/                     pure helpers: api, findings, grouping, format, risk
  theme/                   tokens.css (design tokens), base.css (shared primitives)
  components/
    layout/                Workspace (panes, dividers, collapse), ContextPanel tabs, ModalDialog
    header/ sidebar/ code/ review/ flow/ function/ activity/   one folder per area
    help/                  HelpOverlay, generated from keymap.ts
    post/                  PostModal: post the review to GitHub
    shared/                Markdown (safe renderer), AgentTag, SeverityCounts
```

### Store

One store holds the static review context, the live engine stream (findings, result,
status, activity), the selection and its history, and UI state (tab, focused pane,
collapsed panes, modals). Components read slices with `useReview(selector)` (use
`useReviewShallow` when the selector builds an object or array) and call actions on the
store.

The selection says what is selected (file, function, flow node, finding, line).
`selection.reveal` increments each time the code view should scroll to the selection,
so selecting the same thing again still scrolls. Navigation actions push onto `history`
for `Ctrl-o`.

### The human's review (no cognitive surrender)

The AI proposes; the developer verifies and decides. Nothing the AI wrote is posted
unless the developer accepted it.

- **Triage** (`state/triage.ts`): `triage[findingId] = { state: 'accepted' | 'dismissed', reason?, note? }`;
  absent means unreviewed. `a` accepts, `d` dismisses (reason picker, `1`-`4`, Esc cancels),
  `x`/Space toggles accept. An accepted finding's note is appended to its comment as
  `**Reviewer note:** …`. The Review panel filters Unreviewed / Accepted / Dismissed / All.
- **Your comments** (`state/comments.ts`): the gutter "+" (or `c` on the cursor line;
  shift-click or drag for a range) opens a composer; "Comment on file" in the file header.
  They post first, as findings with `source: "you"`, `title: ""`, id `c-…`.
- **Viewed** (`state/viewing.ts`, `code/useViewTracking.ts`): a function counts as viewed
  after its code has been on screen for 2s while selected; a file after 2s open.
- **Think first** (`T`): hides the AI's findings, verdict and counts until the reviewer reveals them.
- **Persistence** (`state/persistence.ts`): triage, viewed, comments and reveal are saved in
  `localStorage` under `kaizen-review:<owner>/<repo>#<number>@<head_sha>` (a new push starts
  fresh); the Think first preference under `kaizen-review-think-first`.

### Keyboard: keymap and commands

`keys/keymap.ts` holds every binding in `BINDINGS` (keys, action, group, description).
The help overlay (`?`) is generated from it, so it cannot drift from what the keys do.
The parser handles counts (`5j`) and sequences (`gt`, `]c`).

`hooks/useKeymap.ts` handles actions that only touch the store (selection, tabs, pane
focus, modals). Actions that need a component's internal state (scrolling, hunks, the
flow diagram's zoom) are registered by that component:

```tsx
useCommands({
  halfPageDown: (count) => scrollBy(count * pageHeight / 2),
  top: () => scrollTo(0),
});
```

To add a key: add a binding to `BINDINGS` (and the `Action` union), then handle it in
`useKeymap` if it is store-only, or with `useCommands` in the component that owns it.

### Design tokens

The look is a dark dev tool: near-black surfaces, structure from hairlines (`--line`) and
spacing rather than boxes, and colour only where it carries information: severity
(`--sev-*`), risk (`--risk-*` / `riskColor()`), diff (`--diff-*`) and one accent
(`--accent`) for selection, focus and the primary action. Inter 13px for UI, JetBrains
Mono for code, paths and numbers. Hierarchy comes from weight and
`--text` / `--text-2` / `--text-3`, not from size jumps. Use only the tokens in
`theme/tokens.css`, and the shared classes in `theme/base.css` (`.btn`, `.label`,
`.count`, `.agent`, `.spinner`, `.empty`, `kbd`).

Component CSS lives next to the component (`Component.css`) with class names prefixed by
area (`.ws-` workspace, `.cp-` context panel, `.pm-` post modal, `.hp-` help) to avoid
collisions.

### Security

PR text, code and AI output are untrusted. Render them as React text or through
`<Markdown>`. Never use `dangerouslySetInnerHTML`. Only `http(s)` URLs become links
(`isSafeHttpUrl`).

## Adding a context panel

1. Create `src/components/<name>/<Name>Panel.tsx` (and `<Name>Panel.css`).
2. Add the tab name to `TabName` / `TABS` in `state/store.ts`.
3. Add its label and `g<n>` key to `TAB_LABELS` / `TAB_KEYS` in
   `components/layout/ContextPanel.tsx`, render it in the tab body, and add a
   `{ keys: ['g', '<n>'], action: 'tab', argument: '<name>' }` binding in `keys/keymap.ts`.
4. Optionally add a badge in `components/layout/tabBadges.ts`.
5. Register any keyboard actions the panel owns with `useCommands`.
6. Add vitest tests for its pure logic next to it (`*.test.ts`).

## Layout

`components/layout/Workspace.tsx` lays out sidebar | code | context with 1px hairline
dividers. Drag a divider to resize; double-click it to reset; hover it for a collapse
button; arrow keys nudge it when focused. Widths are clamped so the code view keeps at
least 360px, and persisted in `localStorage` (`kaizen-review-layout`). A collapsed pane
leaves a rail that restores it. `h`/`l` move pane focus (a thin accent line marks the
focused pane). `F` gives the context pane the full width. Below 768px the sidebar starts
collapsed; below 640px the panes stack vertically.

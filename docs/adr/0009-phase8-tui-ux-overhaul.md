# ADR 0009 — Phase 8: TUI & CLI UX Overhaul

**Status:** Accepted
**Date:** 2026-02-28

---

## Context

The original TUI (`internal/ui/app/model.go`) was a single monolithic 910-line model.
Problems included:

- Source list had no scrolling — unusable with >15 sources
- Reader pane truncated content at 320 characters with no markdown rendering
- Only the library list was keyboard-navigable; all other panes were read-only
- Command palette required memorising exact syntax with no discoverability
- No loading spinners, no help overlay, no search/filter
- `graph-tui/` directory existed but was completely empty
- Status bar overflowed on small terminals
- `--vault` flag was required on every single CLI command

---

## Decision

### 1. Stay with Bubble Tea; adopt `charmbracelet/bubbles` and `glamour`

The framework (`bubbletea v1.3`) is not the problem — the project simply never
adopted the higher-level `bubbles` component library.

**Added dependencies** (already present in `go.mod`):
- `github.com/charmbracelet/bubbles` — list, viewport, textinput, spinner, key, help
- `github.com/charmbracelet/glamour` — markdown rendering in the Reader tab

### 2. Tab-based layout replaces the fixed 3-pane layout

Each tab owns the full terminal height, giving every view enough vertical space
to be useful.

```
┌─────────────────────────────────────────────────────────┐
│  mdht  │ Library │ Reader │ Graph │ Collab │ Plugins    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  (full-screen content for active tab)                   │
│                                                         │
├─────────────────────────────────────────────────────────┤
│  status message              ?:help  tab:switch  q:quit │
└─────────────────────────────────────────────────────────┘
```

| Tab | Layout | Key components |
|-----|---------|----------------|
| Library | 40% list / 60% detail+preview | `bubbles/list`, `bubbles/viewport` |
| Reader | Full viewport | `bubbles/viewport` + `glamour` |
| Graph | 35% node list / 65% neighbor display | `bubbles/list`, `bubbles/viewport` |
| Collab | Peers / Activity / Conflicts sub-tabs | `bubbles/list`, `bubbles/viewport` |
| Plugins | Plugin name input → command list → output | `bubbles/textinput`, `bubbles/list`, `bubbles/viewport` |

Global overlays (any tab):
- `?` — help overlay (`bubbles/help`)
- `:` — command palette with autocomplete hints (`bubbles/textinput`)
- Loading states — spinner (`bubbles/spinner`)

### 3. File structure

```
internal/ui/
├── app/
│   └── model.go          ← top-level model: tab routing, global keys, overlays
├── views/
│   ├── library/view.go   ← Library tab
│   ├── reader/view.go    ← Reader tab
│   ├── graph/view.go     ← Graph tab (replaces empty graph-tui/)
│   ├── collab/view.go    ← Collab tab
│   └── plugins/view.go   ← Plugins tab
├── components/
│   └── palette.go        ← Shared command palette component
└── theme/
    └── catppuccin.go     ← Unchanged
```

Each view implements `Init() tea.Cmd / Update() / View() string` so the
top-level model can delegate to it uniformly.

### 4. Component boundary per view

| View | Owns | Does NOT own |
|------|------|--------------|
| library | list scroll, detail display, filter | session start, reader open |
| reader | viewport scroll, glamour render, page nav | source selection |
| graph | topic list, neighbor display, filter | source/session state |
| collab | peer/activity/conflict sub-tabs, sync | conflict resolution UI |
| plugins | plugin name input, command list, exec output | source/session state |

The top-level model owns: tab routing, session lifecycle, palette, help overlay,
status bar, and global key bindings.

### 5. `MDHT_VAULT` environment variable

`--vault` was required on every CLI command.  Adding an env-var fallback
eliminates the single biggest CLI friction point.

```
MDHT_VAULT=/path/to/vault mdht source list
MDHT_VAULT=/path/to/vault mdht tui
```

Implemented in `cmd/mdht/main.go` in the root command's `PersistentPreRun`,
reading `os.Getenv("MDHT_VAULT")` when `--vault` is not explicitly set.

---

## Consequences

**Positive:**
- Source list scrolls correctly regardless of collection size
- Reader renders markdown with syntax highlighting via glamour
- Every tab is fully keyboard-navigable
- Command palette shows autocomplete hints typed in real time
- Spinners give clear feedback during async operations
- `MDHT_VAULT` eliminates the constant `--vault` repetition
- Graph tab is now implemented (was an empty directory)

**Neutral:**
- All port interfaces from the original model are retained — only their location
  changed (top-level model → per-view files with narrowed interfaces)
- The graph tab uses `GraphCLI.ListTopics` and `GraphCLI.Neighbors`; path
  navigation can be added in a future phase

**Negative / trade-offs:**
- More files to maintain (one view per tab vs. one monolithic model)
- Tab-based layout loses side-by-side comparison ability of the 3-pane design

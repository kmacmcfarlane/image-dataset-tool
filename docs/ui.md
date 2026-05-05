# UI Layout Reference

## Layout structure

Hybrid sidebar + workspace layout.

```
┌──┬──────────────────────────────────────────────┐
│  │  [breadcrumb / context bar]                   │
│ N│                                               │
│ A├──────────────────────────┬────────────────────┤
│ V│                          │                    │
│  │    Main Content          │  Context Panel     │
│ S│    (grid, list, form)    │  (resizable,       │
│ I│                          │   collapsible)     │
│ D│                          │                    │
│ E│                          │                    │
│ B├──────────────────────────┴────────────────────┤
│ A│  [status bar: jobs, rate limit, etc]           │
│ R│                                               │
└──┴──────────────────────────────────────────────┘
```

## Components

### Sidebar (left)

- Narrow icon-only by default, expandable to show labels on hover/click.
- Navigation items (top to bottom):
  - Projects (home)
  - Jobs
  - Studies
  - Queues
  - Accounts
  - Settings
- Active route highlighted.

### Breadcrumb bar (top)

- Shows current navigation path: Project > Subject > Samples.
- Each segment is clickable for quick nav back.
- Right side: global actions (e.g., search, user menu if needed later).

### Main content area

- Takes remaining horizontal space.
- View-specific content: grids, lists, forms, dialogs.
- In review view: virtual-scrolling sample grid.

### Context panel (right)

- Resizable via drag handle.
- Collapsible (toggle button or keyboard shortcut).
- Content depends on current view:
  - **Review grid**: caption panel (captions grouped by study, inline editing).
  - **Job detail**: job log messages (filterable by level).
  - **Sample detail**: full metadata, edit controls, caption history.
- Remembers size in localStorage.

### Status bar (bottom)

- Always visible, single line height.
- Left: active job count badge + current job name/progress.
- Center: rate limit indicator (amber when active, with backoff timer).
- Right: connection status (SSE connected/disconnected).
- Click job area to expand job detail drawer (slides up from bottom).

### Error banner

- Dismissable banner at top of content area (below breadcrumb).
- Red background for errors, amber for warnings.
- Shows API errors, system-level issues.
- Auto-dismisses after 10s for non-critical errors. Persists for critical errors
  until manually dismissed.

## View-specific layouts

### Project list (/)

- Card grid or list of projects.
- Each card: name, description, subject count, created date.
- "Create project" button in top-right.

### Subject list (/projects/:id)

- Cards with: name, sample count (pending/kept/rejected badges), linked accounts.
- "Add subject" button. "Pull" button per subject (triggers ingest job).

### Review grid (/projects/:id/subjects/:sid/samples)

- Full workspace layout with grid + context panel.
- Grid: virtual scroll, lazy thumbnails, selection highlighting.
- Status filter toggles above grid (pending/kept/rejected + duplicate toggle).
- Context panel: caption side panel (default) or sample detail.
- Lightbox: Enter key opens, arrow keys navigate, K/R work inside lightbox.
  Caption panel visible in lightbox too.

### Job list (/jobs)

- List of job_runs with: type, status, progress bar, timing, error/warning count
  badges.
- Click to expand: job log messages in context panel, filterable by level (default:
  warning).
- Active jobs show real-time SSE updates.

### Studies (/studies)

- List of caption studies grouped by project.
- Create/edit/delete with form dialog.

### Export (/export)

- Multi-step dialog: select subjects → select study → validate → choose destination.
- Dialog remembers last values in localStorage.

### Queue admin (/queues)

- Table of consumer stats per subject.
- Click consumer to see messages (peek).
- Action buttons: retry, purge, delete.

### Accounts (/accounts)

- List of social media accounts with platform, handle, last login.
- Add dialog: platform dropdown, handle input, cookie paste textarea.

### Settings (/settings)

- Sections: secrets, providers, config values (all from config.yaml, read-only).
- Data dir path, encryption key status, thumbnail config.

## UX conventions

- **Keyboard shortcuts**: see PRD §8.3.1 for review grid shortcuts.
- **Dialog state**: all dialogs persist last values to localStorage.
- **SSE connect-then-fetch**: views with live data connect SSE first, then fetch.
- **Dismissable errors**: API errors show in top banner, auto-dismiss non-critical.
- **Loading states**: skeleton loaders for initial data fetch, not spinners.
- **Toast notifications**: job messages (error, warning, info) trigger toasts.
  Toast level threshold defaults to warning (configurable in settings, persisted
  in localStorage). Error toasts persist until dismissed. Warning toasts auto-dismiss
  after 5s. Info toasts auto-dismiss after 3s.

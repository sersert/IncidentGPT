# Operator interface and guide review

The interface offers neutral light and graphite dark palettes with restrained
blue links and separate severity colors. A header toggle persists the choice;
the initial default follows the system theme. Navigation is compact; metrics share one strip,
and incident queues use aligned rows rather than repeated floating cards.
The overview focuses on data; instructional content lives in the guide. The main changes are the responsive navigation,
shift overview, paginated/filterable incident queue, incident workspace tabs and an
in-app on-call guide. Shared buttons/cards/inputs retain existing APIs and routes.

## Visual references

All captures use deterministic API fixtures, marked in the images. They illustrate
layout and UI behavior, not a connected cluster or a production deployment.

| Screen | Desktop | Mobile (390px) |
| --- | --- | --- |
| Shift overview | [Overview](images/ui/desktop-overview.png) | [Overview](images/ui/mobile-overview.png) |
| Incident queue | [Queue](images/ui/desktop-incidents.png) | [Queue](images/ui/mobile-incidents.png) |
| Incident workspace | — | [Workspace](images/ui/mobile-incident.png) |
| On-call guide | — | [Guide](images/ui/mobile-guide.png) |
| Dark shift overview | [Overview](images/ui/desktop-overview-dark.png) | [Overview](images/ui/mobile-overview-dark.png) |
| Dark incident queue | [Queue](images/ui/desktop-incidents-dark.png) | — |

## Candidate validation

Local UI TypeScript checking and production build passed. The existing bundle-size
warning remains. Helm dependency packaging, umbrella lint and rendering passed
with [the new preview values](examples/values-preview.yaml). Those are template
checks; placeholders must be replaced and secrets supplied before deployment.

A headless Microsoft Edge session with Playwright, local Vite and intercepted API
fixtures passed 23 checks on 2026-10-19:

- Overview data rendering and desktop layout.
- 25-row pagination, the second page and browser Back.
- Route navigation returns to the content start, while guide fragments retain their target.
- Severity/status filters and reset, including transmitted API query parameters.
- Debounced search, slash shortcut, empty results and recovery.
- Failed list load and retry recovery.
- Incident deep links and keyboard focus on arrow/Home tab navigation.
- Unsaved resolution retained across workspace tabs.
- Failed assignment retains its dialog and entered owner; a successful retry closes it.
- Mobile menu selection, Escape and return of focus.
- Every incident workspace at 390px without page-level horizontal overflow.
- Guide deep link, FAQ and mobile layout.
- Failed dashboard stats display unknown measurements rather than healthy zero counts.
- Additional incident/list/guide overflow checks at 320px and 768px.
- System theme changes before manual selection.
- Explicit toggle, SVG recoloring and persistence across reload/navigation.
- Recharts recoloring immediately on the current route.
- Unsaved incident draft retained while switching themes.
- Dark layouts at all four viewport widths.
- Preference synchronization across browser tabs.
- Stored preference applied before the application bundle.
- Safe operation when browser storage is blocked.
- No uncaught browser exceptions across the scenario.

Main screenshots were reviewed at 1440px and 390px. The main text, link and
semantic text colors were checked against white: minimum contrast 4.8:1.
The light palette also updates chart tooltips, raw-data views, filters and settings. Fixtures exercise UI behavior
without making real mutations or provider calls. They do not verify backend
credentials, Redis persistence, live integrations, a browser/platform matrix or
production ingress/TLS. Existing backend APIs and storage are unchanged.

CI results and the exact candidate hash belong in the PR. A candidate deployment
with intended configuration, image digests and ingress/health/smoke evidence is
still required before merge under the repository working agreement. The local
Docker daemon was unavailable during this work; no production deployment or
promotion was performed.

## Instructions

[Getting started](getting-started.md) covers building the candidate, a separate
preview, credentials, the first alert, expected outcomes and troubleshooting.
[Web UI](web-ui.md) describes navigation, deep links, editor persistence and access.
[UI development](../ui/README.md) explains local API/demo modes and their limits.
The in-app `/help` page focuses on the on-call workflow rather than deployment.

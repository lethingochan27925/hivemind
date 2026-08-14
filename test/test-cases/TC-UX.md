# TC-UX — Control platform interface: search, i18n, theme, layout

Traces to: **Production Readiness** (an operator console you can actually work in). Manual/exploratory unless noted. Precondition: dashboard deployed, fleet has processed cases.

## A. Global search (⌘K / Ctrl+K)

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-UX-01** | P0 | Press `Ctrl+K` anywhere | Palette opens focused; `Esc` closes; `Ctrl+K` again toggles. Works on every page. |
| **TC-UX-02** | P0 | Type `fraud` | Results appear under **Data** within ~1s: matching tasks and memories, alongside the static **Go to** / **Actions** entries. |
| **TC-UX-03** | P0 | Type the first 8 characters of a transaction id | The transaction is listed; `Enter` opens **Transactions** with that row already selected and its Decision trace loaded. |
| **TC-UX-04** | P1 | Type a fragment of an account name (`C1305`) | Matching transactions surface — search covers `name_orig`/`name_dest`, not just ids. |
| **TC-UX-05** | P1 | Type a pattern name (`balance_wipe`) | Matching **memories** appear; `Enter` opens the Database console with the row's query pre-run. |
| **TC-UX-06** | P0 | Type one character | No request is sent (min length 2) — asserted by `TestSearchRejectsUselessQueries`. |
| **TC-UX-07** | P1 | Arrow keys + `Enter` | Selection moves; `Enter` runs the highlighted item; actions show a spinner and report their result inside the palette. |
| **TC-UX-08** | P1 | Run "Simulate agent crash" from the palette | Navigates to Infrastructure and the recovery tracker starts — a live mutation driven entirely from the keyboard. |
| **TC-UX-09** | P2 | Search a string with `%` or `_` (SQL wildcards) | Treated as literal text, no error — the query is parameterised. |

## B. Bilingual EN / VI

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-UX-10** | P0 | Click **VI** in the topbar | Navigation, page titles/descriptions, panel titles/subtitles and empty states switch to Vietnamese **on every page** (translation lives in the shared primitives). |
| **TC-UX-11** | P0 | Reload the page | The language persists (`localStorage`), and `<html lang>` is updated. |
| **TC-UX-12** | P0 | In VI, inspect a verdict badge, a service name, a table name, the SQL console | These stay **English** by design — they are real API/SQL values. A console that translated `fraud` → `gian lận` would misrepresent what is stored. |
| **TC-UX-13** | P1 | In VI, look for any untranslated label | It renders the English source, never a raw key or blank — the dictionary is keyed by the English string with fallback. |
| **TC-UX-14** | P2 | Switch EN↔VI repeatedly while data polls | No flicker, no lost state, no refetch storm. |

## C. Theme

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-UX-15** | P0 | Toggle sun/moon | Whole platform switches light↔dark pastel; charts, badges, grips and the architecture map all remain legible (no hard-coded dark hexes). |
| **TC-UX-16** | P0 | Reload | Theme persists, and there is **no flash of the wrong theme** (pre-paint bootstrap script). |
| **TC-UX-17** | P1 | First visit with OS set to dark | Dark is selected automatically (`prefers-color-scheme`). |
| **TC-UX-18** | P1 | Light theme, read the Learning curve + Verdict donut | Series colours keep contrast on white; nothing disappears into the background. |

## D. Layout: drag-resize, collapse, maximize

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-UX-19** | P0 | Hover the bottom edge of any panel | A grip appears; cursor becomes `ns-resize`. |
| **TC-UX-20** | P0 | Drag that grip down/up | Panel height follows the pointer; border highlights while dragging; content scrolls inside the new height. |
| **TC-UX-21** | P0 | Drag the bottom-right corner | Height **and width** change together. |
| **TC-UX-22** | P0 | Drag fast, leaving the panel with the button held | Resize keeps following the pointer (pointer capture) and stops cleanly on release — no stuck drag state. |
| **TC-UX-23** | P1 | Double-click a grip | Size resets to automatic. |
| **TC-UX-24** | P0 | Resize a panel, reload | The size is remembered per panel (`localStorage`, keyed by title). |
| **TC-UX-25** | P1 | Collapse a panel, reload | It stays collapsed; expanding restores the previous size. |
| **TC-UX-26** | P1 | Maximize a panel; press `Esc` | Full-screen overlay opens with the body scrollable; `Esc` restores. |
| **TC-UX-27** | P1 | Drag the sidebar's right edge | Nav width changes between 180–420px, persists on reload; double-click resets to 240. |
| **TC-UX-28** | P2 | Resize the browser window to mobile width | No horizontal body scroll; panels reflow; a panel wider than the viewport is capped (`maxWidth: 100%`). |

## E. Live-data behaviour

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-UX-29** | P0 | Approve/reject/override a case | The row updates or disappears **immediately** (optimistic), then reconciles with the next poll — no waiting for the interval. |
| **TC-UX-30** | P1 | Switch to another browser tab for a minute | Polling pauses while hidden and resumes on focus (verify Lambda invocation count does not grow). |
| **TC-UX-31** | P1 | Stop the API (or point at a bad URL) | Pages show "Disconnected" and empty states — never a blank white screen or an unhandled crash. |
| **TC-UX-32** | P1 | Click a node in the Architecture map | Navigates to the matching control surface: Lambda → node detail, S3/EventBridge/… → that inventory group, CockroachDB → Database, Bedrock → Cost. |

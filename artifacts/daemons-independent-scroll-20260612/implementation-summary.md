# /daemons Independent Pane Scrolling

## Summary

Implemented independent scrolling for the TUI `/daemons` browser. The previous implementation rendered the left daemon-run list and right detail pane into one combined viewport, so scroll state was shared. The new implementation gives each pane its own viewport and lets the user choose which pane receives page/mouse scrolling.

## Changes

- Replaced the single combined daemon browser viewport with separate `listVP` and `detailVP` viewports.
- Added focused pane state:
  - default focus: detail pane;
  - `tab` / `shift+tab` toggles focus;
  - `left` focuses list;
  - `right` focuses detail.
- Preserved `↑↓/jk` as daemon-run selection controls.
- Selection changes reset only the detail pane scroll to top and keep the selected list row visible.
- `pgup`/`ctrl+u`, `pgdown`/`ctrl+d`, and mouse wheel scroll the focused pane.
- Updated `/daemons` footer hint to show the focused pane and scroll controls.
- Updated `tui/internal/tui/ANATOMY.md`.
- Added focused tests for independent scroll, left/right focus, and list visibility on selection changes.

## Validation

Passed:

- `cd tui && go test ./internal/tui -run 'TestDaemonsPaneScrollFocusIsIndependent|TestDaemonsSelectionKeepsListVisibleAndResetsDetailScroll|TestDaemonsLeftRightChooseFocusedPane|Test.*Daemon|TestDefaultCommandsIncludesDaemons|TestDaemonsCommandOpensDaemonsView' -count=1`
- `cd tui && go test ./internal/tui -count=1`
- `cd tui && go test ./... -count=1`
- `git diff --check`

## Notes

Per standing rule, a Claude Code async attempt (`job-3064ef8f`) was started first with the implementation prompt, but it produced no file changes after several minutes and was canceled. The patch was then implemented manually and fully validated.

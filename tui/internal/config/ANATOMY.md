# config — bootstrap, venv, global config

> **Maintenance:** see `lingtai-tui-anatomy` (at `tui/internal/preset/skills/lingtai-tui-anatomy/SKILL.md`).

This package manages the TUI's bootstrap sequence — the steps that run before any agent launches. It owns the Python runtime venv, the lingtai CLI upgrade path, addon verification, and the global config files under `~/.lingtai-tui/`.

## Components

- **`tui/internal/config/venv.go:50-63`** — `NeedsVenv`: returns true only when the managed runtime venv is missing or cannot import `lingtai`. A working PyPI wheel may still need conversion to local editable/dev mode; that is handled by the always-run `CheckUpgrade` / `UpgradePythonRuntime` path instead of recreating the whole venv.
- **`tui/internal/config/venv.go:78-163`** — `EnsureVenv`: creates the runtime venv at `~/.lingtai-tui/runtime/venv/`. Uses `uv` if available (can auto-download Python 3.13), falls back to system Python. Verifies Python 3.11+, installs `lingtai`, symlinks CLI into PATH. On a dev machine, it installs the discovered `lingtai-kernel` checkout editable rather than PyPI when creating a fresh runtime.
- **`tui/internal/config/devmode.go:64-88`** — `findDevCheckouts`: discovers local LingTai development workspaces only through the explicit `$LINGTAI_DEV_ROOT` contract. The env var points at a root that directly contains sibling `lingtai-kernel/` and `lingtai/` checkouts. A workspace is valid when `lingtai-kernel/pyproject.toml` exists and a sibling `lingtai/` TUI repo exists. Only `lingtai-kernel` is passed to `pip install -e`; the Go `lingtai/` repo is a workspace marker, not a Python package.
- **`tui/internal/config/venv.go:301-308`** — `CheckUpgrade`: thin wrapper over `UpgradePythonRuntime(globalDir, false, ...)` that returns `true` only when a real PyPI upgrade or dev-mode conversion was verified post-install. Used by `EnsureRuntime`, where any failure is silent so the TUI can still launch.
- **`tui/internal/config/venv.go:320-330`** — `EnsureRuntime` / `EnsureRuntimeQuiet`: startup helpers that ensure the managed Python runtime exists, then always run the non-blocking `CheckUpgrade` path after a successful ensure. This is what converts an existing importable PyPI runtime into editable/dev mode on local development machines.
- **`tui/internal/config/venv.go:473-592`** — `RunDoctorUpdate`: the forced check-and-update routine called by `/doctor` and `lingtai-tui doctor`. Verifies the TUI binary against the latest GitHub release, reports the detected install method, routes forced TUI updates through `RunTUIUpdate` (Homebrew installs brew-upgrade; source/user-local or unknown installs get manual guidance), repairs a missing/broken runtime venv, then runs `UpgradePythonRuntime(force=true)`, then reports native file-search sidecar/Rust availability via `checkFileSearchNative`. All command stdout/stderr is captured after command completion and surfaced; nothing is silently swallowed.
- **`tui/internal/config/venv.go:594-712`** — `detectTUIInstallMethod` and helpers: read `~/.lingtai-tui/install.json` source metadata (`lingtai.tui.install/v1`) and match it against the running executable (including the `lingtai` alias symlink in `bin_dir`), then fall back to Homebrew path/env detection (`HOMEBREW_PREFIX`, common Homebrew/Linuxbrew prefixes) or `unknown/other`. Source metadata wins over Homebrew-looking paths so source installs into a Homebrew bin dir are still reported as source.
- **`tui/internal/config/tui_updater.go:1-159`** — `TUIUpdater` interface plus `RunTUIUpdate` backend routing. The Homebrew backend runs the existing `brew upgrade lingtai-ai/lingtai/lingtai-tui` flow, with doctor/self-update optional preceding `brew update` (`IncludeHomebrewUpdate`) and full-path command reporting (`ResolveHomebrewPath`); `RunManualTUIUpdate` turns source/user-local and unknown installs into explicit unsupported command results instead of running brew. Callers keep version checks/prompting; the backend owns only the mutation-or-guidance for one install method. (NOTE: line citations to recompute against current main via the anatomy citation checker.)
- **`tui/internal/config/venv.go:1033-1165`** — `UpgradePythonRuntime`: checks the installed Python `lingtai` runtime. If a local dev checkout is discovered and the runtime is not already editable for that checkout, it runs editable install (`uv pip install -e <lingtai-kernel> -p <venv>` or venv `pip install -e <lingtai-kernel>`) before any PyPI comparison. Otherwise it preserves existing editable installs, compares to PyPI, runs upgrade if needed, and verifies post-install import/version.
- **`tui/internal/config/venv.go:383-466`** — `DoctorOptions` / `CommandRunner` / `TUIInstallMethod` / `TUIInstallInfo`: dependency-injection seams so tests stub brew/pip/python/home/env without dialing the network, plus the small install-method result type used by doctor reporting and updater routing. Production callers leave the seams nil and get real `exec.Command` + `http.Client` + `os.UserHomeDir` / `os.LookupEnv`.
- **`tui/internal/config/venv.go:777-889`** — `checkFileSearchNative` / `FileSearchStatus` / `timeoutCommandRunner` / `FileSearchNativeStatus`: asks the managed Python runtime which file-search backend it will use (`RustFileIOBackend` sidecar vs Python fallback) and reports both packaged sidecar status and local Cargo availability for `/doctor` / startup guidance. The probe runs through a `timeoutCommandRunner` (3s default) so startup and `/doctor` cannot hang on a slow or broken managed Python runtime. A `ModuleNotFoundError` for `lingtai.services.file_io_sidecar` (a runtime that predates the Rust sidecar release) is reported as `FileSearchStatus{Unsupported: true}` rather than an error; `checkFileSearchNative` then emits a single `DoctorInfo` line and the report stays healthy.
- **`tui/internal/config/venv.go:265-300`** — `EnsureAddons`: reads `init.json`'s `addons` map, verifies each addon is importable as `lingtai.addons.<name>`. Error surfaces which addon is missing and suggests `pip install --upgrade lingtai`.
- **`tui/internal/config/venv.go:240-263`** — `CheckTUIUpgrade`: compares running TUI binary version against latest GitHub release. Returns tag if upgrade available. Used by the startup-time prompt in `main.go`.
- **`tui/internal/config/venv.go:936-1011`** — `CompareReleaseVersions` / `ReleaseComparison`: classifies strict `vX.Y.Z` release comparisons as update-available, up-to-date, current missing, dev build, current non-semver, or latest non-semver; `/doctor` uses this so hash-like current versions do not look up-to-date.
- **`tui/internal/config/venv.go:168-190`** — `linkLingtaiCLI`: symlinks `lingtai` CLI from venv into brew prefix or `~/.local/bin`.
- **`tui/internal/config/global.go:30-42`** — `Config`: global config at `~/.lingtai-tui/config.json`. Keys map env-var names to API key values.
- **`tui/internal/config/global.go:78-91`** — `TUIConfig`: TUI preferences at `~/.lingtai-tui/tui_config.json` (language, mail page size, theme, insights, tool-call truncation limit — `tool_call_truncate`, 0 = no truncation).
- **`tui/internal/config/global.go:178-188`** — `WriteEnvFile`: writes `~/.lingtai-tui/.env` from Config.Keys. Loaded by agents via `env_file` in `init.json`.
- **`registry.go`** — preset registry management (see `preset/ANATOMY.md`).

## Connections

- **Called from:** `tui/main.go:132-278`, the manual `lingtai-tui self-update` path, `tui/internal/tui/firstrun.go:672-675`, and `tui/internal/headless/spawn.go:77-83` — startup, manual self-update, first-run, and headless bootstrap paths. (NOTE: line citations to recompute against current main via the anatomy citation checker.)
- **Calls out:** PyPI API (`pypi.org/pypi/lingtai/json`), GitHub API (`api.github.com/repos/Lingtai-AI/lingtai/releases/latest`), `uv` / `pip` CLI, and Homebrew for Homebrew-managed TUI binary updates.
- **Bootstrap sequence:**
  1. `config.MigrateLegacyLanguage(globalDir)` then `config.LoadTUIConfig` + `i18n.SetLang` — one-shot language migration and locale resolution, run early (`tui/main.go:132-134`) so startup banners localize; the rest of the sequence follows at `tui/main.go:237-278`
  2. `config.NeedsVenv(globalDir)` — check if a setup banner should be printed
  3. `config.EnsureRuntime(globalDir)` — create/repair venv if needed, then always auto-check/upgrade or convert the `lingtai` Python runtime
  4. `config.EnsureAddons(python, agentDir)` — verify addon importability
  5. `preset.Bootstrap(globalDir)` — copy preset resources
  6. `tui.ExportCommandsJSON(globalDir)` — export slash commands
  7. `maybePromptRustToolchain(globalDir)` — one-time optional Rust/Cargo prompt when native file search is unavailable and Cargo is missing

## Composition

- **Parent:** `tui/internal/`
- **Sibling packages:** `tui/internal/preset/`, `tui/internal/migrate/`, `tui/internal/process/`

## State

- **Writes:** `~/.lingtai-tui/runtime/venv/` (Python venv), `~/.lingtai-tui/runtime/rust-toolchain-prompted` (one-time Rust prompt marker), `~/.lingtai-tui/config.json` (API keys), `~/.lingtai-tui/tui_config.json` (TUI prefs), `~/.lingtai-tui/.env` (env file for agents).
- **Reads:** local dev checkout root (`$LINGTAI_DEV_ROOT`), `init.json` (addon declarations), PyPI/GitHub APIs (version checks).

## Notes

- **MCP packages are dependencies of `lingtai`.** `lingtai` on PyPI is a meta-package that bundles `lingtai-kernel` + all addon MCPs. `pip install --upgrade lingtai` upgrades everything. Users never install MCP packages individually.
- **`EnsureRuntime` runs on every TUI launch** (for returning users), first-run bootstrap, and headless spawn. It always runs `CheckUpgrade` after any successful venv creation/repair; the PyPI/dev-mode check is non-blocking (3s HTTP timeout for PyPI) and silently no-ops on errors. The same logic — minus the silence — is reachable via `lingtai-tui doctor` or `/doctor`, which call `RunDoctorUpdate` with `ForceTUI=true` / `ForcePython=true` and surface every command's stdout/stderr.
- **Dev mode conversion:** local dev workspaces are enabled explicitly with `$LINGTAI_DEV_ROOT` (for example, `LINGTAI_DEV_ROOT=~/work/GitHub` when that directory contains sibling `lingtai-kernel/` and `lingtai/`). If configured and valid, the runtime should track `lingtai-kernel` editable. Existing PyPI wheels are converted on the next launch via `CheckUpgrade`; existing editable installs for the same checkout are left alone to avoid reinstalling every launch. No common-directory auto-scan runs by default, so ordinary users with source clones are not silently switched off PyPI.
- **`uv` preferred over `pip`:** all pip operations prefer `uv` if available (faster, can auto-download Python).

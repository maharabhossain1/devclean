# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**DevClean** is a macOS developer system intelligence CLI written in Go. The core idea: *prove ownership before trusting a file*. It builds a map of installed apps, scans for all system traces, and classifies each trace as owned, orphaned, or unknown — flagging silent AI/agent installs as a security concern, not just a storage concern.

The full specification lives in `devclean-srs.md`.

## Build & Run

```bash
go build ./...                    # Build all packages
go run ./cmd/devclean             # Run without building
go test ./...                     # Run all tests
go test ./engine/...              # Run tests for a specific package
go test -run TestCorrelator ./engine/  # Run a single test
```

Distribution target: single binary installable via Homebrew formula.

## Architecture

The central piece is the **App-Trace Correlation Engine** (`engine/`):

1. **`engine/app_registry.go`** — Catalogs everything installed: `/Applications`, Homebrew (formulae + casks), MAS, pip, npm/pnpm global, cargo, MCP servers (`~/.claude/mcp_servers.json`), LaunchAgents, ad-hoc binaries in `~/bin`, `~/.local/bin`, `/usr/local/bin`. Each entry stores name, bundle ID, install source, last-launch time (from `mdls`), version.

2. **`engine/trace_scanner.go`** — Independently walks known trace locations: `~/Library/{Application Support,Preferences,Caches,Containers,Logs,LaunchAgents}`, dotfiles (`~/.<name>/`, `~/.<name>rc`), `/tmp/<name>*`, `/Library/LaunchDaemons`.

3. **`engine/correlator.go`** — The brain. Matches each trace to an app via: exact bundle ID, fuzzy name match, a built-in 200+ app dictionary (YAML embedded at build time), plist `Program` key inspection for LaunchAgents. Produces one of four states per trace: **Owned**, **Orphaned**, **Unknown**, **Dead agent**.

4. **`engine/risk_scorer.go`** — Scores orphans by size, age, and risk. Green (safe to remove) / Yellow (warn) / Red (require typing item name to confirm).

### Package Map

```
cmd/            CLI entry points (Cobra commands)
engine/         App registry, trace scanner, correlator, risk scorer
scanners/
  library/      ~/Library sub-scanners (app_support, preferences, caches, containers, logs, launch_agents)
  dotfiles/     ~/.config and ~/.<app> scanners
  hidden_system/ /Library/LaunchDaemons, /usr/local/bin, etc.
  brew/         Homebrew orphan + dependency graph
  dev_caches/   npm, pip, docker, pnpm caches
  ai_agents/    MCP servers, Claude/Cursor/Copilot install detection
cleaner/        Deletion engine; writes undo log to ~/.devclean/undo/<date>.json; uses Trash via AppleScript where possible
uninstaller/    Full app removal (rm -rf .app + all correlated traces + brew uninstall --cask)
tui/            Bubble Tea interactive UI
advisor/        Unused app recommendations via mdls kMDItemLastUsedDate
guardian/       Security scan: unknown binaries, silent installs, MCP server audit
config/         ~/.devcleanrc (TOML) reader/writer
```

## Tech Stack

- **Language:** Go (single binary, fast fs walk)
- **CLI:** Cobra + Viper
- **TUI:** Bubble Tea (charmbracelet)
- **Plist parsing:** `howett.net/plist` (pure Go, no macOS dependency)
- **macOS metadata:** `mdls` via `os/exec` (last-used date, bundle info)
- **Undo storage:** JSON log + macOS Trash via AppleScript
- **App-trace dictionary:** Embedded YAML baked in at build time, updatable via `devclean update-db`

## Safety Invariants

Three-layer safety system — never break these:

1. **Blocklist** (Layer 1): Never touch `~/Documents`, `~/Desktop`, `~/Downloads`, `~/Pictures`, `~/.ssh`, `~/.gnupg`, `~/.aws/credentials`, or any file modified < 24h ago.
2. **Classification gate** (Layer 2): Green = single confirmation; Yellow = explicit "yes"; Red = must type the item name.
3. **Undo log** (Layer 3): Every deletion logged to `~/.devclean/undo/<date>.json`. No root required for any operation.

## Build Phases (MVP Order)

1. App registry + ~/Library orphan scanner + correlator + `devclean scan` output
2. Bubble Tea TUI + undo log/Trash + `devclean uninstall`
3. Unused app advisor (mdls) + `devclean brew audit`
4. Guardian: unknown binaries, MCP tracker, LaunchAgent audit
5. `devclean schedule` (launchd), report export, `update-db`, Homebrew formula

## Key CLI Commands

```bash
devclean scan                # Full audit, ranked report, no deletion
devclean clean               # Interactive TUI cleanup
devclean clean --auto        # Auto-remove all Green orphans silently
devclean clean --dry-run     # Preview only
devclean apps --unused       # Apps not launched in 90+ days, sorted by size × days_unused
devclean uninstall <app>     # App + all its traces in one shot
devclean brew audit          # Homebrew orphan dependency graph
devclean guardian            # Security scan for unknown/AI-installed items
devclean report              # Export last scan to JSON/Markdown
devclean undo                # Restore last N deleted items
```

## Performance Target

Full scan must complete in < 30 seconds on a typical dev machine. Prefer concurrent fs walks; avoid redundant `mdls` calls by batching or caching results per session.

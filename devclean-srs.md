# Software Requirements Specification (v2)
## DevClean — Developer System Intelligence CLI

**Version:** 2.0 Draft | **Date:** 2026-05-24

---

## 1. Core Insight — What Makes This Different

Most cleaners ask *"what's big?"*. DevClean asks *"what has no owner?"*

The central intelligence is an **App-Trace Correlation Engine**: it builds a map of every installed application, then scans the entire system for traces (files, folders, preferences, logs, containers, agents) and classifies each trace as **owned**, **orphaned**, or **unknown**. Unknown items — especially those installed silently by AI agents, MCP servers, or scripts the user ran once — get flagged as a security concern, not just a storage concern.

---

## 2. Expanded Problem Space

| Problem | Example | v1 SRS covered? |
|---|---|---|
| Dev tool caches | npm, Docker, pnpm | Yes |
| App uninstall leaves traces | Zoom folder in `~/Library` after Zoom deleted | **No — new** |
| Rarely used apps wasting space | GarageBand (8 GB, last opened 2 years ago) | **No — new** |
| Brew dependency rot | 40 formulae installed as deps, app that needed them is gone | **No — new** |
| Silent AI/agent installs | MCP server dropped a binary in `/usr/local/bin`, nobody noticed | **No — new** |
| Hidden dotfiles with no app | `~/.mongorc.js`, `~/.zoom/` — app uninstalled, ghost remains | **No — new** |
| Unknown background agents | LaunchAgents plist pointing to a binary that no longer exists | **No — new** |

---

## 3. System Architecture

```
devclean/
├── cmd/                        # CLI entry points
├── engine/
│   ├── app_registry.go         # Builds the installed-app map
│   ├── trace_scanner.go        # Finds all traces system-wide
│   ├── correlator.go           # Links traces → apps (THE BRAIN)
│   └── risk_scorer.go          # Scores orphans by size + risk + age
├── scanners/
│   ├── library/                # ~/Library deep scan
│   │   ├── app_support.go      # ~/Library/Application Support
│   │   ├── preferences.go      # ~/Library/Preferences (plist parsing)
│   │   ├── caches.go           # ~/Library/Caches
│   │   ├── containers.go       # ~/Library/Containers (sandboxed apps)
│   │   ├── logs.go             # ~/Library/Logs
│   │   └── launch_agents.go    # ~/Library/LaunchAgents
│   ├── dotfiles/               # ~/.zoom, ~/.docker, ~/.aws, etc.
│   ├── hidden_system/          # /Library/LaunchDaemons, /usr/local/bin, etc.
│   ├── brew/                   # Homebrew orphan + dependency graph
│   ├── dev_caches/             # npm, pip, docker, etc. (from v1)
│   └── ai_agents/              # MCP servers, Claude, Cursor, Copilot installs
├── cleaner/                    # Deletion engine with undo log
├── uninstaller/                # App removal via `rm -rf` + brew uninstall
├── tui/                        # Bubble Tea interactive UI
├── advisor/                    # Unused app recommendations
├── guardian/                   # Security-focused unknown-install detection
└── config/                     # ~/.devcleanrc
```

---

## 4. Core Engine: App-Trace Correlation

### 4.1 Building the App Registry

The engine first catalogs what IS installed:

```
Sources:
  /Applications/*.app                    → macOS GUI apps
  /Applications/Utilities/*.app
  ~/Applications/*.app
  brew list --formula                    → CLI tools via Homebrew
  brew list --cask                       → GUI apps via Homebrew Cask
  mas list                               → Mac App Store apps
  pip list / uv pip list                 → Python packages
  npm list -g / pnpm list -g            → Global JS tools
  cargo install --list                   → Rust binaries
  ~/.claude/mcp_servers.json             → MCP servers (Claude Code)
  ~/Library/LaunchAgents/*.plist         → Background agents (parsed)
  /Library/LaunchDaemons/*.plist         → System daemons (parsed)
  /usr/local/bin, ~/.local/bin, ~/bin    → Ad-hoc binaries
```

Each entry in the registry contains: name, bundle ID (if app), install source, last launch time (from macOS metadata), and version.

### 4.2 Trace Scanner

Independently scans known trace locations:

```
~/Library/Application Support/<name>/
~/Library/Preferences/com.<vendor>.<app>.plist
~/Library/Caches/com.<vendor>.<app>/
~/Library/Logs/<AppName>/
~/Library/Containers/<bundle.id>/
~/Library/Group Containers/<group.id>/
~/Library/LaunchAgents/<label>.plist
~/.config/<appname>/
~/.<appname>/                             dotfiles
~/.<appname>rc                            dotfiles
/tmp/<appname>*/
```

### 4.3 Correlation Logic

For each trace found, the correlator attempts to match it to an installed app using:

1. **Bundle ID match** — exact (`com.zoom.us` → is Zoom installed?)
2. **Name fuzzy match** — `Application Support/Slack` → is Slack installed?
3. **Known map** — hardcoded dictionary of 200+ well-known app → trace mappings
4. **Plist inspection** — for LaunchAgents, read the `Program` key, check if the binary exists
5. **AI/MCP heuristics** — detect MCP-server-style installs (see §7)

Result for each trace:
- **Owned** — app is installed, skip
- **Orphaned** — app was uninstalled, trace remains, safe to remove
- **Unknown** — no registered app claims this, flag for review
- **Dead agent** — LaunchAgent/Daemon plist pointing to a missing binary

---

## 5. Functional Requirements

### 5.1 Commands

```bash
devclean scan              # Full audit, no deletion, print ranked report
devclean clean             # Interactive guided cleanup session
devclean clean --auto      # Auto-remove all Green orphans silently
devclean clean --dry-run   # Show what would be deleted, touch nothing
devclean apps              # Show installed apps ranked by size + last-used date
devclean apps --unused     # Show apps not launched in N days (default 90)
devclean uninstall <app>   # Guided uninstall: app + ALL its traces in one shot
devclean brew audit        # Show brew orphan deps + suggest what to uninstall
devclean guardian          # Security scan: unknown agents, silent installs, AI traces
devclean report            # Export last scan to JSON / Markdown
devclean schedule          # Set up weekly launchd reminder
```

### 5.2 `devclean scan` — Full Audit Output

```
DevClean v2.0 | Scan complete in 18s | Recoverable: 34.2 GB

ORPHANED APP TRACES                                    SIZE    AGE     RISK
──────────────────────────────────────────────────────────────────────────
● ~/Library/Application Support/zoom.us               1.2 GB  8 mo    Green
● ~/Library/Caches/com.microsoft.teams                890 MB  3 mo    Green
● ~/Library/Preferences/com.docker.docker.plist       12 KB   1 yr    Green
● ~/.zoom/                                             340 MB  8 mo    Green
● ~/Library/Logs/MongoDB/                             220 MB  1 yr    Green

DEAD LAUNCH AGENTS (binary missing)                   SIZE    AGE     RISK
──────────────────────────────────────────────────────────────────────────
▲ ~/Library/LaunchAgents/com.unknown.agent.plist      4 KB    2 mo    Yellow
▲ /Library/LaunchDaemons/io.somevendor.helper.plist   8 KB    5 mo    Yellow

UNKNOWN INSTALLS (no registered owner)                SIZE    AGE     RISK
──────────────────────────────────────────────────────────────────────────
⚠ ~/.local/bin/mcp-filesystem-server                  18 MB   3 wk    Red
⚠ ~/Library/Application Support/Claude/mcp_servers/  240 MB  1 mo    Red

UNUSED APPS (not launched in 90+ days)
──────────────────────────────────────
  GarageBand          8.1 GB  last used: 2024-01-12   via: App Store
  Xcode Simulator     6.4 GB  last used: never         via: Xcode
  Microsoft Excel     2.1 GB  last used: 2024-03-01   via: App Store

DEV CACHES (regenerable)                              SIZE
──────────────────────────────────────────────────────────
  Docker build cache                                  12.4 GB
  pnpm store                                           4.1 GB
  pip cache                                            1.8 GB
  Homebrew downloads                                   900 MB
```

### 5.3 `devclean uninstall <app>` — Total Uninstall

When a user uninstalls an app (e.g., `devclean uninstall zoom`):

1. Find the `.app` bundle → move to Trash or `brew uninstall --cask zoom`
2. Run the correlator to find ALL traces for Zoom
3. Show the complete list with sizes
4. Ask once: `Remove all 14 items? (1.8 GB) [y/N]`
5. Delete, write an **undo log** (`~/.devclean/undo/2026-05-24-zoom.json`)
6. Verify the LaunchAgent for Zoom (if any) is unloaded before deletion

### 5.4 `devclean apps --unused` — Usage Advisor

Reads `kMDItemLastUsedDate` via `mdls` for every installed app. Presents a table sorted by `size × days_unused`. Recommends uninstall for apps:
- Not launched in > 90 days
- AND size > 500 MB
- AND not a system app

User can hit `u` on any row to run a total uninstall immediately.

### 5.5 `devclean brew audit` — Homebrew Orphan Graph

```
brew uses --installed --eval-all <formula>
```

Finds formulae that:
- Were installed as a dependency of something
- The parent formula/cask has since been removed
- Nothing else depends on them

Shows a dependency tree and lets user uninstall the entire dead branch in one shot.

---

## 6. Safety System

Every operation goes through a 3-layer safety check:

```
Layer 1 — Blocklist
  Never touch: ~/Documents, ~/Desktop, ~/Downloads, ~/Pictures, ~/.ssh,
               ~/.gnupg, ~/.aws/credentials, any file modified < 24h ago

Layer 2 — Classification gate
  Green  → Can proceed after single confirmation
  Yellow → Shows warning, requires explicit "yes" typed
  Red    → Shows warning + reason, requires typing the item name to confirm

Layer 3 — Undo log
  Every deletion is logged to ~/.devclean/undo/<date>.json
  devclean undo (last N operations) restores from Trash or log
  Trash is used where possible (safe), hard delete only where necessary
```

---

## 7. Guardian — AI Agent & Silent Install Detection

This is the most novel feature. AI tools (Claude, Cursor, Copilot), MCP servers, and automation scripts install binaries, agents, and data in locations users never see.

**Guardian detects:**

| Signal | What it means |
|---|---|
| LaunchAgent plist added in last 30 days, binary unknown | Something installed a background process recently |
| Binary in `~/.local/bin` or `/usr/local/bin` with no brew/pip/cargo record | Installed by a script, not a package manager |
| MCP server entries in `~/.claude/`, `~/.cursor/`, VS Code settings | AI agent registered a server — is it still needed? |
| `~/Library/Application Support/<unknown>/` created by a non-App-Store app | Sandboxed-looking dir with no app |
| Network-capable binary (checked via `codesign -d` + entitlements) owned by no app | Potentially phones home |

**Guardian report:**

```
GUARDIAN REPORT — Unknown & AI-Installed Items

⚠ UNKNOWN BINARY
  ~/.local/bin/mcp-filesystem-server
  Installed: 2026-04-03  Size: 18 MB  Network entitlements: YES
  Not registered with any package manager
  Recommend: Review and remove if unused

⚠ MCP SERVER (registered in Claude Code)
  server: "filesystem" → path: ~/.local/bin/mcp-filesystem-server
  Last used by Claude: 2026-04-03
  Status: binary exists, but no recent usage
  Recommend: Remove from MCP config + delete binary

ℹ LAUNCH AGENT (recently added)
  ~/Library/LaunchAgents/com.cursor.ai.helper.plist
  Binary: /Applications/Cursor.app/... ✓ (app installed, looks legit)
  Status: Normal
```

---

## 8. Known App-Trace Dictionary (built-in)

The tool ships with a hardcoded map of ~200 popular apps → known trace paths. A subset:

```yaml
zoom:
  bundle_id: us.zoom.xos
  traces:
    - ~/Library/Application Support/zoom.us
    - ~/Library/Caches/us.zoom.xos
    - ~/Library/Logs/zoom.us
    - ~/Library/Preferences/us.zoom.xos.plist
    - ~/.zoom
    - ~/Library/LaunchAgents/us.zoom.ZoomDaemon.plist

mongodb-community:
  bundle_id: null  # CLI, not app
  brew_name: mongodb-community
  traces:
    - ~/Library/Logs/MongoDB
    - /usr/local/var/log/mongodb
    - /usr/local/var/mongodb
    - ~/.mongorc.js

docker:
  bundle_id: com.docker.docker
  traces:
    - ~/Library/Application Support/Docker Desktop
    - ~/Library/Containers/com.docker.docker
    - ~/Library/Group Containers/group.com.docker
    - ~/.docker
    - ~/Library/LaunchAgents/com.docker.helper.plist
```

This dictionary is versioned and can be updated via `devclean update-db`.

---

## 9. Tech Stack

| Component | Choice | Reason |
|---|---|---|
| Language | Go | Single binary, fast fs walk, easy brew formula |
| TUI | Bubble Tea (charmbracelet) | Best-in-class interactive terminal UI |
| CLI | Cobra + Viper | Standard Go CLI stack |
| Plist parsing | `howett.net/plist` | Pure Go, no macOS dep |
| macOS metadata | `mdls` via `os/exec` | Last-used date, app bundle info |
| Undo storage | JSON log + macOS Trash via AppleScript | Safe, reversible |
| App map DB | Embedded YAML (baked in at build) | Offline, fast |

---

## 10. MVP Scope (Build Order)

```
Phase 1 — Core skeleton
  ✦ App registry builder (Applications + brew)
  ✦ ~/Library orphan scanner
  ✦ Correlation engine
  ✦ CLI scan + report output

Phase 2 — Interactive cleaner
  ✦ Bubble Tea TUI
  ✦ Undo log + Trash integration
  ✦ devclean uninstall <app>

Phase 3 — Advisor + Brew
  ✦ Unused app advisor (mdls integration)
  ✦ brew audit orphan graph

Phase 4 — Guardian
  ✦ Unknown binary detection
  ✦ MCP server tracker
  ✦ LaunchAgent audit

Phase 5 — Polish
  ✦ devclean schedule (launchd)
  ✦ devclean report (JSON/MD export)
  ✦ update-db (fetch latest app-trace dictionary)
  ✦ brew formula + install script
```

---

## 11. Non-Functional Requirements

| Requirement | Target |
|---|---|
| Scan speed | Full scan completes in < 30 seconds on a typical dev machine |
| Safety | Zero data loss from Green-category operations |
| No root required | All operations run as the current user |
| Offline first | Core functionality works without internet; advisory feeds are optional |
| Idempotent | Running twice in a row produces no error and no double-deletion |
| Output | Plain text + optional color (respects `NO_COLOR`) |
| Config file | `~/.devcleanrc` (TOML) for custom paths, exclusions, schedule |

---

**Bottom line:** The core idea is *prove ownership before trusting a file*. Every item on this machine either has a living owner or it doesn't — and if it doesn't, it should earn its stay. The Guardian module makes this a security tool, not just a cleaner, which is increasingly important as AI tools silently populate `~/.local/bin` and register background agents without users noticing.

# <img src="skwad-icon.svg" width="48" height="48" alt="Skwad icon"> Skwad

A frequency coordinator for FPV drone pilots. When multiple pilots fly together, everyone needs to be on a different video channel to avoid interference. Skwad handles the channel math so pilots can scan a QR code, enter their gear info, and get told which channel to use.

## The Problem

FPV video transmitters share the 5.8 GHz band, but different systems (analog, DJI, HDZero, Walksnail) have different channel tables with different center frequencies and signal widths. A DJI O3 running at 40 MHz bandwidth takes up twice the spectrum of a 20 MHz analog transmitter. You can't just "stay two channels apart" because the channels aren't evenly spaced and the signals aren't the same width.

## Running It

### Docker

```sh
docker build -t skwad .
docker run -p 8080:8080 -v skwad-data:/data skwad
```

### From Source

Requires Go 1.24+.

```sh
go build -o skwad .
DB_PATH=./skwad.db ./skwad
```

The server starts on port 8080 by default. Set `PORT`, `DB_PATH`, and `STATIC_DIR` environment variables to override defaults.

## Supported Video Systems

| System | Channels | Bandwidth | Notes |
|--------|----------|-----------|-------|
| **Analog 5.8 GHz** | Up to 32 across 4 bands | 20 MHz | Pilot selects bands: R (Race), F (Fatshark), E (Boscam E), L (Low Race) |
| **HDZero** | R1-R8 (Race Band) | 20 MHz | Same frequencies as analog Race Band |
| **DJI V1 / Vista** | 4 stock, 8 FCC | 20 MHz | Different center frequencies than Race Band |
| **DJI O3** | 3 stock, 7 FCC (20 MHz); 3 FCC at 40 MHz | 20/40 MHz | |
| **DJI O4 / O4 Pro** | 3 stock, 7 FCC (20 MHz); 1-3 at 40 MHz; 1 at 60 MHz | 20/40/60 MHz | Race Mode (Goggles 3/N3) uses Race Band |
| **Walksnail Avatar** | Standard (same as DJI V1) or Race Mode (Race Band) | 20 MHz | FCC unlock applies to standard mode |
| **OpenIPC** | WiFi-165 | 20 MHz | Single channel at 5825 MHz |

### Analog Bands

Analog pilots select which VTX bands they have available. Race Band is the default; additional bands expand the channel pool for the optimizer:

| Band | Channels | Range |
|------|----------|-------|
| **R** - Race Band | R1-R8 | 5658-5917 MHz |
| **F** - Fatshark | F1-F8 | 5740-5880 MHz |
| **E** - Boscam E | E1-E8 | 5645-5945 MHz |
| **L** - Low Race | L1-L8 | 5362-5621 MHz |

When multiple bands are selected, the optimizer merges them with frequency deduplication (R7 and F8 share 5880 MHz). See [frequency-reference.md](frequency-reference.md) for the complete frequency tables.

Available channels depend on the pilot's settings: FCC unlock status, which goggles they use (for DJI O4 Race Mode), bandwidth setting, and selected analog bands. See [fpv-optimizer.md](fpv-optimizer.md) for the optimization logic.

## How Spacing Works

The optimizer doesn't use a single fixed spacing number. It calculates the required separation for each pair of pilots based on their actual signal widths and the session's power ceiling.

**Occupied bandwidth** is how wide the signal actually is:
- Analog, HDZero, DJI V1, Walksnail: **20 MHz**
- DJI O3/O4 at 20 MHz setting: **20 MHz**
- DJI O3/O4 at 40 MHz setting: **40 MHz**
- DJI O4 at 60 MHz setting: **60 MHz**

**Required spacing** between two pilots:

```
(pilot A bandwidth / 2) + (pilot B bandwidth / 2) + guard band
```

The guard band depends on the session's power ceiling. Higher TX power means wider guard bands:

| Power Ceiling | Guard Band |
|---------------|------------|
| No ceiling (default) | 10 MHz |
| 25 mW | 10 MHz |
| 100 mW | 12 MHz |
| 200 mW | 14 MHz |
| 400 mW | 16 MHz |
| 600 mW | 24 MHz |
| 800 mW | 28 MHz |
| 1000 mW | 32 MHz |

Examples (at default 10 MHz guard band):
- Two analog pilots (20 + 20): need **30 MHz** center-to-center
- Analog next to DJI O3 at 40 MHz (20 + 40): need **40 MHz** center-to-center
- Two DJI O4 at 40 MHz (40 + 40): need **50 MHz** center-to-center
- DJI O4 at 60 MHz next to analog (60 + 20): need **50 MHz** center-to-center

At 600 mW, those same pairs need 44, 54, 64, and 64 MHz respectively, which reduces usable raceband channels from 8 to 4.

## How the Optimizer Works

The optimizer uses **graduated escalation** -- it tries the least disruptive solution first, and only escalates if it has to. This runs every time a pilot joins or changes their channel/video system.

### Level 0: Slot in without moving anyone

Lock all existing pilots in place and try to assign the new pilot to a conflict-free channel. If one exists, done -- nobody moves.

### Level 1: Buddy or partial rebalance (pilot's choice)

If no clean slot exists, the app builds two options for the pilot to choose from:

- **Buddy option**: Share a frequency with the most compatible existing pilot. Buddy groups can still fly but need to take turns or accept interference. The UI highlights buddy groups with matching colored borders and "SHARING WITH" labels.
- **Rebalance option**: Unlock one flexible pilot (or pair), find a clean arrangement by moving the minimum number of pilots.

The pilot picks which option they prefer.

### Full rebalance (leader only, separate action)

The session leader can trigger a full reoptimize that unlocks all flexible pilots and reassigns from scratch. This uses a two-phase approach: surgical pass first (pin non-conflicting pilots, re-optimize only conflicted ones), full re-optimize fallback. The leader can also adjust or remove the power ceiling during rebalance.

### Core optimizer (used by all levels)

Pinned pilots (set by `OptimizeWithLocks`, never from the database) are placed first. Remaining pilots are sorted most-constrained-first and assigned to the channel with the best margin. Pilots with a preferred channel get it when the margin is non-negative. If a pilot was already on a channel and it still has non-negative margin, they stay put.

**No rebalance on pilot leave** -- when a pilot leaves, remaining pilots keep their current channels. This prevents unexpected channel shuffles mid-session.

### Fixed Channels

The session leader can optionally select a preset channel set (2-5 channels) during session creation. The optimizer constrains all assignments to the fixed set. When all channels are occupied, overflow pilots buddy up on the least-loaded channel. Presets include analog-only, DJI-only, and mixed sets optimized for spacing and IMD.

### Channel Preferences

Pilots choose AUTO-ASSIGN or "I HAVE A PREFERENCE" when joining. Preferences are soft -- the optimizer honors them when possible but can override them for session quality. `PreferredFreqMHz` in the database (0 = auto-assign).

## Conflict Detection

After optimization, Skwad checks every pair of pilots for conflicts:

- **Danger** (red): Signals actually overlap -- center-to-center separation is less than the sum of half-bandwidths. Definite interference.
- **Warning** (amber): Separation is less than the required spacing but signals don't overlap. Interference is likely, especially with reflections and multipath.

Conflicts appear on pilot cards showing actual separation vs. required (e.g., "OVERLAP WAYNE (26/40 MHz)").

## IMD Scoring

Third-order intermodulation products (F3 = 2*F1 - F2) are calculated for all pilot pairs and scored using quadratic proximity weighting -- products closer to active channels penalize more heavily. The session header shows an aggregate IMD score (0-100, green/amber/red). IMD products appear as tick marks on the spectrum visualization. Affected pilots get an "IMD" flag with source attribution (tap to see which two pilots create the interference).

IMD is informational -- the optimizer does not use IMD in its channel assignments.

## User Workflows

### Starting a Session

1. One pilot taps **START SESSION** -- gets a 6-character code
2. They see a "YOU'RE THE LEADER" info screen with optional session settings:
   - **Power Ceiling**: Set a TX power limit for the session
   - **Fixed Channels**: Pick a preset channel set
3. They share the code or QR code with the group
4. Other pilots scan the QR or enter the code to join

### Joining a Session

1. Enter your callsign
2. If the session has a power ceiling, see the limit before proceeding
3. Pick your video system (Analog, DJI V1, DJI O3, DJI O4, HDZero, Walksnail, OpenIPC)
4. Answer follow-up questions based on your system:
   - **Analog:** Which bands does your VTX support? (R, F, E, L -- Race Band pre-selected)
   - FCC unlocked? (DJI V1, O3, O4, Walksnail Standard)
   - Which goggles? (DJI O4)
   - Bandwidth? (DJI O3, O4) -- with recommended/warning indicators when a power ceiling is set
   - Race Mode? (DJI O4 with Goggles 3/N3, Walksnail)
5. Choose channel preference: **AUTO-ASSIGN** (recommended) or **I HAVE A PREFERENCE**
6. Hit JOIN -- you get your optimized channel assignment

The first pilot to join becomes the **session leader** (see below).

### Level 1 Conflict Resolution

If joining or changing channels can't find a clean slot (Level 1), the pilot gets two options:

- **Buddy up** with a compatible pilot on a shared frequency
- **Partial rebalance** -- move one or two flexible pilots to open a clean slot

The pilot picks which approach they prefer. If they cancel, nothing changes.

### Channel Change Notification

If someone else's join or a rebalance moves your channel, you see a dialog showing the change so you can update your VTX.

### Session Leader

The first pilot to join a session becomes the leader. Leaders have additional controls:

- **Add pilots** -- add a phantom pilot with any video system (useful for reserving a channel)
- **Remove pilots** -- slide-to-confirm on another pilot's card
- **Change others' channels** -- tap another pilot's card to reassign (force placement, bypasses escalation)
- **Rebalance all** -- full optimizer rebalance with before/after spectrum preview; optionally adjust the power ceiling
- **Transfer leadership** -- hand off the leader role to another pilot

Leadership is explicit handoff only -- there's no auto-succession or heartbeat.

### Managing Your Session

- **Tap your own card** to change channel, change video system, change callsign, or leave
- **Tap another pilot's card** (leader only) to remove, change channel, or transfer leadership
- **Tap the session code** for a fullscreen QR code

Sessions expire after **12 hours**.

### Spectrum Visualization

The session footer shows a spectrum visualization of the 5.8 GHz band. Each pilot appears as a bell-curve waveform whose width represents their occupied bandwidth. Colors indicate status: green (you), red (danger), orange (warning), gray (clear). Red tick marks show IMD products, with diamond markers when they land on an active pilot's channel. The frequency range dynamically expands when pilots use Low Race band (down to ~5350 MHz) or upper Boscam E channels (up to ~5960 MHz).

### Live Updates

Clients poll for changes every 5 seconds. Any pilot joining, leaving, or changing channels automatically updates all connected clients.

## Project Structure

```
skwad/
  main.go              # HTTP server and routing
  freq/
    tables.go          # Channel tables, guard band mapping, occupied bandwidth
    optimizer.go       # Frequency assignment and graduated escalation
    *_test.go          # Optimizer and table tests
  api/
    handlers.go        # API endpoint handlers
    handlers_test.go   # Handler tests
  db/
    db.go              # SQLite database layer
    db_test.go         # Database tests
  static/
    index.html         # Single-page app (landing + session UI)
    app.js             # Client-side logic
    style.css          # Styles
    sw.js              # Service worker (network-first caching)
    jsqr.min.js        # QR code scanner fallback library
    changelog.html     # User-facing release notes
    freq-guide.html    # Interactive frequency guide
  fpv-optimizer.md     # Optimizer design doc
  frequency-reference.md  # Complete channel/frequency tables
  docs/
    channel-set-analysis.md  # Fixed channel set spacing analysis
```

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/sessions` | Create session (accepts `power_ceiling_mw`, `fixed_channels`) |
| `GET` | `/api/sessions/{code}` | Get session state (includes `leader_pilot_id`, `rebalance_recommended`) |
| `POST` | `/api/sessions/{code}/join` | Join session (graduated escalation) |
| `POST` | `/api/sessions/{code}/preview-join` | Preview join -- returns level, assignment, displaced, buddy/rebalance options |
| `GET` | `/api/sessions/{code}/poll` | Long-poll for version changes |
| `POST` | `/api/sessions/{code}/preview-rebalance` | Preview rebalance (dry-run) |
| `POST` | `/api/sessions/{code}/rebalance` | Full reoptimize (leader only) |
| `POST` | `/api/sessions/{code}/transfer-leader` | Transfer leadership (leader only) |
| `POST` | `/api/sessions/{code}/add-pilot` | Add phantom pilot (leader only) |
| `POST` | `/api/pilots/{id}/preview-channel?session=CODE` | Preview channel change (dry-run) |
| `PUT` | `/api/pilots/{id}/channel?session=CODE` | Change channel (graduated escalation) |
| `PUT` | `/api/pilots/{id}/video-system?session=CODE` | Change video system |
| `PUT` | `/api/pilots/{id}/callsign?session=CODE` | Change callsign |
| `DELETE` | `/api/pilots/{id}?session=CODE` | Remove pilot (leader only for others, self always allowed) |

Leader-only endpoints check the `X-Pilot-ID` request header against the session's `leader_pilot_id`.

## Tech Stack

- **Backend:** Go 1.24, net/http (stdlib router), SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- **Frontend:** Vanilla HTML/CSS/JS, no build step, installable as a PWA
- **Database:** SQLite with WAL mode

## License

Apache License 2.0. See [LICENSE](LICENSE).

# Changelog

All notable changes to Skwad are documented in this file.

> **Note:** User-facing release notes are maintained separately in `static/changelog.html`. Keep both in sync — developer details here, plain-language descriptions there.

## [0.3.0] - 2026-03-10

### Changed
- **Channel preferences replace locks.** Pilots set a preferred channel (or auto-assign). Preferences are soft signals -- the optimizer honors them when possible but can override for session quality.
- **Simplified escalation.** Two levels instead of four: Level 0 (clean placement) and Level 1 (pilot chooses buddy-up or partial rebalance).
- **All pilots are rebalanceable.** No permanent locks. Leader's Rebalance All can move anyone, respecting preferences as weights.
- **Rebalance uses two-phase approach.** Surgical pass first (only move conflicted pilots), full re-optimize fallback.

### Added
- **Preference override dialog.** When a preference can't be honored, pilot sees why and where they landed (GOT IT button).
- **Buddy/rebalance choice dialog.** When no clean channel is available, pilot picks: buddy up with someone, or partial rebalance (move the most flexible pilot).
- **AUTO-ASSIGN NEW button.** Self-service channel change option that picks a different channel.
- **Moved-by-rebalance notification.** When a partial rebalance moves you, you see a "You've been moved" dialog with GOT IT button on your next poll.
- **Rebalance recommended indicator.** Subtle nudge on leader's screen when conflicts exist.
- **Force placement for leaders.** Leader can place a pilot on a conflicting channel (buddy-up or overlap accepted).
- `preferred_frequency_mhz` column in pilots table.
- `rebalance_recommended` flag in GET session response.
- `force` flag on channel change request (leader-only).

### Removed
- `channel_locked` and `locked_frequency_mhz` no longer used (columns kept for SQLite compatibility).
- Level 2 (pair unlocking) and Level 3 (buddy-only) escalation -- replaced by Level 1 choice dialog.

## [0.2.1] - 2026-03-09

### Added
- **Expanded analog bands** — analog pilots can now select from 4 VTX bands:
  - R (Race Band) — 8 channels, 5658–5917 MHz (existing, still the default)
  - F (Fatshark) — 8 channels, 5740–5880 MHz
  - E (Boscam E) — 8 channels, 5645–5945 MHz
  - L (Low Race) — 8 channels, 5362–5621 MHz
- `FatsharkBand`, `BoscamEBand`, `LowRaceBand` channel tables in `freq/tables.go`
- `AnalogBandMap` lookup and `MergeAnalogBands()` — unions selected bands with frequency deduplication
- `ChannelPool()` gains `analogBands []string` parameter; `PilotInput.AnalogBands` field
- `analog_bands TEXT DEFAULT 'R'` column on pilots table with idempotent migration
- `joinBands()`/`splitBands()` helpers in API handlers for string↔slice conversion
- Band selector UI in join wizard follow-up step (4 toggle buttons, R pre-selected)
- "NOT SURE? JUST USE RACE BAND" helper text for uncertain users
- Band selector in leader's add-pilot dialog
- Frontend `mergeAnalogBands()`, `ANALOG_BAND_MAP`, fatshark/boscam_e/lowrace in `CHANNELS`
- Dynamic spectrum visualization range — expands when pilots use Low Race or upper Boscam E

### Changed
- Analog separated from HDZero in `ChannelPool()` switch (was shared case)
- `analog` removed from `SIMPLE_SYSTEMS` and join wizard no-followup skip list
- Service worker cache bumped to `skwad-v6`

## [0.2.0] - 2026-03-08

### Added
- **Stability-first optimizer** with graduated escalation (`FindMinimalDisplacement`):
  - Level 0: Lock all existing pilots, slot new pilot into best available channel
  - Level 1: Unlock one flexible pilot at a time, pick solution with best worst-case margin
  - Level 2: Unlock pairs of flexible pilots
  - Level 3: Buddy suggestion — find most compatible pilot to share a frequency with
- `OptimizeWithLocks(pilots, lockedIDs)` — wraps `Optimize()` with forced channel locks
- **Session leader** role (`leader_pilot_id` column on sessions table):
  - First pilot to join becomes leader
  - `POST /api/sessions/{code}/rebalance` — leader-only full reoptimize, returns moved pilots
  - `POST /api/sessions/{code}/transfer-leader` — hand off leadership
  - `POST /api/sessions/{code}/add-pilot` — leader adds phantom pilot with video system options
  - `DELETE /api/pilots/{id}` — leader-only for removing others; self-removal always allowed
- Authorization via `X-Pilot-ID` request header, checked against `leader_pilot_id`
- Leader UI: badge, rebalance-all with confirmation + result dialog, add-pilot with follow-up options (FCC/bandwidth for DJI/Walksnail), change-channel for other pilots, transfer leadership, slide-to-remove (leader only), leader-leave handoff prompt
- Buddy suggestion dialog for Level 3 (join and channel change flows)
- `hasDangerInvolving()` — checks danger conflicts only for moved pilots, ignores pre-existing conflicts
- `flexiblePilots()`, `worstMargin()`, `copyAssignments()` optimizer helpers
- Idempotent schema migration via `ALTER TABLE ... ADD COLUMN` with duplicate-column error handling
- `static/changelog.html` — self-hosted user-facing release notes page
- "What's New" link in landing page footer

### Changed
- `HandleJoinSession` uses `FindMinimalDisplacement` instead of `reoptimize()`
- `HandlePreviewJoin` returns `{level, assignment, displaced, buddy_suggestion}` (was `{displaced, has_danger}`)
- `HandleUpdatePilotChannel` uses graduated escalation, removed `?rebalance` query parameter
- `HandlePreviewChannelChange` rewritten with graduated escalation and new response shape
- `HandleUpdatePilotVideoSystem` uses graduated escalation instead of `reoptimize()`
- `HandleDeactivatePilot` no longer calls `reoptimize()` — remaining pilots keep their channels
- `HandleRebalanceAll` returns JSON `{moved: [...]}` instead of 204 No Content
- Displacement preview UI: single "JOIN" + "CANCEL" buttons (was "MOVE EVERYONE" / "JUST MOVE ME" / "CANCEL")
- Non-leaders cannot tap other pilot cards (no action sheet)
- Service worker cache bumped to `skwad-v5`
- `buildPilotInputs()` helper DRYs up `db.Pilot` → `freq.PilotInput` conversion

### Removed
- `reoptimizeForPilot()` — replaced by `FindMinimalDisplacement` Level 0
- `?rebalance` query parameter on join and channel-change endpoints
- `has_danger` field from preview API responses

## [0.1.0] - 2026-03-03

Initial release of the Skwad FPV frequency coordinator.

### Added
- Session creation with 6-character hex codes (`crypto/rand`) and 24-hour expiry
- Collision retry (up to 5 attempts) on session ID generation
- Setup wizard: callsign, video system, FCC unlock, goggles, bandwidth, race mode, channel preference
- Frequency optimizer: greedy single-pass, most-constrained-first, stability tie-breaker on `PrevFreqMHz`
- Channel tables for Analog, HDZero, DJI V1/O3/O4, Walksnail (standard + race), OpenIPC
- Bandwidth-aware spacing: `RequiredSpacing(bwA, bwB) = bwA/2 + bwB/2 + 10 MHz guard`
- Conflict detection: danger (signal overlap) and warning (guard band violation) levels
- Buddy group identification for shared frequencies
- REST API: CRUD for sessions and pilots, preview endpoints for dry-run optimization
- Spectrum visualization with bell-curve waveforms, devicePixelRatio scaling, label staggering
- Real-time updates via version polling (`GET /api/sessions/{code}/poll`)
- QR code generation (built-in alphanumeric/byte mode encoder, Version 1-6)
- QR code scanner: native `BarcodeDetector` with `jsQR` fallback
- Recent sessions in localStorage with validation on app load
- PWA: service worker with network-first strategy, precaching, install prompt
- Hourly background goroutine for `DeleteExpiredSessions()`
- Callsign change in-session, video system change via leave-and-rejoin
- Docker container with multi-stage build, Traefik integration
- CORS middleware for API endpoints

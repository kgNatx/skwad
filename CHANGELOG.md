# Changelog

All notable changes to Skwad are documented in this file.

> **Note:** User-facing release notes are maintained separately in `static/changelog.html`. Keep both in sync — developer details here, plain-language descriptions there.

## [0.6.0] - 2026-03-15

### Added
- **Fixed channels.** Session leaders can select a preset channel set (2-5 unique channels) during session creation. The optimizer constrains all assignments to the fixed set, buddying up overflow pilots. Presets include analog-only, DJI-only, and mixed sets optimized for spacing and IMD.
- **Session options on leader info.** The "YOU'RE THE LEADER" screen now shows optional checkboxes for Power Ceiling and Fixed Channels. Only checked options appear in the wizard — unchecked options are skipped for a faster setup.
- **Per-system channel availability.** Fixed channel set cards show how many channels are usable per system type (e.g., "2 RACEBAND · 4 DJI"), so leaders understand the trade-offs of mixed sets.
- **Channel picker pilot counts.** In fixed-channel sessions, the channel picker shows how many pilots are already on each channel (e.g., "R1 5658 (2)") to help pilots choose where to buddy up.
- **Add-pilot fixed channel hint.** The leader's add-pilot dialog shows the active fixed channel set.
- **Joiner channel restriction.** Pilots joining a fixed-channel session see which channels are available and can only pick from the fixed set. Channel change pickers are also restricted.
- **Fixed channels badge.** Session header shows "FIXED · N CH" badge when fixed channels are active.
- **IMD proximity-weighted scoring.** IMD score now uses quadratic proximity weighting (inspired by ET's IMD Tools). Products closer to active channels penalize more heavily. More meaningful 0-100 scale.
- **Pilot card layout.** Bottom row (IMD flag, conflict/buddy text, leader dot) is now a zero-height overlay — no extra vertical space consumed.

### Changed
- **Session creation flow.** Power ceiling step is now optional (only shown when the leader checks the Power Ceiling option). Sessions without options checked go straight from leader info to video system selection.
- **`CreateSession` accepts fixed_channels.** `POST /api/sessions` now accepts an optional `fixed_channels` JSON string.
- **Optimizer accepts fixed frequency constraint.** `Optimize()`, `OptimizeWithLocks()`, and `FindMinimalDisplacement()` accept a `fixedFreqs []int` parameter. When set, pilot channel pools are filtered to only include frequencies in the fixed set.
- **IMD badge placement.** Moved to same line as pilot count in session header.

### Fixed
- **Buddy preference in fixed sessions.** Optimizer now honors pilot channel preference even when all channels are occupied (buddying unavoidable). Previously ignored preference and picked least-loaded channel.
- **Even buddy distribution.** When all channels are occupied, optimizer prefers the least-loaded channel instead of picking by margin alone.
- **Creator channel filter.** `filterPoolToFixedChannels` now checks `state.fixedChannels` (creator flow) in addition to `state.sessionFixedChannels` (joiner flow). Fixes channel picker not being filtered for the session creator.
- **Video system change in fixed sessions.** Optimizer preserves previous frequency when changing video systems in a fixed-channel session, keeping the pilot on their channel instead of reassigning.
- **Channel picker in full fixed sessions.** Self-service channel change now shows all fixed channels (with pilot counts) instead of hiding occupied channels and showing an empty picker.

## [0.5.1] - 2026-03-14

### Added
- **IMD source attribution.** Tapping a pilot with an IMD flag shows which two pilots are creating the interference. Visible to all users, not just the leader.
- **IMD preview on channel picker.** When selecting a channel (join, self-service change, or leader moving a pilot), the spectrum preview now shows IMD products for the hypothetical assignment, helping pilots pick IMD-clean channels.

### Fixed
- **O3 FCC 40 MHz frontend channel table.** Frontend channel picker now shows 3 channels for O3 FCC 40 MHz, matching the backend. Previously only showed 1 channel.
- **IMD index mapping.** Fixed `calcIMDProducts` to use original pilot array indices instead of filtered array indices. Prevents wrong pilot being flagged if any pilot has no assigned frequency.

## [0.5.0] - 2026-03-14

### Added
- **IMD visibility (informational only).** Third-order intermodulation products are now calculated and displayed. IMD score badge in session header (green ≥80, amber ≥50, red <50). Red tick marks on the spectrum canvas show where IMD products land, with diamond markers when they hit an active pilot's channel. Affected pilots get an "IMD" flag on their row. This is purely informational — the optimizer does not use IMD in its channel assignments.
- **Power step redesign.** Hero stats layout shows mW and guard band side by side above the slider. Spectrum preview bar visualizes occupied bandwidth and guard band. Removed dBm and channel count displays. Bigger slider labels. Added "Guidance only" disclaimer.
- **Rebalance power slider.** Leaders can adjust (or remove) the power ceiling during rebalance. Slider includes a NO LIMIT position. Preview updates when the slider is released. Backend accepts optional `power_ceiling_mw` on rebalance and preview-rebalance endpoints.
- **Freq guide page.** Full frequency reference published as a styled HTML page at `/freq-guide.html`, linked from the landing page footer.

### Fixed
- **O3 FCC 40 MHz channels.** O3 now correctly returns 3 channels (5735, 5795, 5855) in FCC mode at 40 MHz, matching O4. Previously only returned 1 channel regardless of FCC status.
- **DJI bandwidth hints for session creators.** `shouldWarnBandwidth()` now checks `state.powerCeilingMW` in addition to `state.sessionPowerCeiling`, so bandwidth recommended/warning indicators appear for the leader during session creation, not just for joiners.
- **IMD 3-pilot set corrected** in frequency reference. R1/R4/R7 was not IMD-free (2×R4−R1 = R7). Changed to R1/R4/R8.
- **Guard band formula clarification.** Documentation now notes the formula approximates 25–200 mW only; 600+ mW values are calibrated overrides.
- **DJI EIRP corrected.** Documentation updated to 33 dBm (~2W) for O3/O4 Pro, 30 dBm (~1W) for standard O4.

## [0.4.1] - 2026-03-14

### Added
- **DJI bandwidth guidance.** Power ceiling interstitial now includes a tip for DJI pilots to use 20 MHz bandwidth for best channel compatibility (shown when ceiling < 600 mW).
- **Bandwidth button indicators.** DJI O3/O4 bandwidth buttons show "RECOMMENDED" on 10/20 MHz and amber warning on 40/60 MHz when a power ceiling is set below 600 mW. Applies in join wizard, video system change, and leader add-pilot dialog.
- **DJI Dynamic Power Control section** in `frequency-reference.md` documenting DJI's automatic power behavior, bandwidth impact on spacing, and practical group flying experience.

## [0.4.0] - 2026-03-14

### Added
- **Power ceiling.** Session leaders can set a TX power ceiling during session creation. Higher power widens the optimizer's guard band, reducing available unique channels but improving signal quality. Calibrated against raceband channel spacing — see `frequency-reference.md`.
- **Joiner power alert.** Pilots joining a session with a power ceiling see an interstitial alert showing the limit before picking their video system.
- **Session power badge.** Active sessions with a power ceiling show an amber "X mW MAX" badge in the header.

### Changed
- **Guard band is now configurable.** `RequiredSpacing()` and all optimizer functions accept a `guardBandMHz` parameter instead of using the hardcoded `DefaultGuardBandMHz` constant. Default behavior (10 MHz) is preserved when no power ceiling is set.
- **`CreateSession` accepts power ceiling.** `POST /api/sessions` now accepts an optional `power_ceiling_mw` field.
- **Session creation deferred.** Session is now created after the leader completes the power ceiling step (or skips it), not when they click START SESSION.
- **Session struct JSON tags.** All Session fields now use explicit lowercase JSON tags (`id`, `version`, `leader_pilot_id`, `power_ceiling_mw`).

## [0.3.3] - 2026-03-14

### Fixed
- **Session code O/0 confusion.** Typing "O" in the join code input was silently stripped (hex only allows 0-9, A-F). Now auto-corrects O→0 and I→1 instead of eating the character.
- **Join vs create ambiguity.** Joining a session by code showed the same callsign screen as creating a new session with no indication you were joining. Now shows "JOINING SESSION XXXXXX" above the callsign prompt.

## [0.3.2] - 2026-03-13

### Fixed
- **Preference picker not showing in join wizard.** Clicking "I HAVE A PREFERENCE" showed the hint text but not the channel grid or spectrum — `classList.add('hidden')` (`display: none !important`) was being "shown" with `style.display = ''`, which can't override `!important`. Switched to consistent `classList.remove('hidden')`.
- **Join without selecting a preference.** JOIN button was always enabled on the channel preference step, allowing pilots to submit with `preferredFreqMHz: 0` (auto-assign) after explicitly choosing "I HAVE A PREFERENCE". JOIN is now disabled until a channel is selected.

## [0.3.1] - 2026-03-11

### Fixed
- **Video system change no longer kicks you out.** Changing your video system now updates in place instead of deleting and re-adding the pilot. Cancel returns you to the session with your previous settings intact.
- **Add-pilot FCC buttons.** Leader's add-pilot dialog now uses explicit YES/NO buttons for FCC unlock instead of a single toggle, matching the join wizard.

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
- **Rebalance preview spectrums.** REBALANCE ALL confirmation now shows before/after spectrum visualizations so the leader can see what will change.
- `POST /api/sessions/{code}/preview-rebalance` — dry-run optimizer endpoint, returns proposed assignments without committing.
- `preferred_frequency_mhz` column in pilots table.
- `rebalance_recommended` flag in GET session response.
- `force` flag on channel change request (leader-only).

### Fixed
- **Leader force-move simplified.** `submitChannelChangeForPilot` always sends `force: true` and commits directly — no more preview/override/choice dialogs that were meant for the pilot, not the leader.
- **Leader force-move to free channel.** Without `force: true`, the optimizer was treating leader's explicit choice as a soft preference and overriding it.
- **Buddy confirmation z-index.** Channel-change sheet now hides before showing buddy confirmation, preventing it from covering the dialog.
- **Cancel from buddy confirmation.** Returns to channel picker instead of dead end.
- **DJI O4 wizard step ordering.** Goggles step now appears before bandwidth step in HTML, matching the actual O4 flow (FCC → Goggles → Bandwidth → Race Mode).
- **Callsign change cancel button.** Fixed duplicate `btn-callsign-cancel` ID — rename to `btn-callsign-change-cancel`.

### Changed
- **Action sheets tightened.** Padding 36px→20px, gap 18px→12px, title 26px→22px for more usable space.
- **Full-screen channel picker.** Channel change sheet uses 100dvh with no border radius.
- **Adaptive channel grid.** `adaptPickerGrid()` sets grid columns dynamically (≤3 channels = N cols, 4+ = 4 cols).
- Non-leader channel picker filters out conflicting channels instead of showing them grayed.
- `white-space: nowrap` on system badges, channel names, and leader buttons to prevent wrapping.
- Leader button padding and letter-spacing reduced to prevent overflow on narrow screens.
- **Session expiry reduced from 24 hours to 12 hours.** `CreateSession()` now uses `12 * time.Hour`.
- **Shrink-to-fit callsigns.** `fitText()` scales long callsigns down (28px→14px min) instead of wrapping.
- **Landing page leader hint.** Explains session leader role and transfer-leadership reminder under START SESSION.
- **Spectrum canvases 10% shorter.** Main 120→108px, action sheets 90→80px. Renderer reads height from CSS.

### Removed
- `channel_locked` and `locked_frequency_mhz` no longer used (columns kept for SQLite compatibility).
- Level 2 (pair unlocking) and Level 3 (buddy-only) escalation -- replaced by Level 1 choice dialog.
- Preview/override/choice dialog flow from leader force-move path (was `submitChannelChangeForPilot`, ~35 lines → 5 lines).

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
- Session creation with 6-character hex codes (`crypto/rand`) and 12-hour expiry
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

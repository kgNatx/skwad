# Changelog

All notable changes to Skwad are documented in this file.

## [0.2.0] - 2026-03-08

### Added
- **Stability-first optimizer**: New pilots are placed without moving anyone when possible. If movement is needed, the system tries the least disruptive option first (move 1 pilot, then pairs) before suggesting frequency sharing as a last resort.
- **Session leader**: The first pilot to join becomes the session leader. The leader can:
  - Remove other pilots from the session
  - Manually add pilots who don't have a phone
  - Change another pilot's channel assignment
  - Trigger a full rebalance of all channel assignments
  - Transfer leadership to another pilot
- **Leader leave prompt**: Leaders are prompted to hand off leadership before leaving.
- **Buddy suggestion**: When no clear channel is available, the system suggests sharing a frequency with the most compatible pilot.
- **Leader badge**: The leader's name is tagged with a "LEADER" badge in the pilot list.

### Changed
- Joining a session no longer moves existing pilots unless necessary.
- Leaving a session no longer triggers rebalancing of remaining pilots.
- Channel and video system changes use the same graduated escalation as joins.
- Displacement preview simplified to "JOIN" or "CANCEL" (removed "MOVE EVERYONE" / "JUST MOVE ME" options — the system now finds the minimal displacement automatically).
- Only the session leader can remove other pilots. Non-leaders can only leave.
- Full rebalance is now an explicit leader action ("REBALANCE ALL" button), not the default on every join.

### Removed
- `?rebalance` query parameter on join and channel-change endpoints.
- `reoptimizeForPilot()` server function (replaced by graduated escalation).
- `has_danger` field from preview API responses (replaced by escalation levels).

## [0.1.0] - 2026-03-03

Initial release of the Skwad FPV frequency coordinator.

### Added
- Session creation with 6-character hex codes and 24-hour expiry.
- Setup wizard: callsign, video system, FCC unlock, goggles, bandwidth, race mode, channel preference.
- Frequency optimizer with bandwidth-aware spacing and buddy group detection.
- Support for Analog, HDZero, DJI V1/O3/O4, Walksnail (standard + race), and OpenIPC.
- Spectrum visualization with bell-curve waveforms and conflict indicators.
- Conflict detection at warning (guard band) and danger (signal overlap) levels.
- Displacement preview before joining or changing channels.
- Real-time session updates via version polling.
- QR code generation for session sharing.
- QR code scanner using native BarcodeDetector with jsQR fallback.
- Recent sessions saved to localStorage.
- PWA support with service worker, install prompt, and offline caching.
- Automatic cleanup of expired sessions via background goroutine.
- Callsign and channel changes in-session.
- Video system change via leave-and-rejoin flow.

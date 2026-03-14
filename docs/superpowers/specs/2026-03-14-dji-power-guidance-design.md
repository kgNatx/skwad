# DJI Power Ceiling Guidance

**Date:** 2026-03-14
**Status:** Approved
**Scope:** Frontend-only changes + documentation

## Problem

DJI O3/O4 pilots use dynamic power control — the system automatically adjusts TX power based on link quality. Pilots have no manual mW setting. When a session leader sets a power ceiling (e.g., 200 mW), the current interstitial tells all pilots "set your VTX to 200 mW or below," which is advice DJI pilots literally cannot follow.

Two problems:
1. **The joiner alert is useless for DJI pilots** — they can't manually set power
2. **Should the optimizer treat DJI differently?** — knowing their power is dynamic and uncontrollable

## Decision

**Approach 1: Guidance-only.** No optimizer changes. DJI pilots get better messaging and visual cues on bandwidth selection. The optimizer continues to treat all video systems identically for guard band purposes.

**Why not optimizer differentiation?** In practice, DJI at typical meetup distances (~100m line-of-sight) runs at relatively low power (estimated under 200 mW). Groups of 6-8 mixed DJI/analog pilots fit comfortably on raceband with 20 MHz bandwidth guidance. Penalizing DJI in the optimizer would solve a theoretical problem at the cost of reducing channel availability.

**Why not enforce bandwidth?** Skwad is a coordinator, not a controller. It's an informational dashboard — pilots make their own choices. There may be legitimate reasons to use 40 MHz (e.g., only 2 pilots, plenty of spectrum).

## Design

### Change 1: Power ceiling interstitial — DJI bandwidth guidance

The existing interstitial keeps its current content (mW number + "set your VTX to X mW or below"). Below that, add a secondary line:

> "DJI pilots: use 20 MHz bandwidth mode for best channel compatibility."

**Visibility rules:**
- Only shown when `state.sessionPowerCeiling > 0 && state.sessionPowerCeiling < 600`
- At 600+ mW, channels are already sparse (4 raceband max), so bandwidth advice is less relevant
- Shows for all joiners regardless of video system (video system isn't known yet at this step)
- Analog/HDZero pilots will see it but can ignore it — not harmful

**No changes to the interstitial flow.** It still appears after callsign entry and before video system selection.

### Change 2: Bandwidth button highlighting for DJI O3/O4

When a DJI O3 or O4 pilot reaches the bandwidth selection step, and `state.sessionPowerCeiling > 0 && state.sessionPowerCeiling < 600`:

- **10 MHz and 20 MHz buttons**: get a "RECOMMENDED" label or subtle green indicator
- **40 MHz button** (O3 and O4): gets an amber/yellow tint or border
- **60 MHz button** (O4 only): gets an amber/yellow tint or border

This reinforces the interstitial guidance at the point of decision. Note: O3 offers `[10, 20, 40]` while O4 offers `[10, 20, 40, 60]`.

**Applies to all bandwidth selection paths:**
- **Join wizard**: `showBandwidthOptions()` during initial join flow
- **Video system change**: same `showBandwidthOptions()` called when a pilot changes their video system mid-session (the interstitial is NOT re-shown, but the bandwidth button styling applies since the pilot already saw the guidance at join time)
- **Leader add-pilot dialog**: the add-pilot bandwidth buttons in `showAddPilotOptions()` use a separate code path (`add-pilot-bw-buttons` with `btn-toggle` elements). Apply the same recommended/warning styling here.

**When NOT to highlight:**
- No power ceiling set (`state.sessionPowerCeiling === 0`)
- Power ceiling >= 600 mW — channels are already sparse, bandwidth choice is less impactful
- Non-DJI video systems (they don't see bandwidth buttons anyway)
- Walksnail is intentionally excluded — it has race mode vs standard mode but doesn't use variable bandwidth in the same way

### Change 3: Frequency reference documentation

Add a "DJI Dynamic Power Control" section to `frequency-reference.md` covering:

- DJI O3/O4 use automatic power control — no manual mW setting
- Power scales with link quality (distance, obstacles, antenna orientation)
- At meetup distances (~100m LOS), estimated under 200 mW; ramps toward max (~1W) at range
- These are community observations, not manufacturer specs
- Bandwidth is the meaningful lever — 20 MHz vs 40 MHz saves 10 MHz required spacing per neighbor
- Practical experience: 6-8 mixed pilots fitting on raceband with 20 MHz guidance

Placement: new `## DJI Dynamic Power Control` section (h2) after the `### Assumptions and Limitations` subsection (line 308), as a peer section to `## Transmit Power and Channel Separation` (line 226).

## What's NOT changing

- **Optimizer**: No DJI-specific guard band logic. `PowerToGuardBand()` and `RequiredSpacing()` remain video-system-agnostic.
- **API**: No backend changes.
- **Session creation**: Power ceiling wizard unchanged.
- **Session badge**: Unchanged.

## Files to modify

| File | Change |
|------|--------|
| `static/index.html` | Add DJI guidance line to `step-power-alert` |
| `static/app.js` | Conditionally show DJI guidance line; add recommended/warning styling to bandwidth buttons in join wizard, video system change, and leader add-pilot dialog |
| `static/style.css` | Styles for DJI guidance text, recommended badge, amber bandwidth warning |
| `frequency-reference.md` | New "DJI Dynamic Power Control" section |
| `static/sw.js` | Bump cache version |

# Expanded Analog Bands Design

**Date:** 2026-03-09
**GitHub Issue:** kgNatx/skwad#4

## Problem

Analog pilots are hard-coded to Race Band only (R1-R8, 8 channels). This limits capacity when many analog pilots are flying and causes unnecessary conflicts with digital systems (DJI, Walksnail) whose channels overlap heavily with Race Band.

## Goals

1. **More capacity** — give analog pilots access to up to 32 channels across 4 bands
2. **Better digital coexistence** — the optimizer can place analog pilots on frequencies that avoid digital channels
3. **Per-pilot band selection** — not every VTX supports all bands, so pilots choose which bands their gear supports

## Design

### New Bands

| Band | Code | Channels | Frequency Range |
|------|------|----------|----------------|
| Race (existing) | R | R1-R8 | 5658-5917 MHz |
| Fatshark | F | F1-F8 | 5740-5880 MHz |
| Boscam E | E | E1-E8 | 5645-5945 MHz |
| Low Race | L | L1-L8 | 5362-5621 MHz |

### Data Model Changes

- **Pilot struct**: add `AnalogBands string` field (comma-separated band codes, e.g. `"R,F,E"`)
- **pilots table**: add `analog_bands TEXT DEFAULT 'R'` column
- **JoinRequest**: add `AnalogBands []string json:"analog_bands"` field
- Default: `"R"` (Race Band only) — backward compatible with existing behavior

### Backend Changes

#### `freq/tables.go`

- Add `FatsharkBand`, `BoscamEBand`, `LowRaceBand` channel slice variables
- Add `AnalogBands` map: `map[string][]Channel` mapping code letters to slices
- Add `mergeAnalogBands(bands []string) []Channel` — unions selected bands, deduplicates by frequency
- Modify `ChannelPool()` signature to accept `analogBands []string` parameter
- Analog case: call `mergeAnalogBands()` instead of returning `RaceBand` directly; fall back to `RaceBand` if empty

#### `api/handlers.go`

- Add `AnalogBands` to `JoinRequest` struct
- Pass `AnalogBands` through to `ChannelPool()` calls
- Validate band codes (only R, F, E, L accepted; at least one required for analog)

#### `db/db.go`

- Add `AnalogBands` field to `Pilot` struct
- Add `analog_bands` column to schema
- Update all SQL queries that read/write pilots to include the new column

### Frontend Changes

#### Join/Add Flow (`app.js`)

- Remove `"analog"` from `SIMPLE_SYSTEMS`
- When analog is selected in `startFollowUpFlow()`, show analog band selector:
  - Title: "ANALOG SETTINGS"
  - Four toggle buttons: **R** (Race), **F** (Fatshark), **E** (Boscam E), **L** (Low Race)
  - R is pre-selected by default
  - Helper text below: "Not sure? Just use Race Band" — clicking resets to R only
  - At least one band must be selected to enable the Next button
- Store selected bands in `state.analogBands` array
- Send `analog_bands` field in join/add API requests

#### Channel Selector

- Add new band frequency arrays to the `CHANNELS` object
- `getChannelPool()` function: for analog, union channels from selected bands (mirrors backend logic)

#### Pilot Card Display

- No changes needed — existing freq/channel display handles any channel name and frequency

### Optimizer

- **No changes needed** — the optimizer is system-agnostic and works with whatever channel pool `ChannelPool()` returns

### Migration

- Existing analog pilots in active sessions will have `analog_bands = 'R'` by default
- SQLite schema uses `DEFAULT 'R'` so existing rows get the correct value automatically

## UX Flow

1. User taps ANALOG in system picker
2. Follow-up screen shows "ANALOG SETTINGS" with R/F/E/L band toggles (R pre-selected)
3. "Not sure? Just use Race Band" helper text for uncertain users
4. User selects bands, taps Next
5. Channel preference step (same as today)
6. Join completes with expanded pool available to optimizer

## Out of Scope

- Band A (Boscam A) — too much overlap with R and F to add value
- Regulatory warnings for Band L — pilots who know L know what they're doing
- Auto-detecting VTX capabilities

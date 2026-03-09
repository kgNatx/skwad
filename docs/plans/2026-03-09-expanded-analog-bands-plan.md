# Expanded Analog Bands Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Give analog pilots access to Fatshark (F), Boscam E (E), and Low Race (L) bands in addition to Race Band (R), with per-pilot band selection.

**Architecture:** Add band table definitions to `freq/tables.go`, a new `AnalogBands` field threaded through the Pilot struct / DB / API / frontend, and a `MergeAnalogBands()` function that unions selected bands into one channel pool. The optimizer needs zero changes.

**Tech Stack:** Go (backend), vanilla JS (frontend), SQLite (DB)

**Design doc:** `docs/plans/2026-03-09-expanded-analog-bands-design.md`

**Go binary:** `/home/kyleg/.local/go/bin/go` (not on PATH — use full path for all go commands)

**Test command:** `/home/kyleg/.local/go/bin/go test ./...`

**Build command:** `docker compose build skwad && docker compose up -d skwad`

---

### Task 1: Add new band tables to `freq/tables.go`

**Files:**
- Modify: `freq/tables.go:37-50` (after RaceBand, before DJI section)

**Step 1: Add FatsharkBand, BoscamEBand, LowRaceBand variables**

Add after `RaceBand` (line 50), before the DJI V1 section comment:

```go
// ---------- Fatshark Band ----------

// FatsharkBand contains the 8 Fatshark channels (F1-F8).
var FatsharkBand = []Channel{
	{"F1", 5740},
	{"F2", 5760},
	{"F3", 5780},
	{"F4", 5800},
	{"F5", 5820},
	{"F6", 5840},
	{"F7", 5860},
	{"F8", 5880},
}

// ---------- Boscam E Band ----------

// BoscamEBand contains the 8 Boscam E channels (E1-E8).
var BoscamEBand = []Channel{
	{"E1", 5705},
	{"E2", 5685},
	{"E3", 5665},
	{"E4", 5645},
	{"E5", 5885},
	{"E6", 5905},
	{"E7", 5925},
	{"E8", 5945},
}

// ---------- Low Race Band ----------

// LowRaceBand contains the 8 Low Race channels (L1-L8).
var LowRaceBand = []Channel{
	{"L1", 5362},
	{"L2", 5399},
	{"L3", 5436},
	{"L4", 5473},
	{"L5", 5510},
	{"L6", 5547},
	{"L7", 5584},
	{"L8", 5621},
}

// AnalogBandMap maps single-letter band codes to their channel slices.
var AnalogBandMap = map[string][]Channel{
	"R": RaceBand,
	"F": FatsharkBand,
	"E": BoscamEBand,
	"L": LowRaceBand,
}
```

**Step 2: Add `MergeAnalogBands` function**

Add before `ChannelPool()` (before line 147):

```go
// MergeAnalogBands returns the union of channels from the given band codes,
// deduplicating by frequency (keeps the first name encountered).
func MergeAnalogBands(bands []string) []Channel {
	if len(bands) == 0 {
		return RaceBand
	}
	seen := make(map[int]bool)
	var merged []Channel
	for _, code := range bands {
		band, ok := AnalogBandMap[code]
		if !ok {
			continue
		}
		for _, ch := range band {
			if !seen[ch.FreqMHz] {
				seen[ch.FreqMHz] = true
				merged = append(merged, ch)
			}
		}
	}
	if len(merged) == 0 {
		return RaceBand
	}
	return merged
}
```

**Step 3: Modify `ChannelPool()` signature and analog case**

Change the function signature to add `analogBands []string`:

```go
func ChannelPool(videoSystem string, fccUnlocked bool, bandwidthMHz int, raceMode bool, goggles string, analogBands []string) []Channel {
```

Change the analog case (line 151-152) from:

```go
	case "analog", "hdzero":
		return RaceBand
```

To:

```go
	case "analog":
		return MergeAnalogBands(analogBands)
	case "hdzero":
		return RaceBand
```

**Step 4: Run tests to see what breaks**

Run: `/home/kyleg/.local/go/bin/go build ./...`
Expected: Compilation errors in all callers of `ChannelPool()` — this is expected. We'll fix them in later tasks.

**Step 5: Commit**

```
feat(freq): add Fatshark, Boscam E, Low Race band tables and MergeAnalogBands
```

---

### Task 2: Write tests for new band tables and MergeAnalogBands

**Files:**
- Modify: `freq/tables_test.go`

**Step 1: Add tests**

Add to `freq/tables_test.go`:

```go
func TestChannelPool_Analog_DefaultRaceBand(t *testing.T) {
	// No analog bands specified — should fall back to race band
	pool := ChannelPool("analog", false, 0, false, "", nil)
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
	if pool[0].Name != "R1" {
		t.Errorf("first channel = %v, want R1", pool[0].Name)
	}
}

func TestChannelPool_Analog_RaceBandOnly(t *testing.T) {
	pool := ChannelPool("analog", false, 0, false, "", []string{"R"})
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
	if pool[0].Name != "R1" {
		t.Errorf("first channel = %v, want R1", pool[0].Name)
	}
}

func TestChannelPool_Analog_MultiBand(t *testing.T) {
	pool := ChannelPool("analog", false, 0, false, "", []string{"R", "F"})
	// R has 8, F has 8, but some may overlap in frequency
	if len(pool) < 9 {
		t.Fatalf("expected at least 9 channels for R+F, got %d", len(pool))
	}
	// Check deduplication: no duplicate frequencies
	seen := make(map[int]bool)
	for _, ch := range pool {
		if seen[ch.FreqMHz] {
			t.Errorf("duplicate frequency %d MHz", ch.FreqMHz)
		}
		seen[ch.FreqMHz] = true
	}
}

func TestChannelPool_Analog_AllBands(t *testing.T) {
	pool := ChannelPool("analog", false, 0, false, "", []string{"R", "F", "E", "L"})
	// Should have many channels but with dedup
	if len(pool) < 20 {
		t.Fatalf("expected at least 20 channels for all bands, got %d", len(pool))
	}
	seen := make(map[int]bool)
	for _, ch := range pool {
		if seen[ch.FreqMHz] {
			t.Errorf("duplicate frequency %d MHz", ch.FreqMHz)
		}
		seen[ch.FreqMHz] = true
	}
}

func TestChannelPool_Analog_InvalidBandFallback(t *testing.T) {
	pool := ChannelPool("analog", false, 0, false, "", []string{"Z"})
	// Invalid band code should fall back to race band
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels (fallback), got %d", len(pool))
	}
}

func TestMergeAnalogBands_Empty(t *testing.T) {
	pool := MergeAnalogBands(nil)
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels (RaceBand fallback), got %d", len(pool))
	}
}

func TestMergeAnalogBands_NoDuplicateFreqs(t *testing.T) {
	pool := MergeAnalogBands([]string{"R", "F", "E", "L"})
	seen := make(map[int]bool)
	for _, ch := range pool {
		if seen[ch.FreqMHz] {
			t.Errorf("duplicate freq %d MHz (channel %s)", ch.FreqMHz, ch.Name)
		}
		seen[ch.FreqMHz] = true
	}
}

func TestFatsharkBand(t *testing.T) {
	if len(FatsharkBand) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(FatsharkBand))
	}
	if FatsharkBand[0].Name != "F1" || FatsharkBand[0].FreqMHz != 5740 {
		t.Errorf("first channel = %v, want F1/5740", FatsharkBand[0])
	}
}

func TestBoscamEBand(t *testing.T) {
	if len(BoscamEBand) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(BoscamEBand))
	}
	if BoscamEBand[0].Name != "E1" || BoscamEBand[0].FreqMHz != 5705 {
		t.Errorf("first channel = %v, want E1/5705", BoscamEBand[0])
	}
}

func TestLowRaceBand(t *testing.T) {
	if len(LowRaceBand) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(LowRaceBand))
	}
	if LowRaceBand[0].Name != "L1" || LowRaceBand[0].FreqMHz != 5362 {
		t.Errorf("first channel = %v, want L1/5362", LowRaceBand[0])
	}
}

func TestChannelPool_HDZero_UnaffectedByAnalogBands(t *testing.T) {
	// HDZero should still return race band regardless of analogBands param
	pool := ChannelPool("hdzero", false, 0, false, "", []string{"F", "E"})
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
	if pool[0].Name != "R1" {
		t.Errorf("HDZero should use RaceBand, got %s", pool[0].Name)
	}
}
```

**Step 2: Update existing `TestChannelPool_Analog` test**

Change the existing test (line 5-13) to pass `nil` for the new `analogBands` parameter:

```go
func TestChannelPool_Analog(t *testing.T) {
	pool := ChannelPool("analog", false, 0, false, "", nil)
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
	if pool[0].Name != "R1" || pool[0].FreqMHz != 5658 {
		t.Errorf("first channel = %v, want R1/5658", pool[0])
	}
}
```

**Step 3: Update ALL other existing `ChannelPool()` test calls**

Every existing test that calls `ChannelPool()` needs `nil` appended as the 6th arg. Update these tests in `freq/tables_test.go`:
- `TestChannelPool_HDZero` (line 16)
- `TestChannelPool_DJI_V1_Stock` (line 23)
- `TestChannelPool_DJI_V1_FCC` (line 30)
- `TestChannelPool_DJI_O3_Stock_20MHz` (line 37)
- `TestChannelPool_DJI_O3_FCC_20MHz` (line 44)
- `TestChannelPool_DJI_O4_RaceMode` (line 51)
- `TestChannelPool_Walksnail_RaceMode` (line 61)
- `TestChannelPool_Walksnail_Std_Stock` (line 68)
- `TestChannelPool_Walksnail_Std_FCC` (line 75)

**Step 4: Run tests**

Run: `/home/kyleg/.local/go/bin/go test ./freq/ -v -run "TestChannelPool|TestMerge|TestFatshark|TestBoscam|TestLowRace"`
Expected: PASS (tests that don't depend on callers in other packages)

**Step 5: Commit**

```
test(freq): add tests for expanded analog band tables and MergeAnalogBands
```

---

### Task 3: Update `ChannelPool()` callers in `freq/optimizer.go`

**Files:**
- Modify: `freq/optimizer.go`

The `ChannelPool()` function now takes 6 args. The `PilotInput` struct needs an `AnalogBands` field, and all `ChannelPool()` calls in the optimizer need to pass it.

**Step 1: Add `AnalogBands` to `PilotInput` struct**

In `freq/optimizer.go:6-17`, add after the `PrevFreqMHz` field:

```go
	AnalogBands   []string // Band codes for analog: "R", "F", "E", "L"
```

**Step 2: Update all `ChannelPool()` calls in optimizer.go**

There are 5 calls to `ChannelPool()` in `freq/optimizer.go`. Each needs `p.AnalogBands` appended:

- Line 46: `ChannelPool(p.VideoSystem, p.FCCUnlocked, p.BandwidthMHz, p.RaceMode, p.Goggles)` → add `, p.AnalogBands`
- Line 414: `ChannelPool(flex[i].VideoSystem, flex[i].FCCUnlocked, flex[i].BandwidthMHz, flex[i].RaceMode, flex[i].Goggles)` → add `, flex[i].AnalogBands`
- Line 415: `ChannelPool(flex[j].VideoSystem, flex[j].FCCUnlocked, flex[j].BandwidthMHz, flex[j].RaceMode, flex[j].Goggles)` → add `, flex[j].AnalogBands`
- Line 452: `ChannelPool(newPilot.VideoSystem, newPilot.FCCUnlocked, newPilot.BandwidthMHz, newPilot.RaceMode, newPilot.Goggles)` → add `, newPilot.AnalogBands`
- Line 499: `ChannelPool(best.pilot.VideoSystem, best.pilot.FCCUnlocked, best.pilot.BandwidthMHz, best.pilot.RaceMode, best.pilot.Goggles)` → add `, best.pilot.AnalogBands`

**Step 3: Verify it compiles**

Run: `/home/kyleg/.local/go/bin/go build ./freq/`
Expected: Success

**Step 4: Commit**

```
feat(freq): thread AnalogBands through PilotInput and optimizer ChannelPool calls
```

---

### Task 4: Update database schema and Pilot struct

**Files:**
- Modify: `db/db.go`

**Step 1: Add `AnalogBands` field to Pilot struct**

In `db/db.go:29-44`, add after the `Active` field (line 43):

```go
	AnalogBands        string // comma-separated band codes: "R", "R,F,E", etc.
```

**Step 2: Add column to schema**

In `db/db.go:54-72`, add `analog_bands TEXT DEFAULT 'R'` after the `active` column (line 69), before the `FOREIGN KEY`:

```go
	analog_bands TEXT DEFAULT 'R',
```

**Step 3: Add migration**

In `db/db.go:102-108`, add to the `migrate()` function after the existing migration:

```go
	_, err = d.db.Exec(`ALTER TABLE pilots ADD COLUMN analog_bands TEXT DEFAULT 'R'`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate analog_bands: %w", err)
	}
```

**Step 4: Update `AddPilot()` SQL**

In `db/db.go:210-218`, add `analog_bands` to the INSERT:

```go
	res, err := d.db.Exec(
		`INSERT INTO pilots (session_id, callsign, video_system, fcc_unlocked, goggles,
			bandwidth_mhz, race_mode, channel_locked, locked_frequency_mhz,
			assigned_channel, assigned_frequency_mhz, buddy_group, joined_at, active, analog_bands)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?)`,
		sessionID, p.Callsign, p.VideoSystem, p.FCCUnlocked, p.Goggles,
		p.BandwidthMHz, p.RaceMode, p.ChannelLocked, p.LockedFrequencyMHz,
		p.AssignedChannel, p.AssignedFreqMHz, p.BuddyGroup, time.Now().UTC(),
		p.AnalogBands,
	)
```

**Step 5: Update `reactivatePilot()` SQL**

In `db/db.go:257-265`, add `analog_bands` to the UPDATE:

```go
	_, err = d.db.Exec(
		`UPDATE pilots SET video_system = ?, fcc_unlocked = ?, goggles = ?,
			bandwidth_mhz = ?, race_mode = ?, channel_locked = ?, locked_frequency_mhz = ?,
			assigned_channel = '', assigned_frequency_mhz = 0, buddy_group = 0,
			joined_at = ?, active = TRUE, analog_bands = ?
		WHERE id = ?`,
		p.VideoSystem, p.FCCUnlocked, p.Goggles,
		p.BandwidthMHz, p.RaceMode, p.ChannelLocked, p.LockedFrequencyMHz,
		time.Now().UTC(), p.AnalogBands, existingID,
	)
```

**Step 6: Update `GetActivePilots()` SQL**

In `db/db.go:280-308`, add `analog_bands` to the SELECT and Scan:

```go
	rows, err := d.db.Query(
		`SELECT id, session_id, callsign, video_system, fcc_unlocked, goggles,
			bandwidth_mhz, race_mode, channel_locked, locked_frequency_mhz,
			assigned_channel, assigned_frequency_mhz, buddy_group, active, analog_bands
		FROM pilots
		WHERE session_id = ? AND active = TRUE
		ORDER BY id`,
		sessionID,
	)
```

And the Scan:

```go
		if err := rows.Scan(
			&p.ID, &p.SessionID, &p.Callsign, &p.VideoSystem, &p.FCCUnlocked,
			&p.Goggles, &p.BandwidthMHz, &p.RaceMode, &p.ChannelLocked,
			&p.LockedFrequencyMHz, &p.AssignedChannel, &p.AssignedFreqMHz,
			&p.BuddyGroup, &p.Active, &p.AnalogBands,
		); err != nil {
```

**Step 7: Verify it compiles**

Run: `/home/kyleg/.local/go/bin/go build ./db/`
Expected: Success

**Step 8: Commit**

```
feat(db): add analog_bands column to pilots table with migration
```

---

### Task 5: Update API handlers to thread AnalogBands

**Files:**
- Modify: `api/handlers.go`

**Step 1: Add `AnalogBands` to `JoinRequest`**

In `api/handlers.go:46-55`, add after `LockedFreqMHz`:

```go
	AnalogBands   []string `json:"analog_bands"`
```

**Step 2: Add `AnalogBands` to `UpdateVideoSystemRequest`**

In `api/handlers.go:535-541`, add after `RaceMode`:

```go
	AnalogBands  []string `json:"analog_bands"`
```

**Step 3: Helper to convert `[]string` to comma-separated string**

Add near the top of handlers.go (after imports):

```go
// joinBands converts a slice of band codes to a comma-separated string.
// Returns "R" if the slice is empty (default to Race Band).
func joinBands(bands []string) string {
	if len(bands) == 0 {
		return "R"
	}
	return strings.Join(bands, ",")
}

// splitBands converts a comma-separated band string to a slice.
func splitBands(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
```

**Step 4: Update `HandleJoinSession` pilot construction**

In `api/handlers.go:174-183`, add `AnalogBands`:

```go
	pilot := &db.Pilot{
		Callsign:           req.Callsign,
		VideoSystem:        req.VideoSystem,
		FCCUnlocked:        req.FCCUnlocked,
		Goggles:            req.Goggles,
		BandwidthMHz:       req.BandwidthMHz,
		RaceMode:           req.RaceMode,
		ChannelLocked:      req.ChannelLocked,
		LockedFrequencyMHz: req.LockedFreqMHz,
		AnalogBands:        joinBands(req.AnalogBands),
	}
```

**Step 5: Update `HandleAddPilot` pilot construction**

In `api/handlers.go:891-900`, add `AnalogBands`:

```go
	pilot := &db.Pilot{
		Callsign:           req.Callsign,
		VideoSystem:        req.VideoSystem,
		FCCUnlocked:        req.FCCUnlocked,
		Goggles:            req.Goggles,
		BandwidthMHz:       req.BandwidthMHz,
		RaceMode:           req.RaceMode,
		ChannelLocked:      req.ChannelLocked,
		LockedFrequencyMHz: req.LockedFreqMHz,
		AnalogBands:        joinBands(req.AnalogBands),
	}
```

**Step 6: Update `HandlePreviewJoin` newPilotInput construction**

In `api/handlers.go:331-340`, add `AnalogBands`:

```go
	newPilotInput := freq.PilotInput{
		ID:            tempID,
		VideoSystem:   req.VideoSystem,
		FCCUnlocked:   req.FCCUnlocked,
		BandwidthMHz:  req.BandwidthMHz,
		RaceMode:      req.RaceMode,
		Goggles:       req.Goggles,
		ChannelLocked: req.ChannelLocked,
		LockedFreqMHz: req.LockedFreqMHz,
		AnalogBands:   req.AnalogBands,
	}
```

**Step 7: Update `buildPilotInputs` to pass AnalogBands**

In `api/handlers.go:700-717`, add `AnalogBands` to the `PilotInput`:

```go
	inputs[i] = freq.PilotInput{
		ID:            p.ID,
		VideoSystem:   p.VideoSystem,
		FCCUnlocked:   p.FCCUnlocked,
		BandwidthMHz:  p.BandwidthMHz,
		RaceMode:      p.RaceMode,
		Goggles:       p.Goggles,
		ChannelLocked: p.ChannelLocked,
		LockedFreqMHz: p.LockedFrequencyMHz,
		PrevChannel:   p.AssignedChannel,
		PrevFreqMHz:   p.AssignedFreqMHz,
		AnalogBands:   splitBands(p.AnalogBands),
	}
```

**Step 8: Update `reoptimize` to pass AnalogBands**

In `api/handlers.go:976-989`, add `AnalogBands` to the input construction:

```go
	inputs[i] = freq.PilotInput{
		ID:            p.ID,
		VideoSystem:   p.VideoSystem,
		FCCUnlocked:   p.FCCUnlocked,
		BandwidthMHz:  p.BandwidthMHz,
		RaceMode:      p.RaceMode,
		Goggles:       p.Goggles,
		ChannelLocked: p.ChannelLocked,
		LockedFreqMHz: p.LockedFrequencyMHz,
		PrevChannel:   p.AssignedChannel,
		PrevFreqMHz:   p.AssignedFreqMHz,
		AnalogBands:   splitBands(p.AnalogBands),
	}
```

**Step 9: Update `HandleUpdatePilotVideoSystem` to persist AnalogBands**

In `db/db.go`, update `UpdatePilotVideoSystem` to also set `analog_bands`:

Change signature:
```go
func (d *DB) UpdatePilotVideoSystem(pilotID int, videoSystem string, fccUnlocked bool, goggles string, bandwidthMHz int, raceMode bool, analogBands string) error {
```

Change SQL:
```go
	res, err := d.db.Exec(
		`UPDATE pilots SET video_system = ?, fcc_unlocked = ?, goggles = ?, bandwidth_mhz = ?, race_mode = ?, analog_bands = ? WHERE id = ? AND active = TRUE`,
		videoSystem, fccUnlocked, goggles, bandwidthMHz, raceMode, analogBands, pilotID,
	)
```

Then update the caller in `api/handlers.go:557`:

```go
	if err := s.DB.UpdatePilotVideoSystem(pilotID, req.VideoSystem, req.FCCUnlocked, req.Goggles, req.BandwidthMHz, req.RaceMode, joinBands(req.AnalogBands)); err != nil {
```

**Step 10: Build and run tests**

Run: `/home/kyleg/.local/go/bin/go test ./... -v`
Expected: All tests pass

**Step 11: Commit**

```
feat(api): thread AnalogBands through handlers, JoinRequest, and buildPilotInputs
```

---

### Task 6: Update frontend — band tables and channel pool logic

**Files:**
- Modify: `static/app.js`

**Step 1: Add new band arrays to CHANNELS object**

In `static/app.js:52-97`, add after `raceband` (line 58), before `dji_v1_fcc`:

```javascript
    fatshark: [
      { name: 'F1', freq: 5740 }, { name: 'F2', freq: 5760 },
      { name: 'F3', freq: 5780 }, { name: 'F4', freq: 5800 },
      { name: 'F5', freq: 5820 }, { name: 'F6', freq: 5840 },
      { name: 'F7', freq: 5860 }, { name: 'F8', freq: 5880 },
    ],
    boscam_e: [
      { name: 'E1', freq: 5705 }, { name: 'E2', freq: 5685 },
      { name: 'E3', freq: 5665 }, { name: 'E4', freq: 5645 },
      { name: 'E5', freq: 5885 }, { name: 'E6', freq: 5905 },
      { name: 'E7', freq: 5925 }, { name: 'E8', freq: 5945 },
    ],
    lowrace: [
      { name: 'L1', freq: 5362 }, { name: 'L2', freq: 5399 },
      { name: 'L3', freq: 5436 }, { name: 'L4', freq: 5473 },
      { name: 'L5', freq: 5510 }, { name: 'L6', freq: 5547 },
      { name: 'L7', freq: 5584 }, { name: 'L8', freq: 5621 },
    ],
```

**Step 2: Add analog band map constant**

Add after CHANNELS:

```javascript
  // Maps band code letters to CHANNELS keys
  const ANALOG_BAND_MAP = { R: 'raceband', F: 'fatshark', E: 'boscam_e', L: 'lowrace' };
```

**Step 3: Add `mergeAnalogBands` helper**

Add after `ANALOG_BAND_MAP`:

```javascript
  function mergeAnalogBands(bands) {
    if (!bands || bands.length === 0) return CHANNELS.raceband;
    var seen = {};
    var merged = [];
    bands.forEach(function (code) {
      var key = ANALOG_BAND_MAP[code];
      if (!key || !CHANNELS[key]) return;
      CHANNELS[key].forEach(function (ch) {
        if (!seen[ch.freq]) {
          seen[ch.freq] = true;
          merged.push(ch);
        }
      });
    });
    return merged.length > 0 ? merged : CHANNELS.raceband;
  }
```

**Step 4: Add `analogBands` to state**

In `static/app.js:10-31`, add after `raceMode: false,`:

```javascript
    analogBands: ['R'],
```

**Step 5: Update `getChannelPool()` for analog**

In `static/app.js:278-310` (the `getChannelPool` function), change the analog case from:

```javascript
      case 'analog':
      case 'hdzero':
        return CHANNELS.raceband;
```

To:

```javascript
      case 'analog':
        return mergeAnalogBands(state.analogBands);
      case 'hdzero':
        return CHANNELS.raceband;
```

**Step 6: Update `getChannelPoolForPilot()` for analog**

In `static/app.js:1530-1560` (the `getChannelPoolForPilot` function), change the analog case from:

```javascript
      case 'analog':
      case 'hdzero':
        return CHANNELS.raceband;
```

To:

```javascript
      case 'analog':
        return mergeAnalogBands(pilot.AnalogBands ? pilot.AnalogBands.split(',') : ['R']);
      case 'hdzero':
        return CHANNELS.raceband;
```

**Step 7: Update `buildJoinBody()` to include analog_bands**

In `static/app.js:725-736`, add `analog_bands`:

```javascript
  function buildJoinBody() {
    return {
      callsign: state.callsign,
      video_system: getEffectiveVideoSystem(),
      fcc_unlocked: state.fccUnlocked,
      goggles: state.goggles,
      bandwidth_mhz: state.bandwidthMHz,
      race_mode: state.raceMode,
      channel_locked: state.channelLocked,
      locked_frequency_mhz: state.lockedFreqMHz,
      analog_bands: state.analogBands,
    };
  }
```

**Step 8: Commit**

```
feat(ui): add analog band tables, mergeAnalogBands, and thread through join/pool logic
```

---

### Task 7: Update frontend — analog follow-up flow (join wizard)

**Files:**
- Modify: `static/index.html`
- Modify: `static/app.js`

**Step 1: Add analog bands followup group to HTML**

In `static/index.html`, add a new followup group after the walksnail-mode group (after line 147), before the Next button:

```html
      <div id="followup-analog-bands" class="followup-group hidden">
        <p class="followup-label">VTX BANDS</p>
        <div class="btn-row" id="analog-band-buttons">
          <button class="btn btn-option analog-band-btn selected" data-band="R">R (RACE)</button>
          <button class="btn btn-option analog-band-btn" data-band="F">F (FATSHARK)</button>
          <button class="btn btn-option analog-band-btn" data-band="E">E (BOSCAM)</button>
          <button class="btn btn-option analog-band-btn" data-band="L">L (LOW RACE)</button>
        </div>
        <p class="followup-helper" id="analog-bands-helper">NOT SURE? JUST USE RACE BAND</p>
      </div>
```

**Step 2: Remove "analog" from the simple-systems skip list in `startFollowUpFlow()`**

In `static/app.js:533`, change:

```javascript
    if (['analog', 'hdzero', 'openipc'].includes(system)) {
```

To:

```javascript
    if (['hdzero', 'openipc'].includes(system)) {
```

**Step 3: Add analog handling to `startFollowUpFlow()`**

In `static/app.js:538-552`, add an `else if` for analog before the existing `if (system === 'walksnail')`:

```javascript
    if (system === 'analog') {
      $('followup-title').textContent = 'ANALOG SETTINGS';
      $('followup-analog-bands').classList.remove('hidden');
      $('btn-followup-next').classList.remove('hidden');
      state.analogBands = ['R'];
      // Reset band button states
      document.querySelectorAll('.analog-band-btn').forEach(function (b) {
        b.classList.toggle('selected', b.dataset.band === 'R');
      });
    } else if (system === 'walksnail') {
```

**Step 4: Wire up analog band button click handlers in `initSetupWizard()`**

Add in the setup wizard init section (after the existing followup button handlers):

```javascript
    // Analog band toggle buttons
    document.querySelectorAll('.analog-band-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        btn.classList.toggle('selected');
        // Collect selected bands
        var selected = [];
        document.querySelectorAll('.analog-band-btn.selected').forEach(function (b) {
          selected.push(b.dataset.band);
        });
        // Must have at least one band
        if (selected.length === 0) {
          btn.classList.add('selected');
          selected.push(btn.dataset.band);
        }
        state.analogBands = selected;
      });
    });

    // "Not sure? Just use Race Band" helper
    $('analog-bands-helper').addEventListener('click', function () {
      document.querySelectorAll('.analog-band-btn').forEach(function (b) {
        b.classList.toggle('selected', b.dataset.band === 'R');
      });
      state.analogBands = ['R'];
    });
```

**Step 5: Add CSS for helper text**

In `static/style.css`, add:

```css
.followup-helper {
  font-size: 14px;
  color: #999;
  text-decoration: underline;
  cursor: pointer;
  margin-top: 8px;
  text-align: center;
}
.followup-helper:active {
  color: #fff;
}
```

**Step 6: Commit**

```
feat(ui): analog band selector in join wizard follow-up flow
```

---

### Task 8: Update frontend — add-pilot dialog for analog bands

**Files:**
- Modify: `static/index.html`
- Modify: `static/app.js`

**Step 1: Add analog band options to add-pilot dialog HTML**

In `static/index.html`, inside `add-pilot-options` (after the bandwidth div, before the confirm button), add:

```html
        <div id="add-pilot-bands" class="add-pilot-option-row hidden">
          <label class="toggle-label">VTX BANDS</label>
          <div id="add-pilot-band-buttons" class="btn-row">
            <button class="btn btn-toggle add-band-btn active" data-add-band="R">R</button>
            <button class="btn btn-toggle add-band-btn" data-add-band="F">F</button>
            <button class="btn btn-toggle add-band-btn" data-add-band="E">E</button>
            <button class="btn btn-toggle add-band-btn" data-add-band="L">L</button>
          </div>
        </div>
```

**Step 2: Update `addPilotState` to include analogBands**

In `static/app.js:1922`, change:

```javascript
  var addPilotState = { system: '', fccUnlocked: false, bandwidthMHz: 0 };
```

To:

```javascript
  var addPilotState = { system: '', fccUnlocked: false, bandwidthMHz: 0, analogBands: ['R'] };
```

**Step 3: Remove "analog" from SIMPLE_SYSTEMS**

In `static/app.js:1916`, change:

```javascript
  var SIMPLE_SYSTEMS = ['analog', 'hdzero', 'openipc', 'walksnail_race'];
```

To:

```javascript
  var SIMPLE_SYSTEMS = ['hdzero', 'openipc', 'walksnail_race'];
```

**Step 4: Update `showAddPilotOptions()` to show band selector for analog**

In `static/app.js:1941-1978`, add after the bandwidth section (after line 1976):

```javascript
    // Analog band buttons
    if (system === 'analog') {
      addPilotState.analogBands = ['R'];
      document.querySelectorAll('.add-band-btn').forEach(function (b) {
        b.classList.toggle('active', b.dataset.addBand === 'R');
      });
      $('add-pilot-bands').classList.remove('hidden');
    } else {
      $('add-pilot-bands').classList.add('hidden');
    }
```

**Step 5: Wire up add-pilot band button click handlers in `initAddPilotDialog()`**

Add in `initAddPilotDialog()` (after the FCC toggle handler):

```javascript
    // Analog band toggles for add-pilot
    document.querySelectorAll('.add-band-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        btn.classList.toggle('active');
        var selected = [];
        document.querySelectorAll('.add-band-btn.active').forEach(function (b) {
          selected.push(b.dataset.addBand);
        });
        if (selected.length === 0) {
          btn.classList.add('active');
          selected.push(btn.dataset.addBand);
        }
        addPilotState.analogBands = selected;
      });
    });
```

**Step 6: Update `addPilot()` function to pass analog_bands**

In `static/app.js:2039-2057`, change the function signature and API call:

```javascript
  async function addPilot(callsign, videoSystem, fccUnlocked, bandwidthMHz, analogBands) {
    try {
      await apiPost('/api/sessions/' + state.sessionCode + '/add-pilot', {
        callsign: callsign,
        video_system: videoSystem,
        fcc_unlocked: fccUnlocked || false,
        bandwidth_mhz: bandwidthMHz || 0,
        analog_bands: analogBands || ['R'],
      });
```

**Step 7: Update callers of `addPilot()`**

The simple-system call at line 2007:

```javascript
          addPilot(callsign, system, false, 0, ['R']);
```

The confirm button call at line 2030:

```javascript
      addPilot(callsign, addPilotState.system, addPilotState.fccUnlocked, addPilotState.bandwidthMHz, addPilotState.analogBands);
```

**Step 8: Reset add-pilot state in `showAddPilotDialog()`**

In `static/app.js:1932`, update the reset:

```javascript
    addPilotState = { system: '', fccUnlocked: false, bandwidthMHz: 0, analogBands: ['R'] };
```

**Step 9: Commit**

```
feat(ui): analog band selector in leader add-pilot dialog
```

---

### Task 9: Build, manual test, and bump service worker cache

**Files:**
- Modify: `static/app.js` (service worker CACHE_NAME)

**Step 1: Bump service worker cache version**

Search for `CACHE_NAME` in `static/app.js` and change `skwad-v5` to `skwad-v6`.

**Step 2: Build and deploy to dev**

Run: `cd /home/kyleg/containers/atxfpv.org && docker compose build skwad && docker compose up -d skwad`

**Step 3: Manual test checklist**

Test on `skwad.atxfpv.hippienet.wtf`:

1. Create session
2. Join as analog — verify band selector shows with R pre-selected
3. Select R+F, join — verify assigned frequency could be from either band
4. Check channel change picker shows R+F channels
5. As leader, add analog pilot — verify band selector appears
6. Add analog pilot with just R — verify race band only
7. Check existing digital pilots (DJI, HDZero) unaffected
8. Rebalance all — verify no errors

**Step 4: Commit**

```
chore: bump service worker cache to v6 for analog bands release
```

---

### Task 10: Final commit and version bump

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `static/changelog.html`

**Step 1: Update developer changelog**

Add entry to `CHANGELOG.md` under a new version heading.

**Step 2: Update user-facing changelog**

Add entry to `static/changelog.html` describing the new analog band options.

**Step 3: Commit**

```
docs: update changelogs for expanded analog bands feature
```

Plan complete and saved to `docs/plans/2026-03-09-expanded-analog-bands-plan.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?
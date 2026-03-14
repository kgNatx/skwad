# Fixed Channels Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let session leaders select a fixed set of channels (2-5 unique frequencies), restricting the optimizer to assign only within that set and buddying up overflow pilots.

**Architecture:** Store the fixed channel set as a JSON array of `{name, freq}` pairs on the session. The optimizer filters each pilot's channel pool to only include frequencies in the fixed set. If no overlap exists (e.g., analog pilot in a DJI-only set), the pilot gets the full fixed set as their pool — the optimizer will pick the best-spaced frequency and buddy them up. The leader wizard gets session option checkboxes that gate the power ceiling and fixed channels steps. Joiners see the fixed set and are restricted to those channels in the preference picker.

**Tech Stack:** Go backend, vanilla JS frontend, SQLite

**Reference:** `docs/channel-set-analysis.md` for preset data and analysis.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `db/db.go` | Modify | Add `fixed_channels` TEXT column to sessions, update Session struct, CreateSession, GetSession |
| `freq/optimizer.go` | Modify | Accept optional `fixedFreqs []int` parameter on Optimize, filter channel pools |
| `freq/optimizer_test.go` | Modify | Add tests for fixed channel constraint |
| `api/handlers.go` | Modify | Thread fixed channels from session to optimizer in all handlers, accept in CreateSession |
| `static/index.html` | Modify | Add session options to leader info step, add fixed channels wizard step |
| `static/style.css` | Modify | Styles for session options, fixed channels step, channel set cards |
| `static/app.js` | Modify | Session options toggle logic, wizard routing, fixed channels step interaction, joiner channel restriction |
| `static/sw.js` | Modify | Bump cache version |
| `CHANGELOG.md` | Modify | Developer changelog |
| `static/changelog.html` | Modify | User-facing changelog |

---

## Chunk 1: Backend — DB, Optimizer, and API

### Task 1: Add fixed_channels to the database

**Files:**
- Modify: `db/db.go`

- [ ] **Step 1: Add FixedChannels field to Session struct**

In `db/db.go`, add a `FixedChannels` field to the Session struct (after `PowerCeilingMW`):

```go
type Session struct {
	ID             string    `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Version        int       `json:"version"`
	LeaderPilotID  int       `json:"leader_pilot_id"`
	PowerCeilingMW int       `json:"power_ceiling_mw"`
	FixedChannels  string    `json:"fixed_channels"` // JSON array of {name, freq} or empty
}
```

- [ ] **Step 2: Add migration**

In the `migrate()` function, after the `power_ceiling_mw` migration, add:

```go
_, err = d.db.Exec(`ALTER TABLE sessions ADD COLUMN fixed_channels TEXT DEFAULT ''`)
if err != nil && !strings.Contains(err.Error(), "duplicate column") {
	return nil, fmt.Errorf("migrate fixed_channels: %w", err)
}
```

- [ ] **Step 3: Update CreateSession to accept fixedChannels**

Change `CreateSession` signature:

```go
func (d *DB) CreateSession(powerCeilingMW int, fixedChannels string) (*Session, error) {
```

Add `fixed_channels` to the INSERT statement. Update the returned Session struct to include it.

- [ ] **Step 4: Update GetSession to read fixed_channels**

Add `COALESCE(fixed_channels, '')` to the SELECT in GetSession and scan it into `sess.FixedChannels`.

- [ ] **Step 5: Fix all existing CreateSession callers**

Search for `CreateSession(` in `api/handlers.go` and test files. Update each call to pass `""` as the second argument (preserves existing behavior).

Run: `grep -rn "CreateSession(" --include="*.go"`

- [ ] **Step 6: Run all tests**

Run: `cd /home/kyleg/containers/atxfpv.org/skwad && /home/kyleg/.local/go/bin/go test ./... -v`

- [ ] **Step 7: Commit**

```bash
git add db/db.go api/handlers.go
git commit -m "feat: add fixed_channels column to sessions table"
```

### Task 2: Add fixed channel constraint to the optimizer

**Files:**
- Modify: `freq/optimizer.go`
- Modify: `freq/optimizer_test.go`

The approach: `Optimize()` accepts an optional `fixedFreqs []int`. When non-nil and non-empty, each pilot's channel pool is filtered to only include channels whose frequency is in the fixed set. If the intersection is empty (pilot's video system has no channels matching the fixed set), the pilot gets a synthetic pool built from the fixed frequencies with generic channel names.

- [ ] **Step 1: Write failing test**

Add to `freq/optimizer_test.go`:

```go
func TestOptimize_FixedChannels(t *testing.T) {
	// 4 pilots, fixed to R1, R3, R6, R8
	fixedFreqs := []int{5658, 5732, 5843, 5917}
	inputs := []PilotInput{
		{ID: 1, VideoSystem: "analog", AnalogBands: []string{"R"}},
		{ID: 2, VideoSystem: "analog", AnalogBands: []string{"R"}},
		{ID: 3, VideoSystem: "analog", AnalogBands: []string{"R"}},
		{ID: 4, VideoSystem: "analog", AnalogBands: []string{"R"}},
	}
	result := Optimize(inputs, DefaultGuardBandMHz, fixedFreqs)
	if len(result) != 4 {
		t.Fatalf("expected 4 assignments, got %d", len(result))
	}
	// All assignments must be in the fixed set
	fixedSet := map[int]bool{5658: true, 5732: true, 5843: true, 5917: true}
	for _, a := range result {
		if !fixedSet[a.FreqMHz] {
			t.Errorf("pilot %d assigned to %d, not in fixed set", a.PilotID, a.FreqMHz)
		}
	}
	// All 4 unique frequencies should be used (no unnecessary buddying)
	usedFreqs := map[int]bool{}
	for _, a := range result {
		usedFreqs[a.FreqMHz] = true
	}
	if len(usedFreqs) != 4 {
		t.Errorf("expected 4 unique frequencies, got %d", len(usedFreqs))
	}
}

func TestOptimize_FixedChannels_BuddyOverflow(t *testing.T) {
	// 6 pilots, only 4 fixed channels — 2 must buddy up
	fixedFreqs := []int{5658, 5732, 5843, 5917}
	inputs := make([]PilotInput, 6)
	for i := range inputs {
		inputs[i] = PilotInput{ID: i + 1, VideoSystem: "analog", AnalogBands: []string{"R"}}
	}
	result := Optimize(inputs, DefaultGuardBandMHz, fixedFreqs)
	// All assignments must be in the fixed set
	fixedSet := map[int]bool{5658: true, 5732: true, 5843: true, 5917: true}
	for _, a := range result {
		if !fixedSet[a.FreqMHz] {
			t.Errorf("pilot %d assigned to %d, not in fixed set", a.PilotID, a.FreqMHz)
		}
	}
	// Should have buddy groups (some frequencies shared)
	hasBuddy := false
	for _, a := range result {
		if a.BuddyGroup > 0 {
			hasBuddy = true
			break
		}
	}
	if !hasBuddy {
		t.Error("expected buddy groups with 6 pilots on 4 channels")
	}
}

func TestOptimize_FixedChannels_Nil(t *testing.T) {
	// nil fixedFreqs = no constraint (backward compatible)
	inputs := []PilotInput{
		{ID: 1, VideoSystem: "analog", AnalogBands: []string{"R"}},
		{ID: 2, VideoSystem: "analog", AnalogBands: []string{"R"}},
	}
	result := Optimize(inputs, DefaultGuardBandMHz, nil)
	if len(result) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(result))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/kyleg/containers/atxfpv.org/skwad && /home/kyleg/.local/go/bin/go test ./freq/ -run TestOptimize_Fixed -v`
Expected: FAIL — `Optimize` takes wrong number of arguments

- [ ] **Step 3: Update Optimize signature**

Change `Optimize` to accept `fixedFreqs`:

```go
func Optimize(pilots []PilotInput, guardBandMHz int, fixedFreqs []int) []Assignment {
```

Add pool filtering after the pool-building loop (after line 52):

```go
	// If fixed channels are set, filter each pilot's pool to only fixed frequencies.
	if len(fixedFreqs) > 0 {
		fixedSet := make(map[int]bool, len(fixedFreqs))
		for _, f := range fixedFreqs {
			fixedSet[f] = true
		}
		for id, pool := range pools {
			var filtered []Channel
			for _, ch := range pool {
				if fixedSet[ch.FreqMHz] {
					filtered = append(filtered, ch)
				}
			}
			if len(filtered) == 0 {
				// Pilot's system has no channels in the fixed set — give them
				// the fixed frequencies directly so they buddy up on the closest match.
				for _, f := range fixedFreqs {
					filtered = append(filtered, Channel{Name: fmt.Sprintf("CH-%d", f), FreqMHz: f})
				}
			}
			pools[id] = filtered
		}
	}
```

- [ ] **Step 4: Update all existing Optimize callers**

Every call to `Optimize(inputs, guardBand)` becomes `Optimize(inputs, guardBand, nil)`.

Search: `grep -rn "Optimize(" --include="*.go" | grep -v "_test.go" | grep -v "func Optimize"`

Update each call in `freq/optimizer.go` (OptimizeWithLocks, internal callers) and `api/handlers.go`.

- [ ] **Step 5: Update all existing test calls**

Every test call to `Optimize(inputs, ...)` needs `nil` as the third argument.

Search: `grep -rn "Optimize(" freq/optimizer_test.go`

- [ ] **Step 6: Run all tests**

Run: `cd /home/kyleg/containers/atxfpv.org/skwad && /home/kyleg/.local/go/bin/go test ./... -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add freq/optimizer.go freq/optimizer_test.go api/handlers.go
git commit -m "feat: add fixed channel constraint to optimizer"
```

### Task 3: Thread fixed channels through API handlers

**Files:**
- Modify: `api/handlers.go`

- [ ] **Step 1: Parse fixed channels in HandleCreateSession**

Update the request struct and pass to CreateSession:

```go
func (s *Server) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PowerCeilingMW int    `json:"power_ceiling_mw"`
		FixedChannels  string `json:"fixed_channels"` // JSON array or empty
	}
	json.NewDecoder(r.Body).Decode(&req)

	sess, err := s.DB.CreateSession(req.PowerCeilingMW, req.FixedChannels)
```

- [ ] **Step 2: Add helper to parse fixed channels to frequency list**

Add a helper function in `api/handlers.go`:

```go
func parseFixedFreqs(fixedChannels string) []int {
	if fixedChannels == "" {
		return nil
	}
	var channels []struct {
		Name string `json:"name"`
		Freq int    `json:"freq"`
	}
	if err := json.Unmarshal([]byte(fixedChannels), &channels); err != nil {
		return nil
	}
	freqs := make([]int, len(channels))
	for i, ch := range channels {
		freqs[i] = ch.Freq
	}
	return freqs
}
```

- [ ] **Step 3: Thread fixedFreqs through all optimizer calls**

In every handler that calls `freq.Optimize`, `freq.OptimizeWithLocks`, or `freq.FindMinimalDisplacement`:

```go
fixedFreqs := parseFixedFreqs(sess.FixedChannels)
```

Then pass `fixedFreqs` to the optimizer calls.

Handlers to update:
- `HandleGetSession` — passes to `DetectConflicts` (no change needed, conflicts don't use pools)
- `HandleJoinSession` — passes to `FindMinimalDisplacement`
- `HandlePreviewJoin` — passes to `FindMinimalDisplacement`
- `HandlePreviewChannelChange` — passes to optimizer
- `HandleUpdatePilotChannel` — passes to optimizer
- `HandleRebalanceAll` — passes to `reoptimize`
- `HandlePreviewRebalance` — passes to optimizer
- `HandleAddPilot` — passes to `FindMinimalDisplacement`
- `reoptimize()` helper — add `fixedFreqs []int` parameter, pass through

Note: `FindMinimalDisplacement` and `OptimizeWithLocks` also need the `fixedFreqs` parameter threaded through. Update their signatures in `freq/optimizer.go` the same way as `Optimize`.

- [ ] **Step 4: Include fixed_channels in GetSession response**

The Session struct already has `json:"fixed_channels"` — verify it appears in the JSON response. The frontend will read `data.session.fixed_channels` to know the set.

- [ ] **Step 5: Run all tests**

Run: `cd /home/kyleg/containers/atxfpv.org/skwad && /home/kyleg/.local/go/bin/go test ./... -v`

- [ ] **Step 6: Commit**

```bash
git add api/handlers.go freq/optimizer.go
git commit -m "feat: thread fixed channels through API handlers to optimizer"
```

---

## Chunk 2: Frontend — Leader Wizard Flow

### Task 4: Add session options to leader info step

**Files:**
- Modify: `static/index.html`
- Modify: `static/style.css`
- Modify: `static/app.js`

- [ ] **Step 1: Add session options HTML**

In `static/index.html`, in the `step-leader-info` div, after the `leader-info-body` div and before the GOT IT button, add:

```html
<div class="session-options">
  <div class="session-options-label">SESSION OPTIONS</div>
  <button id="opt-power-ceiling" class="session-option-btn" data-option="power">
    <span class="session-option-check">&#x2610;</span>
    <span class="session-option-text">POWER CEILING</span>
  </button>
  <button id="opt-fixed-channels" class="session-option-btn" data-option="channels">
    <span class="session-option-check">&#x2610;</span>
    <span class="session-option-text">FIXED CHANNELS</span>
  </button>
</div>
```

- [ ] **Step 2: Add CSS**

Add to `static/style.css` after the `.leader-info-item strong` rule:

```css
.session-options { margin-top: 24px; }
.session-options-label { font-size: 11px; font-weight: 800; color: #666; letter-spacing: 1.5px; margin-bottom: 8px; }
.session-option-btn { display: flex; align-items: center; gap: 10px; width: 100%; padding: 12px 14px; margin-bottom: 6px; background: #151515; border: 1px solid #333; border-radius: 10px; color: #aaa; font-size: 14px; font-weight: 700; letter-spacing: 0.5px; cursor: pointer; text-align: left; }
.session-option-btn.active { border-color: #33ff33; background: rgba(51,255,51,0.04); color: #fff; }
.session-option-check { font-size: 18px; line-height: 1; }
.session-option-btn.active .session-option-check { color: #33ff33; }
```

- [ ] **Step 3: Wire up toggle behavior**

In `static/app.js`, add state variables:

```javascript
state.optPowerCeiling = false;
state.optFixedChannels = false;
```

Add click handlers for the option buttons — toggle active class and update state:

```javascript
$('opt-power-ceiling').addEventListener('click', function () {
  state.optPowerCeiling = !state.optPowerCeiling;
  this.classList.toggle('active', state.optPowerCeiling);
  this.querySelector('.session-option-check').textContent = state.optPowerCeiling ? '\u2611' : '\u2610';
});
$('opt-fixed-channels').addEventListener('click', function () {
  state.optFixedChannels = !state.optFixedChannels;
  this.classList.toggle('active', state.optFixedChannels);
  this.querySelector('.session-option-check').textContent = state.optFixedChannels ? '\u2611' : '\u2610';
});
```

- [ ] **Step 4: Update GOT IT routing**

Change the GOT IT click handler from `showStep('step-power')` to route through checked options:

```javascript
$('btn-leader-info-got-it').addEventListener('click', function () {
  if (state.optPowerCeiling) {
    showStep('step-power');
  } else if (state.optFixedChannels) {
    showStep('step-fixed-channels');
  } else {
    showStep('step-video');
  }
});
```

- [ ] **Step 5: Update power step flow**

The power step's SET CEILING and SKIP buttons currently go to `step-video`. Update them to check if fixed channels is also selected:

```javascript
// After power step completes (both SET CEILING and SKIP handlers):
if (state.optFixedChannels) {
  showStep('step-fixed-channels');
} else {
  showStep('step-video');
}
```

- [ ] **Step 6: Reset options on new session**

In the START SESSION handler, reset the options:

```javascript
state.optPowerCeiling = false;
state.optFixedChannels = false;
```

And reset the button states visually when showing the leader info step.

- [ ] **Step 7: Commit**

```bash
git add static/index.html static/style.css static/app.js
git commit -m "feat: add session options to leader info step with wizard routing"
```

### Task 5: Add fixed channels wizard step

**Files:**
- Modify: `static/index.html`
- Modify: `static/style.css`
- Modify: `static/app.js`

This step uses the design from the mockup at `static/mockup-channel-sets.html`. It has a pilot count slider (2-5), channel set cards ranked by spacing, and a USE THIS SET button.

- [ ] **Step 1: Add HTML for the fixed channels step**

In `static/index.html`, after the `step-power` div and before `step-video`, add a new step:

```html
<!-- Step 1.85: Fixed Channels (only for session creators who opted in) -->
<div id="step-fixed-channels" class="setup-step hidden">
  <h2 class="step-title">FIXED CHANNELS</h2>
  <p class="power-subtitle">Wider spacing for higher power and cleaner video. Extra pilots buddy up on shared channels.</p>
  <div class="fc-pilot-display">
    <span class="fc-pilot-count" id="fc-pilot-count">4</span>
    <span class="fc-pilot-label">UNIQUE CHANNELS</span>
  </div>
  <div class="fc-slider-container">
    <div class="fc-slider-track" id="fc-slider-track">
      <div class="fc-slider-notches" id="fc-slider-notches"></div>
      <div class="power-slider-thumb" id="fc-slider-thumb"></div>
    </div>
    <div class="fc-slider-labels">
      <span>2</span>
      <span>5</span>
    </div>
  </div>
  <div class="fc-set-list" id="fc-set-list"></div>
  <p class="fc-hint">For 6+ pilots, skip this and let the optimizer assign channels.</p>
  <button id="btn-fc-use-set" class="btn btn-primary btn-large" disabled>USE THIS SET</button>
  <button id="btn-fc-skip" class="btn btn-secondary btn-large">SKIP</button>
</div>
```

- [ ] **Step 2: Add CSS for the fixed channels step**

Add styles matching the mockup. Key classes: `.fc-pilot-display`, `.fc-pilot-count`, `.fc-slider-*`, `.fc-set-list`, `.fc-set-card`, `.fc-set-header`, `.fc-set-badge`, `.ch-pill`, `.ch-pill.ch-dji`, `.fc-hint`. Use the existing power slider thumb class (`.power-slider-thumb`) for the slider thumb.

- [ ] **Step 3: Add channel set data to app.js**

Add the `FIXED_CHANNEL_SETS` constant near the top of `app.js` (after `POWER_STEPS`):

```javascript
var FIXED_CHANNEL_SETS = {
  2: [
    { name: 'MAX SPREAD', channels: [{ n: 'R1', f: 5658 }, { n: 'R8', f: 5917 }], spacing: 259, imd: 100, power: 'ANY POWER', systems: 'ANALOG / HDZERO', powerColor: '#4ade80' },
    { name: 'DJI SPREAD', channels: [{ n: 'DJI-CH1', f: 5669, dji: true }, { n: 'DJI-CH7', f: 5876, dji: true }], spacing: 207, imd: 100, power: 'ANY POWER', systems: 'DJI', powerColor: '#4ade80' }
  ],
  3: [
    { name: 'IMD CLEAN', channels: [{ n: 'R1', f: 5658 }, { n: 'R4', f: 5769 }, { n: 'R8', f: 5917 }], spacing: 111, imd: 100, power: 'ANY POWER', systems: 'ANALOG / HDZERO', powerColor: '#4ade80' },
    { name: 'MIXED CLEAN', channels: [{ n: 'R1', f: 5658 }, { n: 'DJI-CH5', f: 5805, dji: true }, { n: 'R8', f: 5917 }], spacing: 112, imd: 100, power: 'ANY POWER', systems: 'MIXED', powerColor: '#4ade80' },
    { name: 'DJI SPREAD', channels: [{ n: 'DJI-CH1', f: 5669, dji: true }, { n: 'DJI-CH4', f: 5769, dji: true }, { n: 'DJI-CH7', f: 5876, dji: true }], spacing: 100, imd: 78, power: 'ANY POWER', systems: 'DJI', powerColor: '#4ade80' }
  ],
  4: [
    { name: 'IMD CLEAN', channels: [{ n: 'R1', f: 5658 }, { n: 'R3', f: 5732 }, { n: 'R6', f: 5843 }, { n: 'R8', f: 5917 }], spacing: 74, imd: 100, power: 'ANY POWER', systems: 'ANALOG / HDZERO', powerColor: '#4ade80' },
    { name: 'MIXED CLEAN', channels: [{ n: 'R1', f: 5658 }, { n: 'DJI-CH3', f: 5741, dji: true }, { n: 'DJI-CH6', f: 5840, dji: true }, { n: 'R8', f: 5917 }], spacing: 77, imd: 98, power: 'ANY POWER', systems: 'MIXED', powerColor: '#4ade80' },
    { name: 'DJI SPREAD', channels: [{ n: 'DJI-CH1', f: 5669, dji: true }, { n: 'DJI-CH3', f: 5741, dji: true }, { n: 'DJI-CH5', f: 5805, dji: true }, { n: 'DJI-CH7', f: 5876, dji: true }], spacing: 64, imd: 69, power: 'ANY POWER', systems: 'DJI', powerColor: '#4ade80' }
  ],
  5: [
    { name: 'MIXED OPTIMAL', channels: [{ n: 'R1', f: 5658 }, { n: 'DJI-CH2', f: 5705, dji: true }, { n: 'R4', f: 5769 }, { n: 'R6', f: 5843 }, { n: 'R8', f: 5917 }], spacing: 47, imd: 91, power: '\u2264 600 mW', systems: 'MIXED \u2014 BEST AT 5', powerColor: '#60a5fa' },
    { name: 'ET5A', channels: [{ n: 'E3', f: 5665, multi: true }, { n: 'F1', f: 5740, multi: true }, { n: 'F4', f: 5800, multi: true }, { n: 'F7', f: 5860, multi: true }, { n: 'E6', f: 5905, multi: true }], spacing: 45, imd: 88, power: '\u2264 600 mW', systems: 'ANALOG (MULTI-BAND)', powerColor: '#60a5fa' },
    { name: 'RACEBAND 5', channels: [{ n: 'R1', f: 5658 }, { n: 'R3', f: 5732 }, { n: 'R5', f: 5806 }, { n: 'R6', f: 5843 }, { n: 'R8', f: 5917 }], spacing: 37, imd: 40, power: '\u2264 400 mW', systems: 'ANALOG / HDZERO', powerColor: '#f59e0b' },
    { name: 'DJI 5', channels: [{ n: 'DJI-CH1', f: 5669, dji: true }, { n: 'DJI-CH3', f: 5741, dji: true }, { n: 'DJI-CH5', f: 5805, dji: true }, { n: 'DJI-CH6', f: 5840, dji: true }, { n: 'DJI-CH7', f: 5876, dji: true }], spacing: 36, imd: 100, power: '\u2264 400 mW', systems: 'DJI', powerColor: '#f59e0b' }
  ]
};
```

- [ ] **Step 4: Add fixed channels step initialization and interaction**

Add `initFixedChannelsStep()` function with:
- Build slider notches (4 positions for 2-5)
- Slider drag interaction (same pattern as power step)
- `renderFixedChannelSets(count)` function that builds cards from `FIXED_CHANNEL_SETS`
- Each card shows: name, IMD badge, power badge, system label, channel pills (with DJI pills in blue), mini spectrum canvas
- Card click selects it, enables USE THIS SET button
- Mini spectrum rendering function (bezier humps matching app style)

- [ ] **Step 5: Wire up buttons**

USE THIS SET: store selected set in `state.fixedChannels` as JSON string, proceed to `step-video`.

```javascript
$('btn-fc-use-set').addEventListener('click', function () {
  state.fixedChannels = JSON.stringify(selectedFixedSet.channels.map(function (c) {
    return { name: c.n, freq: c.f };
  }));
  showStep('step-video');
});
```

SKIP: set `state.fixedChannels = ''`, proceed to `step-video`.

- [ ] **Step 6: Update session creation to send fixed_channels**

In `createSessionWithPower()`, include fixed channels in the API call:

```javascript
async function createSessionWithPower(powerCeilingMW) {
  var body = {};
  if (powerCeilingMW > 0) body.power_ceiling_mw = powerCeilingMW;
  if (state.fixedChannels) body.fixed_channels = state.fixedChannels;
  var sess = await apiPost('/api/sessions', body);
  state.sessionCode = sess.id;
  state.powerCeilingMW = powerCeilingMW;
  saveState();
}
```

- [ ] **Step 7: Commit**

```bash
git add static/index.html static/style.css static/app.js
git commit -m "feat: fixed channels wizard step with channel set presets"
```

---

## Chunk 3: Frontend — Joiner Flow and Session Display

### Task 6: Restrict joiner channel selection to fixed set

**Files:**
- Modify: `static/app.js`

- [ ] **Step 1: Store fixed channels from session data**

When joining a session (in `handleJoinByCode` and `route()`), store the fixed channels:

```javascript
state.sessionFixedChannels = data.session.fixed_channels || '';
```

Also in `enterSessionView` / `refreshSession`, capture it:

```javascript
state.sessionFixedChannels = data.session.fixed_channels || '';
```

- [ ] **Step 2: Show fixed channels info on join interstitial**

After the power ceiling alert (or in place of it if no power ceiling), show the fixed channel set to the joiner. Add a new interstitial step or a note on the existing flow showing which channels are available.

In the callsign step's NEXT handler, after the power ceiling check:

```javascript
if (state.sessionFixedChannels) {
  // Show fixed channels info to joiner
  var channels = JSON.parse(state.sessionFixedChannels);
  // Populate a display element showing the channel names
}
```

- [ ] **Step 3: Filter channel picker to fixed set**

In `renderChannelPicker()`, when `state.sessionFixedChannels` is set, filter the pool:

```javascript
function renderChannelPicker() {
  var pool = getChannelPool();
  // If session has fixed channels, restrict to those frequencies
  if (state.sessionFixedChannels) {
    var fixedChannels = JSON.parse(state.sessionFixedChannels);
    var fixedFreqs = {};
    fixedChannels.forEach(function (c) { fixedFreqs[c.freq] = c.name; });
    pool = pool.filter(function (ch) { return fixedFreqs[ch.freq]; });
    // If pilot's system has no overlap, show fixed channels directly
    if (pool.length === 0) {
      pool = fixedChannels.map(function (c) { return { name: c.name, freq: c.freq }; });
    }
  }
  // ... rest of existing picker code
}
```

Apply the same filter in `showChannelChange()` and `showChannelChangeForPilot()`.

- [ ] **Step 4: Show fixed channels badge on session view**

Add a badge similar to the power ceiling badge showing the fixed set is active:

```javascript
if (state.sessionFixedChannels) {
  var channels = JSON.parse(state.sessionFixedChannels);
  // Show "FIXED · N CH" badge
}
```

- [ ] **Step 5: Commit**

```bash
git add static/app.js static/index.html static/style.css
git commit -m "feat: restrict joiner channel selection to fixed set"
```

### Task 7: Release housekeeping

**Files:**
- Modify: `static/sw.js`
- Modify: `CHANGELOG.md`
- Modify: `static/changelog.html`

- [ ] **Step 1: Bump service worker**

Change `CACHE_NAME` to next version.

- [ ] **Step 2: Update CHANGELOG.md**

Add v0.6.0 entry with fixed channels feature description.

- [ ] **Step 3: Update static/changelog.html**

Add user-facing v0.6.0 entry.

- [ ] **Step 4: Commit**

```bash
git add static/sw.js CHANGELOG.md static/changelog.html
git commit -m "release: v0.6.0 — fixed channels"
```

---

## Execution Notes

**Build and test after each chunk:**

```bash
cd /home/kyleg/containers/atxfpv.org/skwad
/home/kyleg/.local/go/bin/go test ./... -v    # backend tests
cd /home/kyleg/containers/atxfpv.org && docker compose build skwad && docker compose up -d skwad  # deploy to dev
```

**Manual testing checklist:**

1. Create session with no options checked → wizard goes callsign → leader info → video system (fast path)
2. Create session with power ceiling checked → wizard goes callsign → leader info → power → video system
3. Create session with fixed channels checked → wizard goes callsign → leader info → fixed channels → video system
4. Create session with both checked → wizard goes callsign → leader info → power → fixed channels → video system
5. Select a 4-pilot channel set → verify session stores fixed_channels
6. Join a fixed-channel session → verify channel picker only shows fixed frequencies
7. Add more pilots than fixed channels → verify buddying works correctly
8. Join as DJI pilot in a mixed set → verify DJI channels from the set are available
9. Join as analog pilot in a DJI-only set → verify they get the fixed frequencies as options
10. Rebalance in a fixed-channel session → verify optimizer stays within the fixed set
11. Channel change in a fixed-channel session → verify picker is restricted

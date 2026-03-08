package freq

import "sort"

// PilotInput is what the optimizer needs from each pilot.
type PilotInput struct {
	ID            int
	VideoSystem   string
	FCCUnlocked   bool
	BandwidthMHz  int
	RaceMode      bool
	Goggles       string
	ChannelLocked bool
	LockedFreqMHz int
	PrevChannel   string // Previous assignment for stability
	PrevFreqMHz   int
}

// Assignment is the optimizer's output for one pilot.
type Assignment struct {
	PilotID      int    `json:"pilot_id"`
	Channel      string `json:"channel"`
	FreqMHz      int    `json:"freq_mhz"`
	BandwidthMHz int    `json:"bandwidth_mhz"`
	BuddyGroup   int    `json:"buddy_group"`
}

// usedEntry tracks a frequency assignment with its bandwidth.
type usedEntry struct {
	freqMHz      int
	bandwidthMHz int
	pilotID      int
}

// Optimize takes a list of pilots and assigns each one a channel that
// maximizes frequency separation accounting for signal bandwidth.
// When channels must be shared, it creates buddy groups.
func Optimize(pilots []PilotInput) []Assignment {
	// Result map keyed by pilot ID for order preservation.
	results := make(map[int]Assignment, len(pilots))

	// Build channel pools and compute occupied bandwidth for each pilot.
	pools := make(map[int][]Channel, len(pilots))
	pilotBW := make(map[int]int, len(pilots))
	for _, p := range pilots {
		pools[p.ID] = ChannelPool(p.VideoSystem, p.FCCUnlocked, p.BandwidthMHz, p.RaceMode, p.Goggles)
		pilotBW[p.ID] = OccupiedBandwidth(p.VideoSystem, p.BandwidthMHz)
	}

	// Separate locked vs flexible pilots.
	var locked, flexible []PilotInput
	for _, p := range pilots {
		if p.ChannelLocked && p.LockedFreqMHz > 0 {
			locked = append(locked, p)
		} else {
			flexible = append(flexible, p)
		}
	}

	// Sort flexible pilots by pool size ascending (most constrained first).
	sort.SliceStable(flexible, func(i, j int) bool {
		return len(pools[flexible[i].ID]) < len(pools[flexible[j].ID])
	})

	// used tracks all assigned frequencies with their bandwidths.
	var used []usedEntry

	// Step 1: Lock in fixed-channel pilots.
	for _, p := range locked {
		bw := pilotBW[p.ID]
		chName := findChannelName(pools[p.ID], p.LockedFreqMHz)
		if chName == "" {
			// Locked frequency not in pool; use it anyway with a generic name.
			chName = "LOCKED"
		}
		results[p.ID] = Assignment{
			PilotID:      p.ID,
			Channel:      chName,
			FreqMHz:      p.LockedFreqMHz,
			BandwidthMHz: bw,
		}
		used = append(used, usedEntry{freqMHz: p.LockedFreqMHz, bandwidthMHz: bw, pilotID: p.ID})
	}

	// Step 2: Assign each flexible pilot the channel that maximizes
	// the effective separation margin from all already-used frequencies.
	for _, p := range flexible {
		pool := pools[p.ID]
		bw := pilotBW[p.ID]
		bestCh := pool[0]
		bestMargin := -(1 << 30)

		for _, ch := range pool {
			margin := effectiveSeparation(ch.FreqMHz, bw, used)

			// Prefer previous frequency for stability when margin >= 0.
			if ch.FreqMHz == p.PrevFreqMHz && margin >= 0 {
				if margin >= bestMargin {
					bestCh = ch
					bestMargin = margin
					continue
				}
			}

			if margin > bestMargin {
				bestCh = ch
				bestMargin = margin
			}
		}

		results[p.ID] = Assignment{
			PilotID:      p.ID,
			Channel:      bestCh.Name,
			FreqMHz:      bestCh.FreqMHz,
			BandwidthMHz: bw,
		}
		used = append(used, usedEntry{freqMHz: bestCh.FreqMHz, bandwidthMHz: bw, pilotID: p.ID})
	}

	// Step 3: Identify buddy groups — frequencies shared by multiple pilots.
	buddyGroupID := 1
	freqCount := make(map[int][]int)
	for _, u := range used {
		freqCount[u.freqMHz] = append(freqCount[u.freqMHz], u.pilotID)
	}
	freqToGroup := make(map[int]int)
	for freq, ids := range freqCount {
		if len(ids) > 1 {
			freqToGroup[freq] = buddyGroupID
			buddyGroupID++
		}
	}

	// Apply buddy groups to assignments.
	for id, a := range results {
		if gid, ok := freqToGroup[a.FreqMHz]; ok {
			a.BuddyGroup = gid
			results[id] = a
		}
	}

	// Return assignments in same order as input pilots.
	out := make([]Assignment, len(pilots))
	for i, p := range pilots {
		out[i] = results[p.ID]
	}
	return out
}

// OptimizeWithLocks runs the optimizer but forces pilots in lockedIDs to be
// channel-locked at their PrevFreqMHz, regardless of their actual preference.
// This preserves existing assignments while allowing unlocked pilots to move.
func OptimizeWithLocks(pilots []PilotInput, lockedIDs map[int]bool) []Assignment {
	modified := make([]PilotInput, len(pilots))
	for i, p := range pilots {
		modified[i] = p
		if lockedIDs[p.ID] {
			modified[i].ChannelLocked = true
			modified[i].LockedFreqMHz = p.PrevFreqMHz
		}
	}
	return Optimize(modified)
}

// effectiveSeparation returns the worst-case margin between a candidate
// frequency (with given bandwidth) and all used entries. The margin is
// the actual center-to-center separation minus the required spacing.
// Negative values indicate overlap or insufficient guard band.
// Returns a large positive number if no frequencies are in use.
func effectiveSeparation(freq, bw int, used []usedEntry) int {
	if len(used) == 0 {
		return 1<<31 - 1 // MaxInt
	}
	worstMargin := 1<<31 - 1
	for _, u := range used {
		d := freq - u.freqMHz
		if d < 0 {
			d = -d
		}
		required := RequiredSpacing(bw, u.bandwidthMHz)
		margin := d - required
		if margin < worstMargin {
			worstMargin = margin
		}
	}
	return worstMargin
}

// ConflictLevel indicates the severity of a frequency conflict.
type ConflictLevel string

const (
	ConflictWarning ConflictLevel = "warning"
	ConflictDanger  ConflictLevel = "danger"
)

// Conflict describes a frequency conflict between two pilots.
type Conflict struct {
	PilotA        int           `json:"pilot_a"`
	PilotB        int           `json:"pilot_b"`
	Level         ConflictLevel `json:"level"`
	SeparationMHz int           `json:"separation_mhz"`
	RequiredMHz   int           `json:"required_mhz"`
}

// DetectConflicts checks all pairs of assignments for frequency conflicts.
// A "danger" conflict means the signals actually overlap (center separation
// is less than the sum of half-bandwidths). A "warning" conflict means the
// separation is less than the required spacing (which includes the guard band)
// but the signals don't overlap.
func DetectConflicts(assignments []Assignment) []Conflict {
	var conflicts []Conflict
	for i := 0; i < len(assignments); i++ {
		for j := i + 1; j < len(assignments); j++ {
			a, b := assignments[i], assignments[j]
			sep := a.FreqMHz - b.FreqMHz
			if sep < 0 {
				sep = -sep
			}

			halfA := a.BandwidthMHz / 2
			halfB := b.BandwidthMHz / 2
			overlapThreshold := halfA + halfB
			required := RequiredSpacing(a.BandwidthMHz, b.BandwidthMHz)

			if sep < overlapThreshold {
				conflicts = append(conflicts, Conflict{
					PilotA:        a.PilotID,
					PilotB:        b.PilotID,
					Level:         ConflictDanger,
					SeparationMHz: sep,
					RequiredMHz:   required,
				})
			} else if sep < required {
				conflicts = append(conflicts, Conflict{
					PilotA:        a.PilotID,
					PilotB:        b.PilotID,
					Level:         ConflictWarning,
					SeparationMHz: sep,
					RequiredMHz:   required,
				})
			}
		}
	}
	return conflicts
}

// BuddySuggestion recommends sharing a frequency with an existing pilot.
type BuddySuggestion struct {
	PilotID int    `json:"pilot_id"`
	Channel string `json:"channel"`
	FreqMHz int    `json:"freq_mhz"`
}

// DisplacementResult is the output of FindMinimalDisplacement.
type DisplacementResult struct {
	Level           int              // 0-3
	Assignments     []Assignment     // full assignment set
	BuddySuggestion *BuddySuggestion // non-nil only at Level 3
}

// FindMinimalDisplacement tries progressively more disruptive strategies to
// place newPilot into an existing session. Stops at the first level that
// produces no danger-level conflicts involving any moved pilot.
//
//	Level 0: lock all existing, slot new pilot only
//	Level 1: unlock one existing pilot at a time
//	Level 2: unlock pairs of existing pilots
//	Level 3: suggest buddying up with an existing pilot
func FindMinimalDisplacement(existing []PilotInput, newPilot PilotInput) DisplacementResult {
	all := make([]PilotInput, len(existing)+1)
	copy(all, existing)
	all[len(existing)] = newPilot

	// Build the set of all existing pilot IDs.
	allExistingIDs := make(map[int]bool, len(existing))
	for _, p := range existing {
		allExistingIDs[p.ID] = true
	}

	// Level 0: lock all existing pilots, only new pilot is flexible.
	assignments := OptimizeWithLocks(all, allExistingIDs)
	movedIDs := movedPilotIDs(existing, assignments)
	movedIDs[newPilot.ID] = true // new pilot is always "moved"
	if !hasDangerInvolving(assignments, movedIDs) {
		return DisplacementResult{Level: 0, Assignments: assignments}
	}

	// Level 1: try unlocking one existing pilot at a time.
	// Sort by flexibility (largest channel pool first = most likely to relocate).
	flexible := flexiblePilots(existing)
	var bestResult *DisplacementResult
	var bestMargin int

	for _, pilot := range flexible {
		tryLocked := make(map[int]bool, len(allExistingIDs))
		for id := range allExistingIDs {
			tryLocked[id] = true
		}
		delete(tryLocked, pilot.ID)

		assignments = OptimizeWithLocks(all, tryLocked)
		movedIDs = movedPilotIDs(existing, assignments)
		movedIDs[newPilot.ID] = true
		if !hasDangerInvolving(assignments, movedIDs) {
			margin := worstMargin(assignments)
			if bestResult == nil || margin > bestMargin {
				result := DisplacementResult{Level: 1, Assignments: copyAssignments(assignments)}
				bestResult = &result
				bestMargin = margin
			}
		}
	}
	if bestResult != nil {
		return *bestResult
	}

	// Level 2: try unlocking pairs.
	for i := 0; i < len(flexible); i++ {
		for j := i + 1; j < len(flexible); j++ {
			tryLocked := make(map[int]bool, len(allExistingIDs))
			for id := range allExistingIDs {
				tryLocked[id] = true
			}
			delete(tryLocked, flexible[i].ID)
			delete(tryLocked, flexible[j].ID)

			assignments = OptimizeWithLocks(all, tryLocked)
			movedIDs = movedPilotIDs(existing, assignments)
			movedIDs[newPilot.ID] = true
			if !hasDangerInvolving(assignments, movedIDs) {
				margin := worstMargin(assignments)
				if bestResult == nil || margin > bestMargin {
					result := DisplacementResult{Level: 2, Assignments: copyAssignments(assignments)}
					bestResult = &result
					bestMargin = margin
				}
			}
		}
	}
	if bestResult != nil {
		return *bestResult
	}

	// Level 3: buddy suggestion (next task).
	return DisplacementResult{Level: 0, Assignments: OptimizeWithLocks(all, allExistingIDs)}
}

// hasDangerInvolving returns true if any danger-level conflict involves
// a pilot in the given set. Pre-existing conflicts between pilots NOT
// in the set are ignored.
func hasDangerInvolving(assignments []Assignment, pilotIDs map[int]bool) bool {
	for _, c := range DetectConflicts(assignments) {
		if c.Level == ConflictDanger {
			if pilotIDs[c.PilotA] || pilotIDs[c.PilotB] {
				return true
			}
		}
	}
	return false
}

// movedPilotIDs compares assignments against PrevFreqMHz to find which
// existing pilots were displaced.
func movedPilotIDs(existing []PilotInput, assignments []Assignment) map[int]bool {
	moved := make(map[int]bool)
	prevFreqs := make(map[int]int, len(existing))
	for _, p := range existing {
		prevFreqs[p.ID] = p.PrevFreqMHz
	}
	for _, a := range assignments {
		if prev, ok := prevFreqs[a.PilotID]; ok && prev != 0 && a.FreqMHz != prev {
			moved[a.PilotID] = true
		}
	}
	return moved
}

// findChannelName finds the channel name for a given frequency in a pool.
func findChannelName(pool []Channel, freqMHz int) string {
	for _, ch := range pool {
		if ch.FreqMHz == freqMHz {
			return ch.Name
		}
	}
	return ""
}

// flexiblePilots returns existing pilots that are NOT channel-locked,
// sorted by channel pool size descending (most flexible first).
func flexiblePilots(existing []PilotInput) []PilotInput {
	var flex []PilotInput
	for _, p := range existing {
		if !p.ChannelLocked || p.LockedFreqMHz == 0 {
			flex = append(flex, p)
		}
	}
	sort.SliceStable(flex, func(i, j int) bool {
		poolI := ChannelPool(flex[i].VideoSystem, flex[i].FCCUnlocked, flex[i].BandwidthMHz, flex[i].RaceMode, flex[i].Goggles)
		poolJ := ChannelPool(flex[j].VideoSystem, flex[j].FCCUnlocked, flex[j].BandwidthMHz, flex[j].RaceMode, flex[j].Goggles)
		return len(poolI) > len(poolJ)
	})
	return flex
}

// worstMargin returns the worst effective separation margin across all
// assignment pairs. Higher is better.
func worstMargin(assignments []Assignment) int {
	worst := 1<<31 - 1
	for i := 0; i < len(assignments); i++ {
		for j := i + 1; j < len(assignments); j++ {
			a, b := assignments[i], assignments[j]
			sep := a.FreqMHz - b.FreqMHz
			if sep < 0 {
				sep = -sep
			}
			required := RequiredSpacing(a.BandwidthMHz, b.BandwidthMHz)
			margin := sep - required
			if margin < worst {
				worst = margin
			}
		}
	}
	return worst
}

// copyAssignments returns a deep copy of an assignment slice.
func copyAssignments(a []Assignment) []Assignment {
	out := make([]Assignment, len(a))
	copy(out, a)
	return out
}

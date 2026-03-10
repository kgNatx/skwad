package freq

import "sort"

// PilotInput is what the optimizer needs from each pilot.
type PilotInput struct {
	ID               int
	VideoSystem      string
	FCCUnlocked      bool
	BandwidthMHz     int
	RaceMode         bool
	Goggles          string
	PreferredFreqMHz int      // Pilot's preference (0 = auto-assign). Soft signal.
	Pinned           bool     // true = immovable (set by OptimizeWithLocks, never from DB)
	PinnedFreqMHz    int      // frequency to pin to (set by OptimizeWithLocks)
	PrevChannel      string   // Current assignment for stability
	PrevFreqMHz      int
	AnalogBands      []string // Band codes for analog: "R", "F", "E", "L"
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
		pools[p.ID] = ChannelPool(p.VideoSystem, p.FCCUnlocked, p.BandwidthMHz, p.RaceMode, p.Goggles, p.AnalogBands)
		pilotBW[p.ID] = OccupiedBandwidth(p.VideoSystem, p.BandwidthMHz)
	}

	// Separate pinned vs flexible pilots.
	var pinned, flexible []PilotInput
	for _, p := range pilots {
		if p.Pinned && p.PinnedFreqMHz > 0 {
			pinned = append(pinned, p)
		} else {
			flexible = append(flexible, p)
		}
	}

	// Sort flexible pilots: preference pilots with small pools first (most constrained),
	// then auto-assign pilots by pool size ascending.
	sort.SliceStable(flexible, func(i, j int) bool {
		iHasPref := flexible[i].PreferredFreqMHz > 0
		jHasPref := flexible[j].PreferredFreqMHz > 0
		if iHasPref != jHasPref {
			return iHasPref // preference pilots first
		}
		return len(pools[flexible[i].ID]) < len(pools[flexible[j].ID])
	})

	// used tracks all assigned frequencies with their bandwidths.
	var used []usedEntry

	// Step 1: Pin immovable pilots.
	for _, p := range pinned {
		bw := pilotBW[p.ID]
		chName := findChannelName(pools[p.ID], p.PinnedFreqMHz)
		if chName == "" {
			chName = "PINNED"
		}
		results[p.ID] = Assignment{
			PilotID:      p.ID,
			Channel:      chName,
			FreqMHz:      p.PinnedFreqMHz,
			BandwidthMHz: bw,
		}
		used = append(used, usedEntry{freqMHz: p.PinnedFreqMHz, bandwidthMHz: bw, pilotID: p.ID})
	}

	// Step 2: Assign each flexible pilot. Prefer their preference, then previous
	// frequency for stability, then best available by margin.
	for _, p := range flexible {
		pool := pools[p.ID]
		bw := pilotBW[p.ID]
		bestCh := pool[0]
		bestMargin := -(1 << 30)

		for _, ch := range pool {
			margin := effectiveSeparation(ch.FreqMHz, bw, used)

			// Prefer preferred frequency when margin >= 0 (no conflict).
			if p.PreferredFreqMHz > 0 && ch.FreqMHz == p.PreferredFreqMHz && margin >= 0 {
				if margin >= bestMargin {
					bestCh = ch
					bestMargin = margin
					continue
				}
			}

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
// pinned at their PrevFreqMHz, regardless of their preference.
func OptimizeWithLocks(pilots []PilotInput, lockedIDs map[int]bool) []Assignment {
	modified := make([]PilotInput, len(pilots))
	for i, p := range pilots {
		modified[i] = p
		if lockedIDs[p.ID] {
			modified[i].Pinned = true
			modified[i].PinnedFreqMHz = p.PrevFreqMHz
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
	PilotID  int    `json:"pilot_id"`
	Callsign string `json:"callsign"`
	Channel  string `json:"channel"`
	FreqMHz  int    `json:"freq_mhz"`
}

// RebalanceOption describes a partial rebalance that would resolve conflicts.
type RebalanceOption struct {
	Assignments   []Assignment // full assignment set after rebalance
	MovedPilotIDs []int        // IDs of pilots that would be displaced
}

// DisplacementResult is the output of FindMinimalDisplacement.
type DisplacementResult struct {
	Level           int              // 0 or 1
	Assignments     []Assignment     // Level 0 assignment set
	BuddyOption     *BuddySuggestion // Level 1: buddy up option
	RebalanceOption *RebalanceOption  // Level 1: partial rebalance option
}

// FindMinimalDisplacement tries to place newPilot into an existing session.
//
//	Level 0: lock all existing, slot new pilot. If clean, done.
//	Level 1: no clean placement. Build buddy and rebalance options for pilot choice.
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
	movedIDs[newPilot.ID] = true
	if !hasDangerInvolving(assignments, movedIDs) {
		return DisplacementResult{Level: 0, Assignments: assignments}
	}

	// Level 1: build options for pilot choice.
	level0Assignments := copyAssignments(assignments)

	// Option A: buddy suggestion.
	buddyOption := findBestBuddy(existing, newPilot)

	// Option B: partial rebalance -- try unlocking one flexible pilot at a time.
	flexible := flexiblePilots(existing)
	var rebalanceOption *RebalanceOption

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
			var moved []int
			for id := range movedIDs {
				if id != newPilot.ID {
					moved = append(moved, id)
				}
			}
			rebalanceOption = &RebalanceOption{
				Assignments:   copyAssignments(assignments),
				MovedPilotIDs: moved,
			}
			break // take the first clean solution (most flexible pilot)
		}
	}

	// If no single-pilot rebalance worked, try pairs.
	if rebalanceOption == nil {
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
					var moved []int
					for id := range movedIDs {
						if id != newPilot.ID {
							moved = append(moved, id)
						}
					}
					rebalanceOption = &RebalanceOption{
						Assignments:   copyAssignments(assignments),
						MovedPilotIDs: moved,
					}
					break
				}
			}
			if rebalanceOption != nil {
				break
			}
		}
	}

	return DisplacementResult{
		Level:           1,
		Assignments:     level0Assignments,
		BuddyOption:     buddyOption,
		RebalanceOption: rebalanceOption,
	}
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

// flexiblePilots returns ALL existing pilots ranked by displacement flexibility.
// Most flexible (best displacement candidates) first:
//   - Auto-assign pilots (no preference) with large channel pools
//   - Preference pilots with large pools not on their preferred channel
//   - Preference pilots on their preferred channel with tenure (least flexible)
func flexiblePilots(existing []PilotInput) []PilotInput {
	flex := make([]PilotInput, len(existing))
	copy(flex, existing)

	sort.SliceStable(flex, func(i, j int) bool {
		scoreI := flexibilityScore(flex[i])
		scoreJ := flexibilityScore(flex[j])
		if scoreI != scoreJ {
			return scoreI > scoreJ // higher score = more flexible = first
		}
		// Tie-break: larger pool = more flexible.
		poolI := ChannelPool(flex[i].VideoSystem, flex[i].FCCUnlocked, flex[i].BandwidthMHz, flex[i].RaceMode, flex[i].Goggles, flex[i].AnalogBands)
		poolJ := ChannelPool(flex[j].VideoSystem, flex[j].FCCUnlocked, flex[j].BandwidthMHz, flex[j].RaceMode, flex[j].Goggles, flex[j].AnalogBands)
		return len(poolI) > len(poolJ)
	})
	return flex
}

// flexibilityScore returns a score indicating how flexible a pilot is for displacement.
// Higher = more flexible (better candidate to move).
func flexibilityScore(p PilotInput) int {
	if p.PreferredFreqMHz == 0 {
		return 3 // auto-assign: most flexible
	}
	if p.PrevFreqMHz != p.PreferredFreqMHz {
		return 2 // has preference but not currently on it
	}
	return 1 // on their preferred channel with tenure: least flexible
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

// findBestBuddy picks the best existing pilot to share a frequency with.
// Prefers: same video system, similar bandwidth, most margin to other pilots.
func findBestBuddy(existing []PilotInput, newPilot PilotInput) *BuddySuggestion {
	newPool := ChannelPool(newPilot.VideoSystem, newPilot.FCCUnlocked, newPilot.BandwidthMHz, newPilot.RaceMode, newPilot.Goggles, newPilot.AnalogBands)
	newPoolFreqs := make(map[int]bool, len(newPool))
	for _, ch := range newPool {
		newPoolFreqs[ch.FreqMHz] = true
	}

	newBW := OccupiedBandwidth(newPilot.VideoSystem, newPilot.BandwidthMHz)

	type candidate struct {
		pilot PilotInput
		score int // higher is better
	}
	var candidates []candidate

	for _, p := range existing {
		// Buddy's frequency must be in the new pilot's channel pool.
		if !newPoolFreqs[p.PrevFreqMHz] {
			continue
		}

		score := 0
		// Prefer same video system.
		if p.VideoSystem == newPilot.VideoSystem {
			score += 100
		}
		// Prefer similar bandwidth (penalize difference).
		pBW := OccupiedBandwidth(p.VideoSystem, p.BandwidthMHz)
		bwDiff := newBW - pBW
		if bwDiff < 0 {
			bwDiff = -bwDiff
		}
		score -= bwDiff

		candidates = append(candidates, candidate{pilot: p, score: score})
	}

	if len(candidates) == 0 {
		return nil
	}

	// Pick highest score.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	best := candidates[0]

	chName := findChannelName(
		ChannelPool(best.pilot.VideoSystem, best.pilot.FCCUnlocked, best.pilot.BandwidthMHz, best.pilot.RaceMode, best.pilot.Goggles, best.pilot.AnalogBands),
		best.pilot.PrevFreqMHz,
	)

	return &BuddySuggestion{
		PilotID: best.pilot.ID,
		Channel: chName,
		FreqMHz: best.pilot.PrevFreqMHz,
	}
}

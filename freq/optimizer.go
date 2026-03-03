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
	PilotID    int    `json:"pilot_id"`
	Channel    string `json:"channel"`
	FreqMHz    int    `json:"freq_mhz"`
	BuddyGroup int   `json:"buddy_group"`
}

// Optimize takes a list of pilots and assigns each one a channel that
// maximizes frequency separation. When channels must be shared, it
// creates buddy groups.
func Optimize(pilots []PilotInput) []Assignment {
	// Result map keyed by pilot ID for order preservation.
	results := make(map[int]Assignment, len(pilots))

	// Build channel pools for each pilot.
	pools := make(map[int][]Channel, len(pilots))
	for _, p := range pilots {
		pools[p.ID] = ChannelPool(p.VideoSystem, p.FCCUnlocked, p.BandwidthMHz, p.RaceMode, p.Goggles)
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

	// usedFreqs tracks which frequencies are in use. Key is frequency,
	// value is list of pilot IDs on that frequency.
	usedFreqs := make(map[int][]int)

	// Step 1: Lock in fixed-channel pilots.
	for _, p := range locked {
		chName := findChannelName(pools[p.ID], p.LockedFreqMHz)
		if chName == "" {
			// Locked frequency not in pool; use it anyway with a generic name.
			chName = "LOCKED"
		}
		results[p.ID] = Assignment{
			PilotID: p.ID,
			Channel: chName,
			FreqMHz: p.LockedFreqMHz,
		}
		usedFreqs[p.LockedFreqMHz] = append(usedFreqs[p.LockedFreqMHz], p.ID)
	}

	// Step 2: Assign each flexible pilot the channel that maximizes
	// minimum separation from all already-used frequencies.
	for _, p := range flexible {
		pool := pools[p.ID]
		bestCh := pool[0]
		bestSep := -1

		for _, ch := range pool {
			sep := minSeparation(ch.FreqMHz, usedFreqs)

			// Prefer previous frequency for stability when separation
			// is still safe.
			if ch.FreqMHz == p.PrevFreqMHz && sep >= MinSafeSpacingMHz {
				if sep >= bestSep {
					bestCh = ch
					bestSep = sep
					continue
				}
			}

			if sep > bestSep {
				bestCh = ch
				bestSep = sep
			}
		}

		results[p.ID] = Assignment{
			PilotID: p.ID,
			Channel: bestCh.Name,
			FreqMHz: bestCh.FreqMHz,
		}
		usedFreqs[bestCh.FreqMHz] = append(usedFreqs[bestCh.FreqMHz], p.ID)
	}

	// Step 3: Identify buddy groups — frequencies shared by multiple pilots.
	buddyGroupID := 1
	freqToGroup := make(map[int]int)
	for freq, ids := range usedFreqs {
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

// minSeparation returns the minimum distance from freq to any used
// frequency. Returns a large number if no frequencies are in use.
func minSeparation(freq int, usedFreqs map[int][]int) int {
	if len(usedFreqs) == 0 {
		return 1<<31 - 1 // MaxInt
	}
	min := 1<<31 - 1
	for f := range usedFreqs {
		d := freq - f
		if d < 0 {
			d = -d
		}
		if d < min {
			min = d
		}
	}
	return min
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

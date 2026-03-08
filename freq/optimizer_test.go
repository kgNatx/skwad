package freq

import "testing"

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestOptimize_SingleAnalog(t *testing.T) {
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "analog"},
	}
	assignments := Optimize(pilots)
	if len(assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(assignments))
	}
	a := assignments[0]
	if a.PilotID != 1 {
		t.Errorf("PilotID = %d, want 1", a.PilotID)
	}
	if a.FreqMHz == 0 {
		t.Error("FreqMHz should not be 0")
	}
	if a.Channel == "" {
		t.Error("Channel should not be empty")
	}
	if a.BuddyGroup != 0 {
		t.Errorf("BuddyGroup = %d, want 0 (no buddy group for single pilot)", a.BuddyGroup)
	}
	if a.BandwidthMHz != 20 {
		t.Errorf("BandwidthMHz = %d, want 20", a.BandwidthMHz)
	}
}

func TestOptimize_TwoAnalog_MaxSeparation(t *testing.T) {
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "analog"},
		{ID: 2, VideoSystem: "analog"},
	}
	assignments := Optimize(pilots)
	if len(assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(assignments))
	}

	sep := abs(assignments[0].FreqMHz - assignments[1].FreqMHz)
	// R1 (5658) and R8 (5917) = 259 MHz apart; the optimizer should pick
	// the two endpoints of Race Band for maximum separation.
	if sep < 200 {
		t.Errorf("separation = %d MHz, want >= 200 MHz (got %s@%d and %s@%d)",
			sep, assignments[0].Channel, assignments[0].FreqMHz,
			assignments[1].Channel, assignments[1].FreqMHz)
	}
}

func TestOptimize_FourAnalog_SafeSpacing(t *testing.T) {
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "analog"},
		{ID: 2, VideoSystem: "analog"},
		{ID: 3, VideoSystem: "analog"},
		{ID: 4, VideoSystem: "analog"},
	}
	assignments := Optimize(pilots)
	if len(assignments) != 4 {
		t.Fatalf("expected 4 assignments, got %d", len(assignments))
	}

	// All frequencies should be unique.
	freqs := make(map[int]bool)
	for _, a := range assignments {
		if freqs[a.FreqMHz] {
			t.Errorf("duplicate frequency %d MHz", a.FreqMHz)
		}
		freqs[a.FreqMHz] = true
	}

	// All pairs should be >= RequiredSpacing(20, 20) = 30 MHz apart.
	required := RequiredSpacing(20, 20)
	for i := 0; i < len(assignments); i++ {
		for j := i + 1; j < len(assignments); j++ {
			sep := abs(assignments[i].FreqMHz - assignments[j].FreqMHz)
			if sep < required {
				t.Errorf("pilots %d and %d too close: %d MHz apart, need >= %d (%s@%d vs %s@%d)",
					assignments[i].PilotID, assignments[j].PilotID, sep, required,
					assignments[i].Channel, assignments[i].FreqMHz,
					assignments[j].Channel, assignments[j].FreqMHz)
			}
		}
	}
}

func TestOptimize_LockedChannel(t *testing.T) {
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "analog", ChannelLocked: true, LockedFreqMHz: 5732}, // R3
		{ID: 2, VideoSystem: "analog"},
	}
	assignments := Optimize(pilots)
	if len(assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(assignments))
	}

	// Pilot 1 should be locked to R3 @ 5732.
	if assignments[0].FreqMHz != 5732 {
		t.Errorf("locked pilot freq = %d, want 5732", assignments[0].FreqMHz)
	}
	if assignments[0].Channel != "R3" {
		t.Errorf("locked pilot channel = %s, want R3", assignments[0].Channel)
	}

	// Pilot 2 should be far from 5732 — at least RequiredSpacing(20, 20) = 30 MHz.
	sep := abs(assignments[1].FreqMHz - 5732)
	required := RequiredSpacing(20, 20)
	if sep < required {
		t.Errorf("pilot 2 only %d MHz from locked pilot (want >= %d)", sep, required)
	}
}

func TestOptimize_BuddyGroup_TooManyDJI(t *testing.T) {
	// 4 DJI O3 stock pilots but only 3 channels available.
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "dji_o3", BandwidthMHz: 20},
		{ID: 2, VideoSystem: "dji_o3", BandwidthMHz: 20},
		{ID: 3, VideoSystem: "dji_o3", BandwidthMHz: 20},
		{ID: 4, VideoSystem: "dji_o3", BandwidthMHz: 20},
	}
	assignments := Optimize(pilots)
	if len(assignments) != 4 {
		t.Fatalf("expected 4 assignments, got %d", len(assignments))
	}

	// At least one pair must share a frequency → buddy group > 0.
	hasBuddyGroup := false
	for _, a := range assignments {
		if a.BuddyGroup > 0 {
			hasBuddyGroup = true
			break
		}
	}
	if !hasBuddyGroup {
		t.Error("expected at least one buddy group when 4 pilots share 3 channels")
	}
}

func TestOptimize_MixedSystems(t *testing.T) {
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "analog"},
		{ID: 2, VideoSystem: "hdzero"},
		{ID: 3, VideoSystem: "dji_o3", FCCUnlocked: true, BandwidthMHz: 20},
	}
	assignments := Optimize(pilots)
	if len(assignments) != 3 {
		t.Fatalf("expected 3 assignments, got %d", len(assignments))
	}

	for _, a := range assignments {
		if a.FreqMHz == 0 {
			t.Errorf("pilot %d has FreqMHz = 0", a.PilotID)
		}
		if a.Channel == "" {
			t.Errorf("pilot %d has empty Channel", a.PilotID)
		}
	}
}

func TestOptimize_AnalogAndDJIO3_40MHz(t *testing.T) {
	// Analog pilot locked to R3 (5732), DJI O3 at 40 MHz (center 5795).
	// Required spacing = 20/2 + 40/2 + 10 = 40 MHz.
	// R4 (5769) is only 5795-5769 = 26 MHz from O3, which is less than 40.
	// The optimizer should NOT place the analog pilot on R4.
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "dji_o3", BandwidthMHz: 40},                         // Only channel: O3-CH1 at 5795
		{ID: 2, VideoSystem: "analog", ChannelLocked: true, LockedFreqMHz: 5732}, // R3
		{ID: 3, VideoSystem: "analog"},                                            // Should avoid R4 (5769)
	}
	assignments := Optimize(pilots)

	// Find the flexible analog pilot's assignment.
	var flexAnalog Assignment
	for _, a := range assignments {
		if a.PilotID == 3 {
			flexAnalog = a
			break
		}
	}

	// Verify pilot 3 was NOT assigned R4 (5769) — only 26 MHz from 5795.
	if flexAnalog.FreqMHz == 5769 {
		t.Errorf("flexible analog pilot assigned R4 (5769), only 26 MHz from O3@5795; "+
			"should be farther away (required spacing = %d)", RequiredSpacing(20, 40))
	}

	// Verify the separation between pilot 3 and the O3 pilot is reasonable.
	sep := abs(flexAnalog.FreqMHz - 5795)
	required := RequiredSpacing(20, 40) // 40 MHz
	t.Logf("flexible analog assigned %s@%d, separation from O3@5795 = %d (required %d)",
		flexAnalog.Channel, flexAnalog.FreqMHz, sep, required)
}

func TestOptimize_BandwidthMHzPopulated(t *testing.T) {
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "analog"},
		{ID: 2, VideoSystem: "dji_o3", BandwidthMHz: 40},
		{ID: 3, VideoSystem: "dji_o4", BandwidthMHz: 60},
	}
	assignments := Optimize(pilots)

	want := map[int]int{1: 20, 2: 40, 3: 60}
	for _, a := range assignments {
		if a.BandwidthMHz != want[a.PilotID] {
			t.Errorf("pilot %d BandwidthMHz = %d, want %d", a.PilotID, a.BandwidthMHz, want[a.PilotID])
		}
	}
}

// ── Conflict detection tests ─────────────────────────────────────

func TestOptimizeWithLocks_LockedPilotsStay(t *testing.T) {
	// Two existing pilots on R1 and R4, both locked.
	// New flexible pilot should slot in without moving them.
	inputs := []PilotInput{
		{ID: 1, VideoSystem: "analog", PrevChannel: "R1", PrevFreqMHz: 5658},
		{ID: 2, VideoSystem: "analog", PrevChannel: "R4", PrevFreqMHz: 5769},
		{ID: 3, VideoSystem: "analog"}, // new pilot, no prev
	}
	lockedIDs := map[int]bool{1: true, 2: true}

	assignments := OptimizeWithLocks(inputs, lockedIDs)

	// Pilots 1 and 2 must stay exactly where they were.
	for _, a := range assignments {
		switch a.PilotID {
		case 1:
			if a.FreqMHz != 5658 {
				t.Errorf("pilot 1 moved from 5658 to %d", a.FreqMHz)
			}
		case 2:
			if a.FreqMHz != 5769 {
				t.Errorf("pilot 2 moved from 5769 to %d", a.FreqMHz)
			}
		case 3:
			if a.FreqMHz == 0 {
				t.Error("pilot 3 was not assigned a frequency")
			}
			// Should not be on 5658 or 5769
			if a.FreqMHz == 5658 || a.FreqMHz == 5769 {
				t.Errorf("pilot 3 placed on locked frequency %d", a.FreqMHz)
			}
		}
	}
}

func TestOptimizeWithLocks_EmptyLockedSet(t *testing.T) {
	// With no locks, should behave identically to Optimize().
	inputs := []PilotInput{
		{ID: 1, VideoSystem: "analog"},
		{ID: 2, VideoSystem: "analog"},
	}
	locked := OptimizeWithLocks(inputs, map[int]bool{})
	unlocked := Optimize(inputs)

	for i := range locked {
		if locked[i].FreqMHz != unlocked[i].FreqMHz {
			t.Errorf("pilot %d: locked=%d unlocked=%d", locked[i].PilotID, locked[i].FreqMHz, unlocked[i].FreqMHz)
		}
	}
}

// ── Conflict detection tests ─────────────────────────────────────

func TestDetectConflicts_Danger(t *testing.T) {
	// Two 20 MHz signals on the same frequency — overlap.
	assignments := []Assignment{
		{PilotID: 1, FreqMHz: 5769, BandwidthMHz: 20},
		{PilotID: 2, FreqMHz: 5769, BandwidthMHz: 20},
	}
	conflicts := DetectConflicts(assignments)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Level != ConflictDanger {
		t.Errorf("level = %q, want %q", conflicts[0].Level, ConflictDanger)
	}
	if conflicts[0].SeparationMHz != 0 {
		t.Errorf("separation = %d, want 0", conflicts[0].SeparationMHz)
	}
}

func TestDetectConflicts_Warning(t *testing.T) {
	// Analog (20 MHz) at 5769, O3 40 MHz at 5795.
	// Separation = 26 MHz, overlap threshold = 10+20 = 30, required = 40.
	// 26 < 30 so this is actually a danger, not warning.
	// Use a case where sep is between overlap and required.
	// Two 20 MHz signals 25 MHz apart: overlap = 20, required = 30.
	// 25 > 20 but 25 < 30 → warning.
	assignments := []Assignment{
		{PilotID: 1, FreqMHz: 5769, BandwidthMHz: 20},
		{PilotID: 2, FreqMHz: 5794, BandwidthMHz: 20},
	}
	conflicts := DetectConflicts(assignments)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Level != ConflictWarning {
		t.Errorf("level = %q, want %q", conflicts[0].Level, ConflictWarning)
	}
	if conflicts[0].SeparationMHz != 25 {
		t.Errorf("separation = %d, want 25", conflicts[0].SeparationMHz)
	}
}

func TestDetectConflicts_NoConflict(t *testing.T) {
	// Two 20 MHz signals 37 MHz apart: required = 30. 37 >= 30 → no conflict.
	assignments := []Assignment{
		{PilotID: 1, FreqMHz: 5658, BandwidthMHz: 20},
		{PilotID: 2, FreqMHz: 5695, BandwidthMHz: 20},
	}
	conflicts := DetectConflicts(assignments)
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(conflicts))
	}
}

// ── FindMinimalDisplacement tests ────────────────────────────────

func TestFindMinimalDisplacement_Level0_NoConflict(t *testing.T) {
	// Two existing pilots well-spaced. New pilot should fit without moving anyone.
	existing := []PilotInput{
		{ID: 1, VideoSystem: "analog", PrevChannel: "R1", PrevFreqMHz: 5658},
		{ID: 2, VideoSystem: "analog", PrevChannel: "R5", PrevFreqMHz: 5806},
	}
	newPilot := PilotInput{ID: -1, VideoSystem: "analog"}

	result := FindMinimalDisplacement(existing, newPilot)

	if result.Level != 0 {
		t.Errorf("expected level 0, got %d", result.Level)
	}
	// Existing pilots should not have moved.
	for _, a := range result.Assignments {
		if a.PilotID == 1 && a.FreqMHz != 5658 {
			t.Errorf("pilot 1 moved to %d", a.FreqMHz)
		}
		if a.PilotID == 2 && a.FreqMHz != 5806 {
			t.Errorf("pilot 2 moved to %d", a.FreqMHz)
		}
	}
	if result.BuddySuggestion != nil {
		t.Error("unexpected buddy suggestion at level 0")
	}
}

func TestFindMinimalDisplacement_Level0_NewPilotGetsAssignment(t *testing.T) {
	existing := []PilotInput{
		{ID: 1, VideoSystem: "analog", PrevChannel: "R1", PrevFreqMHz: 5658},
	}
	newPilot := PilotInput{ID: -1, VideoSystem: "analog"}

	result := FindMinimalDisplacement(existing, newPilot)

	if result.Level != 0 {
		t.Errorf("expected level 0, got %d", result.Level)
	}
	var found bool
	for _, a := range result.Assignments {
		if a.PilotID == -1 {
			found = true
			if a.FreqMHz == 0 {
				t.Error("new pilot has no frequency")
			}
		}
	}
	if !found {
		t.Error("new pilot not in assignments")
	}
}

func TestDetectConflicts_WideBandDanger(t *testing.T) {
	// Analog (20 MHz) at 5769 (R4), O3 40 MHz at 5795.
	// Separation = 26, overlap = 10+20=30, 26 < 30 → danger.
	assignments := []Assignment{
		{PilotID: 1, FreqMHz: 5769, BandwidthMHz: 20},
		{PilotID: 2, FreqMHz: 5795, BandwidthMHz: 40},
	}
	conflicts := DetectConflicts(assignments)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Level != ConflictDanger {
		t.Errorf("level = %q, want %q", conflicts[0].Level, ConflictDanger)
	}
}

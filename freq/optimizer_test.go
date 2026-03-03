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

func TestOptimize_FourAnalog_IMDFree(t *testing.T) {
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

	// All pairs should be >= MinSafeSpacingMHz apart.
	for i := 0; i < len(assignments); i++ {
		for j := i + 1; j < len(assignments); j++ {
			sep := abs(assignments[i].FreqMHz - assignments[j].FreqMHz)
			if sep < MinSafeSpacingMHz {
				t.Errorf("pilots %d and %d too close: %d MHz apart (%s@%d vs %s@%d)",
					assignments[i].PilotID, assignments[j].PilotID, sep,
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

	// Pilot 2 should be far from 5732.
	sep := abs(assignments[1].FreqMHz - 5732)
	if sep < MinSafeSpacingMHz {
		t.Errorf("pilot 2 only %d MHz from locked pilot (want >= %d)", sep, MinSafeSpacingMHz)
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

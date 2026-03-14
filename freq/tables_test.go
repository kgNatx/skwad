package freq

import "testing"

func TestChannelPool_Analog(t *testing.T) {
	pool := ChannelPool("analog", false, 0, false, "", nil)
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
	if pool[0].Name != "R1" || pool[0].FreqMHz != 5658 {
		t.Errorf("first channel = %v, want R1/5658", pool[0])
	}
}

func TestChannelPool_HDZero(t *testing.T) {
	pool := ChannelPool("hdzero", false, 0, false, "", nil)
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
}

func TestChannelPool_DJI_V1_Stock(t *testing.T) {
	pool := ChannelPool("dji_v1", false, 0, false, "", nil)
	if len(pool) != 4 {
		t.Fatalf("expected 4 channels, got %d", len(pool))
	}
}

func TestChannelPool_DJI_V1_FCC(t *testing.T) {
	pool := ChannelPool("dji_v1", true, 0, false, "", nil)
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
}

func TestChannelPool_DJI_O3_Stock_20MHz(t *testing.T) {
	pool := ChannelPool("dji_o3", false, 20, false, "", nil)
	if len(pool) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(pool))
	}
}

func TestChannelPool_DJI_O3_FCC_20MHz(t *testing.T) {
	pool := ChannelPool("dji_o3", true, 20, false, "", nil)
	if len(pool) != 7 {
		t.Fatalf("expected 7 channels, got %d", len(pool))
	}
}

func TestChannelPool_DJI_O4_RaceMode(t *testing.T) {
	pool := ChannelPool("dji_o4", false, 20, true, "goggles_3", nil)
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
	if pool[0].Name != "R1" || pool[0].FreqMHz != 5658 {
		t.Errorf("first channel = %v, want R1/5658", pool[0])
	}
}

func TestChannelPool_Walksnail_RaceMode(t *testing.T) {
	pool := ChannelPool("walksnail_race", false, 0, false, "", nil)
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
}

func TestChannelPool_Walksnail_Std_Stock(t *testing.T) {
	pool := ChannelPool("walksnail_std", false, 0, false, "", nil)
	if len(pool) != 4 {
		t.Fatalf("expected 4 channels, got %d", len(pool))
	}
}

func TestChannelPool_Walksnail_Std_FCC(t *testing.T) {
	pool := ChannelPool("walksnail_std", true, 0, false, "", nil)
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
}

func TestOccupiedBandwidth(t *testing.T) {
	tests := []struct {
		system string
		bw     int
		want   int
	}{
		{"analog", 0, 20},
		{"hdzero", 0, 20},
		{"dji_v1", 0, 20},
		{"walksnail_std", 0, 20},
		{"walksnail_race", 0, 20},
		{"dji_o3", 20, 20},
		{"dji_o3", 40, 40},
		{"dji_o4", 20, 20},
		{"dji_o4", 40, 40},
		{"dji_o4", 60, 60},
		{"openipc", 0, 20},
	}
	for _, tt := range tests {
		got := OccupiedBandwidth(tt.system, tt.bw)
		if got != tt.want {
			t.Errorf("OccupiedBandwidth(%q, %d) = %d, want %d", tt.system, tt.bw, got, tt.want)
		}
	}
}

func TestRequiredSpacing(t *testing.T) {
	tests := []struct {
		bwA, bwB, guardBand, want int
	}{
		{20, 20, 10, 30}, // 10+10+10
		{20, 40, 10, 40}, // 10+20+10
		{40, 40, 10, 50}, // 20+20+10
		{40, 60, 10, 60}, // 20+30+10
		{60, 60, 10, 70}, // 30+30+10
	}
	for _, tt := range tests {
		got := RequiredSpacing(tt.bwA, tt.bwB, tt.guardBand)
		if got != tt.want {
			t.Errorf("RequiredSpacing(%d, %d, %d) = %d, want %d", tt.bwA, tt.bwB, tt.guardBand, got, tt.want)
		}
	}
}

func TestPowerToGuardBand(t *testing.T) {
	tests := []struct {
		powerMW int
		want    int
	}{
		// Exact thresholds
		{0, 10},
		{25, 10},
		{100, 12},
		{200, 14},
		{400, 16},
		{600, 24},
		{800, 28},
		{1000, 32},
		// Between steps
		{50, 10},
		{300, 14},
		{500, 16},
		{1500, 32},
	}

	for _, tt := range tests {
		got := PowerToGuardBand(tt.powerMW)
		if got != tt.want {
			t.Errorf("PowerToGuardBand(%d) = %d, want %d", tt.powerMW, got, tt.want)
		}
	}
}

func TestRequiredSpacingWithGuardBand(t *testing.T) {
	tests := []struct {
		bwA, bwB, guardBand int
		want                int
	}{
		{20, 20, 10, 30},
		{20, 20, 24, 44},
		{20, 40, 16, 46},
	}

	for _, tt := range tests {
		got := RequiredSpacing(tt.bwA, tt.bwB, tt.guardBand)
		if got != tt.want {
			t.Errorf("RequiredSpacing(%d, %d, %d) = %d, want %d",
				tt.bwA, tt.bwB, tt.guardBand, got, tt.want)
		}
	}
}

func TestChannelPool_Analog_DefaultRaceBand(t *testing.T) {
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
	if len(pool) < 9 {
		t.Fatalf("expected at least 9 channels for R+F, got %d", len(pool))
	}
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
	pool := ChannelPool("hdzero", false, 0, false, "", []string{"F", "E"})
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
	if pool[0].Name != "R1" {
		t.Errorf("HDZero should use RaceBand, got %s", pool[0].Name)
	}
}

package freq

import "testing"

func TestChannelPool_Analog(t *testing.T) {
	pool := ChannelPool("analog", false, 0, false, "")
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
	if pool[0].Name != "R1" || pool[0].FreqMHz != 5658 {
		t.Errorf("first channel = %v, want R1/5658", pool[0])
	}
}

func TestChannelPool_HDZero(t *testing.T) {
	pool := ChannelPool("hdzero", false, 0, false, "")
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
}

func TestChannelPool_DJI_V1_Stock(t *testing.T) {
	pool := ChannelPool("dji_v1", false, 0, false, "")
	if len(pool) != 4 {
		t.Fatalf("expected 4 channels, got %d", len(pool))
	}
}

func TestChannelPool_DJI_V1_FCC(t *testing.T) {
	pool := ChannelPool("dji_v1", true, 0, false, "")
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
}

func TestChannelPool_DJI_O3_Stock_20MHz(t *testing.T) {
	pool := ChannelPool("dji_o3", false, 20, false, "")
	if len(pool) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(pool))
	}
}

func TestChannelPool_DJI_O3_FCC_20MHz(t *testing.T) {
	pool := ChannelPool("dji_o3", true, 20, false, "")
	if len(pool) != 7 {
		t.Fatalf("expected 7 channels, got %d", len(pool))
	}
}

func TestChannelPool_DJI_O4_RaceMode(t *testing.T) {
	pool := ChannelPool("dji_o4", false, 20, true, "goggles_3")
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
	if pool[0].Name != "R1" || pool[0].FreqMHz != 5658 {
		t.Errorf("first channel = %v, want R1/5658", pool[0])
	}
}

func TestChannelPool_Walksnail_RaceMode(t *testing.T) {
	pool := ChannelPool("walksnail_race", false, 0, false, "")
	if len(pool) != 8 {
		t.Fatalf("expected 8 channels, got %d", len(pool))
	}
}

func TestChannelPool_Walksnail_Std_Stock(t *testing.T) {
	pool := ChannelPool("walksnail_std", false, 0, false, "")
	if len(pool) != 4 {
		t.Fatalf("expected 4 channels, got %d", len(pool))
	}
}

func TestChannelPool_Walksnail_Std_FCC(t *testing.T) {
	pool := ChannelPool("walksnail_std", true, 0, false, "")
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
		bwA, bwB, want int
	}{
		{20, 20, 30},  // 10+10+10
		{20, 40, 40},  // 10+20+10
		{40, 40, 50},  // 20+20+10
		{40, 60, 60},  // 20+30+10
		{60, 60, 70},  // 30+30+10
	}
	for _, tt := range tests {
		got := RequiredSpacing(tt.bwA, tt.bwB)
		if got != tt.want {
			t.Errorf("RequiredSpacing(%d, %d) = %d, want %d", tt.bwA, tt.bwB, got, tt.want)
		}
	}
}

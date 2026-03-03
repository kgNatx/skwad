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

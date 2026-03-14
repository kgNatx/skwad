package freq

// Channel represents a single video transmitter channel.
type Channel struct {
	Name    string `json:"name"`     // Human-readable: "R1", "DJI-CH2"
	FreqMHz int    `json:"freq_mhz"` // Center frequency in MHz
}

// DefaultGuardBandMHz is the minimum guard band between the edges of two
// occupied bandwidths to avoid interference.
const DefaultGuardBandMHz = 10

// OccupiedBandwidth returns the actual RF bandwidth in MHz for a given
// video system and bandwidth setting. Most systems use 20 MHz; DJI O3/O4
// can use wider bandwidths based on their bandwidth_mhz setting.
func OccupiedBandwidth(videoSystem string, bandwidthMHz int) int {
	switch videoSystem {
	case "dji_o3", "dji_o4":
		switch bandwidthMHz {
		case 40:
			return 40
		case 60:
			return 60
		default:
			return 20
		}
	default:
		return 20
	}
}

// PowerToGuardBand maps a transmitter power level (in milliwatts) to the
// recommended guard band in MHz. Uses a calibrated lookup table in descending
// order; falls back to DefaultGuardBandMHz for low or unspecified power.
func PowerToGuardBand(powerMW int) int {
	switch {
	case powerMW >= 1000:
		return 32
	case powerMW >= 800:
		return 28
	case powerMW >= 600:
		return 24
	case powerMW >= 400:
		return 16
	case powerMW >= 200:
		return 14
	case powerMW >= 100:
		return 12
	case powerMW >= 25:
		return 10
	default:
		return DefaultGuardBandMHz
	}
}

// RequiredSpacing returns the minimum center-to-center frequency separation
// in MHz required between two signals with the given occupied bandwidths and
// guard band.
func RequiredSpacing(bwA, bwB, guardBandMHz int) int {
	return bwA/2 + bwB/2 + guardBandMHz
}

// ---------- Race Band ----------

// RaceBand contains the 8 standard Race Band channels (R1-R8).
var RaceBand = []Channel{
	{"R1", 5658},
	{"R2", 5695},
	{"R3", 5732},
	{"R4", 5769},
	{"R5", 5806},
	{"R6", 5843},
	{"R7", 5880},
	{"R8", 5917},
}

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

// ---------- DJI V1 (Air Unit / Vista) ----------

// DJIV1FCC contains all 8 DJI V1 channels available in FCC-unlocked mode.
var DJIV1FCC = []Channel{
	{"DJI-CH1", 5660},
	{"DJI-CH2", 5695},
	{"DJI-CH3", 5735},
	{"DJI-CH4", 5770},
	{"DJI-CH5", 5805},
	{"DJI-CH6", 5878},
	{"DJI-CH7", 5914},
	{"DJI-CH8", 5839},
}

// DJIV1Stock contains the 4 DJI V1 channels available without FCC unlock.
var DJIV1Stock = []Channel{
	{"DJI-CH3", 5735},
	{"DJI-CH4", 5770},
	{"DJI-CH5", 5805},
	{"DJI-CH8", 5839},
}

// ---------- DJI O3 ----------

// DJIO3Stock contains the 3 DJI O3 channels available at 20 MHz without FCC unlock.
var DJIO3Stock = []Channel{
	{"O3-CH1", 5769},
	{"O3-CH2", 5805},
	{"O3-CH3", 5840},
}

// DJIO3FCC contains the 7 DJI O3 channels available at 20 MHz with FCC unlock.
var DJIO3FCC = []Channel{
	{"O3-CH1", 5669},
	{"O3-CH2", 5705},
	{"O3-CH3", 5741},
	{"O3-CH4", 5769},
	{"O3-CH5", 5805},
	{"O3-CH6", 5840},
	{"O3-CH7", 5876},
}

// DJIO3_40 contains the single DJI O3 channel available at 40 MHz bandwidth.
var DJIO3_40 = []Channel{
	{"O3-CH1", 5795},
}

// ---------- DJI O4 ----------

// DJIO4Stock contains the 3 DJI O4 channels available at 20 MHz without FCC unlock.
var DJIO4Stock = []Channel{
	{"O4-CH1", 5769},
	{"O4-CH2", 5790},
	{"O4-CH3", 5815},
}

// DJIO4FCC contains the 7 DJI O4 channels available at 20 MHz with FCC unlock.
var DJIO4FCC = []Channel{
	{"O4-CH1", 5669},
	{"O4-CH2", 5705},
	{"O4-CH3", 5741},
	{"O4-CH4", 5769},
	{"O4-CH5", 5790},
	{"O4-CH6", 5815},
	{"O4-CH7", 5876},
}

// DJIO4_40_FCC contains the 3 DJI O4 channels available at 40 MHz with FCC unlock.
var DJIO4_40_FCC = []Channel{
	{"O4-CH1", 5735},
	{"O4-CH2", 5795},
	{"O4-CH3", 5855},
}

// DJIO4_40_Stock contains the single DJI O4 channel at 40 MHz without FCC unlock.
var DJIO4_40_Stock = []Channel{
	{"O4-CH1", 5795},
}

// DJIO4_60 contains the single DJI O4 channel at 60 MHz bandwidth.
var DJIO4_60 = []Channel{
	{"O4-CH1", 5795},
}

// ---------- Walksnail ----------

// WalksnailStdFCC uses the same channel table as DJI V1 FCC.
var WalksnailStdFCC = DJIV1FCC

// WalksnailStdStock uses the same channel table as DJI V1 Stock.
var WalksnailStdStock = DJIV1Stock

// WalksnailRace uses the standard Race Band channels.
var WalksnailRace = RaceBand

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

// ChannelPool returns the available channels for a pilot based on their
// video system, FCC unlock status, bandwidth, race mode, and goggle model.
func ChannelPool(videoSystem string, fccUnlocked bool, bandwidthMHz int, raceMode bool, goggles string, analogBands []string) []Channel {
	switch videoSystem {
	case "analog":
		return MergeAnalogBands(analogBands)
	case "hdzero":
		return RaceBand

	case "dji_v1":
		if fccUnlocked {
			return DJIV1FCC
		}
		return DJIV1Stock

	case "dji_o3":
		if bandwidthMHz >= 40 {
			return DJIO3_40
		}
		if fccUnlocked {
			return DJIO3FCC
		}
		return DJIO3Stock

	case "dji_o4":
		if raceMode && (goggles == "goggles_3" || goggles == "goggles_n3") {
			return RaceBand
		}
		if bandwidthMHz >= 60 {
			return DJIO4_60
		}
		if bandwidthMHz >= 40 {
			if fccUnlocked {
				return DJIO4_40_FCC
			}
			return DJIO4_40_Stock
		}
		if fccUnlocked {
			return DJIO4FCC
		}
		return DJIO4Stock

	case "walksnail_std":
		if fccUnlocked {
			return WalksnailStdFCC
		}
		return WalksnailStdStock

	case "walksnail_race":
		return WalksnailRace

	case "openipc":
		return []Channel{{"WiFi-165", 5825}}

	default:
		return RaceBand
	}
}

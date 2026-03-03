package freq

// Channel represents a single video transmitter channel.
type Channel struct {
	Name    string `json:"name"`     // Human-readable: "R1", "DJI-CH2"
	FreqMHz int    `json:"freq_mhz"` // Center frequency in MHz
}

// MinSafeSpacingMHz is the minimum spacing between two assigned channels
// to avoid interference.
const MinSafeSpacingMHz = 37

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

// ChannelPool returns the available channels for a pilot based on their
// video system, FCC unlock status, bandwidth, race mode, and goggle model.
func ChannelPool(videoSystem string, fccUnlocked bool, bandwidthMHz int, raceMode bool, goggles string) []Channel {
	switch videoSystem {
	case "analog", "hdzero":
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

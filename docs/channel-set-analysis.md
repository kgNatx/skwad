# Mixed Channel Set Analysis

Analysis of optimal fixed channel sets for mixed analog + DJI FPV sessions.

## Key Takeaways

1. **Mixed DJI + analog sessions can beat pure raceband.** At 5 pilots, a mixed set achieves 47 MHz minimum spacing vs raceband's 37 MHz — because DJI's slightly offset frequencies break free of the rigid 37 MHz raceband grid.

2. **DJI-CH4 (5769 MHz) is the golden DJI channel.** It's identical to R4 (5769 MHz) and drops into any raceband arrangement with zero penalty. DJI-CH5/DJI-CH6/DJI-CH7 are also within 1-4 MHz of R5/R6/R7 — close enough to share a slot.

3. **Avoid R1+DJI-CH1 or R2+DJI-CH2 together.** 10-11 MHz gap wastes spectrum — too close for separate channels, too far apart to share a slot.

4. **Fixed channels are most valuable at 2-5 pilots.** At 6+ pilots on raceband, you're at the 37 MHz spacing floor regardless of mix — the optimizer already gives you the best possible assignments. Fixed channels don't buy anything at that density.

5. **All-DJI hard limit is 6 pilots** (and only at 200 mW). 7 DJI pilots on FCC 20 MHz channels can't meet minimum guard band spacing (28 MHz between adjacent channels vs 30 MHz required).

6. **The 5-pilot mixed set is the star.** MIXED OPTIMAL (R1, DJI-CH2, R4, R6, R8) gets 47 MHz spacing, IMD 91, and tolerates 600 mW — objectively better than any pure raceband 5-pilot set.

## Near-Overlap Pairs

These analog-DJI pairs are close enough to effectively share the same spectral slot (one OR the other, not both simultaneously):

| Analog | DJI | Gap | Status |
|--------|-----|-----|--------|
| R4 (5769) | DJI-CH4 (5769) | 0 MHz | Exact overlap |
| R5 (5806) | DJI-CH5 (5805) | 1 MHz | Effectively same |
| R6 (5843) | DJI-CH6 (5840) | 3 MHz | Effectively same |
| R7 (5880) | DJI-CH7 (5876) | 4 MHz | Effectively same |
| R3 (5732) | DJI-CH3 (5741) | 9 MHz | Conflict zone |
| R2 (5695) | DJI-CH2 (5705) | 10 MHz | Conflict zone |
| R1 (5658) | DJI-CH1 (5669) | 11 MHz | Conflict zone |

## Best Sets by Pilot Count

### 2 Pilots

| Set | Channels | Spacing | IMD | Power | Systems |
|-----|----------|---------|-----|-------|---------|
| MAX SPREAD | R1 (5658), R8 (5917) | 259 MHz | 100 | ANY | Analog/HDZero |
| DJI SPREAD | DJI-CH1 (5669), DJI-CH7 (5876) | 207 MHz | 100 | ANY | DJI |

### 3 Pilots

| Set | Channels | Spacing | IMD | Power | Systems |
|-----|----------|---------|-----|-------|---------|
| IMD CLEAN | R1, R4, R8 | 111 MHz | 100 | ANY | Analog/HDZero |
| MIXED CLEAN | R1, DJI-CH5 (5805), R8 | 112 MHz | 100 | ANY | Mixed |
| DJI SPREAD | DJI-CH1, DJI-CH4, DJI-CH7 | 100 MHz | 78 | ANY | DJI |

### 4 Pilots

| Set | Channels | Spacing | IMD | Power | Systems |
|-----|----------|---------|-----|-------|---------|
| IMD CLEAN | R1, R3, R6, R8 | 74 MHz | 100 | ANY | Analog/HDZero |
| MIXED CLEAN | R1, DJI-CH3 (5741), DJI-CH6 (5840), R8 | 77 MHz | 98 | ANY | Mixed |
| DJI SPREAD | DJI-CH1, DJI-CH3, DJI-CH5, DJI-CH7 | 64 MHz | 69 | ANY | DJI |

### 5 Pilots

| Set | Channels | Spacing | IMD | Power | Systems |
|-----|----------|---------|-----|-------|---------|
| **MIXED OPTIMAL** | R1, DJI-CH2 (5705), R4, R6, R8 | **47 MHz** | 91 | **≤600 mW** | Mixed |
| ET5A | E3 (5665), F1 (5740), F4 (5800), F7 (5860), E6 (5905) | 45 MHz | 88 | ≤600 mW | Analog (multi-band) |
| RACEBAND 5 | R1, R3, R5, R6, R8 | 37 MHz | 40 | ≤400 mW | Analog/HDZero |
| DJI 5 | DJI-CH1, DJI-CH3, DJI-CH5, DJI-CH6, DJI-CH7 | 36 MHz | 100 | ≤400 mW | DJI |

### 6+ Pilots — Use the Optimizer

At 6+ pilots, minimum spacing hits the 37 MHz raceband floor regardless of channel selection. Fixed sets don't provide additional value over the optimizer's auto-assignment. Notable options for reference:

| Set | Channels | Spacing | IMD | Power | Systems |
|-----|----------|---------|-----|-------|---------|
| IMD 6C (MultiGP) | R1, R2, F2 (5760), F4 (5800), R7, R8 | 37 MHz | 29 | ≤400 mW | Analog (multi-band) |
| RACEBAND 6 | R1, R2, R4, R5, R7, R8 | 37 MHz | 14 | ≤400 mW | Analog/HDZero |
| DJI 6 | DJI-CH1 through DJI-CH3, DJI-CH5 through DJI-CH7 | 35 MHz | 55 | ≤200 mW | DJI |
| 7 DJI | All 7 FCC channels | 28 MHz | 5 | **BELOW MINIMUM** | Not viable |

## Methodology

- Channel pools: Raceband R1-R8 (analog/HDZero), DJI O3/O4 FCC 20 MHz DJI-CH1 through DJI-CH7
- All channels assumed 20 MHz occupied bandwidth
- Required spacing formula: (BW_A/2) + (BW_B/2) + guard_band
- Default guard band: 10 MHz (required spacing = 30 MHz for two 20 MHz pilots)
- IMD scoring: proximity-weighted quadratic penalty, 20 MHz threshold
- Power tolerance: maximum guard band that fits the set's minimum spacing, mapped back to mW via PowerToGuardBand table
- Enumeration: exhaustive search of all combinations per pilot count and analog/DJI split
- Analysis script: `mixed-channel-analysis.py`

# FPV 5.8 GHz Channel Separation Guide

A technical reference for FPV frequency coordination across mixed analog and digital video systems. Built from exhaustive combinatorial analysis and real-world field testing at group flying events.

## Key Takeaways

1. **Bandwidth matters more than power.** Switching a DJI system from 40 MHz to 20 MHz mode saves 10 MHz of required spacing per neighbor — equivalent to jumping from a 200 mW guard band to a 600 mW guard band. Bandwidth is also the one parameter DJI pilots actually control (power is automatic).

2. **Race Band has a hard capacity cliff.** Race Band channels are uniformly spaced at 37 MHz. This means capacity drops from 8 channels to 4 the instant your required spacing exceeds 37 MHz — there's no gradual degradation. The cliff falls between 400 mW and 600 mW power ceiling.

3. **Mixed DJI+analog groups can space better than pure raceband.** DJI O3's channel frequencies are offset 1–11 MHz from raceband, breaking the rigid 37 MHz grid. An optimized 5-pilot mixed set achieves 47 MHz minimum spacing vs. raceband's 37 MHz.

4. **Perfect IMD separation only exists for ≤4 pilots.** At 5+ pilots on the 5.8 GHz band, every channel set has some third-order intermodulation products near active channels. The standard MultiGP 6-pilot set (IMD 6C) scores 29/100 on IMD quality — it's a practical compromise, not a clean solution.

5. **The guard band values everyone uses are empirically calibrated, not measured.** The 10 MHz default guard band at 25 mW matches decades of community experience. The values at higher power are calibrated to produce sensible raceband capacity transitions. This is honest engineering — the model fits reality, even if it's not derived from specific VTX hardware measurements.

6. **O3 and O4 have different channel centers.** DJI O3 and O4 share only channels 1–2 in FCC 20 MHz mode; channels 3–7 all differ. Mixed-system analysis results are not interchangeable between the two.

---

## Channel Tables

All frequencies in MHz. Source of truth: `freq/tables.go`.

### Analog Bands

Each channel occupies approximately 20 MHz of bandwidth.

**Race Band (R)** — 37 MHz uniform spacing, designed by ImmersionRC for multi-pilot racing:

| R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 |
|----|----|----|----|----|----|----|-----|
| 5658 | 5695 | 5732 | 5769 | 5806 | 5843 | 5880 | 5917 |

**Fatshark (F)** — 20 MHz spacing:

| F1 | F2 | F3 | F4 | F5 | F6 | F7 | F8 |
|----|----|----|----|----|----|----|-----|
| 5740 | 5760 | 5780 | 5800 | 5820 | 5840 | 5860 | 5880 |

**Boscam E** — mixed spacing:

| E1 | E2 | E3 | E4 | E5 | E6 | E7 | E8 |
|----|----|----|----|----|----|----|-----|
| 5705 | 5685 | 5665 | 5645 | 5885 | 5905 | 5925 | 5945 |

**Low Race (L)** — 37 MHz spacing, below the standard 5.8 GHz ISM band:

| L1 | L2 | L3 | L4 | L5 | L6 | L7 | L8 |
|----|----|----|----|----|----|----|-----|
| 5362 | 5399 | 5436 | 5473 | 5510 | 5547 | 5584 | 5621 |

**Cross-band overlap:** R7 (5880 MHz) and F8 (5880 MHz) are the only frequency collision across all four bands.

### Digital Systems

**DJI O3 — FCC 20 MHz** (7 channels):

| O3-CH1 | O3-CH2 | O3-CH3 | O3-CH4 | O3-CH5 | O3-CH6 | O3-CH7 |
|--------|--------|--------|--------|--------|--------|--------|
| 5669 | 5705 | 5769 | 5805 | 5840 | 5876 | 5912 |

**DJI O4 — FCC 20 MHz** (7 channels):

| O4-CH1 | O4-CH2 | O4-CH3 | O4-CH4 | O4-CH5 | O4-CH6 | O4-CH7 |
|--------|--------|--------|--------|--------|--------|--------|
| 5669 | 5705 | 5741 | 5769 | 5790 | 5815 | 5876 |

**O3 vs. O4 divergence:** Only channels 1–2 are identical. Channels 3–7 all differ:
- O3-CH3 (5769) vs. O4-CH3 (5741): 28 MHz apart
- O3-CH4 (5805) vs. O4-CH4 (5769): 36 MHz apart
- O3-CH5 (5840) vs. O4-CH5 (5790): 50 MHz apart
- O3-CH6 (5876) vs. O4-CH6 (5815): 61 MHz apart
- O3-CH7 (5912) vs. O4-CH7 (5876): 36 MHz apart

This means near-overlap analysis and optimal mixed sets are different for O3 vs. O4.

**DJI O3/O4 — FCC 40 MHz** (3 channels):

| CH1 | CH2 | CH3 |
|-----|-----|-----|
| 5735 | 5795 | 5855 |

**DJI O4 — 60 MHz** (1 channel): 5795 MHz

**DJI V1 / Vista — FCC** (8 channels, 20 MHz):

| CH1 | CH2 | CH3 | CH4 | CH5 | CH6 | CH7 | CH8 |
|-----|-----|-----|-----|-----|-----|-----|------|
| 5660 | 5695 | 5735 | 5770 | 5805 | 5878 | 5914 | 5839 |

**Walksnail Standard** uses the same channel table as DJI V1. **Walksnail Race** uses Race Band (R1–R8). **HDZero** uses Race Band (R1–R8). **DJI O4 Race Mode** (with Goggles 3 or N3) uses Race Band (R1–R8).

**OpenIPC**: single channel at WiFi-165 (5825 MHz).

### Raceband vs. DJI Near-Overlap Pairs

These analog-DJI pairs are close enough to effectively share the same spectral slot (one or the other, not both):

**Raceband vs. O3 FCC 20 MHz:**

| Analog | O3 | Gap | Status |
|--------|-----|-----|--------|
| R4 (5769) | O3-CH3 (5769) | 0 MHz | Exact match |
| R5 (5806) | O3-CH4 (5805) | 1 MHz | Effectively same |
| R6 (5843) | O3-CH5 (5840) | 3 MHz | Effectively same |
| R7 (5880) | O3-CH6 (5876) | 4 MHz | Effectively same |
| R8 (5917) | O3-CH7 (5912) | 5 MHz | Effectively same |
| R2 (5695) | O3-CH2 (5705) | 10 MHz | Conflict zone |
| R1 (5658) | O3-CH1 (5669) | 11 MHz | Conflict zone |

**Raceband vs. O4 FCC 20 MHz:**

| Analog | O4 | Gap | Status |
|--------|-----|-----|--------|
| R4 (5769) | O4-CH4 (5769) | 0 MHz | Exact match |
| R7 (5880) | O4-CH7 (5876) | 4 MHz | Effectively same |
| R5 (5806) | O4-CH5 (5790) | 16 MHz | Independent |
| R6 (5843) | O4-CH6 (5815) | 28 MHz | Independent |
| R3 (5732) | O4-CH3 (5741) | 9 MHz | Conflict zone |
| R2 (5695) | O4-CH2 (5705) | 10 MHz | Conflict zone |
| R1 (5658) | O4-CH1 (5669) | 11 MHz | Conflict zone |

O4's channels 5–6 are far enough from raceband to be independent channels rather than near-overlaps. This gives O4 more unique spectral positions but changes the optimal mixed sets.

**Conflict zone** channels (10–11 MHz gap) are problematic: too close for the optimizer to treat as independent channels at low guard bands, but too far apart to share a spectral slot. Avoid pairing R1+O3-CH1 or R2+O3-CH2 in the same session when possible.

---

## Spacing Model

### Required Spacing Formula

The minimum center-to-center frequency separation between two pilots:

```
RequiredSpacing = BW_A/2 + BW_B/2 + GuardBand
```

Where `BW_A` and `BW_B` are the occupied bandwidths of each pilot's signal. This is first-principles geometry — each signal extends half its bandwidth above and below center, and the guard band provides additional margin.

| Pilot A | Pilot B | Guard Band | Required Spacing |
|---------|---------|------------|-----------------|
| 20 MHz | 20 MHz | 10 MHz | 30 MHz |
| 20 MHz | 40 MHz | 10 MHz | 40 MHz |
| 40 MHz | 40 MHz | 10 MHz | 50 MHz |
| 20 MHz | 60 MHz | 10 MHz | 50 MHz |
| 40 MHz | 60 MHz | 10 MHz | 60 MHz |
| 60 MHz | 60 MHz | 10 MHz | 70 MHz |

### Guard Band: The Empirical Part

The guard band accounts for two real-world effects:

**Spectral splatter.** No VTX transmits a perfectly clean signal. Energy leaks into adjacent frequencies — out-of-band emission that worsens as amplifiers are driven harder at higher power. A VTX at 800 mW has measurably worse spectral purity than the same unit at 25 mW.

**Receiver filter limitations.** FPV receivers use bandpass filters (typically ~18–20 MHz wide at -3 dB) to reject other channels. These filters don't have infinite rolloff — an interfering signal close in frequency can punch through the filter skirt, especially at high power.

The guard band grows with transmit power. We model this relationship as:

```
guard_band ≈ 10 × 10^((3 × log₁₀(P_mW / 25)) / 20)  MHz
```

This formula produces 10 MHz at 25 mW (matching the proven community default) and scales gently through the low-power range. **Above 400 mW, the table values are manual overrides** — they are calibrated to place the raceband capacity cliff between 400 mW and 600 mW, matching the community consensus that 400 mW is the upper limit for comfortable group flying.

| Power Ceiling | Guard Band | Required Spacing (20+20 MHz) | Raceband Channels | Derivation |
|--------------|-----------|------------------------------|-------------------|------------|
| 25 mW | 10 MHz | 30 MHz | 8 | Formula |
| 100 mW | 12 MHz | 32 MHz | 8 | Formula |
| 200 mW | 14 MHz | 34 MHz | 8 | Formula (~15 MHz, rounded down) |
| 400 mW | 16 MHz | 36 MHz | 8 (1 MHz margin) | Formula (~15 MHz, rounded up) |
| 600 mW | 24 MHz | 44 MHz | 4 | **Manual override** |
| 800 mW | 28 MHz | 48 MHz | 4 | **Manual override** |
| 1000 mW | 32 MHz | 52 MHz | 4 | **Manual override** |

The jump from 16 MHz to 24 MHz between 400 and 600 mW is deliberate, not formula-derived. It creates a clear signal to session leaders: below 400 mW you have full raceband capacity; above 600 mW you have half.

### The Raceband Cliff

Race Band was designed with 37 MHz uniform channel spacing. This uniformity is a double-edged sword:

- At ≤36 MHz required spacing: all 8 channels are conflict-free. Every adjacent pair has identical margin.
- At ≥38 MHz required spacing: only 4 channels work (every-other: R1/R3/R5/R7 or R2/R4/R6/R8). Half the capacity vanishes instantly.

There is no gradual degradation between 8 and 4 because the channels are uniformly spaced. With non-uniform spacing (like Fatshark or mixed analog+DJI), you'd lose channels one at a time. With raceband, it's all or nothing.

This cliff is the central constraint of the power-to-guard-band table. We calibrated the guard band values so the cliff falls between 400 mW (16 MHz guard → 36 MHz spacing → 8 channels with 1 MHz margin) and 600 mW (24 MHz guard → 44 MHz spacing → 4 channels).

### Bandwidth Impact

Each 20 MHz increase in occupied bandwidth adds 10 MHz to required spacing per neighbor. For DJI pilots, this often matters more than the power ceiling:

| DJI Bandwidth | vs. 20 MHz neighbor (14 MHz guard) | vs. 40 MHz neighbor (14 MHz guard) |
|--------------|-----------------------------------|-----------------------------------|
| 20 MHz | 34 MHz spacing | 44 MHz spacing |
| 40 MHz | 44 MHz spacing | 54 MHz spacing |
| 60 MHz | 54 MHz spacing | 64 MHz spacing |

Switching from 40 MHz to 20 MHz mode saves exactly as much spacing as dropping from 600 mW to 200 mW guard band. Bandwidth is the lever pilots should reach for first.

---

## Intermodulation Distortion (IMD)

### How IMD Works

When two VTXs transmit simultaneously, their signals can mix nonlinearly — in the RF front-end of nearby receivers, or even in the transmitters themselves — to produce phantom signals at new frequencies.

The strongest products are third-order:

```
F_imd = 2 × F₁ - F₂
F_imd = 2 × F₂ - F₁
```

For example, pilots on 5760 MHz and 5800 MHz produce IMD at:
- (2 × 5760) - 5800 = **5720 MHz**
- (2 × 5800) - 5760 = **5840 MHz**

If a third pilot is on 5840 MHz, they see interference from this IMD product even though neither source pilot is on their channel.

Fifth-order products (3F₁ - 2F₂, etc.) also exist but are typically 20–30 dB below third-order — well below the noise floor at FPV power levels and distances. They are excluded from this analysis.

### IMD Scoring

We score IMD quality on a 0–100 scale using a proximity-weighted quadratic penalty, inspired by [ET's IMD Tools](http://www.etheli.com/IMD/):

1. Compute all third-order IMD products for every pair of active channels
2. For each product within 20 MHz of an active channel: `penalty = (20 - distance)²`
3. A product directly on a channel (0 MHz) scores 400 points. A product 15 MHz away scores 25.
4. Final score: `100 - (penalty_sum / (5 × pilot_count))`, clamped to 0–100

The 20 MHz threshold aligns with typical FPV receiver filter bandwidth — products outside the passband are attenuated by the filter and don't contribute meaningful interference.

**Practical interpretation:**

| Score | Meaning |
|-------|---------|
| 100 | No IMD products near any active channel. Clean. |
| 80–99 | Minor products, far from channels. No visible artifacts. |
| 40–79 | Products near some channels. May see occasional interference lines in video. |
| < 30 | Significant IMD exposure. Acceptable for freestyle at distance; problematic for tight racing. |

**IMD scoring is offline reference analysis.** The runtime optimizer does not compute IMD — it maximizes channel separation only, which naturally reduces IMD strength. Known IMD-clean channel sets are provided as recommended presets for race organizers.

### Optimal Channel Sets by Pilot Count

Found by exhaustive enumeration of all possible combinations. IMD scores computed for each set.

**2 pilots** — R1 (5658), R8 (5917): 259 MHz spacing, IMD 100

**3 pilots** — R1 (5658), R4 (5769), R8 (5917): 111 MHz spacing, IMD 100

**4 pilots** — R1 (5658), R3 (5732), R6 (5843), R8 (5917): 74 MHz spacing, IMD 100

These are the only pilot counts where a perfect IMD score (100) is achievable on raceband.

**5 pilots** — ET5A: E3 (5665), F1 (5740), F4 (5800), F7 (5860), E6 (5905): 45 MHz spacing, IMD 88. Requires Fatshark and Boscam E bands.

**6 pilots** — IMD 6C (MultiGP standard): R1 (5658), R2 (5695), F2 (5760), F4 (5800), R7 (5880), R8 (5917): 37 MHz spacing, IMD 29. This is the practical limit — there simply aren't 6 frequencies in the 5.8 GHz band that avoid all third-order products.

---

## Mixed System Analysis

### Methodology

Exhaustive combinatorial search across all Raceband (R1–R8) and DJI O3 FCC 20 MHz (O3-CH1–CH7) channel combinations. For each pilot count from 2 to 8, we tested every possible split of analog vs. DJI channels and evaluated:

- Minimum pairwise spacing
- IMD score
- Maximum power tolerance (highest guard band that still fits the minimum spacing)

All channels assumed 20 MHz occupied bandwidth. Analysis script: `mixed-channel-analysis.py`.

### Key Finding: Grid-Breaking

Race Band's 37 MHz uniform spacing is both its strength (predictable, easy to reason about) and its limitation (rigid, no room to optimize). DJI O3's FCC channels are offset from raceband by varying amounts:

| O3 Channel | Nearest Raceband | Offset |
|-----------|-----------------|--------|
| O3-CH1 (5669) | R1 (5658) | +11 MHz |
| O3-CH2 (5705) | R2 (5695) | +10 MHz |
| O3-CH3 (5769) | R4 (5769) | 0 MHz |
| O3-CH4 (5805) | R5 (5806) | -1 MHz |
| O3-CH5 (5840) | R6 (5843) | -3 MHz |
| O3-CH6 (5876) | R7 (5880) | -4 MHz |
| O3-CH7 (5912) | R8 (5917) | -5 MHz |

These non-uniform offsets mean that inserting a DJI channel into a raceband set can create unequal gaps — and unequal gaps let the optimizer pack channels more efficiently than a uniform grid allows.

### Best Sets by Pilot Count

#### 2 Pilots

| Set | Channels | Min Spacing | IMD | Max Power |
|-----|----------|------------|-----|-----------|
| Max Spread | R1 (5658), R8 (5917) | 259 MHz | 100 | Any |
| DJI Spread | O3-CH1 (5669), O3-CH7 (5912) | 243 MHz | 100 | Any |

#### 3 Pilots

| Set | Channels | Min Spacing | IMD | Max Power |
|-----|----------|------------|-----|-----------|
| Raceband Clean | R1, R4, R8 | 111 MHz | 100 | Any |
| Mixed Clean | R1, O3-CH4 (5805), R8 | 112 MHz | 100 | Any |
| DJI Spread | O3-CH1, O3-CH4, O3-CH7 | 107 MHz | 78 | Any |

#### 4 Pilots

| Set | Channels | Min Spacing | IMD | Max Power |
|-----|----------|------------|-----|-----------|
| Raceband Clean | R1, R3, R6, R8 | 74 MHz | 100 | Any |
| Mixed Clean | R1, O3-CH3 (5769), O3-CH6 (5876), R8 | 41 MHz | 98 | Any |
| DJI Spread | O3-CH1, O3-CH3, O3-CH5, O3-CH7 | 71 MHz | 69 | Any |

#### 5 Pilots — The Sweet Spot

| Set | Channels | Min Spacing | IMD | Max Power |
|-----|----------|------------|-----|-----------|
| **Mixed Optimal** | **R1, O3-CH2 (5705), R4, R6, R8** | **47 MHz** | **91** | **≤600 mW** |
| ET5A (multi-band) | E3, F1, F4, F7, E6 | 45 MHz | 88 | ≤600 mW |
| Raceband 5 | R1, R3, R5, R6, R8 | 37 MHz | 40 | ≤400 mW |
| DJI 5 | O3-CH1, CH3, CH5, CH6, CH7 | 36 MHz | 100 | ≤400 mW |

The mixed set is the standout: **47 MHz minimum spacing and IMD 91** — better than any pure raceband or pure DJI set at 5 pilots. It works because O3-CH2 (5705) sits 47 MHz above R1 (5658) and 64 MHz below R4 (5769), creating a non-uniform gap pattern that avoids the raceband grid's limitations.

It also tolerates up to 600 mW, where pure raceband sets are limited to 400 mW.

#### 6+ Pilots — Use the Optimizer

At 6+ pilots, minimum spacing hits the 37 MHz raceband floor regardless of channel selection. Fixed sets provide little advantage over dynamic auto-assignment.

| Set | Channels | Min Spacing | IMD | Max Power |
|-----|----------|------------|-----|-----------|
| IMD 6C (MultiGP) | R1, R2, F2, F4, R7, R8 | 37 MHz | 29 | ≤400 mW |
| Raceband 6 | R1, R2, R4, R5, R7, R8 | 37 MHz | 14 | ≤400 mW |
| DJI 6 | O3-CH1–CH3, CH5–CH7 | 36 MHz | 55 | ≤400 mW |
| 7 DJI | All 7 FCC channels | 35 MHz | 5 | ≤200 mW |

**All-DJI at 7 pilots.** Seven O3 FCC 20 MHz channels have 35 MHz minimum adjacent spacing. With a 10 MHz guard band (30 MHz required), 7 pilots fits at 25 mW. At ≤200 mW (14 MHz guard → 34 MHz required), it still works with 1 MHz margin. At 400 mW (16 MHz guard → 36 MHz required), it fails. Six DJI pilots with 36 MHz spacing works up to 400 mW.

### O4 Caveat

The mixed-set results above use O3 FCC 20 MHz channels. DJI O4's channels 3–7 differ from their O3 counterparts by 28–61 MHz. For O4:

- R5 (5806) vs. O4-CH5 (5790) = 16 MHz gap — an independent channel, not a near-overlap
- R6 (5843) vs. O4-CH6 (5815) = 28 MHz gap — fully independent

This means O4 has more unique spectral positions than O3 when mixed with raceband, but the specific optimal sets will differ. O4 pilots can also use **Race Mode** (with Goggles 3 or N3), which maps to Race Band R1–R8, making them indistinguishable from analog pilots for spacing purposes.

---

## Practical Recommendations

### For Casual Meetups (2–5 pilots)

- Use the recommended channel sets above
- Keep power at or below 400 mW to maintain full raceband capacity
- DJI pilots: use 20 MHz bandwidth mode
- The 5-pilot mixed set (R1, O3-CH2, R4, R6, R8) is the best option for mixed groups

### For Larger Groups (6+ pilots)

- Let the optimizer auto-assign channels — fixed sets don't buy anything at this density
- 20 MHz bandwidth is critical for DJI pilots at these densities
- Accept that some pilots may need to buddy up (share a channel) at 8+ pilots
- Consider using multiple analog bands (R+F+E) for more frequency options

### For Racing

- Use known IMD-clean channel sets as hard constraints
- 25 mW power (MultiGP standard)
- IMD 6C for 6 pilots is the proven standard
- For 4 or fewer pilots, use the IMD-100 sets (R1/R3/R6/R8, R1/R4/R8, R1/R8)

### General

- Bandwidth is the biggest lever for reducing interference between mixed systems
- When in doubt, more spacing is always better — prefer wider guard bands over tighter packing
- The optimizer maximizes separation, not IMD scores — for race events, use curated channel sets

---

## References

- [ET's FPV IMD Tools](http://www.etheli.com/IMD/) — IMD calculator and optimal frequency set generator
- [Skwad source code](https://github.com/kgNatx/skwad) — optimizer implementation in `freq/tables.go` and `freq/optimizer.go`
- [RCForces 5.8 GHz Guide](https://rcforces.com/blogs/blog5.shtml) — channel tables and IMD-aware pilot recommendations

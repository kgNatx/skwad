# FPV Video Frequency Reference

All frequencies and channel assignments used by Skwad's optimizer. Source of truth: `freq/tables.go`.

## Analog Bands

Analog pilots select which bands their VTX supports. Default is Race Band only.

### R — Race Band (20 MHz)

| Channel | Freq (MHz) |
|---------|-----------|
| R1      | 5658      |
| R2      | 5695      |
| R3      | 5732      |
| R4      | 5769      |
| R5      | 5806      |
| R6      | 5843      |
| R7      | 5880      |
| R8      | 5917      |

### F — Fatshark Band (20 MHz)

| Channel | Freq (MHz) |
|---------|-----------|
| F1      | 5740      |
| F2      | 5760      |
| F3      | 5780      |
| F4      | 5800      |
| F5      | 5820      |
| F6      | 5840      |
| F7      | 5860      |
| F8      | 5880      |

### E — Boscam E Band (20 MHz)

| Channel | Freq (MHz) |
|---------|-----------|
| E1      | 5705      |
| E2      | 5685      |
| E3      | 5665      |
| E4      | 5645      |
| E5      | 5885      |
| E6      | 5905      |
| E7      | 5925      |
| E8      | 5945      |

### L — Low Race Band (20 MHz)

| Channel | Freq (MHz) |
|---------|-----------|
| L1      | 5362      |
| L2      | 5399      |
| L3      | 5436      |
| L4      | 5473      |
| L5      | 5510      |
| L6      | 5547      |
| L7      | 5584      |
| L8      | 5621      |

### Frequency Overlaps

When multiple analog bands are selected, the optimizer deduplicates by frequency. The only overlap across all four bands is:

| Freq (MHz) | Channels      |
|-----------|---------------|
| 5880      | R7, F8        |

All other frequencies are unique across R, F, E, and L.

## HDZero (20 MHz)

Uses Race Band channels (R1–R8). Same table as analog Race Band.

## DJI V1 / Vista (20 MHz)

### Stock (4 channels)

| Channel  | Freq (MHz) |
|----------|-----------|
| DJI-CH3  | 5735      |
| DJI-CH4  | 5770      |
| DJI-CH5  | 5805      |
| DJI-CH8  | 5839      |

### FCC Unlocked (8 channels)

| Channel  | Freq (MHz) |
|----------|-----------|
| DJI-CH1  | 5660      |
| DJI-CH2  | 5695      |
| DJI-CH3  | 5735      |
| DJI-CH4  | 5770      |
| DJI-CH5  | 5805      |
| DJI-CH6  | 5878      |
| DJI-CH7  | 5914      |
| DJI-CH8  | 5839      |

## DJI O3

### Stock 20 MHz (3 channels)

| Channel  | Freq (MHz) |
|----------|-----------|
| O3-CH1   | 5769      |
| O3-CH2   | 5805      |
| O3-CH3   | 5840      |

### FCC 20 MHz (7 channels)

| Channel  | Freq (MHz) |
|----------|-----------|
| O3-CH1   | 5669      |
| O3-CH2   | 5705      |
| O3-CH3   | 5769      |
| O3-CH4   | 5805      |
| O3-CH5   | 5840      |
| O3-CH6   | 5876      |
| O3-CH7   | 5912      |

### FCC 40 MHz (3 channels)

| Channel  | Freq (MHz) |
|----------|-----------|
| O3-CH1   | 5677      |
| O3-CH2   | 5795      |
| O3-CH3   | 5902      |

### Stock 40 MHz (1 channel)

| Channel  | Freq (MHz) |
|----------|-----------|
| O3-CH1   | 5795      |

## DJI O4

### Stock 20 MHz (3 channels)

| Channel  | Freq (MHz) |
|----------|-----------|
| O4-CH1   | 5769      |
| O4-CH2   | 5790      |
| O4-CH3   | 5815      |

### FCC 20 MHz (7 channels)

| Channel  | Freq (MHz) |
|----------|-----------|
| O4-CH1   | 5669      |
| O4-CH2   | 5705      |
| O4-CH3   | 5741      |
| O4-CH4   | 5769      |
| O4-CH5   | 5790      |
| O4-CH6   | 5815      |
| O4-CH7   | 5876      |

### FCC 40 MHz (3 channels)

| Channel  | Freq (MHz) |
|----------|-----------|
| O4-CH1   | 5735      |
| O4-CH2   | 5795      |
| O4-CH3   | 5855      |

### Stock 40 MHz (1 channel)

| Channel  | Freq (MHz) |
|----------|-----------|
| O4-CH1   | 5795      |

### 60 MHz (1 channel)

| Channel  | Freq (MHz) |
|----------|-----------|
| O4-CH1   | 5795      |

### Race Mode (Goggles 3 / N3)

Race mode aligns DJI O4 onto standard race band channels with 20 MHz bandwidth, making it compatible with analog and HDZero channel spacing. The goggles' telemetry link moves onto the same frequency as the video carrier, eliminating the random telemetry hops across the 5.8 GHz band that occur in normal mode.

**Compatible hardware:**
- Air units: O4 Pro, O4 Light (NOT O3 or earlier)
- Goggles: Goggles 3, Goggles N3 (NOT Goggles 2 or Integra)
- Spectator mode works between Goggles 3/N3 only

| Channel  | Freq (MHz) |
|----------|-----------|
| R1       | 5658      |
| R2       | 5695      |
| R3       | 5732      |
| R4       | 5769      |
| R5       | 5806      |
| R6       | 5843      |
| R7       | 5880      |
| R8       | 5917      |

**Settings in race mode:**
- 20 MHz bandwidth, 20 Mbps bitrate
- 1080p @ 100fps, 4:3 only
- Manual channel select (R1–R8), manual power control with fine-tuning
- DVR recording still available
- Latency is reduced compared to normal mode

**Operational notes:**
- Do not power up O4 air units while other pilots are in the air — there is a brief band scan on startup that can cause momentary interference.
- Goggles still transmit a telemetry link (DJI is a two-way system). Maintain physical separation between goggles — don't sit directly on top of other pilots or lap timers.
- In race mode, DJI O4 channels are clean and do not cause out-of-channel interference. Any cross-system interference issues are on the receiving equipment side, not DJI.

## Walksnail Avatar

### Standard Stock (4 channels)

Same as DJI V1 Stock (DJI-CH3, CH4, CH5, CH8).

### Standard FCC (8 channels)

Same as DJI V1 FCC (DJI-CH1–CH8).

### Race Mode (8 channels)

Uses Race Band channels (R1–R8).

## OpenIPC (20 MHz)

| Channel   | Freq (MHz) |
|-----------|-----------|
| WiFi-165  | 5825      |

## Spacing Rules

The optimizer enforces minimum center-to-center spacing between any two pilots:

```
Required Spacing = (Bandwidth_A / 2) + (Bandwidth_B / 2) + 10 MHz guard band
```

| Pilot A | Pilot B | Required Spacing |
|---------|---------|-----------------|
| 20 MHz  | 20 MHz  | 30 MHz          |
| 20 MHz  | 40 MHz  | 40 MHz          |
| 40 MHz  | 40 MHz  | 50 MHz          |
| 40 MHz  | 60 MHz  | 60 MHz          |
| 60 MHz  | 60 MHz  | 70 MHz          |

## System Identifiers

Internal identifiers used in the API and database:

| System          | ID               | Bandwidth | Channel Pool Selection |
|-----------------|------------------|-----------|----------------------|
| Analog          | `analog`         | 20 MHz    | Selected bands (R/F/E/L) |
| HDZero          | `hdzero`         | 20 MHz    | Race Band |
| DJI V1 / Vista  | `dji_v1`         | 20 MHz    | FCC toggle |
| DJI O3          | `dji_o3`         | 20/40 MHz | FCC toggle + bandwidth |
| DJI O4          | `dji_o4`         | 20/40/60 MHz | FCC + bandwidth + race mode + goggles |
| Walksnail Std   | `walksnail_std`  | 20 MHz    | FCC toggle |
| Walksnail Race  | `walksnail_race` | 20 MHz    | Race Band |
| OpenIPC         | `openipc`        | 20 MHz    | Single channel |

---

## Transmit Power and Channel Separation

The optimizer's guard band — the minimum frequency gap it enforces between pilots beyond their occupied bandwidth — isn't a fixed number. It depends on how much power the VTXs in the session are transmitting. Higher power means more spectral splatter and stronger adjacent-channel interference, which demands wider spacing.

### Common VTX Power Steps

Most FPV video transmitters offer adjustable power in fixed steps. The exact steps vary by manufacturer, but these are the most common:

| Power | dBm | Typical Use |
|-------|-----|-------------|
| 25 mW | 14 dBm | Indoor, pit area, crowded racing |
| 100 mW | 20 dBm | Close-range outdoor, warm-up |
| 200 mW | 23 dBm | Standard outdoor group flying |
| 400 mW | 26 dBm | Freestyle, moderate range |
| 600 mW | 27.8 dBm | Long range |
| 800 mW | 29 dBm | Extended range |
| 1000 mW | 30 dBm | Maximum power |

The dBm scale is logarithmic: every +3 dB doubles the power, but it takes +6 dB (4x the power) to double your range. Going from 200 mW to 800 mW is only +6 dB — noticeable, but not transformative for range.

### Why Power Affects Required Spacing

Two factors drive the relationship between TX power and required channel separation:

**1. Spectral splatter.** No VTX transmits a perfectly clean signal on exactly one frequency. Some energy leaks into adjacent frequencies — this is called out-of-band emission or spectral splatter. Higher power amplifiers produce proportionally more splatter because they're driven harder and closer to their nonlinear region. A VTX running at 800 mW has noticeably worse spectral purity than the same VTX at 25 mW.

**2. Receiver filter limitations.** FPV receivers use bandpass filters (typically ~18-20 MHz wide at -3 dB) to reject signals on other channels. These filters have a finite rolloff — they don't brick-wall at the passband edge. The filter attenuates an interfering signal by roughly 20 dB per decade of frequency offset beyond the passband edge. At low power, even modest attenuation is enough. At high power, the interfering signal is strong enough to punch through the filter skirt.

### The Math: Deriving Guard Band from TX Power

The receiver needs a minimum **carrier-to-interference ratio (C/I)** of ~20 dB for clean video. Several factors determine how much frequency separation is needed to achieve that:

- **Capture effect**: Your own quad is typically much closer to your receiver than other pilots' quads. In typical group flying geometry this gives your desired signal a ~20 dB advantage over other pilots' signals.
- **Filter attenuation**: The receiver's bandpass filter (typically ~20 MHz wide at -3 dB) rejects signals at offset frequencies. Attenuation is approximately `20 × log10(offset / (BW/2))` dB beyond the passband edge.
- **Spectral splatter**: Higher TX power degrades spectral purity. We model the additional interference as `3 × log10(P_tx / 25)` dB relative to a 25 mW baseline. This is calibrated against the known-good 10 MHz guard band at 25 mW.

The condition for clean video:

```
filter_attenuation(offset) >= C/I_required - capture_advantage + splatter_penalty
```

With these calibrated values, the guard band formula becomes:

```
guard_band = 10 × 10^((3 × log10(P_mW / 25)) / 20)  MHz
```

This produces 10 MHz at 25 mW (matching the current proven default) and scales gently through the low-power range (25–200 mW). Above 400 mW the formula output stays below 18 MHz, but the table values are manually calibrated overrides designed to place the raceband cliff between 400 and 600 mW.

### Recommended Guard Band by Power Level

The theoretical formula above provides a continuous curve, but in practice we calibrate against a key constraint: **Race Band channels are spaced exactly 37 MHz apart.** This creates a hard cliff — at ≤17 MHz guard band (required spacing ≤37 MHz), all 8 raceband channels are conflict-free. Above 17 MHz, you can only use every-other channel (4 max).

We calibrate so this cliff falls between 400 mW and 600 mW, aligning with community consensus that 400 mW is the upper limit for comfortable group flying:

| Session Power Ceiling | Guard Band | Required Spacing (20 MHz) | Raceband Channels |
|----------------------|-----------|--------------------------|-------------------|
| 25 mW | **10 MHz** | 30 MHz | 8 |
| 100 mW | **12 MHz** | 32 MHz | 8 |
| 200 mW | **14 MHz** | 34 MHz | 8 |
| 400 mW | **16 MHz** | 36 MHz | 8 (1 MHz margin) |
| 600 mW | **24 MHz** | 44 MHz | 4 |
| 800 mW | **28 MHz** | 48 MHz | 4 |
| 1000 mW | **32 MHz** | 52 MHz | 4 |

Key observations:
- **≤400 mW**: All 8 raceband channels remain usable. The guard band grows gently from 10 to 16 MHz, staying safely below the 17 MHz cliff. This matches real-world experience — pilots routinely use adjacent raceband channels at these power levels.
- **600+ mW**: The cliff hits. Only every-other raceband channel is conflict-free (R1/R3/R5/R7 or R2/R4/R6/R8). This is where leaders should consider the race day channel set feature or accept buddying up.
- The jump from 400→600 mW is intentionally dramatic — it signals a real change in the interference environment and matches the community rule of thumb: "stay under 200-400 mW for group flying."

### The Raceband Cliff

The sharp transition at 37 MHz required spacing deserves emphasis because it's unintuitive. Race Band was designed with 37 MHz channel spacing — just barely enough for low-to-moderate power group flying. This means:

- At 36 MHz required spacing (400 mW ceiling): 8 channels, 1 MHz margin. Everyone fits.
- At 38 MHz required spacing: 4 channels. Half the capacity vanishes instantly.

There is no gradual degradation between 8 and 4 on Race Band because the channels are uniformly spaced. With mixed analog bands (R+F+E), the non-uniform spacing provides more intermediate options, but for pure raceband the cliff is real.

For sessions at 600+ mW, the race day channel set feature (#8) becomes essential — the leader should pre-select which 3-4 well-spaced channels to use and let the optimizer buddy up from there.

### Assumptions and Limitations

- The capture advantage (20 dB) assumes typical geometry where your quad is closer to your receiver than other pilots' quads. On a tight race course with stacked quads, this drops to ~10-15 dB, effectively pushing all guard bands wider. A future "race course" mode could account for this.
- The splatter model is calibrated to match the current 10 MHz default at 25 mW, not measured from specific VTX hardware. Cheap VTXs with poor PA linearity will splatter more.
- These calculations assume 20 MHz occupied bandwidth. For DJI O3/O4 at 40 or 60 MHz, the bandwidth terms in the spacing formula dominate and the guard band contribution matters less.
- IMD (intermodulation distortion) is a separate concern not captured by this model. See the IMD section below.

### Impact Summary

The fundamental trade-off: **more power = wider guard band = fewer unique channels = more buddying up.**

For a session leader, the practical question is simple: do you need everyone on a unique channel, or is buddying acceptable? If unique channels matter, keep the power ceiling at 400 mW or below. If power matters more than density, set the ceiling higher and plan for buddying.

---

## DJI Dynamic Power Control

DJI O3 and O4 video systems use automatic power control — the VTX adjusts transmit power dynamically based on link quality. Pilots have no manual mW setting. This has several implications for session power ceilings:

**How DJI dynamic power behaves:**

- Power scales with link quality, which depends on distance, obstacles, and antenna orientation — not a single variable
- At typical meetup distances (~100m line-of-sight), DJI tends to stay at low power (community estimates suggest under 200 mW)
- Power ramps up progressively at range, approaching the maximum EIRP (up to 33 dBm / ~2W for O3/O4 Pro in FCC mode, 30 dBm / ~1W for standard O4) beyond ~500m or through obstacles
- These are community observations from spectrum analyzer measurements, not manufacturer specifications — DJI does not publish their power control curves
- The exact power at a given distance varies significantly with environment, antenna choice, and interference

**Why bandwidth matters more than power for DJI:**

Bandwidth is the lever DJI pilots actually control. Switching from 40 MHz to 20 MHz mode saves 10 MHz of required spacing per neighbor — a bigger impact than most power ceiling steps. The math:

| DJI Bandwidth | vs 20 MHz neighbor (14 MHz guard*) | vs 40 MHz neighbor (14 MHz guard*) |
|--------------|-----------------------------------|-----------------------------------|
| 20 MHz | 34 MHz spacing | 44 MHz spacing |
| 40 MHz | 44 MHz spacing | 54 MHz spacing |
| 60 MHz | 54 MHz spacing | 64 MHz spacing |

*14 MHz guard band = 200 mW power ceiling step. Each 20 MHz step in DJI bandwidth adds 10 MHz to required spacing — equivalent to going from a 200 mW guard band to a 600 mW guard band.

**Practical experience:**

Groups of 6-8 mixed DJI/analog pilots fit comfortably on raceband channels when DJI pilots use 20 MHz bandwidth and fly within a confined area. The dynamic power at these distances stays within a range where the default guard band provides adequate separation.

**Skwad's approach:**

Skwad treats DJI pilots identically to analog in the optimizer — same guard band for the same power ceiling. The app provides guidance rather than enforcement:
- The power ceiling joiner interstitial includes a DJI-specific bandwidth recommendation (when ceiling < 600 mW)
- Bandwidth buttons show visual recommended/warning indicators based on the session's power ceiling
- The optimizer already accounts for DJI's wider bandwidth through the `RequiredSpacing()` formula

This matches Skwad's philosophy: it's a coordinator, not a controller.

---

## Intermodulation Distortion (IMD)

When two VTXs transmit simultaneously, their signals can mix nonlinearly (in the RF front-end of nearby receivers, or even in the transmitters themselves) to produce phantom signals at new frequencies. This is intermodulation distortion.

### How IMD Works

The strongest IMD products are third-order, calculated as:

```
F_imd = (2 × F1) - F2
F_imd = (2 × F2) - F1
```

For example, pilots on 5760 MHz and 5800 MHz produce IMD at:
- (2 × 5760) - 5800 = **5720 MHz**
- (2 × 5800) - 5760 = **5840 MHz**

If a third pilot is on 5840 MHz, they'll see interference from this IMD product even though neither of the other two pilots is on their channel.

### IMD Ratings

IMD quality for a set of frequencies is measured on a 0-100 scale, where 100 means no IMD products land near any active channel:

| Pilot Count | Recommended Channel Set | Frequencies (MHz) | IMD Rating |
|-------------|------------------------|--------------------|-----------|
| 2 pilots | R1, R8 | 5658, 5917 | 100 |
| 3 pilots | R1, R4, R8 | 5658, 5769, 5917 | 100 |
| 4 pilots | R1, R3, R6, R8 | 5658, 5732, 5843, 5917 | 100 |
| 5 pilots | ET5A | 5665, 5752, 5800, 5866, 5905 | 88 |
| 6 pilots | IMD 6C (MultiGP standard) | 5658, 5695, 5760, 5800, 5880, 5917 | 29 |

The sharp drop at 6 pilots illustrates why IMD matters: there simply aren't 6 frequencies in the 5.8 GHz band that avoid all third-order products. The IMD 6C set is a practical compromise that minimizes the worst cases.

### IMD Scoring Methodology

Skwad calculates IMD scores using proximity-weighted quadratic penalties, inspired by [ET's IMD Tools](http://www.etheli.com/IMD/). The approach:

1. Compute all third-order IMD products for every pair of active pilots
2. For each product within 20 MHz of an active channel, calculate: `penalty = (20 - distance)²`
3. A product landing directly on a channel (0 MHz distance) scores 400 penalty points. A product 15 MHz away scores only 25.
4. Final score: `100 - (penalty_sum / (5 × pilot_count))`, clamped to 0-100

This produces a gradient rather than a binary hit/miss — a product 2 MHz from your channel is a real problem, while one 18 MHz away is barely noticeable. The 20 MHz threshold aligns with typical FPV receiver filter bandwidth.

### IMD vs. Guard Band

IMD and guard band spacing address different problems:
- **Guard band** prevents direct adjacent-channel bleed from a single interferer
- **IMD** prevents phantom signals created by the *interaction* of two interferers

A channel can be perfectly spaced from all active channels but still receive IMD interference. Both must be considered for robust multi-pilot sessions.

### References

- [ET's FPV IMD Tools](http://www.etheli.com/IMD/) — IMD calculator and optimal frequency set generator
- [RCForces 5.8 GHz Guide](https://rcforces.com/blogs/blog5.shtml) — channel tables and IMD-aware pilot recommendations
- [PhaserFPV Group Flying Guide](https://phaserfpv.com.au/blogs/fpv-news/what-is-the-best-fpv-channels-to-use-when-flying-with-mates) — practical channel sets by group size

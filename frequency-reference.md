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
| O3-CH3   | 5741      |
| O3-CH4   | 5769      |
| O3-CH5   | 5805      |
| O3-CH6   | 5840      |
| O3-CH7   | 5876      |

### 40 MHz (1 channel)

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

Uses Race Band channels (R1–R8) when race mode is enabled with compatible goggles.

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

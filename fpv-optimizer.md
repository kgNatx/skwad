# Skwad — FPV Frequency Optimizer

## What It Does

Skwad is a frequency coordinator for FPV drone pilots flying together. When multiple pilots show up to fly, everyone needs to be on a different video channel to avoid interference. Skwad handles the channel math so pilots can just scan a QR code, enter their gear info, and get told which channel to use.

## The Problem

FPV video transmitters all share the 5.8 GHz band, but different systems (analog, DJI, HDZero, Walksnail) have different channel tables with different center frequencies and signal widths. A DJI O3 running at 40 MHz bandwidth takes up twice the spectrum of a 20 MHz analog transmitter. You can't just "stay two channels apart" because the channels aren't evenly spaced and the signals aren't the same width.

## Frequency Tables

Skwad knows the channel tables for every major FPV video system. The available channels depend on the pilot's settings: FCC unlock status, goggles (for DJI O4 Race Mode), and bandwidth setting.

### Race Band (Analog, HDZero)

All analog and HDZero systems use the standard Race Band channels:

| Channel | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 |
|---------|----|----|----|----|----|----|----|----|
| **MHz** | 5658 | 5695 | 5732 | 5769 | 5806 | 5843 | 5880 | 5917 |

Occupied bandwidth: 20 MHz per channel.

### DJI V1 / Vista

| Channel | CH1 | CH2 | CH3 | CH4 | CH5 | CH6 | CH7 | CH8 |
|---------|-----|-----|-----|-----|-----|-----|-----|-----|
| **MHz** | 5660 | 5695 | 5735 | 5770 | 5805 | 5878 | 5914 | 5839 |
| **Stock** | | | yes | yes | yes | | | yes |
| **FCC** | yes | yes | yes | yes | yes | yes | yes | yes |

Stock pilots get 4 channels (CH3, CH4, CH5, CH8). FCC unlock enables all 8. Occupied bandwidth: 20 MHz.

### DJI O3

**20 MHz bandwidth:**

| Channel | CH1 | CH2 | CH3 | CH4 | CH5 | CH6 | CH7 |
|---------|-----|-----|-----|-----|-----|-----|-----|
| **MHz** | 5669 | 5705 | 5741 | 5769 | 5805 | 5840 | 5876 |
| **Stock** | | | | yes | yes | yes | |
| **FCC** | yes | yes | yes | yes | yes | yes | yes |

Stock pilots get 3 channels (CH4, CH5, CH6 — labeled CH1-CH3 in stock mode). FCC unlock enables all 7.

**40 MHz bandwidth:**

| Channel | CH1 | CH2 | CH3 |
|---------|-----|-----|-----|
| **MHz** | 5735 | 5795 | 5855 |
| **Stock** | | yes | |
| **FCC** | yes | yes | yes |

Stock gets 1 channel (5795). FCC unlock enables all 3.

### DJI O4 / O4 Pro

**20 MHz bandwidth:**

| Channel | CH1 | CH2 | CH3 | CH4 | CH5 | CH6 | CH7 |
|---------|-----|-----|-----|-----|-----|-----|-----|
| **MHz** | 5669 | 5705 | 5741 | 5769 | 5790 | 5815 | 5876 |
| **Stock** | | | | yes | yes | yes | |
| **FCC** | yes | yes | yes | yes | yes | yes | yes |

**40 MHz bandwidth:**

| Channel | CH1 | CH2 | CH3 |
|---------|-----|-----|-----|
| **MHz** | 5735 | 5795 | 5855 |
| **Stock** | | yes | |
| **FCC** | yes | yes | yes |

Stock gets 1 channel (5795). FCC unlock enables all 3.

**60 MHz bandwidth:** Single channel at 5795 MHz.

**Race Mode (Goggles 3 / Goggles N3 only):** Uses standard Race Band (R1-R8), same as analog. Not available with Goggles 2 or Integra.

### Walksnail Avatar

- **Standard mode:** Same channel table as DJI V1 (4 stock / 8 FCC). Occupied bandwidth: 20 MHz.
- **Race mode:** Uses standard Race Band (R1-R8). Occupied bandwidth: 20 MHz.

### OpenIPC / Other

Single channel: WiFi-165 at 5825 MHz. Occupied bandwidth: 20 MHz.

## How Spacing Works

The optimizer calculates the required separation for each pair of pilots based on their signal widths and the session's power ceiling.

**Occupied bandwidth** is how wide the signal actually is:
- Analog, HDZero, DJI V1, Walksnail: **20 MHz**
- DJI O3/O4 at 20 MHz setting: **20 MHz**
- DJI O3/O4 at 40 MHz setting: **40 MHz**
- DJI O4 at 60 MHz setting: **60 MHz**

**Required spacing** between two pilots is:

```
(pilot A bandwidth / 2) + (pilot B bandwidth / 2) + guard band
```

### Power-Dependent Guard Band

The guard band depends on the session's TX power ceiling. The leader sets this during session creation. Higher power means wider guard bands because stronger signals bleed further outside their nominal bandwidth.

`PowerToGuardBand()` maps power ceiling to guard band:

| Power Ceiling | Guard Band |
|---------------|------------|
| No ceiling / 25 mW | 10 MHz |
| 100 mW | 12 MHz |
| 200 mW | 14 MHz |
| 400 mW | 16 MHz |
| 600 mW | 24 MHz |
| 800 mW | 28 MHz |
| 1000 mW | 32 MHz |

**The raceband cliff:** At 400 mW or below (guard band 10-16 MHz), required spacing between two 20 MHz signals is 30-36 MHz. Race Band channels are 37 MHz apart, so all 8 channels fit. At 600 mW (guard band 24 MHz), required spacing jumps to 44 MHz — only every other Race Band channel has enough room, dropping you to 4 usable channels.

**Spacing examples at default guard band (10 MHz):**
- Two analog pilots (20 + 20): **30 MHz** center-to-center
- Analog next to DJI O3 at 40 MHz (20 + 40): **40 MHz** center-to-center
- Two DJI O4 at 40 MHz (40 + 40): **50 MHz** center-to-center
- DJI O4 at 60 MHz next to analog (60 + 20): **50 MHz** center-to-center

## How the Optimizer Assigns Channels

The optimizer (`Optimize()`) runs every time a pilot joins, leaves, or changes their channel preference. It works in three steps.

### Step 1: Place pinned pilots

Pilots with `Pinned=true` are placed at their `PinnedFreqMHz` unconditionally. Pinning is set internally by `OptimizeWithLocks()` — it's never stored in the database. This mechanism is used during displacement resolution and rebalancing to lock certain pilots in place while the optimizer works around them.

### Step 2: Assign flexible pilots, most constrained first

Remaining pilots are sorted: preference pilots first (they have smaller effective pools), then auto-assign pilots. Within each group, sorted by channel pool size ascending — fewest options first.

For each pilot, the optimizer scores every channel in their pool by **margin** — the gap between actual center-to-center separation and the required spacing for the worst-case pair. Higher margin means less chance of interference.

Channel selection logic:
- **If clean slots exist** (positive margin available): prefer the pilot's preferred frequency if clean, then their previous frequency for stability, then best margin overall.
- **If all slots are negative** (buddying unavoidable): pick the least-loaded frequency to balance buddy groups, tiebreak on margin.

### Step 3: Buddy groups

When multiple pilots end up on the same frequency — because there are more pilots than channels, or because fixed channels force sharing — they're marked as a buddy group. The UI highlights buddy groups with matching colored borders and "SHARING WITH" labels. They can still fly but will interfere with each other on that channel.

## Fixed Channels

The leader can select a preset channel set (2-5 frequencies) during session creation. When fixed channels are active, `Optimize()` receives a `fixedFreqs` list and filters every pilot's channel pool to only those frequencies. If a pilot's system has no channels matching the fixed set, they get the fixed frequencies directly as synthetic channels and buddy up on the closest match.

When all fixed channels have a pilot, overflow pilots buddy up on the least-loaded channel. Preset channel sets are optimized for spacing and IMD across different system mixes (analog-only, DJI-only, mixed).

## Graduated Escalation (FindMinimalDisplacement)

When a new pilot joins, `FindMinimalDisplacement` tries to place them with minimal disruption. Two levels:

### Level 0: Lock and slot

Lock all existing pilots at their current frequencies, let the optimizer place only the new pilot. If the result has no danger-level conflicts involving moved pilots, done. This is the fast path — most joins land here.

### Level 1: Build options

If Level 0 produces a danger conflict, the pilot gets two options to choose from:

**Buddy option:** Find the most compatible existing pilot to share a frequency with. Prefers same video system and similar bandwidth.

**Rebalance option:** Try unlocking one flexible pilot at a time (most flexible first), re-run the optimizer. If that doesn't produce a clean solution, try pairs of flexible pilots. Take the first clean result.

The `flexiblePilots()` function ranks ALL pilots by a 3-tier flexibility score:
- **Score 3** — auto-assign (no preference): most flexible, move first
- **Score 2** — has a preference but not currently on that channel
- **Score 1** — on their preferred channel with tenure: least flexible, move last

Tiebreaker within each tier: larger channel pool = more flexible.

## Rebalance

Leader-only operation. The `reoptimize()` function uses a two-phase approach:

1. **Surgical:** Run the optimizer normally, detect conflicts. Pin all non-conflicting pilots, re-optimize only the conflicted ones. If this reduces conflicts, use it.
2. **Full fallback:** If surgical doesn't improve things, the initial full optimization result is used (all pilots unlocked).

No automatic rebalance on pilot leave — stability-first design. The leader sees a "rebalance recommended" indicator when conflicts exist and can trigger it manually. A preview endpoint lets the leader see before/after assignments before committing.

## Conflict Detection

After optimization, `DetectConflicts` checks every pair of pilots:

- **Danger** (red): The signals actually overlap — center-to-center separation is less than `(BW_A/2 + BW_B/2)`. They will definitely interfere.
- **Warning** (amber): Separation is within the guard band — less than `RequiredSpacing` but no signal overlap. Interference is likely with reflections and multipath.

Conflicts appear on pilot cards with text like "OVERLAP WAYNE (26/40 MHz)" showing actual separation vs. required.

## IMD Scoring

Intermodulation distortion (IMD) occurs when two transmitters produce phantom signals at frequencies derived from their fundamentals. The third-order product `F3 = (2 * F1) - F2` is the most problematic because it falls close to the active band.

Skwad calculates all third-order IMD products for every pilot pair and scores the session 0-100:
- Products within 12 MHz of an active channel are flagged as hits
- Proximity-weighted scoring: products closer to active channels penalize more (quadratic weighting, 20 MHz threshold)
- 100 = no IMD issues, lower = more interference risk
- Per-pilot IMD flags identify which two source pilots create each interfering product

The IMD score displays as a badge in the session header (green/amber/red) and IMD products appear on spectrum visualizations and channel picker previews. The fixed channel presets include pre-calculated IMD scores so leaders can pick IMD-clean sets.

## User Workflows

### Starting a Session

1. One pilot taps **START SESSION**
2. Optional: set a TX power ceiling (25-1000 mW slider) and/or pick a fixed channel set
3. Gets a 6-character session code and QR code to share

### Joining a Session

1. Enter callsign
2. Pick video system (Analog, DJI V1, DJI O3, DJI O4, HDZero, Walksnail, OpenIPC)
3. Follow-up questions based on system:
   - FCC unlocked? (DJI V1, O3, O4, Walksnail Standard)
   - Which goggles? (DJI O4)
   - Bandwidth? (DJI O3, O4)
   - Race Mode? (DJI O4 with Goggles 3/N3)
   - Standard or Race Mode? (Walksnail)
   - Bands? (Analog — Race, Fatshark, Boscam E, Low Race)
4. Channel preference: **AUTO-ASSIGN** (recommended) or **I HAVE A PREFERENCE** (soft preference, not a lock — the optimizer will honor it when possible but may override it to avoid conflicts)
5. Hit JOIN — you get your optimized channel assignment

If the session has a power ceiling, an interstitial shows the limit before the video system step. DJI pilots see bandwidth guidance when the ceiling is below 600 mW.

### Displacement (Level 1)

When no clean channel exists for a joining pilot, they get two options:

- **Buddy up** — share a frequency with a compatible existing pilot
- **Partial rebalance** — move one or two flexible pilots to make room

The pilot picks which option to use. If their preference can't be honored at Level 0, they see an override dialog explaining why, with the spectrum visualization showing the conflict.

### Channel Change Notification

If another pilot's join or a rebalance moves your channel, you see a "GOT IT" dialog showing your old and new assignment. This reminds you to actually change your VTX channel.

### Managing Your Session

- **Tap your own card** to change channel, change video system, change callsign, or leave
- **Channel change** offers three options: auto-assign new channel, pick a preference, or change video system entirely (removes and re-adds you through the setup wizard)
- **Leader powers:** tap other pilot cards to remove them (slide-to-confirm), add phantom pilots, rebalance all, transfer leadership
- **Tap the session code** to show a fullscreen QR code for sharing

### Spectrum Visualization

The session footer shows a bandwidth-aware spectrum visualization of the 5.8 GHz band (5640-5930 MHz). Each pilot appears as a bell-curve waveform whose width represents their occupied bandwidth. Race Band channel names (R1-R8) are labeled along the baseline. IMD products appear as markers on the spectrum. Colors indicate status: green (you), red (danger conflict), orange (warning conflict), gray (clean).

### Live Updates

Every client polls for changes every 5 seconds. When any pilot joins, leaves, changes channels, or changes their callsign, all clients automatically refresh to show the updated assignments.

## Sessions

Sessions expire after 12 hours. Each session has a unique 6-character hex code. Multiple sessions can run simultaneously for different groups at the same field. The first pilot to join is the session leader — leadership can be explicitly transferred but doesn't auto-rotate.

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

**40 MHz bandwidth:** Single channel at 5795 MHz (same for stock and FCC).

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

The optimizer doesn't use a single fixed spacing number. Instead, it calculates the required separation for each pair of pilots based on their actual signal widths.

**Occupied bandwidth** is how wide the signal actually is:
- Analog, HDZero, DJI V1, Walksnail: **20 MHz**
- DJI O3/O4 at 20 MHz setting: **20 MHz**
- DJI O3/O4 at 40 MHz setting: **40 MHz**
- DJI O4 at 60 MHz setting: **60 MHz**

**Required spacing** between two pilots is:

```
(pilot A bandwidth / 2) + (pilot B bandwidth / 2) + 10 MHz guard band
```

Examples:
- Two analog pilots (20 + 20): need **30 MHz** center-to-center
- Analog next to DJI O3 at 40 MHz (20 + 40): need **40 MHz** center-to-center
- Two DJI O4 at 40 MHz (40 + 40): need **50 MHz** center-to-center
- DJI O4 at 60 MHz next to analog (60 + 20): need **50 MHz** center-to-center

The 10 MHz guard band provides a safety margin beyond the signal edges.

## How the Optimizer Assigns Channels

The optimizer runs every time a pilot joins, leaves, or changes their channel preference. It works in three steps:

### Step 1: Lock in fixed-channel pilots first

Some pilots can't change channels (e.g., their VTX is set and they don't want to change it, or their system only has one channel option like DJI O3 at 40 MHz). These get placed first, exactly where they requested.

### Step 2: Assign flexible pilots, most constrained first

Remaining pilots are sorted by how many channels they have available — fewest options first. A DJI O3 stock pilot with 3 channels gets assigned before an analog pilot with 8 channels, because the analog pilot has more fallback options.

For each pilot, the optimizer tries every channel in their pool and picks the one with the best **margin** — the gap between actual center-to-center separation and the required spacing for that pair. Higher margin = less chance of interference.

**Stability preference:** If a pilot was already on a channel and it still has non-negative margin (meaning it meets the spacing requirement), the optimizer prefers to keep them there rather than shuffling everyone around. This prevents unnecessary channel changes when a new pilot joins.

### Step 3: Buddy groups

If there are more pilots than available channels (e.g., four DJI O3 stock pilots but only three O3 channels), some pilots have to share a frequency. The optimizer marks these as a "buddy group" — they can still fly, but need to take turns or accept that they'll interfere with each other. The UI highlights buddy groups with matching colored borders and "SHARING WITH: [name]" labels.

## Conflict Detection

After optimization, Skwad checks every pair of pilots for conflicts:

- **Danger** (red): The signals actually overlap — center-to-center separation is less than half of each pilot's bandwidth combined. This means they will definitely interfere.
- **Warning** (amber): The separation is less than the required spacing (signals don't overlap but are within the guard band). Interference is likely, especially with reflections and multipath.

Conflicts appear on pilot cards with text like "OVERLAP WAYNE (26/40 MHz)" or "CLOSE TO CRASH (32/40 MHz)" showing actual separation vs. required.

## User Workflows

### Starting a Session

1. One pilot taps **START SESSION** — gets a 6-character code
2. They share the code or QR code with the group
3. Other pilots scan the QR or enter the code to join

### Joining a Session

1. Enter your callsign
2. Pick your video system (Analog, DJI V1, DJI O3, DJI O4, HDZero, Walksnail, OpenIPC)
3. Answer follow-up questions based on your system:
   - FCC unlocked? (DJI V1, O3, O4, Walksnail Standard)
   - Which goggles? (DJI O4)
   - Bandwidth? (DJI O3, O4)
   - Race Mode? (DJI O4 with Goggles 3/N3)
   - Standard or Race Mode? (Walksnail)
4. Choose channel preference: **auto-assign** (recommended) or **lock to a specific channel**
   - Channel preference buttons are styled as toggles (dark bg, white border when active) — distinct from the solid white JOIN action button
   - A **BACK** button below JOIN returns to the video system selection to change gear settings
5. Hit JOIN — you get your optimized channel assignment

### Displacement Preview

If your join or channel change would cause existing pilots to move to different channels, you see a confirmation dialog showing each affected pilot and where they'd move:

> **CRASH**
> R1 (5658) → R7 (5880)
>
> VERIFY WITH THEM BEFORE CONTINUING
>
> [MOVE EVERYONE] [JUST MOVE ME] [CANCEL]

Three options:
- **MOVE EVERYONE** — full rebalance, applies the optimizer's ideal assignments for all pilots
- **JUST MOVE ME** — only applies your new assignment, leaves everyone else where they are. This option is hidden if it would cause a danger-level overlap (red zone). Warning-level proximity (yellow zone) is allowed.
- **CANCEL** — back out, nothing changes

This dialog appears for both initial joins and in-session channel changes.

### Channel Change Notification

If you're already in a session and someone else's join moves your channel, you see an amber banner:

> YOUR CHANNEL CHANGED: R1 (5658) → R7 (5880)
> COORDINATE WITH YOUR GROUP BEFORE SWITCHING
>
> [GOT IT]

This reminds you to actually change your VTX channel and lets you coordinate timing with the group.

### Managing Your Session

- **Tap your own card** to access options: change channel/video, change callsign, or leave session
- **Change channel** opens a picker with auto-assign and manual channel selection. Also includes **CHANGE VIDEO SYSTEM** which removes you from the session and sends you back to the setup wizard (callsign pre-filled) so you can rejoin with different gear settings.
- **Tap another pilot's card** to access removal — uses a slide-to-confirm gesture to prevent accidental removal
- **Tap the session code** to show a fullscreen QR code for sharing

### Spectrum Visualization

The session view footer shows a bandwidth-aware spectrum visualization of the 5.8 GHz band (5640–5930 MHz). Each pilot appears as a bell-curve waveform whose width represents their occupied bandwidth, with their callsign above. Race Band channel names (R1–R8) are labeled above the baseline, with their center frequencies (5658–5917 MHz) shown below.

Colors indicate status:
- **Green** — you (the current pilot)
- **Red** — danger-level conflict (signal overlap)
- **Orange** — warning-level conflict (within guard band)
- **Gray** — no conflicts

Labels are vertically staggered when pilots are on nearby frequencies so callsigns don't overlap.

### Session View Layout

The session screen uses a sticky header (session code + LIVE indicator + pilot count) and sticky footer (spectrum canvas). Only the pilot card list scrolls between them. This keeps the session code and spectrum always visible.

### Live Updates

Every client polls for changes every 5 seconds. When any pilot joins, leaves, changes channels, or changes their callsign, all clients automatically refresh to show the updated assignments.

## Sessions

Sessions expire after 24 hours. Each session has a unique 6-character hex code. Multiple sessions can run simultaneously for different groups at the same field.

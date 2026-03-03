# Skwad: FPV Frequency Coordinator

Design document for a mobile-first web app that manages video frequencies at FPV drone meetups to prevent interference.

## Problem

At Austin FPV Flyers meetups, 5-15 pilots show up with a chaotic mix of video systems (DJI, HDZero, analog, Walksnail) that all share 5.8GHz spectrum. Today, channels are assigned informally using Race Band 1-8, but:

- DJI O3/O4 channels don't align to Race Band without FCC hack + Race Mode
- Some pilots can't change channels (locked VTX, stock firmware)
- When pilot count exceeds available channels, frequency sharing is inevitable
- Cross-system interference rules are complex and nobody memorizes them
- There's no record of who's on what — new arrivals ask around

## Solution

A lightweight web app at `skwad.atxfpv.org` where pilots join a session, declare their video system, and get optimal channel assignments. The app handles the frequency math, buddy pairing when channels must be shared, and shows a live view of who's on what.

## Architecture

Single Go binary serving both the API and static frontend. SQLite for session/pilot storage. Runs as one Docker container behind Traefik, matching the existing atxfpv.org infrastructure pattern.

```
[Browser] <--HTTP--> [Go server (API + static files)] <---> [SQLite]
                           |
                     [Traefik] (TLS, routing skwad.atxfpv.org)
```

No build step for the frontend — plain HTML/CSS/JS. Mobile-first responsive design with big buttons and large text for outdoor readability.

Polling (every 5-10 seconds) for live updates in V1. No WebSocket complexity.

## Sessions

- Any pilot can create a session — no host/admin role
- Session gets a 6-character alphanumeric code (e.g., `FPV42X`)
- QR code generated client-side encoding the join URL: `skwad.atxfpv.org/s/FPV42X`
- Sessions auto-expire 24 hours after creation
- No accounts, no login — session-scoped identity only (V1)

## Pilot Onboarding Flow

When a pilot joins a session:

1. **Enter callsign** (free text, displayed to everyone)
2. **Pick video system** (large tap targets):
   - Analog 5.8GHz
   - DJI V1 / Vista
   - DJI O3
   - DJI O4 / O4 Pro
   - HDZero
   - Walksnail Avatar
   - OpenIPC / Other
3. **System-specific follow-ups**:
   - **DJI V1/Vista**: FCC unlocked? (Yes → 8ch, No → 4ch)
   - **DJI O3**: FCC unlocked? (Yes → 7ch at 20/10MHz, No → 3ch) → Bandwidth? (10/20/40MHz)
   - **DJI O4**: FCC unlocked? → Which goggles? (Goggles 3/N3 → Race Mode available with 8ch R1-R8, Goggles 2/Integra → standard manual) → Bandwidth? (10/20/40/60MHz)
   - **Walksnail**: FCC unlocked? → Race Mode? (Yes → R1-R8, No → standard 8ch)
   - **Analog/HDZero**: No follow-ups needed (full Race Band access)
4. **Channel preference** (toggle):
   - Default: "Assign me the best channel" (auto-assign)
   - Toggle: "I'm locked to a specific channel" → pick from available channels for their system
5. **Join session** → optimizer runs, assignment displayed

## Data Model

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,          -- 6-char code
    created_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL  -- created_at + 24h
);

CREATE TABLE pilots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    callsign TEXT NOT NULL,
    video_system TEXT NOT NULL,   -- analog, dji_v1, dji_o3, dji_o4, hdzero, walksnail_std, walksnail_race, openipc
    fcc_unlocked BOOLEAN DEFAULT FALSE,
    goggles TEXT,                 -- goggles_3, goggles_n3, goggles_2, goggles_integra (DJI only)
    bandwidth_mhz INTEGER,       -- 10, 20, 40, 60 (DJI/Walksnail only)
    race_mode BOOLEAN DEFAULT FALSE, -- O4 Race Mode or Walksnail Race Mode
    channel_locked BOOLEAN DEFAULT FALSE,
    locked_frequency_mhz INTEGER,    -- only if channel_locked
    assigned_channel TEXT,        -- human-readable: "R3", "DJI-CH2", etc.
    assigned_frequency_mhz INTEGER,
    buddy_group INTEGER,         -- pilots sharing a frequency get same group number
    joined_at DATETIME NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    UNIQUE(session_id, callsign)
);
```

## Video System Frequency Tables

### Analog 5.8GHz — Race Band (R1-R8)

| Channel | Frequency (MHz) |
|---------|----------------|
| R1 | 5658 |
| R2 | 5695 |
| R3 | 5732 |
| R4 | 5769 |
| R5 | 5806 |
| R6 | 5843 |
| R7 | 5880 |
| R8 | 5917 |

Spacing: 37MHz. Bandwidth per channel: ~25MHz.

### DJI V1 / Vista (FCC, 8 channels)

| Channel | Frequency (MHz) |
|---------|----------------|
| CH1 | 5660 |
| CH2 | 5695 |
| CH3 | 5735 |
| CH4 | 5770 |
| CH5 | 5805 |
| CH6 | 5878 |
| CH7 | 5914 |
| CH8 | 5839 |

Stock (CE): CH3, CH4, CH5, CH8 only.

### DJI O3 — Manual Mode

| Bandwidth | Channels (Stock) | Channels (FCC) |
|-----------|-----------------|-----------------|
| 40MHz | 1 | 1 |
| 20MHz | 3 | 7 |
| 10MHz | 3 | 7 |

Stock 20/10MHz channels: CH1 (5768.5), CH2 (5804.5), CH3 (5839.5).
FCC 20/10MHz: 7 channels (exact frequencies TBD — need to capture from pilot devices).

### DJI O4 / O4 Pro — Manual Mode

| Bandwidth | Channels (Stock) | Channels (FCC) |
|-----------|-----------------|-----------------|
| 60MHz | 1 | 1 |
| 40MHz | 1 | 3 |
| 20MHz | 3 | 7 |
| 10MHz | 3 | 7 |

**Race Mode (Goggles 3/N3 only):** 8 channels on Race Band R1-R8 (5658-5917MHz). 20MHz or 40MHz carrier. Forces 1080p/100fps.

### HDZero

Uses exact Race Band frequencies (R1-R8). Also supports F1, F2, F4, E1.
Standard bandwidth: 27MHz. Narrow mode: 17MHz.

### Walksnail Avatar

**Standard mode (FCC):** 8 channels matching DJI V1 frequency map.
**Race Mode:** R1-R8 on exact Race Band frequencies + RP at 5500MHz.

## Frequency Optimizer Algorithm

### Priority Order

1. **Lock in fixed-channel pilots** — immovable constraints
2. **Assign unique channels** to remaining pilots, maximizing minimum frequency separation across all active transmitters
3. **When unique channels are exhausted**, create buddy groups — pair pilots on shared frequencies
4. **Stability preference** — minimize reassignments when a new pilot joins or leaves; prefer adding to an open slot over reshuffling everyone

### Channel Pool Per System

The optimizer builds each pilot's available channel pool based on:
- Video system type
- FCC unlock status
- Bandwidth setting
- Race Mode availability
- Goggles (for O4)

### Separation Rules

- Minimum safe separation: 37MHz (analog/HDZero channel spacing)
- IMD consideration: for analog/HDZero, avoid channel combos that produce 3rd-order IMD products on occupied channels
- Cross-system: digital and analog on the same center frequency interfere; treat them identically for separation purposes
- DJI O3 and O4 have different center frequencies at the same bandwidth — the optimizer must use the actual MHz values, not channel numbers

### Buddy System

When the optimizer can't assign a unique frequency:
- Create a buddy group (integer ID)
- Prefer pairing same-system pilots (two DJI on one DJI channel is better than DJI+analog overlap)
- Display buddy groups prominently in the UI
- Pilots in a buddy group coordinate in person (turn-taking)
- Minimize total buddy groups — spread sharing evenly rather than tripling up

### Optimizer Trigger

Re-runs whenever:
- A pilot joins the session
- A pilot leaves (marks inactive)
- A pilot changes their video system or channel lock

Existing assignments are preserved unless the optimizer finds a strictly better arrangement. A pilot who has been flying on R3 for 20 minutes should not be moved unless necessary.

## UI Design

### Principles
- Mobile-first, designed for direct sunlight on phone screens
- **Extreme high contrast**: near-black background (#0a0a0a or similar), bright white text, vivid accent colors. No subtle grays, no transparency effects that reduce contrast. Every element must be readable in direct Texas sun.
- Large text: minimum 18px body, 28px+ for channel assignments, 36px+ for callsigns
- Bold weight for all critical info (channel, callsign)
- Big tap targets (minimum 48px, prefer 64px+) — gloved hands and sweaty fingers
- Minimal scrolling — most important info visible without scroll
- No thin fonts, no light font weights, no low-contrast placeholder text
- Color coding for buddy groups uses saturated, distinct hues (not pastels)

### Screens

**1. Landing Page**
- App name "Skwad" + tagline
- Two big buttons: "Start Session" / "Join Session"
- Join accepts 6-char code or QR scan

**2. Pilot Setup** (join flow)
- Step-by-step, one question per screen
- Large system icons/buttons for video system selection
- Conditional follow-ups based on system choice

**3. Session View** (main screen)
- Session code + QR code at top (tap to copy/share)
- **Spectrum visualization**: horizontal bar showing 5.6-5.9 GHz with colored blocks per pilot
- **Pilot list**: sorted by frequency, showing:
  - Callsign (large, bold)
  - Video system icon
  - Channel assignment (very large: "R3" or "DJI CH2")
  - Frequency in MHz (smaller)
  - Buddy indicator if sharing
- Buddy groups visually connected (same color band, grouped together)
- "I'm leaving" button (marks pilot inactive, triggers re-optimization)
- Auto-refresh via polling (5-10 second interval)

**4. Join QR / Share**
- Full-screen QR code for easy scanning at the field
- Session URL displayed below for manual entry

## Deployment

### Container Setup

New Docker Compose service alongside the existing atxfpv-org Nginx container:

```yaml
services:
  skwad:
    image: ghcr.io/kyleg/skwad:latest  # or build locally
    container_name: skwad
    volumes:
      - ./skwad/data:/data  # SQLite database
    networks:
      - traefik
    labels:
      - traefik.enable=true
      - traefik.http.routers.skwad-https.rule=Host(`skwad.${SITE_DOMAIN}`)
      - traefik.http.routers.skwad-https.entrypoints=websecure
      - traefik.http.routers.skwad-https.tls.certresolver=cloudflare
      - traefik.http.services.skwad.loadbalancer.server.port=8080
    restart: unless-stopped
```

### DNS

Add `skwad.atxfpv.org` A/CNAME record pointing to the same server.

## API Endpoints

```
POST   /api/sessions              → Create session (returns code + QR URL)
GET    /api/sessions/:code        → Get session with all pilots
POST   /api/sessions/:code/join   → Join session (callsign + system config)
PATCH  /api/pilots/:id            → Update pilot (change system, toggle active)
DELETE /api/pilots/:id            → Remove pilot from session
GET    /api/sessions/:code/poll   → Lightweight poll (returns version number + assignments if changed)
```

## Future (Not V1)

- Pilot accounts with persistent preferences (email or SMS auth)
- WebSocket for real-time push updates
- Heat management for organized races
- Spectrum analyzer integration
- Power level tracking and recommendations
- Location-based auto-session
- Historical session data and frequency usage stats

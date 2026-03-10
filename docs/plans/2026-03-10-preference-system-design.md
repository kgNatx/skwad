# Preference System Design

## Overview

Replace the "channel lock" mechanism with a preference-and-tenure model. No pilot is ever permanently locked to a channel. Preferences guide placement, tenure (first-come-first-served) protects existing assignments, and the optimizer balances individual preference against overall session quality.

## Core Model

### Two Concepts Replace "Channel Lock"

**Preference** — what channel you'd like. Set at join time or changed later. The optimizer tries to honor it. If it can't, it assigns the best available and tells you why. Preferences persist across rebalances — the optimizer tries to keep you near your preference when possible.

**Tenure** — first-come-first-served protection. Existing pilots don't get bumped by a new arrival's preference. Tenure is implicit — your current assignment (`PrevFreqMHz`) is your stability anchor. Only these things can move a tenured pilot:
- A rebalance (leader-initiated)
- A technical constraint (video system change shrinks channel pool)
- The optimizer determining that moving a preference pilot who has available pool improves the session for everyone
- The pilot's own choice

### No Locks Exist

The `channel_locked` boolean and `locked_frequency_mhz` integer are removed entirely. The database stores only `preferred_frequency_mhz` (0 = auto-assign). There is no `leader_locked` flag — a leader force-placement is stored as a regular preference and can be rebalanced later.

Preferences are weights, not walls. The optimizer uses them as strong signals but can override when overall session quality benefits.

### Leader Force-Placement

When the leader forces a pilot onto a conflicting channel, it's stored as a regular preference. The conflict is accepted at request time but the placement is not permanent. Two cases:
- **Same frequency as another pilot**: creates a buddy group (intentional channel sharing)
- **Adjacent overlap (wide-band bleed)**: warning accepted, no buddy group, just a known conflict

---

## Join Flow

### Channel Preference Step (after video system selection)

Two buttons:
- **AUTO-ASSIGN** — system picks the best available channel
- **I HAVE A PREFERENCE** — opens channel picker

When "I HAVE A PREFERENCE" is tapped:
- Friendly message at top: *"This is just a preference — sometimes we shuffle to fit everyone."*
- Channel picker buttons
- Spectrum visualization showing existing pilots
- Selecting a channel shows preview hump on spectrum
- JOIN SESSION button

### Join Outcomes

**1. Clean placement** — preference honored (or auto-assign finds a clean slot). Pilot lands in session. No special message.

**2. Preference overridden** — preferred channel conflicts or is taken. Optimizer assigns best clear alternative. Pilot sees a dismissable dialog: *"R2 conflicts with RZA's signal. You've been placed on R4 (5769 MHz)."* with GOT IT button. Spectrum visible for context.

**3. Session is crowded** — no clean channel available (8+ pilots territory). Pilot sees a choice dialog:
- *"BUDDY UP — Share R3 (5732 MHz) with PONY"*
- *"PARTIAL REBALANCE — Move PILOT_X from R3 to R5, you get R3"*

Pilot picks. If partial rebalance, affected pilots see a notification with GOT IT confirmation on their next poll.

Auto-assign follows the same logic but skips the preference picker.

---

## Self-Service Channel Change

When a pilot taps their own card and hits "Change Channel," three options:

- **CHANGE VIDEO SYSTEM** — video system picker, then re-runs assignment
- **AUTO-ASSIGN NEW** — optimizer picks a different channel than current (keyword: NEW — always different)
- **I HAVE A PREFERENCE** — channel picker with spectrum, same friendly message

### Preference Flow
- Channel picker + spectrum (same as join)
- Conflicting channels NOT shown for regular pilots (only clear channels)
- Confirm button: "USE THIS CHANNEL"

### Outcomes
Same three outcomes as join: clean placement, preference overridden (dialog with GOT IT), or crowded session (buddy/rebalance choice).

### Preference Persistence
The stored preference survives rebalances. If a pilot prefers R1 because their quads work better on that channel or there's less environmental interference, the optimizer tries to keep them there. First-come-first-served tenure reinforces this.

---

## Leader Flow

### Leader Changes Another Pilot's Channel

Same three options as self-service:
- **CHANGE VIDEO SYSTEM**
- **AUTO-ASSIGN NEW**
- **I HAVE A PREFERENCE**

### Key Difference: Force Conflicting Channels

- Channel picker shows ALL channels
- Conflicting channels are **grayed out but still tappable**
- When leader taps a grayed-out channel:
  - **Same frequency as existing pilot**: *"This will buddy PAPAISABIGBOSS with PONY on R3 (5732 MHz). They'll share the channel."* — BUDDY UP / CANCEL. Creates buddy group.
  - **Adjacent overlap (wide-band bleed)**: *"R2 overlaps with RZA's O4 signal. 37 MHz separation vs 40 MHz recommended."* — FORCE / CANCEL. No buddy group.
- Force placement is stored as a regular preference — future rebalance can move them.

### Leader-Added (Phantom) Pilots
Same flow. Leader can also change their video system since they created them.

### Rebalance All

Two-phase approach:
1. **Surgical (try first)**: fix conflicts and tight spacing, keep clean pilots in place. Respect stored preferences when deciding where to move displaced pilots.
2. **Full re-optimize (fallback)**: if surgical can't resolve, recalculate everyone. Preferences guide placement but everyone is eligible to move.

Result dialog shows who moved and any unresolved conflicts (same as today).

### Rebalance Recommended Indicator

When the session accumulates conflicts or tight spacing that a rebalance would improve, a subtle indicator appears on the leader's session screen: *"Rebalance recommended."* Not a blocking alert — just a visual nudge. Goes away after rebalance.

---

## Feedback & Notifications

All "you've been moved" messages require a GOT IT confirmation button.

### Pilot-Facing
- **Preference overridden** (join or channel change): dialog naming the conflict, showing assignment. GOT IT button.
- **Moved by partial rebalance**: on next poll, dismissable message — *"You've been moved to R5 (5806 MHz) to make room. Talk to [LEADER], the session leader, if you have questions."* GOT IT button.
- **Buddy group created**: BUDDIES badge on card (already exists).

### Leader-Facing
- **Rebalance recommended**: indicator on session screen when optimizer detects improvable conflicts/spacing.
- **Force placement warnings**: buddy-up confirmation or overlap warning (as described above).
- **Rebalance result**: dialog showing moved pilots and unresolved conflicts (already exists).

---

## Optimizer Changes

### Current Model
Two-way split: locked (immovable) vs flexible (can be moved). Four escalation levels (0-3) that silently try progressively disruptive strategies.

### New Model

**Preference-weighted placement** with three inputs per pilot:
1. Preferred frequency (0 = no preference)
2. Current assignment / tenure anchor (`PrevFreqMHz`)
3. Channel pool (determined by video system)

**Placement priority:**
1. Tenured pilots on a clean channel → don't touch
2. Pilots with a preference that's clean → honor it
3. Pilots with a preference that conflicts → best available, explain why
4. Auto-assign pilots → best available by margin

Preferences are weights in the scoring, not hard constraints. Moving a preference pilot who has available pool is acceptable if it improves overall session quality.

### Simplified Escalation

- **Level 0**: Place new pilot without moving anyone. Try preference first, fall back to best available.
- **Level 1**: No clean channel. Present the pilot with a choice: buddy up with best match, OR partial rebalance (move the most flexible existing pilot). Both options shown — pilot decides. Pilots on auto-assign (no preference) are the most flexible candidates for rebalance.
- **Level 2**: Full rebalance (leader-only, via Rebalance All). Surgical first, full re-optimize fallback.

### flexiblePilots() Change
Instead of excluding "locked" pilots from displacement candidates, rank candidates by flexibility:
- Auto-assign pilots (no preference) = most flexible
- Preference pilots with large channel pools = moderately flexible
- Preference pilots on their preferred channel with tenure = least flexible (strongest protection)

---

## Database Changes

### Remove
- `channel_locked BOOLEAN` column (stop reading; leave in schema for SQLite compatibility)
- `locked_frequency_mhz INTEGER` column (stop reading; leave in schema)

### Add
- `preferred_frequency_mhz INTEGER DEFAULT 0` — pilot's preferred frequency (0 = auto-assign)

### Migration
- Copy `locked_frequency_mhz` → `preferred_frequency_mhz` for existing locked pilots
- No `leader_locked` column — leader placements are regular preferences

---

## API Changes

### Request Fields
- `channel_locked` / `locked_frequency_mhz` → `preferred_frequency_mhz`
- No `leader_locked` field — the leader's force action is handled at the endpoint level, not stored
- New field on force-placement: `force: true` (request-time flag, not stored)

### New Response Data
- Preview endpoints return richer data for the frontend to build choice dialogs:
  - `override_reason`: why the preference was overridden (conflict details)
  - `assigned_frequency_mhz`: where the pilot actually lands
  - `buddy_option`: buddy-up suggestion (if crowded)
  - `rebalance_option`: partial rebalance suggestion (if crowded, showing who would move)

### Rebalance Recommended
- `GET /api/sessions/{code}` response includes a `rebalance_recommended: true/false` flag based on detected conflicts or suboptimal spacing

---

## Design Principles

1. **Low friction** — this supplements in-person coordination, doesn't replace it
2. **Transparency** — always tell pilots what happened and why
3. **Pilot choice** — when there's a trade-off (buddy vs rebalance), the pilot decides
4. **First-come-first-served** — tenure matters, new arrivals don't bump existing pilots
5. **Preferences are weights, not walls** — strong signal, but overall session quality wins
6. **No permanent locks** — everything is rebalanceable
7. **GOT IT** — every "you've been moved" message requires confirmation

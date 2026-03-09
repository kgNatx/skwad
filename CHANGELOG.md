# Changelog

What's new in Skwad — the FPV frequency coordinator.

## [0.2.0] - 2026-03-08

### Smarter Channel Assignments

Previously, every time a new pilot joined, the system would reassign everyone's channels from scratch — even if most pilots were already on good channels. Now it works differently:

1. **Your channel stays put.** When someone new joins, Skwad tries to place them without moving anyone else.
2. **Minimal disruption.** If there's no clean slot, Skwad finds the smallest possible shuffle — moving just one pilot instead of everyone.
3. **Buddy system.** When the spectrum is truly full, Skwad suggests sharing a channel with the most compatible pilot instead of forcing a bad assignment.
4. **Leaving doesn't shuffle.** When a pilot leaves, everyone else stays where they are. The freed-up channel is available for the next person who joins.

### Session Leader

The first pilot to join a session becomes the **session leader** (shown with a LEADER badge). The leader has extra controls that other pilots don't see:

- **Add Pilot** — Add someone who doesn't have their phone or can't get the app working. Pick their callsign and video system (including bandwidth and FCC settings for DJI/Walksnail), and Skwad assigns them a channel.
- **Change Channel** — Tap any pilot to change their channel assignment. Useful for phantom pilots or coordinating manually.
- **Remove Pilot** — Remove a pilot who left without hitting Leave, or clean up a phantom entry.
- **Rebalance All** — Reassign every pilot's channel from scratch. Use this between heats or when assignments have drifted. Shows you exactly who moved and where.
- **Transfer Leadership** — Hand off the leader role to another pilot. Skwad prompts you to do this if you try to leave while you're still the leader.

Non-leaders can still change their own channel, change their callsign, and leave — they just can't affect other pilots.

## [0.1.0] - 2026-03-03

### Initial Release

The first version of Skwad — a frequency coordinator for FPV pilots flying together.

- **Start or join a session** using a 6-character code or QR scan. Sessions last 24 hours.
- **Setup wizard** walks you through your video system, FCC unlock status, goggles, bandwidth, and channel preference.
- **Automatic channel assignment** accounts for signal bandwidth, guard bands, and occupied spectrum across all major FPV video systems: Analog, HDZero, DJI V1/O3/O4, Walksnail, and OpenIPC.
- **Spectrum visualization** shows where everyone is on the band with bell-curve waveforms — green for you, red for danger-close overlap, yellow for tight spacing.
- **Buddy groups** — when pilots share a frequency, they're color-coded and labeled so they know to take turns.
- **Real-time updates** — when someone joins, leaves, or changes channels, everyone's view updates automatically.
- **QR code sharing** — tap the session code to show a scannable QR for others to join.
- **Works offline** — installable as a PWA with service worker caching.
- **Recent sessions** — your last sessions are saved so you can quickly rejoin.

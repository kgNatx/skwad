# Recent Sessions & Session Cleanup Design

## Problem

Users who close and reopen the Skwad app (especially as a PWA) must remember and re-enter a 6-character hex code to rejoin their session. Additionally, the backend has no cleanup of expired sessions and no collision protection on session ID generation — both issues that matter as usage scales.

## Design

### Recent Sessions (Frontend)

**Storage:** `skwad_recent_sessions` localStorage key containing a JSON array:

```json
[
  { "code": "A3F7B2", "pilotId": 12, "callsign": "KYLE" }
]
```

- Entries added on successful `POST /api/sessions/{code}/join` response
- Capped at 10 entries (FIFO eviction)
- Existing `skwad_session` / `skwad_pilot` keys unchanged

**Startup flow:**

1. App loads. If `skwad_recent_sessions` has entries, show the SKWAD icon + spinner (no buttons yet).
2. Validate all entries in parallel via `GET /api/sessions/{code}`.
3. Prune entries where the API returns non-200. Write pruned list back to localStorage.
4. Reveal the landing page with buttons and validated recent sessions.
5. If no stored sessions exist, skip validation and show landing page immediately (no spinner).

**UI:**

- Appears below the START SESSION / JOIN SESSION buttons, only if validated entries exist.
- Small "RECENT SESSIONS" label in gray, subtitle-sized text.
- One button per session, styled like secondary/dark buttons but slightly smaller than the main action buttons.
- Button text: `A3F7B2 — KYLE` (session code + callsign).
- Tap behavior: set `sessionCode` + `pilotId` in state, navigate to `#screen-session`, start polling. No wizard, no loading — goes straight in.
- If zero sessions survive validation, the entire section is hidden (no empty box, no label).

### Session ID Collision Retry (Backend)

- `CreateSession()` retries with a new code if the `INSERT` hits a primary key conflict.
- Retry up to 5 times before returning an error.
- With 16.7M possible codes and periodic cleanup, collisions are extremely rare.

### Expired Session Cleanup (Backend)

- Background goroutine in `main.go` runs `DeleteExpiredSessions()` on a periodic ticker (once per hour).
- `DeleteExpiredSessions()` already exists and is tested — just needs to be wired up.
- Pilot records cascade-delete with their sessions (foreign key ON DELETE CASCADE).

## Non-Goals

- Longer session codes or different alphabets (unnecessary with collision retry + cleanup)
- Showing expired sessions in the UI (pruned on load)
- New API endpoints (existing GET session endpoint returns 404 for expired sessions)

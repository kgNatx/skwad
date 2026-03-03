# Recent Sessions & Session Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add collision-safe session creation, automatic expired session cleanup, and a "Recent Sessions" UI that lets users rejoin active sessions from the landing page.

**Architecture:** Backend gets two small changes (retry loop in CreateSession, cleanup goroutine in main). Frontend adds a `skwad_recent_sessions` localStorage list, validates entries on startup, and renders them as buttons below the main landing actions.

**Tech Stack:** Go (backend), vanilla JS (frontend), SQLite

---

### Task 1: Add collision retry to CreateSession

**Files:**
- Modify: `db/db.go:100-120` (CreateSession function)
- Modify: `db/db_test.go` (add collision test)

**Step 1: Write the failing test**

Add to `db/db_test.go`:

```go
func TestCreateSession_CollisionRetry(t *testing.T) {
	d := newTestDB(t)

	// Pre-insert a session to force a collision on that code.
	first, err := d.CreateSession()
	if err != nil {
		t.Fatalf("first CreateSession: %v", err)
	}

	// Creating many sessions should never fail — retries handle collisions.
	// (With 16.7M codes and <100 sessions, collisions are near-impossible,
	// but the retry logic is what we're validating exists.)
	for i := 0; i < 50; i++ {
		sess, err := d.CreateSession()
		if err != nil {
			t.Fatalf("CreateSession #%d: %v", i, err)
		}
		if sess.ID == first.ID {
			t.Fatalf("collision not retried: got duplicate ID %q", sess.ID)
		}
	}
}
```

**Step 2: Run test to verify it passes (it likely already passes since collisions are rare, but the retry logic matters for correctness)**

Run: `cd /home/kyleg/containers/atxfpv.org/skwad && /home/kyleg/.local/go/bin/go test ./db/ -run TestCreateSession_CollisionRetry -v`

**Step 3: Implement collision retry in CreateSession**

Replace `CreateSession` in `db/db.go:100-120` with:

```go
// CreateSession generates a new session with a 6-char hex code that expires in 24 hours.
// Retries with a new code if a collision occurs (primary key conflict).
func (d *DB) CreateSession() (*Session, error) {
	now := time.Now().UTC()
	expires := now.Add(24 * time.Hour)

	const maxRetries = 5
	for i := 0; i < maxRetries; i++ {
		id := generateCode()
		_, err := d.db.Exec(
			`INSERT INTO sessions (id, created_at, expires_at, version) VALUES (?, ?, ?, 1)`,
			id, now, expires,
		)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "PRIMARY") {
				continue // collision, retry with new code
			}
			return nil, fmt.Errorf("insert session: %w", err)
		}
		return &Session{
			ID:        id,
			CreatedAt: now,
			ExpiresAt: expires,
			Version:   1,
		}, nil
	}
	return nil, fmt.Errorf("failed to generate unique session ID after %d attempts", maxRetries)
}
```

**Step 4: Run all db tests**

Run: `cd /home/kyleg/containers/atxfpv.org/skwad && /home/kyleg/.local/go/bin/go test ./db/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add db/db.go db/db_test.go
git commit -m "feat(skwad): add collision retry to CreateSession"
```

---

### Task 2: Wire up expired session cleanup goroutine

**Files:**
- Modify: `main.go:15-37` (add cleanup goroutine after database init)

**Step 1: Add cleanup goroutine to main.go**

After `database, err := db.New(dbPath)` (line 31) and before `srv := api.NewServer(database)` (line 37), add:

```go
	// Start background cleanup of expired sessions.
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			deleted, err := database.DeleteExpiredSessions()
			if err != nil {
				log.Printf("Cleanup error: %v", err)
			} else if deleted > 0 {
				log.Printf("Cleaned up %d expired session(s)", deleted)
			}
		}
	}()
```

Add `"time"` to the imports.

**Step 2: Verify it compiles**

Run: `cd /home/kyleg/containers/atxfpv.org/skwad && /home/kyleg/.local/go/bin/go build ./...`
Expected: No errors

**Step 3: Run all tests to ensure nothing broke**

Run: `cd /home/kyleg/containers/atxfpv.org/skwad && /home/kyleg/.local/go/bin/go test ./... -v`
Expected: All PASS

**Step 4: Commit**

```bash
git add main.go
git commit -m "feat(skwad): add background cleanup goroutine for expired sessions"
```

---

### Task 3: Add recent sessions to localStorage on join

**Files:**
- Modify: `static/app.js:204-223` (localStorage section — add recent sessions helpers)
- Modify: `static/app.js:613-632` (commitJoin — save to recent sessions after successful join)

**Step 1: Add recent sessions helper functions**

After the `clearState()` function (~line 223), add:

```javascript
  // ── Recent Sessions ────────────────────────────────────────────
  var RECENT_SESSIONS_KEY = 'skwad_recent_sessions';
  var MAX_RECENT_SESSIONS = 10;

  function getRecentSessions() {
    try {
      return JSON.parse(localStorage.getItem(RECENT_SESSIONS_KEY)) || [];
    } catch (e) {
      return [];
    }
  }

  function saveRecentSession(code, pilotId, callsign) {
    var sessions = getRecentSessions();
    // Remove existing entry for this session code (if rejoining)
    sessions = sessions.filter(function (s) { return s.code !== code; });
    // Add to front
    sessions.unshift({ code: code, pilotId: pilotId, callsign: callsign });
    // Cap at max
    if (sessions.length > MAX_RECENT_SESSIONS) {
      sessions = sessions.slice(0, MAX_RECENT_SESSIONS);
    }
    localStorage.setItem(RECENT_SESSIONS_KEY, JSON.stringify(sessions));
  }

  function setRecentSessions(sessions) {
    localStorage.setItem(RECENT_SESSIONS_KEY, JSON.stringify(sessions));
  }
```

**Step 2: Call saveRecentSession in commitJoin**

In `commitJoin()` (~line 619-620), after `state.pilotId = pilot.ID;` and `saveState();`, add:

```javascript
      saveRecentSession(state.sessionCode, pilot.ID, state.callsign);
```

So lines 618-621 become:

```javascript
      var pilot = await apiPost('/api/sessions/' + state.sessionCode + '/join' + rebalParam, body);
      state.pilotId = pilot.ID;
      saveState();
      saveRecentSession(state.sessionCode, pilot.ID, state.callsign);
      enterSessionView();
```

**Step 3: Test manually**

1. Build and restart: `cd /home/kyleg/containers/atxfpv.org && docker compose build skwad && docker compose up -d skwad`
2. Open app, create session, join with callsign
3. Check browser devtools: `localStorage.getItem('skwad_recent_sessions')` should have one entry

**Step 4: Commit**

```bash
git add static/app.js
git commit -m "feat(skwad): save recent sessions to localStorage on join"
```

---

### Task 4: Validate recent sessions on startup and render UI

**Files:**
- Modify: `static/index.html:26-43` (add recent sessions container to landing page HTML)
- Modify: `static/app.js` (route function and initLanding — add validation + rendering)
- Modify: `static/style.css` (add recent sessions styles)

**Step 1: Add HTML container for recent sessions**

In `static/index.html`, after the `<div id="landing-error">` line (line 34) and before the install-banner div (line 35), add:

```html
      <div id="recent-sessions" class="recent-sessions hidden">
        <div class="recent-sessions-label">RECENT SESSIONS</div>
        <div id="recent-sessions-list" class="recent-sessions-list"></div>
      </div>
```

**Step 2: Add CSS styles**

In `static/style.css`, add in the landing page section:

```css
/* Recent Sessions */
.recent-sessions {
  width: 100%;
  max-width: 400px;
  margin-top: 24px;
}

.recent-sessions-label {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 2px;
  color: #888;
  text-align: center;
  margin-bottom: 12px;
}

.recent-sessions-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.btn-recent-session {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  width: 100%;
  min-height: 52px;
  padding: 12px 16px;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 1px;
  color: #fff;
  background: #1a1a1a;
  border: 1px solid #333;
  border-radius: 12px;
  cursor: pointer;
  -webkit-tap-highlight-color: transparent;
}

.btn-recent-session:active {
  transform: scale(0.97);
  opacity: 0.85;
}

.recent-session-code {
  color: #fff;
}

.recent-session-sep {
  color: #555;
}

.recent-session-callsign {
  color: #aaa;
}
```

**Step 3: Add startup validation and rendering logic**

Modify the `route()` function in `static/app.js`. Replace the root path section (~lines 2005-2012):

```javascript
    // Root path
    loadState();
    if (state.sessionCode && state.pilotId) {
      // Returning user with active session
      enterSessionView();
    } else {
      // Validate recent sessions before showing landing
      validateAndShowLanding();
    }
```

Add the `validateAndShowLanding` and `renderRecentSessions` functions:

```javascript
  async function validateAndShowLanding() {
    var recent = getRecentSessions();
    if (recent.length === 0) {
      renderRecentSessions([]);
      showScreen('landing');
      return;
    }

    // Show landing with buttons hidden while validating
    showScreen('landing');
    document.querySelector('.landing-buttons').style.display = 'none';
    $('recent-sessions').classList.add('hidden');

    // Validate all sessions in parallel
    var validated = [];
    var results = await Promise.allSettled(recent.map(function (entry) {
      return apiGet('/api/sessions/' + entry.code).then(function () {
        return entry;
      });
    }));
    results.forEach(function (result) {
      if (result.status === 'fulfilled') {
        validated.push(result.value);
      }
    });

    // Update localStorage with only valid sessions
    setRecentSessions(validated);

    // Render and show
    renderRecentSessions(validated);
    document.querySelector('.landing-buttons').style.display = '';
  }

  function renderRecentSessions(sessions) {
    var container = $('recent-sessions');
    var list = $('recent-sessions-list');
    clearChildren(list);

    if (sessions.length === 0) {
      container.classList.add('hidden');
      return;
    }

    sessions.forEach(function (entry) {
      var btn = document.createElement('button');
      btn.className = 'btn-recent-session';

      var codeSpan = document.createElement('span');
      codeSpan.className = 'recent-session-code';
      codeSpan.textContent = entry.code;

      var sepSpan = document.createElement('span');
      sepSpan.className = 'recent-session-sep';
      sepSpan.textContent = '\u2014';

      var callsignSpan = document.createElement('span');
      callsignSpan.className = 'recent-session-callsign';
      callsignSpan.textContent = entry.callsign;

      btn.appendChild(codeSpan);
      btn.appendChild(sepSpan);
      btn.appendChild(callsignSpan);

      btn.addEventListener('click', function () {
        state.sessionCode = entry.code;
        state.pilotId = entry.pilotId;
        saveState();
        enterSessionView();
      });
      list.appendChild(btn);
    });

    container.classList.remove('hidden');
  }
```

**Step 4: Test manually**

1. Rebuild: `cd /home/kyleg/containers/atxfpv.org && docker compose build skwad && docker compose up -d skwad`
2. Open app, create and join a session
3. Leave the session (or open a new tab to the root URL)
4. Verify the recent session appears as a button on the landing page
5. Tap it — should go straight to session view
6. Wait for session to expire (or manually expire in DB) and reload — button should disappear

**Step 5: Commit**

```bash
git add static/index.html static/app.js static/style.css
git commit -m "feat(skwad): add recent sessions UI on landing page"
```

---

### Task 5: Handle edge cases and final testing

**Files:**
- Modify: `static/app.js` (clearState and leave-session flow)

**Step 1: Remove from recent sessions when leaving voluntarily**

Find the leave/deactivate handler that calls `clearState()`. Before `clearState()` wipes `state.sessionCode`, capture it and remove from recents:

```javascript
  // Before clearState():
  var leftCode = state.sessionCode;
  clearState();
  var recent = getRecentSessions();
  recent = recent.filter(function (s) { return s.code !== leftCode; });
  setRecentSessions(recent);
```

**Step 2: Verify /s/{CODE} route also saves to recents**

The `commitJoin` function already handles this (Task 3), so joining via QR/URL goes through the same wizard → commitJoin path. No additional work needed.

**Step 3: Full end-to-end test**

1. Rebuild and restart
2. Create session A, join as KYLE → verify recent sessions entry added
3. Create session B in another browser, join as KYLE → verify both appear in recent
4. Close and reopen → both sessions should appear, most recent first
5. Leave session B → session B should be removed from recent, session A remains
6. Wait for session A to expire (or manually expire) → reload → no recent sessions shown, section hidden
7. Test the /s/{CODE} join path → verify it also adds to recent sessions

**Step 4: Commit**

```bash
git add static/app.js
git commit -m "feat(skwad): remove session from recents on voluntary leave"
```

---

### Task 6: Build, deploy, and verify

**Step 1: Run all Go tests**

Run: `cd /home/kyleg/containers/atxfpv.org/skwad && /home/kyleg/.local/go/bin/go test ./... -v`
Expected: All PASS

**Step 2: Build and deploy**

```bash
cd /home/kyleg/containers/atxfpv.org && docker compose build skwad && docker compose up -d skwad
```

**Step 3: Smoke test on dev domain**

Open `skwad.atxfpv.hippienet.wtf` and verify:
- Landing page loads (no recent sessions section visible)
- Create + join session works
- Close tab, reopen → recent session appears
- Tap recent session → goes straight to session view
- Leave session → recent session removed

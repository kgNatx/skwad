# Skwad Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a mobile-first web app at `skwad.atxfpv.org` that manages FPV video frequencies at drone meetups to prevent interference.

**Architecture:** Single Go binary serving REST API + static HTML/CSS/JS frontend, SQLite for storage, Docker container behind Traefik. No build step, no frameworks, no WebSockets — just polling.

**Tech Stack:** Go 1.22+, `net/http` (stdlib), `modernc.org/sqlite` (pure-Go SQLite), vanilla HTML/CSS/JS, Docker, Traefik

**Design doc:** `docs/plans/2026-03-03-skwad-frequency-coordinator-design.md`

---

## Task 1: Go Project Scaffolding

**Files:**
- Create: `skwad/go.mod`
- Create: `skwad/main.go`
- Create: `skwad/Dockerfile`

**Step 1: Initialize Go module**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go mod init github.com/kyleg/skwad
```

**Step 2: Create minimal main.go**

Create `skwad/main.go`:
```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	log.Printf("Skwad listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
```

**Step 3: Create Dockerfile**

Create `skwad/Dockerfile`:
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o skwad .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/skwad .
COPY static/ ./static/
EXPOSE 8080
CMD ["./skwad"]
```

**Step 4: Verify it compiles**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go build -o /dev/null .
```

Expected: no errors.

**Step 5: Commit**

```bash
git add skwad/go.mod skwad/main.go skwad/Dockerfile
git commit -m "feat(skwad): scaffold Go project with health endpoint"
```

---

## Task 2: SQLite Database Layer

**Files:**
- Create: `skwad/db/db.go`
- Create: `skwad/db/db_test.go`

**Step 1: Add SQLite dependency**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go get modernc.org/sqlite
```

**Step 2: Write failing test for DB initialization**

Create `skwad/db/db_test.go`:
```go
package db

import (
	"os"
	"testing"
)

func TestNewDB_CreatesTables(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	database, err := New(tmpFile)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer database.Close()

	// Verify sessions table exists
	var name string
	err = database.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&name)
	if err != nil {
		t.Fatal("sessions table not created")
	}

	// Verify pilots table exists
	err = database.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='pilots'").Scan(&name)
	if err != nil {
		t.Fatal("pilots table not created")
	}
}

func TestCreateSession(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	session, err := database.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}
	if len(session.ID) != 6 {
		t.Errorf("session ID length = %d, want 6", len(session.ID))
	}
	if session.ExpiresAt.IsZero() {
		t.Error("session ExpiresAt is zero")
	}
}

func TestGetSession(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	created, _ := database.CreateSession()
	got, err := database.GetSession(created.ID)
	if err != nil {
		t.Fatalf("GetSession() error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("got ID %s, want %s", got.ID, created.ID)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	_, err := database.GetSession("XXXXXX")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func newTestDB(t *testing.T) *DB {
	t.Helper()
	tmpFile := t.TempDir() + "/test.db"
	database, err := New(tmpFile)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return database
}

// Ensure we don't leak temp files
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
```

**Step 3: Run tests to verify they fail**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go test ./db/ -v
```

Expected: FAIL — package `db` doesn't exist yet.

**Step 4: Implement database layer**

Create `skwad/db/db.go`:
```go
package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Version   int       `json:"version"`
}

type Pilot struct {
	ID                 int    `json:"id"`
	SessionID          string `json:"session_id"`
	Callsign           string `json:"callsign"`
	VideoSystem        string `json:"video_system"`
	FCCUnlocked        bool   `json:"fcc_unlocked"`
	Goggles            string `json:"goggles,omitempty"`
	BandwidthMHz       int    `json:"bandwidth_mhz,omitempty"`
	RaceMode           bool   `json:"race_mode"`
	ChannelLocked      bool   `json:"channel_locked"`
	LockedFrequencyMHz int    `json:"locked_frequency_mhz,omitempty"`
	AssignedChannel    string `json:"assigned_channel,omitempty"`
	AssignedFreqMHz    int    `json:"assigned_frequency_mhz,omitempty"`
	BuddyGroup         int    `json:"buddy_group,omitempty"`
	Active             bool   `json:"active"`
}

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	created_at DATETIME NOT NULL,
	expires_at DATETIME NOT NULL,
	version INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS pilots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	callsign TEXT NOT NULL,
	video_system TEXT NOT NULL,
	fcc_unlocked BOOLEAN DEFAULT FALSE,
	goggles TEXT DEFAULT '',
	bandwidth_mhz INTEGER DEFAULT 0,
	race_mode BOOLEAN DEFAULT FALSE,
	channel_locked BOOLEAN DEFAULT FALSE,
	locked_frequency_mhz INTEGER DEFAULT 0,
	assigned_channel TEXT DEFAULT '',
	assigned_frequency_mhz INTEGER DEFAULT 0,
	buddy_group INTEGER DEFAULT 0,
	joined_at DATETIME NOT NULL,
	active BOOLEAN DEFAULT TRUE,
	UNIQUE(session_id, callsign)
);
`

func New(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := sqlDB.Exec(schema); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &DB{db: sqlDB}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) CreateSession() (*Session, error) {
	id := generateCode()
	now := time.Now().UTC()
	expires := now.Add(24 * time.Hour)
	_, err := d.db.Exec(
		"INSERT INTO sessions (id, created_at, expires_at, version) VALUES (?, ?, ?, 1)",
		id, now, expires,
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return &Session{ID: id, CreatedAt: now, ExpiresAt: expires, Version: 1}, nil
}

func (d *DB) GetSession(id string) (*Session, error) {
	s := &Session{}
	err := d.db.QueryRow(
		"SELECT id, created_at, expires_at, version FROM sessions WHERE id = ? AND expires_at > ?",
		id, time.Now().UTC(),
	).Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt, &s.Version)
	if err != nil {
		return nil, fmt.Errorf("get session %s: %w", id, err)
	}
	return s, nil
}

func (d *DB) IncrementVersion(sessionID string) error {
	_, err := d.db.Exec("UPDATE sessions SET version = version + 1 WHERE id = ?", sessionID)
	return err
}

func (d *DB) AddPilot(sessionID string, p *Pilot) (*Pilot, error) {
	now := time.Now().UTC()
	result, err := d.db.Exec(
		`INSERT INTO pilots (session_id, callsign, video_system, fcc_unlocked, goggles, bandwidth_mhz,
		 race_mode, channel_locked, locked_frequency_mhz, joined_at, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE)`,
		sessionID, p.Callsign, p.VideoSystem, p.FCCUnlocked, p.Goggles, p.BandwidthMHz,
		p.RaceMode, p.ChannelLocked, p.LockedFrequencyMHz, now,
	)
	if err != nil {
		return nil, fmt.Errorf("add pilot: %w", err)
	}
	id, _ := result.LastInsertId()
	p.ID = int(id)
	p.SessionID = sessionID
	p.Active = true
	return p, nil
}

func (d *DB) GetActivePilots(sessionID string) ([]Pilot, error) {
	rows, err := d.db.Query(
		`SELECT id, session_id, callsign, video_system, fcc_unlocked, goggles, bandwidth_mhz,
		 race_mode, channel_locked, locked_frequency_mhz, assigned_channel, assigned_frequency_mhz,
		 buddy_group, active
		 FROM pilots WHERE session_id = ? AND active = TRUE ORDER BY id`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("get pilots: %w", err)
	}
	defer rows.Close()

	var pilots []Pilot
	for rows.Next() {
		var p Pilot
		if err := rows.Scan(&p.ID, &p.SessionID, &p.Callsign, &p.VideoSystem, &p.FCCUnlocked,
			&p.Goggles, &p.BandwidthMHz, &p.RaceMode, &p.ChannelLocked, &p.LockedFrequencyMHz,
			&p.AssignedChannel, &p.AssignedFreqMHz, &p.BuddyGroup, &p.Active); err != nil {
			return nil, fmt.Errorf("scan pilot: %w", err)
		}
		pilots = append(pilots, p)
	}
	return pilots, rows.Err()
}

func (d *DB) UpdatePilotAssignment(pilotID int, channel string, freqMHz int, buddyGroup int) error {
	_, err := d.db.Exec(
		"UPDATE pilots SET assigned_channel = ?, assigned_frequency_mhz = ?, buddy_group = ? WHERE id = ?",
		channel, freqMHz, buddyGroup, pilotID,
	)
	return err
}

func (d *DB) DeactivatePilot(pilotID int) error {
	_, err := d.db.Exec("UPDATE pilots SET active = FALSE WHERE id = ?", pilotID)
	return err
}

func (d *DB) DeleteExpiredSessions() (int64, error) {
	result, err := d.db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func generateCode() string {
	b := make([]byte, 3)
	rand.Read(b)
	code := strings.ToUpper(hex.EncodeToString(b))
	return code[:6]
}
```

**Step 5: Run tests**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go test ./db/ -v
```

Expected: all PASS.

**Step 6: Add pilot CRUD tests**

Add to `skwad/db/db_test.go`:
```go
func TestAddPilot(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	session, _ := database.CreateSession()
	pilot := &Pilot{
		Callsign:    "RazorFPV",
		VideoSystem: "dji_o3",
		FCCUnlocked: true,
		BandwidthMHz: 20,
	}
	got, err := database.AddPilot(session.ID, pilot)
	if err != nil {
		t.Fatalf("AddPilot() error: %v", err)
	}
	if got.ID == 0 {
		t.Error("pilot ID should be non-zero")
	}
	if got.Callsign != "RazorFPV" {
		t.Errorf("callsign = %s, want RazorFPV", got.Callsign)
	}
}

func TestAddPilot_DuplicateCallsign(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	session, _ := database.CreateSession()
	pilot := &Pilot{Callsign: "RazorFPV", VideoSystem: "analog"}
	database.AddPilot(session.ID, pilot)

	_, err := database.AddPilot(session.ID, pilot)
	if err == nil {
		t.Fatal("expected error for duplicate callsign in same session")
	}
}

func TestGetActivePilots(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	session, _ := database.CreateSession()
	database.AddPilot(session.ID, &Pilot{Callsign: "Pilot1", VideoSystem: "analog"})
	database.AddPilot(session.ID, &Pilot{Callsign: "Pilot2", VideoSystem: "hdzero"})

	pilots, err := database.GetActivePilots(session.ID)
	if err != nil {
		t.Fatalf("GetActivePilots() error: %v", err)
	}
	if len(pilots) != 2 {
		t.Errorf("got %d pilots, want 2", len(pilots))
	}
}

func TestDeactivatePilot(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	session, _ := database.CreateSession()
	pilot, _ := database.AddPilot(session.ID, &Pilot{Callsign: "Leaving", VideoSystem: "analog"})
	database.DeactivatePilot(pilot.ID)

	pilots, _ := database.GetActivePilots(session.ID)
	if len(pilots) != 0 {
		t.Errorf("got %d active pilots, want 0", len(pilots))
	}
}
```

**Step 7: Run all DB tests**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go test ./db/ -v
```

Expected: all PASS.

**Step 8: Commit**

```bash
git add skwad/db/
git commit -m "feat(skwad): add SQLite database layer with session and pilot CRUD"
```

---

## Task 3: Frequency Tables Data

**Files:**
- Create: `skwad/freq/tables.go`
- Create: `skwad/freq/tables_test.go`

**Step 1: Write failing test for channel pool lookup**

Create `skwad/freq/tables_test.go`:
```go
package freq

import (
	"testing"
)

func TestChannelPool_Analog(t *testing.T) {
	pool := ChannelPool("analog", false, 0, false, "")
	if len(pool) != 8 {
		t.Errorf("analog pool size = %d, want 8", len(pool))
	}
	// R1 should be first
	if pool[0].Name != "R1" || pool[0].FreqMHz != 5658 {
		t.Errorf("first channel = %+v, want R1/5658", pool[0])
	}
}

func TestChannelPool_HDZero(t *testing.T) {
	pool := ChannelPool("hdzero", false, 0, false, "")
	if len(pool) != 8 {
		t.Errorf("hdzero pool size = %d, want 8", len(pool))
	}
}

func TestChannelPool_DJI_V1_Stock(t *testing.T) {
	pool := ChannelPool("dji_v1", false, 0, false, "")
	if len(pool) != 4 {
		t.Errorf("dji_v1 stock pool size = %d, want 4", len(pool))
	}
}

func TestChannelPool_DJI_V1_FCC(t *testing.T) {
	pool := ChannelPool("dji_v1", true, 0, false, "")
	if len(pool) != 8 {
		t.Errorf("dji_v1 FCC pool size = %d, want 8", len(pool))
	}
}

func TestChannelPool_DJI_O3_Stock_20MHz(t *testing.T) {
	pool := ChannelPool("dji_o3", false, 20, false, "")
	if len(pool) != 3 {
		t.Errorf("dji_o3 stock 20MHz pool size = %d, want 3", len(pool))
	}
}

func TestChannelPool_DJI_O3_FCC_20MHz(t *testing.T) {
	pool := ChannelPool("dji_o3", true, 20, false, "")
	if len(pool) != 7 {
		t.Errorf("dji_o3 FCC 20MHz pool size = %d, want 7", len(pool))
	}
}

func TestChannelPool_DJI_O4_RaceMode(t *testing.T) {
	pool := ChannelPool("dji_o4", true, 20, true, "goggles_3")
	if len(pool) != 8 {
		t.Errorf("dji_o4 race mode pool size = %d, want 8", len(pool))
	}
	// Race mode should give Race Band channels
	if pool[0].Name != "R1" || pool[0].FreqMHz != 5658 {
		t.Errorf("first race mode channel = %+v, want R1/5658", pool[0])
	}
}

func TestChannelPool_Walksnail_RaceMode(t *testing.T) {
	pool := ChannelPool("walksnail_race", false, 0, false, "")
	if len(pool) != 8 {
		t.Errorf("walksnail_race pool size = %d, want 8", len(pool))
	}
}

func TestChannelPool_Walksnail_Std_Stock(t *testing.T) {
	pool := ChannelPool("walksnail_std", false, 0, false, "")
	if len(pool) != 4 {
		t.Errorf("walksnail_std stock pool size = %d, want 4", len(pool))
	}
}

func TestChannelPool_Walksnail_Std_FCC(t *testing.T) {
	pool := ChannelPool("walksnail_std", true, 0, false, "")
	if len(pool) != 8 {
		t.Errorf("walksnail_std FCC pool size = %d, want 8", len(pool))
	}
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go test ./freq/ -v
```

Expected: FAIL — package doesn't exist.

**Step 3: Implement frequency tables**

Create `skwad/freq/tables.go`:
```go
package freq

// Channel represents a single frequency channel a pilot can use.
type Channel struct {
	Name    string `json:"name"`     // Human-readable: "R1", "DJI-CH2"
	FreqMHz int    `json:"freq_mhz"` // Center frequency in MHz
}

// Race Band — the universal standard for FPV racing
var RaceBand = []Channel{
	{"R1", 5658}, {"R2", 5695}, {"R3", 5732}, {"R4", 5769},
	{"R5", 5806}, {"R6", 5843}, {"R7", 5880}, {"R8", 5917},
}

// DJI V1/Vista FCC (8 channels)
var DJIV1FCC = []Channel{
	{"DJI-CH1", 5660}, {"DJI-CH2", 5695}, {"DJI-CH3", 5735}, {"DJI-CH4", 5770},
	{"DJI-CH5", 5805}, {"DJI-CH6", 5878}, {"DJI-CH7", 5914}, {"DJI-CH8", 5839},
}

// DJI V1/Vista Stock/CE (4 channels)
var DJIV1Stock = []Channel{
	{"DJI-CH3", 5735}, {"DJI-CH4", 5770}, {"DJI-CH5", 5805}, {"DJI-CH8", 5839},
}

// DJI O3 Stock — 3 channels at 20/10MHz
var DJIO3Stock = []Channel{
	{"O3-CH1", 5769}, {"O3-CH2", 5805}, {"O3-CH3", 5840},
}

// DJI O3 FCC — 7 channels at 20/10MHz
// Channels sourced from community documentation of FCC-unlocked O3 units
var DJIO3FCC = []Channel{
	{"O3-CH1", 5669}, {"O3-CH2", 5705}, {"O3-CH3", 5741},
	{"O3-CH4", 5769}, {"O3-CH5", 5805}, {"O3-CH6", 5840}, {"O3-CH7", 5876},
}

// DJI O3 40MHz — only 1 channel regardless of FCC
var DJIO3_40 = []Channel{
	{"O3-CH1", 5795},
}

// DJI O4 Stock — 3 channels at 20/10MHz
var DJIO4Stock = []Channel{
	{"O4-CH1", 5769}, {"O4-CH2", 5790}, {"O4-CH3", 5815},
}

// DJI O4 FCC — 7 channels at 20/10MHz
var DJIO4FCC = []Channel{
	{"O4-CH1", 5669}, {"O4-CH2", 5705}, {"O4-CH3", 5741},
	{"O4-CH4", 5769}, {"O4-CH5", 5790}, {"O4-CH6", 5815}, {"O4-CH7", 5876},
}

// DJI O4 40MHz FCC — 3 channels
var DJIO4_40_FCC = []Channel{
	{"O4-CH1", 5735}, {"O4-CH2", 5795}, {"O4-CH3", 5855},
}

// DJI O4 40MHz Stock — 1 channel
var DJIO4_40_Stock = []Channel{
	{"O4-CH1", 5795},
}

// DJI O4 60MHz — 1 channel
var DJIO4_60 = []Channel{
	{"O4-CH1", 5795},
}

// Walksnail Standard FCC (same as DJI V1)
var WalksnailStdFCC = DJIV1FCC

// Walksnail Standard Stock/CE (4 channels)
var WalksnailStdStock = DJIV1Stock

// Walksnail Race Mode uses Race Band
var WalksnailRace = RaceBand

// ChannelPool returns the available channels for a given pilot configuration.
func ChannelPool(videoSystem string, fccUnlocked bool, bandwidthMHz int, raceMode bool, goggles string) []Channel {
	switch videoSystem {
	case "analog", "hdzero":
		return RaceBand

	case "dji_v1":
		if fccUnlocked {
			return DJIV1FCC
		}
		return DJIV1Stock

	case "dji_o3":
		if bandwidthMHz >= 40 {
			return DJIO3_40
		}
		if fccUnlocked {
			return DJIO3FCC
		}
		return DJIO3Stock

	case "dji_o4":
		// Race Mode: full Race Band (requires Goggles 3 or N3)
		if raceMode && (goggles == "goggles_3" || goggles == "goggles_n3") {
			return RaceBand
		}
		if bandwidthMHz >= 60 {
			return DJIO4_60
		}
		if bandwidthMHz >= 40 {
			if fccUnlocked {
				return DJIO4_40_FCC
			}
			return DJIO4_40_Stock
		}
		if fccUnlocked {
			return DJIO4FCC
		}
		return DJIO4Stock

	case "walksnail_std":
		if fccUnlocked {
			return WalksnailStdFCC
		}
		return WalksnailStdStock

	case "walksnail_race":
		return WalksnailRace

	case "openipc":
		// OpenIPC can be on any WiFi channel; default to 5825
		return []Channel{{"WiFi-165", 5825}}

	default:
		return RaceBand
	}
}

// MinSafeSpacingMHz is the minimum frequency separation to avoid interference.
const MinSafeSpacingMHz = 37
```

**Step 4: Run tests**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go test ./freq/ -v
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add skwad/freq/
git commit -m "feat(skwad): add frequency tables for all FPV video systems"
```

---

## Task 4: Frequency Optimizer

**Files:**
- Create: `skwad/freq/optimizer.go`
- Create: `skwad/freq/optimizer_test.go`

**Step 1: Write failing test — basic assignment**

Create `skwad/freq/optimizer_test.go`:
```go
package freq

import (
	"testing"
)

// PilotConfig matches what the optimizer needs from a pilot
type testPilot struct {
	ID            int
	VideoSystem   string
	FCCUnlocked   bool
	BandwidthMHz  int
	RaceMode      bool
	Goggles       string
	ChannelLocked bool
	LockedFreqMHz int
	// Previous assignment (for stability)
	PrevChannel string
	PrevFreqMHz int
}

func TestOptimize_SingleAnalog(t *testing.T) {
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "analog"},
	}
	result := Optimize(pilots)
	if len(result) != 1 {
		t.Fatalf("got %d assignments, want 1", len(result))
	}
	if result[0].FreqMHz == 0 {
		t.Error("frequency should be assigned")
	}
	if result[0].BuddyGroup != 0 {
		t.Error("single pilot should have no buddy group")
	}
}

func TestOptimize_TwoAnalog_MaxSeparation(t *testing.T) {
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "analog"},
		{ID: 2, VideoSystem: "analog"},
	}
	result := Optimize(pilots)
	if len(result) != 2 {
		t.Fatalf("got %d assignments, want 2", len(result))
	}
	sep := abs(result[0].FreqMHz - result[1].FreqMHz)
	// Two analog pilots on Race Band: best separation is R1(5658) and R8(5917) = 259MHz
	if sep < 200 {
		t.Errorf("separation = %d MHz, want >= 200", sep)
	}
}

func TestOptimize_FourAnalog_IMDFree(t *testing.T) {
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "analog"},
		{ID: 2, VideoSystem: "analog"},
		{ID: 3, VideoSystem: "analog"},
		{ID: 4, VideoSystem: "analog"},
	}
	result := Optimize(pilots)
	if len(result) != 4 {
		t.Fatalf("got %d assignments, want 4", len(result))
	}
	// All should have unique frequencies
	freqs := map[int]bool{}
	for _, a := range result {
		if freqs[a.FreqMHz] {
			t.Errorf("duplicate frequency %d", a.FreqMHz)
		}
		freqs[a.FreqMHz] = true
	}
	// Minimum separation should be >= 37MHz
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			sep := abs(result[i].FreqMHz - result[j].FreqMHz)
			if sep < MinSafeSpacingMHz {
				t.Errorf("pilots %d and %d only %d MHz apart", result[i].PilotID, result[j].PilotID, sep)
			}
		}
	}
}

func TestOptimize_LockedChannel(t *testing.T) {
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "analog", ChannelLocked: true, LockedFreqMHz: 5732}, // R3
		{ID: 2, VideoSystem: "analog"},
	}
	result := Optimize(pilots)
	// Pilot 1 must be on 5732
	for _, a := range result {
		if a.PilotID == 1 && a.FreqMHz != 5732 {
			t.Errorf("locked pilot on %d, want 5732", a.FreqMHz)
		}
	}
	// Pilot 2 should be far from 5732
	for _, a := range result {
		if a.PilotID == 2 {
			sep := abs(a.FreqMHz - 5732)
			if sep < MinSafeSpacingMHz {
				t.Errorf("pilot 2 only %d MHz from locked pilot", sep)
			}
		}
	}
}

func TestOptimize_BuddyGroup_TooManyDJI(t *testing.T) {
	// 4 DJI O3 stock pilots — only 3 channels available
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "dji_o3", BandwidthMHz: 20},
		{ID: 2, VideoSystem: "dji_o3", BandwidthMHz: 20},
		{ID: 3, VideoSystem: "dji_o3", BandwidthMHz: 20},
		{ID: 4, VideoSystem: "dji_o3", BandwidthMHz: 20},
	}
	result := Optimize(pilots)
	if len(result) != 4 {
		t.Fatalf("got %d assignments, want 4", len(result))
	}
	// At least one pilot should be in a buddy group
	hasBuddy := false
	for _, a := range result {
		if a.BuddyGroup > 0 {
			hasBuddy = true
			break
		}
	}
	if !hasBuddy {
		t.Error("expected at least one buddy group with 4 pilots on 3 channels")
	}
}

func TestOptimize_MixedSystems(t *testing.T) {
	pilots := []PilotInput{
		{ID: 1, VideoSystem: "analog"},
		{ID: 2, VideoSystem: "hdzero"},
		{ID: 3, VideoSystem: "dji_o3", FCCUnlocked: true, BandwidthMHz: 20},
	}
	result := Optimize(pilots)
	if len(result) != 3 {
		t.Fatalf("got %d assignments, want 3", len(result))
	}
	// All should be assigned
	for _, a := range result {
		if a.FreqMHz == 0 {
			t.Errorf("pilot %d has no frequency", a.PilotID)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go test ./freq/ -v -run TestOptimize
```

Expected: FAIL — `PilotInput` and `Optimize` don't exist.

**Step 3: Implement the optimizer**

Create `skwad/freq/optimizer.go`:
```go
package freq

import "sort"

// PilotInput is what the optimizer needs from each pilot.
type PilotInput struct {
	ID            int
	VideoSystem   string
	FCCUnlocked   bool
	BandwidthMHz  int
	RaceMode      bool
	Goggles       string
	ChannelLocked bool
	LockedFreqMHz int
	// Previous assignment for stability
	PrevChannel string
	PrevFreqMHz int
}

// Assignment is the optimizer's output for one pilot.
type Assignment struct {
	PilotID    int    `json:"pilot_id"`
	Channel    string `json:"channel"`
	FreqMHz    int    `json:"freq_mhz"`
	BuddyGroup int   `json:"buddy_group"`
}

// Optimize assigns channels to pilots, maximizing minimum frequency separation.
// Fixed-channel pilots are honored as immovable constraints.
// When channels are exhausted, buddy groups are created.
func Optimize(pilots []PilotInput) []Assignment {
	if len(pilots) == 0 {
		return nil
	}

	// Build channel pools
	type pilotWithPool struct {
		input PilotInput
		pool  []Channel
	}
	var locked []pilotWithPool
	var flexible []pilotWithPool

	for _, p := range pilots {
		pool := ChannelPool(p.VideoSystem, p.FCCUnlocked, p.BandwidthMHz, p.RaceMode, p.Goggles)
		pp := pilotWithPool{input: p, pool: pool}
		if p.ChannelLocked && p.LockedFreqMHz > 0 {
			locked = append(locked, pp)
		} else {
			flexible = append(flexible, pp)
		}
	}

	// Sort flexible pilots by pool size ascending (most constrained first)
	sort.Slice(flexible, func(i, j int) bool {
		return len(flexible[i].pool) < len(flexible[j].pool)
	})

	assignments := make(map[int]Assignment)
	usedFreqs := make(map[int][]int) // freq -> list of pilot IDs on that freq

	// Step 1: Lock in fixed-channel pilots
	for _, lp := range locked {
		freq := lp.input.LockedFreqMHz
		ch := findChannelName(lp.pool, freq)
		if ch == "" {
			ch = "LOCKED"
		}
		assignments[lp.input.ID] = Assignment{
			PilotID: lp.input.ID,
			Channel: ch,
			FreqMHz: freq,
		}
		usedFreqs[freq] = append(usedFreqs[freq], lp.input.ID)
	}

	// Step 2: Assign flexible pilots one at a time
	for _, fp := range flexible {
		bestChannel := Channel{}
		bestMinSep := -1

		for _, ch := range fp.pool {
			minSep := minSeparation(ch.FreqMHz, usedFreqs)
			// Prefer previous assignment for stability
			if fp.input.PrevFreqMHz == ch.FreqMHz && minSep >= MinSafeSpacingMHz {
				bestChannel = ch
				bestMinSep = minSep
				break
			}
			if minSep > bestMinSep {
				bestMinSep = minSep
				bestChannel = ch
			}
		}

		assignments[fp.input.ID] = Assignment{
			PilotID: fp.input.ID,
			Channel: bestChannel.Name,
			FreqMHz: bestChannel.FreqMHz,
		}
		usedFreqs[bestChannel.FreqMHz] = append(usedFreqs[bestChannel.FreqMHz], fp.input.ID)
	}

	// Step 3: Identify buddy groups (frequencies shared by multiple pilots)
	buddyGroupID := 1
	freqToBuddy := make(map[int]int)
	for freq, pilotIDs := range usedFreqs {
		if len(pilotIDs) > 1 {
			freqToBuddy[freq] = buddyGroupID
			buddyGroupID++
		}
	}

	// Apply buddy groups
	result := make([]Assignment, 0, len(pilots))
	for _, p := range pilots {
		a := assignments[p.ID]
		if bg, ok := freqToBuddy[a.FreqMHz]; ok {
			a.BuddyGroup = bg
		}
		result = append(result, a)
	}

	return result
}

// minSeparation returns the minimum MHz distance from freq to any used frequency.
// Returns MaxInt if no frequencies are in use.
func minSeparation(freq int, usedFreqs map[int][]int) int {
	if len(usedFreqs) == 0 {
		return 999999
	}
	minSep := 999999
	for usedFreq := range usedFreqs {
		sep := freq - usedFreq
		if sep < 0 {
			sep = -sep
		}
		if sep < minSep {
			minSep = sep
		}
	}
	return minSep
}

func findChannelName(pool []Channel, freqMHz int) string {
	for _, ch := range pool {
		if ch.FreqMHz == freqMHz {
			return ch.Name
		}
	}
	return ""
}
```

**Step 4: Run tests**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go test ./freq/ -v
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add skwad/freq/optimizer.go skwad/freq/optimizer_test.go
git commit -m "feat(skwad): add frequency optimizer with buddy group support"
```

---

## Task 5: HTTP API Handlers

**Files:**
- Create: `skwad/api/handlers.go`
- Create: `skwad/api/handlers_test.go`
- Modify: `skwad/main.go`

**Step 1: Write failing test for session creation**

Create `skwad/api/handlers_test.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kyleg/skwad/db"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	database, err := db.New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return NewServer(database)
}

func TestCreateSession(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/sessions", nil)
	w := httptest.NewRecorder()

	srv.HandleCreateSession(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] == nil {
		t.Error("response missing 'id'")
	}
}

func TestJoinSession(t *testing.T) {
	srv := newTestServer(t)

	// Create session first
	req := httptest.NewRequest("POST", "/api/sessions", nil)
	w := httptest.NewRecorder()
	srv.HandleCreateSession(w, req)
	var session map[string]interface{}
	json.NewDecoder(w.Body).Decode(&session)
	code := session["id"].(string)

	// Join it
	body := `{"callsign":"TestPilot","video_system":"analog"}`
	req = httptest.NewRequest("POST", "/api/sessions/"+code+"/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.HandleJoinSession(w, req, code)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestGetSession(t *testing.T) {
	srv := newTestServer(t)

	// Create + join
	req := httptest.NewRequest("POST", "/api/sessions", nil)
	w := httptest.NewRecorder()
	srv.HandleCreateSession(w, req)
	var session map[string]interface{}
	json.NewDecoder(w.Body).Decode(&session)
	code := session["id"].(string)

	body := `{"callsign":"TestPilot","video_system":"analog"}`
	req = httptest.NewRequest("POST", "/api/sessions/"+code+"/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.HandleJoinSession(w, req, code)

	// Get session
	req = httptest.NewRequest("GET", "/api/sessions/"+code, nil)
	w = httptest.NewRecorder()
	srv.HandleGetSession(w, req, code)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	pilots := resp["pilots"].([]interface{})
	if len(pilots) != 1 {
		t.Errorf("got %d pilots, want 1", len(pilots))
	}
}

func TestGetSession_NotFound(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/sessions/XXXXXX", nil)
	w := httptest.NewRecorder()
	srv.HandleGetSession(w, req, "XXXXXX")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go test ./api/ -v
```

Expected: FAIL — package doesn't exist.

**Step 3: Implement API handlers**

Create `skwad/api/handlers.go`:
```go
package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/kyleg/skwad/db"
	"github.com/kyleg/skwad/freq"
)

type Server struct {
	DB *db.DB
}

func NewServer(database *db.DB) *Server {
	return &Server{DB: database}
}

func (s *Server) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	session, err := s.DB.CreateSession()
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		log.Printf("CreateSession error: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(session)
}

func (s *Server) HandleGetSession(w http.ResponseWriter, r *http.Request, code string) {
	session, err := s.DB.GetSession(code)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	pilots, err := s.DB.GetActivePilots(code)
	if err != nil {
		http.Error(w, "failed to get pilots", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session": session,
		"pilots":  pilots,
	})
}

type JoinRequest struct {
	Callsign       string `json:"callsign"`
	VideoSystem    string `json:"video_system"`
	FCCUnlocked    bool   `json:"fcc_unlocked"`
	Goggles        string `json:"goggles"`
	BandwidthMHz   int    `json:"bandwidth_mhz"`
	RaceMode       bool   `json:"race_mode"`
	ChannelLocked  bool   `json:"channel_locked"`
	LockedFreqMHz  int    `json:"locked_frequency_mhz"`
}

func (s *Server) HandleJoinSession(w http.ResponseWriter, r *http.Request, code string) {
	// Verify session exists
	_, err := s.DB.GetSession(code)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	var req JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Callsign == "" || req.VideoSystem == "" {
		http.Error(w, "callsign and video_system are required", http.StatusBadRequest)
		return
	}

	pilot := &db.Pilot{
		Callsign:           req.Callsign,
		VideoSystem:        req.VideoSystem,
		FCCUnlocked:        req.FCCUnlocked,
		Goggles:            req.Goggles,
		BandwidthMHz:       req.BandwidthMHz,
		RaceMode:           req.RaceMode,
		ChannelLocked:      req.ChannelLocked,
		LockedFrequencyMHz: req.LockedFreqMHz,
	}

	added, err := s.DB.AddPilot(code, pilot)
	if err != nil {
		http.Error(w, "failed to add pilot (callsign may already be taken)", http.StatusConflict)
		return
	}

	// Re-optimize all pilots
	s.reoptimize(code)

	// Return the updated pilot
	pilots, _ := s.DB.GetActivePilots(code)
	for _, p := range pilots {
		if p.ID == added.ID {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(p)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(added)
}

func (s *Server) HandleDeactivatePilot(w http.ResponseWriter, r *http.Request, pilotID int, sessionCode string) {
	if err := s.DB.DeactivatePilot(pilotID); err != nil {
		http.Error(w, "failed to deactivate pilot", http.StatusInternalServerError)
		return
	}
	s.reoptimize(sessionCode)
	s.DB.IncrementVersion(sessionCode)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandlePoll(w http.ResponseWriter, r *http.Request, code string) {
	session, err := s.DB.GetSession(code)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version": session.Version,
	})
}

func (s *Server) reoptimize(sessionCode string) {
	pilots, err := s.DB.GetActivePilots(sessionCode)
	if err != nil {
		log.Printf("reoptimize: failed to get pilots: %v", err)
		return
	}

	inputs := make([]freq.PilotInput, len(pilots))
	for i, p := range pilots {
		inputs[i] = freq.PilotInput{
			ID:            p.ID,
			VideoSystem:   p.VideoSystem,
			FCCUnlocked:   p.FCCUnlocked,
			BandwidthMHz:  p.BandwidthMHz,
			RaceMode:      p.RaceMode,
			Goggles:       p.Goggles,
			ChannelLocked: p.ChannelLocked,
			LockedFreqMHz: p.LockedFrequencyMHz,
			PrevChannel:   p.AssignedChannel,
			PrevFreqMHz:   p.AssignedFreqMHz,
		}
	}

	assignments := freq.Optimize(inputs)
	for _, a := range assignments {
		s.DB.UpdatePilotAssignment(a.PilotID, a.Channel, a.FreqMHz, a.BuddyGroup)
	}
	s.DB.IncrementVersion(sessionCode)
}
```

**Step 4: Run tests**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go test ./api/ -v
```

Expected: all PASS.

**Step 5: Wire handlers into main.go**

Update `skwad/main.go`:
```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/kyleg/skwad/api"
	"github.com/kyleg/skwad/db"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/skwad.db"
	}

	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := api.NewServer(database)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// API routes
	mux.HandleFunc("POST /api/sessions", srv.HandleCreateSession)
	mux.HandleFunc("GET /api/sessions/{code}", func(w http.ResponseWriter, r *http.Request) {
		srv.HandleGetSession(w, r, r.PathValue("code"))
	})
	mux.HandleFunc("POST /api/sessions/{code}/join", func(w http.ResponseWriter, r *http.Request) {
		srv.HandleJoinSession(w, r, r.PathValue("code"))
	})
	mux.HandleFunc("GET /api/sessions/{code}/poll", func(w http.ResponseWriter, r *http.Request) {
		srv.HandlePoll(w, r, r.PathValue("code"))
	})
	mux.HandleFunc("DELETE /api/pilots/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		pilotID, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "invalid pilot id", http.StatusBadRequest)
			return
		}
		// Extract session code from query param
		sessionCode := r.URL.Query().Get("session")
		srv.HandleDeactivatePilot(w, r, pilotID, sessionCode)
	})

	// Static files — serve from ./static/ directory
	staticDir := "./static"
	if env := os.Getenv("STATIC_DIR"); env != "" {
		staticDir = env
	}

	// Serve /s/{code} as the session page (same index.html, JS handles routing)
	mux.HandleFunc("GET /s/{code}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, staticDir+"/index.html")
	})

	// Serve all other static files
	mux.Handle("GET /", http.FileServer(http.Dir(staticDir)))

	log.Printf("Skwad listening on :%s (static: %s, db: %s)", port, staticDir, dbPath)
	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
```

**Step 6: Verify it compiles**

Run:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go build -o /dev/null .
```

Expected: no errors.

**Step 7: Commit**

```bash
git add skwad/api/ skwad/main.go
git commit -m "feat(skwad): add REST API handlers with frequency optimization on join"
```

---

## Task 6: Frontend — Landing Page + Session View

**Files:**
- Create: `skwad/static/index.html`
- Create: `skwad/static/app.js`
- Create: `skwad/static/style.css`

This is a large task. The frontend is a single-page app with client-side routing, all in vanilla JS. The design must be extreme high contrast for outdoor use in direct sunlight.

**Step 1: Create the HTML shell**

Create `skwad/static/index.html` — the single HTML file that loads CSS and JS. Contains:
- Landing page view (start/join session)
- Join flow view (callsign, video system selection, follow-ups)
- Session view (pilot list, channel assignments, QR code)

All views are `<div>` sections toggled by JS. No frameworks.

**Step 2: Create the CSS**

Create `skwad/static/style.css` — extreme high contrast design:
- Background: `#0a0a0a` (near-black)
- Text: `#ffffff` (pure white)
- Accent: vivid saturated colors for buddy groups (`#ff3333`, `#33ff33`, `#3399ff`, `#ffcc00`, `#ff66ff`, `#00ffcc`, `#ff9900`, `#cc66ff`)
- Font: system sans-serif stack, bold weight for channel/callsign
- Minimum font sizes: 18px body, 28px channels, 36px callsigns
- Tap targets: minimum 64px height
- No transparency, no blur, no subtle grays

**Step 3: Create the JavaScript**

Create `skwad/static/app.js` — handles:
- Client-side routing (landing vs session view)
- Session creation (POST /api/sessions)
- Join flow (multi-step form → POST /api/sessions/:code/join)
- Session view (GET /api/sessions/:code, polling every 5s)
- QR code generation (use a small inline QR library or canvas-based generator)
- "I'm leaving" button (DELETE /api/pilots/:id)
- URL parsing for /s/{code} deep links

**Step 4: Test manually**

Run the Go server locally:
```bash
cd /home/kyleg/containers/atxfpv.org/skwad
DB_PATH=./test.db STATIC_DIR=./static go run .
```

Visit `http://localhost:8080` in a browser. Test:
- Create session → get code + QR
- Open QR URL in another tab
- Join with a callsign + video system
- See channel assignment appear
- Add more pilots
- Test "I'm leaving" button

**Step 5: Commit**

```bash
git add skwad/static/
git commit -m "feat(skwad): add mobile-first frontend with high-contrast outdoor UI"
```

---

## Task 7: Docker + Compose Integration

**Files:**
- Modify: `skwad/Dockerfile`
- Modify: `docker-compose.yaml`

**Step 1: Finalize Dockerfile**

Update `skwad/Dockerfile` to ensure the static directory and data volume are correct. The builder stage compiles Go; the runtime stage copies the binary and static files.

**Step 2: Add skwad service to docker-compose.yaml**

Add to `docker-compose.yaml`:
```yaml
    skwad:
        build:
            context: ./skwad
            dockerfile: Dockerfile
        container_name: skwad
        environment:
            - PORT=8080
            - DB_PATH=/data/skwad.db
            - STATIC_DIR=/app/static
        volumes:
            - ./skwad/data:/data
        networks:
            - traefik
        labels:
            - traefik.enable=true
            - traefik.http.routers.skwad-http.rule=Host(`skwad.${SITE_DOMAIN}`)
            - traefik.http.routers.skwad-http.entrypoints=web
            - traefik.http.routers.skwad-http.service=skwad
            - traefik.http.routers.skwad-https.rule=Host(`skwad.${SITE_DOMAIN}`)
            - traefik.http.routers.skwad-https.entrypoints=websecure
            - traefik.http.routers.skwad-https.tls.certresolver=cloudflare
            - traefik.http.routers.skwad-https.service=skwad
            - traefik.http.services.skwad.loadbalancer.server.port=8080
        restart: unless-stopped
```

**Step 3: Create data directory and add to gitignore**

```bash
mkdir -p /home/kyleg/containers/atxfpv.org/skwad/data
echo 'skwad/data/' >> /home/kyleg/containers/atxfpv.org/.gitignore
```

**Step 4: Build and test**

```bash
cd /home/kyleg/containers/atxfpv.org
docker compose build skwad
docker compose up -d skwad
```

Verify: `curl http://localhost:8080/healthz` returns `ok`.

**Step 5: Commit**

```bash
git add skwad/Dockerfile docker-compose.yaml .gitignore
git commit -m "feat(skwad): add Docker container and compose integration with Traefik"
```

---

## Task 8: End-to-End Smoke Test

**Step 1: Run all Go tests**

```bash
cd /home/kyleg/containers/atxfpv.org/skwad
go test ./... -v
```

Expected: all PASS.

**Step 2: Manual integration test**

With the Docker container running:
```bash
# Create session
curl -s -X POST http://skwad.atxfpv.hippienet.wtf/api/sessions | jq

# Join with analog pilot
curl -s -X POST http://skwad.atxfpv.hippienet.wtf/api/sessions/{CODE}/join \
  -H 'Content-Type: application/json' \
  -d '{"callsign":"TestPilot","video_system":"analog"}' | jq

# Join with DJI pilot
curl -s -X POST http://skwad.atxfpv.hippienet.wtf/api/sessions/{CODE}/join \
  -H 'Content-Type: application/json' \
  -d '{"callsign":"DJI_Guy","video_system":"dji_o3","fcc_unlocked":true,"bandwidth_mhz":20}' | jq

# Get session — verify both pilots have different frequencies
curl -s http://skwad.atxfpv.hippienet.wtf/api/sessions/{CODE} | jq
```

**Step 3: Verify on mobile**

Open `skwad.atxfpv.hippienet.wtf` on a phone. Check:
- Landing page renders, buttons are large and tappable
- Can create and join sessions
- Session view shows channel assignments with high contrast
- QR code is scannable
- Text is readable (test outdoors if possible)

**Step 4: Commit any fixes**

```bash
git add -A
git commit -m "fix(skwad): smoke test fixes"
```

---

## Summary

| Task | What | Key Files |
|------|------|-----------|
| 1 | Go scaffolding | `main.go`, `go.mod`, `Dockerfile` |
| 2 | SQLite database | `db/db.go`, `db/db_test.go` |
| 3 | Frequency tables | `freq/tables.go`, `freq/tables_test.go` |
| 4 | Optimizer | `freq/optimizer.go`, `freq/optimizer_test.go` |
| 5 | API handlers | `api/handlers.go`, `api/handlers_test.go`, `main.go` |
| 6 | Frontend | `static/index.html`, `static/style.css`, `static/app.js` |
| 7 | Docker + Compose | `Dockerfile`, `docker-compose.yaml` |
| 8 | Smoke test | Manual verification |

**NOTE on frequency data accuracy:** The DJI O3 and O4 FCC 7-channel frequency values in `freq/tables.go` are best-effort from community documentation. These exact MHz values should be verified against actual hardware. The app's architecture makes it easy to update these values — just edit the channel arrays in `tables.go`.

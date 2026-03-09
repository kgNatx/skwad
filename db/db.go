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

// DB wraps a sql.DB connection to the SQLite database.
type DB struct {
	db *sql.DB
}

// Session represents a frequency-coordination session.
type Session struct {
	ID            string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	Version       int
	LeaderPilotID int
}

// Pilot represents a pilot within a session.
type Pilot struct {
	ID                 int
	SessionID          string
	Callsign           string
	VideoSystem        string
	FCCUnlocked        bool
	Goggles            string
	BandwidthMHz       int
	RaceMode           bool
	ChannelLocked      bool
	LockedFrequencyMHz int
	AssignedChannel    string
	AssignedFreqMHz    int
	BuddyGroup        int
	Active             bool
	AnalogBands        string // comma-separated band codes: "R", "R,F,E", etc.
}

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	created_at DATETIME,
	expires_at DATETIME,
	version INTEGER DEFAULT 1
);

CREATE TABLE IF NOT EXISTS pilots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT,
	callsign TEXT,
	video_system TEXT,
	fcc_unlocked BOOLEAN DEFAULT FALSE,
	goggles TEXT DEFAULT '',
	bandwidth_mhz INTEGER DEFAULT 0,
	race_mode BOOLEAN DEFAULT FALSE,
	channel_locked BOOLEAN DEFAULT FALSE,
	locked_frequency_mhz INTEGER DEFAULT 0,
	assigned_channel TEXT DEFAULT '',
	assigned_frequency_mhz INTEGER DEFAULT 0,
	buddy_group INTEGER DEFAULT 0,
	joined_at DATETIME,
	active BOOLEAN DEFAULT TRUE,
	analog_bands TEXT DEFAULT 'R',
	FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
	UNIQUE(session_id, callsign)
);
`

// New opens or creates a SQLite database at path and ensures tables exist.
func New(path string) (*DB, error) {
	dsn := path + "?_pragma=journal_mode(wal)&_pragma=foreign_keys(on)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	d := &DB{db: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

// migrate runs idempotent schema migrations.
func (d *DB) migrate() error {
	_, err := d.db.Exec(`ALTER TABLE sessions ADD COLUMN leader_pilot_id INTEGER DEFAULT 0`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate leader_pilot_id: %w", err)
	}
	_, err = d.db.Exec(`ALTER TABLE pilots ADD COLUMN analog_bands TEXT DEFAULT 'R'`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate analog_bands: %w", err)
	}
	return nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

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

// GetSession retrieves a session by ID. Returns an error if the session does
// not exist or has expired.
func (d *DB) GetSession(id string) (*Session, error) {
	var s Session
	err := d.db.QueryRow(
		`SELECT id, created_at, expires_at, version, leader_pilot_id FROM sessions WHERE id = ? AND expires_at > datetime('now')`,
		id,
	).Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt, &s.Version, &s.LeaderPilotID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session %q not found or expired", id)
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	return &s, nil
}

// IncrementVersion bumps the session version by 1.
func (d *DB) IncrementVersion(sessionID string) error {
	res, err := d.db.Exec(
		`UPDATE sessions SET version = version + 1 WHERE id = ?`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("increment version: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("session %q not found", sessionID)
	}
	return nil
}

// SetLeader sets the leader pilot ID for a session.
func (d *DB) SetLeader(sessionID string, pilotID int) error {
	res, err := d.db.Exec(
		`UPDATE sessions SET leader_pilot_id = ? WHERE id = ?`,
		pilotID, sessionID,
	)
	if err != nil {
		return fmt.Errorf("set leader: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("session %q not found", sessionID)
	}
	return nil
}

// GetLeader returns the leader pilot ID for a session (0 = no leader).
func (d *DB) GetLeader(sessionID string) (int, error) {
	var leaderID int
	err := d.db.QueryRow(
		`SELECT leader_pilot_id FROM sessions WHERE id = ?`,
		sessionID,
	).Scan(&leaderID)
	if err != nil {
		return 0, fmt.Errorf("get leader: %w", err)
	}
	return leaderID, nil
}

// AddPilot inserts a new pilot into the given session and returns the pilot
// with its auto-generated ID populated. If the callsign already exists but is
// inactive (pilot left and came back), reactivate them with updated settings.
func (d *DB) AddPilot(sessionID string, p *Pilot) (*Pilot, error) {
	res, err := d.db.Exec(
		`INSERT INTO pilots (session_id, callsign, video_system, fcc_unlocked, goggles,
			bandwidth_mhz, race_mode, channel_locked, locked_frequency_mhz,
			assigned_channel, assigned_frequency_mhz, buddy_group, joined_at, active, analog_bands)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?)`,
		sessionID, p.Callsign, p.VideoSystem, p.FCCUnlocked, p.Goggles,
		p.BandwidthMHz, p.RaceMode, p.ChannelLocked, p.LockedFrequencyMHz,
		p.AssignedChannel, p.AssignedFreqMHz, p.BuddyGroup, time.Now().UTC(),
		p.AnalogBands,
	)
	if err != nil {
		// If the callsign already exists but is inactive, reactivate them.
		if strings.Contains(err.Error(), "UNIQUE") {
			return d.reactivatePilot(sessionID, p)
		}
		return nil, fmt.Errorf("insert pilot: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	out := *p
	out.ID = int(id)
	out.SessionID = sessionID
	out.Active = true
	return &out, nil
}

// reactivatePilot reactivates an inactive pilot with updated settings.
// Returns an error if the pilot is still active (true duplicate).
func (d *DB) reactivatePilot(sessionID string, p *Pilot) (*Pilot, error) {
	// Check if the existing pilot is inactive.
	var existingID int
	var active bool
	err := d.db.QueryRow(
		`SELECT id, active FROM pilots WHERE session_id = ? AND callsign = ?`,
		sessionID, p.Callsign,
	).Scan(&existingID, &active)
	if err != nil {
		return nil, fmt.Errorf("check existing pilot: %w", err)
	}
	if active {
		return nil, fmt.Errorf("insert pilot: UNIQUE constraint failed: callsign already active")
	}

	// Reactivate with updated settings.
	_, err = d.db.Exec(
		`UPDATE pilots SET video_system = ?, fcc_unlocked = ?, goggles = ?,
			bandwidth_mhz = ?, race_mode = ?, channel_locked = ?, locked_frequency_mhz = ?,
			assigned_channel = '', assigned_frequency_mhz = 0, buddy_group = 0,
			joined_at = ?, active = TRUE, analog_bands = ?
		WHERE id = ?`,
		p.VideoSystem, p.FCCUnlocked, p.Goggles,
		p.BandwidthMHz, p.RaceMode, p.ChannelLocked, p.LockedFrequencyMHz,
		time.Now().UTC(), p.AnalogBands, existingID,
	)
	if err != nil {
		return nil, fmt.Errorf("reactivate pilot: %w", err)
	}

	out := *p
	out.ID = existingID
	out.SessionID = sessionID
	out.Active = true
	return &out, nil
}

// GetActivePilots returns all active pilots for a session, ordered by ID.
func (d *DB) GetActivePilots(sessionID string) ([]Pilot, error) {
	rows, err := d.db.Query(
		`SELECT id, session_id, callsign, video_system, fcc_unlocked, goggles,
			bandwidth_mhz, race_mode, channel_locked, locked_frequency_mhz,
			assigned_channel, assigned_frequency_mhz, buddy_group, active, analog_bands
		FROM pilots
		WHERE session_id = ? AND active = TRUE
		ORDER BY id`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query active pilots: %w", err)
	}
	defer rows.Close()

	var pilots []Pilot
	for rows.Next() {
		var p Pilot
		if err := rows.Scan(
			&p.ID, &p.SessionID, &p.Callsign, &p.VideoSystem, &p.FCCUnlocked,
			&p.Goggles, &p.BandwidthMHz, &p.RaceMode, &p.ChannelLocked,
			&p.LockedFrequencyMHz, &p.AssignedChannel, &p.AssignedFreqMHz,
			&p.BuddyGroup, &p.Active, &p.AnalogBands,
		); err != nil {
			return nil, fmt.Errorf("scan pilot: %w", err)
		}
		pilots = append(pilots, p)
	}
	return pilots, rows.Err()
}

// UpdatePilotAssignment sets the assigned channel, frequency, and buddy group for a pilot.
func (d *DB) UpdatePilotAssignment(pilotID int, channel string, freqMHz int, buddyGroup int) error {
	res, err := d.db.Exec(
		`UPDATE pilots SET assigned_channel = ?, assigned_frequency_mhz = ?, buddy_group = ? WHERE id = ?`,
		channel, freqMHz, buddyGroup, pilotID,
	)
	if err != nil {
		return fmt.Errorf("update pilot assignment: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pilot %d not found", pilotID)
	}
	return nil
}

// UpdatePilotPreferences updates a pilot's channel lock and locked frequency.
func (d *DB) UpdatePilotPreferences(pilotID int, channelLocked bool, lockedFreqMHz int) error {
	res, err := d.db.Exec(
		`UPDATE pilots SET channel_locked = ?, locked_frequency_mhz = ? WHERE id = ? AND active = TRUE`,
		channelLocked, lockedFreqMHz, pilotID,
	)
	if err != nil {
		return fmt.Errorf("update pilot preferences: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pilot %d not found or inactive", pilotID)
	}
	return nil
}

// UpdatePilotVideoSystem changes an active pilot's video system and related settings.
func (d *DB) UpdatePilotVideoSystem(pilotID int, videoSystem string, fccUnlocked bool, goggles string, bandwidthMHz int, raceMode bool, analogBands string) error {
	res, err := d.db.Exec(
		`UPDATE pilots SET video_system = ?, fcc_unlocked = ?, goggles = ?, bandwidth_mhz = ?, race_mode = ?, analog_bands = ? WHERE id = ? AND active = TRUE`,
		videoSystem, fccUnlocked, goggles, bandwidthMHz, raceMode, analogBands, pilotID,
	)
	if err != nil {
		return fmt.Errorf("update pilot video system: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pilot %d not found or inactive", pilotID)
	}
	return nil
}

// UpdatePilotCallsign changes an active pilot's callsign.
func (d *DB) UpdatePilotCallsign(pilotID int, callsign string) error {
	res, err := d.db.Exec(
		`UPDATE pilots SET callsign = ? WHERE id = ? AND active = TRUE`,
		callsign, pilotID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("callsign already in session")
		}
		return fmt.Errorf("update pilot callsign: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pilot %d not found or inactive", pilotID)
	}
	return nil
}

// FindActivePilotByCallsign returns the ID of an active pilot with the given
// callsign in the session, or 0 if not found.
func (d *DB) FindActivePilotByCallsign(sessionID, callsign string) (int, error) {
	var id int
	err := d.db.QueryRow(
		`SELECT id FROM pilots WHERE session_id = ? AND callsign = ? AND active = TRUE`,
		sessionID, callsign,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// DeactivatePilot sets a pilot's active flag to FALSE.
func (d *DB) DeactivatePilot(pilotID int) error {
	res, err := d.db.Exec(
		`UPDATE pilots SET active = FALSE WHERE id = ?`,
		pilotID,
	)
	if err != nil {
		return fmt.Errorf("deactivate pilot: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pilot %d not found", pilotID)
	}
	return nil
}

// DeleteExpiredSessions removes sessions that have passed their expiration time
// and returns the number of sessions deleted.
func (d *DB) DeleteExpiredSessions() (int64, error) {
	res, err := d.db.Exec(
		`DELETE FROM sessions WHERE expires_at <= datetime('now')`,
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return res.RowsAffected()
}

// generateCode produces a 6-character uppercase hex string using crypto/rand.
func generateCode() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return strings.ToUpper(hex.EncodeToString(b))
}

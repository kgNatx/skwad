package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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
	ID             string    `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Version        int       `json:"version"`
	LeaderPilotID  int       `json:"leader_pilot_id"`
	PowerCeilingMW int       `json:"power_ceiling_mw"`
	FixedChannels  string    `json:"fixed_channels"` // JSON array of {name, freq} or empty
}

// Pilot represents a pilot within a session.
type Pilot struct {
	ID               int
	SessionID        string
	Callsign         string
	VideoSystem      string
	FCCUnlocked      bool
	Goggles          string
	BandwidthMHz     int
	RaceMode         bool
	PreferredFreqMHz int // 0 = auto-assign, >0 = preferred frequency
	AssignedChannel  string
	AssignedFreqMHz  int
	BuddyGroup       int
	Active           bool
	AnalogBands      string // comma-separated band codes: "R", "R,F,E", etc.
	AddedByLeader    bool
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
	_, err = d.db.Exec(`ALTER TABLE pilots ADD COLUMN added_by_leader BOOLEAN DEFAULT FALSE`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate added_by_leader: %w", err)
	}
	_, err = d.db.Exec(`ALTER TABLE pilots ADD COLUMN preferred_frequency_mhz INTEGER DEFAULT 0`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate preferred_frequency_mhz: %w", err)
	}
	// Copy locked_frequency_mhz -> preferred_frequency_mhz for existing locked pilots.
	_, err = d.db.Exec(`UPDATE pilots SET preferred_frequency_mhz = locked_frequency_mhz WHERE channel_locked = TRUE AND preferred_frequency_mhz = 0`)
	if err != nil {
		return fmt.Errorf("migrate lock to preference: %w", err)
	}
	_, err = d.db.Exec(`ALTER TABLE sessions ADD COLUMN power_ceiling_mw INTEGER DEFAULT 0`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate power_ceiling_mw: %w", err)
	}
	_, err = d.db.Exec(`ALTER TABLE sessions ADD COLUMN fixed_channels TEXT DEFAULT ''`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate fixed_channels: %w", err)
	}

	// Usage metrics columns on sessions.
	for _, col := range []struct{ name, def string }{
		{"city", "TEXT DEFAULT ''"},
		{"region", "TEXT DEFAULT ''"},
		{"country", "TEXT DEFAULT ''"},
		{"latitude", "REAL DEFAULT 0"},
		{"longitude", "REAL DEFAULT 0"},
		{"peak_pilot_count", "INTEGER DEFAULT 0"},
		{"total_joins", "INTEGER DEFAULT 0"},
		{"rebalance_count", "INTEGER DEFAULT 0"},
		{"channel_change_count", "INTEGER DEFAULT 0"},
	} {
		_, err = d.db.Exec(fmt.Sprintf(`ALTER TABLE sessions ADD COLUMN %s %s`, col.name, col.def))
		if err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migrate %s: %w", col.name, err)
		}
	}

	// Snapshot table for metrics after session expiry.
	_, err = d.db.Exec(`CREATE TABLE IF NOT EXISTS session_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_code TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		expired_at DATETIME NOT NULL,
		duration_minutes INTEGER NOT NULL,
		peak_pilot_count INTEGER NOT NULL,
		total_joins INTEGER NOT NULL,
		rebalance_count INTEGER NOT NULL,
		channel_change_count INTEGER NOT NULL,
		video_systems TEXT NOT NULL DEFAULT '{}',
		power_ceiling_mw INTEGER NOT NULL DEFAULT 0,
		used_fixed_channels BOOLEAN NOT NULL DEFAULT 0,
		city TEXT NOT NULL DEFAULT '',
		region TEXT NOT NULL DEFAULT '',
		country TEXT NOT NULL DEFAULT '',
		latitude REAL NOT NULL DEFAULT 0,
		longitude REAL NOT NULL DEFAULT 0,
		UNIQUE(session_code, created_at)
	)`)
	if err != nil {
		return fmt.Errorf("create session_snapshots: %w", err)
	}

	return nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// CreateSession generates a new session with a 6-char hex code that expires in 12 hours.
// powerCeilingMW sets the TX power ceiling for the session (0 = no limit).
// fixedChannels is a JSON array of {name, freq} objects, or empty string for none.
// Retries with a new code if a collision occurs (primary key conflict).
func (d *DB) CreateSession(powerCeilingMW int, fixedChannels string) (*Session, error) {
	now := time.Now().UTC()
	expires := now.Add(12 * time.Hour)

	const maxRetries = 5
	for i := 0; i < maxRetries; i++ {
		id := generateCode()
		_, err := d.db.Exec(
			`INSERT INTO sessions (id, created_at, expires_at, version, power_ceiling_mw, fixed_channels) VALUES (?, ?, ?, 1, ?, ?)`,
			id, now, expires, powerCeilingMW, fixedChannels,
		)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "PRIMARY") {
				continue // collision, retry with new code
			}
			return nil, fmt.Errorf("insert session: %w", err)
		}
		return &Session{
			ID:             id,
			CreatedAt:      now,
			ExpiresAt:      expires,
			Version:        1,
			PowerCeilingMW: powerCeilingMW,
			FixedChannels:  fixedChannels,
		}, nil
	}
	return nil, fmt.Errorf("failed to generate unique session ID after %d attempts", maxRetries)
}

// GetSession retrieves a session by ID. Returns an error if the session does
// not exist or has expired.
func (d *DB) GetSession(id string) (*Session, error) {
	var s Session
	err := d.db.QueryRow(
		`SELECT id, created_at, expires_at, version, leader_pilot_id, COALESCE(power_ceiling_mw, 0), COALESCE(fixed_channels, '') FROM sessions WHERE id = ? AND expires_at > datetime('now')`,
		id,
	).Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt, &s.Version, &s.LeaderPilotID, &s.PowerCeilingMW, &s.FixedChannels)
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

func (d *DB) UpdateSessionPowerCeiling(sessionID string, powerCeilingMW int) error {
	res, err := d.db.Exec(
		`UPDATE sessions SET power_ceiling_mw = ? WHERE id = ?`,
		powerCeilingMW, sessionID,
	)
	if err != nil {
		return fmt.Errorf("update power ceiling: %w", err)
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
			bandwidth_mhz, race_mode, preferred_frequency_mhz,
			assigned_channel, assigned_frequency_mhz, buddy_group, joined_at, active, analog_bands, added_by_leader)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?)`,
		sessionID, p.Callsign, p.VideoSystem, p.FCCUnlocked, p.Goggles,
		p.BandwidthMHz, p.RaceMode, p.PreferredFreqMHz,
		p.AssignedChannel, p.AssignedFreqMHz, p.BuddyGroup, time.Now().UTC(),
		p.AnalogBands, p.AddedByLeader,
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
			bandwidth_mhz = ?, race_mode = ?, preferred_frequency_mhz = ?,
			assigned_channel = '', assigned_frequency_mhz = 0, buddy_group = 0,
			joined_at = ?, active = TRUE, analog_bands = ?
		WHERE id = ?`,
		p.VideoSystem, p.FCCUnlocked, p.Goggles,
		p.BandwidthMHz, p.RaceMode, p.PreferredFreqMHz,
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
			bandwidth_mhz, race_mode, preferred_frequency_mhz,
			assigned_channel, assigned_frequency_mhz, buddy_group, active, analog_bands,
			added_by_leader
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
			&p.Goggles, &p.BandwidthMHz, &p.RaceMode, &p.PreferredFreqMHz,
			&p.AssignedChannel, &p.AssignedFreqMHz,
			&p.BuddyGroup, &p.Active, &p.AnalogBands, &p.AddedByLeader,
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

// UpdatePilotPreference updates a pilot's preferred frequency (0 = auto-assign).
func (d *DB) UpdatePilotPreference(pilotID int, preferredFreqMHz int) error {
	res, err := d.db.Exec(
		`UPDATE pilots SET preferred_frequency_mhz = ? WHERE id = ? AND active = TRUE`,
		preferredFreqMHz, pilotID,
	)
	if err != nil {
		return fmt.Errorf("update pilot preference: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pilot %d not found or inactive", pilotID)
	}
	return nil
}

// UpdatePilotVideoSystem changes an active pilot's video system and related settings.
func (d *DB) UpdatePilotVideoSystem(pilotID int, videoSystem string, fccUnlocked bool, goggles string, bandwidthMHz int, raceMode bool, analogBands string, preferredFreqMHz int) error {
	res, err := d.db.Exec(
		`UPDATE pilots SET video_system = ?, fcc_unlocked = ?, goggles = ?, bandwidth_mhz = ?, race_mode = ?, analog_bands = ?, preferred_frequency_mhz = ? WHERE id = ? AND active = TRUE`,
		videoSystem, fccUnlocked, goggles, bandwidthMHz, raceMode, analogBands, preferredFreqMHz, pilotID,
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

// UpdateSessionGeo sets the geolocation fields for a session.
func (d *DB) UpdateSessionGeo(sessionID, city, region, country string, lat, lng float64) error {
	_, err := d.db.Exec(
		`UPDATE sessions SET city = ?, region = ?, country = ?, latitude = ?, longitude = ? WHERE id = ?`,
		city, region, country, lat, lng, sessionID,
	)
	if err != nil {
		return fmt.Errorf("update session geo: %w", err)
	}
	return nil
}

// IncrementJoinCount bumps total_joins and updates peak_pilot_count for a session.
func (d *DB) IncrementJoinCount(sessionID string) error {
	_, err := d.db.Exec(
		`UPDATE sessions SET total_joins = total_joins + 1,
			peak_pilot_count = MAX(peak_pilot_count,
				(SELECT COUNT(*) FROM pilots WHERE session_id = ? AND active = TRUE))
		WHERE id = ?`,
		sessionID, sessionID,
	)
	if err != nil {
		return fmt.Errorf("increment join count: %w", err)
	}
	return nil
}

// IncrementRebalanceCount bumps rebalance_count for a session.
func (d *DB) IncrementRebalanceCount(sessionID string) error {
	_, err := d.db.Exec(
		`UPDATE sessions SET rebalance_count = rebalance_count + 1 WHERE id = ?`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("increment rebalance count: %w", err)
	}
	return nil
}

// IncrementChannelChangeCount bumps channel_change_count for a session.
func (d *DB) IncrementChannelChangeCount(sessionID string) error {
	_, err := d.db.Exec(
		`UPDATE sessions SET channel_change_count = channel_change_count + 1 WHERE id = ?`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("increment channel change count: %w", err)
	}
	return nil
}

// SnapshotAndDeleteExpiredSessions creates snapshot rows for expired sessions,
// then deletes them. All within a single transaction.
func (d *DB) SnapshotAndDeleteExpiredSessions() (int64, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Find expired sessions.
	rows, err := tx.Query(
		`SELECT id, created_at, expires_at, COALESCE(power_ceiling_mw, 0),
			COALESCE(fixed_channels, ''), COALESCE(city, ''), COALESCE(region, ''),
			COALESCE(country, ''), COALESCE(latitude, 0), COALESCE(longitude, 0),
			COALESCE(peak_pilot_count, 0), COALESCE(total_joins, 0),
			COALESCE(rebalance_count, 0), COALESCE(channel_change_count, 0)
		FROM sessions WHERE expires_at <= datetime('now')`,
	)
	if err != nil {
		return 0, fmt.Errorf("query expired sessions: %w", err)
	}

	type expiredSession struct {
		id, fixedChannels, city, region, country                    string
		createdAt, expiresAt                                        time.Time
		powerCeilingMW, peakPilots, totalJoins, rebalances, changes int
		lat, lng                                                    float64
	}
	var expired []expiredSession

	for rows.Next() {
		var s expiredSession
		if err := rows.Scan(&s.id, &s.createdAt, &s.expiresAt, &s.powerCeilingMW,
			&s.fixedChannels, &s.city, &s.region, &s.country, &s.lat, &s.lng,
			&s.peakPilots, &s.totalJoins, &s.rebalances, &s.changes); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan expired session: %w", err)
		}
		expired = append(expired, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate expired sessions: %w", err)
	}

	if len(expired) == 0 {
		return 0, nil
	}

	// Snapshot each session.
	for _, s := range expired {
		// Build video system summary from all pilots (active or not).
		vsRows, err := tx.Query(
			`SELECT video_system, COUNT(*) FROM pilots WHERE session_id = ? GROUP BY video_system`,
			s.id,
		)
		if err != nil {
			return 0, fmt.Errorf("query video systems for %s: %w", s.id, err)
		}

		vsSummary := make(map[string]int)
		for vsRows.Next() {
			var vs string
			var count int
			if err := vsRows.Scan(&vs, &count); err != nil {
				vsRows.Close()
				return 0, fmt.Errorf("scan video system: %w", err)
			}
			vsSummary[vs] = count
		}
		vsRows.Close()

		vsJSON, _ := json.Marshal(vsSummary)

		durationMinutes := int(s.expiresAt.Sub(s.createdAt).Minutes())
		usedFixed := s.fixedChannels != "" && s.fixedChannels != "[]"

		_, err = tx.Exec(
			`INSERT OR IGNORE INTO session_snapshots
				(session_code, created_at, expired_at, duration_minutes,
				peak_pilot_count, total_joins, rebalance_count, channel_change_count,
				video_systems, power_ceiling_mw, used_fixed_channels,
				city, region, country, latitude, longitude)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.id, s.createdAt, s.expiresAt, durationMinutes,
			s.peakPilots, s.totalJoins, s.rebalances, s.changes,
			string(vsJSON), s.powerCeilingMW, usedFixed,
			s.city, s.region, s.country, s.lat, s.lng,
		)
		if err != nil {
			return 0, fmt.Errorf("insert snapshot for %s: %w", s.id, err)
		}
	}

	// Delete expired sessions (cascade deletes pilots).
	res, err := tx.Exec(`DELETE FROM sessions WHERE expires_at <= datetime('now')`)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return res.RowsAffected()
}

// UsageLocation represents a geographic location with session count.
type UsageLocation struct {
	City    string  `json:"city"`
	Region  string  `json:"region"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Count   int     `json:"count"`
}

// UsageStats holds aggregate metrics from session snapshots.
type UsageStats struct {
	TotalSessions             int                `json:"total_sessions"`
	TotalPilotsJoined         int                `json:"total_pilots_joined"`
	AvgPilotsPerSession       float64            `json:"avg_pilots_per_session"`
	PeakSessionSize           int                `json:"peak_session_size"`
	AvgSessionDurationMinutes float64            `json:"avg_session_duration_minutes"`
	VideoSystemBreakdown      map[string]int     `json:"video_system_breakdown"`
	TotalRebalances           int                `json:"total_rebalances"`
	TotalChannelChanges       int                `json:"total_channel_changes"`
	SessionsWithPowerCeiling  float64            `json:"sessions_with_power_ceiling_pct"`
	SessionsWithFixedChannels float64            `json:"sessions_with_fixed_channels_pct"`
	Locations                 []UsageLocation    `json:"locations"`
}

// GetUsageStats computes aggregate metrics from session_snapshots.
func (d *DB) GetUsageStats() (*UsageStats, error) {
	stats := &UsageStats{
		VideoSystemBreakdown: make(map[string]int),
	}

	// Core aggregates.
	err := d.db.QueryRow(`
		SELECT COUNT(*),
			COALESCE(SUM(total_joins), 0),
			COALESCE(MAX(peak_pilot_count), 0),
			COALESCE(SUM(rebalance_count), 0),
			COALESCE(SUM(channel_change_count), 0)
		FROM session_snapshots
	`).Scan(&stats.TotalSessions, &stats.TotalPilotsJoined,
		&stats.PeakSessionSize, &stats.TotalRebalances, &stats.TotalChannelChanges)
	if err != nil {
		return nil, fmt.Errorf("query core stats: %w", err)
	}

	if stats.TotalSessions == 0 {
		stats.Locations = []UsageLocation{}
		return stats, nil
	}

	// Averages (exclude unused sessions).
	d.db.QueryRow(`
		SELECT COALESCE(AVG(peak_pilot_count), 0)
		FROM session_snapshots WHERE total_joins > 0
	`).Scan(&stats.AvgPilotsPerSession)

	d.db.QueryRow(`
		SELECT COALESCE(AVG(duration_minutes), 0)
		FROM session_snapshots WHERE total_joins > 0
	`).Scan(&stats.AvgSessionDurationMinutes)

	// Feature adoption percentages.
	var withPower, withFixed int
	d.db.QueryRow(`SELECT COUNT(*) FROM session_snapshots WHERE power_ceiling_mw > 0`).Scan(&withPower)
	d.db.QueryRow(`SELECT COUNT(*) FROM session_snapshots WHERE used_fixed_channels = 1`).Scan(&withFixed)
	stats.SessionsWithPowerCeiling = float64(withPower) / float64(stats.TotalSessions) * 100
	stats.SessionsWithFixedChannels = float64(withFixed) / float64(stats.TotalSessions) * 100

	// Video system breakdown via json_each.
	vsRows, err := d.db.Query(`
		SELECT j.key, SUM(j.value)
		FROM session_snapshots, json_each(video_systems) j
		WHERE video_systems != '{}'
		GROUP BY j.key
	`)
	if err == nil {
		defer vsRows.Close()
		for vsRows.Next() {
			var system string
			var count int
			if vsRows.Scan(&system, &count) == nil {
				stats.VideoSystemBreakdown[system] = count
			}
		}
	}

	// Locations grouped by city.
	locRows, err := d.db.Query(`
		SELECT city, region, country, AVG(latitude), AVG(longitude), COUNT(*)
		FROM session_snapshots
		WHERE city != ''
		GROUP BY city, region, country
		ORDER BY COUNT(*) DESC
	`)
	if err == nil {
		defer locRows.Close()
		for locRows.Next() {
			var loc UsageLocation
			if locRows.Scan(&loc.City, &loc.Region, &loc.Country, &loc.Lat, &loc.Lng, &loc.Count) == nil {
				stats.Locations = append(stats.Locations, loc)
			}
		}
	}

	if stats.Locations == nil {
		stats.Locations = []UsageLocation{}
	}

	return stats, nil
}

// generateCode produces a 6-character uppercase hex string using crypto/rand.
func generateCode() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return strings.ToUpper(hex.EncodeToString(b))
}

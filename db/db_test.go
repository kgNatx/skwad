package db

import (
	"path/filepath"
	"testing"
)

// newTestDB creates a temporary database for testing.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	d, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestNewDB_CreatesTables(t *testing.T) {
	d := newTestDB(t)

	// Verify sessions table exists by querying sqlite_master.
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sessions'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("querying sqlite_master for sessions: %v", err)
	}
	if count != 1 {
		t.Errorf("sessions table: got count %d, want 1", count)
	}

	// Verify pilots table exists.
	err = d.db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='pilots'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("querying sqlite_master for pilots: %v", err)
	}
	if count != 1 {
		t.Errorf("pilots table: got count %d, want 1", count)
	}
}

func TestCreateSession(t *testing.T) {
	d := newTestDB(t)

	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// ID should be 6 uppercase hex characters.
	if len(sess.ID) != 6 {
		t.Errorf("session ID length: got %d, want 6", len(sess.ID))
	}
	for _, c := range sess.ID {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			t.Errorf("session ID contains non-hex char: %c", c)
		}
	}

	// ExpiresAt should be after CreatedAt.
	if !sess.ExpiresAt.After(sess.CreatedAt) {
		t.Errorf("ExpiresAt (%v) should be after CreatedAt (%v)", sess.ExpiresAt, sess.CreatedAt)
	}

	// Version should be 1.
	if sess.Version != 1 {
		t.Errorf("session version: got %d, want 1", sess.Version)
	}
}

func TestGetSession(t *testing.T) {
	d := newTestDB(t)

	created, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := d.GetSession(created.ID)
	if err != nil {
		t.Fatalf("GetSession(%q): %v", created.ID, err)
	}

	if got.ID != created.ID {
		t.Errorf("ID: got %q, want %q", got.ID, created.ID)
	}
	if got.Version != created.Version {
		t.Errorf("Version: got %d, want %d", got.Version, created.Version)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	d := newTestDB(t)

	_, err := d.GetSession("ZZZZZZ")
	if err == nil {
		t.Fatal("GetSession for missing session should return error")
	}
}

func TestGetSession_Expired(t *testing.T) {
	d := newTestDB(t)

	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Manually expire the session by setting expires_at to the past.
	_, err = d.db.Exec(
		`UPDATE sessions SET expires_at = datetime('now', '-1 hour') WHERE id = ?`,
		sess.ID,
	)
	if err != nil {
		t.Fatalf("expiring session: %v", err)
	}

	_, err = d.GetSession(sess.ID)
	if err == nil {
		t.Fatal("GetSession for expired session should return error")
	}
}

func TestAddPilot(t *testing.T) {
	d := newTestDB(t)

	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	p := &Pilot{
		SessionID:   sess.ID,
		Callsign:    "RACER1",
		VideoSystem: "HDZero",
		FCCUnlocked: true,
		Goggles:     "HDZero Goggles",
	}
	got, err := d.AddPilot(sess.ID, p)
	if err != nil {
		t.Fatalf("AddPilot: %v", err)
	}

	if got.ID == 0 {
		t.Error("pilot ID should be non-zero after insert")
	}
	if got.Callsign != "RACER1" {
		t.Errorf("callsign: got %q, want %q", got.Callsign, "RACER1")
	}
	if got.SessionID != sess.ID {
		t.Errorf("session_id: got %q, want %q", got.SessionID, sess.ID)
	}
	if !got.Active {
		t.Error("pilot should be active after insert")
	}
}

func TestAddPilot_DuplicateCallsign(t *testing.T) {
	d := newTestDB(t)

	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	p := &Pilot{
		Callsign:    "DUPES",
		VideoSystem: "Analog",
	}
	_, err = d.AddPilot(sess.ID, p)
	if err != nil {
		t.Fatalf("first AddPilot: %v", err)
	}

	_, err = d.AddPilot(sess.ID, p)
	if err == nil {
		t.Fatal("second AddPilot with same callsign should return error")
	}
}

func TestGetActivePilots(t *testing.T) {
	d := newTestDB(t)

	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	_, err = d.AddPilot(sess.ID, &Pilot{Callsign: "ALPHA", VideoSystem: "HDZero"})
	if err != nil {
		t.Fatalf("AddPilot ALPHA: %v", err)
	}
	_, err = d.AddPilot(sess.ID, &Pilot{Callsign: "BRAVO", VideoSystem: "Walksnail"})
	if err != nil {
		t.Fatalf("AddPilot BRAVO: %v", err)
	}

	pilots, err := d.GetActivePilots(sess.ID)
	if err != nil {
		t.Fatalf("GetActivePilots: %v", err)
	}

	if len(pilots) != 2 {
		t.Fatalf("got %d pilots, want 2", len(pilots))
	}

	// Should be ordered by id, so ALPHA first.
	if pilots[0].Callsign != "ALPHA" {
		t.Errorf("first pilot callsign: got %q, want %q", pilots[0].Callsign, "ALPHA")
	}
	if pilots[1].Callsign != "BRAVO" {
		t.Errorf("second pilot callsign: got %q, want %q", pilots[1].Callsign, "BRAVO")
	}
}

func TestDeactivatePilot(t *testing.T) {
	d := newTestDB(t)

	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	p, err := d.AddPilot(sess.ID, &Pilot{Callsign: "GHOST", VideoSystem: "DJI"})
	if err != nil {
		t.Fatalf("AddPilot: %v", err)
	}

	err = d.DeactivatePilot(p.ID)
	if err != nil {
		t.Fatalf("DeactivatePilot: %v", err)
	}

	pilots, err := d.GetActivePilots(sess.ID)
	if err != nil {
		t.Fatalf("GetActivePilots: %v", err)
	}

	if len(pilots) != 0 {
		t.Errorf("got %d active pilots, want 0 after deactivation", len(pilots))
	}
}

func TestUpdatePilotAssignment(t *testing.T) {
	d := newTestDB(t)

	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	p, err := d.AddPilot(sess.ID, &Pilot{Callsign: "ASSIGN", VideoSystem: "HDZero"})
	if err != nil {
		t.Fatalf("AddPilot: %v", err)
	}

	err = d.UpdatePilotAssignment(p.ID, "R1", 5658, 1)
	if err != nil {
		t.Fatalf("UpdatePilotAssignment: %v", err)
	}

	pilots, err := d.GetActivePilots(sess.ID)
	if err != nil {
		t.Fatalf("GetActivePilots: %v", err)
	}
	if len(pilots) != 1 {
		t.Fatalf("got %d pilots, want 1", len(pilots))
	}
	if pilots[0].AssignedChannel != "R1" {
		t.Errorf("assigned_channel: got %q, want %q", pilots[0].AssignedChannel, "R1")
	}
	if pilots[0].AssignedFreqMHz != 5658 {
		t.Errorf("assigned_frequency_mhz: got %d, want %d", pilots[0].AssignedFreqMHz, 5658)
	}
	if pilots[0].BuddyGroup != 1 {
		t.Errorf("buddy_group: got %d, want %d", pilots[0].BuddyGroup, 1)
	}
}

func TestIncrementVersion(t *testing.T) {
	d := newTestDB(t)

	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	err = d.IncrementVersion(sess.ID)
	if err != nil {
		t.Fatalf("IncrementVersion: %v", err)
	}

	got, err := d.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Version != 2 {
		t.Errorf("version: got %d, want 2", got.Version)
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	d := newTestDB(t)

	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Expire the session.
	_, err = d.db.Exec(
		`UPDATE sessions SET expires_at = datetime('now', '-1 hour') WHERE id = ?`,
		sess.ID,
	)
	if err != nil {
		t.Fatalf("expiring session: %v", err)
	}

	deleted, err := d.DeleteExpiredSessions()
	if err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted: got %d, want 1", deleted)
	}

	// Verify session is gone.
	_, err = d.GetSession(sess.ID)
	if err == nil {
		t.Fatal("session should be gone after delete")
	}
}

func TestUpdatePilotCallsign(t *testing.T) {
	d := newTestDB(t)

	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	p, err := d.AddPilot(sess.ID, &Pilot{Callsign: "OLDNAME", VideoSystem: "analog"})
	if err != nil {
		t.Fatalf("AddPilot: %v", err)
	}

	err = d.UpdatePilotCallsign(p.ID, "NEWNAME")
	if err != nil {
		t.Fatalf("UpdatePilotCallsign: %v", err)
	}

	pilots, err := d.GetActivePilots(sess.ID)
	if err != nil {
		t.Fatalf("GetActivePilots: %v", err)
	}
	if len(pilots) != 1 {
		t.Fatalf("got %d pilots, want 1", len(pilots))
	}
	if pilots[0].Callsign != "NEWNAME" {
		t.Errorf("callsign: got %q, want %q", pilots[0].Callsign, "NEWNAME")
	}
}

func TestUpdatePilotCallsign_Duplicate(t *testing.T) {
	d := newTestDB(t)

	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	_, err = d.AddPilot(sess.ID, &Pilot{Callsign: "ALPHA", VideoSystem: "analog"})
	if err != nil {
		t.Fatalf("AddPilot ALPHA: %v", err)
	}
	p2, err := d.AddPilot(sess.ID, &Pilot{Callsign: "BRAVO", VideoSystem: "analog"})
	if err != nil {
		t.Fatalf("AddPilot BRAVO: %v", err)
	}

	err = d.UpdatePilotCallsign(p2.ID, "ALPHA")
	if err == nil {
		t.Fatal("expected error for duplicate callsign, got nil")
	}
}

func TestCreateSession_CollisionRetry(t *testing.T) {
	d := newTestDB(t)

	first, err := d.CreateSession()
	if err != nil {
		t.Fatalf("first CreateSession: %v", err)
	}

	// Creating many sessions should never fail — retries handle collisions.
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

func TestSetAndGetLeader(t *testing.T) {
	d := newTestDB(t)
	sess, _ := d.CreateSession()

	// No leader initially.
	leaderID, err := d.GetLeader(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leaderID != 0 {
		t.Errorf("expected no leader (0), got %d", leaderID)
	}

	// Set leader.
	if err := d.SetLeader(sess.ID, 42); err != nil {
		t.Fatal(err)
	}
	leaderID, err = d.GetLeader(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leaderID != 42 {
		t.Errorf("expected leader 42, got %d", leaderID)
	}
}

func TestTransferLeader(t *testing.T) {
	d := newTestDB(t)
	sess, _ := d.CreateSession()
	d.SetLeader(sess.ID, 10)

	if err := d.SetLeader(sess.ID, 20); err != nil {
		t.Fatal(err)
	}
	leaderID, _ := d.GetLeader(sess.ID)
	if leaderID != 20 {
		t.Errorf("expected leader 20 after transfer, got %d", leaderID)
	}
}

func TestMigration_PreferredFrequency(t *testing.T) {
	d := newTestDB(t)

	// Verify the preferred_frequency_mhz column exists.
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('pilots') WHERE name='preferred_frequency_mhz'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("checking column: %v", err)
	}
	if count != 1 {
		t.Error("preferred_frequency_mhz column should exist after migration")
	}
}

func TestAddPilot_PreferredFrequency(t *testing.T) {
	d := newTestDB(t)
	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	pilot := &Pilot{
		Callsign:         "TESTPILOT",
		VideoSystem:      "analog",
		PreferredFreqMHz: 5732,
	}
	added, err := d.AddPilot(sess.ID, pilot)
	if err != nil {
		t.Fatalf("AddPilot: %v", err)
	}
	if added.PreferredFreqMHz != 5732 {
		t.Errorf("PreferredFreqMHz = %d, want 5732", added.PreferredFreqMHz)
	}

	// Verify it round-trips through GetActivePilots.
	pilots, err := d.GetActivePilots(sess.ID)
	if err != nil {
		t.Fatalf("GetActivePilots: %v", err)
	}
	if len(pilots) != 1 {
		t.Fatalf("expected 1 pilot, got %d", len(pilots))
	}
	if pilots[0].PreferredFreqMHz != 5732 {
		t.Errorf("round-trip PreferredFreqMHz = %d, want 5732", pilots[0].PreferredFreqMHz)
	}
}

func TestUpdatePilotPreference(t *testing.T) {
	d := newTestDB(t)
	sess, err := d.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	pilot := &Pilot{Callsign: "TESTPILOT", VideoSystem: "analog"}
	added, err := d.AddPilot(sess.ID, pilot)
	if err != nil {
		t.Fatalf("AddPilot: %v", err)
	}

	// Update preference.
	if err := d.UpdatePilotPreference(added.ID, 5806); err != nil {
		t.Fatalf("UpdatePilotPreference: %v", err)
	}

	pilots, err := d.GetActivePilots(sess.ID)
	if err != nil {
		t.Fatalf("GetActivePilots: %v", err)
	}
	if pilots[0].PreferredFreqMHz != 5806 {
		t.Errorf("after update PreferredFreqMHz = %d, want 5806", pilots[0].PreferredFreqMHz)
	}
}

func TestGenerateCode(t *testing.T) {
	code := generateCode()
	if len(code) != 6 {
		t.Errorf("code length: got %d, want 6", len(code))
	}
	for _, c := range code {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			t.Errorf("code contains non-hex char: %c in %q", c, code)
		}
	}

	// Two codes should be different (probabilistically).
	code2 := generateCode()
	if code == code2 {
		t.Errorf("two generated codes are identical: %q", code)
	}
}

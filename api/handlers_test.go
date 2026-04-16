package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleg/skwad/db"
	"github.com/kyleg/skwad/freq"
)

// newTestServer creates a Server backed by a temporary SQLite database.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("db.New(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { database.Close() })
	return NewServer(database, "")
}

func TestCreateSession(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	w := httptest.NewRecorder()

	s.HandleCreateSession(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var sess db.Session
	if err := json.NewDecoder(resp.Body).Decode(&sess); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if sess.ID == "" {
		t.Error("session ID should not be empty")
	}
	if len(sess.ID) != 6 {
		t.Errorf("session ID length = %d, want 6", len(sess.ID))
	}
}

func TestJoinSession(t *testing.T) {
	s := newTestServer(t)

	// Create a session first.
	createReq := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, createReq)

	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Join with an analog pilot.
	joinBody := JoinRequest{
		Callsign:    "TESTPILOT",
		VideoSystem: "analog",
	}
	body, _ := json.Marshal(joinBody)
	joinReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body))
	joinW := httptest.NewRecorder()

	s.HandleJoinSession(joinW, joinReq, sess.ID)

	resp := joinW.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var pilot db.Pilot
	if err := json.NewDecoder(resp.Body).Decode(&pilot); err != nil {
		t.Fatalf("decode pilot: %v", err)
	}

	if pilot.Callsign != "TESTPILOT" {
		t.Errorf("callsign = %q, want %q", pilot.Callsign, "TESTPILOT")
	}
	if pilot.ID == 0 {
		t.Error("pilot ID should be non-zero")
	}
	if pilot.AssignedChannel == "" {
		t.Error("pilot should have an assigned channel after join")
	}
	if pilot.AssignedFreqMHz == 0 {
		t.Error("pilot should have an assigned frequency after join")
	}
}

func TestJoinSession_DuplicateCallsign(t *testing.T) {
	s := newTestServer(t)

	// Create a session.
	createReq := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, createReq)

	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Join once.
	joinBody := JoinRequest{Callsign: "DUPES", VideoSystem: "analog"}
	body, _ := json.Marshal(joinBody)
	joinReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body))
	joinW := httptest.NewRecorder()
	s.HandleJoinSession(joinW, joinReq, sess.ID)

	if joinW.Result().StatusCode != http.StatusCreated {
		t.Fatalf("first join: status = %d, want %d", joinW.Result().StatusCode, http.StatusCreated)
	}

	// Join again with same callsign — should succeed (deactivates old, creates new).
	// This supports the "change video system" flow where a pilot leaves and rejoins.
	body2, _ := json.Marshal(joinBody)
	joinReq2 := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body2))
	joinW2 := httptest.NewRecorder()
	s.HandleJoinSession(joinW2, joinReq2, sess.ID)

	if joinW2.Result().StatusCode != http.StatusCreated {
		t.Errorf("rejoin with same callsign: status = %d, want %d", joinW2.Result().StatusCode, http.StatusCreated)
	}

	// Verify only one active pilot with that callsign.
	var pilot db.Pilot
	json.NewDecoder(joinW2.Result().Body).Decode(&pilot)
	if pilot.Callsign != "DUPES" {
		t.Errorf("rejoin callsign = %q, want %q", pilot.Callsign, "DUPES")
	}
}

func TestJoinSession_MissingFields(t *testing.T) {
	s := newTestServer(t)

	// Create a session.
	createReq := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, createReq)

	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Join without callsign.
	joinBody := JoinRequest{VideoSystem: "analog"}
	body, _ := json.Marshal(joinBody)
	joinReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body))
	joinW := httptest.NewRecorder()
	s.HandleJoinSession(joinW, joinReq, sess.ID)

	if joinW.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("missing callsign: status = %d, want %d", joinW.Result().StatusCode, http.StatusBadRequest)
	}
}

func TestGetSession(t *testing.T) {
	s := newTestServer(t)

	// Create a session and add a pilot.
	createReq := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, createReq)

	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	joinBody := JoinRequest{Callsign: "GETTER", VideoSystem: "analog"}
	body, _ := json.Marshal(joinBody)
	joinReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body))
	joinW := httptest.NewRecorder()
	s.HandleJoinSession(joinW, joinReq, sess.ID)

	// GET the session.
	getReq := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sess.ID, nil)
	getW := httptest.NewRecorder()
	s.HandleGetSession(getW, getReq, sess.ID)

	resp := getW.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result struct {
		Session *db.Session `json:"session"`
		Pilots  []db.Pilot  `json:"pilots"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.Session == nil {
		t.Fatal("session should not be nil")
	}
	if result.Session.ID != sess.ID {
		t.Errorf("session ID = %q, want %q", result.Session.ID, sess.ID)
	}
	if len(result.Pilots) != 1 {
		t.Fatalf("got %d pilots, want 1", len(result.Pilots))
	}
	if result.Pilots[0].Callsign != "GETTER" {
		t.Errorf("pilot callsign = %q, want %q", result.Pilots[0].Callsign, "GETTER")
	}
}

func TestPreviewJoin_ShowsDisplacement(t *testing.T) {
	s := newTestServer(t)

	// Create session.
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, httptest.NewRequest(http.MethodPost, "/api/sessions", nil))
	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Add an analog pilot (will get R1 or R8).
	join1 := JoinRequest{Callsign: "FIRST", VideoSystem: "analog"}
	body1, _ := json.Marshal(join1)
	s.HandleJoinSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body1)), sess.ID)

	// Preview joining a pilot locked to the same channel as FIRST.
	pilots, _ := s.DB.GetActivePilots(sess.ID)
	firstFreq := pilots[0].AssignedFreqMHz

	preview := JoinRequest{Callsign: "SECOND", VideoSystem: "analog", PreferredFreqMHz: firstFreq}
	previewBody, _ := json.Marshal(preview)
	previewW := httptest.NewRecorder()
	s.HandlePreviewJoin(previewW, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(previewBody)), sess.ID)

	if previewW.Result().StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d, want 200", previewW.Result().StatusCode)
	}

	var result PreviewResponse
	json.NewDecoder(previewW.Result().Body).Decode(&result)

	// In the preference system, preferences are soft — the optimizer may place the new
	// pilot on a different channel rather than displacing FIRST. Either the new pilot
	// gets the preferred channel (displacing FIRST) or gets an override reason explaining
	// the alternative placement. Both are valid outcomes.
	if len(result.Displaced) == 0 && result.OverrideReason == "" && result.Assignment.FreqMHz == firstFreq {
		t.Error("expected either displacement of FIRST or override reason or different assignment")
	}

	// Verify the session is unchanged (preview should not commit).
	pilotsAfter, _ := s.DB.GetActivePilots(sess.ID)
	if len(pilotsAfter) != 1 {
		t.Errorf("expected 1 active pilot after preview (no commit), got %d", len(pilotsAfter))
	}
}

func TestPreviewJoin_NoDisplacement(t *testing.T) {
	s := newTestServer(t)

	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, httptest.NewRequest(http.MethodPost, "/api/sessions", nil))
	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Add one analog pilot.
	join1 := JoinRequest{Callsign: "SOLO", VideoSystem: "analog"}
	body1, _ := json.Marshal(join1)
	s.HandleJoinSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body1)), sess.ID)

	// Preview adding a second analog pilot (auto-assigned — should not displace).
	preview := JoinRequest{Callsign: "BUDDY", VideoSystem: "analog"}
	previewBody, _ := json.Marshal(preview)
	previewW := httptest.NewRecorder()
	s.HandlePreviewJoin(previewW, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(previewBody)), sess.ID)

	var result struct {
		Displaced []struct{} `json:"displaced"`
	}
	json.NewDecoder(previewW.Result().Body).Decode(&result)

	if len(result.Displaced) != 0 {
		t.Errorf("expected 0 displaced, got %d", len(result.Displaced))
	}
}

func TestGetSession_NotFound(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/ZZZZZZ", nil)
	w := httptest.NewRecorder()

	s.HandleGetSession(w, req, "ZZZZZZ")

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Result().StatusCode, http.StatusNotFound)
	}
}

func TestPoll(t *testing.T) {
	s := newTestServer(t)

	// Create a session.
	createReq := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, createReq)

	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Poll for version.
	pollReq := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sess.ID+"/poll", nil)
	pollW := httptest.NewRecorder()
	s.HandlePoll(pollW, pollReq, sess.ID)

	resp := pollW.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result struct {
		Version int `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.Version != 1 {
		t.Errorf("version = %d, want 1", result.Version)
	}
}

func TestGetSession_WithConflicts(t *testing.T) {
	s := newTestServer(t)

	// Create session.
	createReq := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, createReq)

	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Add a DJI O3 pilot (40 MHz bandwidth) and an analog pilot.
	join1 := JoinRequest{Callsign: "DJIPILOT", VideoSystem: "dji_o3", BandwidthMHz: 40}
	body1, _ := json.Marshal(join1)
	s.HandleJoinSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body1)), sess.ID)

	join2 := JoinRequest{Callsign: "ANALOGPILOT", VideoSystem: "analog"}
	body2, _ := json.Marshal(join2)
	s.HandleJoinSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body2)), sess.ID)

	// Force conflicting placement via DB: place analog pilot near the DJI O3 pilot.
	pilots, _ := s.DB.GetActivePilots(sess.ID)
	var djiPilot, analogPilot db.Pilot
	for _, p := range pilots {
		if p.Callsign == "DJIPILOT" {
			djiPilot = p
		}
		if p.Callsign == "ANALOGPILOT" {
			analogPilot = p
		}
	}
	// Place analog on R4 (5769) which is within 30 MHz of O3-CH1 (5795) — conflicts.
	// Use BuddyGroup 0 so this is NOT a buddy pair.
	s.DB.UpdatePilotAssignment(sess.ID, djiPilot.ID, "O3-CH1", 5795, 0)
	s.DB.UpdatePilotAssignment(sess.ID, analogPilot.ID, "R4", 5769, 0)

	// GET session and check for conflicts.
	getW := httptest.NewRecorder()
	s.HandleGetSession(getW, httptest.NewRequest(http.MethodGet, "/", nil), sess.ID)

	var result struct {
		Pilots []struct {
			ID        int `json:"ID"`
			Conflicts []struct {
				Level string `json:"level"`
			} `json:"conflicts"`
		} `json:"pilots"`
	}
	if err := json.NewDecoder(getW.Result().Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// At least one pilot should have conflicts.
	hasConflict := false
	for _, p := range result.Pilots {
		if len(p.Conflicts) > 0 {
			hasConflict = true
			break
		}
	}
	if !hasConflict {
		t.Error("expected at least one pilot to have conflicts when analog@5769 is near O3@5795")
	}
}

func TestGetSession_BuddyPairsNoConflict(t *testing.T) {
	s := newTestServer(t)

	// Create session.
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, httptest.NewRequest(http.MethodPost, "/api/sessions", nil))
	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Add two analog pilots.
	join1 := JoinRequest{Callsign: "PILOT1", VideoSystem: "analog"}
	body1, _ := json.Marshal(join1)
	s.HandleJoinSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body1)), sess.ID)

	join2 := JoinRequest{Callsign: "PILOT2", VideoSystem: "analog"}
	body2, _ := json.Marshal(join2)
	s.HandleJoinSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body2)), sess.ID)

	// Manually buddy them up on the same frequency via DB (simulates accepting buddy option).
	pilots, _ := s.DB.GetActivePilots(sess.ID)
	sharedFreq := pilots[0].AssignedFreqMHz
	sharedChannel := pilots[0].AssignedChannel
	buddyGroup := 1
	for _, p := range pilots {
		s.DB.UpdatePilotAssignment(sess.ID, p.ID, sharedChannel, sharedFreq, buddyGroup)
	}

	// GET session — buddy pairs should NOT show conflicts.
	getW := httptest.NewRecorder()
	s.HandleGetSession(getW, httptest.NewRequest(http.MethodGet, "/", nil), sess.ID)

	var result struct {
		Pilots []struct {
			ID         int `json:"ID"`
			BuddyGroup int `json:"BuddyGroup"`
			Conflicts  []struct {
				Level string `json:"level"`
			} `json:"conflicts"`
		} `json:"pilots"`
	}
	if err := json.NewDecoder(getW.Result().Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Both pilots should be in the same buddy group.
	if len(result.Pilots) < 2 {
		t.Fatalf("expected at least 2 pilots, got %d", len(result.Pilots))
	}

	buddied := 0
	for _, p := range result.Pilots {
		if p.BuddyGroup > 0 {
			buddied++
		}
		if len(p.Conflicts) > 0 {
			t.Errorf("pilot %d: expected no conflicts for buddy pair, got %d", p.ID, len(p.Conflicts))
		}
	}
	if buddied < 2 {
		t.Errorf("expected at least 2 pilots in a buddy group, got %d", buddied)
	}
}

func TestUpdateCallsign(t *testing.T) {
	s := newTestServer(t)

	// Create session and join.
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, httptest.NewRequest(http.MethodPost, "/api/sessions", nil))
	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	joinBody := JoinRequest{Callsign: "OLDNAME", VideoSystem: "analog"}
	body, _ := json.Marshal(joinBody)
	joinW := httptest.NewRecorder()
	s.HandleJoinSession(joinW, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)), sess.ID)

	var pilot db.Pilot
	json.NewDecoder(joinW.Result().Body).Decode(&pilot)

	// Change callsign.
	csBody, _ := json.Marshal(map[string]string{"callsign": "NEWNAME"})
	csW := httptest.NewRecorder()
	csReq := httptest.NewRequest(http.MethodPut, "/api/pilots/1/callsign?session="+sess.ID, bytes.NewReader(csBody))
	s.HandleUpdatePilotCallsign(csW, csReq, pilot.ID, sess.ID)

	if csW.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", csW.Result().StatusCode, http.StatusNoContent)
	}

	// Verify the callsign changed.
	getW := httptest.NewRecorder()
	s.HandleGetSession(getW, httptest.NewRequest(http.MethodGet, "/", nil), sess.ID)

	var result struct {
		Pilots []struct {
			Callsign string `json:"Callsign"`
		} `json:"pilots"`
	}
	json.NewDecoder(getW.Result().Body).Decode(&result)

	if len(result.Pilots) != 1 || result.Pilots[0].Callsign != "NEWNAME" {
		t.Errorf("callsign after update: got %v, want NEWNAME", result.Pilots)
	}
}

func TestUpdateCallsign_Duplicate(t *testing.T) {
	s := newTestServer(t)

	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, httptest.NewRequest(http.MethodPost, "/api/sessions", nil))
	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Add two pilots.
	join1 := JoinRequest{Callsign: "ALPHA", VideoSystem: "analog"}
	body1, _ := json.Marshal(join1)
	s.HandleJoinSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body1)), sess.ID)

	join2 := JoinRequest{Callsign: "BRAVO", VideoSystem: "analog"}
	body2, _ := json.Marshal(join2)
	joinW2 := httptest.NewRecorder()
	s.HandleJoinSession(joinW2, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body2)), sess.ID)

	var pilot2 db.Pilot
	json.NewDecoder(joinW2.Result().Body).Decode(&pilot2)

	// Try to change BRAVO to ALPHA — should fail.
	csBody, _ := json.Marshal(map[string]string{"callsign": "ALPHA"})
	csW := httptest.NewRecorder()
	s.HandleUpdatePilotCallsign(csW, httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(csBody)), pilot2.ID, sess.ID)

	if csW.Result().StatusCode != http.StatusConflict {
		t.Errorf("duplicate callsign: status = %d, want %d", csW.Result().StatusCode, http.StatusConflict)
	}
}

func TestDeactivatePilot(t *testing.T) {
	s := newTestServer(t)

	// Create a session and add a pilot.
	createReq := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, createReq)

	var sess db.Session
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	joinBody := JoinRequest{Callsign: "DEACT", VideoSystem: "analog"}
	body, _ := json.Marshal(joinBody)
	joinReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body))
	joinW := httptest.NewRecorder()
	s.HandleJoinSession(joinW, joinReq, sess.ID)

	var pilot db.Pilot
	json.NewDecoder(joinW.Result().Body).Decode(&pilot)

	// Deactivate the pilot.
	deactReq := httptest.NewRequest(http.MethodDelete, "/api/pilots/1?session="+sess.ID, nil)
	deactReq.Header.Set("X-Pilot-ID", fmt.Sprint(pilot.ID))
	deactW := httptest.NewRecorder()
	s.HandleDeactivatePilot(deactW, deactReq, pilot.ID, sess.ID)

	if deactW.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", deactW.Result().StatusCode, http.StatusNoContent)
	}

	// Verify pilot is no longer active.
	getReq := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sess.ID, nil)
	getW := httptest.NewRecorder()
	s.HandleGetSession(getW, getReq, sess.ID)

	var result struct {
		Pilots []db.Pilot `json:"pilots"`
	}
	json.NewDecoder(getW.Result().Body).Decode(&result)

	if len(result.Pilots) != 0 {
		t.Errorf("got %d active pilots, want 0 after deactivation", len(result.Pilots))
	}
}

// --- Test helpers ---

func createTestSession(t *testing.T, srv *Server) *db.Session {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/sessions", nil)
	w := httptest.NewRecorder()
	srv.HandleCreateSession(w, req)
	var sess db.Session
	json.NewDecoder(w.Body).Decode(&sess)
	return &sess
}

func joinTestPilot(t *testing.T, srv *Server, sessionID, callsign, videoSystem string) {
	t.Helper()
	body := fmt.Sprintf(`{"callsign":"%s","video_system":"%s"}`, callsign, videoSystem)
	req := httptest.NewRequest("POST", "/api/sessions/"+sessionID+"/join", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.HandleJoinSession(w, req, sessionID)
	if w.Code != http.StatusCreated {
		t.Fatalf("join %s failed: %d %s", callsign, w.Code, w.Body.String())
	}
}

func getTestPilots(t *testing.T, srv *Server, sessionID string) []db.Pilot {
	t.Helper()
	pilots, err := srv.DB.GetActivePilots(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	return pilots
}

// --- Graduated escalation tests ---

func TestJoinSession_NoUnnecessaryDisplacement(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)
	joinTestPilot(t, srv, sess.ID, "PILOT1", "analog")

	// Get pilot 1's assignment.
	pilots := getTestPilots(t, srv, sess.ID)
	pilot1Freq := pilots[0].AssignedFreqMHz

	// Join second pilot.
	joinTestPilot(t, srv, sess.ID, "PILOT2", "analog")

	// Pilot 1 should not have moved.
	updated := getTestPilots(t, srv, sess.ID)
	for _, p := range updated {
		if p.Callsign == "PILOT1" && p.AssignedFreqMHz != pilot1Freq {
			t.Errorf("PILOT1 moved from %d to %d", pilot1Freq, p.AssignedFreqMHz)
		}
	}
}

func TestJoinSession_FirstPilotBecomesLeader(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)

	body := `{"callsign":"LEADER","video_system":"analog"}`
	req := httptest.NewRequest("POST", "/api/sessions/"+sess.ID+"/join", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.HandleJoinSession(w, req, sess.ID)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	leaderID, _ := srv.DB.GetLeader(sess.ID)
	var joined db.Pilot
	json.NewDecoder(w.Body).Decode(&joined)
	if leaderID != joined.ID {
		t.Errorf("expected leader=%d, got leader=%d", joined.ID, leaderID)
	}
}

func TestPreviewJoin_ReturnsLevel(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)
	joinTestPilot(t, srv, sess.ID, "PILOT1", "analog")

	body := `{"callsign":"PILOT2","video_system":"analog"}`
	req := httptest.NewRequest("POST", "/api/sessions/"+sess.ID+"/preview-join", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.HandlePreviewJoin(w, req, sess.ID)

	var resp struct {
		Level      int              `json:"level"`
		Assignment freq.Assignment  `json:"assignment"`
		Displaced  []DisplacedPilot `json:"displaced"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Level != 0 {
		t.Errorf("expected level 0, got %d", resp.Level)
	}
	if resp.Assignment.FreqMHz == 0 {
		t.Error("no assignment in preview")
	}
}

func TestDeactivatePilot_OthersStayPut(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)
	joinTestPilot(t, srv, sess.ID, "PILOT1", "analog")
	joinTestPilot(t, srv, sess.ID, "PILOT2", "analog")
	joinTestPilot(t, srv, sess.ID, "PILOT3", "analog")

	pilots := getTestPilots(t, srv, sess.ID)
	freqsBefore := make(map[string]int)
	var pilot2ID int
	for _, p := range pilots {
		freqsBefore[p.Callsign] = p.AssignedFreqMHz
		if p.Callsign == "PILOT2" {
			pilot2ID = p.ID
		}
	}

	// Deactivate pilot 2.
	req := httptest.NewRequest("DELETE", "/api/pilots/"+fmt.Sprint(pilot2ID)+"?session="+sess.ID, nil)
	req.Header.Set("X-Pilot-ID", fmt.Sprint(pilot2ID))
	w := httptest.NewRecorder()
	srv.HandleDeactivatePilot(w, req, pilot2ID, sess.ID)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	// Remaining pilots should be on same frequencies.
	remaining := getTestPilots(t, srv, sess.ID)
	for _, p := range remaining {
		if freqsBefore[p.Callsign] != p.AssignedFreqMHz {
			t.Errorf("%s moved from %d to %d after deactivation", p.Callsign, freqsBefore[p.Callsign], p.AssignedFreqMHz)
		}
	}
}

func TestRebalanceAll_LeaderOnly(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)
	joinTestPilot(t, srv, sess.ID, "LEADER", "analog")
	joinTestPilot(t, srv, sess.ID, "PILOT2", "analog")

	pilots := getTestPilots(t, srv, sess.ID)
	var leaderID, otherID int
	for _, p := range pilots {
		if p.Callsign == "LEADER" {
			leaderID = p.ID
		} else {
			otherID = p.ID
		}
	}

	// Non-leader should be rejected.
	req := httptest.NewRequest("POST", "/api/sessions/"+sess.ID+"/rebalance", nil)
	req.Header.Set("X-Pilot-ID", fmt.Sprint(otherID))
	w := httptest.NewRecorder()
	srv.HandleRebalanceAll(w, req, sess.ID)
	if w.Code != http.StatusForbidden {
		t.Errorf("non-leader rebalance: expected 403, got %d", w.Code)
	}

	// Leader should succeed.
	req = httptest.NewRequest("POST", "/api/sessions/"+sess.ID+"/rebalance", nil)
	req.Header.Set("X-Pilot-ID", fmt.Sprint(leaderID))
	w = httptest.NewRecorder()
	srv.HandleRebalanceAll(w, req, sess.ID)
	if w.Code != http.StatusOK {
		t.Errorf("leader rebalance: expected 200, got %d", w.Code)
	}
}

func TestTransferLeader(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)
	joinTestPilot(t, srv, sess.ID, "LEADER", "analog")
	joinTestPilot(t, srv, sess.ID, "PILOT2", "analog")

	pilots := getTestPilots(t, srv, sess.ID)
	var leaderID, otherID int
	for _, p := range pilots {
		if p.Callsign == "LEADER" {
			leaderID = p.ID
		} else {
			otherID = p.ID
		}
	}

	body := fmt.Sprintf(`{"pilot_id":%d}`, otherID)
	req := httptest.NewRequest("POST", "/api/sessions/"+sess.ID+"/transfer-leader", strings.NewReader(body))
	req.Header.Set("X-Pilot-ID", fmt.Sprint(leaderID))
	w := httptest.NewRecorder()
	srv.HandleTransferLeader(w, req, sess.ID)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	newLeader, _ := srv.DB.GetLeader(sess.ID)
	if newLeader != otherID {
		t.Errorf("expected new leader %d, got %d", otherID, newLeader)
	}
}

func TestTransferLeader_TargetNotInSession(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)
	joinTestPilot(t, srv, sess.ID, "LEADER", "analog")

	pilots := getTestPilots(t, srv, sess.ID)
	leaderID := pilots[0].ID

	// Try to transfer to a pilot ID that doesn't exist in the session.
	body := `{"pilot_id":99999}`
	req := httptest.NewRequest("POST", "/api/sessions/"+sess.ID+"/transfer-leader", strings.NewReader(body))
	req.Header.Set("X-Pilot-ID", fmt.Sprint(leaderID))
	w := httptest.NewRecorder()
	srv.HandleTransferLeader(w, req, sess.ID)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddPilot_LeaderOnly(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)
	joinTestPilot(t, srv, sess.ID, "LEADER", "analog")

	pilots := getTestPilots(t, srv, sess.ID)
	leaderID := pilots[0].ID

	body := `{"callsign":"PHANTOM","video_system":"analog"}`
	req := httptest.NewRequest("POST", "/api/sessions/"+sess.ID+"/add-pilot", strings.NewReader(body))
	req.Header.Set("X-Pilot-ID", fmt.Sprint(leaderID))
	w := httptest.NewRecorder()
	srv.HandleAddPilot(w, req, sess.ID)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	updated := getTestPilots(t, srv, sess.ID)
	var found bool
	for _, p := range updated {
		if p.Callsign == "PHANTOM" {
			found = true
		}
	}
	if !found {
		t.Error("phantom pilot not found")
	}
}

func TestRemovePilot_LeaderOnly(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)
	joinTestPilot(t, srv, sess.ID, "LEADER", "analog")
	joinTestPilot(t, srv, sess.ID, "PILOT2", "analog")

	pilots := getTestPilots(t, srv, sess.ID)
	var leaderID, otherID int
	for _, p := range pilots {
		if p.Callsign == "LEADER" {
			leaderID = p.ID
		} else {
			otherID = p.ID
		}
	}

	// Non-leader cannot remove others.
	req := httptest.NewRequest("DELETE", "/api/pilots/"+fmt.Sprint(leaderID)+"?session="+sess.ID, nil)
	req.Header.Set("X-Pilot-ID", fmt.Sprint(otherID))
	w := httptest.NewRecorder()
	srv.HandleDeactivatePilot(w, req, leaderID, sess.ID)
	if w.Code != http.StatusForbidden {
		t.Errorf("non-leader remove: expected 403, got %d", w.Code)
	}

	// Leader can remove others.
	req = httptest.NewRequest("DELETE", "/api/pilots/"+fmt.Sprint(otherID)+"?session="+sess.ID, nil)
	req.Header.Set("X-Pilot-ID", fmt.Sprint(leaderID))
	w = httptest.NewRecorder()
	srv.HandleDeactivatePilot(w, req, otherID, sess.ID)
	if w.Code != http.StatusNoContent {
		t.Errorf("leader remove: expected 204, got %d", w.Code)
	}
}

func TestDeactivatePilot_SelfRemovalAlwaysAllowed(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)
	joinTestPilot(t, srv, sess.ID, "LEADER", "analog")
	joinTestPilot(t, srv, sess.ID, "PILOT2", "analog")

	pilots := getTestPilots(t, srv, sess.ID)
	var otherID int
	for _, p := range pilots {
		if p.Callsign == "PILOT2" {
			otherID = p.ID
		}
	}

	req := httptest.NewRequest("DELETE", "/api/pilots/"+fmt.Sprint(otherID)+"?session="+sess.ID, nil)
	req.Header.Set("X-Pilot-ID", fmt.Sprint(otherID))
	w := httptest.NewRecorder()
	srv.HandleDeactivatePilot(w, req, otherID, sess.ID)
	if w.Code != http.StatusNoContent {
		t.Errorf("self-removal: expected 204, got %d", w.Code)
	}
}

func TestDeactivatePilot_NoHeader_Returns401(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)
	joinTestPilot(t, srv, sess.ID, "PILOT1", "analog")

	pilots := getTestPilots(t, srv, sess.ID)
	pilotID := pilots[0].ID

	// Send DELETE without X-Pilot-ID header.
	req := httptest.NewRequest("DELETE", "/api/pilots/"+fmt.Sprint(pilotID)+"?session="+sess.ID, nil)
	w := httptest.NewRecorder()
	srv.HandleDeactivatePilot(w, req, pilotID, sess.ID)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("no X-Pilot-ID header: expected 401, got %d", w.Code)
	}
}

func TestUpdateChannel_NoUnnecessaryDisplacement(t *testing.T) {
	srv := newTestServer(t)
	sess := createTestSession(t, srv)
	joinTestPilot(t, srv, sess.ID, "PILOT1", "analog")
	joinTestPilot(t, srv, sess.ID, "PILOT2", "analog")

	pilots := getTestPilots(t, srv, sess.ID)
	var pilot1Freq int
	var pilot2 db.Pilot
	for _, p := range pilots {
		if p.Callsign == "PILOT1" {
			pilot1Freq = p.AssignedFreqMHz
		}
		if p.Callsign == "PILOT2" {
			pilot2 = p
		}
	}

	// Change pilot 2's channel preference (clear preference = auto-assign).
	body := `{"preferred_frequency_mhz":0}`
	req := httptest.NewRequest("PUT", "/api/pilots/"+fmt.Sprint(pilot2.ID)+"/channel?session="+sess.ID, strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.HandleUpdatePilotChannel(w, req, pilot2.ID, sess.ID)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	// Pilot 1 should not have moved.
	updated := getTestPilots(t, srv, sess.ID)
	for _, p := range updated {
		if p.Callsign == "PILOT1" && p.AssignedFreqMHz != pilot1Freq {
			t.Errorf("PILOT1 moved from %d to %d", pilot1Freq, p.AssignedFreqMHz)
		}
	}
}

func TestGetSession_RebalanceRecommended(t *testing.T) {
	s := newTestServer(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, createReq)
	var sess struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Add a DJI O3 pilot and an analog pilot.
	join1 := JoinRequest{Callsign: "DJI1", VideoSystem: "dji_o3", BandwidthMHz: 40}
	body1, _ := json.Marshal(join1)
	s.HandleJoinSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body1)), sess.ID)

	join2 := JoinRequest{Callsign: "ANALOG1", VideoSystem: "analog"}
	body2, _ := json.Marshal(join2)
	s.HandleJoinSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body2)), sess.ID)

	// Force conflicting placement via DB (not buddy-grouped).
	pilots, _ := s.DB.GetActivePilots(sess.ID)
	for _, p := range pilots {
		if p.Callsign == "DJI1" {
			s.DB.UpdatePilotAssignment(sess.ID, p.ID, "O3-CH1", 5795, 0)
		}
		if p.Callsign == "ANALOG1" {
			s.DB.UpdatePilotAssignment(sess.ID, p.ID, "R4", 5769, 0)
		}
	}

	// GET session should include rebalance_recommended = true.
	getReq := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sess.ID, nil)
	getW := httptest.NewRecorder()
	s.HandleGetSession(getW, getReq, sess.ID)

	var resp struct {
		RebalanceRecommended bool `json:"rebalance_recommended"`
	}
	json.NewDecoder(getW.Result().Body).Decode(&resp)

	if !resp.RebalanceRecommended {
		t.Error("expected rebalance_recommended = true when non-buddy pilots have frequency conflicts")
	}
}

func TestGetSession_RebalanceRecommended_FalseWhenClean(t *testing.T) {
	s := newTestServer(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, createReq)
	var sess struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Add two analog pilots — plenty of channels, no conflicts.
	join1 := JoinRequest{Callsign: "PILOT1", VideoSystem: "analog"}
	body1, _ := json.Marshal(join1)
	s.HandleJoinSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body1)), sess.ID)

	join2 := JoinRequest{Callsign: "PILOT2", VideoSystem: "analog"}
	body2, _ := json.Marshal(join2)
	s.HandleJoinSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body2)), sess.ID)

	getReq := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sess.ID, nil)
	getW := httptest.NewRecorder()
	s.HandleGetSession(getW, getReq, sess.ID)

	var resp struct {
		RebalanceRecommended bool `json:"rebalance_recommended"`
	}
	json.NewDecoder(getW.Result().Body).Decode(&resp)

	if resp.RebalanceRecommended {
		t.Error("expected rebalance_recommended = false when no conflicts exist")
	}
}

func TestJoinSession_Spotter(t *testing.T) {
	s := newTestServer(t)

	// Create a session.
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, httptest.NewRequest(http.MethodPost, "/api/sessions", nil))
	var sess struct{ ID string `json:"id"` }
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Join as spotter.
	join := JoinRequest{Callsign: "OBSERVER", VideoSystem: "spotter"}
	body, _ := json.Marshal(join)
	joinW := httptest.NewRecorder()
	s.HandleJoinSession(joinW, httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body)), sess.ID)

	if joinW.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", joinW.Result().StatusCode, http.StatusCreated)
	}

	var pilot db.Pilot
	json.NewDecoder(joinW.Result().Body).Decode(&pilot)

	if pilot.AssignedFreqMHz != 0 {
		t.Errorf("spotter should have no frequency, got %d", pilot.AssignedFreqMHz)
	}
	if pilot.AssignedChannel != "" {
		t.Errorf("spotter should have no channel, got %q", pilot.AssignedChannel)
	}
}

func TestJoinSession_SpotterBecomesLeader(t *testing.T) {
	s := newTestServer(t)

	// Create a session.
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, httptest.NewRequest(http.MethodPost, "/api/sessions", nil))
	var sess struct{ ID string `json:"id"` }
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Join as spotter — first pilot in session.
	join := JoinRequest{Callsign: "LEAD_SPOTTER", VideoSystem: "spotter"}
	body, _ := json.Marshal(join)
	joinW := httptest.NewRecorder()
	s.HandleJoinSession(joinW, httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body)), sess.ID)

	if joinW.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", joinW.Result().StatusCode, http.StatusCreated)
	}

	var pilot db.Pilot
	json.NewDecoder(joinW.Result().Body).Decode(&pilot)

	// The spotter should be leader since they joined first.
	leaderID, err := s.DB.GetLeader(sess.ID)
	if err != nil {
		t.Fatalf("GetLeader error: %v", err)
	}
	if leaderID != pilot.ID {
		t.Errorf("spotter should be leader: leaderID = %d, pilot.ID = %d", leaderID, pilot.ID)
	}
}

func TestJoinSession_SpotterDoesNotAffectOthers(t *testing.T) {
	s := newTestServer(t)

	// Create a session.
	createW := httptest.NewRecorder()
	s.HandleCreateSession(createW, httptest.NewRequest(http.MethodPost, "/api/sessions", nil))
	var sess struct{ ID string `json:"id"` }
	json.NewDecoder(createW.Result().Body).Decode(&sess)

	// Join an analog pilot first.
	join1 := JoinRequest{Callsign: "ANALOG1", VideoSystem: "analog"}
	body1, _ := json.Marshal(join1)
	joinW1 := httptest.NewRecorder()
	s.HandleJoinSession(joinW1, httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body1)), sess.ID)

	var pilot1 db.Pilot
	json.NewDecoder(joinW1.Result().Body).Decode(&pilot1)
	origFreq := pilot1.AssignedFreqMHz
	origChan := pilot1.AssignedChannel

	if origFreq == 0 {
		t.Fatal("analog pilot should have a frequency assigned")
	}

	// Now join a spotter.
	join2 := JoinRequest{Callsign: "SPOTTER1", VideoSystem: "spotter"}
	body2, _ := json.Marshal(join2)
	joinW2 := httptest.NewRecorder()
	s.HandleJoinSession(joinW2, httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body2)), sess.ID)

	if joinW2.Result().StatusCode != http.StatusCreated {
		t.Fatalf("spotter join status = %d, want %d", joinW2.Result().StatusCode, http.StatusCreated)
	}

	// Re-fetch analog pilot and verify freq unchanged.
	pilots, err := s.DB.GetActivePilots(sess.ID)
	if err != nil {
		t.Fatalf("GetActivePilots error: %v", err)
	}

	for _, p := range pilots {
		if p.Callsign == "ANALOG1" {
			if p.AssignedFreqMHz != origFreq {
				t.Errorf("analog pilot freq changed: was %d, now %d", origFreq, p.AssignedFreqMHz)
			}
			if p.AssignedChannel != origChan {
				t.Errorf("analog pilot channel changed: was %q, now %q", origChan, p.AssignedChannel)
			}
			return
		}
	}
	t.Error("analog pilot not found in active pilots")
}

// TestJoinSession_FixedChannelIncompatible verifies that a pilot whose
// equipment can't tune any of the session's fixed channels is refused at both
// the preview and commit endpoints. Prevents silently assigning a pilot to a
// frequency their radio can't actually use.
func TestJoinSession_FixedChannelIncompatible(t *testing.T) {
	s := newTestServer(t)

	// Create a session with race-band fixed channels directly in the DB
	// (handlers only set power ceiling; fixed channels are set via session
	// update flow, but the DB layer accepts them directly).
	fixedJSON := `[{"name":"R1","freq":5658},{"name":"R3","freq":5732},{"name":"R6","freq":5843},{"name":"R8","freq":5917}]`
	sess, err := s.DB.CreateSession(0, fixedJSON)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// DJI V1 FCC has no channels at 5658 / 5732 / 5843 / 5917 — incompatible.
	joinBody := JoinRequest{
		Callsign:    "V1PILOT",
		VideoSystem: "dji_v1",
		FCCUnlocked: true,
	}
	body, _ := json.Marshal(joinBody)

	// Preview should return incompatible (HTTP 200, no level/assignment).
	previewReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/preview-join", bytes.NewReader(body))
	previewW := httptest.NewRecorder()
	s.HandlePreviewJoin(previewW, previewReq, sess.ID)

	if previewW.Result().StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d, want %d", previewW.Result().StatusCode, http.StatusOK)
	}
	var preview PreviewResponse
	if err := json.NewDecoder(previewW.Result().Body).Decode(&preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.Incompatible == nil {
		t.Fatal("preview.Incompatible should be set for V1 FCC pilot on race-band fixed session")
	}
	if preview.Incompatible.Reason != "no_channel_match" {
		t.Errorf("Incompatible.Reason = %q, want %q", preview.Incompatible.Reason, "no_channel_match")
	}
	if len(preview.Incompatible.FixedFreqs) != 4 {
		t.Errorf("Incompatible.FixedFreqs len = %d, want 4", len(preview.Incompatible.FixedFreqs))
	}

	// Commit should refuse with 409.
	body2, _ := json.Marshal(joinBody)
	joinReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body2))
	joinW := httptest.NewRecorder()
	s.HandleJoinSession(joinW, joinReq, sess.ID)

	if joinW.Result().StatusCode != http.StatusConflict {
		t.Errorf("join status = %d, want %d", joinW.Result().StatusCode, http.StatusConflict)
	}

	// Pilot should NOT have been added to the DB.
	pilots, err := s.DB.GetActivePilots(sess.ID)
	if err != nil {
		t.Fatalf("GetActivePilots: %v", err)
	}
	if len(pilots) != 0 {
		t.Errorf("GetActivePilots returned %d pilots; incompatible join should not have added any", len(pilots))
	}

	// Sanity: a compatible pilot (analog Race Band) succeeds on the same session.
	compatBody := JoinRequest{
		Callsign:    "OKPILOT",
		VideoSystem: "analog",
		AnalogBands: []string{"R"},
	}
	body3, _ := json.Marshal(compatBody)
	joinReq2 := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body3))
	joinW2 := httptest.NewRecorder()
	s.HandleJoinSession(joinW2, joinReq2, sess.ID)
	if joinW2.Result().StatusCode != http.StatusCreated {
		t.Errorf("compatible join status = %d, want %d", joinW2.Result().StatusCode, http.StatusCreated)
	}
}

// TestAddPilot_FixedChannelIncompatible verifies the leader's manual add-pilot
// flow also refuses incompatible equipment with HTTP 409 no_channel_match.
func TestAddPilot_FixedChannelIncompatible(t *testing.T) {
	s := newTestServer(t)

	fixedJSON := `[{"name":"R1","freq":5658},{"name":"R3","freq":5732},{"name":"R6","freq":5843},{"name":"R8","freq":5917}]`
	sess, err := s.DB.CreateSession(0, fixedJSON)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Seat a leader first (must be compatible).
	leaderBody := JoinRequest{Callsign: "LEADER", VideoSystem: "analog", AnalogBands: []string{"R"}}
	lb, _ := json.Marshal(leaderBody)
	lReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(lb))
	lW := httptest.NewRecorder()
	s.HandleJoinSession(lW, lReq, sess.ID)
	if lW.Result().StatusCode != http.StatusCreated {
		t.Fatalf("leader join: status %d", lW.Result().StatusCode)
	}
	pilots := getTestPilots(t, s, sess.ID)
	leaderID := pilots[0].ID

	// Leader tries to add an incompatible pilot (DJI V1 FCC, no race-band channels).
	incompatBody := JoinRequest{Callsign: "V1GUY", VideoSystem: "dji_v1", FCCUnlocked: true}
	ib, _ := json.Marshal(incompatBody)
	aReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/add-pilot", bytes.NewReader(ib))
	aReq.Header.Set("X-Pilot-ID", fmt.Sprint(leaderID))
	aW := httptest.NewRecorder()
	s.HandleAddPilot(aW, aReq, sess.ID)

	if aW.Result().StatusCode != http.StatusConflict {
		t.Errorf("add-pilot status = %d, want %d", aW.Result().StatusCode, http.StatusConflict)
	}
	if !strings.Contains(aW.Body.String(), "no_channel_match") {
		t.Errorf("add-pilot body = %q, want to contain %q", aW.Body.String(), "no_channel_match")
	}

	// Confirm the incompatible pilot was NOT added.
	after := getTestPilots(t, s, sess.ID)
	for _, p := range after {
		if p.Callsign == "V1GUY" {
			t.Errorf("incompatible pilot V1GUY should not have been added")
		}
	}
}

// TestUpdatePilotVideoSystem_FixedChannelIncompatible verifies that a pilot
// already in a fixed-channel session can't switch to an incompatible video
// system. The DB must not be mutated on refusal.
func TestUpdatePilotVideoSystem_FixedChannelIncompatible(t *testing.T) {
	s := newTestServer(t)

	fixedJSON := `[{"name":"R1","freq":5658},{"name":"R3","freq":5732},{"name":"R6","freq":5843},{"name":"R8","freq":5917}]`
	sess, err := s.DB.CreateSession(0, fixedJSON)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Join as compatible analog Race Band pilot.
	joinBody := JoinRequest{Callsign: "SWITCHY", VideoSystem: "analog", AnalogBands: []string{"R"}}
	jb, _ := json.Marshal(joinBody)
	jReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(jb))
	jW := httptest.NewRecorder()
	s.HandleJoinSession(jW, jReq, sess.ID)
	if jW.Result().StatusCode != http.StatusCreated {
		t.Fatalf("initial join: status %d", jW.Result().StatusCode)
	}
	pilots := getTestPilots(t, s, sess.ID)
	pilotID := pilots[0].ID
	origSystem := pilots[0].VideoSystem

	// Try to switch to incompatible DJI V1 FCC.
	updateBody := UpdateVideoSystemRequest{VideoSystem: "dji_v1", FCCUnlocked: true}
	ub, _ := json.Marshal(updateBody)
	uReq := httptest.NewRequest(http.MethodPut, "/api/pilots/"+fmt.Sprint(pilotID)+"/video-system?session="+sess.ID, bytes.NewReader(ub))
	uW := httptest.NewRecorder()
	s.HandleUpdatePilotVideoSystem(uW, uReq, pilotID, sess.ID)

	if uW.Result().StatusCode != http.StatusConflict {
		t.Errorf("update video-system status = %d, want %d", uW.Result().StatusCode, http.StatusConflict)
	}

	// Confirm DB was NOT mutated — video system must still be analog.
	after := getTestPilots(t, s, sess.ID)
	if len(after) != 1 {
		t.Fatalf("expected 1 pilot, got %d", len(after))
	}
	if after[0].VideoSystem != origSystem {
		t.Errorf("video_system = %q, want %q (DB should not mutate on refusal)", after[0].VideoSystem, origSystem)
	}
}

// TestUpdateFixedChannels_Success verifies the leader can switch the session's
// fixed channel set and everyone gets reoptimized against the new set.
func TestUpdateFixedChannels_Success(t *testing.T) {
	s := newTestServer(t)

	// Start with a 4-channel race-band fixed set.
	startingSet := `[{"name":"R1","freq":5658},{"name":"R3","freq":5732},{"name":"R6","freq":5843},{"name":"R8","freq":5917}]`
	sess, err := s.DB.CreateSession(0, startingSet)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Seat a leader (compatible with both sets below).
	joinTestPilot(t, s, sess.ID, "LEADER", "analog")
	pilots := getTestPilots(t, s, sess.ID)
	leaderID := pilots[0].ID

	// Leader switches to a 2-channel MAX-SPREAD set (still race-band).
	newSet := `[{"name":"R1","freq":5658},{"name":"R8","freq":5917}]`
	body, _ := json.Marshal(UpdateFixedChannelsRequest{FixedChannels: newSet})
	req := httptest.NewRequest(http.MethodPut, "/api/sessions/"+sess.ID+"/fixed-channels", bytes.NewReader(body))
	req.Header.Set("X-Pilot-ID", fmt.Sprint(leaderID))
	w := httptest.NewRecorder()
	s.HandleUpdateFixedChannels(w, req, sess.ID)

	if w.Result().StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d; body=%s", w.Result().StatusCode, http.StatusNoContent, w.Body.String())
	}

	// Session.fixed_channels should now be the new set.
	after, err := s.DB.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if after.FixedChannels != newSet {
		t.Errorf("fixed_channels = %q, want %q", after.FixedChannels, newSet)
	}

	// The leader pilot's assignment should still be on a freq in the new set.
	aftPilots := getTestPilots(t, s, sess.ID)
	if aftPilots[0].AssignedFreqMHz != 5658 && aftPilots[0].AssignedFreqMHz != 5917 {
		t.Errorf("reoptimized freq = %d, not in new fixed set {5658, 5917}", aftPilots[0].AssignedFreqMHz)
	}
}

// TestUpdateFixedChannels_Clear verifies the leader can exit race mode
// entirely by submitting an empty fixed_channels string.
func TestUpdateFixedChannels_Clear(t *testing.T) {
	s := newTestServer(t)

	startingSet := `[{"name":"R1","freq":5658},{"name":"R8","freq":5917}]`
	sess, err := s.DB.CreateSession(0, startingSet)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	joinTestPilot(t, s, sess.ID, "LEADER", "analog")
	pilots := getTestPilots(t, s, sess.ID)
	leaderID := pilots[0].ID

	body, _ := json.Marshal(UpdateFixedChannelsRequest{FixedChannels: ""})
	req := httptest.NewRequest(http.MethodPut, "/api/sessions/"+sess.ID+"/fixed-channels", bytes.NewReader(body))
	req.Header.Set("X-Pilot-ID", fmt.Sprint(leaderID))
	w := httptest.NewRecorder()
	s.HandleUpdateFixedChannels(w, req, sess.ID)

	if w.Result().StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Result().StatusCode, http.StatusNoContent)
	}

	after, err := s.DB.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if after.FixedChannels != "" {
		t.Errorf("fixed_channels = %q, want empty", after.FixedChannels)
	}
}

// TestUpdateFixedChannels_LeaderOnly ensures non-leaders are refused.
func TestUpdateFixedChannels_LeaderOnly(t *testing.T) {
	s := newTestServer(t)
	sess := createTestSession(t, s)
	joinTestPilot(t, s, sess.ID, "LEADER", "analog")
	joinTestPilot(t, s, sess.ID, "OTHER", "analog")

	pilots := getTestPilots(t, s, sess.ID)
	var otherID int
	for _, p := range pilots {
		if p.Callsign == "OTHER" {
			otherID = p.ID
		}
	}

	body, _ := json.Marshal(UpdateFixedChannelsRequest{FixedChannels: `[{"name":"R1","freq":5658},{"name":"R8","freq":5917}]`})
	req := httptest.NewRequest(http.MethodPut, "/api/sessions/"+sess.ID+"/fixed-channels", bytes.NewReader(body))
	req.Header.Set("X-Pilot-ID", fmt.Sprint(otherID))
	w := httptest.NewRecorder()
	s.HandleUpdateFixedChannels(w, req, sess.ID)

	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("non-leader status = %d, want %d", w.Result().StatusCode, http.StatusForbidden)
	}
}

// TestUpdateFixedChannels_IncompatiblePilots verifies the leader's change is
// refused with HTTP 409 and a pilot list when any pilot's pool can't tune the
// new set. DB must not mutate.
func TestUpdateFixedChannels_IncompatiblePilots(t *testing.T) {
	s := newTestServer(t)

	// Start WITHOUT fixed channels so an incompatible pilot can join.
	sess, err := s.DB.CreateSession(0, "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Leader on Race Band.
	joinTestPilot(t, s, sess.ID, "LEADER", "analog")

	// Incompatible pilot: DJI V1 FCC has no exact race-band matches.
	incompatBody := JoinRequest{Callsign: "V1GUY", VideoSystem: "dji_v1", FCCUnlocked: true}
	ib, _ := json.Marshal(incompatBody)
	j := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(ib))
	jw := httptest.NewRecorder()
	s.HandleJoinSession(jw, j, sess.ID)
	if jw.Result().StatusCode != http.StatusCreated {
		t.Fatalf("V1 pilot join (no fixed set): status %d", jw.Result().StatusCode)
	}

	pilots := getTestPilots(t, s, sess.ID)
	var leaderID int
	for _, p := range pilots {
		if p.Callsign == "LEADER" {
			leaderID = p.ID
		}
	}

	// Leader tries to switch to race-band fixed — V1 pilot can't tune any.
	newSet := `[{"name":"R1","freq":5658},{"name":"R3","freq":5732},{"name":"R6","freq":5843},{"name":"R8","freq":5917}]`
	body, _ := json.Marshal(UpdateFixedChannelsRequest{FixedChannels: newSet})
	req := httptest.NewRequest(http.MethodPut, "/api/sessions/"+sess.ID+"/fixed-channels", bytes.NewReader(body))
	req.Header.Set("X-Pilot-ID", fmt.Sprint(leaderID))
	w := httptest.NewRecorder()
	s.HandleUpdateFixedChannels(w, req, sess.ID)

	if w.Result().StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", w.Result().StatusCode, http.StatusConflict)
	}
	var resp IncompatiblePilotsResponse
	if err := json.NewDecoder(w.Result().Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Reason != "incompatible_pilots" {
		t.Errorf("reason = %q, want %q", resp.Reason, "incompatible_pilots")
	}
	if len(resp.Pilots) != 1 {
		t.Fatalf("incompatible pilots len = %d, want 1", len(resp.Pilots))
	}
	if resp.Pilots[0].Callsign != "V1GUY" {
		t.Errorf("incompatible callsign = %q, want V1GUY", resp.Pilots[0].Callsign)
	}

	// Confirm session.fixed_channels was NOT mutated.
	after, err := s.DB.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if after.FixedChannels != "" {
		t.Errorf("fixed_channels = %q, want empty (DB should not mutate on refusal)", after.FixedChannels)
	}
}

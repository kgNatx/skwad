package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/kyleg/skwad/db"
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
	return NewServer(database)
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

	// Join again with same callsign.
	body2, _ := json.Marshal(joinBody)
	joinReq2 := httptest.NewRequest(http.MethodPost, "/api/sessions/"+sess.ID+"/join", bytes.NewReader(body2))
	joinW2 := httptest.NewRecorder()
	s.HandleJoinSession(joinW2, joinReq2, sess.ID)

	if joinW2.Result().StatusCode != http.StatusConflict {
		t.Errorf("duplicate join: status = %d, want %d", joinW2.Result().StatusCode, http.StatusConflict)
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

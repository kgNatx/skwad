package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/kyleg/skwad/db"
	"github.com/kyleg/skwad/freq"
)

// Server holds dependencies for the HTTP handlers.
type Server struct {
	DB *db.DB
}

// NewServer creates a new Server with the given database.
func NewServer(database *db.DB) *Server {
	return &Server{DB: database}
}

// JoinRequest is the JSON body for joining a session.
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

// HandleCreateSession creates a new frequency-coordination session.
// POST /api/sessions
func (s *Server) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	sess, err := s.DB.CreateSession()
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		log.Printf("CreateSession error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sess)
}

// HandleGetSession returns a session and its active pilots.
// GET /api/sessions/{code}
func (s *Server) HandleGetSession(w http.ResponseWriter, r *http.Request, code string) {
	sess, err := s.DB.GetSession(code)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	pilots, err := s.DB.GetActivePilots(code)
	if err != nil {
		http.Error(w, "failed to get pilots", http.StatusInternalServerError)
		log.Printf("GetActivePilots error: %v", err)
		return
	}

	resp := struct {
		Session *db.Session `json:"session"`
		Pilots  []db.Pilot  `json:"pilots"`
	}{
		Session: sess,
		Pilots:  pilots,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleJoinSession adds a pilot to a session.
// POST /api/sessions/{code}/join
func (s *Server) HandleJoinSession(w http.ResponseWriter, r *http.Request, code string) {
	// Verify session exists.
	_, err := s.DB.GetSession(code)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	var req JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	// Validate required fields.
	if strings.TrimSpace(req.Callsign) == "" || strings.TrimSpace(req.VideoSystem) == "" {
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
		// Check for duplicate callsign (UNIQUE constraint violation).
		if strings.Contains(err.Error(), "UNIQUE") {
			http.Error(w, "callsign already in session", http.StatusConflict)
			return
		}
		http.Error(w, "failed to add pilot", http.StatusInternalServerError)
		log.Printf("AddPilot error: %v", err)
		return
	}

	// Reoptimize all pilots in the session.
	s.reoptimize(code)

	// Re-fetch the pilot to get the updated assignment.
	pilots, err := s.DB.GetActivePilots(code)
	if err == nil {
		for _, p := range pilots {
			if p.ID == added.ID {
				added = &p
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(added)
}

// HandleDeactivatePilot sets a pilot as inactive and reoptimizes.
// DELETE /api/pilots/{id}?session={code}
func (s *Server) HandleDeactivatePilot(w http.ResponseWriter, r *http.Request, pilotID int, sessionCode string) {
	if err := s.DB.DeactivatePilot(pilotID); err != nil {
		http.Error(w, "pilot not found", http.StatusNotFound)
		return
	}

	s.reoptimize(sessionCode)

	if err := s.DB.IncrementVersion(sessionCode); err != nil {
		log.Printf("IncrementVersion error: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandlePoll returns just the session version number.
// GET /api/sessions/{code}/poll
func (s *Server) HandlePoll(w http.ResponseWriter, r *http.Request, code string) {
	sess, err := s.DB.GetSession(code)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Version int `json:"version"`
	}{Version: sess.Version})
}

// reoptimize gets all active pilots for a session, runs the frequency
// optimizer, and updates each pilot's assignment in the database.
func (s *Server) reoptimize(sessionCode string) {
	pilots, err := s.DB.GetActivePilots(sessionCode)
	if err != nil {
		log.Printf("reoptimize: GetActivePilots error: %v", err)
		return
	}

	if len(pilots) == 0 {
		return
	}

	// Convert DB pilots to optimizer inputs.
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

	// Run the optimizer.
	assignments := freq.Optimize(inputs)

	// Update each pilot's assignment in the database.
	for _, a := range assignments {
		if err := s.DB.UpdatePilotAssignment(a.PilotID, a.Channel, a.FreqMHz, a.BuddyGroup); err != nil {
			log.Printf("reoptimize: UpdatePilotAssignment error for pilot %d: %v", a.PilotID, err)
		}
	}

	// Increment the session version so polling clients detect the change.
	if err := s.DB.IncrementVersion(sessionCode); err != nil {
		log.Printf("reoptimize: IncrementVersion error: %v", err)
	}
}

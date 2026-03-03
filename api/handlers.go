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
	Callsign      string `json:"callsign"`
	VideoSystem   string `json:"video_system"`
	FCCUnlocked   bool   `json:"fcc_unlocked"`
	Goggles       string `json:"goggles"`
	BandwidthMHz  int    `json:"bandwidth_mhz"`
	RaceMode      bool   `json:"race_mode"`
	ChannelLocked bool   `json:"channel_locked"`
	LockedFreqMHz int    `json:"locked_frequency_mhz"`
}

// PilotConflict describes a conflict between the current pilot and another.
type PilotConflict struct {
	OtherPilotID int              `json:"other_pilot_id"`
	OtherCallsign string          `json:"other_callsign"`
	Level        freq.ConflictLevel `json:"level"`
	SeparationMHz int              `json:"separation_mhz"`
	RequiredMHz  int              `json:"required_mhz"`
}

// PilotWithConflicts wraps a Pilot with optional conflict information.
type PilotWithConflicts struct {
	db.Pilot
	Conflicts []PilotConflict `json:"conflicts,omitempty"`
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

// HandleGetSession returns a session and its active pilots with conflict info.
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

	// Build assignments for conflict detection.
	assignments := buildAssignments(pilots)
	conflicts := freq.DetectConflicts(assignments)

	// Build per-pilot conflict map.
	pilotCallsigns := make(map[int]string, len(pilots))
	for _, p := range pilots {
		pilotCallsigns[p.ID] = p.Callsign
	}

	pilotConflicts := make(map[int][]PilotConflict)
	for _, c := range conflicts {
		pilotConflicts[c.PilotA] = append(pilotConflicts[c.PilotA], PilotConflict{
			OtherPilotID:  c.PilotB,
			OtherCallsign: pilotCallsigns[c.PilotB],
			Level:         c.Level,
			SeparationMHz: c.SeparationMHz,
			RequiredMHz:   c.RequiredMHz,
		})
		pilotConflicts[c.PilotB] = append(pilotConflicts[c.PilotB], PilotConflict{
			OtherPilotID:  c.PilotA,
			OtherCallsign: pilotCallsigns[c.PilotA],
			Level:         c.Level,
			SeparationMHz: c.SeparationMHz,
			RequiredMHz:   c.RequiredMHz,
		})
	}

	// Wrap pilots with conflict info.
	pilotsWithConflicts := make([]PilotWithConflicts, len(pilots))
	for i, p := range pilots {
		pilotsWithConflicts[i] = PilotWithConflicts{
			Pilot:     p,
			Conflicts: pilotConflicts[p.ID],
		}
	}

	resp := struct {
		Session *db.Session         `json:"session"`
		Pilots  []PilotWithConflicts `json:"pilots"`
	}{
		Session: sess,
		Pilots:  pilotsWithConflicts,
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

// DisplacedPilot describes a pilot who would be moved by a new join.
type DisplacedPilot struct {
	PilotID    int    `json:"pilot_id"`
	Callsign   string `json:"callsign"`
	OldChannel string `json:"old_channel"`
	OldFreqMHz int    `json:"old_freq_mhz"`
	NewChannel string `json:"new_channel"`
	NewFreqMHz int    `json:"new_freq_mhz"`
}

// HandlePreviewJoin dry-runs the optimizer with a hypothetical new pilot and
// returns which existing pilots would be displaced. Nothing is committed.
// POST /api/sessions/{code}/preview-join
func (s *Server) HandlePreviewJoin(w http.ResponseWriter, r *http.Request, code string) {
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

	if strings.TrimSpace(req.Callsign) == "" || strings.TrimSpace(req.VideoSystem) == "" {
		http.Error(w, "callsign and video_system are required", http.StatusBadRequest)
		return
	}

	pilots, err := s.DB.GetActivePilots(code)
	if err != nil {
		http.Error(w, "failed to get pilots", http.StatusInternalServerError)
		log.Printf("PreviewJoin GetActivePilots error: %v", err)
		return
	}

	// Check for duplicate callsign among active pilots.
	for _, p := range pilots {
		if p.Callsign == req.Callsign {
			http.Error(w, "callsign already in session", http.StatusConflict)
			return
		}
	}

	// Build optimizer inputs from existing pilots.
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

	// Add the hypothetical new pilot with a temporary ID.
	tempID := -1
	inputs = append(inputs, freq.PilotInput{
		ID:            tempID,
		VideoSystem:   req.VideoSystem,
		FCCUnlocked:   req.FCCUnlocked,
		BandwidthMHz:  req.BandwidthMHz,
		RaceMode:      req.RaceMode,
		Goggles:       req.Goggles,
		ChannelLocked: req.ChannelLocked,
		LockedFreqMHz: req.LockedFreqMHz,
	})

	// Run the optimizer.
	assignments := freq.Optimize(inputs)

	// Compare existing pilots' old vs new assignments.
	oldAssignments := make(map[int][2]interface{}) // id -> [channel, freq]
	for _, p := range pilots {
		oldAssignments[p.ID] = [2]interface{}{p.AssignedChannel, p.AssignedFreqMHz}
	}

	var displaced []DisplacedPilot
	for _, a := range assignments {
		if a.PilotID == tempID {
			continue
		}
		old, ok := oldAssignments[a.PilotID]
		if !ok {
			continue
		}
		oldCh := old[0].(string)
		oldFreq := old[1].(int)
		if oldCh != "" && (a.Channel != oldCh || a.FreqMHz != oldFreq) {
			// Find callsign for this pilot.
			var callsign string
			for _, p := range pilots {
				if p.ID == a.PilotID {
					callsign = p.Callsign
					break
				}
			}
			displaced = append(displaced, DisplacedPilot{
				PilotID:    a.PilotID,
				Callsign:   callsign,
				OldChannel: oldCh,
				OldFreqMHz: oldFreq,
				NewChannel: a.Channel,
				NewFreqMHz: a.FreqMHz,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Displaced []DisplacedPilot `json:"displaced"`
	}{Displaced: displaced})
}

// UpdateChannelRequest is the JSON body for changing a pilot's channel preference.
type UpdateChannelRequest struct {
	ChannelLocked bool `json:"channel_locked"`
	LockedFreqMHz int  `json:"locked_frequency_mhz"`
}

// HandleUpdatePilotChannel updates a pilot's channel preference and reoptimizes.
// PUT /api/pilots/{id}/channel?session={code}
func (s *Server) HandleUpdatePilotChannel(w http.ResponseWriter, r *http.Request, pilotID int, sessionCode string) {
	var req UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := s.DB.UpdatePilotPreferences(pilotID, req.ChannelLocked, req.LockedFreqMHz); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "pilot not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update pilot", http.StatusInternalServerError)
		log.Printf("UpdatePilotPreferences error: %v", err)
		return
	}

	s.reoptimize(sessionCode)
	w.WriteHeader(http.StatusNoContent)
}

// UpdateCallsignRequest is the JSON body for changing a pilot's callsign.
type UpdateCallsignRequest struct {
	Callsign string `json:"callsign"`
}

// HandleUpdatePilotCallsign changes a pilot's callsign.
// PUT /api/pilots/{id}/callsign?session={code}
func (s *Server) HandleUpdatePilotCallsign(w http.ResponseWriter, r *http.Request, pilotID int, sessionCode string) {
	var req UpdateCallsignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	callsign := strings.TrimSpace(req.Callsign)
	if callsign == "" {
		http.Error(w, "callsign is required", http.StatusBadRequest)
		return
	}

	if err := s.DB.UpdatePilotCallsign(pilotID, callsign); err != nil {
		if strings.Contains(err.Error(), "already in session") {
			http.Error(w, "callsign already in session", http.StatusConflict)
			return
		}
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "pilot not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update callsign", http.StatusInternalServerError)
		log.Printf("UpdatePilotCallsign error: %v", err)
		return
	}

	// Increment version so other clients see the change.
	if err := s.DB.IncrementVersion(sessionCode); err != nil {
		log.Printf("IncrementVersion error: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
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

// buildAssignments converts DB pilots to freq.Assignment structs for conflict detection.
func buildAssignments(pilots []db.Pilot) []freq.Assignment {
	assignments := make([]freq.Assignment, len(pilots))
	for i, p := range pilots {
		assignments[i] = freq.Assignment{
			PilotID:      p.ID,
			Channel:      p.AssignedChannel,
			FreqMHz:      p.AssignedFreqMHz,
			BandwidthMHz: freq.OccupiedBandwidth(p.VideoSystem, p.BandwidthMHz),
			BuddyGroup:   p.BuddyGroup,
		}
	}
	return assignments
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

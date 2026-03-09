package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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

// requireLeader checks the X-Pilot-ID header matches the session leader.
func (s *Server) requireLeader(w http.ResponseWriter, r *http.Request, sessionCode string) (int, bool) {
	idStr := r.Header.Get("X-Pilot-ID")
	if idStr == "" {
		http.Error(w, "X-Pilot-ID header required", http.StatusUnauthorized)
		return 0, false
	}
	pilotID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid X-Pilot-ID", http.StatusBadRequest)
		return 0, false
	}
	leaderID, err := s.DB.GetLeader(sessionCode)
	if err != nil || leaderID != pilotID {
		http.Error(w, "leader access required", http.StatusForbidden)
		return 0, false
	}
	return pilotID, true
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
		// If callsign is still active (e.g., "change video system" flow where
		// the frontend's delete failed or raced), deactivate the old pilot and retry.
		if strings.Contains(err.Error(), "UNIQUE") {
			existingID, findErr := s.DB.FindActivePilotByCallsign(code, req.Callsign)
			if findErr != nil || existingID == 0 {
				http.Error(w, "callsign already in session", http.StatusConflict)
				return
			}
			if deactErr := s.DB.DeactivatePilot(existingID); deactErr != nil {
				http.Error(w, "callsign already in session", http.StatusConflict)
				return
			}
			log.Printf("Deactivated stale pilot %d (%s) for rejoin", existingID, req.Callsign)
			added, err = s.DB.AddPilot(code, pilot)
			if err != nil {
				http.Error(w, "failed to add pilot", http.StatusInternalServerError)
				log.Printf("AddPilot retry error: %v", err)
				return
			}
		} else {
			http.Error(w, "failed to add pilot", http.StatusInternalServerError)
			log.Printf("AddPilot error: %v", err)
			return
		}
	}

	// Graduated escalation: minimize displacement of existing pilots.
	pilots, err := s.DB.GetActivePilots(code)
	if err != nil {
		http.Error(w, "failed to get pilots", http.StatusInternalServerError)
		log.Printf("HandleJoinSession GetActivePilots error: %v", err)
		return
	}

	inputs := buildPilotInputs(pilots)

	// Separate the newly added pilot from existing pilots.
	var newPilotInput freq.PilotInput
	var existingInputs []freq.PilotInput
	for _, inp := range inputs {
		if inp.ID == added.ID {
			newPilotInput = inp
		} else {
			existingInputs = append(existingInputs, inp)
		}
	}

	result := freq.FindMinimalDisplacement(existingInputs, newPilotInput)

	// Apply all assignments from the result.
	for _, a := range result.Assignments {
		if err := s.DB.UpdatePilotAssignment(a.PilotID, a.Channel, a.FreqMHz, a.BuddyGroup); err != nil {
			log.Printf("HandleJoinSession: UpdatePilotAssignment error for pilot %d: %v", a.PilotID, err)
		}
	}

	if err := s.DB.IncrementVersion(code); err != nil {
		log.Printf("IncrementVersion error: %v", err)
	}

	// First pilot becomes leader.
	leaderID, _ := s.DB.GetLeader(code)
	if leaderID == 0 {
		s.DB.SetLeader(code, added.ID)
	}

	// Re-fetch the pilot to get the updated assignment.
	pilots, err = s.DB.GetActivePilots(code)
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

// PreviewResponse is the JSON response shape for preview-join and preview-channel.
type PreviewResponse struct {
	Level           int                   `json:"level"`
	Assignment      freq.Assignment       `json:"assignment"`
	Displaced       []DisplacedPilot      `json:"displaced"`
	BuddySuggestion *freq.BuddySuggestion `json:"buddy_suggestion"`
}

// HandlePreviewJoin dry-runs graduated escalation with a hypothetical new pilot
// and returns the escalation level, new pilot's assignment, and displaced pilots.
// Nothing is committed.
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

	// If the callsign already exists as an active pilot, exclude that pilot
	// from the preview (they'll be reactivated with new settings on actual join).
	// This handles the "change video system" flow where a pilot leaves and rejoins.
	filteredPilots := make([]db.Pilot, 0, len(pilots))
	for _, p := range pilots {
		if p.Callsign != req.Callsign {
			filteredPilots = append(filteredPilots, p)
		}
	}
	pilots = filteredPilots

	existingInputs := buildPilotInputs(pilots)

	// Build the hypothetical new pilot with a temporary ID.
	tempID := -1
	newPilotInput := freq.PilotInput{
		ID:            tempID,
		VideoSystem:   req.VideoSystem,
		FCCUnlocked:   req.FCCUnlocked,
		BandwidthMHz:  req.BandwidthMHz,
		RaceMode:      req.RaceMode,
		Goggles:       req.Goggles,
		ChannelLocked: req.ChannelLocked,
		LockedFreqMHz: req.LockedFreqMHz,
	}

	result := freq.FindMinimalDisplacement(existingInputs, newPilotInput)

	// Find the new pilot's assignment and build displaced list.
	var newAssignment freq.Assignment
	var displaced []DisplacedPilot
	pilotCallsigns := make(map[int]string, len(pilots))
	pilotOldFreqs := make(map[int]int, len(pilots))
	pilotOldChannels := make(map[int]string, len(pilots))
	for _, p := range pilots {
		pilotCallsigns[p.ID] = p.Callsign
		pilotOldFreqs[p.ID] = p.AssignedFreqMHz
		pilotOldChannels[p.ID] = p.AssignedChannel
	}

	for _, a := range result.Assignments {
		if a.PilotID == tempID {
			newAssignment = a
			continue
		}
		oldCh := pilotOldChannels[a.PilotID]
		oldFreq := pilotOldFreqs[a.PilotID]
		if oldCh != "" && (a.Channel != oldCh || a.FreqMHz != oldFreq) {
			displaced = append(displaced, DisplacedPilot{
				PilotID:    a.PilotID,
				Callsign:   pilotCallsigns[a.PilotID],
				OldChannel: oldCh,
				OldFreqMHz: oldFreq,
				NewChannel: a.Channel,
				NewFreqMHz: a.FreqMHz,
			})
		}
	}

	// Enrich buddy suggestion with callsign from pilot data.
	if result.BuddySuggestion != nil {
		result.BuddySuggestion.Callsign = pilotCallsigns[result.BuddySuggestion.PilotID]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PreviewResponse{
		Level:           result.Level,
		Assignment:      newAssignment,
		Displaced:       displaced,
		BuddySuggestion: result.BuddySuggestion,
	})
}

// UpdateChannelRequest is the JSON body for changing a pilot's channel preference.
type UpdateChannelRequest struct {
	ChannelLocked bool `json:"channel_locked"`
	LockedFreqMHz int  `json:"locked_frequency_mhz"`
}

// HandlePreviewChannelChange dry-runs graduated escalation for a pilot changing
// their channel preference. Returns the escalation level, assignment, and displaced pilots.
// POST /api/pilots/{id}/preview-channel?session={code}
func (s *Server) HandlePreviewChannelChange(w http.ResponseWriter, r *http.Request, pilotID int, sessionCode string) {
	var req UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	pilots, err := s.DB.GetActivePilots(sessionCode)
	if err != nil {
		http.Error(w, "failed to get pilots", http.StatusInternalServerError)
		log.Printf("PreviewChannelChange GetActivePilots error: %v", err)
		return
	}

	inputs := buildPilotInputs(pilots)

	// Separate the changing pilot (with updated preferences) from the rest.
	var changingPilotInput freq.PilotInput
	var existingInputs []freq.PilotInput
	for _, inp := range inputs {
		if inp.ID == pilotID {
			changingPilotInput = inp
			changingPilotInput.ChannelLocked = req.ChannelLocked
			changingPilotInput.LockedFreqMHz = req.LockedFreqMHz
			// Clear prev assignment since preferences changed.
			changingPilotInput.PrevChannel = ""
			changingPilotInput.PrevFreqMHz = 0
		} else {
			existingInputs = append(existingInputs, inp)
		}
	}

	result := freq.FindMinimalDisplacement(existingInputs, changingPilotInput)

	// Find the changing pilot's assignment and build displaced list.
	var myAssignment freq.Assignment
	var displaced []DisplacedPilot
	pilotCallsigns := make(map[int]string, len(pilots))
	pilotOldFreqs := make(map[int]int, len(pilots))
	pilotOldChannels := make(map[int]string, len(pilots))
	for _, p := range pilots {
		pilotCallsigns[p.ID] = p.Callsign
		pilotOldFreqs[p.ID] = p.AssignedFreqMHz
		pilotOldChannels[p.ID] = p.AssignedChannel
	}

	for _, a := range result.Assignments {
		if a.PilotID == pilotID {
			myAssignment = a
			continue
		}
		oldCh := pilotOldChannels[a.PilotID]
		oldFreq := pilotOldFreqs[a.PilotID]
		if oldCh != "" && (a.Channel != oldCh || a.FreqMHz != oldFreq) {
			displaced = append(displaced, DisplacedPilot{
				PilotID:    a.PilotID,
				Callsign:   pilotCallsigns[a.PilotID],
				OldChannel: oldCh,
				OldFreqMHz: oldFreq,
				NewChannel: a.Channel,
				NewFreqMHz: a.FreqMHz,
			})
		}
	}

	// Enrich buddy suggestion with callsign from pilot data.
	if result.BuddySuggestion != nil {
		result.BuddySuggestion.Callsign = pilotCallsigns[result.BuddySuggestion.PilotID]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PreviewResponse{
		Level:           result.Level,
		Assignment:      myAssignment,
		Displaced:       displaced,
		BuddySuggestion: result.BuddySuggestion,
	})
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

	// Graduated escalation: treat the changing pilot as "new".
	pilots, err := s.DB.GetActivePilots(sessionCode)
	if err != nil {
		http.Error(w, "failed to get pilots", http.StatusInternalServerError)
		log.Printf("HandleUpdatePilotChannel GetActivePilots error: %v", err)
		return
	}

	inputs := buildPilotInputs(pilots)

	var changingPilotInput freq.PilotInput
	var existingInputs []freq.PilotInput
	for _, inp := range inputs {
		if inp.ID == pilotID {
			changingPilotInput = inp
			// Clear prev assignment since preferences changed.
			changingPilotInput.PrevChannel = ""
			changingPilotInput.PrevFreqMHz = 0
		} else {
			existingInputs = append(existingInputs, inp)
		}
	}

	result := freq.FindMinimalDisplacement(existingInputs, changingPilotInput)

	for _, a := range result.Assignments {
		if err := s.DB.UpdatePilotAssignment(a.PilotID, a.Channel, a.FreqMHz, a.BuddyGroup); err != nil {
			log.Printf("HandleUpdatePilotChannel: UpdatePilotAssignment error for pilot %d: %v", a.PilotID, err)
		}
	}

	if err := s.DB.IncrementVersion(sessionCode); err != nil {
		log.Printf("IncrementVersion error: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateVideoSystemRequest is the JSON body for changing a pilot's video system.
type UpdateVideoSystemRequest struct {
	VideoSystem  string `json:"video_system"`
	FCCUnlocked  bool   `json:"fcc_unlocked"`
	Goggles      string `json:"goggles"`
	BandwidthMHz int    `json:"bandwidth_mhz"`
	RaceMode     bool   `json:"race_mode"`
}

// HandleUpdatePilotVideoSystem changes a pilot's video system and reoptimizes.
// PUT /api/pilots/{id}/video-system?session={code}
func (s *Server) HandleUpdatePilotVideoSystem(w http.ResponseWriter, r *http.Request, pilotID int, sessionCode string) {
	var req UpdateVideoSystemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.VideoSystem) == "" {
		http.Error(w, "video_system is required", http.StatusBadRequest)
		return
	}

	if err := s.DB.UpdatePilotVideoSystem(pilotID, req.VideoSystem, req.FCCUnlocked, req.Goggles, req.BandwidthMHz, req.RaceMode); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "pilot not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update pilot", http.StatusInternalServerError)
		log.Printf("UpdatePilotVideoSystem error: %v", err)
		return
	}

	// Graduated escalation: treat the changing pilot as "new".
	pilots, err := s.DB.GetActivePilots(sessionCode)
	if err != nil {
		http.Error(w, "failed to get pilots", http.StatusInternalServerError)
		log.Printf("HandleUpdatePilotVideoSystem GetActivePilots error: %v", err)
		return
	}

	inputs := buildPilotInputs(pilots)

	var changingPilotInput freq.PilotInput
	var existingInputs []freq.PilotInput
	for _, inp := range inputs {
		if inp.ID == pilotID {
			changingPilotInput = inp
			// Clear prev assignment since video system changed.
			changingPilotInput.PrevChannel = ""
			changingPilotInput.PrevFreqMHz = 0
		} else {
			existingInputs = append(existingInputs, inp)
		}
	}

	result := freq.FindMinimalDisplacement(existingInputs, changingPilotInput)

	for _, a := range result.Assignments {
		if err := s.DB.UpdatePilotAssignment(a.PilotID, a.Channel, a.FreqMHz, a.BuddyGroup); err != nil {
			log.Printf("HandleUpdatePilotVideoSystem: UpdatePilotAssignment error for pilot %d: %v", a.PilotID, err)
		}
	}

	if err := s.DB.IncrementVersion(sessionCode); err != nil {
		log.Printf("IncrementVersion error: %v", err)
	}

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

// HandleDeactivatePilot sets a pilot as inactive.
// DELETE /api/pilots/{id}?session={code}
//
// Authorization:
//   - Self-removal (X-Pilot-ID == pilotID): always allowed
//   - Removing another pilot: requires leader
//   - No X-Pilot-ID header: allowed (backwards compatibility)
func (s *Server) HandleDeactivatePilot(w http.ResponseWriter, r *http.Request, pilotID int, sessionCode string) {
	idStr := r.Header.Get("X-Pilot-ID")
	if idStr != "" {
		requestingID, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "invalid X-Pilot-ID", http.StatusBadRequest)
			return
		}
		// If not self-removal, require leader.
		if requestingID != pilotID {
			leaderID, err := s.DB.GetLeader(sessionCode)
			if err != nil || leaderID != requestingID {
				http.Error(w, "leader access required", http.StatusForbidden)
				return
			}
		}
	}

	if err := s.DB.DeactivatePilot(pilotID); err != nil {
		http.Error(w, "pilot not found", http.StatusNotFound)
		return
	}

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

// buildPilotInputs converts DB pilots to freq.PilotInput structs for the optimizer.
func buildPilotInputs(pilots []db.Pilot) []freq.PilotInput {
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
	return inputs
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

// HandleRebalanceAll runs the full optimizer on all pilots (leader-only).
// Returns which pilots moved and which stayed.
// POST /api/sessions/{code}/rebalance
func (s *Server) HandleRebalanceAll(w http.ResponseWriter, r *http.Request, code string) {
	if _, ok := s.requireLeader(w, r, code); !ok {
		return
	}

	// Snapshot before assignments.
	pilots, err := s.DB.GetActivePilots(code)
	if err != nil {
		http.Error(w, "failed to get pilots", http.StatusInternalServerError)
		return
	}
	before := make(map[int][2]string) // id -> [channel, callsign]
	beforeFreq := make(map[int]int)
	for _, p := range pilots {
		before[p.ID] = [2]string{p.AssignedChannel, p.Callsign}
		beforeFreq[p.ID] = p.AssignedFreqMHz
	}

	s.reoptimize(code)

	// Snapshot after assignments.
	pilotsAfter, err := s.DB.GetActivePilots(code)
	if err != nil {
		// Rebalance happened but can't read result — return empty.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Moved []DisplacedPilot `json:"moved"`
		}{})
		return
	}

	var moved []DisplacedPilot
	for _, p := range pilotsAfter {
		old, ok := before[p.ID]
		if !ok {
			continue
		}
		oldFreq := beforeFreq[p.ID]
		if p.AssignedChannel != old[0] || p.AssignedFreqMHz != oldFreq {
			moved = append(moved, DisplacedPilot{
				PilotID:    p.ID,
				Callsign:   old[1],
				OldChannel: old[0],
				OldFreqMHz: oldFreq,
				NewChannel: p.AssignedChannel,
				NewFreqMHz: p.AssignedFreqMHz,
			})
		}
	}

	// Check for remaining danger conflicts with detail.
	assignments := buildAssignments(pilotsAfter)
	dangerConflicts := freq.DetectConflicts(assignments)

	// Build lookup maps.
	pilotMap := make(map[int]db.Pilot, len(pilotsAfter))
	for _, p := range pilotsAfter {
		pilotMap[p.ID] = p
	}

	type UnresolvedConflict struct {
		PilotA   string `json:"pilot_a"`
		PilotB   string `json:"pilot_b"`
		LockedBy string `json:"locked_by"` // which pilot is locked (or "" if neither)
		Reason   string `json:"reason"`    // human-readable explanation
	}
	var unresolved []UnresolvedConflict
	for _, c := range dangerConflicts {
		if c.Level != freq.ConflictDanger {
			continue
		}
		pA, pB := pilotMap[c.PilotA], pilotMap[c.PilotB]

		lockedBy := ""
		if pA.ChannelLocked && pB.ChannelLocked {
			lockedBy = pA.Callsign + " and " + pB.Callsign
		} else if pA.ChannelLocked {
			lockedBy = pA.Callsign
		} else if pB.ChannelLocked {
			lockedBy = pB.Callsign
		}

		reason := pA.Callsign + " (" + pA.AssignedChannel + ") and " +
			pB.Callsign + " (" + pB.AssignedChannel + ") are " +
			fmt.Sprintf("%d", c.SeparationMHz) + " MHz apart but need " +
			fmt.Sprintf("%d", c.RequiredMHz) + " MHz"
		if lockedBy != "" {
			reason += ". " + lockedBy + " is channel-locked"
		}

		unresolved = append(unresolved, UnresolvedConflict{
			PilotA:   pA.Callsign,
			PilotB:   pB.Callsign,
			LockedBy: lockedBy,
			Reason:   reason,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Moved      []DisplacedPilot   `json:"moved"`
		Unresolved []UnresolvedConflict `json:"unresolved"`
	}{Moved: moved, Unresolved: unresolved})
}

// HandleTransferLeader designates another pilot as session leader.
// POST /api/sessions/{code}/transfer-leader
func (s *Server) HandleTransferLeader(w http.ResponseWriter, r *http.Request, code string) {
	if _, ok := s.requireLeader(w, r, code); !ok {
		return
	}
	var req struct {
		PilotID int `json:"pilot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PilotID == 0 {
		http.Error(w, "pilot_id required", http.StatusBadRequest)
		return
	}
	if err := s.DB.SetLeader(code, req.PilotID); err != nil {
		http.Error(w, "failed to transfer leadership", http.StatusInternalServerError)
		log.Printf("SetLeader error: %v", err)
		return
	}
	if err := s.DB.IncrementVersion(code); err != nil {
		log.Printf("IncrementVersion error: %v", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleAddPilot lets the leader add a phantom pilot to the session.
// POST /api/sessions/{code}/add-pilot
func (s *Server) HandleAddPilot(w http.ResponseWriter, r *http.Request, code string) {
	if _, ok := s.requireLeader(w, r, code); !ok {
		return
	}

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
		if strings.Contains(err.Error(), "UNIQUE") {
			http.Error(w, "callsign already in session", http.StatusConflict)
			return
		}
		http.Error(w, "failed to add pilot", http.StatusInternalServerError)
		log.Printf("HandleAddPilot AddPilot error: %v", err)
		return
	}

	// Graduated escalation: minimize displacement of existing pilots.
	pilots, err := s.DB.GetActivePilots(code)
	if err != nil {
		http.Error(w, "failed to get pilots", http.StatusInternalServerError)
		log.Printf("HandleAddPilot GetActivePilots error: %v", err)
		return
	}

	inputs := buildPilotInputs(pilots)

	var newPilotInput freq.PilotInput
	var existingInputs []freq.PilotInput
	for _, inp := range inputs {
		if inp.ID == added.ID {
			newPilotInput = inp
		} else {
			existingInputs = append(existingInputs, inp)
		}
	}

	result := freq.FindMinimalDisplacement(existingInputs, newPilotInput)

	for _, a := range result.Assignments {
		if err := s.DB.UpdatePilotAssignment(a.PilotID, a.Channel, a.FreqMHz, a.BuddyGroup); err != nil {
			log.Printf("HandleAddPilot: UpdatePilotAssignment error for pilot %d: %v", a.PilotID, err)
		}
	}

	if err := s.DB.IncrementVersion(code); err != nil {
		log.Printf("IncrementVersion error: %v", err)
	}

	// Re-fetch the pilot to get the updated assignment.
	pilots, err = s.DB.GetActivePilots(code)
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

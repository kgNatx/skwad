package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kyleg/skwad/db"
	"github.com/kyleg/skwad/freq"
)

// geoClient is a shared HTTP client with a short timeout for IP geolocation.
var geoClient = &http.Client{Timeout: 5 * time.Second}

// isPrivateIP checks if an IP is private/loopback and should skip geolocation.
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified()
}

// clientIP extracts the client's IP from the request.
// Checks CF-Connecting-IP (Cloudflare Tunnel), then X-Forwarded-For, then RemoteAddr.
func clientIP(r *http.Request) string {
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return strings.TrimSpace(cfIP)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// lookupGeoAsync does an IP geolocation lookup in a goroutine and updates the session.
func (s *Server) lookupGeoAsync(sessionID string, r *http.Request) {
	ip := clientIP(r)
	if isPrivateIP(ip) {
		return
	}

	go func() {
		resp, err := geoClient.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=city,region,countryCode,lat,lon", ip))
		if err != nil {
			log.Printf("Geo lookup error for %s: %v", ip, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Geo lookup returned %d for %s", resp.StatusCode, ip)
			return
		}

		var geo struct {
			City    string  `json:"city"`
			Region  string  `json:"region"`
			Country string  `json:"countryCode"`
			Lat     float64 `json:"lat"`
			Lng     float64 `json:"lon"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil {
			log.Printf("Geo decode error for %s: %v", ip, err)
			return
		}

		if err := s.DB.UpdateSessionGeo(sessionID, geo.City, geo.Region, geo.Country, geo.Lat, geo.Lng); err != nil {
			log.Printf("Geo DB update error for session %s: %v", sessionID, err)
		}
	}()
}

// joinBands converts a slice of band codes to a comma-separated string.
// Returns "R" if the slice is empty (default to Race Band).
func joinBands(bands []string) string {
	if len(bands) == 0 {
		return "R"
	}
	return strings.Join(bands, ",")
}

// splitBands converts a comma-separated band string to a slice.
func splitBands(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// Server holds dependencies for the HTTP handlers.
type Server struct {
	DB                  *db.DB
	GitHubFeedbackToken string
}

// NewServer creates a new Server with the given database and GitHub feedback token.
func NewServer(database *db.DB, githubFeedbackToken string) *Server {
	return &Server{DB: database, GitHubFeedbackToken: githubFeedbackToken}
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
	if err != nil || leaderID == 0 || leaderID != pilotID {
		http.Error(w, "leader access required", http.StatusForbidden)
		return 0, false
	}
	return pilotID, true
}

// JoinRequest is the JSON body for joining a session.
type JoinRequest struct {
	Callsign         string   `json:"callsign"`
	VideoSystem      string   `json:"video_system"`
	FCCUnlocked      bool     `json:"fcc_unlocked"`
	Goggles          string   `json:"goggles"`
	BandwidthMHz     int      `json:"bandwidth_mhz"`
	RaceMode         bool     `json:"race_mode"`
	PreferredFreqMHz int      `json:"preferred_frequency_mhz"`
	AnalogBands      []string `json:"analog_bands"`
	Choice           string   `json:"choice"` // "", "buddy", or "rebalance"
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
	var req struct {
		PowerCeilingMW int    `json:"power_ceiling_mw"`
		FixedChannels  string `json:"fixed_channels"`
	}
	json.NewDecoder(r.Body).Decode(&req) // ignore error — empty body is fine

	sess, err := s.DB.CreateSession(req.PowerCeilingMW, req.FixedChannels)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		log.Printf("CreateSession error: %v", err)
		return
	}

	s.lookupGeoAsync(sess.ID, r)

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
	guardBand := freq.PowerToGuardBand(sess.PowerCeilingMW)
	assignments := buildAssignments(pilots)
	conflicts := freq.DetectConflicts(assignments, guardBand)

	// Build per-pilot lookup maps.
	pilotCallsigns := make(map[int]string, len(pilots))
	pilotBuddyGroup := make(map[int]int, len(pilots))
	for _, p := range pilots {
		pilotCallsigns[p.ID] = p.Callsign
		pilotBuddyGroup[p.ID] = p.BuddyGroup
	}

	pilotConflicts := make(map[int][]PilotConflict)
	for _, c := range conflicts {
		// Skip conflicts between pilots that are intentionally sharing (same buddy group).
		bgA, bgB := pilotBuddyGroup[c.PilotA], pilotBuddyGroup[c.PilotB]
		if bgA > 0 && bgA == bgB {
			continue
		}
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

	rebalanceRecommended := len(pilotConflicts) > 0

	resp := struct {
		Session              *db.Session          `json:"session"`
		Pilots               []PilotWithConflicts  `json:"pilots"`
		RebalanceRecommended bool                  `json:"rebalance_recommended"`
	}{
		Session:              sess,
		Pilots:               pilotsWithConflicts,
		RebalanceRecommended: rebalanceRecommended,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleJoinSession adds a pilot to a session.
// POST /api/sessions/{code}/join
func (s *Server) HandleJoinSession(w http.ResponseWriter, r *http.Request, code string) {
	// Verify session exists and get power ceiling.
	sess, err := s.DB.GetSession(code)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	guardBand := freq.PowerToGuardBand(sess.PowerCeilingMW)
	fixedFreqs := parseFixedFreqs(sess.FixedChannels)

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
		Callsign:         req.Callsign,
		VideoSystem:      req.VideoSystem,
		FCCUnlocked:      req.FCCUnlocked,
		Goggles:          req.Goggles,
		BandwidthMHz:     req.BandwidthMHz,
		RaceMode:         req.RaceMode,
		PreferredFreqMHz: req.PreferredFreqMHz,
		AnalogBands:      joinBands(req.AnalogBands),
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
			if deactErr := s.DB.DeactivatePilot(code, existingID); deactErr != nil {
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

	// Spotters skip frequency assignment entirely.
	if added.VideoSystem == "spotter" {
		if err := s.DB.IncrementVersion(code); err != nil {
			log.Printf("IncrementVersion error: %v", err)
		}
		if err := s.DB.IncrementJoinCount(code); err != nil {
			log.Printf("IncrementJoinCount error: %v", err)
		}
		leaderID, _ := s.DB.GetLeader(code)
		if leaderID == 0 {
			s.DB.SetLeader(code, added.ID)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(added)
		return
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

	result := freq.FindMinimalDisplacement(existingInputs, newPilotInput, guardBand, fixedFreqs)

	// Select the assignment set based on the client's choice.
	assignments := result.Assignments
	if req.Choice == "rebalance" && result.Level == 1 && result.RebalanceOption != nil {
		assignments = result.RebalanceOption.Assignments
	}

	// Apply all assignments from the selected set.
	for _, a := range assignments {
		if err := s.DB.UpdatePilotAssignment(code, a.PilotID, a.Channel, a.FreqMHz, a.BuddyGroup); err != nil {
			log.Printf("HandleJoinSession: UpdatePilotAssignment error for pilot %d: %v", a.PilotID, err)
		}
	}

	if err := s.DB.IncrementVersion(code); err != nil {
		log.Printf("IncrementVersion error: %v", err)
	}

	// Update usage counters.
	if err := s.DB.IncrementJoinCount(code); err != nil {
		log.Printf("IncrementJoinCount error: %v", err)
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

// RebalancePreview describes the partial rebalance option for Level 1.
type RebalancePreview struct {
	Assignment freq.Assignment  `json:"assignment"`
	Displaced  []DisplacedPilot `json:"displaced"`
}

// PreviewResponse is the JSON response shape for preview-join and preview-channel.
type PreviewResponse struct {
	Level           int                   `json:"level"`
	Assignment      freq.Assignment       `json:"assignment"`
	Displaced       []DisplacedPilot      `json:"displaced"`
	OverrideReason  string                `json:"override_reason,omitempty"`
	BuddyOption     *freq.BuddySuggestion `json:"buddy_option,omitempty"`
	RebalanceOption *RebalancePreview      `json:"rebalance_option,omitempty"`
}

// HandlePreviewJoin dry-runs graduated escalation with a hypothetical new pilot
// and returns the escalation level, new pilot's assignment, and displaced pilots.
// Nothing is committed.
// POST /api/sessions/{code}/preview-join
func (s *Server) HandlePreviewJoin(w http.ResponseWriter, r *http.Request, code string) {
	sess, err := s.DB.GetSession(code)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	guardBand := freq.PowerToGuardBand(sess.PowerCeilingMW)
	fixedFreqs := parseFixedFreqs(sess.FixedChannels)

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
		ID:               tempID,
		VideoSystem:      req.VideoSystem,
		FCCUnlocked:      req.FCCUnlocked,
		BandwidthMHz:     req.BandwidthMHz,
		RaceMode:         req.RaceMode,
		Goggles:          req.Goggles,
		PreferredFreqMHz: req.PreferredFreqMHz,
		AnalogBands:      req.AnalogBands,
	}

	result := freq.FindMinimalDisplacement(existingInputs, newPilotInput, guardBand, fixedFreqs)

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

	// Build override reason when Level 0 but preference was overridden.
	overrideReason := ""
	if result.Level == 0 && req.PreferredFreqMHz > 0 && newAssignment.FreqMHz != req.PreferredFreqMHz {
		for _, p := range pilots {
			if p.AssignedFreqMHz == req.PreferredFreqMHz {
				overrideReason = fmt.Sprintf("%s is on %s (%d MHz). You've been placed on %s (%d MHz).",
					p.Callsign, p.AssignedChannel, p.AssignedFreqMHz,
					newAssignment.Channel, newAssignment.FreqMHz)
				break
			}
		}
		if overrideReason == "" {
			overrideReason = fmt.Sprintf("Your preferred channel conflicts. You've been placed on %s (%d MHz).",
				newAssignment.Channel, newAssignment.FreqMHz)
		}
	}

	// Enrich buddy option with callsign from pilot data.
	if result.BuddyOption != nil {
		result.BuddyOption.Callsign = pilotCallsigns[result.BuddyOption.PilotID]
	}

	// Build rebalance preview.
	var rebalancePreview *RebalancePreview
	if result.RebalanceOption != nil {
		var rebalAssignment freq.Assignment
		var rebalDisplaced []DisplacedPilot
		for _, a := range result.RebalanceOption.Assignments {
			if a.PilotID == tempID {
				rebalAssignment = a
				continue
			}
			oldCh := pilotOldChannels[a.PilotID]
			oldFreq := pilotOldFreqs[a.PilotID]
			if oldCh != "" && (a.Channel != oldCh || a.FreqMHz != oldFreq) {
				rebalDisplaced = append(rebalDisplaced, DisplacedPilot{
					PilotID:    a.PilotID,
					Callsign:   pilotCallsigns[a.PilotID],
					OldChannel: oldCh,
					OldFreqMHz: oldFreq,
					NewChannel: a.Channel,
					NewFreqMHz: a.FreqMHz,
				})
			}
		}
		rebalancePreview = &RebalancePreview{
			Assignment: rebalAssignment,
			Displaced:  rebalDisplaced,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PreviewResponse{
		Level:           result.Level,
		Assignment:      newAssignment,
		Displaced:       displaced,
		OverrideReason:  overrideReason,
		BuddyOption:     result.BuddyOption,
		RebalanceOption: rebalancePreview,
	})
}

// UpdateChannelRequest is the JSON body for changing a pilot's channel preference.
type UpdateChannelRequest struct {
	PreferredFreqMHz int    `json:"preferred_frequency_mhz"`
	Force            bool   `json:"force"`            // leader-only: accept conflicting placement
	Choice           string `json:"choice"`           // "", "buddy", or "rebalance"
	ExcludeFreqMHz   int    `json:"exclude_freq_mhz"` // auto-assign: avoid this frequency
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

	sess, err := s.DB.GetSession(sessionCode)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	guardBand := freq.PowerToGuardBand(sess.PowerCeilingMW)
	fixedFreqs := parseFixedFreqs(sess.FixedChannels)

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
			changingPilotInput.PreferredFreqMHz = req.PreferredFreqMHz
			// If force flag (leader), pin the pilot at the requested frequency.
			if req.Force && req.PreferredFreqMHz > 0 {
				changingPilotInput.Pinned = true
				changingPilotInput.PinnedFreqMHz = req.PreferredFreqMHz
			}
			// Clear prev assignment since preferences changed.
			changingPilotInput.PrevChannel = ""
			changingPilotInput.PrevFreqMHz = 0
		} else {
			existingInputs = append(existingInputs, inp)
		}
	}

	result := freq.FindMinimalDisplacement(existingInputs, changingPilotInput, guardBand, fixedFreqs)

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

	// Build override reason when Level 0 but preference was overridden.
	overrideReason := ""
	if result.Level == 0 && req.PreferredFreqMHz > 0 && myAssignment.FreqMHz != req.PreferredFreqMHz {
		for _, p := range pilots {
			if p.AssignedFreqMHz == req.PreferredFreqMHz {
				overrideReason = fmt.Sprintf("%s is on %s (%d MHz). You've been placed on %s (%d MHz).",
					p.Callsign, p.AssignedChannel, p.AssignedFreqMHz,
					myAssignment.Channel, myAssignment.FreqMHz)
				break
			}
		}
		if overrideReason == "" {
			overrideReason = fmt.Sprintf("Your preferred channel conflicts. You've been placed on %s (%d MHz).",
				myAssignment.Channel, myAssignment.FreqMHz)
		}
	}

	// Enrich buddy option with callsign from pilot data.
	if result.BuddyOption != nil {
		result.BuddyOption.Callsign = pilotCallsigns[result.BuddyOption.PilotID]
	}

	// Build rebalance preview.
	var rebalancePreview *RebalancePreview
	if result.RebalanceOption != nil {
		var rebalAssignment freq.Assignment
		var rebalDisplaced []DisplacedPilot
		for _, a := range result.RebalanceOption.Assignments {
			if a.PilotID == pilotID {
				rebalAssignment = a
				continue
			}
			oldCh := pilotOldChannels[a.PilotID]
			oldFreq := pilotOldFreqs[a.PilotID]
			if oldCh != "" && (a.Channel != oldCh || a.FreqMHz != oldFreq) {
				rebalDisplaced = append(rebalDisplaced, DisplacedPilot{
					PilotID:    a.PilotID,
					Callsign:   pilotCallsigns[a.PilotID],
					OldChannel: oldCh,
					OldFreqMHz: oldFreq,
					NewChannel: a.Channel,
					NewFreqMHz: a.FreqMHz,
				})
			}
		}
		rebalancePreview = &RebalancePreview{
			Assignment: rebalAssignment,
			Displaced:  rebalDisplaced,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PreviewResponse{
		Level:           result.Level,
		Assignment:      myAssignment,
		Displaced:       displaced,
		OverrideReason:  overrideReason,
		BuddyOption:     result.BuddyOption,
		RebalanceOption: rebalancePreview,
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

	if err := s.DB.UpdatePilotPreference(sessionCode, pilotID, req.PreferredFreqMHz); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "pilot not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update pilot", http.StatusInternalServerError)
		log.Printf("UpdatePilotPreference error: %v", err)
		return
	}

	// Graduated escalation: treat the changing pilot as "new".
	sess, err := s.DB.GetSession(sessionCode)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	guardBand := freq.PowerToGuardBand(sess.PowerCeilingMW)
	fixedFreqs := parseFixedFreqs(sess.FixedChannels)

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
			changingPilotInput.PreferredFreqMHz = req.PreferredFreqMHz
			changingPilotInput.PrevChannel = ""
			changingPilotInput.PrevFreqMHz = 0
			if req.Force && req.PreferredFreqMHz > 0 {
				changingPilotInput.Pinned = true
				changingPilotInput.PinnedFreqMHz = req.PreferredFreqMHz
			}
		} else {
			existingInputs = append(existingInputs, inp)
		}
	}

	result := freq.FindMinimalDisplacement(existingInputs, changingPilotInput, guardBand, fixedFreqs)

	// Select the assignment set based on the client's choice.
	assignments := result.Assignments
	if req.Choice == "rebalance" && result.Level == 1 && result.RebalanceOption != nil {
		assignments = result.RebalanceOption.Assignments
	}

	// For auto-assign (PreferredFreqMHz == 0), if the optimizer gave back the
	// same frequency the pilot asked to leave, try to find a different one.
	if req.ExcludeFreqMHz > 0 && req.PreferredFreqMHz == 0 {
		for _, a := range assignments {
			if a.PilotID == pilotID && a.FreqMHz == req.ExcludeFreqMHz {
				// The optimizer picked the excluded frequency. Find the next-best
				// clean channel from the pilot's pool.
				pool := freq.ChannelPool(
					changingPilotInput.VideoSystem,
					changingPilotInput.FCCUnlocked,
					changingPilotInput.BandwidthMHz,
					changingPilotInput.RaceMode,
					changingPilotInput.Goggles,
					changingPilotInput.AnalogBands,
				)
				bw := freq.OccupiedBandwidth(changingPilotInput.VideoSystem, changingPilotInput.BandwidthMHz)
				// Build occupied set from existing pilots (not the changing pilot).
				type occupied struct {
					freq int
					bw   int
				}
				var others []occupied
				for _, ep := range existingInputs {
					for _, ea := range assignments {
						if ea.PilotID == ep.ID {
							others = append(others, occupied{ea.FreqMHz, freq.OccupiedBandwidth(ep.VideoSystem, ep.BandwidthMHz)})
							break
						}
					}
				}
				bestFreq := 0
				bestChannel := ""
				bestMargin := -1
				for _, ch := range pool {
					if ch.FreqMHz == req.ExcludeFreqMHz {
						continue
					}
					minSep := 9999
					for _, o := range others {
						sep := ch.FreqMHz - o.freq
						if sep < 0 {
							sep = -sep
						}
						required := (bw + o.bw) / 2
						margin := sep - required
						if margin < minSep {
							minSep = margin
						}
					}
					if minSep > bestMargin {
						bestMargin = minSep
						bestFreq = ch.FreqMHz
						bestChannel = ch.Name
					}
				}
				if bestFreq > 0 {
					// Patch the assignment in place.
					for i := range assignments {
						if assignments[i].PilotID == pilotID {
							assignments[i].FreqMHz = bestFreq
							assignments[i].Channel = bestChannel
							break
						}
					}
				}
				break
			}
		}
	}

	for _, a := range assignments {
		if err := s.DB.UpdatePilotAssignment(sessionCode, a.PilotID, a.Channel, a.FreqMHz, a.BuddyGroup); err != nil {
			log.Printf("HandleUpdatePilotChannel: UpdatePilotAssignment error for pilot %d: %v", a.PilotID, err)
		}
	}

	if err := s.DB.IncrementVersion(sessionCode); err != nil {
		log.Printf("IncrementVersion error: %v", err)
	}

	if err := s.DB.IncrementChannelChangeCount(sessionCode); err != nil {
		log.Printf("IncrementChannelChangeCount error: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateVideoSystemRequest is the JSON body for changing a pilot's video system.
type UpdateVideoSystemRequest struct {
	VideoSystem      string   `json:"video_system"`
	FCCUnlocked      bool     `json:"fcc_unlocked"`
	Goggles          string   `json:"goggles"`
	BandwidthMHz     int      `json:"bandwidth_mhz"`
	RaceMode         bool     `json:"race_mode"`
	AnalogBands      []string `json:"analog_bands"`
	PreferredFreqMHz int      `json:"preferred_frequency_mhz"`
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

	if err := s.DB.UpdatePilotVideoSystem(sessionCode, pilotID, req.VideoSystem, req.FCCUnlocked, req.Goggles, req.BandwidthMHz, req.RaceMode, joinBands(req.AnalogBands), req.PreferredFreqMHz); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "pilot not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update pilot", http.StatusInternalServerError)
		log.Printf("UpdatePilotVideoSystem error: %v", err)
		return
	}

	// Switching to spotter: clear assignment, skip optimizer.
	if req.VideoSystem == "spotter" {
		if err := s.DB.UpdatePilotAssignment(sessionCode, pilotID, "", 0, 0); err != nil {
			log.Printf("HandleUpdatePilotVideoSystem: clear spotter assignment error: %v", err)
		}
		if err := s.DB.IncrementVersion(sessionCode); err != nil {
			log.Printf("IncrementVersion error: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Graduated escalation: treat the changing pilot as "new".
	sess, err := s.DB.GetSession(sessionCode)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	guardBand := freq.PowerToGuardBand(sess.PowerCeilingMW)
	fixedFreqs := parseFixedFreqs(sess.FixedChannels)

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
			// Clear prev assignment since video system changed — unless
			// fixed channels are set, where the prev freq is still valid.
			if len(fixedFreqs) == 0 {
				changingPilotInput.PrevChannel = ""
				changingPilotInput.PrevFreqMHz = 0
			}
		} else {
			existingInputs = append(existingInputs, inp)
		}
	}

	result := freq.FindMinimalDisplacement(existingInputs, changingPilotInput, guardBand, fixedFreqs)

	for _, a := range result.Assignments {
		if err := s.DB.UpdatePilotAssignment(sessionCode, a.PilotID, a.Channel, a.FreqMHz, a.BuddyGroup); err != nil {
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

	if err := s.DB.UpdatePilotCallsign(sessionCode, pilotID, callsign); err != nil {
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
func (s *Server) HandleDeactivatePilot(w http.ResponseWriter, r *http.Request, pilotID int, sessionCode string) {
	idStr := r.Header.Get("X-Pilot-ID")
	if idStr == "" {
		http.Error(w, "X-Pilot-ID header required", http.StatusUnauthorized)
		return
	}
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

	if err := s.DB.DeactivatePilot(sessionCode, pilotID); err != nil {
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
// Spotters are excluded — they don't participate in frequency assignment.
func buildPilotInputs(pilots []db.Pilot) []freq.PilotInput {
	var inputs []freq.PilotInput
	for _, p := range pilots {
		if p.VideoSystem == "spotter" {
			continue
		}
		inputs = append(inputs, freq.PilotInput{
			ID:               p.ID,
			VideoSystem:      p.VideoSystem,
			FCCUnlocked:      p.FCCUnlocked,
			BandwidthMHz:     p.BandwidthMHz,
			RaceMode:         p.RaceMode,
			Goggles:          p.Goggles,
			PreferredFreqMHz: p.PreferredFreqMHz,
			PrevChannel:      p.AssignedChannel,
			PrevFreqMHz:      p.AssignedFreqMHz,
			AnalogBands:      splitBands(p.AnalogBands),
		})
	}
	return inputs
}

// buildAssignments converts DB pilots to freq.Assignment structs for conflict detection.
// Spotters are excluded — they have no frequency and can't conflict.
func buildAssignments(pilots []db.Pilot) []freq.Assignment {
	var assignments []freq.Assignment
	for _, p := range pilots {
		if p.VideoSystem == "spotter" {
			continue
		}
		assignments = append(assignments, freq.Assignment{
			PilotID:      p.ID,
			Channel:      p.AssignedChannel,
			FreqMHz:      p.AssignedFreqMHz,
			BandwidthMHz: freq.OccupiedBandwidth(p.VideoSystem, p.BandwidthMHz),
			BuddyGroup:   p.BuddyGroup,
		})
	}
	return assignments
}

// parseFixedFreqs parses a JSON fixed_channels string into a slice of frequency MHz values.
// Returns nil if the string is empty or cannot be parsed.
func parseFixedFreqs(fixedChannels string) []int {
	if fixedChannels == "" {
		return nil
	}
	var channels []struct {
		Name string `json:"name"`
		Freq int    `json:"freq"`
	}
	if err := json.Unmarshal([]byte(fixedChannels), &channels); err != nil {
		return nil
	}
	freqs := make([]int, len(channels))
	for i, ch := range channels {
		freqs[i] = ch.Freq
	}
	return freqs
}

// HandlePreviewRebalance runs the optimizer dry-run (leader-only).
// Returns proposed assignments without committing.
// POST /api/sessions/{code}/preview-rebalance
func (s *Server) HandlePreviewRebalance(w http.ResponseWriter, r *http.Request, code string) {
	if _, ok := s.requireLeader(w, r, code); !ok {
		return
	}

	var req struct {
		PowerCeilingMW *int `json:"power_ceiling_mw"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	sess, err := s.DB.GetSession(code)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	ceiling := sess.PowerCeilingMW
	if req.PowerCeilingMW != nil {
		ceiling = *req.PowerCeilingMW
	}
	guardBand := freq.PowerToGuardBand(ceiling)
	fixedFreqs := parseFixedFreqs(sess.FixedChannels)

	pilots, err := s.DB.GetActivePilots(code)
	if err != nil {
		http.Error(w, "failed to get pilots", http.StatusInternalServerError)
		return
	}

	inputs := buildPilotInputs(pilots)

	// Run same two-phase logic as reoptimize but don't save.
	assignments := freq.Optimize(inputs, guardBand, fixedFreqs)
	conflicts := freq.DetectConflicts(assignments, guardBand)

	if len(conflicts) > 0 {
		conflictPilots := make(map[int]bool)
		for _, c := range conflicts {
			conflictPilots[c.PilotA] = true
			conflictPilots[c.PilotB] = true
		}
		cleanIDs := make(map[int]bool)
		for _, p := range inputs {
			if !conflictPilots[p.ID] {
				cleanIDs[p.ID] = true
			}
		}
		surgical := freq.OptimizeWithLocks(inputs, cleanIDs, guardBand, fixedFreqs)
		surgicalConflicts := freq.DetectConflicts(surgical, guardBand)
		if len(surgicalConflicts) < len(conflicts) {
			assignments = surgical
		}
	}

	// Build response with pilot info needed for spectrum rendering.
	type ProposedPilot struct {
		ID             int    `json:"ID"`
		Callsign       string `json:"Callsign"`
		VideoSystem    string `json:"VideoSystem"`
		BandwidthMHz   int    `json:"BandwidthMHz"`
		AssignedFreqMHz int   `json:"AssignedFreqMHz"`
		AssignedChannel string `json:"AssignedChannel"`
	}

	pilotMap := make(map[int]db.Pilot, len(pilots))
	for _, p := range pilots {
		pilotMap[p.ID] = p
	}

	proposed := make([]ProposedPilot, 0, len(assignments))
	for _, a := range assignments {
		p := pilotMap[a.PilotID]
		proposed = append(proposed, ProposedPilot{
			ID:              p.ID,
			Callsign:        p.Callsign,
			VideoSystem:     p.VideoSystem,
			BandwidthMHz:    p.BandwidthMHz,
			AssignedFreqMHz: a.FreqMHz,
			AssignedChannel: a.Channel,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proposed)
}

// HandleRebalanceAll runs the full optimizer on all pilots (leader-only).
// Returns which pilots moved and which stayed.
// POST /api/sessions/{code}/rebalance
func (s *Server) HandleRebalanceAll(w http.ResponseWriter, r *http.Request, code string) {
	if _, ok := s.requireLeader(w, r, code); !ok {
		return
	}

	var req struct {
		PowerCeilingMW *int `json:"power_ceiling_mw"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	sess, err := s.DB.GetSession(code)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Update power ceiling if the leader changed it
	if req.PowerCeilingMW != nil && *req.PowerCeilingMW != sess.PowerCeilingMW {
		if err := s.DB.UpdateSessionPowerCeiling(code, *req.PowerCeilingMW); err != nil {
			http.Error(w, "failed to update power ceiling", http.StatusInternalServerError)
			return
		}
		sess.PowerCeilingMW = *req.PowerCeilingMW
	}
	guardBand := freq.PowerToGuardBand(sess.PowerCeilingMW)
	fixedFreqs := parseFixedFreqs(sess.FixedChannels)

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

	s.reoptimize(code, guardBand, fixedFreqs)

	if err := s.DB.IncrementRebalanceCount(code); err != nil {
		log.Printf("IncrementRebalanceCount error: %v", err)
	}

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
	dangerConflicts := freq.DetectConflicts(assignments, guardBand)

	// Build lookup maps.
	pilotMap := make(map[int]db.Pilot, len(pilotsAfter))
	for _, p := range pilotsAfter {
		pilotMap[p.ID] = p
	}

	type UnresolvedConflict struct {
		PilotA      string `json:"pilot_a"`
		PilotB      string `json:"pilot_b"`
		PreferredBy string `json:"preferred_by"`
		Reason      string `json:"reason"`
	}
	var unresolved []UnresolvedConflict
	for _, c := range dangerConflicts {
		if c.Level != freq.ConflictDanger {
			continue
		}
		pA, pB := pilotMap[c.PilotA], pilotMap[c.PilotB]

		preferenceNote := ""
		if pA.PreferredFreqMHz > 0 && pB.PreferredFreqMHz > 0 {
			preferenceNote = pA.Callsign + " and " + pB.Callsign + " both have channel preferences"
		} else if pA.PreferredFreqMHz > 0 {
			preferenceNote = pA.Callsign + " has a channel preference"
		} else if pB.PreferredFreqMHz > 0 {
			preferenceNote = pB.Callsign + " has a channel preference"
		}

		reason := pA.Callsign + " (" + pA.AssignedChannel + ") and " +
			pB.Callsign + " (" + pB.AssignedChannel + ") are " +
			fmt.Sprintf("%d", c.SeparationMHz) + " MHz apart but need " +
			fmt.Sprintf("%d", c.RequiredMHz) + " MHz"
		if preferenceNote != "" {
			reason += ". " + preferenceNote
		}

		preferredBy := ""
		if pA.PreferredFreqMHz > 0 && pB.PreferredFreqMHz > 0 {
			preferredBy = pA.Callsign + " and " + pB.Callsign
		} else if pA.PreferredFreqMHz > 0 {
			preferredBy = pA.Callsign
		} else if pB.PreferredFreqMHz > 0 {
			preferredBy = pB.Callsign
		}

		unresolved = append(unresolved, UnresolvedConflict{
			PilotA:      pA.Callsign,
			PilotB:      pB.Callsign,
			PreferredBy: preferredBy,
			Reason:      reason,
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
	pilots, err := s.DB.GetActivePilots(code)
	if err != nil {
		http.Error(w, "failed to get pilots", http.StatusInternalServerError)
		return
	}
	found := false
	for _, p := range pilots {
		if p.ID == req.PilotID {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "target pilot not in session", http.StatusBadRequest)
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

	// Verify session exists and get power ceiling.
	sess, err := s.DB.GetSession(code)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	guardBand := freq.PowerToGuardBand(sess.PowerCeilingMW)
	fixedFreqs := parseFixedFreqs(sess.FixedChannels)

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
		Callsign:         req.Callsign,
		VideoSystem:      req.VideoSystem,
		FCCUnlocked:      req.FCCUnlocked,
		Goggles:          req.Goggles,
		BandwidthMHz:     req.BandwidthMHz,
		RaceMode:         req.RaceMode,
		PreferredFreqMHz: req.PreferredFreqMHz,
		AnalogBands:      joinBands(req.AnalogBands),
		AddedByLeader:    true,
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

	// Spotters skip frequency assignment entirely.
	if added.VideoSystem == "spotter" {
		if err := s.DB.IncrementVersion(code); err != nil {
			log.Printf("IncrementVersion error: %v", err)
		}
		if err := s.DB.IncrementJoinCount(code); err != nil {
			log.Printf("IncrementJoinCount error: %v", err)
		}
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

	result := freq.FindMinimalDisplacement(existingInputs, newPilotInput, guardBand, fixedFreqs)

	for _, a := range result.Assignments {
		if err := s.DB.UpdatePilotAssignment(code, a.PilotID, a.Channel, a.FreqMHz, a.BuddyGroup); err != nil {
			log.Printf("HandleAddPilot: UpdatePilotAssignment error for pilot %d: %v", a.PilotID, err)
		}
	}

	if err := s.DB.IncrementVersion(code); err != nil {
		log.Printf("IncrementVersion error: %v", err)
	}

	if err := s.DB.IncrementJoinCount(code); err != nil {
		log.Printf("IncrementJoinCount error: %v", err)
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
// guardBandMHz is derived from the session's power ceiling.
// fixedFreqs lists frequencies that must be avoided by the optimizer.
func (s *Server) reoptimize(sessionCode string, guardBandMHz int, fixedFreqs []int) {
	pilots, err := s.DB.GetActivePilots(sessionCode)
	if err != nil {
		log.Printf("reoptimize: GetActivePilots error: %v", err)
		return
	}
	if len(pilots) == 0 {
		return
	}

	inputs := buildPilotInputs(pilots)

	// Phase 1: surgical -- only move pilots involved in conflicts.
	assignments := freq.Optimize(inputs, guardBandMHz, fixedFreqs)
	conflicts := freq.DetectConflicts(assignments, guardBandMHz)

	if len(conflicts) > 0 {
		// Identify pilots involved in conflicts.
		conflictPilots := make(map[int]bool)
		for _, c := range conflicts {
			conflictPilots[c.PilotA] = true
			conflictPilots[c.PilotB] = true
		}

		// Pin everyone except conflicted pilots.
		cleanIDs := make(map[int]bool)
		for _, p := range inputs {
			if !conflictPilots[p.ID] {
				cleanIDs[p.ID] = true
			}
		}

		surgical := freq.OptimizeWithLocks(inputs, cleanIDs, guardBandMHz, fixedFreqs)
		surgicalConflicts := freq.DetectConflicts(surgical, guardBandMHz)

		if len(surgicalConflicts) < len(conflicts) {
			assignments = surgical
		}
	}

	for _, a := range assignments {
		if err := s.DB.UpdatePilotAssignment(sessionCode, a.PilotID, a.Channel, a.FreqMHz, a.BuddyGroup); err != nil {
			log.Printf("reoptimize: UpdatePilotAssignment error for pilot %d: %v", a.PilotID, err)
		}
	}

	if err := s.DB.IncrementVersion(sessionCode); err != nil {
		log.Printf("reoptimize: IncrementVersion error: %v", err)
	}
}

// HandleUsage returns aggregate usage metrics.
// GET /api/usage
func (s *Server) HandleUsage(w http.ResponseWriter, r *http.Request) {
	stats, err := s.DB.GetUsageStats()
	if err != nil {
		http.Error(w, "failed to get usage stats", http.StatusInternalServerError)
		log.Printf("GetUsageStats error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// feedbackRequest is the JSON body for POST /api/feedback.
type feedbackRequest struct {
	Type    string          `json:"type"`
	Message string          `json:"message"`
	Context json.RawMessage `json:"context,omitempty"`
}

// rateLimiter tracks per-IP last-request times with a cooldown window.
type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]time.Time
	window  time.Duration
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		entries: make(map[string]time.Time),
		window:  60 * time.Second,
	}
}

// allow returns true if the IP is allowed to proceed (not rate-limited).
// Updates the last-seen time on success.
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	last, seen := rl.entries[ip]
	if seen && time.Since(last) < rl.window {
		return false
	}
	rl.entries[ip] = time.Now()
	return true
}

// cleanup removes entries older than 5 minutes.
func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-5 * time.Minute)
	for ip, t := range rl.entries {
		if t.Before(cutoff) {
			delete(rl.entries, ip)
		}
	}
}

// feedbackLimiter is the package-level rate limiter for feedback submissions.
var feedbackLimiter = newRateLimiter()

// validFeedbackTypes lists the accepted feedback type values.
var validFeedbackTypes = map[string]bool{
	"bug":         true,
	"feedback":    true,
	"translation": true,
}

// labelForType maps feedback types to GitHub issue labels.
var labelForType = map[string]string{
	"bug":         "bug",
	"feedback":    "feedback",
	"translation": "translation",
}

// HandleFeedback handles POST /api/feedback.
// It validates the request, rate-limits by IP, and creates a GitHub issue.
func (s *Server) HandleFeedback(w http.ResponseWriter, r *http.Request) {
	if s.GitHubFeedbackToken == "" {
		http.Error(w, "feedback unavailable", http.StatusServiceUnavailable)
		return
	}

	ip := clientIP(r)
	if !feedbackLimiter.allow(ip) {
		http.Error(w, "rate limited: please wait before submitting again", http.StatusTooManyRequests)
		return
	}

	var req feedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if !validFeedbackTypes[req.Type] {
		http.Error(w, "invalid feedback type", http.StatusBadRequest)
		return
	}

	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}
	if len(msg) > 2000 {
		http.Error(w, "message too long (max 2000 characters)", http.StatusBadRequest)
		return
	}

	title := buildTitle(req.Type, msg)
	body := buildBody(msg, req.Context)
	label := labelForType[req.Type]

	if err := createGitHubIssue(s.GitHubFeedbackToken, title, body, label); err != nil {
		log.Printf("feedback: failed to create GitHub issue: %v", err)
		http.Error(w, "failed to submit feedback", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// buildTitle constructs a GitHub issue title from the feedback type and message.
// Total length is capped at ~60 chars, truncated at a word boundary.
func buildTitle(fbType, msg string) string {
	prefix := "[" + fbType + "] "
	maxBody := 60 - len(prefix)
	if maxBody < 1 {
		maxBody = 1
	}

	if len(msg) <= maxBody {
		return prefix + msg
	}

	// Truncate at word boundary.
	truncated := msg[:maxBody]
	if idx := strings.LastIndex(truncated, " "); idx > 0 {
		truncated = truncated[:idx]
	}
	return prefix + truncated + "..."
}

// buildBody composes the GitHub issue body from the message and optional context.
func buildBody(msg string, ctx json.RawMessage) string {
	var sb strings.Builder
	sb.WriteString(msg)
	if details := formatContext(ctx); details != "" {
		sb.WriteString("\n\n")
		sb.WriteString(details)
	}
	return sb.String()
}

// formatContext renders a JSON context blob as a GitHub <details> block.
// Known fields are rendered with human-friendly labels; unknown fields are skipped.
// Zero/empty values are omitted.
func formatContext(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var ctx map[string]interface{}
	if err := json.Unmarshal(raw, &ctx); err != nil {
		return ""
	}
	if len(ctx) == 0 {
		return ""
	}

	// Ordered list of fields to render, with friendly labels.
	type field struct {
		key   string
		label string
	}
	fields := []field{
		{"page", "Page"},
		{"session", "Session"},
		{"pilots", "Pilots"},
		{"power_ceiling", "Power Ceiling"},
		{"video_system", "Video System"},
		{"language", "Language"},
		{"user_agent", "User-Agent"},
		{"timestamp", "Timestamp"},
	}

	var rows strings.Builder
	for _, f := range fields {
		val, ok := ctx[f.key]
		if !ok {
			continue
		}
		// Skip zero/empty values.
		switch v := val.(type) {
		case string:
			if v == "" {
				continue
			}
			fmt.Fprintf(&rows, "- **%s**: %s\n", f.label, v)
		case float64:
			if v == 0 {
				continue
			}
			fmt.Fprintf(&rows, "- **%s**: %v\n", f.label, v)
		case bool:
			fmt.Fprintf(&rows, "- **%s**: %v\n", f.label, v)
		default:
			if val == nil {
				continue
			}
			fmt.Fprintf(&rows, "- **%s**: %v\n", f.label, val)
		}
	}

	body := rows.String()
	if body == "" {
		return ""
	}

	return "<details>\n<summary>Context</summary>\n\n" + body + "</details>"
}

// createGitHubIssue POSTs a new issue to the kgNatx/skwad GitHub repository.
func createGitHubIssue(token, title, body, label string) error {
	payload := map[string]interface{}{
		"title":  title,
		"body":   body,
		"labels": []string{label},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost,
		"https://api.github.com/repos/kgNatx/skwad/issues",
		bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	return nil
}

// StartFeedbackCleanup runs the rate limiter cleanup every 5 minutes.
// Call as a goroutine from main.
func StartFeedbackCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		feedbackLimiter.cleanup()
	}
}

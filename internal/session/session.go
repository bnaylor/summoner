package session

import (
	"sync"
	"time"
)

// ActiveModel holds the details needed to re-spawn a summoned CLI process.
type ActiveModel struct {
	Name    string // "claude" or "gemini"
	Variant string // "opus", "pro", etc. or "" for CLI default
	Prompt  string // initial summon prompt, unchanged across re-spawns
}

// Session tracks a single active consulting session in one Discord channel.
type Session struct {
	ChannelID string // Discord channel ID this session belongs to
	mu        sync.Mutex
	models    map[string]*ActiveModel
	timer     *time.Timer
}

// NewSession creates a new Session for the given Discord channel ID.
func NewSession(channelID string) *Session {
	return &Session{
		ChannelID: channelID,
		models:    make(map[string]*ActiveModel),
	}
}

// AddModel registers a model as active in this session.
// If the model is already active, its variant is updated.
func (s *Session) AddModel(name, variant, prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.models[name] = &ActiveModel{Name: name, Variant: variant, Prompt: prompt}
}

// HasModel reports whether the named model is currently active.
func (s *Session) HasModel(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.models[name]
	return ok
}

// Models returns a snapshot of all active models.
func (s *Session) Models() []ActiveModel {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ActiveModel, 0, len(s.models))
	for _, m := range s.models {
		out = append(out, *m)
	}
	return out
}

// ResetTimer starts or restarts the inactivity timer.
// fn is called when the timer fires (in its own goroutine).
func (s *Session) ResetTimer(d time.Duration, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(d, fn)
}

// StopTimer cancels the inactivity timer if one is running.
func (s *Session) StopTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

package session

import (
	"sync"
	"time"
)

// ActiveModel holds the details needed to re-spawn a summoned CLI process.
// Prompt stores the fully-formatted -p payload (not the raw user input).
type ActiveModel struct {
	Name    string
	Variant string
	Prompt  string // fully-formatted CLI payload, reused on every re-spawn
}

// Session tracks a single active consulting session in one Discord channel.
type Session struct {
	ChannelID   string
	mu          sync.Mutex
	models      map[string]*ActiveModel
	timer       *time.Timer
	leaderModel string
}

// NewSession creates a new Session for the given Discord channel ID.
func NewSession(channelID string) *Session {
	return &Session{
		ChannelID: channelID,
		models:    make(map[string]*ActiveModel),
	}
}

// AddModel registers a model as active. If already present, updates variant and prompt.
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

// Model returns the ActiveModel for the given name, if present.
func (s *Session) Model(name string) (ActiveModel, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.models[name]
	if !ok {
		return ActiveModel{}, false
	}
	return *m, true
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

// SetLeader marks this session as a roundtable and designates the named model as leader.
// The model must already have been added via AddModel.
func (s *Session) SetLeader(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.models[model]; !ok {
		panic("session.SetLeader: model not registered: " + model)
	}
	s.leaderModel = model
}

// IsRoundtable reports whether this is a structured roundtable session.
func (s *Session) IsRoundtable() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.leaderModel != ""
}

// LeaderModel returns the name of the leader model, or "" for non-roundtable sessions.
func (s *Session) LeaderModel() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.leaderModel
}

// ParticipantNames returns the names of all active models that are not the leader.
func (s *Session) ParticipantNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for name := range s.models {
		if name != s.leaderModel {
			out = append(out, name)
		}
	}
	return out
}

// ResetTimer starts or restarts the inactivity timer. fn is called when it fires.
func (s *Session) ResetTimer(d time.Duration, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(d, fn)
}

// StopTimer cancels the inactivity timer.
func (s *Session) StopTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

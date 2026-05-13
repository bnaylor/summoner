package session

import "sync"

// Manager holds all active sessions and the registry of known agent bot user IDs.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	agentIDs map[string]bool
}

// NewManager creates a Manager. agentIDs is the list of Discord user IDs
// belonging to summoned-agent bots (e.g. BTClaude, BTGemini) whose messages
// should not trigger re-spawns.
func NewManager(agentIDs []string) *Manager {
	ids := make(map[string]bool, len(agentIDs))
	for _, id := range agentIDs {
		ids[id] = true
	}
	return &Manager{
		sessions: make(map[string]*Session),
		agentIDs: ids,
	}
}

// Get returns the active session for a channel, or nil if none exists.
func (m *Manager) Get(channelID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[channelID]
}

// GetOrCreate returns the active session for a channel, creating one if needed.
func (m *Manager) GetOrCreate(channelID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[channelID]; ok {
		return s
	}
	s := NewSession(channelID)
	m.sessions[channelID] = s
	return s
}

// Remove deletes the session for a channel, stopping its timer.
func (m *Manager) Remove(channelID string) {
	m.mu.Lock()
	s := m.sessions[channelID]
	delete(m.sessions, channelID)
	m.mu.Unlock()
	if s != nil {
		s.StopTimer()
	}
}

// IsAgent reports whether a Discord user ID belongs to a known summoned agent.
func (m *Manager) IsAgent(userID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.agentIDs[userID]
}

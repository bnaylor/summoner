package session_test

import (
	"testing"

	"github.com/bnaylor/summoner/internal/session"
)

func TestManager_GetOrCreate(t *testing.T) {
	m := session.NewManager(nil)
	s1 := m.GetOrCreate("chan1")
	s2 := m.GetOrCreate("chan1")
	if s1 != s2 {
		t.Fatal("GetOrCreate should return same session for same channel")
	}
}

func TestManager_GetNonexistent(t *testing.T) {
	m := session.NewManager(nil)
	if m.Get("missing") != nil {
		t.Fatal("Get should return nil for unknown channel")
	}
}

func TestManager_Remove(t *testing.T) {
	m := session.NewManager(nil)
	m.GetOrCreate("chan1")
	m.Remove("chan1")
	if m.Get("chan1") != nil {
		t.Fatal("session should be gone after Remove")
	}
}

func TestManager_IsAgent(t *testing.T) {
	m := session.NewManager([]string{"bot-id-1", "bot-id-2"})
	if !m.IsAgent("bot-id-1") {
		t.Fatal("should be agent")
	}
	if m.IsAgent("human-id") {
		t.Fatal("should not be agent")
	}
}

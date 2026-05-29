package session_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/bnaylor/summoner/internal/session"
)

func TestSession_AddAndHasModel(t *testing.T) {
	s := session.NewSession("chan1")
	if s.HasModel("claude") {
		t.Fatal("should not have claude before add")
	}
	s.AddModel("claude", "opus", "design the cache")
	if !s.HasModel("claude") {
		t.Fatal("should have claude after add")
	}
}

func TestSession_ModelsReturnsAll(t *testing.T) {
	s := session.NewSession("chan1")
	s.AddModel("claude", "opus", "discuss")
	s.AddModel("gemini", "pro", "discuss")
	models := s.Models()
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
}

func TestSession_AddModelIdempotent(t *testing.T) {
	s := session.NewSession("chan1")
	s.AddModel("claude", "opus", "discuss")
	s.AddModel("claude", "sonnet", "discuss more")
	models := s.Models()
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].Variant != "sonnet" {
		t.Fatalf("expected updated variant sonnet, got %q", models[0].Variant)
	}
}

func TestSession_TimerFires(t *testing.T) {
	s := session.NewSession("chan1")
	fired := make(chan struct{})
	s.ResetTimer(50*time.Millisecond, func() { close(fired) })
	select {
	case <-fired:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timer did not fire")
	}
}

func TestSession_TimerReset(t *testing.T) {
	s := session.NewSession("chan1")
	var count atomic.Int32
	s.ResetTimer(50*time.Millisecond, func() { count.Add(1) })
	time.Sleep(20 * time.Millisecond)
	s.ResetTimer(50*time.Millisecond, func() { count.Add(1) })
	time.Sleep(20 * time.Millisecond)
	if count.Load() != 0 {
		t.Fatal("timer fired before reset period elapsed")
	}
	time.Sleep(100 * time.Millisecond)
	if count.Load() != 1 {
		t.Fatalf("expected 1 fire after reset, got %d", count.Load())
	}
}

func TestSession_StopTimer(t *testing.T) {
	s := session.NewSession("chan1")
	fired := make(chan struct{}, 1)
	s.ResetTimer(50*time.Millisecond, func() { fired <- struct{}{} })
	s.StopTimer()
	time.Sleep(100 * time.Millisecond)
	if len(fired) > 0 {
		t.Fatal("timer fired after stop")
	}
}

func TestSession_SetLeaderIsRoundtable(t *testing.T) {
	s := session.NewSession("chan1")
	if s.IsRoundtable() {
		t.Fatal("should not be roundtable before SetLeader")
	}
	s.AddModel("claude", "opus", "payload-for-claude")
	s.AddModel("gemini", "", "payload-for-gemini")
	s.SetLeader("claude")
	if !s.IsRoundtable() {
		t.Fatal("should be roundtable after SetLeader")
	}
	if s.LeaderModel() != "claude" {
		t.Fatalf("expected leader claude, got %q", s.LeaderModel())
	}
}

func TestSession_ParticipantNamesExcludesLeader(t *testing.T) {
	s := session.NewSession("chan1")
	s.AddModel("claude", "opus", "payload-claude")
	s.AddModel("gemini", "", "payload-gemini")
	s.AddModel("deepseek", "", "payload-deepseek")
	s.SetLeader("claude")
	names := s.ParticipantNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(names))
	}
	for _, n := range names {
		if n == "claude" {
			t.Fatal("leader should not appear in ParticipantNames")
		}
	}
}

func TestSession_ModelLookup(t *testing.T) {
	s := session.NewSession("chan1")
	s.AddModel("claude", "opus", "the-payload")
	m, ok := s.Model("claude")
	if !ok {
		t.Fatal("expected to find claude")
	}
	if m.Prompt != "the-payload" {
		t.Fatalf("unexpected prompt: %q", m.Prompt)
	}
	_, ok = s.Model("gemini")
	if ok {
		t.Fatal("should not find gemini (not added)")
	}
}

package session_test

import (
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
	count := 0
	s.ResetTimer(50*time.Millisecond, func() { count++ })
	time.Sleep(20 * time.Millisecond)
	s.ResetTimer(50*time.Millisecond, func() { count++ })
	time.Sleep(20 * time.Millisecond)
	if count != 0 {
		t.Fatal("timer fired before reset period elapsed")
	}
	time.Sleep(100 * time.Millisecond)
	if count != 1 {
		t.Fatalf("expected 1 fire after reset, got %d", count)
	}
}

func TestSession_StopTimer(t *testing.T) {
	s := session.NewSession("chan1")
	fired := false
	s.ResetTimer(50*time.Millisecond, func() { fired = true })
	s.StopTimer()
	time.Sleep(100 * time.Millisecond)
	if fired {
		t.Fatal("timer fired after stop")
	}
}

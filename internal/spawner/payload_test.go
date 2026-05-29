package spawner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bnaylor/summoner/internal/spawner"
)

func TestFormatPayload_ContainsPrompt(t *testing.T) {
	out := spawner.FormatPayload("let's design the caching layer")
	if !strings.Contains(out, "let's design the caching layer") {
		t.Fatalf("payload missing prompt: %q", out)
	}
}

func TestFormatPayload_ContainsSummoningContext(t *testing.T) {
	out := spawner.FormatPayload("review the schema")
	if !strings.Contains(out, "seasoned architect") {
		t.Fatal("payload missing architect framing")
	}
	if !strings.Contains(out, "Discord") {
		t.Fatal("payload missing Discord context")
	}
}

func TestFormatPayload_ContainsDepartureInstruction(t *testing.T) {
	out := spawner.FormatPayload("anything")
	if !strings.Contains(out, "stepping out") {
		t.Fatal("payload missing departure instruction")
	}
}

func TestFormatLeaderPayload_ContainsTopic(t *testing.T) {
	out := spawner.FormatLeaderPayload("design the caching layer", "/artifacts", []string{"BTGemini", "BTDeepseek"}, "")
	if !strings.Contains(out, "design the caching layer") {
		t.Fatal("leader payload missing topic")
	}
}

func TestFormatLeaderPayload_ContainsParticipants(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini", "BTDeepseek"}, "")
	if !strings.Contains(out, "BTGemini") {
		t.Fatal("leader payload missing BTGemini")
	}
	if !strings.Contains(out, "BTDeepseek") {
		t.Fatal("leader payload missing BTDeepseek")
	}
}

func TestFormatLeaderPayload_ContainsArtifactsDir(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/my/artifacts", []string{"BTGemini"}, "")
	if !strings.Contains(out, "/my/artifacts") {
		t.Fatal("leader payload missing artifacts dir")
	}
}

func TestFormatLeaderPayload_ContainsLastCall(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini"}, "")
	if !strings.Contains(out, "Last call") {
		t.Fatal("leader payload missing Last call instruction")
	}
}

func TestFormatLeaderPayload_ContainsDismissInstruction(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini"}, "")
	if !strings.Contains(out, "@Summoner dismiss") {
		t.Fatal("leader payload missing dismiss instruction")
	}
}

func TestFormatLeaderPayload_InjectsLeaderMd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "LEADER.md"), []byte("Write artifacts to /projects/myproject/docs/"), 0644); err != nil {
		t.Fatal(err)
	}
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini"}, dir)
	if !strings.Contains(out, "Write artifacts to /projects/myproject/docs/") {
		t.Fatal("leader payload missing injected LEADER.md content")
	}
}

func TestFormatLeaderPayload_NoInjectionWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini"}, dir)
	if strings.Contains(out, "Additional Instructions") {
		t.Fatal("leader payload should not contain injection section when LEADER.md absent")
	}
}

func TestFormatLeaderPayload_NoInjectionWhenDirEmpty(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini"}, "")
	if strings.Contains(out, "Additional Instructions") {
		t.Fatal("leader payload should not contain injection section when instructionsDir is empty")
	}
}

func TestFormatLeaderPayload_CacheReloadsOnMtimeChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "LEADER.md")

	if err := os.WriteFile(path, []byte("first version"), 0644); err != nil {
		t.Fatal(err)
	}
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini"}, dir)
	if !strings.Contains(out, "first version") {
		t.Fatal("expected first version in payload")
	}

	// Advance mtime by 1s so the cache considers it stale.
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("second version"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}

	out = spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini"}, dir)
	if !strings.Contains(out, "second version") {
		t.Fatal("expected second version after mtime change")
	}
}

func TestFormatParticipantPayload_ContainsTopic(t *testing.T) {
	out := spawner.FormatParticipantPayload("design the caching layer", "BTClaude", "")
	if !strings.Contains(out, "design the caching layer") {
		t.Fatal("participant payload missing topic")
	}
}

func TestFormatParticipantPayload_ContainsLeaderName(t *testing.T) {
	out := spawner.FormatParticipantPayload("topic", "BTClaude", "")
	if !strings.Contains(out, "BTClaude") {
		t.Fatal("participant payload missing leader name")
	}
}

func TestFormatParticipantPayload_ContainsWaitInstruction(t *testing.T) {
	out := spawner.FormatParticipantPayload("topic", "BTClaude", "")
	if !strings.Contains(out, "wait") && !strings.Contains(out, "addressed") {
		t.Fatal("participant payload missing wait/addressed instruction")
	}
}

func TestFormatParticipantPayload_InjectsParticipantMd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PARTICIPANT.md"), []byte("Focus only on security tradeoffs."), 0644); err != nil {
		t.Fatal(err)
	}
	out := spawner.FormatParticipantPayload("topic", "BTClaude", dir)
	if !strings.Contains(out, "Focus only on security tradeoffs.") {
		t.Fatal("participant payload missing injected PARTICIPANT.md content")
	}
}

func TestFormatParticipantPayload_NoInjectionWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	out := spawner.FormatParticipantPayload("topic", "BTClaude", dir)
	if strings.Contains(out, "Additional Instructions") {
		t.Fatal("participant payload should not contain injection section when PARTICIPANT.md absent")
	}
}

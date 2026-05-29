package spawner_test

import (
	"strings"
	"testing"

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
	out := spawner.FormatLeaderPayload("design the caching layer", "/artifacts", []string{"BTGemini", "BTDeepseek"})
	if !strings.Contains(out, "design the caching layer") {
		t.Fatal("leader payload missing topic")
	}
}

func TestFormatLeaderPayload_ContainsParticipants(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini", "BTDeepseek"})
	if !strings.Contains(out, "BTGemini") {
		t.Fatal("leader payload missing BTGemini")
	}
	if !strings.Contains(out, "BTDeepseek") {
		t.Fatal("leader payload missing BTDeepseek")
	}
}

func TestFormatLeaderPayload_ContainsArtifactsDir(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/my/artifacts", []string{"BTGemini"})
	if !strings.Contains(out, "/my/artifacts") {
		t.Fatal("leader payload missing artifacts dir")
	}
}

func TestFormatLeaderPayload_ContainsLastCall(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini"})
	if !strings.Contains(out, "Last call") {
		t.Fatal("leader payload missing Last call instruction")
	}
}

func TestFormatLeaderPayload_ContainsDismissInstruction(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini"})
	if !strings.Contains(out, "@Summoner dismiss") {
		t.Fatal("leader payload missing dismiss instruction")
	}
}

func TestFormatParticipantPayload_ContainsTopic(t *testing.T) {
	out := spawner.FormatParticipantPayload("design the caching layer", "BTClaude")
	if !strings.Contains(out, "design the caching layer") {
		t.Fatal("participant payload missing topic")
	}
}

func TestFormatParticipantPayload_ContainsLeaderName(t *testing.T) {
	out := spawner.FormatParticipantPayload("topic", "BTClaude")
	if !strings.Contains(out, "BTClaude") {
		t.Fatal("participant payload missing leader name")
	}
}

func TestFormatParticipantPayload_ContainsWaitInstruction(t *testing.T) {
	out := spawner.FormatParticipantPayload("topic", "BTClaude")
	if !strings.Contains(out, "wait") && !strings.Contains(out, "addressed") {
		t.Fatal("participant payload missing wait/addressed instruction")
	}
}

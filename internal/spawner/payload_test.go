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

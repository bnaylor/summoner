package spawner_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bnaylor/summoner/internal/spawner"
)

func TestSpawnError_CausePrefersStdout(t *testing.T) {
	e := &spawner.SpawnError{
		Model:    "claude",
		ExitCode: 1,
		Stdout:   "Credit balance is too low\n",
		Err:      errors.New("exit status 1"),
	}
	if got := e.Cause(); got != "Credit balance is too low" {
		t.Fatalf("Cause() = %q, want %q", got, "Credit balance is too low")
	}
}

func TestSpawnError_CauseFallsBackToStderr(t *testing.T) {
	e := &spawner.SpawnError{
		Model:    "claude",
		ExitCode: 1,
		Stdout:   " \n",
		Stderr:   "API key rejected\n",
		Err:      errors.New("exit status 1"),
	}
	if got := e.Cause(); got != "API key rejected" {
		t.Fatalf("Cause() = %q, want %q", got, "API key rejected")
	}
}

func TestSpawnError_CauseFallsBackToExitCode(t *testing.T) {
	e := &spawner.SpawnError{
		Model:    "claude",
		ExitCode: 2,
		Err:      errors.New("exit status 2"),
	}
	if got := e.Cause(); got != "exit status 2" {
		t.Fatalf("Cause() = %q, want %q", got, "exit status 2")
	}
}

func TestSpawnError_CauseCollapsesWhitespace(t *testing.T) {
	e := &spawner.SpawnError{
		Model:  "claude",
		Stdout: "line one\nline two\t  line three",
		Err:    errors.New("exit status 1"),
	}
	got := e.Cause()
	if strings.ContainsAny(got, "\n\t") {
		t.Fatalf("Cause() still contains whitespace: %q", got)
	}
	if got != "line one line two line three" {
		t.Fatalf("Cause() = %q", got)
	}
}

func TestSpawnError_Unwrap(t *testing.T) {
	inner := errors.New("exit status 1")
	e := &spawner.SpawnError{Err: inner}
	if !errors.Is(e, inner) {
		t.Fatal("errors.Is should find the wrapped error via Unwrap")
	}
}

package trigger_test

import (
	"testing"

	"github.com/bnaylor/summoner/internal/trigger"
)

func TestParse_NotMentioned(t *testing.T) {
	_, ok := trigger.Parse("hello world", "123456", "")
	if ok {
		t.Fatal("expected no command when bot not mentioned")
	}
}

func TestParse_Dismiss(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> dismiss", "123456", "")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Type != trigger.CommandDismiss {
		t.Fatalf("expected dismiss, got %v", cmd.Type)
	}
}

func TestParse_SummonClaude(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> claude let's design the auth layer", "123456", "")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Type != trigger.CommandSummon {
		t.Fatalf("expected summon, got %v", cmd.Type)
	}
	if cmd.Model != "claude" {
		t.Fatalf("expected claude, got %q", cmd.Model)
	}
	if cmd.Variant != "" {
		t.Fatalf("expected no variant, got %q", cmd.Variant)
	}
	if cmd.Prompt != "let's design the auth layer" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParse_SummonClaudeOpus(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> claude opus deep dive on tradeoffs", "123456", "")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Model != "claude" {
		t.Fatalf("expected claude, got %q", cmd.Model)
	}
	if cmd.Variant != "opus" {
		t.Fatalf("expected opus, got %q", cmd.Variant)
	}
	if cmd.Prompt != "deep dive on tradeoffs" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParse_SummonGeminiPro(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> gemini pro review the schema", "123456", "")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Model != "gemini" {
		t.Fatalf("expected gemini, got %q", cmd.Model)
	}
	if cmd.Variant != "pro" {
		t.Fatalf("expected pro, got %q", cmd.Variant)
	}
	if cmd.Prompt != "review the schema" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParse_SummonBoth(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> both we're stuck", "123456", "")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Model != "both" {
		t.Fatalf("expected both, got %q", cmd.Model)
	}
}

func TestParse_UnknownCommand(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> bogus", "123456", "")
	if !ok {
		t.Fatal("expected command (mention present)")
	}
	if cmd.Type != trigger.CommandUnknown {
		t.Fatalf("expected unknown, got %v", cmd.Type)
	}
}

func TestParse_MentionWithBang(t *testing.T) {
	cmd, ok := trigger.Parse("<@!123456> claude help", "123456", "")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Model != "claude" {
		t.Fatalf("expected claude, got %q", cmd.Model)
	}
}

func TestParse_RoleMention(t *testing.T) {
	cmd, ok := trigger.Parse("<@&999> claude design the cache", "123456", "999")
	if !ok {
		t.Fatal("expected command via role mention")
	}
	if cmd.Type != trigger.CommandSummon {
		t.Fatalf("expected summon, got %v", cmd.Type)
	}
	if cmd.Model != "claude" {
		t.Fatalf("expected claude, got %q", cmd.Model)
	}
	if cmd.Prompt != "design the cache" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParse_RoleMentionEmptyRoleID(t *testing.T) {
	_, ok := trigger.Parse("<@&999> claude design", "123456", "")
	if ok {
		t.Fatal("expected no match when roleID is empty")
	}
}

func TestParse_RoundtableClaudeLeader(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> roundtable claude design the auth system", "123456", "")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Type != trigger.CommandRoundtable {
		t.Fatalf("expected roundtable, got %v", cmd.Type)
	}
	if cmd.Leader != "claude" {
		t.Fatalf("expected leader claude, got %q", cmd.Leader)
	}
	if cmd.Prompt != "design the auth system" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParse_RoundtableGeminiProLeader(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> roundtable gemini pro design the cache", "123456", "")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Type != trigger.CommandRoundtable {
		t.Fatalf("expected roundtable, got %v", cmd.Type)
	}
	if cmd.Leader != "gemini" {
		t.Fatalf("expected leader gemini, got %q", cmd.Leader)
	}
	if cmd.Variant != "pro" {
		t.Fatalf("expected variant pro, got %q", cmd.Variant)
	}
	if cmd.Prompt != "design the cache" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParse_RoundtableDeepseekLeader(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> roundtable deepseek tradeoffs on storage backends", "123456", "")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Type != trigger.CommandRoundtable {
		t.Fatalf("expected roundtable, got %v", cmd.Type)
	}
	if cmd.Leader != "deepseek" {
		t.Fatalf("expected leader deepseek, got %q", cmd.Leader)
	}
	if cmd.Prompt != "tradeoffs on storage backends" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParse_RoundtableDefaultLeader(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> roundtable design the auth system", "123456", "")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Type != trigger.CommandRoundtable {
		t.Fatalf("expected roundtable, got %v", cmd.Type)
	}
	if cmd.Leader != "" {
		t.Fatalf("expected empty leader (use default), got %q", cmd.Leader)
	}
	if cmd.Prompt != "design the auth system" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParse_RoundtableNoArgs(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> roundtable", "123456", "")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Type != trigger.CommandRoundtable {
		t.Fatalf("expected roundtable, got %v", cmd.Type)
	}
	if cmd.Leader != "" {
		t.Fatalf("expected empty leader, got %q", cmd.Leader)
	}
	if cmd.Prompt != "" {
		t.Fatalf("expected empty prompt, got %q", cmd.Prompt)
	}
}

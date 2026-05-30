package spawner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const participantTemplate = `You are being summoned as a seasoned architect to join an ongoing technical
discussion on Discord.

Discord channel ID: %s
Opening context from the person who summoned you: %s

Do the following in order:
1. Call the discord read_messages tool on channel %s to read recent history
   and get up to speed on the conversation.
2. Post your response using the discord send_message tool to channel %s.
   Do not write your reply to stdout — it must go to Discord.

Engage as a thoughtful design partner. Ask clarifying questions, surface
tradeoffs, and push back where appropriate. Be deliberate, not hasty. Do not
produce implementation artifacts; the team will handle those after consensus.

Do not browse the working directory speculatively. Read files only when the
discussion makes a specific file relevant — the directory will accumulate
content over time and pre-reading it wastes time.

When you sense the discussion has reached consensus, say so clearly in Discord
and indicate you are stepping out.`

const leaderTemplate = `You are the session leader for a multi-agent design roundtable on Discord.

Discord channel ID: %s
Topic: %s

The following agents are participating and will respond when you address them:
%s

Each time you are spawned:
1. Call the discord read_messages tool on channel %s to read the full channel
   history and understand where the conversation left off.
2. Post your response using the discord send_message tool to channel %s.
   Do not write your reply to stdout — it must go to Discord.

Your responsibilities:
- Drive the discussion. Ask targeted questions and address a participant by
  @mentioning their display name (e.g. @BTGemini) — this signals Summoner to
  re-spawn them.
- After each participant responds, synthesize what you heard before moving on.
- Keep the discussion focused. If it drifts, redirect it.
- When consensus is near, announce "Last call!" and ask if anyone has lingering
  concerns before closing.
- Once consensus is confirmed, write the agreed design as a Markdown document
  to: %s
  Then post a brief summary to Discord and issue: @Summoner dismiss

You can also add a model mid-session: @Summoner summon <model>

Do not browse the working directory speculatively. Read files only when the
discussion makes a specific file relevant — the directory will accumulate
content over time and pre-reading it wastes time.

Your previous messages are in the channel history — pick up exactly where you
left off each time you are re-spawned.`

// FormatPayload returns the -p payload for a non-roundtable summoned CLI.
func FormatPayload(channelID, initialPrompt string) string {
	return fmt.Sprintf(participantTemplate, channelID, initialPrompt, channelID, channelID)
}

// FormatLeaderPayload returns the -p payload for the roundtable leader.
// participants is the list of display names (e.g. "BTGemini", "BTDeepseek").
// artifactsDir is the filesystem path where the leader should write output docs.
// instructionsDir is the path to check for a LEADER.md override file; empty disables injection.
func FormatLeaderPayload(channelID, topic, artifactsDir string, participants []string, instructionsDir string) string {
	participantList := "  - " + strings.Join(participants, "\n  - ")
	base := fmt.Sprintf(leaderTemplate, channelID, topic, participantList, channelID, channelID, artifactsDir)
	if extra := readInstruction(instructionsDir, "LEADER.md"); extra != "" {
		base += "\n\n## Additional Instructions\n\n" + extra
	}
	return base
}

// FormatParticipantPayload returns the -p payload for a roundtable participant.
// leaderDisplayName is the Discord display name of the leader (e.g. "BTClaude").
// instructionsDir is the path to check for a PARTICIPANT.md override file; empty disables injection.
func FormatParticipantPayload(channelID, topic, leaderDisplayName, instructionsDir string) string {
	const tmpl = `You are a participant in a multi-model design roundtable on Discord.

Discord channel ID: %s
Topic: %s

%s is leading this session. Each time you are spawned:
1. Call the discord read_messages tool on channel %s to read the full channel
   history and understand the current state of the discussion.
2. Post your response using the discord send_message tool to channel %s.
   Do not write your reply to stdout — it must go to Discord.

Your role:
- Wait until the leader directly addresses you before contributing.
  Do not post on your own initiative.
- When addressed, respond substantively and concisely. Challenge assumptions,
  surface alternatives, and flag risks you see.
- Do not write files or take unilateral action — that is the leader's job.
- Do not dismiss the session — that is the leader's call.
- Do not browse the working directory speculatively. Read files only when the
  discussion makes a specific file relevant.`
	base := fmt.Sprintf(tmpl, channelID, topic, leaderDisplayName, channelID, channelID)
	if extra := readInstruction(instructionsDir, "PARTICIPANT.md"); extra != "" {
		base += "\n\n## Additional Instructions\n\n" + extra
	}
	return base
}

// instruction cache — mtime-keyed so stale files are reloaded automatically.
type cachedFile struct {
	content string
	modTime time.Time
}

var (
	fileCacheMu sync.Mutex
	fileCache   = map[string]cachedFile{}
)

// readInstruction returns the contents of name inside dir, or "" if the file
// does not exist or cannot be read. Results are cached by path and reloaded
// whenever the file's modification time changes.
func readInstruction(dir, name string) string {
	if dir == "" {
		return ""
	}
	path := filepath.Join(dir, name)

	info, err := os.Stat(path)
	if err != nil {
		return ""
	}

	fileCacheMu.Lock()
	cached, hit := fileCache[path]
	fileCacheMu.Unlock()

	if hit && !info.ModTime().After(cached.modTime) {
		return cached.content
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	entry := cachedFile{content: string(data), modTime: info.ModTime()}
	fileCacheMu.Lock()
	fileCache[path] = entry
	fileCacheMu.Unlock()

	return entry.content
}

# Summoner

An always-on Discord bot that lets your agents call in frontier model CLI harnesses for task-bound design consultations.

Your lightweight always-on bots run cheap models day-to-day. When they hit a hard design decision, they summon Claude or Gemini into the channel as a seasoned second opinion. The summoned agent joins the discussion, engages as a thoughtful design partner, and steps out when consensus is reached. No artifacts — just deliberation.

---

## How it works

Three actors:

```
┌─────────────────────────────────────────────────────────┐
│                      Discord channel                     │
│                                                         │
│  Hermes bot ──@Summoner claude opus design the cache──► │
│                         │                               │
│               ┌─────────▼──────────┐                   │
│               │      Summoner      │  always-on bot     │
│               │  (orchestrator)    │  parses @mentions  │
│               └─────────┬──────────┘  manages sessions  │
│                         │                               │
│               ┌─────────▼──────────┐                   │
│               │  claude -p "..."   │  ephemeral CLI     │
│               │   (BTClaude)       │  posts directly    │
│               └─────────┬──────────┘  via discord-mcp  │
│                         │                               │
│  BTClaude ◄─────────────┘  reads channel history,      │
│            posts response  asks questions, surfaces     │
│                            tradeoffs                    │
└─────────────────────────────────────────────────────────┘
```

Summoned agents use their own Discord bot tokens and post directly via [discord-mcp](https://github.com/SaseQ/discord-mcp). Summoner only posts its own orchestration announcements.

---

## Commands

All commands @mention the Summoner bot.

### Summon

```
@Summoner <model> [variant] [prompt]
```

| Token | Values |
|---|---|
| `model` | `claude`, `gemini`, `both` |
| `variant` | `opus` `sonnet` `haiku` (Claude) · `pro` `flash` (Gemini) · omit for default |
| `prompt` | rest of message — opening context for the session |

```
@Summoner claude let's design the caching layer
@Summoner claude opus we need deep reasoning on this tradeoff
@Summoner gemini pro take a look at what we've got in /shared/myproject
@Summoner both we're stuck on an architecture decision, fresh eyes needed
```

### Dismiss

```
@Summoner dismiss
```

### Session rules

- One active session per channel. A second summon while a session is active adds the new model rather than starting fresh.
- Sessions auto-close after 20 minutes of inactivity. Summoner announces departure either way.
- The summoned agent reads the channel's recent history and the NFS share for context — Summoner doesn't relay messages.

---

## Session lifecycle

```
@Summoner claude opus let's design the caching layer
        │
        ▼
  📡 Summoning BTClaude (opus). Stand by...
  session created, inactivity timer started (20m)
        │
        ▼
  claude --model claude-opus-4-7 -p "<payload>"
  cwd: /nfs/shared   env: inherits ANTHROPIC_API_KEY
        │
        ▼
  BTClaude reads channel history via discord-mcp
  posts response directly to Discord
        │
        ▼
  human/bot replies ──► inactivity timer resets
                         claude re-spawned with same payload
        │
        ▼
  @Summoner dismiss  ──► "BTClaude is leaving. o7"
    OR inactivity    ──► "BTClaude has gone quiet and stepped out."
        │
        ▼
  session removed
```

Re-spawns happen on human/Hermes turns only — not when a summoned agent posts — preventing feedback loops.

---

## Configuration

All configuration via environment variables:

| Variable | Purpose | Default |
|---|---|---|
| `SUMMONER_TOKEN` | Summoner bot's Discord token | required |
| `BTCLAUDE_TOKEN` | BTClaude Discord token (discord-mcp sidecar) | required if using claude |
| `BTGEMINI_TOKEN` | BTGemini Discord token (discord-mcp sidecar) | required if using gemini |
| `ANTHROPIC_API_KEY` | Inherited by spawned `claude` processes | required if using claude |
| `GEMINI_API_KEY` | Inherited by spawned `gemini` processes | required if using gemini |
| `NFS_MOUNT` | Working directory for spawned CLIs | `/nfs/shared` |
| `INACTIVITY_TIMEOUT` | Session idle timeout | `20m` |
| `CLAUDE_DEFAULT_MODEL` | Claude model when no variant specified | CLI default |
| `GEMINI_DEFAULT_MODEL` | Gemini model when no variant specified | CLI default |

---

## Building

```bash
docker build -t summoner:latest .
```

The image is intentionally rebuilt frequently — `claude-code` and `gemini-cli` are installed unpinned so a weekly rebuild picks up CLI updates automatically.

---

## Deployment

Three containers in one pod:

```
┌─────────────────────────── k8s Pod ────────────────────────────┐
│                                                                 │
│  ┌──────────────┐   ┌─────────────────┐   ┌────────────────┐  │
│  │   summoner   │   │ discord-mcp     │   │ discord-mcp    │  │
│  │  (Go binary) │   │ claude          │   │ gemini         │  │
│  │              │   │ :8085           │   │ :8086          │  │
│  │ claude ──────┼──►│ BTCLAUDE_TOKEN  │   │ BTGEMINI_TOKEN │  │
│  │ gemini ──────┼───┼─────────────────┼──►│                │  │
│  │              │   │                 │   │                │  │
│  └──────────────┘   └─────────────────┘   └────────────────┘  │
│         │                                                       │
│  /nfs/shared (read-only NFS mount)                             │
└─────────────────────────────────────────────────────────────────┘
```

The discord-mcp sidecars ([SaseQ/discord-mcp](https://github.com/SaseQ/discord-mcp)) run in HTTP mode. The pre-baked MCP configs in `deploy/` point each CLI at its sidecar over localhost.

Create the secret:

```bash
kubectl create secret generic summoner-secrets \
  --from-literal=SUMMONER_TOKEN=<summoner-bot-token> \
  --from-literal=BTCLAUDE_TOKEN=<btclaude-token> \
  --from-literal=BTGEMINI_TOKEN=<btgemini-token> \
  --from-literal=ANTHROPIC_API_KEY=<anthropic-key> \
  --from-literal=GEMINI_API_KEY=<gemini-key>
```

Update the NFS host in `deploy/deployment.yaml`, then:

```bash
kubectl apply -f deploy/deployment.yaml
```

# Summoner

An always-on Discord bot that lets your agents call in frontier model CLI harnesses for task-bound design consultations — or kick off a structured multi-model roundtable.

Your lightweight always-on bots run cheap models day-to-day. When they hit a hard design decision, they summon Claude, Gemini, or Deepseek into the channel as a seasoned second opinion. The summoned agent joins the discussion, engages as a thoughtful design partner, and steps out when consensus is reached.

For larger decisions, a **roundtable** session summons all configured models at once, with a designated leader driving the discussion and writing output artifacts when consensus is reached.

---

## How it works

Three actors:

```
┌─────────────────────────────────────────────────────────┐
│                      Discord channel                    │
│                                                         │
│  Hermes bot ──@Summoner claude opus design the cache──► │
│                         │                               │
│               ┌─────────▼──────────┐                    │
│               │      Summoner      │  always-on bot     │
│               │  (orchestrator)    │  parses @mentions  │
│               └─────────┬──────────┘  manages sessions  │
│                         │                               │
│               ┌─────────▼──────────┐                    │
│               │  claude -p "..."   │  ephemeral CLI     │
│               │   (BTClaude)       │  posts directly    │
│               └─────────┬──────────┘  via discord-mcp   │
│                         │                               │
│  BTClaude ◄─────────────┘  reads channel history,       │
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
| `model` | `claude`, `gemini`, `deepseek` |
| `variant` | `opus` `sonnet` `haiku` (Claude) · `pro` `flash` (Gemini) · omit for default |
| `prompt` | rest of message — opening context for the session |

```
@Summoner claude let's design the caching layer
@Summoner claude opus we need deep reasoning on this tradeoff
@Summoner gemini pro take a look at what we've got in /shared/myproject
@Summoner deepseek what are the tradeoffs on these storage backends?
```

To bring in multiple models, use `@Summoner roundtable` instead.

### Roundtable

```
@Summoner roundtable [leader-model] [prompt]
```

Summons all configured models at once. One model is the **leader** — it drives the discussion by asking targeted questions and @mentioning participants to give them the floor. When consensus is reached, the leader writes a Markdown decision doc to `ARTIFACTS_DIR` and dismisses the session.

| Token | Values |
|---|---|
| `leader-model` | `claude`, `gemini`, `deepseek` · omit to use `ROUNDTABLE_LEADER` default |
| `prompt` | the design topic |

```
@Summoner roundtable design the auth layer
@Summoner roundtable claude design the caching strategy
@Summoner roundtable gemini pro evaluate these three schema options
```

**Roundtable routing rules:**

- Leader @mentions a participant → Summoner re-spawns that participant
- Human or participant posts → Summoner re-spawns the leader
- Leader issues `@Summoner dismiss` → session ends (leader is on the command allowlist)
- Leader issues `@Summoner summon <model>` → adds a model mid-session

### Dismiss

```
@Summoner dismiss
```

---

## Session lifecycle

### Standard summon

```
@Summoner claude opus let's design the caching layer
        │
        ▼
  Summoning BTClaude (opus). Stand by...
  session created, inactivity timer started (20m)
        │
        ▼
  claude --model claude-opus-4-7 -p "<payload>"
  cwd: WORK_DIR   env: inherits ANTHROPIC_API_KEY
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

### Roundtable

```
@Summoner roundtable claude design the event bus
        │
        ▼
  Starting roundtable. BTClaude is leading. Summoning participants...
  Summoning BTClaude...   ← leader payload: drive discussion, write artifacts
  Summoning BTGemini...   ← participant payload: wait to be addressed
  Summoning BTDeepseek... ← participant payload: wait to be addressed
        │
        ▼
  BTClaude reads history, opens discussion, @mentions BTGemini
        │
        ▼
  Summoner sees BTGemini @mention ──► re-spawns BTGemini
        │
        ▼
  BTGemini responds, @mentions BTClaude implicitly (no action from Summoner)
        │
        ▼
  Human or participant posts ──► Summoner re-spawns BTClaude (leader)
        │
        ▼
  BTClaude: "Last call! Any lingering concerns?"
        │
        ▼
  BTClaude writes decision doc to ARTIFACTS_DIR, posts summary
  BTClaude: "@Summoner dismiss"
        │
        ▼
  session removed
```

---

## Configuration

All configuration via environment variables:

| Variable | Purpose | Default |
|---|---|---|
| `SUMMONER_TOKEN` | Summoner bot's Discord token | required |
| `BTCLAUDE_TOKEN` | BTClaude Discord token (discord-mcp sidecar) | optional |
| `BTGEMINI_TOKEN` | BTGemini Discord token (discord-mcp sidecar) | optional |
| `BTDEEPSEEK_TOKEN` | BTDeepseek Discord token (discord-mcp sidecar) | optional |
| `ANTHROPIC_API_KEY` | Inherited by spawned `claude`/`claude-ds` processes | required if using claude or deepseek |
| `GEMINI_API_KEY` | Inherited by spawned `gemini` processes | required if using gemini |
| `DEEPSEEK_API_KEY` | Inherited by spawned `claude-ds` processes if needed | optional |
| `WORK_DIR` | Working directory for spawned CLIs | `.` (or `NFS_MOUNT` if set) |
| `ARTIFACTS_DIR` | Where the roundtable leader writes output docs | `WORK_DIR` |
| `INACTIVITY_TIMEOUT` | Session idle timeout | `20m` |
| `CLAUDE_DEFAULT_MODEL` | Claude model when no variant specified | CLI default |
| `GEMINI_DEFAULT_MODEL` | Gemini model when no variant specified | CLI default |
| `DEEPSEEK_CMD` | CLI binary name for Deepseek | `claude-ds` |
| `DEEPSEEK_DEFAULT_MODEL` | Deepseek model ID passed via `--model` | CLI default |
| `ROUNDTABLE_LEADER` | Default leader model when not specified in command | `claude` |

Only models with a configured bot token are activated. A roundtable with only `BTCLAUDE_TOKEN` and `BTGEMINI_TOKEN` summons two models.

---

## Building

```bash
docker build -t summoner:latest .
```

The image is intentionally rebuilt frequently — `claude-code` and `agy` are installed unpinned so a weekly rebuild picks up CLI updates automatically.

---

## Deployment

Four containers in one pod:

```
┌──────────────────────────────── k8s Pod ──────────────────────────────┐
│                                                                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌───────────┐  │
│  │   summoner   │  │ discord-mcp  │  │ discord-mcp  │  │discord-mcp│  │
│  │  (Go binary) │  │   claude     │  │   gemini     │  │ deepseek  │  │
│  │              │  │   :8085      │  │   :8086      │  │  :8087    │  │
│  └──────┬───────┘  └──────────────┘  └──────────────┘  └───────────┘  │
│         │                                                             │
│  /nfs/shared (NFS mount — WORK_DIR + ARTIFACTS_DIR)                   │
└───────────────────────────────────────────────────────────────────────┘
```

The discord-mcp sidecars ([SaseQ/discord-mcp](https://github.com/SaseQ/discord-mcp)) run in HTTP mode. The pre-baked MCP configs in `deploy/` point each CLI at its sidecar over localhost.

Create the secret:

```bash
kubectl create secret generic summoner-secrets \
  --from-literal=SUMMONER_TOKEN=<summoner-bot-token> \
  --from-literal=BTCLAUDE_TOKEN=<btclaude-token> \
  --from-literal=BTGEMINI_TOKEN=<btgemini-token> \
  --from-literal=BTDEEPSEEK_TOKEN=<btdeepseek-token> \
  --from-literal=ANTHROPIC_API_KEY=<anthropic-key> \
  --from-literal=GEMINI_API_KEY=<gemini-key> \
  --from-literal=DEEPSEEK_API_KEY=<deepseek-key> \
  --dry-run=client -o yaml | kubectl apply -f -
```

Update the NFS host in `deploy/deployment.yaml`, then:

```bash
kubectl apply -f deploy/deployment.yaml
```

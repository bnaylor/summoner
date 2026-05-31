#!/bin/sh

# Seed ~/.claude.json with MCP server config before claude runs.
# claude reads mcpServers from ~/.claude.json, not ~/.claude/settings.json.
# We write it here rather than at image build time because claude mcp add
# requires an API key that isn't present during docker build.
# claude merges its runtime state (feature flags etc.) into the existing file
# on first run, so seeding it here is safe.
printf '{"mcpServers":{"discord":{"type":"sse","url":"http://localhost:8085/mcp"}}}\n' \
    > /home/agent/.claude.json

# Same for BTDeepseek — claude-ds overrides HOME to /home/agent/deepseek-home
# so it reads a separate .claude.json pointing at port 8087.
mkdir -p /home/agent/deepseek-home
printf '{"mcpServers":{"discord":{"type":"sse","url":"http://localhost:8087/mcp"}}}\n' \
    > /home/agent/deepseek-home/.claude.json

# Write the claude-ds API config from DEEPSEEK_API_KEY.
# The config cannot be passed via env vars — claude-ds requires the file.
if [ -n "$DEEPSEEK_API_KEY" ]; then
    mkdir -p /home/agent/deepseek-home/.config/claude-ds
    chmod 700 /home/agent/deepseek-home/.config/claude-ds
    printf '_schema=1\napi_key_ref=%s\nbase_url=https://api.deepseek.com/anthropic\nmodel=deepseek-chat\n' \
        "$DEEPSEEK_API_KEY" > /home/agent/deepseek-home/.config/claude-ds/config
    chmod 600 /home/agent/deepseek-home/.config/claude-ds/config
fi

exec summoner "$@"

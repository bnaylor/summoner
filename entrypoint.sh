#!/bin/sh
# Write the claude-ds config from DEEPSEEK_API_KEY at startup.
# The config cannot be passed via env vars — claude-ds requires the file.
# We use /root/deepseek-home as HOME for claude-ds so its child `claude`
# process reads a separate settings.json (port 8087) instead of BTClaude's
# settings.json (port 8085).
if [ -n "$DEEPSEEK_API_KEY" ]; then
    mkdir -p /root/deepseek-home/.config/claude-ds
    chmod 700 /root/deepseek-home/.config/claude-ds
    printf '_schema=1\napi_key_ref=%s\nbase_url=https://api.deepseek.com/anthropic\nmodel=deepseek-chat\n' \
        "$DEEPSEEK_API_KEY" > /root/deepseek-home/.config/claude-ds/config
    chmod 600 /root/deepseek-home/.config/claude-ds/config
fi
exec summoner "$@"

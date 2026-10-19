#!/bin/sh
set -eu

# Write wherever opencode would keep state; every root must be private.
mkdir -p "$HOME/.local/share/opencode" "$XDG_DATA_HOME/opencode" "$XDG_STATE_HOME/opencode"
printf '%s\n' "launcher touched HOME" >"$HOME/.local/share/opencode/launcher-state"
printf '%s\n' "launcher touched data" >"$XDG_DATA_HOME/opencode/launcher-state"
printf '%s\n' "launcher touched state" >"$XDG_STATE_HOME/opencode/launcher-state"

case "${1:-}" in
--version)
  printf '%s\n' "opencode v2.0.32"
  ;;
serve)
  trap 'exit 0' TERM
  while :; do sleep 1; done
  ;;
run)
  config="$XDG_CONFIG_HOME/opencode/opencode.json"
  result=$(sed -n 's/.*"--result-file","\([^"]*\)".*/\1/p' "$config")
  cat >"$result" <<'EOF'
{"fixture_id":"empty_object","call_count":1,"raw_sha256":"44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","canonical_arguments":{},"schema_sha256":"schema-sha","run_token_sha256":"1923cae09dc7b3efd3d3ef85eb12fb7b7e70e32496c359e44d5c24b4fab442c0","classification":"exact_valid","advertised_valid":true,"canonical_valid":true,"diagnostics":[]}
EOF
  printf '%s\n' '{"type":"tool_use","sessionID":"ses_1","part":{"type":"tool","tool":"issueops_probe_harness_probe_empty_object","state":{"status":"completed","output":"captured"}}}'
  ;;
esac
